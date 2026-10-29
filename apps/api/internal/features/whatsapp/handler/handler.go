// Package handler serves whatsapp's HTTP endpoints: it decodes requests, calls the service and
// encodes responses. Routes are registered in routes.go.
package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/service"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *service.Service
}

// NewHandler builds the WhatsApp HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) signupConfig(context.Context, *struct{}) (*dto.SignupConfigOutput, error) {
	c := h.svc.SignupConfig()
	out := &dto.SignupConfigOutput{}
	out.Body.Enabled, out.Body.AppID, out.Body.ConfigID, out.Body.GraphVersion = c.Enabled, c.AppID, c.ConfigID, c.GraphVersion
	return out, nil
}

func (h *Handler) connect(ctx context.Context, in *dto.ConnectInput) (*dto.NumberOutput, error) {
	userID, _ := authctx.UserID(ctx)
	n, err := h.svc.Connect(ctx, in.WorkspaceID, userID, model.ConnectInput{
		Code: in.Body.Code, WABAID: in.Body.WABAID, PhoneNumberID: in.Body.PhoneNumberID, NumberType: in.Body.NumberType,
	})
	if err != nil {
		return nil, err
	}
	return &dto.NumberOutput{Body: dto.ToNumberDTO(n)}, nil
}

func (h *Handler) numbers(ctx context.Context, in *dto.WorkspaceInput) (*dto.NumbersOutput, error) {
	list, err := h.svc.Numbers(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.NumbersOutput{}
	out.Body.Numbers = make([]dto.NumberDTO, 0, len(list))
	for _, n := range list {
		out.Body.Numbers = append(out.Body.Numbers, dto.ToNumberDTO(n))
	}
	return out, nil
}

func (h *Handler) number(ctx context.Context, in *dto.NumberInput) (*dto.NumberOutput, error) {
	n, err := h.svc.Number(ctx, in.WorkspaceID, in.NumberID)
	if err != nil {
		return nil, err
	}
	return &dto.NumberOutput{Body: dto.ToNumberDTO(n)}, nil
}

func (h *Handler) sendTest(ctx context.Context, in *dto.SendTestInput) (*dto.TestMessageOutput, error) {
	m, err := h.svc.SendTestMessage(ctx, in.WorkspaceID, in.NumberID, in.Body.To)
	if err != nil {
		return nil, err
	}
	return dto.ToTestMessageOutput(m), nil
}

func (h *Handler) testMessage(ctx context.Context, in *dto.TestMessageInput) (*dto.TestMessageOutput, error) {
	m, err := h.svc.TestMessage(ctx, in.WorkspaceID, in.MessageID)
	if err != nil {
		return nil, err
	}
	return dto.ToTestMessageOutput(m), nil
}

func (h *Handler) verifyWebhook(_ context.Context, in *dto.WebhookVerifyInput) (*dto.WebhookVerifyOutput, error) {
	challenge, err := h.svc.VerifyWebhook(in.Mode, in.Token, in.Challenge)
	if err != nil {
		return nil, err
	}
	return &dto.WebhookVerifyOutput{ContentType: "text/plain", Body: []byte(challenge)}, nil
}

func (h *Handler) receiveWebhook(ctx context.Context, in *dto.WebhookInput) (*struct{}, error) {
	return nil, h.svc.ReceiveWebhook(ctx, in.RawBody, in.Signature)
}
