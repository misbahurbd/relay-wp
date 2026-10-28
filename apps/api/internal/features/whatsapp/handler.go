package whatsapp

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *Service
}

// NewHandler builds the WhatsApp HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) signupConfig(context.Context, *struct{}) (*signupConfigOutput, error) {
	c := h.svc.SignupConfig()
	out := &signupConfigOutput{}
	out.Body.Enabled, out.Body.AppID, out.Body.ConfigID, out.Body.GraphVersion = c.Enabled, c.AppID, c.ConfigID, c.GraphVersion
	return out, nil
}

func (h *Handler) connect(ctx context.Context, in *connectInput) (*numberOutput, error) {
	userID, _ := authctx.UserID(ctx)
	n, err := h.svc.Connect(ctx, in.WorkspaceID, userID, ConnectInput{
		Code: in.Body.Code, WABAID: in.Body.WABAID, PhoneNumberID: in.Body.PhoneNumberID, NumberType: in.Body.NumberType,
	})
	if err != nil {
		return nil, err
	}
	return &numberOutput{Body: toNumberDTO(n)}, nil
}

func (h *Handler) numbers(ctx context.Context, in *workspaceInput) (*numbersOutput, error) {
	list, err := h.svc.Numbers(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &numbersOutput{}
	out.Body.Numbers = make([]numberDTO, 0, len(list))
	for _, n := range list {
		out.Body.Numbers = append(out.Body.Numbers, toNumberDTO(n))
	}
	return out, nil
}

func (h *Handler) number(ctx context.Context, in *numberInput) (*numberOutput, error) {
	n, err := h.svc.Number(ctx, in.WorkspaceID, in.NumberID)
	if err != nil {
		return nil, err
	}
	return &numberOutput{Body: toNumberDTO(n)}, nil
}

func (h *Handler) sendTest(ctx context.Context, in *sendTestInput) (*testMessageOutput, error) {
	m, err := h.svc.SendTestMessage(ctx, in.WorkspaceID, in.NumberID, in.Body.To)
	if err != nil {
		return nil, err
	}
	return toTestMessageOutput(m), nil
}

func (h *Handler) testMessage(ctx context.Context, in *testMessageInput) (*testMessageOutput, error) {
	m, err := h.svc.TestMessage(ctx, in.WorkspaceID, in.MessageID)
	if err != nil {
		return nil, err
	}
	return toTestMessageOutput(m), nil
}

func (h *Handler) verifyWebhook(_ context.Context, in *webhookVerifyInput) (*webhookVerifyOutput, error) {
	challenge, err := h.svc.VerifyWebhook(in.Mode, in.Token, in.Challenge)
	if err != nil {
		return nil, err
	}
	return &webhookVerifyOutput{ContentType: "text/plain", Body: []byte(challenge)}, nil
}

func (h *Handler) receiveWebhook(ctx context.Context, in *webhookInput) (*struct{}, error) {
	return nil, h.svc.ReceiveWebhook(ctx, in.RawBody, in.Signature)
}
