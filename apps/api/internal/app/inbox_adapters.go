package app

import (
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	contactsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/service"
	inboxmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	inboxservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/service"
	integrationsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/service"
	teamservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/team/service"
	templatesmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/model"
	templatesservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/service"
	whatsappmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	whatsappservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// This file is the only place that knows both the inbox and the features it
// works with: whatsapp, contacts and templates (docs/PHASE-7-INBOX.md AD-1).
// Team (MemberDirectory) and integrations (EventPublisher) already have the
// inbox's method signatures and are passed in directly. The inbox is
// channel-neutral (ADR 0010): whatsapp stays WhatsApp-specific, and the
// adapters here convert between the two.

var (
	_ inboxservice.MemberDirectory = (*teamservice.Resolver)(nil)
	_ inboxservice.EventPublisher  = (*integrationsservice.Service)(nil)
)

// inboxChannels is the inbox's channel registry, filled once at startup (an
// approved factory point, ADR 0010). WhatsApp is the only channel today; each new
// channel adds its adapter here. Tests replace it through a.inboxChannels.
func (a *App) inboxChannels() inboxservice.Channels {
	if a.inboxChannelsOverride != nil {
		return a.inboxChannelsOverride
	}
	if a.cfg.WhatsApp.Fake && a.cfg.IsDevelopment() {
		return inboxservice.Channels{inboxmodel.ChannelWhatsApp: &devWhatsApp{app: a}}
	}
	return inboxservice.Channels{inboxmodel.ChannelWhatsApp: whatsappChannel{wa: a.whatsapp}}
}

// ---------- whatsapp → inbox ----------

// inboxSink hears whatsapp's parsed webhooks and hands them to the inbox in its
// own, channel-neutral types: the channel is the number, the message's external
// ID is its wamid, and the sender's is their E.164 phone.
type inboxSink struct{ inbox *inboxservice.Service }

var _ whatsappservice.InboundSink = inboxSink{}

func (s inboxSink) InboundMessage(ctx context.Context, m whatsappmodel.InboundMessage) error {
	in := inboxmodel.Inbound{
		WorkspaceID: m.WorkspaceID, ChannelType: inboxmodel.ChannelWhatsApp, ChannelID: m.NumberID, ExternalID: m.WAMID,
		Sender:    inboxmodel.Sender{ExternalID: m.From, Phone: m.From, Name: m.ProfileName},
		Timestamp: m.Timestamp, Type: m.Type, Text: m.Text, Contacts: m.Contacts, ContextExternalID: m.ContextWAMID,
	}
	if m.Media != nil {
		in.Media = &inboxmodel.InboundMedia{ID: m.Media.ID, Mime: m.Media.MimeType, SHA256: m.Media.SHA256, Filename: m.Media.Filename}
	}
	if m.Location != nil {
		in.Location = &inboxmodel.Location{
			Latitude: m.Location.Latitude, Longitude: m.Location.Longitude, Name: m.Location.Name, Address: m.Location.Address,
		}
	}
	if m.Interactive != nil {
		in.Interactive = &inboxmodel.Interactive{
			Kind: m.Interactive.Kind, ID: m.Interactive.ID, Title: m.Interactive.Title, Description: m.Interactive.Description,
		}
	}
	if m.Reaction != nil {
		in.Reaction = &inboxmodel.InboundReaction{ExternalID: m.Reaction.WAMID, Emoji: m.Reaction.Emoji}
	}
	if m.Referral != nil {
		in.Referral = &inboxmodel.Referral{
			SourceType: m.Referral.SourceType, SourceID: m.Referral.SourceID, SourceURL: m.Referral.SourceURL,
			Headline: m.Referral.Headline, Body: m.Referral.Body,
		}
	}
	return s.inbox.InboundMessage(ctx, in)
}

func (s inboxSink) MessageStatus(ctx context.Context, u whatsappmodel.StatusUpdate) error {
	code := ""
	if u.ErrorCode != 0 {
		code = strconv.Itoa(u.ErrorCode)
	}
	return s.inbox.MessageStatus(ctx, inboxmodel.StatusUpdate{
		WorkspaceID: u.WorkspaceID, ChannelType: inboxmodel.ChannelWhatsApp, ChannelID: u.NumberID, ExternalID: u.WAMID,
		Status: u.Status, Timestamp: u.Timestamp, ErrorCode: code, ErrorTitle: u.ErrorTitle,
	})
}

// ---------- inbox → whatsapp ----------

// whatsappChannel is the inbox's WhatsApp channel, on top of the whatsapp
// service. The channel ID is the number's id, and to is the customer's phone.
type whatsappChannel struct{ wa *whatsappservice.Service }

var _ inboxservice.Channel = whatsappChannel{}

const mb = 1 << 20

// whatsappCapabilities are WhatsApp's rules (docs/PHASE-7-INBOX.md section 11, AD-11).
func whatsappCapabilities() inboxmodel.Capabilities {
	return inboxmodel.Capabilities{
		Window: 24 * time.Hour, Templates: true, Reactions: true, ReadReceipts: true, MaxTextLen: 4096,
		Media: []inboxmodel.MediaKind{
			{Kind: inboxmodel.TypeImage, MaxBytes: 5 * mb, MimeTypes: []string{"image/jpeg", "image/png"}},
			{Kind: inboxmodel.TypeVideo, MaxBytes: 16 * mb, MimeTypes: []string{"video/mp4", "video/3gpp"}},
			{Kind: inboxmodel.TypeAudio, MaxBytes: 16 * mb, MimeTypes: []string{"audio/aac", "audio/mp4", "audio/mpeg", "audio/amr", "audio/ogg"}},
			{Kind: inboxmodel.TypeDocument, MaxBytes: 100 * mb},
		},
	}
}

func (whatsappChannel) Capabilities() inboxmodel.Capabilities { return whatsappCapabilities() }

// Accounts names the workspace's numbers: the verified name and the display phone.
func (c whatsappChannel) Accounts(ctx context.Context, ws uuid.UUID) (map[uuid.UUID]inboxmodel.ChannelAccount, error) {
	numbers, err := c.wa.Numbers(ctx, ws)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]inboxmodel.ChannelAccount, len(numbers))
	for _, n := range numbers {
		out[n.ID] = inboxmodel.ChannelAccount{Name: n.VerifiedName, Handle: n.DisplayPhoneNumber}
	}
	return out, nil
}

