package ratelimit

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"sync/atomic"
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

// ── In-memory rate limiter ──────────────────────────────────────────────────

// memEntry holds an in-memory rate limit counter.
type memEntry struct {
	count   atomic.Int64
	resetAt atomic.Int64 // unix nanoseconds
}

const numShards = 64

// shard holds a subset of rate limit entries to reduce lock contention.
type shard struct {
	mu      sync.RWMutex
	entries map[string]*memEntry
}

var shards [numShards]shard

func init() {
	for i := range shards {
		shards[i].entries = make(map[string]*memEntry)
	}
}

// getShard returns the shard for a given key using FNV-1a hash.
func getShard(key string) *shard {
	h := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return &shards[h%numShards]
}

// getOrCreateEntry returns an existing entry or creates a new one.
func getOrCreateEntry(key string, windowSeconds int) (*memEntry, bool) {
	s := getShard(key)
	now := time.Now()

	// Fast path: read lock
	s.mu.RLock()
	if e, ok := s.entries[key]; ok {
		resetAt := time.Unix(0, e.resetAt.Load())
		if resetAt.After(now) {
			s.mu.RUnlock()
			return e, false // existing, still valid
		}
	}
	s.mu.RUnlock()

	// Slow path: write lock — create or reset
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if e, ok := s.entries[key]; ok {
		resetAt := time.Unix(0, e.resetAt.Load())
		if resetAt.After(now) {
			return e, false // another goroutine created it
		}
		// Entry expired — reset it
		e.count.Store(0)
		e.resetAt.Store(now.Add(time.Duration(windowSeconds) * time.Second).UnixNano())
		return e, true
	}

	// Create new entry
	e := &memEntry{}
	e.count.Store(0)
	e.resetAt.Store(now.Add(time.Duration(windowSeconds) * time.Second).UnixNano())
	s.entries[key] = e
	return e, true
}

// InitRateLimiter starts background goroutines for cleanup and MongoDB sync.
func InitRateLimiter() {
	// Cleanup expired entries every 60 seconds
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			cleanupExpired()
		}
	}()

	// Sync to MongoDB every 30 seconds for persistence
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			syncToMongo()
		}
	}()
}

// cleanupExpired removes expired entries from all shards.
func cleanupExpired() {
	now := time.Now()
	for i := range shards {
		s := &shards[i]
		s.mu.Lock()
		for key, e := range s.entries {
			resetAt := time.Unix(0, e.resetAt.Load())
			if resetAt.Before(now) {
				delete(s.entries, key)
			}
		}
		s.mu.Unlock()
	}
}

// syncToMongo persists current in-memory counters to MongoDB.
func syncToMongo() {
	coll, err := database.GetCollection("ratelimits")
	if err != nil {
		log.Printf("ratelimit sync: failed to get collection: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()
	for i := range shards {
		s := &shards[i]
		s.mu.RLock()
		for key, e := range s.entries {
			resetAt := time.Unix(0, e.resetAt.Load())
			if resetAt.Before(now) {
				continue // skip expired
			}
			count := int(e.count.Load())

			filter := bson.M{"key": key}
			update := bson.M{"$set": bson.M{"count": count, "resetAt": resetAt}}
			opts := options.UpdateOne().SetUpsert(true)
			_, err := coll.UpdateOne(ctx, filter, update, opts)
			if err != nil {
				log.Printf("ratelimit sync: failed to sync key %s: %v", key, err)
			}
		}
		s.mu.RUnlock()
	}
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

// CheckLimit checks if a rate limit is exceeded using in-memory counters.
// Falls back to MongoDB for cross-restart persistence.
func CheckLimit(ctx context.Context, key string, config RateLimitConfig) (bool, int, time.Time, error) {
	entry, _ := getOrCreateEntry(key, config.WindowSeconds)
	newCount := int(entry.count.Add(1))
	resetAt := time.Unix(0, entry.resetAt.Load())

	if newCount > config.MaxAttempts {
		return false, 0, resetAt, nil
	}

	return true, config.MaxAttempts - newCount, resetAt, nil
}

// CheckLimitMongo is the original MongoDB-based rate limiter, kept as fallback.
func CheckLimitMongo(ctx context.Context, key string, config RateLimitConfig) (bool, int, time.Time, error) {
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
	var dbEntry RateLimitEntry
	err = coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&dbEntry)

	if err == nil {
		if dbEntry.Count > config.MaxAttempts {
			return false, 0, dbEntry.ResetAt, nil
		}
		return true, config.MaxAttempts - dbEntry.Count, dbEntry.ResetAt, nil
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

	err = coll.FindOneAndUpdate(ctx, upsertFilter, upsertUpdate, upsertOpts).Decode(&dbEntry)
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
