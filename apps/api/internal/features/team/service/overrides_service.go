package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// AddOverride gives one member a permission their role doesn't have (grant),
// or takes one away (deny), optionally until a date.
func (s *Service) AddOverride(ctx context.Context, actor authctx.Access, meta model.Meta, userID uuid.UUID, in model.OverrideInput) error {
	p, ok := authctx.PermissionByKey(in.PermissionKey)
	var details []apperr.Detail
	if !ok {
		details = append(details, apperr.Detail{Field: "permission_key", Message: "Choose a permission"})
	}
	if in.Effect != model.EffectGrant && in.Effect != model.EffectDeny {
		details = append(details, apperr.Detail{Field: "effect", Message: "Choose grant or deny"})
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(s.now()) {
		details = append(details, apperr.Detail{Field: "expires_at", Message: "Pick a date in the future"})
	}
	reason := strings.TrimSpace(in.Reason)
	if utf8.RuneCountInString(reason) > 200 {
		details = append(details, apperr.Detail{Field: "reason", Message: "Keep the reason under 200 characters"})
	}
	if len(details) > 0 {
		return model.ErrInvalidInput.WithDetails(details...)
	}
	if !actor.Can(p) {
		return model.ErrCannotGrant.WithDetails(apperr.Detail{Field: "permission_key", Message: p.Key()})
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("override id: %w", err)
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		m, err := lockTarget(ctx, r, actor, userID)
		if err != nil {
			return err
		}
		if m.IsOwner {
			return model.ErrOwnerProtected // the owner always has everything
		}
		if err := r.RevokeActiveOverride(ctx, gen.RevokeActiveOverrideParams{
			WorkspaceID: actor.WorkspaceID, UserID: userID, PermissionKey: p.Key(), RevokedBy: &actor.UserID,
		}); err != nil {
			return fmt.Errorf("clear override: %w", err)
		}
		if err := r.CreateOverride(ctx, gen.CreateOverrideParams{
			ID: id, WorkspaceID: actor.WorkspaceID, UserID: userID, PermissionKey: p.Key(), Effect: in.Effect,
			Reason: reason, GrantedBy: &actor.UserID, ExpiresAt: in.ExpiresAt,
		}); err != nil {
			return fmt.Errorf("create override: %w", err)
		}
		if err := r.BumpAccessVersions(ctx, gen.BumpAccessVersionsParams{WorkspaceID: actor.WorkspaceID, UserIds: []uuid.UUID{userID}}); err != nil {
			return fmt.Errorf("bump access: %w", err)
		}
		action := repository.ActionPermissionGranted
		if in.Effect == model.EffectDeny {
			action = repository.ActionPermissionDenied
		}
		after := map[string]any{"effect": in.Effect}
		if in.ExpiresAt != nil {
			after["expires_at"] = in.ExpiresAt
		}
		return r.Log(ctx, repository.Event{
			WorkspaceID: actor.WorkspaceID, Actor: &actor.UserID, Subject: &userID, SubjectEmail: &m.Email,
			Action: action, Target: p.Key(), After: after, Note: reason,
		}, meta)
	})
}

// RevokeOverride removes one person's extra or removed permission. Their role's permissions stay.
func (s *Service) RevokeOverride(ctx context.Context, actor authctx.Access, meta model.Meta, userID, overrideID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		m, err := lockTarget(ctx, r, actor, userID)
		if err != nil {
			return err
		}
		o, err := r.RevokeOverride(ctx, gen.RevokeOverrideParams{
			WorkspaceID: actor.WorkspaceID, UserID: userID, ID: overrideID, RevokedBy: &actor.UserID,
		})
		if repository.IsNoRows(err) {
			return model.ErrOverrideGone
		}
		if err != nil {
			return fmt.Errorf("revoke override: %w", err)
		}
		if err := r.BumpAccessVersions(ctx, gen.BumpAccessVersionsParams{WorkspaceID: actor.WorkspaceID, UserIds: []uuid.UUID{userID}}); err != nil {
			return fmt.Errorf("bump access: %w", err)
		}
		return r.Log(ctx, repository.Event{
			WorkspaceID: actor.WorkspaceID, Actor: &actor.UserID, Subject: &userID, SubjectEmail: &m.Email,
			Action: repository.ActionPermissionRevoked, Target: o.PermissionKey, Before: map[string]any{"effect": o.Effect},
		}, meta)
	})
}

// ExpireOverrides records temporary permissions that have reached their end date.
func (s *Service) ExpireOverrides(ctx context.Context) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		rows, err := r.ExpireOverrides(ctx)
		if err != nil {
			return fmt.Errorf("expire overrides: %w", err)
		}
		for _, o := range rows {
			if err := r.BumpAccessVersions(ctx, gen.BumpAccessVersionsParams{WorkspaceID: o.WorkspaceID, UserIds: []uuid.UUID{o.UserID}}); err != nil {
				return fmt.Errorf("bump access: %w", err)
			}
			if err := r.Log(ctx, repository.Event{
				WorkspaceID: o.WorkspaceID, Subject: &o.UserID, Action: repository.ActionPermissionExpired, Target: o.PermissionKey,
				Before: map[string]any{"effect": o.Effect},
			}, model.Meta{}); err != nil {
				return err
			}
		}
		return nil
	})
}
