package auth

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// Refresh rotates a refresh token and issues a new access token. Presenting a
// token that was already rotated means it was copied: the whole session is
// revoked, unless it was rotated less than refreshReuseGrace ago.
func (s *Service) Refresh(ctx context.Context, rawToken string, meta RequestMeta) (Tokens, error) {
	if rawToken == "" {
		return Tokens{}, ErrInvalidRefresh
	}
	now := s.now()
	var tokens Tokens
	reused := false

	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		row, err := r.GetRefreshTokenForUpdate(ctx, hashToken(rawToken))
		if isNoRows(err) {
			return ErrInvalidRefresh
		}
		if err != nil {
			return fmt.Errorf("load refresh token: %w", err)
		}

		// Inside the grace window a used token gets a sibling instead of a revocation
		// (another tab refreshed with the same cookie a moment ago).
		graced := row.UsedAt != nil && now.Sub(*row.UsedAt) < refreshReuseGrace
		if row.UsedAt != nil && !graced {
			// Commit the revocation, then report failure after the transaction.
			reused = true
			reason := revokeReuseDetected
			if err := r.RevokeSessionByID(ctx, gen.RevokeSessionByIDParams{ID: row.SessionID, Reason: &reason}); err != nil {
				return fmt.Errorf("revoke reused session: %w", err)
			}
			return s.record(ctx, r, &row.UserID, eventRefreshReuseDetected, "", meta,
				map[string]any{"session_id": row.SessionID})
		}
		if row.SessionRevokedAt != nil || !now.Before(row.ExpiresAt) || !now.Before(row.SessionExpiresAt) ||
			row.UserStatus != "active" || row.UserEmailVerifiedAt == nil {
			return ErrInvalidRefresh
		}

		if !graced {
			if err := r.MarkRefreshTokenUsed(ctx, gen.MarkRefreshTokenUsedParams{ID: row.ID, UsedAt: &now}); err != nil {
				return fmt.Errorf("mark refresh token used: %w", err)
			}
		}
		exp := s.sessionExpiry(row.SessionCreatedAt, now)
		if !now.Before(exp) {
			return ErrInvalidRefresh // past SessionMaxAge: sign in again
		}
		refresh, refreshExp, err := s.newRefreshToken(ctx, r, row.SessionID, exp)
		if err != nil {
			return err
		}
		if err := r.ExtendSession(ctx, gen.ExtendSessionParams{ID: row.SessionID, Ip: parseIP(meta.IP), ExpiresAt: refreshExp}); err != nil {
			return fmt.Errorf("extend session: %w", err)
		}
		access, accessExp, err := s.tokens.Issue(row.UserID, row.SessionID, now)
		if err != nil {
			return fmt.Errorf("issue access token: %w", err)
		}
		tokens = Tokens{
			Client: row.Client, AccessToken: access, AccessExpiresAt: accessExp,
			RefreshToken: refresh, RefreshExpiresAt: refreshExp,
		}
		return nil
	})
	if err != nil {
		return Tokens{}, err
	}
	if reused {
		return Tokens{}, ErrInvalidRefresh
	}
	return tokens, nil
}

// Logout ends the current session.
func (s *Service) Logout(ctx context.Context, userID, sessionID uuid.UUID, meta RequestMeta) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		reason := revokeLogout
		if _, err := r.RevokeSession(ctx, gen.RevokeSessionParams{ID: sessionID, UserID: userID, Reason: &reason}); err != nil {
			return fmt.Errorf("revoke session: %w", err)
		}
		return s.record(ctx, r, &userID, eventLogout, "", meta, nil)
	})
}

// LogoutAll ends every session of the user, including this one.
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID, meta RequestMeta) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		reason := revokeLogoutAll
		n, err := r.RevokeUserSessions(ctx, gen.RevokeUserSessionsParams{UserID: userID, Reason: &reason})
		if err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		return s.record(ctx, r, &userID, eventLogoutAll, "", meta, map[string]any{"sessions": n})
	})
}

// --- Sessions and events ---

