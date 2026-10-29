package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/repository"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
)

// Wrong passwords are counted from auth_events (login_failed rows that carry
// metadata.key), so the limits hold across instances and restarts.
//
// The key is an HMAC of the email, recorded for unknown emails too: a lockout
// then says nothing about which emails have accounts. Counting per IP as well
// as in total means one attacker can't lock the owner out from elsewhere.
const (
	passwordFailWindow = 15 * time.Minute
	passwordFailsPerIP = 5  // one IP guessing one account
	passwordFailsTotal = 50 // many IPs guessing one account
)

// passwordKey identifies an email in auth_events without storing it.
func (s *Service) passwordKey(email string) string {
	m := hmac.New(sha256.New, s.cfg.EmailCodeKey)
	m.Write([]byte("password-attempts:" + email))
	return hex.EncodeToString(m.Sum(nil)[:16])
}

// passwordBlocked reports whether key has had too many wrong passwords lately,
// from this IP or from everywhere. A successful sign-in starts the count again.
func (s *Service) passwordBlocked(ctx context.Context, key string, meta model.RequestMeta) (bool, error) {
	n, err := repository.NewRepository(s.pool).CountPasswordFailures(ctx, gen.CountPasswordFailuresParams{
		ClientIp: repository.ParseIP(meta.IP), Key: key, Since: s.now().Add(-passwordFailWindow),
	})
	if err != nil {
		return false, fmt.Errorf("count password failures: %w", err)
	}
	return n.FromIp >= passwordFailsPerIP || n.Total >= passwordFailsTotal, nil
}

// recordFailedLogin logs outside any transaction: a wrong guess must count even
// when the caller's transaction rolls back, and a logging failure must not change
// the answer. key is set for wrong passwords (counted) and empty otherwise.
func (s *Service) recordFailedLogin(ctx context.Context, userID *uuid.UUID, meta model.RequestMeta, reason, key string) {
	md := map[string]any{"reason": reason}
	if key != "" {
		md["key"] = key
	}
	if err := s.record(ctx, repository.NewRepository(s.pool), userID, model.EventLoginFailed, model.ProviderPassword, meta, md); err != nil {
		logger.FromContext(ctx).Error("record failed login", "err", err)
	}
}
