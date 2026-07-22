package models

import (
	"context"
	"time"

	"notes-go-backend/pkg/database"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Note matches the MongoDB Notes schema
type Note struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"_id"`
	Title      string        `bson:"title" json:"title"`
	Content    string        `bson:"content" json:"content"`
	Category   string        `bson:"category" json:"category"`
	Tags       []string      `bson:"tags" json:"tags"`
	IsPinned   bool          `bson:"isPinned" json:"isPinned"`
	IsArchived bool          `bson:"isArchived" json:"isArchived"`
	Color      string        `bson:"color" json:"color"`
	UserID     bson.ObjectID `bson:"userId" json:"userId"`
	CreatedAt  time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time     `bson:"updatedAt" json:"updatedAt"`
}

// FindNotes returns notes for a user filtered by isArchived state, sorted by isPinned desc and updatedAt desc
func FindNotes(ctx context.Context, userID bson.ObjectID, isArchived bool) ([]Note, error) {
	coll, err := database.GetCollection("notes")
	if err != nil {
		return nil, err
	}

	userFilter := bson.M{
		"$or": []bson.M{
			{"userId": userID},
			{"userId": userID.Hex()},
		},
	}

	var archiveFilter bson.M
	if isArchived {
		archiveFilter = bson.M{"isArchived": true}
	} else {
		archiveFilter = bson.M{
			"$or": []bson.M{
				{"isArchived": false},
				{"isArchived": bson.M{"$exists": false}},
			},
		}
	}

	filter := bson.M{
		"$and": []bson.M{
			userFilter,
			archiveFilter,
		},
	}

	findOpts := options.Find().SetSort(bson.D{
		{Key: "isPinned", Value: -1},
		{Key: "updatedAt", Value: -1},
	})

	cursor, err := coll.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var notes []Note
	if err = cursor.All(ctx, &notes); err != nil {
		return nil, err
	}

	// In case MongoDB returns nil for empty array, make it empty slice for JSON compatibility
	if notes == nil {
		notes = []Note{}
	}

	return notes, nil
}

// SaveNote creates a new note or updates an existing one
func SaveNote(ctx context.Context, note *Note) (*Note, error) {
	coll, err := database.GetCollection("notes")
	if err != nil {
		return nil, err
	}

	now := time.Now()
	note.UpdatedAt = now

	if note.ID.IsZero() {
		// Create flow
		note.ID = bson.NewObjectID()
		note.CreatedAt = now
		if note.Color == "" {
			note.Color = "#ffffff"
		}
		if note.Category == "" {
			note.Category = "other"
		}
		if note.Tags == nil {
			note.Tags = []string{}
		}

		_, err = coll.InsertOne(ctx, note)
		if err != nil {
			return nil, err
		}
	} else {
		// Update flow
		filter := bson.M{"_id": note.ID, "userId": note.UserID}
		update := bson.M{
			"$set": bson.M{
				"title":      note.Title,
				"content":    note.Content,
				"category":   note.Category,
				"tags":       note.Tags,
				"color":      note.Color,
				"isPinned":   note.IsPinned,
				"isArchived": note.IsArchived,
				"updatedAt":  note.UpdatedAt,
			},
		}

		opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
		var updatedNote Note
		err = coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedNote)
		if err != nil {
			return nil, err
		}
		*note = updatedNote
	}

	return note, nil
}

// DeleteNote deletes a note for a user
func DeleteNote(ctx context.Context, noteID bson.ObjectID, userID bson.ObjectID) error {
	coll, err := database.GetCollection("notes")
	if err != nil {
		return err
	}

	filter := bson.M{"_id": noteID, "userId": userID}
	res, err := coll.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}
