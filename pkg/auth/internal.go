package auth

import (
	"errors"
	"os"

	"github.com/valyala/fasthttp"
)

// ValidateInternalKey checks that a request came through the trusted internal channel.
func ValidateInternalKey(ctx *fasthttp.RequestCtx) error {
	internalKey := string(ctx.Request.Header.Peek("X-Internal-Key"))
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
func ValidateInternalRequest(ctx *fasthttp.RequestCtx) (string, error) {
	if err := ValidateInternalKey(ctx); err != nil {
		return "", err
	}

	userID := string(ctx.Request.Header.Peek("X-User-ID"))
	if userID == "" {
		return "", errors.New("unauthorized: missing user ID header")
	}

	return userID, nil
}
