package models

import (
	"context"
	"time"

	"github.com/stashapp/stash/pkg/user"
)

// User is a stash user account.
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         user.Role `json:"role"`
	APIKey       string    `json:"-"`
	// Libraries are the library paths the user may access. Empty means all.
	Libraries []string `json:"libraries"`
	// UIConfig holds the per-user overrides of the UI configuration.
	UIConfig  map[string]interface{} `json:"ui_config"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// Principal returns the lightweight representation used in request contexts.
func (u User) Principal() user.User {
	return user.User{
		ID:        u.ID,
		Username:  u.Username,
		Role:      u.Role,
		Libraries: u.Libraries,
	}
}

type UserReader interface {
	All(ctx context.Context) ([]*User, error)
	Find(ctx context.Context, id int) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	Count(ctx context.Context) (int, error)
}

type UserWriter interface {
	Create(ctx context.Context, obj *User) error
	Update(ctx context.Context, obj *User) error
	Destroy(ctx context.Context, id int) error
}

type UserReaderWriter interface {
	UserReader
	UserWriter
}
