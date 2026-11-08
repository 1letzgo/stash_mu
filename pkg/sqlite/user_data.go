package sqlite

// perUserTables lists the tables holding per-user data, keyed by a user_id
// column. Rows in these tables are removed when a user is destroyed.
var perUserTables = []string{}
