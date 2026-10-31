package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

const day = 24 * time.Hour

// OnTagged enrols contacts just given the drip's tag, on db so it commits with
// the tag. Enrolling once is enforced by the dedupe key, so taking a tag off and
// on again doesn't restart a sequence. Contacts tagged earlier are never swept in.
func (s *Service) OnTagged(ctx context.Context, db database.DBTX, workspaceID, tagID uuid.UUID, contactIDs []uuid.UUID) error {
	allowed, err := s.deps.Plans.HasAccess(ctx, workspaceID, model.PlanResource)
	if err != nil || !allowed {
		return err
	}
	items, err := s.enabled(ctx, workspaceID)
	if err != nil {
		return err
	}
	drip, ok := items[model.KindDrip].(*model.DripSettings)
	if !ok || drip.TagID == nil || *drip.TagID != tagID || len(drip.Steps) == 0 {
		return nil
	}
	ids, keys := make([]uuid.UUID, len(contactIDs)), make([]string, len(contactIDs))
	for i, c := range contactIDs {
		if ids[i], err = uuid.NewV7(); err != nil {
			return fmt.Errorf("new run id: %w", err)
		}
		keys[i] = "drip:" + c.String()
	}
	at := s.now().Add(time.Duration(drip.Steps[0].Day) * day)
	if err := repository.NewRepository(db).EnrolDrip(ctx, gen.EnrolDripParams{
		WorkspaceID: workspaceID, NextAt: &at, Ids: ids, ContactIds: contactIDs, DedupeKeys: keys,
	}); err != nil {
		return fmt.Errorf("enrol drip: %w", err)
	}
	return nil
}

// stopDrip ends a contact's sequence when they reply, if the setting says so. It
// reads the setting even while paused: a reply still ends the sequence.
func (s *Service) stopDrip(ctx context.Context, in model.Inbound) error {
	r := repository.NewRepository(s.pool)
	row, err := r.GetAutomation(ctx, gen.GetAutomationParams{WorkspaceID: in.WorkspaceID, Kind: string(model.KindDrip)})
	if repository.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get drip: %w", err)
	}
	settings, err := model.Decode(model.KindDrip, row.Settings)
	if err != nil {
		return err
	}
	if !settings.(*model.DripSettings).StopOnReply {
		return nil
	}
	if err := r.CancelDripRuns(ctx, gen.CancelDripRunsParams{WorkspaceID: in.WorkspaceID, ContactID: &in.ContactID}); err != nil {
		return fmt.Errorf("cancel drip: %w", err)
	}
	return nil
}

// dripStep is what a run's current step sends; ok is false past the last step.
func dripStep(raw []byte, step int32) (model.DripStep, *model.DripStep, bool, error) {
	settings, err := model.Decode(model.KindDrip, raw)
	if err != nil {
		return model.DripStep{}, nil, false, err
	}
	steps := settings.(*model.DripSettings).Steps
	if int(step) >= len(steps) {
		return model.DripStep{}, nil, false, nil
	}
	var next *model.DripStep
	if int(step)+1 < len(steps) {
		next = &steps[step+1]
	}
	return steps[step], next, true, nil
}
