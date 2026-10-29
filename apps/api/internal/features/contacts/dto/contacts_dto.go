package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
)

type PersonDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type TagRefDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// ContactDTO is exported so huma flattens it into ContactDetailDTO (it skips unexported embeds).
type ContactDTO struct {
	ID            uuid.UUID   `json:"id"`
	Name          string      `json:"name"`
	Phone         string      `json:"phone" doc:"E.164, for example +447700900412" example:"+447700900412"`
	PhoneDisplay  string      `json:"phone_display" doc:"International format for display" example:"+44 7700 900412"`
	Email         *string     `json:"email"`
	OptIn         string      `json:"opt_in" enum:"in,out,pending"`
	Source        string      `json:"source" enum:"manual,import,inbox,api"`
	Language      *string     `json:"language"`
	AssignedTo    *PersonDTO  `json:"assigned_to"`
	LastMessageAt *time.Time  `json:"last_message_at"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	Tags          []TagRefDTO `json:"tags"`
}

func ToContactDTO(c model.Contact) ContactDTO {
	d := ContactDTO{
		ID: c.ID, Name: c.Name, Phone: c.Phone, PhoneDisplay: c.PhoneDisplay, Email: c.Email, OptIn: c.OptIn,
		Source: c.Source, Language: c.Language, LastMessageAt: c.LastMessageAt, CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt, Tags: make([]TagRefDTO, len(c.Tags)),
	}
	if c.AssignedTo != nil {
		d.AssignedTo = &PersonDTO{ID: c.AssignedTo.ID, Name: c.AssignedTo.Name}
	}
	for i, t := range c.Tags {
		d.Tags[i] = TagRefDTO(t)
	}
	return d
}

type ContactDetailDTO struct {
	ContactDTO
	OptInSource *string           `json:"opt_in_source"`
	OptInAt     *time.Time        `json:"opt_in_at"`
	Attributes  map[string]string `json:"attributes" doc:"Custom fields; string values only"`
}

func ToContactDetailDTO(c model.ContactDetail) ContactDetailDTO {
	d := ContactDetailDTO{ContactDTO: ToContactDTO(c.Contact), OptInSource: c.OptInSource, OptInAt: c.OptInAt, Attributes: c.Attributes}
	if d.Attributes == nil {
		d.Attributes = map[string]string{}
	}
	return d
}

// ContactFilters are the list's filters, shared with the export.
type ContactFilters struct {
	Query       string `query:"q" maxLength:"100" doc:"Name, phone or email contains this (case-insensitive)"`
	OptIn       string `query:"opt_in" enum:"in,out,pending"`
	TagID       string `query:"tag_id" format:"uuid"`
	LastMessage string `query:"last_message" enum:"24h,7d,30d,over_90d" doc:"Last message within 24h/7d/30d, or over 90 days ago"`
}

func (f ContactFilters) ToModel() model.Filter {
	return model.Filter{
		Query: optional(f.Query), OptIn: optional(f.OptIn), TagID: optionalID(f.TagID), LastMessage: optional(f.LastMessage),
	}
}

type ListContactsInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	ContactFilters
	Limit  int32  `query:"limit" minimum:"1" maximum:"100" default:"25"`
	Cursor string `query:"cursor" doc:"next_cursor from the previous page"`
}

type ContactsOutput struct {
	Body struct {
		Items      []ContactDTO `json:"items"`
		NextCursor *string      `json:"next_cursor"`
		Total      int64        `json:"total" doc:"Contacts matching the filter"`
	}
}

type SummaryOutput struct {
	Body struct {
		Total   int64 `json:"total"`
		OptedIn int64 `json:"opted_in"`
	}
}

type ContactPath struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	ContactID   uuid.UUID `path:"contactId"`
}

type ContactOutput struct {
	Body ContactDetailDTO
}

type CreateContactInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	Body        struct {
		Name       string            `json:"name" maxLength:"200"`
		Phone      string            `json:"phone" doc:"With country code; stored as E.164"`
		Email      *string           `json:"email,omitempty"`
		OptIn      *string           `json:"opt_in,omitempty" enum:"in,out,pending" doc:"Defaults to pending"`
		Language   *string           `json:"language,omitempty" maxLength:"35"`
		Attributes map[string]string `json:"attributes,omitempty"`
		TagIDs     []uuid.UUID       `json:"tag_ids,omitempty"`
	}
}

func (in *CreateContactInput) ToModel() model.ContactInput {
	b := in.Body
	out := model.ContactInput{
		Name: &b.Name, Phone: &b.Phone, Email: b.Email, OptIn: b.OptIn, Language: b.Language, Attributes: b.Attributes,
	}
	if b.TagIDs != nil {
		out.TagIDs = &b.TagIDs
	}
	return out
}

type UpdateContactInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	ContactID   uuid.UUID `path:"contactId"`
	Body        struct {
		Name       *string           `json:"name,omitempty" maxLength:"200"`
		Phone      *string           `json:"phone,omitempty"`
		Email      *string           `json:"email,omitempty" doc:"\"\" clears it"`
		OptIn      *string           `json:"opt_in,omitempty" enum:"in,out,pending"`
		Language   *string           `json:"language,omitempty" maxLength:"35" doc:"\"\" clears it"`
		Attributes map[string]string `json:"attributes,omitempty" doc:"Replaces every custom field; omit to keep"`
		TagIDs     []uuid.UUID       `json:"tag_ids,omitempty" doc:"Replaces the contact's tags; omit to keep"`
	}
}

func (in *UpdateContactInput) ToModel() model.ContactInput {
	b := in.Body
	out := model.ContactInput{
		Name: b.Name, Phone: b.Phone, Email: b.Email, OptIn: b.OptIn, Language: b.Language, Attributes: b.Attributes,
	}
	if b.TagIDs != nil {
		out.TagIDs = &b.TagIDs
	}
	return out
}

type BulkInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	Body        struct {
		Action     string      `json:"action" enum:"add_tag,remove_tag,assign,delete" doc:"delete needs contacts.delete; the others contacts.edit"`
		ContactIDs []uuid.UUID `json:"contact_ids" minItems:"1" maxItems:"500"`
		TagID      *uuid.UUID  `json:"tag_id,omitempty" doc:"For add_tag and remove_tag"`
		UserID     *uuid.UUID  `json:"user_id,omitempty" doc:"For assign; omit to unassign"`
	}
}

func (in *BulkInput) ToModel() model.Bulk {
	return model.Bulk{Action: in.Body.Action, ContactIDs: in.Body.ContactIDs, TagID: in.Body.TagID, UserID: in.Body.UserID}
}

type BulkOutput struct {
	Body struct {
		Affected int64 `json:"affected"`
	}
}

type ExportContactsInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	ContactFilters
}
