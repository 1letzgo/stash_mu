// Package user provides the notion of the current (authenticated) user and
// their role. It is deliberately dependency free so that it can be used from
// the data layer (pkg/sqlite) as well as from the API layer.
package user

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Role is the permission level of a user.
type Role string

const (
	// RoleViewer may browse and play content, and manage their own
	// ratings, play history, o-counts, saved filters and UI settings.
	RoleViewer Role = "VIEWER"
	// RoleEditor may additionally edit metadata, but may not change server
	// settings, run tasks or delete files.
	RoleEditor Role = "EDITOR"
	// RoleAdmin has unrestricted access.
	RoleAdmin Role = "ADMIN"
)

var AllRoles = []Role{RoleViewer, RoleEditor, RoleAdmin}

func (r Role) level() int {
	switch r {
	case RoleViewer:
		return 1
	case RoleEditor:
		return 2
	case RoleAdmin:
		return 3
	}
	return 0
}

// IsValid returns true if r is a known role.
func (r Role) IsValid() bool {
	return r.level() > 0
}

// AtLeast returns true if r grants at least the permissions of o.
func (r Role) AtLeast(o Role) bool {
	return r.level() >= o.level() && r.IsValid()
}

func (r Role) String() string {
	return string(r)
}

// ParseRole parses a role name case-insensitively.
func ParseRole(s string) (Role, error) {
	r := Role(strings.ToUpper(strings.TrimSpace(s)))
	if !r.IsValid() {
		return "", fmt.Errorf("invalid role %q", s)
	}
	return r, nil
}

// SystemUserID is the id of the user that owns all per-user data when
// no users have been configured, and that background tasks act as.
// The first user created always receives this id.
const SystemUserID = 1

// User is the authenticated principal of a request.
type User struct {
	ID       int
	Username string
	Role     Role
}

// IsAdmin returns true if the user has the admin role.
func (u User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// System is the principal used when authentication is not configured, and for
// background operations that are not associated with a request.
var System = User{
	ID:   SystemUserID,
	Role: RoleAdmin,
}

type ctxKey struct{}

// WithUser returns a context carrying the provided user.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

// FromContext returns the user stored in the context, if any.
func FromContext(ctx context.Context) (User, bool) {
	if ctx == nil {
		return User{}, false
	}
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// Current returns the user stored in the context, or the System user if none
// is present.
func Current(ctx context.Context) User {
	if u, ok := FromContext(ctx); ok {
		return u
	}
	return System
}

// CurrentID returns the id of the current user (see Current).
func CurrentID(ctx context.Context) int {
	return Current(ctx).ID
}

// UnmarshalGQL implements the graphql.Unmarshaler interface.
func (r *Role) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*r = Role(str)
	if !r.IsValid() {
		return fmt.Errorf("%s is not a valid UserRole", str)
	}
	return nil
}

// MarshalGQL implements the graphql.Marshaler interface.
func (r Role) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(r.String()))
}
