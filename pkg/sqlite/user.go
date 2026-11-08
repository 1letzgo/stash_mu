package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
	"gopkg.in/guregu/null.v4/zero"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/user"
)

const (
	userTable = "users"
)

type userRow struct {
	ID           int         `db:"id" goqu:"skipinsert"`
	Username     string      `db:"username"`
	PasswordHash string      `db:"password_hash"`
	Role         string      `db:"role"`
	APIKey       zero.String `db:"api_key"`
	UIConfig     zero.String `db:"ui_config"`
	Libraries    zero.String `db:"libraries"`
	CreatedAt    Timestamp   `db:"created_at"`
	UpdatedAt    Timestamp   `db:"updated_at"`
}

func (r *userRow) fromUser(o models.User) {
	r.ID = o.ID
	r.Username = o.Username
	r.PasswordHash = o.PasswordHash
	r.Role = string(o.Role)
	r.APIKey = zero.StringFrom(o.APIKey)
	r.UIConfig = zero.StringFrom("")
	if len(o.UIConfig) > 0 {
		r.UIConfig = zero.StringFrom(encodeJSONOrEmpty(o.UIConfig))
	}
	r.Libraries = zero.StringFrom("")
	if len(o.Libraries) > 0 {
		r.Libraries = zero.StringFrom(encodeJSONOrEmpty(o.Libraries))
	}
	r.CreatedAt = Timestamp{Timestamp: o.CreatedAt}
	r.UpdatedAt = Timestamp{Timestamp: o.UpdatedAt}
}

func (r *userRow) resolve() *models.User {
	ret := &models.User{
		ID:           r.ID,
		Username:     r.Username,
		PasswordHash: r.PasswordHash,
		Role:         user.Role(r.Role),
		APIKey:       r.APIKey.String,
		CreatedAt:    r.CreatedAt.Timestamp,
		UpdatedAt:    r.UpdatedAt.Timestamp,
	}

	if r.Libraries.String != "" {
		decodeJSON(r.Libraries.String, &ret.Libraries)
	}

	if r.UIConfig.String != "" {
		ret.UIConfig = make(map[string]interface{})
		decodeJSON(r.UIConfig.String, &ret.UIConfig)
	}

	return ret
}

type UserStore struct {
	repository
	tableMgr *table
}

func NewUserStore() *UserStore {
	return &UserStore{
		repository: repository{
			tableName: userTable,
			idColumn:  idColumn,
		},
		tableMgr: userTableMgr,
	}
}

func (qb *UserStore) table() exp.IdentifierExpression {
	return qb.tableMgr.table
}

func (qb *UserStore) selectDataset() *goqu.SelectDataset {
	return dialect.From(qb.table()).Select(qb.table().All())
}

// Create inserts a new user. The first user created always receives
// user.SystemUserID, so that it owns pre-existing per-user data.
func (qb *UserStore) Create(ctx context.Context, newObject *models.User) error {
	var r userRow
	r.fromUser(*newObject)

	id, err := qb.tableMgr.insertID(ctx, r)
	if err != nil {
		return err
	}

	updated, err := qb.Find(ctx, id)
	if err != nil {
		return fmt.Errorf("finding after create: %w", err)
	}

	*newObject = *updated

	return nil
}

// CreateWithID inserts a user keeping its id. Used to restore users after
// the database has been reset.
func (qb *UserStore) CreateWithID(ctx context.Context, u *models.User) error {
	var r userRow
	r.fromUser(*u)

	q := dialect.Insert(qb.table()).Rows(goqu.Record{
		"id":            u.ID,
		"username":      r.Username,
		"password_hash": r.PasswordHash,
		"role":          r.Role,
		"api_key":       r.APIKey,
		"ui_config":     r.UIConfig,
		"libraries":     r.Libraries,
		"created_at":    r.CreatedAt,
		"updated_at":    r.UpdatedAt,
	})
	if _, err := exec(ctx, q); err != nil {
		return fmt.Errorf("restoring user %d: %w", u.ID, err)
	}
	return nil
}

func (qb *UserStore) Update(ctx context.Context, updatedObject *models.User) error {
	var r userRow
	r.fromUser(*updatedObject)

	return qb.tableMgr.updateByID(ctx, updatedObject.ID, r)
}

// Destroy removes the user and all of their per-user data.
func (qb *UserStore) Destroy(ctx context.Context, id int) error {
	for _, t := range perUserTables {
		stmt := fmt.Sprintf("DELETE FROM %s WHERE user_id = ?", t)
		if _, err := dbWrapper.Exec(ctx, stmt, id); err != nil {
			return fmt.Errorf("deleting user data from %s: %w", t, err)
		}
	}

	return qb.destroyExisting(ctx, []int{id})
}

// returns nil, nil if not found
func (qb *UserStore) Find(ctx context.Context, id int) (*models.User, error) {
	ret, err := qb.get(ctx, qb.selectDataset().Where(qb.tableMgr.byID(id)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

// returns nil, nil if not found
func (qb *UserStore) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	q := qb.selectDataset().Prepared(true).Where(
		goqu.L("username = ? COLLATE NOCASE", strings.TrimSpace(username)),
	)
	ret, err := qb.get(ctx, q)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ret, err
}

func (qb *UserStore) All(ctx context.Context) ([]*models.User, error) {
	return qb.getMany(ctx, qb.selectDataset().Order(qb.table().Col(idColumn).Asc()))
}

func (qb *UserStore) Count(ctx context.Context) (int, error) {
	return count(ctx, dialect.From(qb.table()).Select(goqu.COUNT("*")))
}

func (qb *UserStore) get(ctx context.Context, q *goqu.SelectDataset) (*models.User, error) {
	ret, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}

	if len(ret) == 0 {
		return nil, sql.ErrNoRows
	}

	return ret[0], nil
}

func (qb *UserStore) getMany(ctx context.Context, q *goqu.SelectDataset) ([]*models.User, error) {
	const single = false
	var ret []*models.User
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var f userRow
		if err := r.StructScan(&f); err != nil {
			return err
		}

		ret = append(ret, f.resolve())
		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}
