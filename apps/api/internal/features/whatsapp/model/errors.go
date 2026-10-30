package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the whatsapp service.
var (
	ErrDisabled         = apperr.NotFound("whatsapp_disabled", "WhatsApp isn't available on this server yet.")
	ErrInvalidInput     = apperr.Validation("invalid_input", "Some fields are invalid")
	ErrCodeExchange     = apperr.Validation("whatsapp_code_exchange_failed", "Meta didn't confirm the connection. Try connecting again.")
	ErrMetaUnavailable  = apperr.Unavailable("whatsapp_unavailable", "Meta isn't answering right now. Try again in a minute.")
	ErrAlreadyConnected = apperr.Conflict("whatsapp_already_connected", "This WhatsApp Business account is connected to another workspace.")
	ErrNumberInUse      = apperr.Conflict("whatsapp_number_in_use", "This number is connected to another workspace.")
	ErrNumberLimit      = apperr.Conflict("plan_number_limit", "Your plan's WhatsApp number limit is reached. Upgrade your plan to connect another number.")
	ErrNumberNotFound   = apperr.NotFound("whatsapp_number_not_found", "WhatsApp number not found")
	ErrMessageNotFound  = apperr.NotFound("test_message_not_found", "Message not found")
	ErrNotConnected     = apperr.Conflict("whatsapp_not_connected", "Your number is still being set up.")
	ErrDisconnected     = apperr.Conflict("whatsapp_disconnected", "WhatsApp is disconnected. Connect your number again.")
	ErrSameNumber       = apperr.Validation("same_number", "Use a different number from the one you just connected.")
	ErrTooManyTests     = apperr.RateLimited("too_many_test_messages", "You've sent 5 test messages this hour. Try again later.")
	ErrWebhookForbidden = apperr.Forbidden("webhook_forbidden", "Webhook verification failed")
	ErrWebhookSignature = apperr.Unauthorized("invalid_signature", "Webhook signature doesn't match")
	ErrNoAccount        = apperr.Conflict("whatsapp_not_connected", "Connect a WhatsApp number first.")
	ErrTemplateRefused  = apperr.Validation("meta_template_refused", "Meta refused this template")
	// ErrMediaTooLarge is a media file over WhatsApp's 100 MB limit.
	ErrMediaTooLarge = apperr.Validation("media_too_large", "This file is larger than 100 MB.")
	// ErrNumberNotConnected is a send from a number that is still being set up, failed or disconnected.
	ErrNumberNotConnected = apperr.Conflict("whatsapp_number_not_connected", "This WhatsApp number isn't connected.")
)
