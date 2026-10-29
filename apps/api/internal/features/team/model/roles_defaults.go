package model

import (
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// SystemRole is one of the four roles every workspace starts with.
type SystemRole struct {
	Key, Name, Description, Color, Scope string
	Perms                                []authctx.Permission
}

func allExcept(skip authctx.Permission) []authctx.Permission {
	var out []authctx.Permission
	for _, p := range authctx.AllPermissions() {
		if p != skip {
			out = append(out, p)
		}
	}
	return out
}

// SystemRoleDefaults returns the default role for a system role key.
func SystemRoleDefaults(key string) (SystemRole, bool) {
	for _, r := range SystemRoles {
		if r.Key == key {
			return r, true
		}
	}
	return SystemRole{}, false
}

// KeysOf returns the permission keys as strings.
func KeysOf(perms []authctx.Permission) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = p.Key()
	}
	return out
}
