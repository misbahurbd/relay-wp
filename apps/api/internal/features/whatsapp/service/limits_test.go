package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

type fakePlans struct {
	limit int
	err   error
}

func (f fakePlans) NumberLimit(context.Context, uuid.UUID) (int, error) { return f.limit, f.err }

func TestNumberLimit(t *testing.T) {
	ctx := context.Background()
	s := &Service{}
	if got, err := s.numberLimit(ctx, uuid.Nil); err != nil || got != -1 {
		t.Errorf("no plan limits: got %d, %v; want -1", got, err)
	}
	s.SetPlanLimits(fakePlans{limit: 2})
	if got, err := s.numberLimit(ctx, uuid.Nil); err != nil || got != 2 {
		t.Errorf("got %d, %v; want 2", got, err)
	}
	boom := errors.New("boom")
	s.SetPlanLimits(fakePlans{err: boom})
	if _, err := s.numberLimit(ctx, uuid.Nil); !errors.Is(err, boom) {
		t.Errorf("err = %v; want boom", err)
	}
}

func TestOverNumberLimit(t *testing.T) {
	tests := []struct {
		name             string
		limit, connected int
		blocked          bool
	}{
		{"under the cap", 3, 1, false},
		{"one below the cap", 3, 2, false},
		{"at the cap", 3, 3, true},
		{"over the cap", 3, 5, true}, // the plan shrank
		{"zero cap", 0, 0, true},
		{"unlimited", -1, 100, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := overNumberLimit(tt.limit, tt.connected)
			if e, ok := apperr.As(err); tt.blocked != (ok && e.Code == model.ErrNumberLimit.Code) || (!tt.blocked && err != nil) {
				t.Errorf("overNumberLimit(%d, %d) = %v; blocked want %v", tt.limit, tt.connected, err, tt.blocked)
			}
		})
	}
}

func TestCheckNumberLimitUnlimitedSkipsDB(t *testing.T) {
	// A nil DB would panic if touched: no cap means no queries.
	if err := checkNumberLimit(context.Background(), nil, uuid.Nil, "123456", -1); err != nil {
		t.Fatal(err)
	}
}
