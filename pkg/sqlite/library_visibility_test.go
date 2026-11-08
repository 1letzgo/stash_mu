//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/user"
)

func TestLibraryVisibility(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		now := time.Now()

		mkFolder := func(path string) models.FolderID {
			f := &models.Folder{Path: path, DirEntry: models.DirEntry{ModTime: now}}
			if err := db.Folder.Create(ctx, f); err != nil {
				t.Fatal(err)
			}
			return f.ID
		}
		mkVideo := func(folder models.FolderID, name string) models.FileID {
			f := &models.VideoFile{BaseFile: &models.BaseFile{Basename: name, ParentFolderID: folder, DirEntry: models.DirEntry{ModTime: now}}}
			if err := db.File.Create(ctx, f); err != nil {
				t.Fatal(err)
			}
			return f.ID
		}
		mkImageFile := func(folder models.FolderID, name string) models.FileID {
			f := &models.ImageFile{BaseFile: &models.BaseFile{Basename: name, ParentFolderID: folder, DirEntry: models.DirEntry{ModTime: now}}}
			if err := db.File.Create(ctx, f); err != nil {
				t.Fatal(err)
			}
			return f.ID
		}

		folderA := mkFolder("/libtestA")
		folderB := mkFolder("/libtestB")
		// prefix of library A, but not inside it
		folderAX := mkFolder("/libtestAX")

		sceneA := &models.Scene{Title: "libtest scene A"}
		assert.NoError(t, db.Scene.Create(ctx, sceneA, []models.FileID{mkVideo(folderA, "a.mp4")}))
		sceneB := &models.Scene{Title: "libtest scene B"}
		assert.NoError(t, db.Scene.Create(ctx, sceneB, []models.FileID{mkVideo(folderB, "b.mp4")}))
		sceneAX := &models.Scene{Title: "libtest scene AX"}
		assert.NoError(t, db.Scene.Create(ctx, sceneAX, []models.FileID{mkVideo(folderAX, "ax.mp4")}))

		imageB := &models.Image{Title: "libtest image B"}
		assert.NoError(t, db.Image.Create(ctx, &models.CreateImageInput{Image: imageB, FileIDs: []models.FileID{mkImageFile(folderB, "b.jpg")}}))

		performerB := models.NewPerformer()
		performerB.Name = "libtest performer B"
		assert.NoError(t, db.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &performerB}))
		_, err := db.Scene.UpdatePartial(ctx, sceneB.ID, models.ScenePartial{
			PerformerIDs: &models.UpdateIDs{IDs: []int{performerB.ID}, Mode: models.RelationshipUpdateModeSet},
		})
		assert.NoError(t, err)

		restricted := user.WithUser(ctx, user.User{
			ID: userIDs[userIdxViewer], Username: "viewer", Role: user.RoleViewer,
			Libraries: []string{"/libtestA"},
		})

		// find by id
		s, err := db.Scene.Find(restricted, sceneA.ID)
		assert.NoError(t, err)
		assert.NotNil(t, s)
		s, err = db.Scene.Find(restricted, sceneB.ID)
		assert.NoError(t, err)
		assert.Nil(t, s, "scene in other library must not be visible")
		s, err = db.Scene.Find(restricted, sceneAX.ID)
		assert.NoError(t, err)
		assert.Nil(t, s, "library path prefix must not match sibling folders")

		many, err := db.Scene.FindMany(restricted, []int{sceneA.ID, sceneB.ID})
		assert.NoError(t, err)
		assert.Equal(t, sceneA.ID, many[0].ID)
		assert.Nil(t, many[1])

		img, err := db.Image.Find(restricted, imageB.ID)
		assert.NoError(t, err)
		assert.Nil(t, img)

		// queries and counts
		q := "libtest"
		res, err := db.Scene.Query(restricted, models.SceneQueryOptions{
			QueryOptions: models.QueryOptions{FindFilter: &models.FindFilterType{Q: &q}, Count: true},
		})
		assert.NoError(t, err)
		assert.Equal(t, []int{sceneA.ID}, res.IDs)
		assert.Equal(t, 1, res.Count)

		res, err = db.Scene.Query(ctx, models.SceneQueryOptions{
			QueryOptions: models.QueryOptions{FindFilter: &models.FindFilterType{Q: &q}, Count: true},
		})
		assert.NoError(t, err)
		assert.Equal(t, 3, res.Count, "admin sees everything")

		n, err := db.Scene.CountByPerformerID(restricted, performerB.ID)
		assert.NoError(t, err)
		assert.Equal(t, 0, n)

		// performers without visible content are hidden from lists
		pq := "libtest performer"
		performers, _, err := db.Performer.Query(restricted, nil, &models.FindFilterType{Q: &pq})
		assert.NoError(t, err)
		assert.Len(t, performers, 0)
		performers, _, err = db.Performer.Query(ctx, nil, &models.FindFilterType{Q: &pq})
		assert.NoError(t, err)
		assert.Len(t, performers, 1)

		// writes to hidden objects are rejected
		title := "changed"
		_, err = db.Scene.UpdatePartial(restricted, sceneB.ID, models.ScenePartial{Title: models.NewOptionalString(title)})
		assert.Error(t, err)
		assert.Error(t, db.Scene.Destroy(restricted, sceneB.ID))

		// admins are never restricted
		adminWithLibs := user.WithUser(ctx, user.User{ID: 1, Role: user.RoleAdmin, Libraries: []string{"/libtestA"}})
		s, err = db.Scene.Find(adminWithLibs, sceneB.ID)
		assert.NoError(t, err)
		assert.NotNil(t, s)

		// users without libraries are not restricted
		s, err = db.Scene.Find(viewerCtx(ctx), sceneB.ID)
		assert.NoError(t, err)
		assert.NotNil(t, s)
		return nil
	})
}
