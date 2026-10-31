package app

import (
	"context"
	"errors"
	"regexp"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	billingservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/service"
	broadcastsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/model"
	broadcastsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/service"
	contactsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	contactsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/service"
	inboxmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	inboxservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/service"
	templatesmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/model"
	templatesservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/service"
	whatsappmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	whatsappservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/service"
	workspacesservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

var (
	_ broadcastsservice.Usage      = (*billingservice.Service)(nil)
	_ broadcastsservice.Activation = (*workspacesservice.Directory)(nil)
)

// broadcastsInbox queues each broadcast message through the inbox, on the bulk queue (ADR 0013).
type broadcastsInbox struct{ inbox *inboxservice.Service }

func (i broadcastsInbox) SendBroadcast(ctx context.Context, tx pgx.Tx, in broadcastsmodel.Send) (uuid.UUID, error) {
	id, err := i.inbox.SendAutomated(ctx, tx, inboxmodel.AutomatedText{
		WorkspaceID: in.WorkspaceID, ContactID: &in.ContactID, ChannelID: &in.NumberID, TemplateID: &in.TemplateID,
		HeaderText: in.HeaderText, Params: in.Body, Source: inboxmodel.SourceBroadcast, Bulk: true,
	})
	switch {
	case err == nil:
		return id, nil
	case errors.Is(err, inboxmodel.ErrContactOptedOut):
		return uuid.Nil, broadcastsmodel.ErrSendOptedOut
	case errors.Is(err, inboxmodel.ErrChannelUnavailable), errors.Is(err, inboxmodel.ErrTemplatesUnsupported):
		return uuid.Nil, broadcastsmodel.ErrSendNoChannel
	case errors.Is(err, inboxmodel.ErrTemplateNotApproved):
		return uuid.Nil, broadcastsmodel.ErrSendTemplate
	case apperr.IsKind(err, apperr.KindValidation):
		return uuid.Nil, broadcastsmodel.ErrSendMissingVariable
	}
	return uuid.Nil, err
}

// broadcastsTemplates tells broadcasts which placeholders an approved template has.
type broadcastsTemplates struct{ templates *templatesservice.Service }

var placeholder = regexp.MustCompile(`\{\{(\d+)\}\}`)

func (t broadcastsTemplates) ApprovedTemplate(ctx context.Context, ws, id uuid.UUID) (broadcastsmodel.Template, error) {
	tpl, err := t.templates.Get(ctx, ws, id)
	if apperr.IsKind(err, apperr.KindNotFound) || (err == nil && tpl.Status != templatesmodel.StatusApproved) {
		return broadcastsmodel.Template{}, broadcastsmodel.ErrTemplateNotApproved
	}
	if err != nil {
		return broadcastsmodel.Template{}, err
	}
	c := tpl.Components
	out := broadcastsmodel.Template{
		Name: tpl.Name, Category: tpl.Category,
		HeaderVariable: c.Header.Type == "text" && placeholder.MatchString(c.Header.Text),
	}
	// Templates number body placeholders {{1}}…{{n}} without gaps.
	for _, m := range placeholder.FindAllStringSubmatch(c.Body.Text, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > out.BodyVariables {
			out.BodyVariables = n
		}
	}
	return out, nil
}

// broadcastsNumbers reads the number a broadcast sends from.
type broadcastsNumbers struct{ wa *whatsappservice.Service }

func (n broadcastsNumbers) Number(ctx context.Context, ws, id uuid.UUID) (broadcastsmodel.Number, error) {
	numbers, err := n.wa.Numbers(ctx, ws)
	if err != nil {
		return broadcastsmodel.Number{}, err
	}
	for _, num := range numbers {
		if num.ID == id {
			return broadcastsmodel.Number{Connected: num.Status == whatsappmodel.StatusConnected, TierLimit: tierLimit(num.MessagingLimit)}, nil
		}
	}
	return broadcastsmodel.Number{}, broadcastsmodel.ErrNumberNotConnected
}

// tierLimit is Meta's messaging tier as new customers a day; 0 when unknown or unlimited.
func tierLimit(tier *string) int {
	if tier == nil {
		return 0
	}
	switch *tier {
	case "TIER_50":
		return 50
	case "TIER_250":
		return 250
	case "TIER_1K":
		return 1000
	case "TIER_10K":
		return 10000
	case "TIER_100K":
		return 100000
	}
	return 0
}

// broadcastsSegments resolves an audience's contact segments (ADR 0014).
type broadcastsSegments struct{ contacts *contactsservice.Service }

func (s broadcastsSegments) Members(ctx context.Context, ws uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	members, err := s.contacts.SegmentMembers(ctx, ws, ids)
	if errors.Is(err, contactsmodel.ErrSegmentNotFound) {
		return nil, broadcastsmodel.ErrSegmentNotFound
	}
	return members, err
}
