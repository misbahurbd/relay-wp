// Package service holds whatsapp's business rules (no HTTP, no SQL).
package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Enqueuer inserts background jobs in a transaction.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Notifier hears about setup milestones. Implemented by features/onboarding.
type Notifier interface {
	TestMessageDelivered(ctx context.Context, workspaceID uuid.UUID) error
}

// Config holds the Meta app settings.
type Config struct {
	Enabled            bool
	AppID              string
	AppSecret          string
	ESConfigID         string
	GraphVersion       string
	WebhookVerifyToken string
	// TokenKey is the AES-256 key for stored tokens and PINs.
	TokenKey []byte
}

// Service connects WhatsApp numbers, sets them up, sends test messages and handles Meta's webhooks.
type Service struct {
	pool     *pgxpool.Pool
	graph    *Graph
	queue    Enqueuer
	notifier Notifier
	plans    PlanLimits // nil: no plan number cap
	// templates hears template status webhooks; nil until SetTemplateStatusSink.
	templates TemplateStatusSink
	box       *secretBox
	cfg       Config
	log       *slog.Logger
	webhooks  map[string]webhookHandler
}

// New builds the service. With cfg.Enabled the Meta app settings are required.
func New(pool *pgxpool.Pool, graph *Graph, queue Enqueuer, notifier Notifier, cfg Config, log *slog.Logger) (*Service, error) {
	if cfg.Enabled && (cfg.AppID == "" || cfg.AppSecret == "" || cfg.ESConfigID == "" || cfg.WebhookVerifyToken == "") {
		return nil, fmt.Errorf("whatsapp: META_APP_ID, META_APP_SECRET, META_ES_CONFIG_ID and META_WEBHOOK_VERIFY_TOKEN are required when WHATSAPP_ENABLED")
	}
	box, err := newSecretBox(cfg.TokenKey)
	if err != nil {
		return nil, err
	}
	s := &Service{pool: pool, graph: graph, queue: queue, notifier: notifier, box: box, cfg: cfg, log: log}
	s.webhooks = s.webhookHandlers()
	return s, nil
}

// SignupConfig returns the public values the frontend needs to open Embedded Signup.
func (s *Service) SignupConfig() model.SignupConfig {
	if !s.cfg.Enabled {
		return model.SignupConfig{}
	}
	return model.SignupConfig{Enabled: true, AppID: s.cfg.AppID, ConfigID: s.cfg.ESConfigID, GraphVersion: s.cfg.GraphVersion}
}

var metaIDRe = regexp.MustCompile(`^[0-9]{5,25}$`)

