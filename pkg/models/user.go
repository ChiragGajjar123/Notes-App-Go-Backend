package models

import (
	"context"

	"notes-go-backend/pkg/database"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// User represents the MongoDB User document subset we need for category and theme settings
type User struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name       string        `bson:"name" json:"name"`
	Email      string        `bson:"email" json:"email"`
	Categories []string      `bson:"categories" json:"categories"`
	Theme      string        `bson:"theme" json:"theme"`
}

// GetUserSettings retrieves the user settings including profile details
func GetUserSettings(ctx context.Context, userID bson.ObjectID) (*User, error) {
	coll, err := database.GetCollection("users")
	if err != nil {
		return nil, err
	}

	var user User
	err = coll.FindOne(ctx, bson.M{"_id": userID}, options.FindOne().SetProjection(bson.M{"name": 1, "email": 1, "theme": 1, "categories": 1})).Decode(&user)
	if err != nil {
		return nil, err
	}

	if user.Theme == "" {
		user.Theme = "light"
	}
	if user.Categories == nil {
		user.Categories = []string{}
	}

	return &user, nil
}

// GetCategories returns user categories list
func GetCategories(ctx context.Context, userID bson.ObjectID) ([]string, error) {
	coll, err := database.GetCollection("users")
	if err != nil {
		return nil, err
	}

	var user User
	err = coll.FindOne(ctx, bson.M{"_id": userID}, options.FindOne().SetProjection(bson.M{"categories": 1})).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return []string{}, nil
		}
		return nil, err
	}

	if user.Categories == nil {
		user.Categories = []string{}
	}

	return user.Categories, nil
}

// CreateCategory adds a category name if it doesn't exist ($addToSet)
func CreateCategory(ctx context.Context, userID bson.ObjectID, name string) ([]string, error) {
	coll, err := database.GetCollection("users")
	if err != nil {
		return nil, err
	}

	filter := bson.M{"_id": userID}
	update := bson.M{"$addToSet": bson.M{"categories": name}}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After).SetProjection(bson.M{"categories": 1})
	var user User
	err = coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&user)
	if err != nil {
		return nil, err
	}

	if user.Categories == nil {
		user.Categories = []string{}
	}

	return user.Categories, nil
}

// RenameCategory updates the category name in user list, and updates notes that reference it
func RenameCategory(ctx context.Context, userID bson.ObjectID, oldName, newName string) error {
	usersColl, err := database.GetCollection("users")
	if err != nil {
		return err
	}

	filter := bson.M{"_id": userID, "categories": oldName}
	update := bson.M{"$set": bson.M{"categories.$": newName}}
	_, err = usersColl.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	// Update associated notes
	notesColl, err := database.GetCollection("notes")
	if err != nil {
		return err
	}

	noteFilter := bson.M{"userId": userID, "category": oldName}
	noteUpdate := bson.M{"$set": bson.M{"category": newName}}
	_, err = notesColl.UpdateMany(ctx, noteFilter, noteUpdate)
	return err
}

// DeleteCategory pulls the category name from user categories array, and sets all matching notes' category to "other"
func DeleteCategory(ctx context.Context, userID bson.ObjectID, name string) error {
	usersColl, err := database.GetCollection("users")
	if err != nil {
		return err
	}

	filter := bson.M{"_id": userID}
	update := bson.M{"$pull": bson.M{"categories": name}}
	_, err = usersColl.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	// Update associated notes to category "other"
	notesColl, err := database.GetCollection("notes")
	if err != nil {
		return err
	}

	noteFilter := bson.M{"userId": userID, "category": name}
	noteUpdate := bson.M{"$set": bson.M{"category": "other"}}
	_, err = notesColl.UpdateMany(ctx, noteFilter, noteUpdate)
	return err
}

// GetTheme retrieves user's theme setting
func GetTheme(ctx context.Context, userID bson.ObjectID) (string, error) {
	coll, err := database.GetCollection("users")
	if err != nil {
		return "", err
	}

	var user User
	err = coll.FindOne(ctx, bson.M{"_id": userID}, options.FindOne().SetProjection(bson.M{"theme": 1})).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "light", nil
		}
		return "", err
	}

	if user.Theme == "" {
		user.Theme = "light"
	}

	return user.Theme, nil
}

// UpdateTheme updates user's theme setting
func UpdateTheme(ctx context.Context, userID bson.ObjectID, theme string) (string, error) {
	coll, err := database.GetCollection("users")
	if err != nil {
		return "", err
	}

	filter := bson.M{"_id": userID}
	update := bson.M{"$set": bson.M{"theme": theme}}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After).SetProjection(bson.M{"theme": 1})
	var user User
	err = coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&user)
	if err != nil {
		return "", err
	}

	return user.Theme, nil
}