// sendFailure turns whatsapp's refusals into the inbox's: Meta's own verdict on
// whether to retry, and whatsapp's client errors (number not connected,
// disconnected, file too large …), which no retry fixes. Anything else is
// returned as is, so the job retries.
func sendFailure(err error) error {
	if err == nil {
		return nil
	}
	var se *whatsappmodel.SendError
	if errors.As(err, &se) {
		title := se.Title
		if se.Message != "" && se.Message != se.Title {
			title = se.Title + ": " + se.Message
		}
		return &inboxmodel.SendFailure{Code: strconv.Itoa(se.Code), Title: title, Permanent: se.Permanent}
	}
	if ae, ok := apperr.As(err); ok {
		switch ae.Kind {
		case apperr.KindNotFound, apperr.KindConflict, apperr.KindValidation, apperr.KindForbidden:
			return &inboxmodel.SendFailure{Code: ae.Code, Title: ae.Message, Permanent: true}
		}
	}
	return err
}

func (c whatsappChannel) SendText(ctx context.Context, ws, numberID uuid.UUID, to, body, replyTo string) (string, error) {
	id, err := c.wa.SendText(ctx, ws, numberID, to, body, replyTo)
	return id, sendFailure(err)
}

func (c whatsappChannel) SendMedia(ctx context.Context, ws, numberID uuid.UUID, to string, o inboxmodel.OutboundMedia, replyTo string) (string, error) {
	id, err := c.wa.SendMedia(ctx, ws, numberID, to, whatsappmodel.OutboundMedia{
		Kind: o.Kind, MediaID: o.MediaID, Caption: o.Caption, Filename: o.Filename,
	}, replyTo)
	return id, sendFailure(err)
}

func (c whatsappChannel) SendTemplate(ctx context.Context, ws, numberID uuid.UUID, to string, t inboxmodel.OutboundTemplate) (string, error) {
	id, err := c.wa.SendTemplateMessage(ctx, ws, numberID, to, whatsappmodel.OutboundTemplate{
		Name: t.Name, Language: t.Language, HeaderText: t.HeaderText, HeaderKind: t.HeaderKind,
		HeaderMediaID: t.HeaderMediaID, BodyParams: t.BodyParams, URLButtonParams: t.URLButtonParams,
	})
	return id, sendFailure(err)
}

