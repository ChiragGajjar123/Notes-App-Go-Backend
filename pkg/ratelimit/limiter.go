package ratelimit

import (
	"context"
	"errors"
	"strings"
	"time"

	"notes-go-backend/pkg/database"

	"github.com/valyala/fasthttp"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type RateLimitConfig struct {
	MaxAttempts   int
	WindowSeconds int
}

var (
	LoginLimit                 = RateLimitConfig{MaxAttempts: 5, WindowSeconds: 15 * 60}
	SignupLimit                = RateLimitConfig{MaxAttempts: 3, WindowSeconds: 60 * 60}
	PasswordResetLimit         = RateLimitConfig{MaxAttempts: 3, WindowSeconds: 60 * 60}
	PasswordResetCooldownLimit = RateLimitConfig{MaxAttempts: 1, WindowSeconds: 60}
)

type RateLimitEntry struct {
	Key     string    `bson:"key"`
	Count   int       `bson:"count"`
	ResetAt time.Time `bson:"resetAt"`
}

// GetClientIp extracts the user's IP address from headers
func GetClientIp(ctx *fasthttp.RequestCtx) string {
	forwarded := string(ctx.Request.Header.Peek("X-Forwarded-For"))
	if forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	realIP := string(ctx.Request.Header.Peek("X-Real-IP"))
	if realIP != "" {
		return realIP
	}
	remoteIP := ctx.RemoteIP()
	if remoteIP != nil {
		return remoteIP.String()
	}
	return "127.0.0.1"
}

// CheckLimit checks if a rate limit is exceeded, incrementing the attempts atomically.
func CheckLimit(ctx context.Context, key string, config RateLimitConfig) (bool, int, time.Time, error) {
	coll, err := database.GetCollection("ratelimits")
	if err != nil {
		return false, 0, time.Time{}, err
	}

	now := time.Now()

	// Try to increment atomically if the entry is still within the window
	filter := bson.M{
		"key":     key,
		"resetAt": bson.M{"$gt": now},
	}
	update := bson.M{"$inc": bson.M{"count": 1}}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var entry RateLimitEntry
	err = coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&entry)

	if err == nil {
		if entry.Count > config.MaxAttempts {
			return false, 0, entry.ResetAt, nil
		}
		return true, config.MaxAttempts - entry.Count, entry.ResetAt, nil
	}

	if err != mongo.ErrNoDocuments {
		return false, 0, time.Time{}, err
	}

	// Not found or expired: create new window
	resetAt := now.Add(time.Duration(config.WindowSeconds) * time.Second)
	upsertFilter := bson.M{"key": key}
	upsertUpdate := bson.M{"$set": bson.M{"count": 1, "resetAt": resetAt}}
	upsertOpts := options.FindOneAndUpdate().
		SetReturnDocument(options.After).
		SetUpsert(true)

	err = coll.FindOneAndUpdate(ctx, upsertFilter, upsertUpdate, upsertOpts).Decode(&entry)
	if err != nil {
		// Concurrent request might have inserted it first. Let's retry finding.
		var retryEntry RateLimitEntry
		err = coll.FindOne(ctx, bson.M{"key": key}).Decode(&retryEntry)
		if err != nil {
			return false, 0, time.Time{}, err
		}
		if retryEntry.Count > config.MaxAttempts {
			return false, 0, retryEntry.ResetAt, nil
		}
		return true, config.MaxAttempts - retryEntry.Count, retryEntry.ResetAt, nil
	}

	return true, config.MaxAttempts - 1, resetAt, nil
}

// EnforceRateLimit enforces rate limit by key and returns error if limit exceeded
func EnforceRateLimit(ctx context.Context, prefix string, ip string, config RateLimitConfig) (int, time.Time, error) {
	key := prefix + ":" + ip
	ok, remaining, resetAt, err := CheckLimit(ctx, key, config)
	if err != nil {
		return 0, time.Time{}, err
	}
	if !ok {
		return 0, resetAt, errors.New("Too many attempts. Please try again later.")
	}
	return remaining, resetAt, nil
}
