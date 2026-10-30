package service

import (
	"context"
	"errors"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/model"

	"github.com/google/uuid"
)

type fakeBilling struct {
	check DeletionCheck
	err   error
}

func (f fakeBilling) DeletionCheck(context.Context, uuid.UUID) (DeletionCheck, error) {
	return f.check, f.err
}

func TestCheckDeletion(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		billing Billing
		confirm bool
		want    error
	}{
		{"no billing", nil, false, nil},
		{"free or trial", fakeBilling{check: DeletionCheck{Allowed: true}}, false, nil},
		{"live subscription", fakeBilling{check: DeletionCheck{Reason: "subscription"}}, false, model.ErrDeleteSubscription},
		{"live subscription, confirmed", fakeBilling{check: DeletionCheck{Reason: "subscription"}}, true, model.ErrDeleteSubscription},
		{"lifetime deal", fakeBilling{check: DeletionCheck{Reason: "lifetime"}}, false, model.ErrDeleteLifetime},
		{"lifetime deal, confirmed", fakeBilling{check: DeletionCheck{Reason: "lifetime"}}, true, nil},
		{"billing fails", fakeBilling{err: boom}, true, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{}
			s.SetBilling(tt.billing)
			err := s.CheckDeletion(context.Background(), uuid.Nil, tt.confirm)
			if !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
				t.Errorf("CheckDeletion = %v; want %v", err, tt.want)
			}
		})
	}

	s := &Service{}
	s.SetBilling(fakeBilling{check: DeletionCheck{Reason: "mystery"}})
	if err := s.CheckDeletion(context.Background(), uuid.Nil, true); err == nil {
		t.Error("unknown reason allowed deletion")
	}
}
