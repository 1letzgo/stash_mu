package api

import (
	"context"
	"net/url"
	"time"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/signedurl"
	"github.com/stashapp/stash/pkg/user"
)

// userSigningKey returns the HMAC signing key for a given user.
func userSigningKey(c *config.Config, _ string) []byte {
	return c.GetJWTSignKey()
}

// signedParams generates signed URL query parameters for the given path prefix and user.
func signedParams(c *config.Config, userID string, prefix string) url.Values {
	secret := userSigningKey(c, userID)
	cid := signedurl.GenerateCredentialID(secret, userID)
	expires := time.Now().Add(c.GetSignedURLExpiry())
	return signedurl.SignPrefix(prefix, secret, cid, expires)
}

// resolveCredentialID maps a credential ID back to a username and their signing key.
func resolveCredentialID(c *config.Config, cid string) (string, []byte, bool) {
	usernames := []string{c.GetUsername()}
	for _, u := range manager.GetInstance().Users.All() {
		usernames = append(usernames, u.Username)
	}

	for _, username := range usernames {
		if username == "" {
			continue
		}
		secret := userSigningKey(c, username)
		if signedurl.GenerateCredentialID(secret, username) == cid {
			return username, secret, true
		}
	}
	return "", nil, false
}

// currentUserAPIKey returns the api key of the user making the request. When
// no user accounts exist, the legacy api key from the configuration is used.
func currentUserAPIKey(ctx context.Context, c *config.Config) string {
	users := manager.GetInstance().Users
	if !users.HasUsers() {
		return c.GetAPIKey()
	}

	if u, ok := users.ByID(user.CurrentID(ctx)); ok {
		return u.APIKey
	}
	return ""
}
