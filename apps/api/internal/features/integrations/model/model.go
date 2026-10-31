// Package model holds integrations' domain types, constants and errors.
package model

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
)

// MaxAPIKeys is how many active keys one workspace may hold.
const MaxAPIKeys = 20

// No-code apps an API key can be made for. The app's card shows connected while one is active.
const (
	AppZapier = "zapier"
	AppMake   = "make"
	AppN8N    = "n8n"
)

// KeyApps lists the apps an API key can be tagged with.
var KeyApps = []string{AppZapier, AppMake, AppN8N}

// APIKey is a key as listed: never the key itself.
type APIKey struct {
	ID            uuid.UUID
	Name          string
	App           *string
	Prefix        string
	Last4         string
	CreatedBy     uuid.UUID
	CreatedByName string
	LastUsedAt    *time.Time
	CreatedAt     time.Time
}

// NewAPIKey is a key just made, with the one time its full value is shown.
type NewAPIKey struct {
	APIKey
	Key string
}

// Providers a workspace connects with stored credentials.
const (
	ProviderShopify      = "shopify"
	ProviderWooCommerce  = "woocommerce"
	ProviderGoogleSheets = "google_sheets"
)

// Connection statuses.
const (
	StatusConnected    = "connected"
	StatusError        = "error"
	StatusDisconnected = "disconnected"
)

// Connection is a store or app connected to the workspace. Credentials are never on it.
type Connection struct {
	Provider      string
	Status        string
	AccountLabel  string
	LastCheckedAt *time.Time
	LastError     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ConnectInput is what a person enters to connect a store. Which fields count depends on the provider.
type ConnectInput struct {
	ShopDomain     string
	AccessToken    string
	StoreURL       string
	ConsumerKey    string
	ConsumerSecret string
}

// Webhook event types.
const (
	EventPing                 = "ping"
	EventMessageReceived      = "message.received"
	EventMessageStatus        = "message.status"
	EventContactCreated       = "contact.created"
	EventContactUpdated       = "contact.updated"
	EventBroadcastCompleted   = "broadcast.completed"
	EventTemplateStatusChange = "template.status_changed"
)

// EventDef describes one event an endpoint can subscribe to.
type EventDef struct {
	Type        string
	Description string
}

// Events is the catalog endpoints subscribe from, in display order. ping is sent only as a test.
var Events = []EventDef{
	{EventMessageReceived, "A customer sent a message"},
	{EventMessageStatus, "A message you sent was delivered, read or failed"},
	{EventContactCreated, "A contact was added"},
	{EventContactUpdated, "A contact's details, tags or opt-in changed"},
	{EventBroadcastCompleted, "A broadcast finished sending"},
	{EventTemplateStatusChange, "Meta approved, rejected or paused a template"},
}

// IsEvent reports whether t is in the catalog.
func IsEvent(t string) bool {
	return slices.ContainsFunc(Events, func(e EventDef) bool { return e.Type == t })
}

// Webhook limits.
const (
	MaxEndpoints = 10
	// MaxConsecutiveFailures turns an endpoint off once this many deliveries in a row fail for good.
	MaxConsecutiveFailures = 20
	// DeliveryRetention is how long the delivery log is kept.
	DeliveryRetention = 30 * 24 * time.Hour
)

// Delivery statuses.
const (
	DeliveryPending   = "pending"
	DeliverySucceeded = "succeeded"
	DeliveryFailed    = "failed"
)

// Endpoint is a webhook endpoint with its recent delivery health. The secret is never on it.
type Endpoint struct {
	ID                  uuid.UUID
	URL                 string
	Description         string
	Events              []string
	Enabled             bool
	ConsecutiveFailures int32
	DisabledReason      *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	LastStatus          *string
	LastResponseStatus  *int32
	LastDeliveryAt      *time.Time
	WeekTotal           int64
	WeekSucceeded       int64
}

// EndpointInput is what a person sets on an endpoint.
type EndpointInput struct {
	URL         string
	Description string
	Events      []string
	Enabled     bool
}

// Delivery is one event sent to one endpoint, with its latest attempt.
type Delivery struct {
	ID             uuid.UUID
	EventID        uuid.UUID
	EventType      string
	Payload        json.RawMessage
	Status         string
	Attempts       int32
	ResponseStatus *int32
	ResponseMs     *int32
	Error          *string
	CreatedAt      time.Time
	DeliveredAt    *time.Time
}

// Event is the JSON body POSTed to endpoints.
type Event struct {
	ID          uuid.UUID       `json:"id"`
	Type        string          `json:"type"`
	CreatedAt   time.Time       `json:"created_at"`
	WorkspaceID uuid.UUID       `json:"workspace_id"`
	Data        json.RawMessage `json:"data"`
}

// Overview is what the Integrations page shows on each card.
type Overview struct {
	Connections     []Connection
	GoogleAvailable bool
	// ShopifyOAuth and WooOAuth: the store can connect in one click rather than with credentials.
	ShopifyOAuth   bool
	WooOAuth       bool
	ActiveKeys     int
	KeysLastUsedAt *time.Time
	AppKeys        map[string]AppKeyStatus
	Endpoints      []Endpoint
}

// AppKeyStatus is a no-code app's keys: how many are active and when one was last used.
type AppKeyStatus struct {
	ActiveKeys int
	LastUsedAt *time.Time
}

// What happened to an order.
const (
	OrderCreated = "created"
	OrderPaid    = "paid"
)

// CommerceDelivery is one webhook as a store sent it.
type CommerceDelivery struct {
	Topic, DeliveryID, Signature string
	Body                         []byte
}

// OrderEvent is a store order in one shape for every provider.
type OrderEvent struct {
	WorkspaceID uuid.UUID
	Provider    string
	Status      string
	OrderID     string
	Number      string
	Total       string
	FirstName   string
	LastName    string
	Phone       string
	StatusURL   string
	// CheckoutToken links the order to the checkout it came from.
	CheckoutToken string
	AmountMinor   int64
	Currency      string
}

// CheckoutEvent is a shopper's checkout, as it changes.
type CheckoutEvent struct {
	WorkspaceID uuid.UUID
	Token       string
	FirstName   string
	LastName    string
	Phone       string
	Total       string
	CheckoutURL string
	// Consent means the shopper accepted marketing.
	Consent bool
}
