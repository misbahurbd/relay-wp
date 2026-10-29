package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// filter is a model.Filter turned into query parameters.
type filter struct {
	optIn                       *string
	tagID                       *uuid.UUID
	query, phoneQuery           *string
	messageAfter, messageBefore *time.Time
}

func (s *Service) toFilter(f model.Filter) filter {
	out := filter{optIn: f.OptIn, tagID: f.TagID}
	if f.Query != nil {
		q := strings.TrimSpace(*f.Query)
		if q != "" {
			like := likeEscaper.Replace(q)
			out.query = &like
			if digits, ok := phoneDigits(q); ok {
				out.phoneQuery = &digits
			}
		}
	}
	if f.LastMessage != nil {
		now := s.now()
		var after time.Time
		switch *f.LastMessage {
		case model.LastMessage24h:
			after = now.Add(-24 * time.Hour)
		case model.LastMessage7d:
			after = now.AddDate(0, 0, -7)
		case model.LastMessage30d:
			after = now.AddDate(0, 0, -30)
		case model.LastMessageOver90:
			before := now.AddDate(0, 0, -90)
			out.messageBefore = &before
		}
		if !after.IsZero() {
			out.messageAfter = &after
		}
	}
	return out
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// phoneDigits returns the digits of q when q looks like (part of) a phone number.
func phoneDigits(q string) (string, bool) {
	var b strings.Builder
	for _, r := range q {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case strings.ContainsRune(" +-().", r):
		default:
			return "", false
		}
	}
	return b.String(), b.Len() >= 3
}

// List returns one page of contacts, newest first, with the total for the filter.
func (s *Service) List(ctx context.Context, workspaceID uuid.UUID, f model.Filter) (model.Page, error) {
	if f.Limit <= 0 || f.Limit > model.MaxPageLimit {
		f.Limit = model.DefaultPageLimit
	}
	p := s.toFilter(f)
	r := repository.NewRepository(s.pool)
	rows, err := r.ListContacts(ctx, gen.ListContactsParams{
		WorkspaceID: workspaceID, OptIn: p.optIn, TagID: p.tagID, Query: p.query, PhoneQuery: p.phoneQuery,
		MessageAfter: p.messageAfter, MessageBefore: p.messageBefore, AfterID: f.After, PageSize: f.Limit + 1,
	})
	if err != nil {
		return model.Page{}, fmt.Errorf("list contacts: %w", err)
	}
	total, err := r.CountContacts(ctx, gen.CountContactsParams{
		WorkspaceID: workspaceID, OptIn: p.optIn, TagID: p.tagID, Query: p.query, PhoneQuery: p.phoneQuery,
		MessageAfter: p.messageAfter, MessageBefore: p.messageBefore,
	})
	if err != nil {
		return model.Page{}, fmt.Errorf("count contacts: %w", err)
	}
	page := model.Page{Total: total, Items: make([]model.Contact, 0, len(rows))}
	if len(rows) > int(f.Limit) {
		rows = rows[:f.Limit]
		next := rows[len(rows)-1].ID
		page.Next = &next
	}
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
		page.Items = append(page.Items, model.Contact{
			ID: row.ID, Name: row.Name, Phone: row.Phone, PhoneDisplay: DisplayPhone(row.Phone), Email: row.Email,
			OptIn: row.OptIn, Source: row.Source, Language: row.Language, LastMessageAt: row.LastMessageAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Tags: []model.TagRef{},
			AssignedTo: person(row.AssignedTo, row.AssignedDisplayName, row.AssignedFirstName, row.AssignedLastName, row.AssignedEmail),
		})
	}
	tags, err := s.tagsOf(ctx, r, workspaceID, ids)
	if err != nil {
		return model.Page{}, err
	}
	for i := range page.Items {
		if t := tags[page.Items[i].ID]; t != nil {
			page.Items[i].Tags = t
		}
	}
	return page, nil
}

