//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/user"
	"github.com/stretchr/testify/assert"
)

const (
	userIdxAdmin = iota
	userIdxViewer
	totalUsers
)

var userIDs []int

var userNames = []string{"admin", "viewer"}
var userRoles = []user.Role{user.RoleAdmin, user.RoleViewer}

func createUsers(ctx context.Context) error {
	now := time.Now()
	for i := 0; i < totalUsers; i++ {
		u := &models.User{
			Username:     userNames[i],
			PasswordHash: "hash",
			Role:         userRoles[i],
			APIKey:       "key-" + userNames[i],
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := db.User.Create(ctx, u); err != nil {
			return err
		}
		userIDs = append(userIDs, u.ID)
	}
	return nil
}

// viewerCtx returns a context acting as the (non-system) viewer user.
func viewerCtx(ctx context.Context) context.Context {
	return user.WithUser(ctx, user.User{ID: userIDs[userIdxViewer], Username: userNames[userIdxViewer], Role: user.RoleViewer})
}

func TestUserFirstIsSystemUser(t *testing.T) {
	assert.Equal(t, user.SystemUserID, userIDs[userIdxAdmin])
}

func TestUserFind(t *testing.T) {
	withTxn(func(ctx context.Context) error {
		u, err := db.User.FindByUsername(ctx, "VIEWER")
		assert.NoError(t, err)
		if assert.NotNil(t, u) {
			assert.Equal(t, userIDs[userIdxViewer], u.ID)
			assert.Equal(t, user.RoleViewer, u.Role)
			assert.Equal(t, "key-viewer", u.APIKey)
		}

		missing, err := db.User.FindByUsername(ctx, "nobody")
		assert.NoError(t, err)
		assert.Nil(t, missing)

		n, err := db.User.Count(ctx)
		assert.NoError(t, err)
		assert.Equal(t, totalUsers, n)
		return nil
	})
}

func TestUserCreateUpdateDestroy(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		now := time.Now()
		u := &models.User{Username: "temp", PasswordHash: "x", Role: user.RoleEditor, CreatedAt: now, UpdatedAt: now}
		if !assert.NoError(t, db.User.Create(ctx, u)) {
			return nil
		}

		// usernames are unique case-insensitively
		dup := &models.User{Username: "TEMP", PasswordHash: "x", Role: user.RoleViewer, CreatedAt: now, UpdatedAt: now}
		assert.Error(t, db.User.Create(ctx, dup))

		u.Role = user.RoleViewer
		u.UIConfig = map[string]interface{}{"foo": "bar"}
		assert.NoError(t, db.User.Update(ctx, u))

		got, err := db.User.Find(ctx, u.ID)
		assert.NoError(t, err)
		assert.Equal(t, user.RoleViewer, got.Role)
		assert.Equal(t, "bar", got.UIConfig["foo"])
		assert.Equal(t, "", got.APIKey)

		assert.NoError(t, db.User.Destroy(ctx, u.ID))
		got, err = db.User.Find(ctx, u.ID)
		assert.NoError(t, err)
		assert.Nil(t, got)
		return nil
	})
}
