package api

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/utils"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/user"
)

// personaliseConfigResult adjusts the configuration result for the current
// user: credentials and api key are those of the current user, and server
// internals are hidden from non-admin users.
func personaliseConfigResult(ctx context.Context, ret *ConfigResult) {
	users := manager.GetInstance().Users
	if !users.HasUsers() {
		return
	}

	current := user.Current(ctx)
	u, ok := users.ByID(current.ID)
	if !ok {
		return
	}

	ret.UI = getUIConfig(ctx)

	if ret.General != nil {
		ret.General.Username = u.Username
		ret.General.Password = u.PasswordHash
		ret.General.APIKey = u.APIKey

		if !current.IsAdmin() {
			redactConfigGeneral(ret.General)
		}
	}
}

// redactConfigGeneral removes server paths and secrets from the general
// configuration for non-admin users.
func redactConfigGeneral(g *ConfigGeneralResult) {
	g.DatabasePath = ""
	g.BackupDirectoryPath = ""
	g.DeleteTrashPath = ""
	g.GeneratedPath = ""
	g.MetadataPath = ""
	g.ConfigFilePath = ""
	g.ScrapersPath = ""
	g.PluginsPath = ""
	g.CachePath = ""
	g.BlobsPath = ""
	g.FfmpegPath = ""
	g.FfprobePath = ""
	g.PythonPath = ""
	g.LogFile = nil

	// keep stash-box endpoints so that editors can submit, but hide keys
	boxes := make([]*models.StashBox, len(g.StashBoxes))
	for i, b := range g.StashBoxes {
		redacted := *b
		redacted.APIKey = ""
		boxes[i] = &redacted
	}
	g.StashBoxes = boxes
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

// usesGlobalUIConfig returns true if the current user reads and writes the
// global UI configuration. This is the case when authentication is not
// configured, and for the primary admin user, whose settings act as the
// defaults for all other users.
func usesGlobalUIConfig(ctx context.Context) bool {
	return !manager.GetInstance().Users.HasUsers() || user.CurrentID(ctx) == user.SystemUserID
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{}, len(m))
	for k, v := range m {
		if sub, ok := v.(map[string]interface{}); ok {
			v = copyMap(sub)
		}
		ret[k] = v
	}
	return ret
}

// getUIConfig returns the UI configuration of the current user: the global
// configuration overlaid with the user's own settings.
func getUIConfig(ctx context.Context) map[string]interface{} {
	global := config.GetInstance().GetUIConfiguration()
	if usesGlobalUIConfig(ctx) {
		return global
	}

	ret := copyMap(global)
	if u, ok := manager.GetInstance().Users.ByID(user.CurrentID(ctx)); ok && u.UIConfig != nil {
		utils.MergeMaps(ret, copyMap(u.UIConfig))
	}
	return ret
}

// setUIConfig replaces the UI configuration of the current user.
func setUIConfig(ctx context.Context, cfg map[string]interface{}) error {
	if usesGlobalUIConfig(ctx) {
		c := config.GetInstance()
		c.SetUIConfiguration(cfg)
		return c.Write()
	}

	return manager.GetInstance().Users.SetUIConfig(ctx, user.CurrentID(ctx), cfg)
}

// newSavedFilterOwner returns the owner of a newly created saved filter.
// Filters created by the primary admin (or when authentication is not
// configured) are shared with all users; filters of other users are private.
func newSavedFilterOwner(ctx context.Context) *int {
	if usesGlobalUIConfig(ctx) {
		return nil
	}
	id := user.CurrentID(ctx)
	return &id
}

// checkSavedFilterWritable returns the saved filter if the current user may
// change it: users may change their own filters, and admins shared filters.
func checkSavedFilterWritable(ctx context.Context, qb models.SavedFilterReader, id int) (*models.SavedFilter, error) {
	existing, err := qb.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("saved filter %d not found", id)
	}

	if existing.UserID == nil {
		if err := requireRole(ctx, user.RoleAdmin); err != nil {
			return nil, fmt.Errorf("%w: only admins may change shared filters", ErrForbidden)
		}
	} else if *existing.UserID != user.CurrentID(ctx) {
		return nil, fmt.Errorf("saved filter %d not found", id)
	}

	return existing, nil
}
