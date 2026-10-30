package service

import (
	"context"
	"errors"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

type fakePlans struct {
	limit int
	err   error
}

func (f fakePlans) MemberLimit(context.Context, uuid.UUID) (int, error) { return f.limit, f.err }

func TestMemberLimit(t *testing.T) {
	ctx := context.Background()
	s := &Service{}
	if got, err := s.memberLimit(ctx, uuid.Nil); err != nil || got != -1 {
		t.Errorf("no plan limits: got %d, %v; want -1", got, err)
	}
	s.SetPlanLimits(fakePlans{limit: 3})
	if got, err := s.memberLimit(ctx, uuid.Nil); err != nil || got != 3 {
		t.Errorf("got %d, %v; want 3", got, err)
	}
	boom := errors.New("boom")
	s.SetPlanLimits(fakePlans{err: boom})
	if _, err := s.memberLimit(ctx, uuid.Nil); !errors.Is(err, boom) {
		t.Errorf("err = %v; want boom", err)
	}
}

func TestOverMemberLimit(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		taken, adding int64
		blocked       bool
	}{
		{"invite under the cap", 5, 3, 1, false},
		{"invite reaching the cap", 5, 3, 2, false},
		{"invite at the cap", 5, 5, 1, true},
		{"invite past the cap", 5, 4, 2, true},
		{"accept under the cap", 5, 4, 0, false},
		{"accept at the cap", 5, 5, 0, false},  // the invite's own seat is already taken
		{"accept over the cap", 5, 6, 0, true}, // the plan shrank since the invite
		{"unlimited", -1, 1000, 50, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := overMemberLimit(tt.limit, tt.taken, tt.adding)
			if e, ok := apperr.As(err); tt.blocked != (ok && e.Code == model.ErrMemberLimit.Code) || (!tt.blocked && err != nil) {
				t.Errorf("overMemberLimit(%d, %d, %d) = %v; blocked want %v", tt.limit, tt.taken, tt.adding, err, tt.blocked)
			}
		})
	}
}
