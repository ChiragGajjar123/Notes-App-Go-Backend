package auth

import (
	"errors"
	"net/http"
	"os"
)

// ValidateInternalKey checks that a request came through the trusted internal channel.
func ValidateInternalKey(r *http.Request) error {
	internalKey := r.Header.Get("X-Internal-Key")
	expectedKey := os.Getenv("INTERNAL_API_KEY")

	if expectedKey == "" {
		return errors.New("INTERNAL_API_KEY is not configured on the server")
	}

	if internalKey != expectedKey {
		return errors.New("unauthorized: invalid internal API key")
	}

	return nil
}

// ValidateInternalRequest checks the internal key and returns the signed-in user ID.
func ValidateInternalRequest(r *http.Request) (string, error) {
	if err := ValidateInternalKey(r); err != nil {
		return "", err
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		return "", errors.New("unauthorized: missing user ID header")
	}

	return userID, nil
}
