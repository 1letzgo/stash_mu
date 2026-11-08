package sqlite

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/user"
)

// userIDColumn is the column holding the owning user in per-user tables.
const userIDColumn = "user_id"

// currentUserToken is substituted with the id of the current user (taken
// from the context) in all SQL executed through the database wrapper. This
// allows statically defined SQL fragments, such as sort and filter
// expressions, to refer to the current user.
const currentUserToken = "__STASH_CURRENT_USER_ID__"

// bindCurrentUser replaces currentUserToken in the query with the id of the
// current user.
func bindCurrentUser(ctx context.Context, query string) string {
	if !strings.Contains(query, visibilityTokenPrefix) {
		return query
	}
	// visibility conditions may themselves refer to the current user
	query = bindVisibility(ctx, query)
	return strings.ReplaceAll(query, currentUserToken, strconv.Itoa(user.CurrentID(ctx)))
}

// userScoped returns a derived table expression selecting the rows of a
// per-user table that belong to the current user. It can be used anywhere a
// table name is expected, provided it is given an alias where required.
func userScoped(table string) string {
	return "(SELECT * FROM " + table + " WHERE " + userIDColumn + " = " + currentUserToken + ")"
}

// userCondition returns an SQL condition restricting the qualified table to
// the current user.
func userCondition(table string) string {
	return table + "." + userIDColumn + " = " + currentUserToken
}

// perUserTables lists the tables holding per-user data, keyed by a user_id
// column. Rows in these tables are removed when a user is destroyed.
var perUserTables = []string{
	scenesViewDatesTable,
	scenesODatesTable,
	scenesUserActivityTable,
	"scene_ratings",
	"image_ratings",
	"gallery_ratings",
	"performer_ratings",
	"studio_ratings",
	"group_ratings",
	imagesOCountsTable,
}

// userRatingTable describes the table holding per-user ratings of an entity.
type userRatingTable struct {
	// entity table, e.g. scenes
	entity string
	// rating table, e.g. scene_ratings
	table string
	// foreign key column to the entity, e.g. scene_id
	fk string
}

var (
	sceneRatings     = userRatingTable{entity: sceneTable, table: "scene_ratings", fk: sceneIDColumn}
	imageRatings     = userRatingTable{entity: imageTable, table: "image_ratings", fk: imageIDColumn}
	galleryRatings   = userRatingTable{entity: galleryTable, table: "gallery_ratings", fk: galleryIDColumn}
	performerRatings = userRatingTable{entity: performerTable, table: "performer_ratings", fk: performerIDColumn}
	studioRatings    = userRatingTable{entity: studioTable, table: "studio_ratings", fk: studioIDColumn}
	groupRatings     = userRatingTable{entity: groupTable, table: "group_ratings", fk: groupIDColumn}

	userRatingTables = []userRatingTable{
		sceneRatings, imageRatings, galleryRatings, performerRatings, studioRatings, groupRatings,
	}
)

const (
	imagesOCountsTable = "images_o_counts"
	ratingColumn       = "rating"
	oCounterColumn     = "o_counter"
)

// expr returns an expression selecting the current user's rating of the
// entity in the outer query.
func (t userRatingTable) expr() string {
	return "(SELECT ur." + ratingColumn + " FROM " + t.table + " ur WHERE ur." + t.fk + " = " + t.entity + ".id AND ur." + userIDColumn + " = " + currentUserToken + ")"
}

// set sets (or clears, if rating is nil) the current user's rating.
func (t userRatingTable) set(ctx context.Context, id int, rating *int) error {
	if rating == nil {
		stmt := "DELETE FROM " + t.table + " WHERE " + userIDColumn + " = ? AND " + t.fk + " = ?"
		if _, err := dbWrapper.Exec(ctx, stmt, user.CurrentID(ctx), id); err != nil {
			return fmt.Errorf("clearing rating: %w", err)
		}
		return nil
	}

	stmt := "INSERT INTO " + t.table + " (" + userIDColumn + ", " + t.fk + ", " + ratingColumn + ") VALUES (?, ?, ?) " +
		"ON CONFLICT (" + userIDColumn + ", " + t.fk + ") DO UPDATE SET " + ratingColumn + " = excluded." + ratingColumn
	if _, err := dbWrapper.Exec(ctx, stmt, user.CurrentID(ctx), id, *rating); err != nil {
		return fmt.Errorf("setting rating: %w", err)
	}
	return nil
}

// setPartial applies a partial rating update.
func (t userRatingTable) setPartial(ctx context.Context, id int, v models.OptionalInt) error {
	if !v.Set {
		return nil
	}
	if v.Null {
		return t.set(ctx, id, nil)
	}
	value := v.Value
	return t.set(ctx, id, &value)
}

// imageOCounterExpr selects the current user's o-counter of the image in the
// outer query.
var imageOCounterExpr = "COALESCE((SELECT ioc." + oCounterColumn + " FROM " + imagesOCountsTable + " ioc WHERE ioc." + imageIDColumn + " = " + imageTable + ".id AND ioc." + userIDColumn + " = " + currentUserToken + "), 0)"

// perUserColumnExpr returns the expression to use in place of a per-user
// column of an entity table, if the column is per-user.
func perUserColumnExpr(table string, column string) (string, bool) {
	if column == ratingColumn {
		for _, t := range userRatingTables {
			if t.entity == table {
				return t.expr(), true
			}
		}
	}

	if table == imageTable && column == oCounterColumn {
		return imageOCounterExpr, true
	}

	return "", false
}

// ratingIsMissingClause returns the where clause for a missing rating.
func ratingIsMissingClause(table string) string {
	expr, _ := perUserColumnExpr(table, ratingColumn)
	return expr + " IS NULL"
}
