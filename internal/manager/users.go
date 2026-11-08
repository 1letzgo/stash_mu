package manager

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/user"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrUsernameTaken       = errors.New("username is already taken")
	ErrInvalidUsername     = errors.New("username must not be empty")
	ErrInvalidPassword     = errors.New("password must not be empty")
	ErrCannotDeletePrimary = errors.New("the primary admin user cannot be deleted")
	ErrLastAdmin           = errors.New("at least one admin user is required")
	ErrWrongPassword       = errors.New("current password is incorrect")
)

// UserService manages user accounts. It keeps an in-memory copy of all users
// so that authenticating a request does not require a database query.
type UserService struct {
	Repository models.Repository
	Config     *config.Config
	// dbReady returns nil when the database is usable.
	dbReady func() error

	mu     sync.RWMutex
	loaded bool
	users  []models.User
}

func NewUserService(repo models.Repository, cfg *config.Config, dbReady func() error) *UserService {
	return &UserService{
		Repository: repo,
		Config:     cfg,
		dbReady:    dbReady,
	}
}

// Invalidate discards the cached users; they are reloaded on next use.
func (s *UserService) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loaded = false
	s.users = nil
}

func (s *UserService) snapshot() []models.User {
	s.mu.RLock()
	if s.loaded {
		defer s.mu.RUnlock()
		return s.users
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return s.users
	}

	if s.dbReady == nil || s.dbReady() != nil {
		// database not available (yet) - fall back to legacy credentials
		return nil
	}

	var users []*models.User
	ctx := context.Background()
	if err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		var err error
		users, err = s.Repository.User.All(ctx)
		if err != nil {
			return err
		}

		if len(users) == 0 {
			// credentials may have been set in the config file (by setup, the
			// legacy settings or manually) - create the primary admin user
			primary, err := s.createPrimaryFromConfig(ctx)
			if err != nil {
				return err
			}
			if primary != nil {
				users = append(users, primary)
			}
		}
		return nil
	}); err != nil {
		logger.Errorf("loading users: %v", err)
		return nil
	}

	s.users = make([]models.User, len(users))
	for i, u := range users {
		s.users[i] = *u
	}
	s.loaded = true
	return s.users
}

// HasUsers implements config.UserCredentialProvider.
func (s *UserService) HasUsers() bool {
	return len(s.snapshot()) > 0
}

// All returns all users.
func (s *UserService) All() []models.User {
	return s.snapshot()
}

// ByUsername returns the user with the given (case-insensitive) username.
func (s *UserService) ByUsername(username string) (models.User, bool) {
	username = strings.TrimSpace(username)
	for _, u := range s.snapshot() {
		if strings.EqualFold(u.Username, username) {
			return u, true
		}
	}
	return models.User{}, false
}

// ByID returns the user with the given id.
func (s *UserService) ByID(id int) (models.User, bool) {
	for _, u := range s.snapshot() {
		if u.ID == id {
			return u, true
		}
	}
	return models.User{}, false
}

// ValidateUserCredentials implements config.UserCredentialProvider.
func (s *UserService) ValidateUserCredentials(username string, password string) bool {
	u, ok := s.ByUsername(username)
	if !ok || u.PasswordHash == "" {
		// compare anyway to avoid leaking valid usernames through timing
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(password))
		return false
	}

	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}

// UsernameForAPIKey implements config.UserCredentialProvider.
func (s *UserService) UsernameForAPIKey(apiKey string) (string, bool) {
	if apiKey == "" {
		return "", false
	}
	for _, u := range s.snapshot() {
		if u.APIKey != "" && u.APIKey == apiKey {
			return u.Username, true
		}
	}
	return "", false
}

// Principal resolves the request principal for an authenticated username.
// If no users exist, the System user is returned for any username.
func (s *UserService) Principal(username string) (user.User, bool) {
	if !s.HasUsers() {
		return user.System, true
	}

	u, ok := s.ByUsername(username)
	if !ok {
		return user.User{}, false
	}
	return u.Principal(), true
}

var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

func dummyPasswordHash() []byte {
	dummyHashOnce.Do(func() {
		dummyHash, _ = bcrypt.GenerateFromPassword([]byte("stash-dummy-password"), bcrypt.DefaultCost)
	})
	return dummyHash
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (s *UserService) withTxn(ctx context.Context, fn func(ctx context.Context, qb models.UserReaderWriter) error) error {
	defer s.Invalidate()
	return s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		return fn(ctx, s.Repository.User)
	})
}

