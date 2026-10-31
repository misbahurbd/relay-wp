package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the contacts service.
var (
	// ErrInvalidInput carries per-field details.
	ErrInvalidInput = apperr.Validation("invalid_input", "Some fields are invalid")
	ErrInvalidPhone = apperr.Validation("invalid_phone",
		"Enter the number with its country code, for example +44 7700 900412")
	ErrInvalidCursor = apperr.Validation("invalid_cursor", "That page link has expired. Start from the first page.")

	ErrContactNotFound = apperr.NotFound("contact_not_found", "Contact not found")
	ErrTagNotFound     = apperr.NotFound("tag_not_found", "Tag not found")
	// ErrSegmentNotFound: no such segment in this workspace.
	ErrSegmentNotFound = apperr.NotFound("segment_not_found", "Segment not found")
	// ErrInvalidSegment carries per-field details, located like "rules.groups[0].conditions[1]".
	ErrInvalidSegment = apperr.Validation("invalid_segment", "Some conditions need changes")
	// ErrSegmentNameTaken: another segment has this name.
	ErrSegmentNameTaken = apperr.Conflict("segment_name_taken", "A segment with this name already exists")
	ErrUserNotMember    = apperr.Validation("user_not_member", "That person isn't a member of this workspace")
	ErrContactExists    = apperr.Conflict("contact_exists", "A contact with this phone number already exists")
	ErrContactLimit     = apperr.Conflict("plan_contact_limit", "Your plan's contact limit is reached. Upgrade your plan to add more contacts.")
	// ErrIdentityNeedsPhone: a sender without a contact can't be added yet without a phone
	// (phone-less contacts arrive with the second channel, ADR 0010).
	ErrIdentityNeedsPhone = apperr.Validation("identity_needs_phone", "This sender has no phone number, so no contact can be created")
	ErrInvalidIdentity    = apperr.Validation("invalid_identity", "The sender's channel or ID is missing")

	// ErrPermissionDenied matches middleware.RequirePermission's error, for checks that depend on the request.
	ErrPermissionDenied = apperr.Forbidden("permission_denied", "Your role in this workspace can't do that")

	ErrImportNotFound    = apperr.NotFound("import_not_found", "Import not found")
	ErrUnsupportedFile   = apperr.Validation("unsupported_file", "Upload a .csv or .xlsx file")
	ErrFileTooLarge      = apperr.Validation("file_too_large", "The file is larger than 10 MB")
	ErrTooManyRows       = apperr.Validation("too_many_rows", "The file has more than 50,000 rows. Split it and import each part.")
	ErrUnreadableFile    = apperr.Validation("unreadable_file", "We couldn't read this file. Check that it's a valid .csv or .xlsx file.")
	ErrEmptyFile         = apperr.Validation("empty_file", "The file has no rows to import")
	ErrOptInRequired     = apperr.Validation("opt_in_required", "Confirm that these contacts agreed to get WhatsApp messages from you")
	ErrInvalidMapping    = apperr.Validation("invalid_mapping", "Check the column mapping")
	ErrImportNotMapped   = apperr.Conflict("import_not_mapped", "Map the file's columns first")
	ErrImportStarted     = apperr.Conflict("import_started", "This import has already started")
	ErrNoSkippedRows     = apperr.NotFound("no_skipped_rows", "This import skipped no rows")
	ErrImportFileExpired = apperr.Conflict("import_file_missing", "The uploaded file is no longer available. Upload it again.")
)
