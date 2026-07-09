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
	clientMu       sync.Mutex
)

// ConnectDB returns a thread-safe singleton connection to MongoDB.
func ConnectDB() (*mongo.Client, error) {
	clientMu.Lock()
	defer clientMu.Unlock()

	if clientInstance != nil {
		return clientInstance, nil
	}

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		// default for local development if not set
		uri = "mongodb://localhost:27017/notes-app"
	}
	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return nil, err
	}

	// Ping the database
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pingCancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}

	clientInstance = client
	log.Println("Successfully connected to MongoDB")
	return clientInstance, nil
}

// GetCollection returns a mongo.Collection from the configured database.
func GetCollection(name string) (*mongo.Collection, error) {
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
	return client.Database(dbName).Collection(name), nil
}