func countAdmins(users []*models.User) int {
	n := 0
	for _, u := range users {
		if u.Role == user.RoleAdmin {
			n++
		}
	}
	return n
}

// syncPrimaryToConfig mirrors the primary admin's credentials into the config
// file, so that the legacy single-user settings remain consistent.
func (s *UserService) syncPrimaryToConfig(u *models.User) {
	if u == nil || u.ID != user.SystemUserID || s.Config == nil {
		return
	}

	s.Config.SetString(config.Username, u.Username)
	s.Config.SetString(config.Password, u.PasswordHash)
	s.Config.SetString(config.ApiKey, u.APIKey)
	if err := s.Config.Write(); err != nil {
		logger.Errorf("writing config after updating primary user: %v", err)
	}
}

// UserCreateInput holds the values for creating a user.
type UserCreateInput struct {
	Username string
	Password string
	Role     user.Role
}

// Create creates a new user. The first user created is always an admin.
func (s *UserService) Create(ctx context.Context, input UserCreateInput) (*models.User, error) {
	username := strings.TrimSpace(input.Username)
	if username == "" {
		return nil, ErrInvalidUsername
	}
	if input.Password == "" {
		return nil, ErrInvalidPassword
	}
	if !input.Role.IsValid() {
		return nil, fmt.Errorf("invalid role %q", input.Role)
	}

	hash, err := hashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	var ret *models.User
	if err := s.withTxn(ctx, func(ctx context.Context, qb models.UserReaderWriter) error {
		existing, err := qb.FindByUsername(ctx, username)
		if err != nil {
			return err
		}
		if existing != nil {
			return ErrUsernameTaken
		}

		n, err := qb.Count(ctx)
		if err != nil {
			return err
		}

		role := input.Role
		apiKey := ""
		if n == 0 {
			// first user owns all existing data and must be able to administer
			role = user.RoleAdmin
			// keep the existing legacy api key working
			apiKey = s.Config.GetAPIKey()
		}

		now := time.Now()
		ret = &models.User{
			Username:     username,
			PasswordHash: hash,
			Role:         role,
			APIKey:       apiKey,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		return qb.Create(ctx, ret)
	}); err != nil {
		return nil, err
	}

	s.syncPrimaryToConfig(ret)
	return ret, nil
}

// UserUpdateInput holds the values for updating a user. Nil fields are not
// changed.
type UserUpdateInput struct {
	ID       int
	Username *string
	Password *string
	Role     *user.Role
	UIConfig map[string]interface{}
}

// Update updates an existing user.
func (s *UserService) Update(ctx context.Context, input UserUpdateInput) (*models.User, error) {
	var ret *models.User
	if err := s.withTxn(ctx, func(ctx context.Context, qb models.UserReaderWriter) error {
		u, err := qb.Find(ctx, input.ID)
		if err != nil {
			return err
		}
		if u == nil {
			return ErrUserNotFound
		}

		if input.Username != nil {
			username := strings.TrimSpace(*input.Username)
			if username == "" {
				return ErrInvalidUsername
			}
			existing, err := qb.FindByUsername(ctx, username)
			if err != nil {
				return err
			}
			if existing != nil && existing.ID != u.ID {
				return ErrUsernameTaken
			}
			u.Username = username
		}

		if input.Password != nil {
			if *input.Password == "" {
				return ErrInvalidPassword
			}
			hash, err := hashPassword(*input.Password)
			if err != nil {
				return err
			}
			u.PasswordHash = hash
		}

		if input.Role != nil && *input.Role != u.Role {
			if !input.Role.IsValid() {
				return fmt.Errorf("invalid role %q", *input.Role)
			}
			if u.Role == user.RoleAdmin {
				all, err := qb.All(ctx)
				if err != nil {
					return err
				}
				if countAdmins(all) <= 1 {
					return ErrLastAdmin
				}
			}
			if u.ID == user.SystemUserID {
				return ErrLastAdmin
			}
			u.Role = *input.Role
		}

		if input.UIConfig != nil {
			u.UIConfig = input.UIConfig
		}

		u.UpdatedAt = time.Now()
		if err := qb.Update(ctx, u); err != nil {
			return err
		}
		ret = u
		return nil
	}); err != nil {
		return nil, err
	}

	s.syncPrimaryToConfig(ret)
	return ret, nil
}

// SetUIConfig replaces the per-user UI configuration.
func (s *UserService) SetUIConfig(ctx context.Context, id int, cfg map[string]interface{}) error {
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	_, err := s.Update(ctx, UserUpdateInput{ID: id, UIConfig: cfg})
	return err
}

// ChangePassword changes the password of a user after verifying the current
// password.
func (s *UserService) ChangePassword(ctx context.Context, id int, current string, newPassword string) error {
	u, ok := s.ByID(id)
	if !ok {
		return ErrUserNotFound
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(current)) != nil {
		return ErrWrongPassword
	}
	_, err := s.Update(ctx, UserUpdateInput{ID: id, Password: &newPassword})
	return err
}

// Destroy deletes a user and all of their per-user data.
func (s *UserService) Destroy(ctx context.Context, id int) error {
	if id == user.SystemUserID {
		return ErrCannotDeletePrimary
	}

	return s.withTxn(ctx, func(ctx context.Context, qb models.UserReaderWriter) error {
		u, err := qb.Find(ctx, id)
		if err != nil {
			return err
		}
		if u == nil {
			return ErrUserNotFound
		}

		if u.Role == user.RoleAdmin {
			all, err := qb.All(ctx)
			if err != nil {
				return err
			}
			if countAdmins(all) <= 1 {
				return ErrLastAdmin
			}
		}

		return qb.Destroy(ctx, id)
	})
}

// GenerateAPIKey generates (or clears, if clear is true) the api key of a
// user. Any previous key stops working immediately.
func (s *UserService) GenerateAPIKey(ctx context.Context, id int, clearKey bool) (string, error) {
	var ret *models.User
	if err := s.withTxn(ctx, func(ctx context.Context, qb models.UserReaderWriter) error {
		u, err := qb.Find(ctx, id)
		if err != nil {
			return err
		}
		if u == nil {
			return ErrUserNotFound
		}

		u.APIKey = ""
		if !clearKey {
			key, err := GenerateAPIKey(u.Username)
			if err != nil {
				return fmt.Errorf("generating api key: %w", err)
			}
			u.APIKey = key
		}

		u.UpdatedAt = time.Now()
		if err := qb.Update(ctx, u); err != nil {
			return err
		}
		ret = u
		return nil
	}); err != nil {
		return "", err
	}

	s.syncPrimaryToConfig(ret)
	return ret.APIKey, nil
}

// createPrimaryFromConfig creates the primary admin user from the legacy
// config credentials, if set. Must be called in a write transaction when no
// users exist.
func (s *UserService) createPrimaryFromConfig(ctx context.Context) (*models.User, error) {
	if s.Config == nil {
		return nil, nil
	}

	username := s.Config.GetUsername()
	hash := s.Config.GetPasswordHash()
	if username == "" || hash == "" {
		return nil, nil
	}

	now := time.Now()
	u := &models.User{
		Username:     username,
		PasswordHash: hash,
		Role:         user.RoleAdmin,
		APIKey:       s.Config.GetAPIKey(),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.Repository.User.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("creating primary user: %w", err)
	}

	logger.Infof("Created primary admin user %q from configured credentials", username)
	return u, nil
}

// EnsurePrimaryFromConfig creates the primary admin user from the config
// credentials if no users exist yet, for example after credentials were set
// for the first time through the legacy general settings.
func (s *UserService) EnsurePrimaryFromConfig(ctx context.Context) error {
	s.Invalidate()
	s.snapshot()
	return nil
}

type userRestorer interface {
	CreateWithID(ctx context.Context, u *models.User) error
}

// Restore re-creates the provided users with their ids, for example after the
// database has been reset by a full import. Users that already exist are
// skipped.
func (s *UserService) Restore(ctx context.Context, users []models.User) error {
	if len(users) == 0 {
		return nil
	}

	restorer, ok := s.Repository.User.(userRestorer)
	if !ok {
		return fmt.Errorf("user repository does not support restoring users")
	}

	return s.withTxn(ctx, func(ctx context.Context, qb models.UserReaderWriter) error {
		for i := range users {
			existing, err := qb.Find(ctx, users[i].ID)
			if err != nil {
				return err
			}
			if existing != nil {
				continue
			}
			if err := restorer.CreateWithID(ctx, &users[i]); err != nil {
				return err
			}
		}
		return nil
	})
}
