package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/repository"
)

// messagesValue is the value of a "messages" webhook change.
type messagesValue struct {
	Metadata struct {
		PhoneNumberID string `json:"phone_number_id"`
	} `json:"metadata"`
	Contacts []struct {
		WaID    string `json:"wa_id"`
		Profile struct {
			Name string `json:"name"`
		} `json:"profile"`
	} `json:"contacts"`
	Messages []json.RawMessage `json:"messages"`
	Statuses []metaStatus      `json:"statuses"`
}

type metaStatus struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
	RecipientID string `json:"recipient_id"`
	Errors      []struct {
		Code  int    `json:"code"`
		Title string `json:"title"`
	} `json:"errors"`
}

type metaMedia struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	Caption  string `json:"caption"`
	Filename string `json:"filename"`
}

type metaReply struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// metaMessage is one entry of messages[] (Cloud API webhook reference).
type metaMessage struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      *struct {
		Body string `json:"body"`
	} `json:"text"`
	Image    *metaMedia `json:"image"`
	Video    *metaMedia `json:"video"`
	Audio    *metaMedia `json:"audio"`
	Document *metaMedia `json:"document"`
	Sticker  *metaMedia `json:"sticker"`
	Location *struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Name      string  `json:"name"`
		Address   string  `json:"address"`
	} `json:"location"`
	Contacts    json.RawMessage `json:"contacts"`
	Interactive *struct {
		Type        string     `json:"type"`
		ButtonReply *metaReply `json:"button_reply"`
		ListReply   *metaReply `json:"list_reply"`
	} `json:"interactive"`
	Button *struct {
		Text    string `json:"text"`
		Payload string `json:"payload"`
	} `json:"button"`
	Reaction *struct {
		MessageID string `json:"message_id"`
		Emoji     string `json:"emoji"`
	} `json:"reaction"`
	Context *struct {
		ID string `json:"id"`
	} `json:"context"`
	Referral *struct {
		SourceURL  string `json:"source_url"`
		SourceID   string `json:"source_id"`
		SourceType string `json:"source_type"`
		Headline   string `json:"headline"`
		Body       string `json:"body"`
	} `json:"referral"`
}

// messageLookups are the database steps of a messages webhook, apart so the
// parsing and the sink hand-off can be tested without a database.
type messageLookups struct {
	// number returns whatsapp_numbers.id for Meta's phone_number_id in the account.
	number func(ctx context.Context, acc account, phoneNumberID string) (uuid.UUID, bool, error)
	// testStatus applies a status to a test message; false if wamid isn't one
	// (or the status is older than the one stored).
	testStatus func(ctx context.Context, acc account, st metaStatus) (bool, error)
}

// onMessages hands customer messages and delivery statuses to the inbox.
// Statuses of test messages are handled here and go no further.
func (s *Service) onMessages(ctx context.Context, acc account, value json.RawMessage) error {
	return s.handleMessages(ctx, acc, value, messageLookups{number: s.numberByPhoneID, testStatus: s.testMessageStatus})
}

func (s *Service) handleMessages(ctx context.Context, acc account, value json.RawMessage, db messageLookups) error {
	var v messagesValue
	if err := json.Unmarshal(value, &v); err != nil {
		return fmt.Errorf("decode messages: %w", err)
	}
	numberID, found, err := db.number(ctx, acc, v.Metadata.PhoneNumberID)
	if err != nil {
		return err
	}
	if !found && s.log != nil {
		s.log.WarnContext(ctx, "whatsapp webhook for an unknown number", "workspace_id", acc.WorkspaceID,
			"phone_number_id", v.Metadata.PhoneNumberID, "messages", len(v.Messages), "statuses", len(v.Statuses))
	}

	if found && s.inbound != nil {
		for _, raw := range v.Messages {
			m, err := parseInbound(raw, v)
			if err != nil {
				return err
			}
			m.WorkspaceID, m.NumberID, m.PhoneNumberID = acc.WorkspaceID, numberID, v.Metadata.PhoneNumberID
			if err := s.inbound.InboundMessage(ctx, m); err != nil {
				return fmt.Errorf("inbound message: %w", err)
			}
		}
	}

	for _, st := range v.Statuses {
		switch st.Status {
		case "sent", "delivered", "read", "failed":
		default:
			continue // e.g. "deleted" or "warning": nothing to show
		}
		handled, err := db.testStatus(ctx, acc, st)
		if err != nil {
			return err
		}
		if handled || !found || s.inbound == nil {
			continue
		}
		u := model.StatusUpdate{
			WorkspaceID: acc.WorkspaceID, NumberID: numberID, WAMID: st.ID, Status: st.Status,
			Timestamp: unixTime(st.Timestamp), RecipientID: st.RecipientID,
		}
		if st.Status == "failed" && len(st.Errors) > 0 {
			u.ErrorCode, u.ErrorTitle = st.Errors[0].Code, st.Errors[0].Title
		}
		if err := s.inbound.MessageStatus(ctx, u); err != nil {
			return fmt.Errorf("message status: %w", err)
		}
	}
	return nil
}

