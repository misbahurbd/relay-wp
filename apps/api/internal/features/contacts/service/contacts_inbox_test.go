package service

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
)

// wa is a WhatsApp sender known by externalID, with a name and maybe a phone.
func wa(externalID, name, phone string) model.Sender {
	return model.Sender{ChannelType: "whatsapp", ExternalID: externalID, Name: name, Phone: phone}
}

func TestFindOrCreateByIdentity(t *testing.T) {
	pool := dbtest.New(t)
	ws := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(), `INSERT INTO workspaces (id, name) VALUES ($1, 'Inbox test')`, ws); err != nil {
		t.Fatal(err)
	}
	s := New(pool, nil, nil, nil)
	identities := func(contactID uuid.UUID) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM contact_identities WHERE workspace_id = $1 AND contact_id = $2`,
			ws, contactID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Two webhooks for the same new customer race: one contact and one identity, created once.
	var wg sync.WaitGroup
	results := make([]model.Contact, 8)
	created := make([]bool, 8)
	errs := make([]error, 8)
	for i := range results {
		wg.Go(func() {
			results[i], created[i], errs[i] = s.FindOrCreateByIdentity(t.Context(), ws, wa("+447700900123", " Grace ", "447700900123"))
		})
	}
	wg.Wait()
	n := 0
	for i := range results {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if created[i] {
			n++
		}
		if results[i].ID != results[0].ID {
			t.Fatal("different contacts for one sender")
		}
	}
	c := results[0]
	if n != 1 || c.Phone != "+447700900123" || c.Name != "Grace" || c.Source != model.SourceInbox || c.OptIn != model.OptInPending {
		t.Fatalf("created %d times; contact %+v", n, c)
	}
	if got := identities(c.ID); got != 1 {
		t.Fatalf("identities: %d", got)
	}

	// A known sender keeps its contact and name.
	again, isNew, err := s.FindOrCreateByIdentity(t.Context(), ws, wa("+447700900123", "Someone else", "+447700900123"))
	if err != nil || isNew || again.ID != c.ID || again.Name != "Grace" {
		t.Fatalf("again: %+v %v %v", again, isNew, err)
	}
	if _, _, err := s.FindOrCreateByIdentity(t.Context(), ws, wa("x", "", "not a phone")); !errors.Is(err, model.ErrInvalidPhone) {
		t.Fatalf("an invalid phone must fail: %v", err)
	}
	if _, _, err := s.FindOrCreateByIdentity(t.Context(), ws, wa("", "", "+447700900123")); !errors.Is(err, model.ErrInvalidIdentity) {
		t.Fatalf("an empty external ID must fail: %v", err)
	}

	// Last message only moves forward.
	later, earlier := time.Now().UTC().Truncate(time.Second), time.Now().UTC().Add(-time.Hour)
	for _, at := range []time.Time{later, earlier} {
		if err := database.WithTx(t.Context(), pool, func(tx pgx.Tx) error {
			return s.SetLastMessage(t.Context(), tx, ws, c.ID, at)
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Get(t.Context(), ws, c.ID)
	if err != nil || got.LastMessageAt == nil || !got.LastMessageAt.Equal(later) {
		t.Fatalf("last message: %+v %v", got.LastMessageAt, err)
	}
}

// A sender whose phone matches a contact added by hand links to it, and keeps
// reaching it after the contact's phone is edited (ADR 0010).
func TestFindOrCreateByIdentityLinksExistingContact(t *testing.T) {
	pool := dbtest.New(t)
	ws := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(), `INSERT INTO workspaces (id, name) VALUES ($1, 'Inbox test')`, ws); err != nil {
		t.Fatal(err)
	}
	s := New(pool, nil, nil, nil)
	manual := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(), `INSERT INTO contacts (id, workspace_id, name, phone) VALUES ($1, $2, 'Ada', '+447700900555')`,
		manual, ws); err != nil {
		t.Fatal(err)
	}
	c, created, err := s.FindOrCreateByIdentity(t.Context(), ws, wa("+447700900555", "Ada L.", "+447700900555"))
	if err != nil || created || c.ID != manual || c.Name != "Ada" || c.Source != model.SourceManual {
		t.Fatalf("link: %+v %v %v", c, created, err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE contacts SET phone = '+447700900999' WHERE id = $1`, manual); err != nil {
		t.Fatal(err)
	}
	c, created, err = s.FindOrCreateByIdentity(t.Context(), ws, wa("+447700900555", "Ada L.", "+447700900555"))
	if err != nil || created || c.ID != manual || c.Phone != "+447700900999" {
		t.Fatalf("after the phone edit: %+v %v %v", c, created, err)
	}
	var contacts int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM contacts WHERE workspace_id = $1`, ws).Scan(&contacts); err != nil || contacts != 1 {
		t.Fatalf("contacts: %d %v", contacts, err)
	}
}

// A WhatsApp user who writes with a username and no visible number (ADR 0015)
// becomes a contact without a phone, once however many webhooks race; a later
// message that brings the phone fills it in, and a username change is kept.
func TestFindOrCreateByIdentityUserID(t *testing.T) {
	pool := dbtest.New(t)
	ws := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(), `INSERT INTO workspaces (id, name) VALUES ($1, 'Inbox test')`, ws); err != nil {
		t.Fatal(err)
	}
	s := New(pool, nil, nil, nil)
	sender := model.Sender{ChannelType: "whatsapp", ExternalID: "BD.2294857751239750", Handle: "grace.h", Name: "Grace"}

	var wg sync.WaitGroup
	results := make([]model.Contact, 8)
	created := make([]bool, 8)
	errs := make([]error, 8)
	for i := range results {
		wg.Go(func() { results[i], created[i], errs[i] = s.FindOrCreateByIdentity(t.Context(), ws, sender) })
	}
	wg.Wait()
	n := 0
	for i := range results {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if created[i] {
			n++
		}
		if results[i].ID != results[0].ID {
			t.Fatal("different contacts for one sender")
		}
	}
	c := results[0]
	if n != 1 || c.Phone != "" || c.PhoneDisplay != "" || c.Username != "grace.h" || c.Name != "Grace" || c.Source != model.SourceInbox {
		t.Fatalf("created %d times; contact %+v", n, c)
	}
	var contacts int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM contacts WHERE workspace_id = $1`, ws).Scan(&contacts); err != nil || contacts != 1 {
		t.Fatalf("contacts: %d %v (a lost race must not leave a contact behind)", contacts, err)
	}

	// The phone arrives later, with a new username: same contact, both kept.
	sender.Phone, sender.Handle = "+447700900777", "grace.hopper"
	again, isNew, err := s.FindOrCreateByIdentity(t.Context(), ws, sender)
	if err != nil || isNew || again.ID != c.ID || again.Phone != "+447700900777" || again.Username != "grace.hopper" {
		t.Fatalf("with the phone: %+v %v %v", again, isNew, err)
	}
	// Searching by username finds them.
	q := "hopper"
	page, err := s.List(t.Context(), ws, model.Filter{Query: &q, Limit: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != c.ID {
		t.Fatalf("search by username: %+v %v", page.Items, err)
	}
}

// A customer who wrote before user IDs (their identity is their phone) keeps
// their contact when Meta starts sending a user ID with the phone.
func TestFindOrCreateByIdentityLinksUserIDToPhone(t *testing.T) {
	pool := dbtest.New(t)
	ws := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(), `INSERT INTO workspaces (id, name) VALUES ($1, 'Inbox test')`, ws); err != nil {
		t.Fatal(err)
	}
	s := New(pool, nil, nil, nil)
	old, _, err := s.FindOrCreateByIdentity(t.Context(), ws, wa("+447700900888", "Ada", "+447700900888"))
	if err != nil {
		t.Fatal(err)
	}
	// Their phone changes on the contact; the phone identity still finds them.
	if _, err := pool.Exec(t.Context(), `UPDATE contacts SET phone = '+447700900111' WHERE id = $1`, old.ID); err != nil {
		t.Fatal(err)
	}
	c, created, err := s.FindOrCreateByIdentity(t.Context(), ws, model.Sender{
		ChannelType: "whatsapp", ExternalID: "GB.1111222233334444", Phone: "+447700900888", Name: "Ada",
	})
	if err != nil || created || c.ID != old.ID {
		t.Fatalf("link user ID: %+v %v %v", c, created, err)
	}
	var ids int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM contact_identities WHERE workspace_id = $1 AND contact_id = $2`,
		ws, old.ID).Scan(&ids); err != nil || ids != 2 {
		t.Fatalf("identities: %d %v", ids, err)
	}
}
