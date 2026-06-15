package middleware

import (
	"errors"
	"net/http"
	"strings"
)

// Authenticator validates a bearer token.
// Implement this interface to add JWT, OAuth2, or any other auth strategy.
type Authenticator interface {
	Authenticate(token string) error
}

// StaticKeyAuthenticator accepts a single pre-shared API key.
type StaticKeyAuthenticator struct {
	key string
}

func NewStaticKeyAuthenticator(key string) *StaticKeyAuthenticator {
	return &StaticKeyAuthenticator{key: key}
}

func (a *StaticKeyAuthenticator) Authenticate(token string) error {
	if token != a.key {
		return errors.New("invalid api key")
	}
	return nil
}

// BearerMiddleware extracts the Bearer token from the Authorization header
// and delegates validation to the provided Authenticator.
func BearerMiddleware(auth Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := extractBearerToken(r)
			if err != nil {
				http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}
			if err := auth.Authenticate(token); err != nil {
				http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractBearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", errors.New("missing Authorization header")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errors.New("Authorization header must be 'Bearer <token>'")
	}
	return strings.TrimSpace(parts[1]), nil
}
