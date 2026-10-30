// Package dto holds billing's HTTP request and response shapes and the converters from domain
// types. Amounts are integer minor units (cents): `amount_minor: 2900` is $29.00.
package dto

import (
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

type WorkspaceInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
}

func providerNames(ps []payment.Provider) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, string(p))
	}
	return out
}

// nonNil turns a nil list into an empty one, so the JSON is [] rather than null.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// nonEmpty turns "" into nil for optional response fields.
func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