// ListSessions returns the user's active sessions, this device first.
func (s *Service) ListSessions(ctx context.Context, userID, currentSessionID uuid.UUID) ([]Session, error) {
	rows, err := newRepository(s.pool).ListActiveSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	out := make([]Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, Session{
			ID: row.ID, Provider: row.Provider, Client: row.Client,
			DeviceName: row.DeviceName, DeviceType: row.DeviceType, LastIP: ipString(row.LastIp),
			LastUsedAt: row.LastUsedAt, CreatedAt: row.CreatedAt, Current: row.ID == currentSessionID,
		})
	}
	// Rows arrive most recent first; move the current device to the top, keep the rest in order.
	slices.SortStableFunc(out, func(a, b Session) int {
		switch {
		case a.Current == b.Current:
			return 0
		case a.Current:
			return -1
		default:
			return 1
		}
	})
	return out, nil
}

// RevokeSession signs out one of the user's own sessions.
func (s *Service) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID, meta RequestMeta) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		reason := revokeByUser
		n, err := r.RevokeSession(ctx, gen.RevokeSessionParams{ID: sessionID, UserID: userID, Reason: &reason})
		if err != nil {
			return fmt.Errorf("revoke session: %w", err)
		}
		if n == 0 {
			return ErrSessionNotFound
		}
		return s.record(ctx, r, &userID, eventSessionRevoked, "", meta, map[string]any{"session_id": sessionID})
	})
}

// ListEvents returns the user's 50 most recent security events.
func (s *Service) ListEvents(ctx context.Context, userID uuid.UUID) ([]Event, error) {
	rows, err := newRepository(s.pool).ListAuthEvents(ctx, &userID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	out := make([]Event, 0, len(rows))
	for _, row := range rows {
		device, _ := describeDevice(clientWeb, row.UserAgent, "")
		e := Event{ID: row.ID, Event: row.Event, IP: ipString(row.Ip), Device: device, CreatedAt: row.CreatedAt}
		if row.Provider != nil {
			e.Provider = *row.Provider
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *Service) startSession(ctx context.Context, r repository, userID uuid.UUID, provider string, meta RequestMeta, now time.Time) (Tokens, error) {
	sessionID, err := uuid.NewV7()
	if err != nil {
		return Tokens{}, fmt.Errorf("session id: %w", err)
	}
	client := meta.Client
	if client != clientMobile {
		client = clientWeb
	}
	name, kind := describeDevice(client, meta.UserAgent, meta.DeviceHeader)

	if err := r.CreateSession(ctx, gen.CreateSessionParams{
		ID: sessionID, UserID: userID, Provider: provider, Client: client,
		UserAgent: truncate(meta.UserAgent, 512), DeviceName: name, DeviceType: kind,
		Ip: parseIP(meta.IP), Now: now, ExpiresAt: s.sessionExpiry(now, now),
	}); err != nil {
		return Tokens{}, fmt.Errorf("create session: %w", err)
	}
	refresh, refreshExp, err := s.newRefreshToken(ctx, r, sessionID, s.sessionExpiry(now, now))
	if err != nil {
		return Tokens{}, err
	}
	access, accessExp, err := s.tokens.Issue(userID, sessionID, now)
	if err != nil {
		return Tokens{}, fmt.Errorf("issue access token: %w", err)
	}
	return Tokens{
		Client: client, AccessToken: access, AccessExpiresAt: accessExp,
		RefreshToken: refresh, RefreshExpiresAt: refreshExp,
	}, nil
}

// sessionExpiry is when a session started at created ends if refreshed at now:
// RefreshTTL after the last refresh, but never past SessionMaxAge.
func (s *Service) sessionExpiry(created, now time.Time) time.Time {
	exp := now.Add(s.cfg.RefreshTTL)
	if s.cfg.SessionMaxAge > 0 {
		if limit := created.Add(s.cfg.SessionMaxAge); limit.Before(exp) {
			return limit
		}
	}
	return exp
}

func (s *Service) newRefreshToken(ctx context.Context, r repository, sessionID uuid.UUID, exp time.Time) (string, time.Time, error) {
	raw, hash, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("refresh token id: %w", err)
	}
	if err := r.CreateRefreshToken(ctx, gen.CreateRefreshTokenParams{ID: id, SessionID: sessionID, TokenHash: hash, ExpiresAt: exp}); err != nil {
		return "", time.Time{}, fmt.Errorf("create refresh token: %w", err)
	}
	return raw, exp, nil
}