// Connect finishes Embedded Signup: it swaps the code for a business token,
// checks the token can see the WABA and number, saves both and queues setup.
// Reconnecting a WABA this workspace already has replaces its token.
func (s *Service) Connect(ctx context.Context, workspaceID, userID uuid.UUID, in model.ConnectInput) (model.Number, error) {
	if !s.cfg.Enabled {
		return model.Number{}, model.ErrDisabled
	}
	var details []apperr.Detail
	if !metaIDRe.MatchString(in.WABAID) {
		details = append(details, apperr.Detail{Field: "waba_id", Message: "Invalid WhatsApp Business account ID"})
	}
	if !metaIDRe.MatchString(in.PhoneNumberID) {
		details = append(details, apperr.Detail{Field: "phone_number_id", Message: "Invalid phone number ID"})
	}
	if !slices.Contains(model.NumberTypes, in.NumberType) {
		details = append(details, apperr.Detail{Field: "number_type", Message: "Choose new, coexistence or migration"})
	}
	if len(details) > 0 {
		return model.Number{}, model.ErrInvalidInput.WithDetails(details...)
	}

	// Checked before the one-time code is spent, and again under the lock below.
	limit, err := s.numberLimit(ctx, workspaceID)
	if err != nil {
		return model.Number{}, err
	}
	if err := checkNumberLimit(ctx, s.pool, workspaceID, in.PhoneNumberID, limit); err != nil {
		return model.Number{}, err
	}

	token, err := s.graph.ExchangeCode(ctx, in.Code)
	if err != nil {
		return model.Number{}, graphErr(err, model.ErrCodeExchange)
	}
	waba, err := s.graph.WABA(ctx, token, in.WABAID)
	if err != nil {
		return model.Number{}, graphErr(err, model.ErrCodeExchange)
	}
	phone, err := s.graph.PhoneNumber(ctx, token, in.PhoneNumberID)
	if err != nil {
		return model.Number{}, graphErr(err, model.ErrCodeExchange)
	}
	sealed, err := s.box.seal(token)
	if err != nil {
		return model.Number{}, err
	}

	tasks := map[string]string{}
	for _, t := range model.SetupTasks {
		tasks[t] = model.TaskWaiting
	}
	if in.NumberType == model.TypeCoexistence {
		tasks[model.TaskRegister] = model.TaskDone // stays registered on the WhatsApp Business app
	}
	tasksJSON, err := json.Marshal(tasks)
	if err != nil {
		return model.Number{}, fmt.Errorf("setup tasks: %w", err)
	}

	var numberID uuid.UUID
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if limit >= 0 {
			// Serialize the workspace's connects so two can't both pass the number cap.
			if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))", "whatsapp_numbers:"+workspaceID.String()); err != nil {
				return fmt.Errorf("lock numbers: %w", err)
			}
		}
		if acc, err := r.GetWhatsAppAccountByWABA(ctx, in.WABAID); err == nil && acc.WorkspaceID != workspaceID {
			return model.ErrAlreadyConnected
		} else if err != nil && !repository.IsNoRows(err) {
			return fmt.Errorf("look up waba: %w", err)
		}
		if ws, err := r.GetNumberWorkspace(ctx, in.PhoneNumberID); err == nil && ws != workspaceID {
			return model.ErrNumberInUse
		} else if err != nil && !repository.IsNoRows(err) {
			return fmt.Errorf("look up number: %w", err)
		}
		if err := checkNumberLimit(ctx, tx, workspaceID, in.PhoneNumberID, limit); err != nil {
			return err
		}

		newAccountID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("account id: %w", err)
		}
		accountID, err := r.UpsertWhatsAppAccount(ctx, gen.UpsertWhatsAppAccountParams{
			ID: newAccountID, WorkspaceID: workspaceID, WabaID: in.WABAID, BusinessID: optional(waba.OwnerBusiness.ID),
			Name: waba.Name, AccessToken: sealed, ConnectedBy: &userID,
		})
		if repository.IsNoRows(err) {
			return model.ErrAlreadyConnected // another workspace won a race for this WABA
		}
		if err != nil {
			return fmt.Errorf("save account: %w", err)
		}
		newNumberID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("number id: %w", err)
		}
		numberID, err = r.UpsertWhatsAppNumber(ctx, gen.UpsertWhatsAppNumberParams{
			ID: newNumberID, WorkspaceID: workspaceID, AccountID: accountID, PhoneNumberID: in.PhoneNumberID,
			DisplayPhoneNumber: phone.DisplayPhoneNumber, NumberType: in.NumberType, VerifiedName: phone.VerifiedName,
			NameStatus: nameStatus(phone.NameStatus), QualityRating: qualityRating(phone.QualityRating),
			MessagingLimit: optional(phone.MessagingLimitTier), SetupTasks: tasksJSON,
		})
		if repository.IsNoRows(err) {
			return model.ErrNumberInUse
		}
		if err != nil {
			return fmt.Errorf("save number: %w", err)
		}
		if _, err := s.queue.InsertTx(ctx, tx, model.SetupArgs{WorkspaceID: workspaceID, NumberID: numberID}, nil); err != nil {
			return fmt.Errorf("queue setup: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Number{}, err
	}
	return s.Number(ctx, workspaceID, numberID)
}

// Numbers lists the workspace's numbers.
func (s *Service) Numbers(ctx context.Context, workspaceID uuid.UUID) ([]model.Number, error) {
	rows, err := repository.NewRepository(s.pool).ListWhatsAppNumbers(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list numbers: %w", err)
	}
	out := make([]model.Number, 0, len(rows))
	for _, r := range rows {
		out = append(out, toNumber(gen.GetWhatsAppNumberRow(r)))
	}
	return out, nil
}

