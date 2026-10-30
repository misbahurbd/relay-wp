package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
)

type WebhookEventDTO struct {
	Type        string `json:"type" example:"message.received"`
	Description string `json:"description"`
}

type WebhookEventsOutput struct {
	Body struct {
		Events []WebhookEventDTO `json:"events"`
	}
}

type EndpointDTO struct {
	ID                  uuid.UUID  `json:"id"`
	URL                 string     `json:"url" example:"https://api.example.com/hooks/relay"`
	Description         string     `json:"description"`
	Events              []string   `json:"events"`
	Enabled             bool       `json:"enabled"`
	ConsecutiveFailures int32      `json:"consecutive_failures"`
	DisabledReason      *string    `json:"disabled_reason" nullable:"true" doc:"Why Relay turned it off"`
	LastStatus          *string    `json:"last_status" nullable:"true" doc:"succeeded, failed or pending"`
	LastResponseStatus  *int32     `json:"last_response_status" nullable:"true"`
	LastDeliveryAt      *time.Time `json:"last_delivery_at" nullable:"true"`
	WeekTotal           int64      `json:"week_total" doc:"Finished deliveries in the last 7 days"`
	WeekSucceeded       int64      `json:"week_succeeded"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func ToEndpointDTO(e model.Endpoint) EndpointDTO {
	return EndpointDTO{
		ID: e.ID, URL: e.URL, Description: e.Description, Events: e.Events, Enabled: e.Enabled,
		ConsecutiveFailures: e.ConsecutiveFailures, DisabledReason: e.DisabledReason, LastStatus: e.LastStatus,
		LastResponseStatus: e.LastResponseStatus, LastDeliveryAt: e.LastDeliveryAt, WeekTotal: e.WeekTotal,
		WeekSucceeded: e.WeekSucceeded, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

type EndpointsOutput struct {
	Body struct {
		Endpoints []EndpointDTO `json:"endpoints"`
	}
}

type EndpointOutput struct {
	Body EndpointDTO
}

type endpointBody struct {
	URL         string   `json:"url" minLength:"1" maxLength:"2048" format:"uri"`
	Description string   `json:"description,omitempty" maxLength:"200"`
	Events      []string `json:"events" minItems:"1" maxItems:"20"`
	Enabled     bool     `json:"enabled"`
}

func (b endpointBody) Model() model.EndpointInput {
	return model.EndpointInput{URL: b.URL, Description: b.Description, Events: b.Events, Enabled: b.Enabled}
}

type CreateEndpointInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        endpointBody
}

type UpdateEndpointInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	EndpointID  uuid.UUID `path:"endpointId"`
	Body        endpointBody
}

type CreatedEndpointOutput struct {
	Body struct {
		Endpoint EndpointDTO `json:"endpoint"`
		Secret   string      `json:"secret" doc:"Signing secret for Relay-Signature"`
	}
}

type EndpointInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	EndpointID  uuid.UUID `path:"endpointId"`
}

type SecretOutput struct {
	Body struct {
		Secret string `json:"secret" example:"whsec_..." doc:"Signing secret for Relay-Signature"`
	}
}

type DeliveryDTO struct {
	ID             uuid.UUID  `json:"id"`
	EventID        uuid.UUID  `json:"event_id"`
	EventType      string     `json:"event_type"`
	Payload        any        `json:"payload" doc:"The JSON body that was sent"`
	Status         string     `json:"status" enum:"pending,succeeded,failed"`
	Attempts       int32      `json:"attempts"`
	ResponseStatus *int32     `json:"response_status" nullable:"true"`
	ResponseMs     *int32     `json:"response_ms" nullable:"true"`
	Error          *string    `json:"error" nullable:"true"`
	CreatedAt      time.Time  `json:"created_at"`
	DeliveredAt    *time.Time `json:"delivered_at" nullable:"true"`
}

func ToDeliveryDTO(d model.Delivery) DeliveryDTO {
	var payload any
	_ = json.Unmarshal(d.Payload, &payload)
	return DeliveryDTO{
		ID: d.ID, EventID: d.EventID, EventType: d.EventType, Payload: payload, Status: d.Status, Attempts: d.Attempts,
		ResponseStatus: d.ResponseStatus, ResponseMs: d.ResponseMs, Error: d.Error, CreatedAt: d.CreatedAt,
		DeliveredAt: d.DeliveredAt,
	}
}

type DeliveriesInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	EndpointID  uuid.UUID `path:"endpointId"`
	Cursor      string    `query:"cursor" doc:"next_cursor from the previous page"`
	Limit       int32     `query:"limit" minimum:"1" maximum:"100" default:"25"`
}

type DeliveriesOutput struct {
	Body struct {
		Deliveries []DeliveryDTO `json:"deliveries"`
		NextCursor *string       `json:"next_cursor"`
	}
}

type DeliveryOutput struct {
	Body DeliveryDTO
}

type RedeliverInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	EndpointID  uuid.UUID `path:"endpointId"`
	DeliveryID  uuid.UUID `path:"deliveryId"`
}
