package team

import (
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// systemRole is one of the four roles every workspace starts with.
type systemRole struct {
	key, name, description, color, scope string
	perms                                []authctx.Permission
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

func systemRoleDefaults(key string) (systemRole, bool) {
	for _, r := range systemRoles {
		if r.key == key {
			return r, true
		}
	}
	return systemRole{}, false
}

func keysOf(perms []authctx.Permission) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = p.Key()
	}
	return out
}