// Number returns one of the workspace's numbers.
func (s *Service) Number(ctx context.Context, workspaceID, numberID uuid.UUID) (model.Number, error) {
	row, err := repository.NewRepository(s.pool).GetWhatsAppNumber(ctx, gen.GetWhatsAppNumberParams{WorkspaceID: workspaceID, ID: numberID})
	if repository.IsNoRows(err) {
		return model.Number{}, model.ErrNumberNotFound
	}
	if err != nil {
		return model.Number{}, fmt.Errorf("get number: %w", err)
	}
	return toNumber(row), nil
}

// --- Setup job ---

// RunSetup works through the setup tasks that aren't done yet. Each task is
// safe to repeat. A failure is retried by River; on the last attempt the
// number is marked failed with the reason.
func (s *Service) RunSetup(ctx context.Context, workspaceID, numberID uuid.UUID, lastAttempt bool) error {
	n, err := s.Number(ctx, workspaceID, numberID)
	if errors.Is(err, model.ErrNumberNotFound) {
		return nil // removed since; nothing to set up
	}
	if err != nil {
		return err
	}
	if n.Status != model.StatusSettingUp {
		return nil
	}
	r := repository.NewRepository(s.pool)
	waba, token, err := s.token(ctx, workspaceID, n.AccountID)
	if errors.Is(err, model.ErrDisconnected) {
		return s.setStatus(ctx, r, workspaceID, numberID, model.StatusDisconnected, nil)
	}
	if err != nil {
		return err
	}
	for _, task := range model.SetupTasks {
		if n.SetupTasks[task] == model.TaskDone {
			continue
		}
		if err := s.setTask(ctx, r, workspaceID, numberID, task, model.TaskRunning); err != nil {
			return err
		}
		if err := s.doTask(ctx, r, task, workspaceID, n, waba, token); err != nil {
			if isAuthFailure(err) {
				return s.disconnect(ctx, workspaceID, n.AccountID)
			}
			if !lastAttempt {
				return fmt.Errorf("setup %s: %w", task, err)
			}
			if err := s.setTask(ctx, r, workspaceID, numberID, task, model.TaskFailed); err != nil {
				return err
			}
			return s.setStatus(ctx, r, workspaceID, numberID, model.StatusFailed, setupError(task, err))
		}
		if err := s.setTask(ctx, r, workspaceID, numberID, task, model.TaskDone); err != nil {
			return err
		}
	}
	return s.setStatus(ctx, r, workspaceID, numberID, model.StatusConnected, nil)
}

func (s *Service) doTask(ctx context.Context, r repository.Repository, task string, workspaceID uuid.UUID, n model.Number, waba, token string) error {
	switch task {
	case model.TaskRegister:
		pin, err := s.registrationPin(ctx, r, workspaceID, n.ID)
		if err != nil {
			return err
		}
		return s.graph.Register(ctx, token, n.PhoneNumberID, pin)
	case model.TaskSubscribe:
		return s.graph.SubscribeApp(ctx, token, waba)
	case model.TaskTemplates:
		// Stored by the Templates feature once it exists; counted for now.
		count, err := s.graph.CountTemplates(ctx, token, waba)
		if err == nil && s.log != nil {
			s.log.InfoContext(ctx, "whatsapp templates found", "workspace_id", workspaceID, "count", count)
		}
		return err
	case model.TaskProfile:
		p, err := s.graph.PhoneNumber(ctx, token, n.PhoneNumberID)
		if err != nil {
			return err
		}
		if err := r.UpdateNumberProfile(ctx, gen.UpdateNumberProfileParams{
			WorkspaceID: workspaceID, ID: n.ID, DisplayPhoneNumber: p.DisplayPhoneNumber, VerifiedName: p.VerifiedName,
			NameStatus: nameStatus(p.NameStatus), QualityRating: qualityRating(p.QualityRating),
			MessagingLimit: optional(p.MessagingLimitTier),
		}); err != nil {
			return fmt.Errorf("save profile: %w", err)
		}
		return nil
	}
	return fmt.Errorf("unknown setup task %q", task)
}