func (s *Service) tagsOf(ctx context.Context, r repository.Repository, workspaceID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]model.TagRef, error) {
	out := map[uuid.UUID][]model.TagRef{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.ListTagsOfContacts(ctx, gen.ListTagsOfContactsParams{WorkspaceID: workspaceID, ContactIds: ids})
	if err != nil {
		return nil, fmt.Errorf("contact tags: %w", err)
	}
	for _, t := range rows {
		out[t.ContactID] = append(out[t.ContactID], model.TagRef{ID: t.ID, Name: t.Name})
	}
	return out, nil
}

// person builds the assignee from a LEFT JOIN of users and user_profiles.
func person(id *uuid.UUID, display, first, last, email *string) *model.Person {
	if id == nil {
		return nil
	}
	p := &model.Person{ID: *id}
	switch {
	case display != nil && *display != "":
		p.Name = *display
	case first != nil && strings.TrimSpace(*first+" "+deref(last)) != "":
		p.Name = strings.TrimSpace(*first + " " + deref(last))
	default:
		p.Name = deref(email)
	}
	return p
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Summary returns the counts for the contacts page header.
func (s *Service) Summary(ctx context.Context, workspaceID uuid.UUID) (model.Summary, error) {
	row, err := repository.NewRepository(s.pool).ContactSummary(ctx, workspaceID)
	if err != nil {
		return model.Summary{}, fmt.Errorf("contact summary: %w", err)
	}
	return model.Summary{Total: row.Total, OptedIn: row.OptedIn}, nil
}

// Get returns the full contact.
func (s *Service) Get(ctx context.Context, workspaceID, id uuid.UUID) (model.ContactDetail, error) {
	return s.get(ctx, s.pool, workspaceID, id)
}

func (s *Service) get(ctx context.Context, db database.DBTX, workspaceID, id uuid.UUID) (model.ContactDetail, error) {
	r := repository.NewRepository(db)
	row, err := r.GetContact(ctx, gen.GetContactParams{WorkspaceID: workspaceID, ID: id})
	if repository.IsNoRows(err) {
		return model.ContactDetail{}, model.ErrContactNotFound
	}
	if err != nil {
		return model.ContactDetail{}, fmt.Errorf("get contact: %w", err)
	}
	d := model.ContactDetail{
		Contact: model.Contact{
			ID: row.ID, Name: row.Name, Phone: row.Phone, PhoneDisplay: DisplayPhone(row.Phone), Email: row.Email,
			OptIn: row.OptIn, Source: row.Source, Language: row.Language, LastMessageAt: row.LastMessageAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Tags: []model.TagRef{},
			AssignedTo: person(row.AssignedTo, row.AssignedDisplayName, row.AssignedFirstName, row.AssignedLastName, row.AssignedEmail),
		},
		OptInSource: row.OptInSource, OptInAt: row.OptInAt, Attributes: map[string]string{},
	}
	if err := json.Unmarshal(row.Attributes, &d.Attributes); err != nil {
		return model.ContactDetail{}, fmt.Errorf("contact attributes: %w", err)
	}
	tags, err := s.tagsOf(ctx, r, workspaceID, []uuid.UUID{id})
	if err != nil {
		return model.ContactDetail{}, err
	}
	if t := tags[id]; t != nil {
		d.Tags = t
	}
	return d, nil
}

// contactFields are a contact's checked values.
type contactFields struct {
	name, phone, optIn string
	email, language    *string
	attributes         []byte
}

// checkInput validates in onto base (the current values, or the defaults on create).
func checkInput(in model.ContactInput, base contactFields) (contactFields, error) {
	out := base
	if in.Phone != nil {
		phone, ok := NormalizePhone(*in.Phone)
		if !ok {
			return out, model.ErrInvalidPhone.WithDetails(apperr.Detail{Field: "body.phone", Message: "Not a valid phone number with a country code"})
		}
		out.phone = phone
	}
	var details []apperr.Detail
	bad := func(field, msg string) {
		details = append(details, apperr.Detail{Field: "body." + field, Message: msg})
	}
	if in.Name != nil {
		out.name = strings.TrimSpace(*in.Name)
	}
	if out.name == "" {
		bad("name", "Enter a name")
	} else if utf8.RuneCountInString(out.name) > model.MaxNameLen {
		bad("name", fmt.Sprintf("At most %d characters", model.MaxNameLen))
	}
	if in.Email != nil {
		out.email = nil
		if e := strings.TrimSpace(*in.Email); e != "" {
			if addr, err := mail.ParseAddress(e); err != nil || addr.Address != e || len(e) > 254 {
				bad("email", "Not a valid email address")
			}
			out.email = &e
		}
	}
	if in.OptIn != nil {
		if !slices.Contains(model.OptIns, *in.OptIn) {
			bad("opt_in", "Must be in, out or pending")
		}
		out.optIn = *in.OptIn
	}
	if in.Language != nil {
		out.language = nil
		if l := strings.TrimSpace(*in.Language); l != "" {
			if len(l) > model.MaxLanguageLen {
				bad("language", fmt.Sprintf("At most %d characters", model.MaxLanguageLen))
			}
			out.language = &l
		}
	}
	if in.Attributes != nil {
		attrs, problems := cleanAttributes(in.Attributes)
		for _, p := range problems {
			bad("attributes", p)
		}
		b, err := json.Marshal(attrs)
		if err != nil {
			return out, fmt.Errorf("attributes: %w", err)
		}
		out.attributes = b
	}
	if len(details) > 0 {
		return out, model.ErrInvalidInput.WithDetails(details...)
	}
	return out, nil
}

// cleanAttributes trims keys and values and drops empty values.
func cleanAttributes(in map[string]string) (map[string]string, []string) {
	out := make(map[string]string, len(in))
	var problems []string
	for k, v := range in {
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch {
		case k == "":
			problems = append(problems, "A custom field needs a name")
		case utf8.RuneCountInString(k) > model.MaxAttributeKey:
			problems = append(problems, fmt.Sprintf("Custom field names are at most %d characters", model.MaxAttributeKey))
		case utf8.RuneCountInString(v) > model.MaxAttributeVal:
			problems = append(problems, fmt.Sprintf("%s: at most %d characters", k, model.MaxAttributeVal))
		case v != "":
			out[k] = v
		}
	}
	if len(out) > model.MaxAttributes {
		problems = append(problems, fmt.Sprintf("At most %d custom fields", model.MaxAttributes))
	}
	return out, problems
}

// checkTags makes sure every id is a tag of the workspace, and returns them without repeats.
func checkTags(ctx context.Context, r repository.Repository, workspaceID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	ids = uniqueIDs(ids)
	if len(ids) == 0 {
		return ids, nil
	}
	n, err := r.CountContactTagsIn(ctx, gen.CountContactTagsInParams{WorkspaceID: workspaceID, Ids: ids})
	if err != nil {
		return nil, fmt.Errorf("check tags: %w", err)
	}
	if int(n) != len(ids) {
		return nil, model.ErrTagNotFound
	}
	return ids, nil
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// linkTags gives every contact every tag.
func linkTags(ctx context.Context, r repository.Repository, workspaceID uuid.UUID, contactIDs, tagIDs []uuid.UUID) (int64, error) {
	if len(contactIDs) == 0 || len(tagIDs) == 0 {
		return 0, nil
	}
	cs := make([]uuid.UUID, 0, len(contactIDs)*len(tagIDs))
	ts := make([]uuid.UUID, 0, cap(cs))
	for _, c := range contactIDs {
		for _, t := range tagIDs {
			cs, ts = append(cs, c), append(ts, t)
		}
	}
	n, err := r.AddContactTagLinks(ctx, gen.AddContactTagLinksParams{WorkspaceID: workspaceID, ContactIds: cs, TagIds: ts})
	if err != nil {
		return 0, fmt.Errorf("tag contacts: %w", err)
	}
	return n, nil
}

// Create adds a contact by hand.
func (s *Service) Create(ctx context.Context, workspaceID uuid.UUID, in model.ContactInput) (model.ContactDetail, error) {
	if in.Phone == nil {
		return model.ContactDetail{}, model.ErrInvalidPhone.WithDetails(apperr.Detail{Field: "body.phone", Message: "Enter a phone number"})
	}
	f, err := checkInput(in, contactFields{optIn: model.OptInPending, attributes: []byte("{}")})
	if err != nil {
		return model.ContactDetail{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return model.ContactDetail{}, fmt.Errorf("contact id: %w", err)
	}
	var detail model.ContactDetail
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		var tagIDs []uuid.UUID
		if in.TagIDs != nil {
			if tagIDs, err = checkTags(ctx, r, workspaceID, *in.TagIDs); err != nil {
				return err
			}
		}
		p := gen.CreateContactParams{
			ID: id, WorkspaceID: workspaceID, Name: f.name, Phone: f.phone, Email: f.email, OptIn: f.optIn,
			Source: model.SourceManual, Language: f.language, Attributes: f.attributes,
		}
		if f.optIn != model.OptInPending {
			src, now := model.SourceManual, s.now()
			p.OptInSource, p.OptInAt = &src, &now
		}
		if err := r.CreateContact(ctx, p); err != nil {
			if repository.IsUnique(err, repository.PhoneConstraint) {
				return model.ErrContactExists
			}
			return fmt.Errorf("create contact: %w", err)
		}
		if _, err := linkTags(ctx, r, workspaceID, []uuid.UUID{id}, tagIDs); err != nil {
			return err
		}
		detail, err = s.get(ctx, tx, workspaceID, id)
		return err
	})
	return detail, err
}

// Update changes the given fields of a contact.
func (s *Service) Update(ctx context.Context, workspaceID, id uuid.UUID, in model.ContactInput) (model.ContactDetail, error) {
	var detail model.ContactDetail
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		cur, err := r.LockContact(ctx, gen.LockContactParams{WorkspaceID: workspaceID, ID: id})
		if repository.IsNoRows(err) {
			return model.ErrContactNotFound
		}
		if err != nil {
			return fmt.Errorf("lock contact: %w", err)
		}
		f, err := checkInput(in, contactFields{
			name: cur.Name, phone: cur.Phone, optIn: cur.OptIn, email: cur.Email, language: cur.Language,
			attributes: cur.Attributes,
		})
		if err != nil {
			return err
		}
		p := gen.UpdateContactParams{
			WorkspaceID: workspaceID, ID: id, Name: f.name, Phone: f.phone, Email: f.email, OptIn: f.optIn,
			OptInSource: cur.OptInSource, OptInAt: cur.OptInAt, Language: f.language, Attributes: f.attributes,
		}
		if f.optIn != cur.OptIn {
			src, now := model.SourceManual, s.now()
			p.OptInSource, p.OptInAt = &src, &now
		}
		if err := r.UpdateContact(ctx, p); err != nil {
			if repository.IsUnique(err, repository.PhoneConstraint) {
				return model.ErrContactExists
			}
			return fmt.Errorf("update contact: %w", err)
		}
		if in.TagIDs != nil {
			tagIDs, err := checkTags(ctx, r, workspaceID, *in.TagIDs)
			if err != nil {
				return err
			}
			if err := r.ClearContactTags(ctx, gen.ClearContactTagsParams{WorkspaceID: workspaceID, ContactID: id}); err != nil {
				return fmt.Errorf("clear contact tags: %w", err)
			}
			if _, err := linkTags(ctx, r, workspaceID, []uuid.UUID{id}, tagIDs); err != nil {
				return err
			}
		}
		detail, err = s.get(ctx, tx, workspaceID, id)
		return err
	})
	return detail, err
}

