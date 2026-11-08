package api

import (
	"context"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/user"
)

// personaliseConfigResult adjusts the configuration result for the current
// user: credentials and api key are those of the current user.
func personaliseConfigResult(ctx context.Context, ret *ConfigResult) {
	users := manager.GetInstance().Users
	if !users.HasUsers() {
		return
	}

	u, ok := users.ByID(user.CurrentID(ctx))
	if !ok {
		return
	}

	if ret.General != nil {
		ret.General.Username = u.Username
		ret.General.Password = u.PasswordHash
		ret.General.APIKey = u.APIKey
	}
}

// updateCurrentUserCredentials applies a username/password change from the
// legacy general settings to the current user.
func updateCurrentUserCredentials(ctx context.Context, username *string, password *string) error {
	users := manager.GetInstance().Users
	u, ok := users.ByID(user.CurrentID(ctx))
	if !ok {
		return nil
	}

	input := manager.UserUpdateInput{ID: u.ID}
	changed := false

	if username != nil && *username != "" && *username != u.Username {
		input.Username = username
		changed = true
	}

	// the UI round-trips the stored hash; only set when different
	if password != nil && *password != "" && *password != u.PasswordHash {
		input.Password = password
		changed = true
	}

	if !changed {
		return nil
	}

	if _, err := users.Update(ctx, input); err != nil {
		return err
	}

	logger.Infof("Credentials of user %q changed", u.Username)
	return nil
}
