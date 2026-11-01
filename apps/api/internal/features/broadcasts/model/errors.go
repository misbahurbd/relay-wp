package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the broadcasts service.
var (
	ErrNotFound = apperr.NotFound("broadcast_not_found", "Broadcast not found")
	// ErrInvalidInput carries per-field details, located like "variables.body[0].value".
	ErrInvalidInput = apperr.Validation("invalid_broadcast", "Some fields are invalid")
	// ErrNotEditable: only drafts and scheduled broadcasts can change.
	ErrNotEditable = apperr.Conflict("broadcast_not_editable", "This broadcast has already started and can't be changed")
	// ErrNotSendable carries what is missing before a draft can go.
	ErrNotSendable = apperr.Validation("broadcast_not_ready", "Finish this broadcast before sending it")
	// ErrEmptyAudience: nobody who opted in matches the audience.
	ErrEmptyAudience = apperr.Validation("broadcast_empty_audience", "No opted-in contacts match this audience")
	// ErrTemplateNotApproved: the template is gone, or Meta hasn't approved it.
	ErrTemplateNotApproved = apperr.Validation("template_not_approved", "Choose a template Meta has approved")
	// ErrNumberNotConnected: the WhatsApp number is gone or disconnected.
	ErrNumberNotConnected = apperr.Validation("number_not_connected", "Choose a connected WhatsApp number")
	// ErrPlanLimit: the plan's broadcasts for this month are used up.
	ErrPlanLimit = apperr.Conflict("plan_broadcast_limit", "You've sent all the broadcasts your plan includes this month")
	// ErrNotCancellable: only scheduled and sending broadcasts can be cancelled.
	ErrNotCancellable = apperr.Conflict("broadcast_not_cancellable", "This broadcast isn't scheduled or sending")
	// ErrSegmentNotFound: an audience names a segment that is gone.
	ErrSegmentNotFound = apperr.Validation("broadcast_segment_not_found", "A segment in this audience was deleted. Choose the audience again.")
)

// Why the inbox couldn't queue one recipient's message. The batch records them
// on the recipient and carries on; any other error retries the batch.
var (
	ErrSendOptedOut        = apperr.Conflict("broadcast_send_opted_out", "This contact has opted out")
	ErrSendTemplate        = apperr.Conflict("broadcast_send_template", "This template can't be sent")
	ErrSendMissingVariable = apperr.Conflict("broadcast_send_missing_variable", "A template variable has no value")
	ErrSendNoChannel       = apperr.Conflict("broadcast_send_no_channel", "The WhatsApp number can't send")
)