// Delete removes a contact.
func (s *Service) Delete(ctx context.Context, workspaceID, id uuid.UUID) error {
	n, err := repository.NewRepository(s.pool).DeleteContacts(ctx, gen.DeleteContactsParams{
		WorkspaceID: workspaceID, Ids: []uuid.UUID{id},
	})
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	if n == 0 {
		return model.ErrContactNotFound
	}
	return nil
}

// BulkPermission is the permission a bulk action needs.
func BulkPermission(action string) authctx.Permission {
	if action == model.BulkDelete {
		return authctx.PermContactsDelete
	}
	return authctx.PermContactsEdit
}

// Bulk runs one action on up to 500 contacts and returns how many rows it changed.
// The permission depends on the action, so it is checked here rather than on the route.
func (s *Service) Bulk(ctx context.Context, actor authctx.Access, b model.Bulk) (int64, error) {
	if need := BulkPermission(b.Action); !actor.Can(need) {
		return 0, model.ErrPermissionDenied.WithDetails(apperr.Detail{Field: "permission", Message: need.Key()})
	}
	ids := uniqueIDs(b.ContactIDs)
	if len(ids) == 0 || len(ids) > model.MaxBulkContacts {
		return 0, model.ErrInvalidInput.WithDetails(apperr.Detail{
			Field: "body.contact_ids", Message: fmt.Sprintf("Choose 1 to %d contacts", model.MaxBulkContacts),
		})
	}
	ws := actor.WorkspaceID
	r := repository.NewRepository(s.pool)
	switch b.Action {
	case model.BulkAddTag, model.BulkRemoveTag:
		if b.TagID == nil {
			return 0, model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "body.tag_id", Message: "Choose a tag"})
		}
		if _, err := checkTags(ctx, r, ws, []uuid.UUID{*b.TagID}); err != nil {
			return 0, err
		}
		if b.Action == model.BulkAddTag {
			return linkTags(ctx, r, ws, ids, []uuid.UUID{*b.TagID})
		}
		n, err := r.RemoveContactTagLinks(ctx, gen.RemoveContactTagLinksParams{WorkspaceID: ws, TagID: *b.TagID, ContactIds: ids})
		if err != nil {
			return 0, fmt.Errorf("untag contacts: %w", err)
		}
		return n, nil
	case model.BulkAssign:
		// No user_id unassigns.
		if b.UserID != nil {
			_, _, ok, err := s.members.ResolveAccess(ctx, ws, *b.UserID)
			if err != nil {
				return 0, fmt.Errorf("check member: %w", err)
			}
			if !ok {
				return 0, model.ErrUserNotMember
			}
		}
		n, err := r.AssignContacts(ctx, gen.AssignContactsParams{WorkspaceID: ws, Ids: ids, UserID: b.UserID})
		if err != nil {
			return 0, fmt.Errorf("assign contacts: %w", err)
		}
		return n, nil
	case model.BulkDelete:
		n, err := r.DeleteContacts(ctx, gen.DeleteContactsParams{WorkspaceID: ws, Ids: ids})
		if err != nil {
			return 0, fmt.Errorf("delete contacts: %w", err)
		}
		return n, nil
	default:
		return 0, model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "body.action", Message: "Unknown action"})
	}
}
