// Package model holds whatsapp's domain types, constants and errors, shared by every layer of the
// feature.
package model

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

// How the number joins the Cloud API.
const (
	TypeNew         = "new"
	TypeCoexistence = "coexistence" // keeps working in the WhatsApp Business app
	TypeMigration   = "migration"
)

// NumberTypes are the ways a WhatsApp number can be connected.
var NumberTypes = []string{TypeNew, TypeCoexistence, TypeMigration}

// Number statuses.
const (
	StatusSettingUp    = "setting_up"
	StatusConnected    = "connected"
	StatusFailed       = "failed"
	StatusDisconnected = "disconnected"
)

// Setup tasks run by the whatsapp_setup job, in order, and their states.
const (
	TaskRegister  = "register"
	TaskSubscribe = "subscribe_webhooks"
	TaskTemplates = "sync_templates"
	TaskProfile   = "import_profile"

	TaskWaiting = "waiting"
	TaskRunning = "running"
	TaskDone    = "done"
	TaskFailed  = "failed"
)

// SetupTasks are the background tasks that finish setting up a number, in order.
var SetupTasks = []string{TaskRegister, TaskSubscribe, TaskTemplates, TaskProfile}

// The template every new WhatsApp Business account has, used for the setup test message.
const (
	TestTemplate     = "hello_world"
	TestTemplateLang = "en_US"
	TestMessageLimit = 5 // per workspace per hour
)

// Number is a WhatsApp phone number connected to a workspace.
type Number struct {
	ID                 uuid.UUID
	AccountID          uuid.UUID
	PhoneNumberID      string
	DisplayPhoneNumber string
	NumberType         string
	VerifiedName       string
	NameStatus         string
	QualityRating      *string
	MessagingLimit     *string
	Status             string
	SetupTasks         map[string]string
	LastError          *SetupError
	CreatedAt          time.Time
}

// SetupError is why a setup task gave up. Never contains tokens.
type SetupError struct {
	Code     string `json:"code"`
	MetaCode int    `json:"meta_code,omitempty"`
	Message  string `json:"message"`
}

// ConnectInput is what Embedded Signup hands the frontend.
type ConnectInput struct {
	Code          string
	WABAID        string
	PhoneNumberID string
	NumberType    string
}

// TestMessage is a message sent during setup to check the number works.
type TestMessage struct {
	ID        uuid.UUID
	NumberID  uuid.UUID
	ToPhone   string
	Status    string
	ErrorCode *string
	CreatedAt time.Time
}

// SignupConfig is what the frontend needs to open Meta's Embedded Signup.
type SignupConfig struct {
	Enabled      bool
	AppID        string
	ConfigID     string
	GraphVersion string
}

// Digits keeps only the digits of a phone number.
func Digits(s string) string { return nonDigits.ReplaceAllString(s, "") }

// --- Test messages ---

var nonDigits = regexp.MustCompile(`[^0-9]`)
