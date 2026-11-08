package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/doug-martin/goqu/v9"

	"github.com/stashapp/stash/pkg/user"
)

// Library visibility
//
// Users may be restricted to specific libraries (stash paths). The SQL
// fragments below decide whether an object is visible to the current user.
// They are written as tokens in statically defined SQL and substituted per
// request by bindCurrentUser: for unrestricted users each token becomes "1".
//
// Scenes and images are visible if they have a file inside one of the
// user's libraries (scenes without files are visible). Galleries are visible
// if their folder or zip file is inside a library; galleries without folder
// and files are visible if they are empty or contain a visible image.
// Markers and chapters follow their scene and gallery. Performers, studios,
// tags and groups are visible if they are linked to visible content.

const (
	sceneVisibleToken     = "__STASH_SCENE_VISIBLE__"
	imageVisibleToken     = "__STASH_IMAGE_VISIBLE__"
	galleryVisibleToken   = "__STASH_GALLERY_VISIBLE__"
	markerVisibleToken    = "__STASH_MARKER_VISIBLE__"
	chapterVisibleToken   = "__STASH_CHAPTER_VISIBLE__"
	performerVisibleToken = "__STASH_PERFORMER_VISIBLE__"
	studioVisibleToken    = "__STASH_STUDIO_VISIBLE__"
	tagVisibleToken       = "__STASH_TAG_VISIBLE__"
	groupVisibleToken     = "__STASH_GROUP_VISIBLE__"
	folderVisibleToken    = "__STASH_FOLDER_VISIBLE__"
	fileVisibleToken      = "__STASH_FILE_VISIBLE__"

	visibilityTokenPrefix = "__STASH_"
)

// visibilityTokens maps table names to their visibility token.
var visibilityTokens = map[string]string{
	sceneTable:             sceneVisibleToken,
	imageTable:             imageVisibleToken,
	galleryTable:           galleryVisibleToken,
	sceneMarkerTable:       markerVisibleToken,
	galleriesChaptersTable: chapterVisibleToken,
	performerTable:         performerVisibleToken,
	studioTable:            studioVisibleToken,
	tagTable:               tagVisibleToken,
	groupTable:             groupVisibleToken,
	folderTable:            folderVisibleToken,
	fileTable:              fileVisibleToken,
}

// visibleCondition returns the visibility condition for the table, or "" if
// the table is not subject to library restrictions.
func visibleCondition(table string) string {
	return visibilityTokens[table]
}

// libraryRestricted returns true if the current user is restricted to
// specific libraries.
func libraryRestricted(ctx context.Context) bool {
	return user.Current(ctx).LibraryRestricted()
}

func sqlString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func normaliseLibraryPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = path.Clean(strings.ReplaceAll(p, "\\", "/"))
	return p
}

// pathInLibraries returns a condition that is true if the path column is one
// of the library paths or inside one.
func pathInLibraries(col string, libraries []string) string {
	var conds []string
	for _, l := range libraries {
		l = normaliseLibraryPath(l)
		if l == "" {
			continue
		}
		if l == "/" {
			return "1"
		}
		prefix := l + "/"
		conds = append(conds, "("+col+" = "+sqlString(l)+" OR substr("+col+", 1, "+itoa(len(prefix))+") = "+sqlString(prefix)+")")
	}
	if len(conds) == 0 {
		return "0"
	}
	return "(" + strings.Join(conds, " OR ") + ")"
}

