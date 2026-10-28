package whatsapp

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the whatsapp service.
var (
	ErrDisabled         = apperr.NotFound("whatsapp_disabled", "WhatsApp isn't available on this server yet.")
	ErrInvalidInput     = apperr.Validation("invalid_input", "Some fields are invalid")
	ErrCodeExchange     = apperr.Validation("whatsapp_code_exchange_failed", "Meta didn't confirm the connection. Try connecting again.")
	ErrMetaUnavailable  = apperr.Unavailable("whatsapp_unavailable", "Meta isn't answering right now. Try again in a minute.")
	ErrAlreadyConnected = apperr.Conflict("whatsapp_already_connected", "This WhatsApp Business account is connected to another workspace.")
	ErrNumberInUse      = apperr.Conflict("whatsapp_number_in_use", "This number is connected to another workspace.")
	ErrNumberNotFound   = apperr.NotFound("whatsapp_number_not_found", "WhatsApp number not found")
	ErrMessageNotFound  = apperr.NotFound("test_message_not_found", "Message not found")
	ErrNotConnected     = apperr.Conflict("whatsapp_not_connected", "Your number is still being set up.")
	ErrDisconnected     = apperr.Conflict("whatsapp_disconnected", "WhatsApp is disconnected. Connect your number again.")
	ErrSameNumber       = apperr.Validation("same_number", "Use a different number from the one you just connected.")
	ErrTooManyTests     = apperr.RateLimited("too_many_test_messages", "You've sent 5 test messages this hour. Try again later.")
	ErrWebhookForbidden = apperr.Forbidden("webhook_forbidden", "Webhook verification failed")
	ErrWebhookSignature = apperr.Unauthorized("invalid_signature", "Webhook signature doesn't match")
)
