package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stashapp/stash/pkg/user"
)

// post87 creates the initial admin user from the single set of credentials
// stored in the configuration file, so that existing logins, sessions and
// API keys keep working after upgrading to multi-user support.
func post87(ctx context.Context, db *sqlx.DB) error {
	logger.Info("Running post-migration for schema version 87")

	c := config.GetInstance()

	username := c.GetUsername()
	passwordHash := c.GetPasswordHash()

	if username == "" || passwordHash == "" {
		// no credentials configured - nothing to migrate
		return nil
	}

	m := migrator{db: db}
	return m.withTxn(ctx, func(tx *sqlx.Tx) error {
		now := time.Now()
		var apiKey interface{}
		if k := c.GetAPIKey(); k != "" {
			apiKey = k
		}

		if _, err := tx.Exec(
			"INSERT INTO `users` (`id`, `username`, `password_hash`, `role`, `api_key`, `created_at`, `updated_at`) VALUES (?, ?, ?, ?, ?, ?, ?)",
			user.SystemUserID, username, passwordHash, string(user.RoleAdmin), apiKey, now, now,
		); err != nil {
			return fmt.Errorf("creating admin user: %w", err)
		}

		logger.Info("Created admin user from existing credentials")
		return nil
	})
}

func init() {
	sqlite.RegisterPostMigration(87, post87)
}