// folderIsLibraryAncestor returns a condition that is true if the folder path
// column is a parent folder of one of the libraries, so that the libraries
// can be navigated to.
func folderIsLibraryAncestor(col string, libraries []string) string {
	var conds []string
	for _, l := range libraries {
		l = normaliseLibraryPath(l)
		if l == "" {
			continue
		}
		conds = append(conds, "substr("+sqlString(l+"/")+", 1, length("+col+") + 1) = "+col+" || '/'")
	}
	if len(conds) == 0 {
		return "0"
	}
	return "(" + strings.Join(conds, " OR ") + ")"
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// visibilityConditions builds the visibility conditions for the libraries.
func visibilityConditions(libraries []string) map[string]string {
	inLib := func(col string) string { return pathInLibraries(col, libraries) }

	scene := "(NOT EXISTS (SELECT 1 FROM scenes_files v_sf0 WHERE v_sf0.scene_id = scenes.id) OR EXISTS (" +
		"SELECT 1 FROM scenes_files v_sf INNER JOIN files v_f ON v_f.id = v_sf.file_id " +
		"INNER JOIN folders v_fo ON v_fo.id = v_f.parent_folder_id " +
		"WHERE v_sf.scene_id = scenes.id AND " + inLib("v_fo.path") + "))"

	image := "(NOT EXISTS (SELECT 1 FROM images_files v_if0 WHERE v_if0.image_id = images.id) OR EXISTS (" +
		"SELECT 1 FROM images_files v_if INNER JOIN files v_f ON v_f.id = v_if.file_id " +
		"INNER JOIN folders v_fo ON v_fo.id = v_f.parent_folder_id " +
		"WHERE v_if.image_id = images.id AND " + inLib("v_fo.path") + "))"

	gallery := "(EXISTS (SELECT 1 FROM folders v_gfo WHERE v_gfo.id = galleries.folder_id AND " + inLib("v_gfo.path") + ")" +
		" OR EXISTS (SELECT 1 FROM galleries_files v_gf INNER JOIN files v_f ON v_f.id = v_gf.file_id " +
		"INNER JOIN folders v_fo ON v_fo.id = v_f.parent_folder_id " +
		"WHERE v_gf.gallery_id = galleries.id AND " + inLib("v_fo.path") + ")" +
		" OR (galleries.folder_id IS NULL AND NOT EXISTS (SELECT 1 FROM galleries_files v_gf0 WHERE v_gf0.gallery_id = galleries.id)" +
		" AND (NOT EXISTS (SELECT 1 FROM galleries_images v_gi0 WHERE v_gi0.gallery_id = galleries.id)" +
		" OR EXISTS (SELECT 1 FROM galleries_images v_gi INNER JOIN images ON images.id = v_gi.image_id WHERE v_gi.gallery_id = galleries.id AND " + image + "))))"

	marker := "EXISTS (SELECT 1 FROM scenes WHERE scenes.id = scene_markers.scene_id AND " + scene + ")"
	chapter := "EXISTS (SELECT 1 FROM galleries WHERE galleries.id = galleries_chapters.gallery_id AND " + gallery + ")"

	performer := "(EXISTS (SELECT 1 FROM performers_scenes v_ps INNER JOIN scenes ON scenes.id = v_ps.scene_id WHERE v_ps.performer_id = performers.id AND " + scene + ")" +
		" OR EXISTS (SELECT 1 FROM performers_images v_pi INNER JOIN images ON images.id = v_pi.image_id WHERE v_pi.performer_id = performers.id AND " + image + ")" +
		" OR EXISTS (SELECT 1 FROM performers_galleries v_pg INNER JOIN galleries ON galleries.id = v_pg.gallery_id WHERE v_pg.performer_id = performers.id AND " + gallery + "))"

	studioContent := func(studioCol string) string {
		return "EXISTS (SELECT 1 FROM scenes WHERE scenes.studio_id = " + studioCol + " AND " + scene + ")" +
			" OR EXISTS (SELECT 1 FROM images WHERE images.studio_id = " + studioCol + " AND " + image + ")" +
			" OR EXISTS (SELECT 1 FROM galleries WHERE galleries.studio_id = " + studioCol + " AND " + gallery + ")"
	}
	studio := "(" + studioContent("studios.id") +
		" OR EXISTS (SELECT 1 FROM studios v_cs WHERE v_cs.parent_id = studios.id AND (" + studioContent("v_cs.id") + ")))"

	tagContent := func(tagCol string) string {
		return "EXISTS (SELECT 1 FROM scenes_tags v_st INNER JOIN scenes ON scenes.id = v_st.scene_id WHERE v_st.tag_id = " + tagCol + " AND " + scene + ")" +
			" OR EXISTS (SELECT 1 FROM images_tags v_it INNER JOIN images ON images.id = v_it.image_id WHERE v_it.tag_id = " + tagCol + " AND " + image + ")" +
			" OR EXISTS (SELECT 1 FROM galleries_tags v_gt INNER JOIN galleries ON galleries.id = v_gt.gallery_id WHERE v_gt.tag_id = " + tagCol + " AND " + gallery + ")" +
			" OR EXISTS (SELECT 1 FROM scene_markers WHERE scene_markers.primary_tag_id = " + tagCol + " AND " + marker + ")" +
			" OR EXISTS (SELECT 1 FROM scene_markers_tags v_smt INNER JOIN scene_markers ON scene_markers.id = v_smt.scene_marker_id WHERE v_smt.tag_id = " + tagCol + " AND " + marker + ")"
	}
	tag := "(" + tagContent("tags.id") +
		" OR EXISTS (SELECT 1 FROM tags_relations v_tr WHERE v_tr.parent_id = tags.id AND (" + tagContent("v_tr.child_id") + ")))"

	group := "EXISTS (SELECT 1 FROM groups_scenes v_gs INNER JOIN scenes ON scenes.id = v_gs.scene_id WHERE v_gs.group_id = groups.id AND " + scene + ")"

	folder := "(" + inLib("folders.path") + " OR " + folderIsLibraryAncestor("folders.path", libraries) + ")"
	file := "EXISTS (SELECT 1 FROM folders v_ffo WHERE v_ffo.id = files.parent_folder_id AND " + inLib("v_ffo.path") + ")"

	return map[string]string{
		sceneVisibleToken:     scene,
		imageVisibleToken:     image,
		galleryVisibleToken:   gallery,
		markerVisibleToken:    marker,
		chapterVisibleToken:   chapter,
		performerVisibleToken: performer,
		studioVisibleToken:    studio,
		tagVisibleToken:       tag,
		groupVisibleToken:     group,
		folderVisibleToken:    folder,
		fileVisibleToken:      file,
	}
}

// bindVisibility replaces the visibility tokens in the query.
func bindVisibility(ctx context.Context, query string) string {
	if !strings.Contains(query, visibilityTokenPrefix) {
		return query
	}

	u := user.Current(ctx)
	var conds map[string]string
	if u.LibraryRestricted() {
		conds = visibilityConditions(u.Libraries)
	}

	for _, token := range visibilityTokens {
		if !strings.Contains(query, token) {
			continue
		}
		replacement := "1"
		if conds != nil {
			replacement = conds[token]
		}
		query = strings.ReplaceAll(query, token, replacement)
	}
	return query
}

// ErrNotVisible is returned when modifying an object that is not visible to
// the current user.
var ErrNotVisible = errors.New("object not found")

// checkVisible returns an error if the object is not visible to the current
// user. Used to protect writes from users restricted to specific libraries.
func checkVisible(ctx context.Context, table string, id int) error {
	if !libraryRestricted(ctx) {
		return nil
	}

	cond := visibleCondition(table)
	if cond == "" {
		return nil
	}

	t := goqu.T(table)
	q := dialect.Select(goqu.COUNT("*")).From(t).Where(t.Col(idColumn).Eq(id), goqu.L(cond))
	n, err := count(ctx, q)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%s %d: %w", table, id, ErrNotVisible)
	}
	return nil
}
