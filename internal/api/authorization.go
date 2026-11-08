package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/stashapp/stash/pkg/user"
)

// ErrForbidden is returned when the current user lacks the permissions for
// an operation.
var ErrForbidden = errors.New("forbidden")

// requireRole returns an error if the current user does not have at least the
// provided role.
func requireRole(ctx context.Context, role user.Role) error {
	u := user.Current(ctx)
	if !u.Role.AtLeast(role) {
		return fmt.Errorf("%w: requires role %s", ErrForbidden, role)
	}
	return nil
}
