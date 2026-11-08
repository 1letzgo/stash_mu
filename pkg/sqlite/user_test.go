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

func TestUserScopedSceneHistory(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		qb := db.Scene
		sceneID := sceneIDs[sceneIdxWithGallery]
		viewer := viewerCtx(ctx)

		adminViews, err := qb.CountViews(ctx, sceneID)
		assert.NoError(t, err)
		adminO, err := qb.GetOCount(ctx, sceneID)
		assert.NoError(t, err)
		adminScene, err := qb.Find(ctx, sceneID)
		assert.NoError(t, err)

		// viewer records activity
		_, err = qb.AddViews(viewer, sceneID, nil)
		assert.NoError(t, err)
		_, err = qb.AddO(viewer, sceneID, []time.Time{time.Now(), time.Now()})
		assert.NoError(t, err)
		resume := 42.5
		duration := 10.0
		_, err = qb.SaveActivity(viewer, sceneID, &resume, &duration)
		assert.NoError(t, err)
		_, err = qb.SaveActivity(viewer, sceneID, nil, &duration)
		assert.NoError(t, err)

		// viewer sees their own values
		views, err := qb.CountViews(viewer, sceneID)
		assert.NoError(t, err)
		assert.Equal(t, 1, views)
		o, err := qb.GetOCount(viewer, sceneID)
		assert.NoError(t, err)
		assert.Equal(t, 2, o)
		s, err := qb.Find(viewer, sceneID)
		assert.NoError(t, err)
		assert.Equal(t, resume, s.ResumeTime)
		assert.Equal(t, 20.0, s.PlayDuration)

		// admin (system user) is unaffected
		views, err = qb.CountViews(ctx, sceneID)
		assert.NoError(t, err)
		assert.Equal(t, adminViews, views)
		o, err = qb.GetOCount(ctx, sceneID)
		assert.NoError(t, err)
		assert.Equal(t, adminO, o)
		s, err = qb.Find(ctx, sceneID)
		assert.NoError(t, err)
		assert.Equal(t, adminScene.ResumeTime, s.ResumeTime)
		assert.Equal(t, adminScene.PlayDuration, s.PlayDuration)

		// filtering by o_counter is per-user
		oCount := 2
		filter := &models.SceneFilterType{
			OCounter: &models.IntCriterionInput{Value: oCount, Modifier: models.CriterionModifierEquals},
		}
		ids, err := qb.Query(viewer, models.SceneQueryOptions{SceneFilter: filter})
		assert.NoError(t, err)
		assert.Contains(t, ids.IDs, sceneID)
		ids, err = qb.Query(ctx, models.SceneQueryOptions{SceneFilter: filter})
		assert.NoError(t, err)
		if adminO != oCount {
			assert.NotContains(t, ids.IDs, sceneID)
		}

		// deleting the user removes their data
		assert.NoError(t, db.User.Destroy(ctx, userIDs[userIdxViewer]))
		views, err = qb.CountViews(viewer, sceneID)
		assert.NoError(t, err)
		assert.Equal(t, 0, views)
		return nil
	})
}

func TestUserScopedRatings(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		viewer := viewerCtx(ctx)
		sceneID := sceneIDs[sceneIdxWithGallery]
		imageID := imageIDs[imageIdxWithGallery]

		adminScene, err := db.Scene.Find(ctx, sceneID)
		assert.NoError(t, err)
		adminImage, err := db.Image.Find(ctx, imageID)
		assert.NoError(t, err)

		rating := 77
		_, err = db.Scene.UpdatePartial(viewer, sceneID, models.ScenePartial{Rating: models.NewOptionalInt(rating)})
		assert.NoError(t, err)
		_, err = db.Image.IncrementOCounter(viewer, imageID)
		assert.NoError(t, err)
		_, err = db.Image.IncrementOCounter(viewer, imageID)
		assert.NoError(t, err)
		n, err := db.Image.DecrementOCounter(viewer, imageID)
		assert.NoError(t, err)
		assert.Equal(t, 1, n)

		s, err := db.Scene.Find(viewer, sceneID)
		assert.NoError(t, err)
		if assert.NotNil(t, s.Rating) {
			assert.Equal(t, rating, *s.Rating)
		}
		i, err := db.Image.Find(viewer, imageID)
		assert.NoError(t, err)
		assert.Equal(t, 1, i.OCounter)

		// admin unaffected
		s, err = db.Scene.Find(ctx, sceneID)
		assert.NoError(t, err)
		assert.Equal(t, adminScene.Rating, s.Rating)
		i, err = db.Image.Find(ctx, imageID)
		assert.NoError(t, err)
		assert.Equal(t, adminImage.OCounter, i.OCounter)

		// filter and sort by rating use the viewer's rating
		filter := &models.SceneFilterType{
			Rating100: &models.IntCriterionInput{Value: rating, Modifier: models.CriterionModifierEquals},
		}
		res, err := db.Scene.Query(viewer, models.SceneQueryOptions{SceneFilter: filter})
		assert.NoError(t, err)
		assert.Equal(t, []int{sceneID}, res.IDs)

		sort := "rating"
		dir := models.SortDirectionEnumDesc
		perPage := 1
		res, err = db.Scene.Query(viewer, models.SceneQueryOptions{
			QueryOptions: models.QueryOptions{FindFilter: &models.FindFilterType{Sort: &sort, Direction: &dir, PerPage: &perPage}},
		})
		assert.NoError(t, err)
		assert.Equal(t, []int{sceneID}, res.IDs)

		// clearing the rating
		_, err = db.Scene.UpdatePartial(viewer, sceneID, models.ScenePartial{Rating: models.NewOptionalIntPtr(nil)})
		assert.NoError(t, err)
		s, err = db.Scene.Find(viewer, sceneID)
		assert.NoError(t, err)
		assert.Nil(t, s.Rating)
		return nil
	})
}

func TestUserScopedSavedFilters(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		viewer := viewerCtx(ctx)
		viewerID := userIDs[userIdxViewer]

		shared, err := db.SavedFilter.FindByMode(ctx, models.FilterModeScenes)
		assert.NoError(t, err)

		f := &models.SavedFilter{Mode: models.FilterModeScenes, Name: "mine", UserID: &viewerID}
		assert.NoError(t, db.SavedFilter.Create(viewer, f))

		// the viewer sees shared filters and their own
		got, err := db.SavedFilter.FindByMode(viewer, models.FilterModeScenes)
		assert.NoError(t, err)
		assert.Len(t, got, len(shared)+1)

		// other users don't see it
		got, err = db.SavedFilter.FindByMode(ctx, models.FilterModeScenes)
		assert.NoError(t, err)
		assert.Len(t, got, len(shared))
		found, err := db.SavedFilter.Find(ctx, f.ID)
		assert.NoError(t, err)
		assert.Nil(t, found)

		// the same name may be used by another user
		other := &models.SavedFilter{Mode: models.FilterModeScenes, Name: "mine"}
		assert.NoError(t, db.SavedFilter.Create(ctx, other))
		return nil
	})
}