// parseInbound turns one messages[] entry into the inbox's shape, without the
// workspace and number.
func parseInbound(raw json.RawMessage, v messagesValue) (model.InboundMessage, error) {
	var mm metaMessage
	if err := json.Unmarshal(raw, &mm); err != nil {
		return model.InboundMessage{}, fmt.Errorf("decode message: %w", err)
	}
	m := model.InboundMessage{
		WAMID:     mm.ID,
		From:      "+" + model.Digits(mm.From),
		Timestamp: unixTime(mm.Timestamp),
		Type:      mm.Type,
		Raw:       raw,
	}
	for _, c := range v.Contacts {
		if c.WaID == mm.From {
			m.ProfileName = c.Profile.Name
		}
	}
	if m.ProfileName == "" && len(v.Contacts) == 1 {
		// wa_id can differ from "from" (e.g. Brazilian numbers gaining a 9).
		m.ProfileName = v.Contacts[0].Profile.Name
	}
	if mm.Context != nil {
		m.ContextWAMID = mm.Context.ID
	}
	if r := mm.Referral; r != nil {
		m.Referral = &model.Referral{SourceType: r.SourceType, SourceID: r.SourceID, SourceURL: r.SourceURL, Headline: r.Headline, Body: r.Body}
	}

	switch mm.Type {
	case "text":
		if mm.Text != nil {
			m.Text = mm.Text.Body
		}
	case "image", "video", "audio", "document", "sticker":
		if md := mm.media(); md != nil {
			m.Text = md.Caption
			m.Media = &model.InboundMedia{ID: md.ID, MimeType: md.MimeType, SHA256: md.SHA256, Filename: md.Filename}
		}
	case "location":
		if l := mm.Location; l != nil {
			m.Location = &model.Location{Latitude: l.Latitude, Longitude: l.Longitude, Name: l.Name, Address: l.Address}
		}
	case "contacts":
		m.Contacts = mm.Contacts
	case "interactive":
		if in := mm.Interactive; in != nil {
			reply := in.ButtonReply
			if in.Type == "list_reply" {
				reply = in.ListReply
			}
			if reply != nil {
				m.Interactive = &model.InteractiveReply{Kind: in.Type, ID: reply.ID, Title: reply.Title, Description: reply.Description}
				m.Text = reply.Title
			}
		}
	case "button":
		if b := mm.Button; b != nil {
			m.Interactive = &model.InteractiveReply{Kind: "button", ID: b.Payload, Title: b.Text}
			m.Text = b.Text
		}
	case "reaction":
		if r := mm.Reaction; r != nil {
			m.Reaction = &model.Reaction{WAMID: r.MessageID, Emoji: r.Emoji}
		}
	}
	return m, nil
}

func (mm metaMessage) media() *metaMedia {
	switch mm.Type {
	case "image":
		return mm.Image
	case "video":
		return mm.Video
	case "audio":
		return mm.Audio
	case "document":
		return mm.Document
	case "sticker":
		return mm.Sticker
	}
	return nil
}

// unixTime parses Meta's unix-seconds string; a bad value means now.
func unixTime(s string) time.Time {
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil || sec <= 0 {
		return time.Now().UTC()
	}
	return time.Unix(sec, 0).UTC()
}

func (s *Service) numberByPhoneID(ctx context.Context, acc account, phoneNumberID string) (uuid.UUID, bool, error) {
	if phoneNumberID == "" {
		return uuid.Nil, false, nil
	}
	id, err := repository.NewRepository(s.pool).GetAccountNumberID(ctx, gen.GetAccountNumberIDParams{
		WorkspaceID: acc.WorkspaceID, AccountID: acc.ID, PhoneNumberID: phoneNumberID,
	})
	if repository.IsNoRows(err) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("look up number: %w", err)
	}
	return id, true, nil
}

func (s *Service) testMessageStatus(ctx context.Context, acc account, st metaStatus) (bool, error) {
	var code *string
	if st.Status == "failed" && len(st.Errors) > 0 {
		c := fmt.Sprintf("meta_%d", st.Errors[0].Code)
		code = &c
	}
	row, err := repository.NewRepository(s.pool).UpdateTestMessageStatus(ctx, gen.UpdateTestMessageStatusParams{
		WorkspaceID: acc.WorkspaceID, Wamid: &st.ID, Status: st.Status, ErrorCode: code,
	})
	if repository.IsNoRows(err) {
		return false, nil // not a test message, or an older status
	}
	if err != nil {
		return false, fmt.Errorf("update test message: %w", err)
	}
	if row.Status == "delivered" || row.Status == "read" {
		if err := s.notifier.TestMessageDelivered(ctx, acc.WorkspaceID); err != nil {
			return false, err
		}
	}
	return true, nil
}
