package database

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	clientInstance *mongo.Client
	clientOnce     sync.Once
	clientErr      error

	// collectionCache avoids re-resolving DB name + collection on every call.
	collectionCache sync.Map // map[string]*mongo.Collection
)

// ConnectDB returns a thread-safe singleton connection to MongoDB.
// Uses sync.Once instead of sync.Mutex — lock-free after the first call.
func ConnectDB() (*mongo.Client, error) {
	clientOnce.Do(func() {
		uri := os.Getenv("MONGODB_URI")
		if uri == "" {
			// default for local development if not set
			uri = "mongodb://localhost:27017/notes-app"
		}

		clientOpts := options.Client().ApplyURI(uri).
			SetMaxPoolSize(1000).
			SetMinPoolSize(100).
			SetMaxConnIdleTime(60 * time.Second).
			SetMaxConnecting(50).
			SetConnectTimeout(5 * time.Second).
			SetCompressors([]string{"zstd", "snappy", "zlib"})

		client, err := mongo.Connect(clientOpts)
		if err != nil {
			clientErr = err
			return
		}

		// Ping the database
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()
		if err := client.Ping(pingCtx, nil); err != nil {
			_ = client.Disconnect(context.Background())
			clientErr = err
			return
		}

		clientInstance = client
		log.Println("Successfully connected to MongoDB")

		// Ensure performance indexes in background
		go ensureIndexes(clientInstance)
	})

	return clientInstance, clientErr
}

// ensureIndexes creates background indexes for maximum query throughput
func ensureIndexes(client *mongo.Client) {
	dbName := os.Getenv("MONGODB_DB")
	if dbName == "" {
		dbName = "notes-app"
	}
	db := client.Database(dbName)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Unique index on user email
	_, _ = db.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	// Compound index on notes queries (userId, isArchived, isPinned, updatedAt)
	_, _ = db.Collection("notes").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "userId", Value: 1},
			{Key: "isArchived", Value: 1},
			{Key: "isPinned", Value: -1},
			{Key: "updatedAt", Value: -1},
		},
	})
}

// GetCollection returns a mongo.Collection from the configured database.
// Collection references are cached in a sync.Map for zero-allocation lookups.
func GetCollection(name string) (*mongo.Collection, error) {
	// Fast path: check cache first (lock-free read)
	if cached, ok := collectionCache.Load(name); ok {
		return cached.(*mongo.Collection), nil
	}

	// Slow path: connect and cache
	client, err := ConnectDB()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, mongo.ErrClientDisconnected
	}

	dbName := os.Getenv("MONGODB_DB")
	if dbName == "" {
		dbName = "notes-app"
	}

	coll := client.Database(dbName).Collection(name)
	collectionCache.Store(name, coll)
	return coll, nil
}

// GetClient returns the singleton MongoDB client (for shutdown purposes).
func GetClient() *mongo.Client {
	return clientInstance
}
