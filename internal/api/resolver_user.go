package api

import (
	"context"
	"fmt"
	"strconv"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/user"
)

func (r *queryResolver) Me(ctx context.Context) (*models.User, error) {
	p := user.Current(ctx)
	if u, ok := manager.GetInstance().Users.ByID(p.ID); ok && manager.GetInstance().Users.HasUsers() {
		return &u, nil
	}

	// authentication not configured - return the system user
	return &models.User{
		ID:        p.ID,
		Username:  p.Username,
		Role:      p.Role,
		Libraries: p.Libraries,
	}, nil
}

func (r *queryResolver) FindUsers(ctx context.Context) ([]*models.User, error) {
	if err := requireRole(ctx, user.RoleAdmin); err != nil {
		return nil, err
	}

	users := manager.GetInstance().Users.All()
	ret := make([]*models.User, len(users))
	for i := range users {
		u := users[i]
		ret[i] = &u
	}
	return ret, nil
}

func (r *mutationResolver) UserCreate(ctx context.Context, input UserCreateInput) (*models.User, error) {
	if err := requireRole(ctx, user.RoleAdmin); err != nil {
		return nil, err
	}

	return manager.GetInstance().Users.Create(ctx, manager.UserCreateInput{
		Username:  input.Username,
		Password:  input.Password,
		Role:      input.Role,
		Libraries: input.Libraries,
	})
}

func (r *mutationResolver) UserUpdate(ctx context.Context, input UserUpdateInput) (*models.User, error) {
	if err := requireRole(ctx, user.RoleAdmin); err != nil {
		return nil, err
	}

	id, err := strconv.Atoi(input.ID)
	if err != nil {
		return nil, fmt.Errorf("converting id: %w", err)
	}

	return manager.GetInstance().Users.Update(ctx, manager.UserUpdateInput{
		ID:        id,
		Username:  input.Username,
		Password:  input.Password,
		Role:      input.Role,
		Libraries: librariesInput(input.Libraries),
	})
}

func (r *mutationResolver) UserDestroy(ctx context.Context, id string) (bool, error) {
	if err := requireRole(ctx, user.RoleAdmin); err != nil {
		return false, err
	}

	idInt, err := strconv.Atoi(id)
	if err != nil {
		return false, fmt.Errorf("converting id: %w", err)
	}

	if idInt == user.CurrentID(ctx) {
		return false, fmt.Errorf("cannot delete the current user")
	}

	if err := manager.GetInstance().Users.Destroy(ctx, idInt); err != nil {
		return false, err
	}
	return true, nil
}

func (r *mutationResolver) UserGenerateAPIKey(ctx context.Context, input UserGenerateAPIKeyInput) (string, error) {
	if err := requireRole(ctx, user.RoleAdmin); err != nil {
		return "", err
	}

	id, err := strconv.Atoi(input.ID)
	if err != nil {
		return "", fmt.Errorf("converting id: %w", err)
	}

	clearKey := input.Clear != nil && *input.Clear
	return manager.GetInstance().Users.GenerateAPIKey(ctx, id, clearKey)
}

func (r *mutationResolver) ChangePassword(ctx context.Context, input UserChangePasswordInput) (bool, error) {
	users := manager.GetInstance().Users
	if !users.HasUsers() {
		return false, fmt.Errorf("authentication is not configured")
	}

	if err := users.ChangePassword(ctx, user.CurrentID(ctx), input.CurrentPassword, input.NewPassword); err != nil {
		return false, err
	}
	return true, nil
}

// librariesInput converts an optional libraries input; a provided empty list
// clears the libraries.
func librariesInput(v []string) *[]string {
	if v == nil {
		return nil
	}
	return &v
}
