package session

type SessionConfig interface {
	// ResolveAPIKey returns the username owning the provided api key.
	ResolveAPIKey(apiKey string) (string, bool)

	GetSessionStoreKey() []byte
	GetMaxSessionAge() int
	ValidateCredentials(username string, password string) bool
}
