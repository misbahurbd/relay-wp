package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the inbox service (docs/PHASE-7-INBOX.md section 6).
var (
	// ErrInvalidInput carries per-field details.
	ErrInvalidInput  = apperr.Validation("invalid_input", "Some fields are invalid")
	ErrInvalidCursor = apperr.Validation("invalid_cursor", "That page link has expired. Start from the first page.")

	// ErrConversationNotFound is also the answer for a chat outside the member's scope (AD-10).
	ErrConversationNotFound = apperr.NotFound("conversation_not_found", "Conversation not found")
	ErrMessageNotFound      = apperr.NotFound("message_not_found", "Message not found")
	ErrQuickReplyNotFound   = apperr.NotFound("quick_reply_not_found", "Quick reply not found")
	ErrMediaNotReady        = apperr.NotFound("media_not_ready", "This file is still loading. Try again in a moment.")

	ErrWindowClosed = apperr.Conflict("window_closed",
		"The reply window is closed. Send an approved template to restart the conversation.")
	// ErrTemplatesUnsupported: the conversation's channel has no templates (ADR 0010).
	ErrTemplatesUnsupported = apperr.Conflict("templates_unsupported", "This channel doesn't support templates")
	// ErrReactionsUnsupported: the conversation's channel has no reactions (ADR 0010).
	ErrReactionsUnsupported = apperr.Conflict("reactions_unsupported", "This channel doesn't support reactions")
	// ErrChannelUnavailable: no adapter is registered for the conversation's channel type.
	ErrChannelUnavailable = apperr.Conflict("channel_unavailable",
		"This chat's channel isn't available, so messages can't be sent")
	ErrContactOptedOut = apperr.Conflict("contact_opted_out",
		"This contact opted out of marketing messages. Send a utility template instead.")
	ErrTemplateNotApproved   = apperr.Validation("template_not_approved", "Choose an approved template")
	ErrTemplateParamsMissing = apperr.Validation("template_params_missing", "Fill in every variable of the template")
	ErrMediaTooLarge         = apperr.Validation("media_too_large", "The file is too large for this channel")
	ErrMediaTypeNotAllowed   = apperr.Validation("media_type_not_allowed", "This channel can't send this type of file")
	ErrInvalidMention        = apperr.Validation("invalid_mention", "You can only mention members who can see this chat")
	ErrInvalidAssignee       = apperr.Validation("invalid_assignee", "Assign the chat to an active member who works on this number")
	ErrInvalidReplyTo        = apperr.Validation("invalid_reply_to", "You can only quote a message from this chat")
	ErrMessageNotSent        = apperr.Conflict("message_not_sent", "That message hasn't been delivered to the channel yet")
	ErrQuickReplyExists      = apperr.Conflict("quick_reply_exists", "A quick reply with this shortcut already exists")
	// ErrSendingPaused is for a workspace whose plan stopped sending (PHASE-3-ONBOARDING, trial over).
	ErrSendingPaused = apperr.Conflict("sending_paused", "Sending is paused. Choose a plan to send messages again.")

	// ErrPermissionDenied matches middleware.RequirePermission's error, for checks that depend on the request.
	ErrPermissionDenied = apperr.Forbidden("permission_denied", "Your role in this workspace can't do that")
	// ErrLiveUnavailable is returned when this process has no live-update hub (worker mode).
	ErrLiveUnavailable = apperr.Unavailable("live_updates_unavailable", "Live updates are unavailable. Refresh to see new messages.")
)
