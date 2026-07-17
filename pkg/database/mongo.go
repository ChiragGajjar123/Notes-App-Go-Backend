package database

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

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
			SetMaxPoolSize(500).
			SetMinPoolSize(50).
			SetMaxConnIdleTime(30 * time.Second).
			SetMaxConnecting(10).
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
	})

	return clientInstance, clientErr
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