// registrationPin returns the number's two-step verification PIN, creating and
// storing one first so a retried registration uses the same PIN.
func (s *Service) registrationPin(ctx context.Context, r repository.Repository, workspaceID, numberID uuid.UUID) (string, error) {
	sealed, err := r.GetNumberPin(ctx, gen.GetNumberPinParams{WorkspaceID: workspaceID, ID: numberID})
	if err != nil {
		return "", fmt.Errorf("load pin: %w", err)
	}
	if len(sealed) > 0 {
		return s.box.open(sealed)
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("pin: %w", err)
	}
	pin := fmt.Sprintf("%06d", n.Int64())
	if sealed, err = s.box.seal(pin); err != nil {
		return "", err
	}
	if err := r.SetNumberPin(ctx, gen.SetNumberPinParams{WorkspaceID: workspaceID, ID: numberID, RegistrationPin: sealed}); err != nil {
		return "", fmt.Errorf("save pin: %w", err)
	}
	return pin, nil
}

func (s *Service) setTask(ctx context.Context, r repository.Repository, workspaceID, numberID uuid.UUID, task, state string) error {
	if err := r.SetNumberTask(ctx, gen.SetNumberTaskParams{WorkspaceID: workspaceID, ID: numberID, Task: task, State: state}); err != nil {
		return fmt.Errorf("set task %s: %w", task, err)
	}
	return nil
}

func (s *Service) setStatus(ctx context.Context, r repository.Repository, workspaceID, numberID uuid.UUID, status string, lastErr *model.SetupError) error {
	var raw []byte
	if lastErr != nil {
		var err error
		if raw, err = json.Marshal(lastErr); err != nil {
			return fmt.Errorf("last error: %w", err)
		}
	}
	if err := r.SetNumberStatus(ctx, gen.SetNumberStatusParams{WorkspaceID: workspaceID, ID: numberID, Status: status, LastError: raw}); err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	return nil
}

func setupError(task string, err error) *model.SetupError {
	e := &model.SetupError{Code: task + "_failed", Message: "Meta couldn't finish this step."}
	if g, ok := asGraphError(err); ok {
		e.MetaCode, e.Message = g.Code, g.Message
	}
	return e
}

// token returns the WABA ID and decrypted business token for an active account.
func (s *Service) token(ctx context.Context, workspaceID, accountID uuid.UUID) (waba, token string, err error) {
	row, err := repository.NewRepository(s.pool).GetAccountToken(ctx, gen.GetAccountTokenParams{WorkspaceID: workspaceID, ID: accountID})
	if repository.IsNoRows(err) {
		return "", "", model.ErrDisconnected
	}
	if err != nil {
		return "", "", fmt.Errorf("load token: %w", err)
	}
	token, err = s.box.open(row.AccessToken)
	return row.WabaID, token, err
}

// disconnect marks an account whose token stopped working, and its numbers.
func (s *Service) disconnect(ctx context.Context, workspaceID, accountID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if err := r.DisconnectWhatsAppAccount(ctx, gen.DisconnectWhatsAppAccountParams{WorkspaceID: workspaceID, ID: accountID}); err != nil {
			return fmt.Errorf("disconnect account: %w", err)
		}
		if err := r.DisconnectAccountNumbers(ctx, gen.DisconnectAccountNumbersParams{WorkspaceID: workspaceID, AccountID: accountID}); err != nil {
			return fmt.Errorf("disconnect numbers: %w", err)
		}
		return nil
	})
}

