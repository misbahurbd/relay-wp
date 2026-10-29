package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the templates service.
var (
	// ErrInvalidTemplate carries per-field details, located like "body.text" or "buttons[1].value".
	ErrInvalidTemplate = apperr.Validation("invalid_template", "Some fields of the template are invalid")
	ErrInvalidSample   = apperr.Validation("invalid_sample", "This file can't be used as the header sample")

	ErrNotFound    = apperr.NotFound("template_not_found", "Template not found")
	ErrNameTaken   = apperr.Conflict("template_exists", "A template with this name and language already exists")
	ErrInReview    = apperr.Conflict("template_in_review", "Meta is reviewing this template. Wait for the result before changing it.")
	ErrNotEditable = apperr.Conflict("template_not_editable", "Paused or disabled templates can't be changed")
)
