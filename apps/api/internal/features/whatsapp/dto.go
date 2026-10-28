package whatsapp

import (
	"time"

	"github.com/google/uuid"
)

type signupConfigOutput struct {
	Body struct {
		Enabled      bool   `json:"enabled" doc:"False: this server can't connect WhatsApp; setup can only skip the step"`
		AppID        string `json:"app_id"`
		ConfigID     string `json:"config_id" doc:"Embedded Signup configuration ID"`
		GraphVersion string `json:"graph_version" example:"v24.0"`
	}
}

type setupErrorDTO struct {
	Code     string `json:"code"`
	MetaCode int    `json:"meta_code,omitempty"`
	Message  string `json:"message"`
}

type setupTasksDTO struct {
	Register          string `json:"register" enum:"waiting,running,done,failed"`
	SubscribeWebhooks string `json:"subscribe_webhooks" enum:"waiting,running,done,failed"`
	SyncTemplates     string `json:"sync_templates" enum:"waiting,running,done,failed"`
	ImportProfile     string `json:"import_profile" enum:"waiting,running,done,failed"`
}

type numberDTO struct {
	ID                 uuid.UUID      `json:"id"`
	DisplayPhoneNumber string         `json:"display_phone_number" example:"+1 415-555-0142"`
	VerifiedName       string         `json:"verified_name"`
	NumberType         string         `json:"number_type" enum:"new,coexistence,migration"`
	NameStatus         string         `json:"name_status" enum:"pending_review,approved,declined"`
	QualityRating      *string        `json:"quality_rating" nullable:"true" enum:"green,yellow,red"`
	MessagingLimit     *string        `json:"messaging_limit" example:"TIER_250"`
	Status             string         `json:"status" enum:"setting_up,connected,failed,disconnected"`
	SetupTasks         setupTasksDTO  `json:"setup_tasks"`
	LastError          *setupErrorDTO `json:"last_error,omitempty" doc:"Why setup failed, when status is failed"`
	WALink             string         `json:"wa_link" example:"wa.me/14155550142"`
	CreatedAt          time.Time      `json:"created_at"`
}

func toNumberDTO(n Number) numberDTO {
	out := numberDTO{
		ID: n.ID, DisplayPhoneNumber: n.DisplayPhoneNumber, VerifiedName: n.VerifiedName, NumberType: n.NumberType,
		NameStatus: n.NameStatus, QualityRating: n.QualityRating, MessagingLimit: n.MessagingLimit, Status: n.Status,
		SetupTasks: setupTasksDTO{
			Register: n.SetupTasks[taskRegister], SubscribeWebhooks: n.SetupTasks[taskSubscribe],
			SyncTemplates: n.SetupTasks[taskTemplates], ImportProfile: n.SetupTasks[taskProfile],
		},
		WALink: "wa.me/" + digits(n.DisplayPhoneNumber), CreatedAt: n.CreatedAt,
	}
	if n.LastError != nil {
		e := setupErrorDTO(*n.LastError)
		out.LastError = &e
	}
	return out
}

type numberOutput struct {
	Body numberDTO
}

type numbersOutput struct {
	Body struct {
		Numbers []numberDTO `json:"numbers"`
	}
}

type connectInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	Body        struct {
		Code          string `json:"code" minLength:"1" maxLength:"2048" doc:"From the FB.login callback"`
		WABAID        string `json:"waba_id" maxLength:"32" doc:"From the WA_EMBEDDED_SIGNUP message"`
		PhoneNumberID string `json:"phone_number_id" maxLength:"32" doc:"From the WA_EMBEDDED_SIGNUP message"`
		NumberType    string `json:"number_type" enum:"new,coexistence,migration"`
	}
}

type workspaceInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
}

type numberInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	NumberID    uuid.UUID `path:"numberId"`
}

type sendTestInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	NumberID    uuid.UUID `path:"numberId"`
	Body        struct {
		To string `json:"to" minLength:"8" maxLength:"24" doc:"E.164, e.g. +8801712345678"`
	}
}

type testMessageInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	MessageID   uuid.UUID `path:"messageId"`
}

type testMessageOutput struct {
	Body struct {
		ID        uuid.UUID `json:"id"`
		NumberID  uuid.UUID `json:"number_id"`
		ToPhone   string    `json:"to_phone"`
		Status    string    `json:"status" enum:"queued,sent,delivered,read,failed"`
		ErrorCode *string   `json:"error_code"`
		CreatedAt time.Time `json:"created_at"`
	}
}

func toTestMessageOutput(m TestMessage) *testMessageOutput {
	out := &testMessageOutput{}
	out.Body.ID, out.Body.NumberID, out.Body.ToPhone = m.ID, m.NumberID, m.ToPhone
	out.Body.Status, out.Body.ErrorCode, out.Body.CreatedAt = m.Status, m.ErrorCode, m.CreatedAt
	return out
}

type webhookVerifyInput struct {
	Mode      string `query:"hub.mode"`
	Token     string `query:"hub.verify_token"`
	Challenge string `query:"hub.challenge" maxLength:"256"`
}

type webhookVerifyOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

type webhookInput struct {
	Signature string `header:"X-Hub-Signature-256"`
	RawBody   []byte
}