// SendTestMessage sends the hello_world template from a connected number. A
// message Meta refuses comes back as failed rather than as an error, so the
// page can show why.
func (s *Service) SendTestMessage(ctx context.Context, workspaceID, numberID uuid.UUID, to string) (model.TestMessage, error) {
	if !s.cfg.Enabled {
		return model.TestMessage{}, model.ErrDisabled
	}
	phone := model.Digits(to)
	if len(phone) < 8 || len(phone) > 15 || !strings.HasPrefix(strings.TrimSpace(to), "+") {
		return model.TestMessage{}, model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "to", Message: "Enter a number with its country code"})
	}
	n, err := s.Number(ctx, workspaceID, numberID)
	if err != nil {
		return model.TestMessage{}, err
	}
	switch n.Status {
	case model.StatusConnected:
	case model.StatusDisconnected:
		return model.TestMessage{}, model.ErrDisconnected
	default:
		return model.TestMessage{}, model.ErrNotConnected
	}
	if phone == model.Digits(n.DisplayPhoneNumber) {
		return model.TestMessage{}, model.ErrSameNumber.WithDetails(apperr.Detail{Field: "to", Message: model.ErrSameNumber.Message})
	}
	r := repository.NewRepository(s.pool)
	recent, err := r.CountRecentTestMessages(ctx, workspaceID)
	if err != nil {
		return model.TestMessage{}, fmt.Errorf("count test messages: %w", err)
	}
	if recent >= model.TestMessageLimit {
		return model.TestMessage{}, model.ErrTooManyTests
	}
	id, err := uuid.NewV7()
	if err != nil {
		return model.TestMessage{}, fmt.Errorf("message id: %w", err)
	}
	if err := r.CreateTestMessage(ctx, gen.CreateTestMessageParams{ID: id, WorkspaceID: workspaceID, NumberID: numberID, ToPhone: "+" + phone}); err != nil {
		return model.TestMessage{}, fmt.Errorf("save test message: %w", err)
	}

	_, token, err := s.token(ctx, workspaceID, n.AccountID)
	if err != nil {
		return model.TestMessage{}, err
	}
	wamid, sendErr := s.graph.SendTemplate(ctx, token, n.PhoneNumberID, phone, model.TestTemplate, model.TestTemplateLang)
	switch {
	case sendErr == nil:
		if err := r.SetTestMessageSent(ctx, gen.SetTestMessageSentParams{WorkspaceID: workspaceID, ID: id, Wamid: &wamid}); err != nil {
			return model.TestMessage{}, fmt.Errorf("mark sent: %w", err)
		}
	case isAuthFailure(sendErr):
		if err := s.disconnect(ctx, workspaceID, n.AccountID); err != nil {
			return model.TestMessage{}, err
		}
		return model.TestMessage{}, model.ErrDisconnected
	default:
		g, ok := asGraphError(sendErr)
		if !ok {
			return model.TestMessage{}, model.ErrMetaUnavailable.Wrap(sendErr)
		}
		code := "meta_" + strconv.Itoa(g.Code)
		if err := r.SetTestMessageFailed(ctx, gen.SetTestMessageFailedParams{WorkspaceID: workspaceID, ID: id, ErrorCode: &code}); err != nil {
			return model.TestMessage{}, fmt.Errorf("mark failed: %w", err)
		}
	}
	return s.TestMessage(ctx, workspaceID, id)
}

// TestMessage returns a test message's delivery status.
func (s *Service) TestMessage(ctx context.Context, workspaceID, messageID uuid.UUID) (model.TestMessage, error) {
	row, err := repository.NewRepository(s.pool).GetTestMessage(ctx, gen.GetTestMessageParams{WorkspaceID: workspaceID, ID: messageID})
	if repository.IsNoRows(err) {
		return model.TestMessage{}, model.ErrMessageNotFound
	}
	if err != nil {
		return model.TestMessage{}, fmt.Errorf("get test message: %w", err)
	}
	return model.TestMessage{ID: row.ID, NumberID: row.NumberID, ToPhone: row.ToPhone, Status: row.Status, ErrorCode: row.ErrorCode, CreatedAt: row.CreatedAt}, nil
}

// graphErr turns a Graph failure into an API error: Meta refusing becomes
// refused, Meta (or the network) failing becomes unavailable.
func graphErr(err error, refused *apperr.Error) error {
	if g, ok := asGraphError(err); ok && g.Status < 500 {
		return refused.Wrap(err)
	}
	return model.ErrMetaUnavailable.Wrap(err)
}

func toNumber(r gen.GetWhatsAppNumberRow) model.Number {
	n := model.Number{
		ID: r.ID, AccountID: r.AccountID, PhoneNumberID: r.PhoneNumberID, DisplayPhoneNumber: r.DisplayPhoneNumber,
		NumberType: r.NumberType, VerifiedName: r.VerifiedName, NameStatus: r.NameStatus,
		QualityRating: r.QualityRating, MessagingLimit: r.MessagingLimit, Status: r.Status, CreatedAt: r.CreatedAt,
		SetupTasks: map[string]string{},
	}
	_ = json.Unmarshal(r.SetupTasks, &n.SetupTasks)
	for _, t := range model.SetupTasks {
		if _, ok := n.SetupTasks[t]; !ok {
			n.SetupTasks[t] = model.TaskWaiting
		}
	}
	if len(r.LastError) > 0 {
		var e model.SetupError
		if json.Unmarshal(r.LastError, &e) == nil {
			n.LastError = &e
		}
	}
	return n
}
