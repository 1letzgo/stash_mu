package sqlite

import (
	"context"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"

	"github.com/stashapp/stash/pkg/user"
)

// oCounterManager manages the per-user o-counter of images, stored in the
// images_o_counts table.
type oCounterManager struct {
	tableMgr *table
}

func (qb *oCounterManager) getOCounter(ctx context.Context, id int) (int, error) {
	t := goqu.T(imagesOCountsTable)
	q := dialect.From(t).Select(oCounterColumn).Where(
		t.Col(imageIDColumn).Eq(id),
		t.Col(userIDColumn).Eq(user.CurrentID(ctx)),
	)

	const single = true
	var ret int
	if err := queryFunc(ctx, q, single, func(rows *sqlx.Rows) error {
		if err := rows.Scan(&ret); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return 0, err
	}

	return ret, nil
}

// setOCounter sets the current user's o-counter of the image to the value
// computed by expr, which may refer to the current value as o_counter.
func (qb *oCounterManager) setOCounter(ctx context.Context, id int, expr string, args ...interface{}) error {
	stmt := fmt.Sprintf(
		"INSERT INTO %[1]s (user_id, image_id, o_counter) VALUES (?, ?, MAX(0, %[2]s)) "+
			"ON CONFLICT (user_id, image_id) DO UPDATE SET o_counter = MAX(0, %[3]s)",
		imagesOCountsTable,
		// for the insert there is no current value
		replaceOCounterRef(expr, "0"),
		replaceOCounterRef(expr, imagesOCountsTable+".o_counter"),
	)

	allArgs := append([]interface{}{user.CurrentID(ctx), id}, args...)
	allArgs = append(allArgs, args...)
	if _, err := dbWrapper.Exec(ctx, stmt, allArgs...); err != nil {
		return fmt.Errorf("updating image o-counter: %w", err)
	}
	return nil
}

const oCounterRef = "{o_counter}"

func replaceOCounterRef(expr string, with string) string {
	ret := ""
	for i := 0; i < len(expr); i++ {
		if i+len(oCounterRef) <= len(expr) && expr[i:i+len(oCounterRef)] == oCounterRef {
			ret += with
			i += len(oCounterRef) - 1
			continue
		}
		ret += string(expr[i])
	}
	return ret
}

func (qb *oCounterManager) IncrementOCounter(ctx context.Context, id int) (int, error) {
	if err := qb.tableMgr.checkIDExists(ctx, id); err != nil {
		return 0, err
	}

	if err := qb.setOCounter(ctx, id, oCounterRef+" + 1"); err != nil {
		return 0, err
	}

	return qb.getOCounter(ctx, id)
}

func (qb *oCounterManager) DecrementOCounter(ctx context.Context, id int) (int, error) {
	if err := qb.tableMgr.checkIDExists(ctx, id); err != nil {
		return 0, err
	}

	if err := qb.setOCounter(ctx, id, oCounterRef+" - 1"); err != nil {
		return 0, err
	}

	return qb.getOCounter(ctx, id)
}

func (qb *oCounterManager) ResetOCounter(ctx context.Context, id int) (int, error) {
	if err := qb.tableMgr.checkIDExists(ctx, id); err != nil {
		return 0, err
	}

	if err := qb.SetOCounter(ctx, id, 0); err != nil {
		return 0, err
	}

	return qb.getOCounter(ctx, id)
}

// SetOCounter sets the current user's o-counter of the image.
func (qb *oCounterManager) SetOCounter(ctx context.Context, id int, value int) error {
	if value <= 0 {
		stmt := fmt.Sprintf("DELETE FROM %s WHERE user_id = ? AND image_id = ?", imagesOCountsTable)
		if _, err := dbWrapper.Exec(ctx, stmt, user.CurrentID(ctx), id); err != nil {
			return fmt.Errorf("resetting image o-counter: %w", err)
		}
		return nil
	}
	return qb.setOCounter(ctx, id, "?", value)
}
