package auth

import (
	"errors"
	"os"
	"sync"

	"strings"

	"github.com/valyala/fasthttp"
)

var (
	cachedInternalKey string
	keyOnce           sync.Once
)

// InitAuth eagerly loads and caches the INTERNAL_API_KEY.
// Call from main() at startup to avoid os.Getenv syscall on every request.
func InitAuth() {
	keyOnce.Do(func() {
		cachedInternalKey = os.Getenv("INTERNAL_API_KEY")
	})
}

// getInternalKey returns the cached key, initializing on first call if needed.
func getInternalKey() string {
	InitAuth() // no-op if already initialized
	return cachedInternalKey
}

// getHeader performs case-insensitive header lookup for HTTP/1.1 and HTTP/2 headers
func getHeader(ctx *fasthttp.RequestCtx, key string) string {
	val := string(ctx.Request.Header.Peek(key))
	if val != "" {
		return val
	}
	val = string(ctx.Request.Header.Peek(strings.ToLower(key)))
	if val != "" {
		return val
	}
	return string(ctx.Request.Header.Peek(strings.ToUpper(key)))
}

// ValidateInternalKey checks that a request came through the trusted internal channel.
func ValidateInternalKey(ctx *fasthttp.RequestCtx) error {
	internalKey := getHeader(ctx, "X-Internal-Key")
	expectedKey := getInternalKey()

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

	userID := getHeader(ctx, "X-User-ID")
	if userID == "" {
		return "", errors.New("unauthorized: missing user ID header")
	}

	return userID, nil
}
