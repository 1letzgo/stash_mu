package sqlite

import (
	"context"
	"strconv"
	"strings"

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
	if !strings.Contains(query, currentUserToken) {
		return query
	}
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
}
