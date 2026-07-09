package models

import (
	"context"
	"errors"
	"time"

	"notes-go-backend/pkg/database"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
)

const (
	MaxFailedAttempts      = 5
	LockoutDurationMinutes = 15
)

// FullUser represents the complete User schema for auth operations
type FullUser struct {
	ID                   bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name                 string        `bson:"name" json:"name"`
	Email                string        `bson:"email" json:"email"`
	Password             string        `bson:"password" json:"password,omitempty"`
	Image                string        `bson:"image" json:"image,omitempty"`
	FailedLoginAttempts  int           `bson:"failedLoginAttempts" json:"-"`
	LockedUntil          *time.Time    `bson:"lockedUntil" json:"-"`
	LastLoginAt          *time.Time    `bson:"lastLoginAt" json:"lastLoginAt,omitempty"`
	PasswordChangedAt    *time.Time    `bson:"passwordChangedAt" json:"-"`
	ResetPasswordToken   string        `bson:"resetPasswordToken" json:"-"`
	ResetPasswordExpires *time.Time    `bson:"resetPasswordExpires" json:"-"`
}

// AuthenticateUser verifies signin credentials and manages lockouts
func AuthenticateUser(ctx context.Context, email, password string) (*FullUser, error) {
	coll, err := database.GetCollection("users")
	if err != nil {
		return nil, err
	}

	var user FullUser
	err = coll.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid email or password")
		}
		return nil, err
	}

	now := time.Now()

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(now) {
		return nil, errors.New("account is temporarily locked. try again in some minutes")
	}

	// Verify password using bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		// Increment failed attempts
		failedAttempts := user.FailedLoginAttempts + 1
		updateData := bson.M{
			"failedLoginAttempts": failedAttempts,
		}

		if failedAttempts >= MaxFailedAttempts {
			lockout := now.Add(LockoutDurationMinutes * time.Minute)
			updateData["lockedUntil"] = lockout
		}

		_, _ = coll.UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": updateData})

		if failedAttempts >= MaxFailedAttempts {
			return nil, errors.New("too many failed attempts. account temporarily locked for 15 minutes")
		}
		return nil, errors.New("invalid email or password")
	}

	// Reset attempts and set login time
	updateData := bson.M{
		"failedLoginAttempts": 0,
		"lockedUntil":         nil,
		"lastLoginAt":         now,
	}
	_, _ = coll.UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": updateData})

	// Hide password
	user.Password = ""
	return &user, nil
}

// RegisterUser signs up a new user
func RegisterUser(ctx context.Context, name, email, password string) (*FullUser, error) {
	coll, err := database.GetCollection("users")
	if err != nil {
		return nil, err
	}

	// Check if email already exists
	var existing FullUser
	err = coll.FindOne(ctx, bson.M{"email": email}).Decode(&existing)
	if err == nil {
		return nil, errors.New("email is already registered")
	} else if err != mongo.ErrNoDocuments {
		return nil, err
	}

	// Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	newUser := FullUser{
		ID:                  bson.NewObjectID(),
		Name:                name,
		Email:               email,
		Password:            string(hashed),
		FailedLoginAttempts: 0,
		LastLoginAt:         &now,
	}

	_, err = coll.InsertOne(ctx, newUser)
	if err != nil {
		return nil, err
	}

	newUser.Password = ""
	return &newUser, nil
}

// SetPasswordResetToken sets the reset token hash and expiration date
func SetPasswordResetToken(ctx context.Context, email, hashedToken string, expires time.Time) (*FullUser, error) {
	coll, err := database.GetCollection("users")
	if err != nil {
		return nil, err
	}

	var user FullUser
	err = coll.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("no account found with this email address")
		}
		return nil, err
	}

	_, err = coll.UpdateOne(ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{
			"resetPasswordToken":   hashedToken,
			"resetPasswordExpires": expires,
		}},
	)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// ResetPasswordByToken resets user password if the token hash matches and is not expired
func ResetPasswordByToken(ctx context.Context, email, hashedToken, newPassword string) error {
	coll, err := database.GetCollection("users")
	if err != nil {
		return err
	}

	var user FullUser
	err = coll.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("invalid or expired password reset token")
		}
		return err
	}

	now := time.Now()

	// Verify token match and expiry
	if user.ResetPasswordToken != hashedToken || user.ResetPasswordExpires == nil || user.ResetPasswordExpires.Before(now) {
		return errors.New("invalid or expired password reset token")
	}

	// Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}

	update := bson.M{
		"$set": bson.M{
			"password":            string(hashed),
			"passwordChangedAt":   now,
			"failedLoginAttempts": 0,
			"lockedUntil":         nil,
		},
		"$unset": bson.M{
			"resetPasswordToken":   "",
			"resetPasswordExpires": "",
		},
	}

	_, err = coll.UpdateOne(ctx, bson.M{"_id": user.ID}, update)
	return err
}