func (c whatsappChannel) SendReaction(ctx context.Context, ws, numberID uuid.UUID, to, target, emoji string) (string, error) {
	id, err := c.wa.SendReaction(ctx, ws, numberID, to, target, emoji)
	return id, sendFailure(err)
}

func (c whatsappChannel) MarkRead(ctx context.Context, ws, numberID uuid.UUID, wamid string) error {
	return sendFailure(c.wa.MarkRead(ctx, ws, numberID, wamid))
}

func (c whatsappChannel) UploadMedia(ctx context.Context, ws, numberID uuid.UUID, contentType, filename string, data io.Reader, size int64) (string, error) {
	id, err := c.wa.UploadMedia(ctx, ws, numberID, contentType, filename, data, size)
	return id, sendFailure(err)
}

func (c whatsappChannel) FetchMedia(ctx context.Context, ws, numberID uuid.UUID, mediaID string) (io.ReadCloser, string, int64, error) {
	return c.wa.FetchMedia(ctx, ws, numberID, mediaID)
}

// ---------- inbox → contacts ----------

type inboxContacts struct{ contacts *contactsservice.Service }

var _ inboxservice.ContactDirectory = inboxContacts{}

func (c inboxContacts) FindOrCreateByIdentity(ctx context.Context, ws uuid.UUID, channelType, externalID, name, phone string) (inboxmodel.Contact, bool, error) {
	got, created, err := c.contacts.FindOrCreateByIdentity(ctx, ws, channelType, externalID, name, phone)
	if err != nil {
		return inboxmodel.Contact{}, false, err
	}
	return inboxmodel.Contact{ID: got.ID, Name: got.Name, Phone: got.Phone, OptIn: got.OptIn}, created, nil
}

func (c inboxContacts) SetLastMessage(ctx context.Context, tx pgx.Tx, ws, contactID uuid.UUID, at time.Time) error {
	return c.contacts.SetLastMessage(ctx, tx, ws, contactID, at)
}

// ---------- inbox → templates ----------

// inboxTemplates gives the inbox the workspace's approved templates.
type inboxTemplates struct{ templates *templatesservice.Service }

var _ inboxservice.TemplateCatalog = inboxTemplates{}

// maxApprovedTemplates bounds the catalog: Meta allows 250 templates per account.
const maxApprovedTemplates = 500

func (t inboxTemplates) ApprovedTemplates(ctx context.Context, ws uuid.UUID) ([]inboxmodel.Template, error) {
	status := templatesmodel.StatusApproved
	var out []inboxmodel.Template
	var before *uuid.UUID
	for len(out) < maxApprovedTemplates {
		page, next, _, err := t.templates.List(ctx, ws, templatesmodel.ListFilter{
			Status: &status, Limit: templatesservice.MaxPageSize, BeforeID: before,
		})
		if err != nil {
			return nil, err
		}
		for _, tpl := range page {
			out = append(out, toInboxTemplate(tpl))
		}
		if next == nil {
			break
		}
		before = next
	}
	if out == nil {
		out = []inboxmodel.Template{}
	}
	return out, nil
}

func (t inboxTemplates) ApprovedTemplate(ctx context.Context, ws, id uuid.UUID) (inboxmodel.Template, error) {
	tpl, err := t.templates.Get(ctx, ws, id)
	if apperr.IsKind(err, apperr.KindNotFound) || (err == nil && tpl.Status != templatesmodel.StatusApproved) {
		return inboxmodel.Template{}, inboxmodel.ErrTemplateNotApproved
	}
	if err != nil {
		return inboxmodel.Template{}, err
	}
	return toInboxTemplate(tpl), nil
}

func toInboxTemplate(t templatesmodel.Template) inboxmodel.Template {
	c := t.Components
	out := inboxmodel.Template{
		ID: t.ID, Name: t.Name, Language: t.Language, Category: t.Category,
		HeaderType: c.Header.Type, HeaderText: c.Header.Text, Body: c.Body.Text, Footer: c.Footer.Text,
		Buttons: make([]inboxmodel.TemplateButton, len(c.Buttons)),
	}
	for i, b := range c.Buttons {
		out.Buttons[i] = inboxmodel.TemplateButton{Type: b.Type, Text: b.Text, Value: b.Value}
	}
	return out
}
