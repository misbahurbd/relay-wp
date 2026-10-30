package service

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
)

// InboundSink receives customer messages and delivery statuses for the inbox
// (docs/PHASE-7-INBOX.md AD-1). Statuses for test messages are handled here
// first; every other status goes to the sink.
type InboundSink interface {
	InboundMessage(ctx context.Context, m model.InboundMessage) error
	MessageStatus(ctx context.Context, s model.StatusUpdate) error
}

// SetInboundSink sets who receives inbound messages and statuses.
func (s *Service) SetInboundSink(sink InboundSink) { s.inbound = sink }

// Errors from the send methods below:
//   - *model.SendError: Meta refused the call. Permanent means don't retry (for
//     example 131047, the 24-hour window is closed). A revoked token (190/10) is
//     a permanent SendError and disconnects the account.
//   - apperr errors: the number isn't the workspace's (model.ErrNumberNotFound),
//     isn't connected (model.ErrNumberNotConnected), or its account was
//     disconnected (model.ErrDisconnected), or WhatsApp is off (model.ErrDisabled).
//     None of these succeed on a retry.
//   - anything else: Meta's servers or the network failed; retry.

// outbound is a connected number ready to call Graph with.
type outbound struct {
	workspaceID   uuid.UUID
	accountID     uuid.UUID
	phoneNumberID string
	token         string
}

// outboundNumber resolves one of the workspace's connected numbers and its token.
func (s *Service) outboundNumber(ctx context.Context, workspaceID, numberID uuid.UUID) (outbound, error) {
	if !s.cfg.Enabled {
		return outbound{}, model.ErrDisabled
	}
	n, err := s.Number(ctx, workspaceID, numberID)
	if err != nil {
		return outbound{}, err
	}
	if n.Status != model.StatusConnected {
		return outbound{}, model.ErrNumberNotConnected
	}
	_, token, err := s.token(ctx, workspaceID, n.AccountID)
	if err != nil {
		return outbound{}, err
	}
	return outbound{workspaceID: workspaceID, accountID: n.AccountID, phoneNumberID: n.PhoneNumberID, token: token}, nil
}

// fail maps a Graph error for the inbox; a revoked token disconnects the account.
func (s *Service) fail(ctx context.Context, o outbound, err error) error {
	if isAuthFailure(err) {
		if derr := s.disconnect(ctx, o.workspaceID, o.accountID); derr != nil {
			return derr
		}
	}
	return classifySend(err)
}

// send runs one Graph send for a connected number and maps its error.
func (s *Service) send(ctx context.Context, workspaceID, numberID uuid.UUID, do func(o outbound) (string, error)) (string, error) {
	o, err := s.outboundNumber(ctx, workspaceID, numberID)
	if err != nil {
		return "", err
	}
	wamid, err := do(o)
	if err != nil {
		return "", s.fail(ctx, o, err)
	}
	return wamid, nil
}

// SendText sends a free-form text message from the number. replyTo quotes an
// earlier message by its wamid. It returns the new message's wamid. Meta's
// refusals come back as *model.SendError.
func (s *Service) SendText(ctx context.Context, workspaceID, numberID uuid.UUID, to, body, replyTo string) (string, error) {
	return s.send(ctx, workspaceID, numberID, func(o outbound) (string, error) {
		return s.graph.SendText(ctx, o.token, o.phoneNumberID, to, body, replyTo)
	})
}

// SendMedia sends a file already uploaded with UploadMedia.
func (s *Service) SendMedia(ctx context.Context, workspaceID, numberID uuid.UUID, to string, m model.OutboundMedia, replyTo string) (string, error) {
	return s.send(ctx, workspaceID, numberID, func(o outbound) (string, error) {
		return s.graph.SendMedia(ctx, o.token, o.phoneNumberID, to, m, replyTo)
	})
}

// SendTemplateMessage sends an approved template with its parameters.
func (s *Service) SendTemplateMessage(ctx context.Context, workspaceID, numberID uuid.UUID, to string, t model.OutboundTemplate) (string, error) {
	return s.send(ctx, workspaceID, numberID, func(o outbound) (string, error) {
		return s.graph.SendTemplateMessage(ctx, o.token, o.phoneNumberID, to, t)
	})
}

// SendReaction reacts to a message by its wamid; an empty emoji removes the reaction.
func (s *Service) SendReaction(ctx context.Context, workspaceID, numberID uuid.UUID, to, targetWAMID, emoji string) (string, error) {
	return s.send(ctx, workspaceID, numberID, func(o outbound) (string, error) {
		return s.graph.SendReaction(ctx, o.token, o.phoneNumberID, to, targetWAMID, emoji)
	})
}

// MarkRead marks a customer's message (and the ones before it) read: blue ticks.
func (s *Service) MarkRead(ctx context.Context, workspaceID, numberID uuid.UUID, wamid string) error {
	_, err := s.send(ctx, workspaceID, numberID, func(o outbound) (string, error) {
		return "", s.graph.MarkRead(ctx, o.token, o.phoneNumberID, wamid)
	})
	return err
}

// UploadMedia uploads a file to Meta for this number and returns its media ID.
// The caller checks the type and size limits (AD-11); size < 0 means unknown.
func (s *Service) UploadMedia(ctx context.Context, workspaceID, numberID uuid.UUID, contentType, filename string, data io.Reader, size int64) (string, error) {
	return s.send(ctx, workspaceID, numberID, func(o outbound) (string, error) {
		return s.graph.UploadMedia(ctx, o.token, o.phoneNumberID, contentType, filename, data, size)
	})
}

// FetchMedia downloads a customer's media file by Meta media ID. The caller
// closes the body. Files over MaxMediaBytes fail, before the download when Meta
// reports the size, otherwise with a read error from body.
func (s *Service) FetchMedia(ctx context.Context, workspaceID, numberID uuid.UUID, mediaID string) (body io.ReadCloser, contentType string, size int64, err error) {
	o, err := s.outboundNumber(ctx, workspaceID, numberID)
	if err != nil {
		return nil, "", 0, err
	}
	body, contentType, size, err = s.graph.FetchMedia(ctx, o.token, mediaID)
	if err != nil {
		return nil, "", 0, s.fail(ctx, o, err)
	}
	return body, contentType, size, nil
}
