package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// webhookHandler processes one change of one webhook field for an account.
type webhookHandler func(ctx context.Context, acc account, value json.RawMessage) error

// account is the workspace-owned WABA a webhook entry belongs to.
type account struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
}

// webhookHandlers is the registry of webhook fields this feature handles,
// filled once at startup (CLAUDE.md: approved "webhook handler registry").
// Fields not listed are ignored.
func (s *Service) webhookHandlers() map[string]webhookHandler {
	return map[string]webhookHandler{
		"messages":                       s.onMessages,
		"phone_number_name_update":       s.onNameUpdate,
		"phone_number_quality_update":    s.onQualityUpdate,
		"account_update":                 s.onAccountUpdate,
		"message_template_status_update": s.onTemplateStatus,
	}
}

// VerifyWebhook answers Meta's subscription handshake.
func (s *Service) VerifyWebhook(mode, token, challenge string) (string, error) {
	if !s.cfg.Enabled {
		return "", model.ErrDisabled
	}
	if mode != "subscribe" || subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.WebhookVerifyToken)) != 1 {
		return "", model.ErrWebhookForbidden
	}
	return challenge, nil
}

// ReceiveWebhook checks the X-Hub-Signature-256 header against the app secret
// and queues the payload. Meta wants a fast 200; the work happens in a job.
func (s *Service) ReceiveWebhook(ctx context.Context, body []byte, signature string) error {
	if !s.cfg.Enabled {
		return model.ErrDisabled
	}
	sig, ok := strings.CutPrefix(signature, "sha256=")
	given, err := hex.DecodeString(sig)
	if !ok || err != nil {
		return model.ErrWebhookSignature
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.AppSecret))
	mac.Write(body)
	if !hmac.Equal(given, mac.Sum(nil)) {
		return model.ErrWebhookSignature
	}
	if !json.Valid(body) {
		return model.ErrInvalidInput
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := s.queue.InsertTx(ctx, tx, model.WebhookArgs{Payload: json.RawMessage(body)}, nil); err != nil {
			return fmt.Errorf("queue webhook: %w", err)
		}
		return nil
	})
}

type webhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"` // the WABA
		Changes []struct {
			Field string          `json:"field"`
			Value json.RawMessage `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// ProcessWebhook dispatches each change to its handler. Handlers are idempotent
// (statuses only move forward), so a redelivered webhook does no harm.
func (s *Service) ProcessWebhook(ctx context.Context, payload []byte) error {
	var p webhookPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode webhook: %w", err)
	}
	if p.Object != "whatsapp_business_account" {
		return nil
	}
	for _, e := range p.Entry {
		acc, found, err := s.accountByWABA(ctx, e.ID)
		if err != nil {
			return err
		}
		if !found {
			continue // a WABA no workspace has (any more)
		}
		for _, c := range e.Changes {
			if h, ok := s.webhooks[c.Field]; ok {
				if err := h(ctx, acc, c.Value); err != nil {
					return fmt.Errorf("webhook %s: %w", c.Field, err)
				}
			}
		}
	}
	return nil
}

func (s *Service) accountByWABA(ctx context.Context, wabaID string) (account, bool, error) {
	row, err := repository.NewRepository(s.pool).GetWhatsAppAccountByWABA(ctx, wabaID)
	if repository.IsNoRows(err) {
		return account{}, false, nil
	}
	if err != nil {
		return account{}, false, fmt.Errorf("look up waba: %w", err)
	}
	return account{ID: row.ID, WorkspaceID: row.WorkspaceID}, true, nil
}

// onMessages handles delivery statuses. Inbound messages belong to the Inbox
// feature; until it exists they are only counted in the log.
func (s *Service) onMessages(ctx context.Context, acc account, value json.RawMessage) error {
	var v struct {
		Statuses []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Errors []struct {
				Code int `json:"code"`
			} `json:"errors"`
		} `json:"statuses"`
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(value, &v); err != nil {
		return fmt.Errorf("decode messages: %w", err)
	}
	if len(v.Messages) > 0 && s.log != nil {
		s.log.InfoContext(ctx, "whatsapp inbound messages", "workspace_id", acc.WorkspaceID, "count", len(v.Messages))
	}
	r := repository.NewRepository(s.pool)
	for _, st := range v.Statuses {
		switch st.Status {
		case "sent", "delivered", "read", "failed":
		default:
			continue
		}
		var code *string
		if st.Status == "failed" && len(st.Errors) > 0 {
			c := fmt.Sprintf("meta_%d", st.Errors[0].Code)
			code = &c
		}
		row, err := r.UpdateTestMessageStatus(ctx, gen.UpdateTestMessageStatusParams{
			WorkspaceID: acc.WorkspaceID, Wamid: &st.ID, Status: st.Status, ErrorCode: code,
		})
		if repository.IsNoRows(err) {
			continue // not a test message, or an older status
		}
		if err != nil {
			return fmt.Errorf("update test message: %w", err)
		}
		if row.Status == "delivered" || row.Status == "read" {
			if err := s.notifier.TestMessageDelivered(ctx, acc.WorkspaceID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) onNameUpdate(ctx context.Context, acc account, value json.RawMessage) error {
	var v struct {
		DisplayPhoneNumber string `json:"display_phone_number"`
		Decision           string `json:"decision"`
	}
	if err := json.Unmarshal(value, &v); err != nil {
		return fmt.Errorf("decode name update: %w", err)
	}
	if err := repository.NewRepository(s.pool).UpdateNumberNameStatus(ctx, gen.UpdateNumberNameStatusParams{
		WorkspaceID: acc.WorkspaceID, AccountID: acc.ID, Digits: model.Digits(v.DisplayPhoneNumber), NameStatus: nameStatus(v.Decision),
	}); err != nil {
		return fmt.Errorf("update name status: %w", err)
	}
	return nil
}

func (s *Service) onQualityUpdate(ctx context.Context, acc account, value json.RawMessage) error {
	var v struct {
		DisplayPhoneNumber string `json:"display_phone_number"`
		Event              string `json:"event"`
		CurrentLimit       string `json:"current_limit"`
	}
	if err := json.Unmarshal(value, &v); err != nil {
		return fmt.Errorf("decode quality update: %w", err)
	}
	var quality *string
	switch v.Event {
	case "FLAGGED":
		quality = qualityRating("red")
	case "UNFLAGGED":
		quality = qualityRating("green")
	}
	if err := repository.NewRepository(s.pool).UpdateNumberQuality(ctx, gen.UpdateNumberQualityParams{
		WorkspaceID: acc.WorkspaceID, AccountID: acc.ID, Digits: model.Digits(v.DisplayPhoneNumber),
		QualityRating: quality, MessagingLimit: optional(v.CurrentLimit),
	}); err != nil {
		return fmt.Errorf("update quality: %w", err)
	}
	return nil
}

// onAccountUpdate disconnects a WABA whose owner removed our access or deleted it.
func (s *Service) onAccountUpdate(ctx context.Context, acc account, value json.RawMessage) error {
	var v struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(value, &v); err != nil {
		return fmt.Errorf("decode account update: %w", err)
	}
	switch v.Event {
	case "PARTNER_REMOVED", "ACCOUNT_DELETED":
		return s.disconnect(ctx, acc.WorkspaceID, acc.ID)
	}
	return nil
}
