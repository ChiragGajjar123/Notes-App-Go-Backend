package main

// This file wires the local dev server routes to the same logic
// used by each Vercel serverless function in api/*.go.
// Each Vercel function exports Handler; here we call the shared pkg/ logic directly.

import (
	json "github.com/goccy/go-json"
	"strings"

	"notes-go-backend/pkg/auth"
	"notes-go-backend/pkg/models"
	"notes-go-backend/pkg/response"

	"github.com/valyala/fasthttp"
	"go.mongodb.org/mongo-driver/v2/bson"

	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// ── /api/categories ──────────────────────────────────────────────────────────

func categoriesHandler(ctx *fasthttp.RequestCtx) {
	userIDStr, err := auth.ValidateInternalRequest(ctx)
	if err != nil {
		response.Error(ctx, fasthttp.StatusUnauthorized, err.Error())
		return
	}
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		response.Error(ctx, fasthttp.StatusBadRequest, "Invalid User ID format")
		return
	}
	switch string(ctx.Method()) {
	case fasthttp.MethodGet:
		cats, err := models.GetCategories(ctx, userID)
		if err != nil {
			response.Error(ctx, fasthttp.StatusInternalServerError, "Failed to get categories: "+err.Error())
			return
		}
		response.JSON(ctx, fasthttp.StatusOK, cats)
	case fasthttp.MethodPost:
		var input struct{ Name string `json:"name"` }
		if err := json.Unmarshal(ctx.PostBody(), &input); err != nil {
			response.Error(ctx, fasthttp.StatusBadRequest, "Invalid request body")
			return
		}
		name := strings.TrimSpace(input.Name)
		if name == "" {
			response.Error(ctx, fasthttp.StatusBadRequest, "Category name is required")
			return
		}
		cats, err := models.CreateCategory(ctx, userID, name)
		if err != nil {
			response.Error(ctx, fasthttp.StatusInternalServerError, "Failed to create category: "+err.Error())
			return
		}
		response.JSON(ctx, fasthttp.StatusOK, cats)
	case fasthttp.MethodPut:
		var input struct {
			OldName string `json:"oldName"`
			NewName string `json:"newName"`
		}
		if err := json.Unmarshal(ctx.PostBody(), &input); err != nil {
			response.Error(ctx, fasthttp.StatusBadRequest, "Invalid request body")
			return
		}
		if strings.TrimSpace(input.OldName) == "" || strings.TrimSpace(input.NewName) == "" {
			response.Error(ctx, fasthttp.StatusBadRequest, "Old and new category names are required")
			return
		}
		if err := models.RenameCategory(ctx, userID, strings.TrimSpace(input.OldName), strings.TrimSpace(input.NewName)); err != nil {
			response.Error(ctx, fasthttp.StatusInternalServerError, "Failed to rename category: "+err.Error())
			return
		}
		response.JSON(ctx, fasthttp.StatusOK, map[string]interface{}{"ok": true})
	case fasthttp.MethodDelete:
		name := string(ctx.QueryArgs().Peek("name"))
		if name == "" {
			response.Error(ctx, fasthttp.StatusBadRequest, "Missing category name parameter")
			return
		}
		if err := models.DeleteCategory(ctx, userID, name); err != nil {
			response.Error(ctx, fasthttp.StatusInternalServerError, "Failed to delete category: "+err.Error())
			return
		}
		response.JSON(ctx, fasthttp.StatusOK, map[string]interface{}{"ok": true})
	default:
		response.Error(ctx, fasthttp.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ── /api/notes ────────────────────────────────────────────────────────────────

func notesHandler(ctx *fasthttp.RequestCtx) {
	userIDStr, err := auth.ValidateInternalRequest(ctx)
	if err != nil {
		response.Error(ctx, fasthttp.StatusUnauthorized, err.Error())
		return
	}
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		response.Error(ctx, fasthttp.StatusBadRequest, "Invalid User ID format")
		return
	}
	switch string(ctx.Method()) {
	case fasthttp.MethodGet:
		isArchived := strings.ToLower(string(ctx.QueryArgs().Peek("archived"))) == "true"
		notes, err := models.FindNotes(ctx, userID, isArchived)
		if err != nil {
			response.Error(ctx, fasthttp.StatusInternalServerError, "Failed to fetch notes: "+err.Error())
			return
		}
		response.JSON(ctx, fasthttp.StatusOK, notes)
	case fasthttp.MethodPost:
		var input struct {
			ID         string   `json:"_id,omitempty"`
			Title      string   `json:"title"`
			Content    string   `json:"content"`
			Category   string   `json:"category"`
			Tags       []string `json:"tags"`
			IsPinned   bool     `json:"isPinned"`
			IsArchived bool     `json:"isArchived"`
			Color      string   `json:"color"`
		}
		if err := json.Unmarshal(ctx.PostBody(), &input); err != nil {
			response.Error(ctx, fasthttp.StatusBadRequest, "Invalid request body")
			return
		}
		if strings.TrimSpace(input.Title) == "" {
			response.Error(ctx, fasthttp.StatusBadRequest, "Title is required")
			return
		}
		var noteID bson.ObjectID
		if input.ID != "" {
			noteID, err = bson.ObjectIDFromHex(input.ID)
			if err != nil {
				response.Error(ctx, fasthttp.StatusBadRequest, "Invalid Note ID format")
				return
			}
		}
		note := &models.Note{
			ID: noteID, Title: input.Title, Content: input.Content,
			Category: input.Category, Tags: input.Tags, IsPinned: input.IsPinned,
			IsArchived: input.IsArchived, Color: input.Color, UserID: userID,
		}
		saved, err := models.SaveNote(ctx, note)
		if err != nil {
			response.Error(ctx, fasthttp.StatusInternalServerError, "Failed to save note: "+err.Error())
			return
		}
		response.JSON(ctx, fasthttp.StatusOK, saved)
	case fasthttp.MethodDelete:
		idStr := string(ctx.QueryArgs().Peek("id"))
		if idStr == "" {
			response.Error(ctx, fasthttp.StatusBadRequest, "Missing note id parameter")
			return
		}
		noteID, err := bson.ObjectIDFromHex(idStr)
		if err != nil {
			response.Error(ctx, fasthttp.StatusBadRequest, "Invalid Note ID format")
			return
		}
		if err := models.DeleteNote(ctx, noteID, userID); err != nil {
			response.Error(ctx, fasthttp.StatusInternalServerError, "Failed to delete note: "+err.Error())
			return
		}
		response.JSON(ctx, fasthttp.StatusOK, map[string]interface{}{"ok": true})
	default:
		response.Error(ctx, fasthttp.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ── /api/settings ─────────────────────────────────────────────────────────────

func settingsHandler(ctx *fasthttp.RequestCtx) {
	userIDStr, err := auth.ValidateInternalRequest(ctx)
	if err != nil {
		response.Error(ctx, fasthttp.StatusUnauthorized, err.Error())
		return
	}
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		response.Error(ctx, fasthttp.StatusBadRequest, "Invalid User ID format")
		return
	}
	switch string(ctx.Method()) {
	case fasthttp.MethodGet:
		s, err := models.GetUserSettings(ctx, userID)
		if err != nil {
			response.Error(ctx, fasthttp.StatusInternalServerError, "Failed to get settings: "+err.Error())
			return
		}
		response.JSON(ctx, fasthttp.StatusOK, s)
	case fasthttp.MethodPut:
		var input struct{ Theme string `json:"theme"` }
		if err := json.Unmarshal(ctx.PostBody(), &input); err != nil {
			response.Error(ctx, fasthttp.StatusBadRequest, "Invalid request body")
			return
		}
		theme := strings.ToLower(strings.TrimSpace(input.Theme))
		if theme != "light" && theme != "dark" {
			response.Error(ctx, fasthttp.StatusBadRequest, "Theme must be either 'light' or 'dark'")
			return
		}
		updated, err := models.UpdateTheme(ctx, userID, theme)
		if err != nil {
			response.Error(ctx, fasthttp.StatusInternalServerError, "Failed to update theme: "+err.Error())
			return
		}
		response.JSON(ctx, fasthttp.StatusOK, map[string]string{"theme": updated})
	default:
		response.Error(ctx, fasthttp.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ── /api/signin ───────────────────────────────────────────────────────────────

func signinHandler(ctx *fasthttp.RequestCtx) {
	if err := auth.ValidateInternalKey(ctx); err != nil {
		response.Error(ctx, fasthttp.StatusUnauthorized, err.Error())
		return
	}
	if string(ctx.Method()) != fasthttp.MethodPost {
		response.Error(ctx, fasthttp.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &input); err != nil {
		response.Error(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" || input.Password == "" {
		response.Error(ctx, fasthttp.StatusBadRequest, "Email and password are required")
		return
	}
	user, err := models.AuthenticateUser(ctx, email, input.Password)
	if err != nil {
		response.Error(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	response.JSON(ctx, fasthttp.StatusOK, user)
}

// ── /api/signup ───────────────────────────────────────────────────────────────

func signupHandler(ctx *fasthttp.RequestCtx) {
	if err := auth.ValidateInternalKey(ctx); err != nil {
		response.Error(ctx, fasthttp.StatusUnauthorized, err.Error())
		return
	}
	if string(ctx.Method()) != fasthttp.MethodPost {
		response.Error(ctx, fasthttp.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &input); err != nil {
		response.Error(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}
	name := strings.TrimSpace(input.Name)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if name == "" || email == "" || input.Password == "" {
		response.Error(ctx, fasthttp.StatusBadRequest, "Name, email, and password are required")
		return
	}
	if len(input.Password) < 8 {
		response.Error(ctx, fasthttp.StatusBadRequest, "Password must be at least 8 characters long")
		return
	}
	user, err := models.RegisterUser(ctx, name, email, input.Password)
	if err != nil {
		response.Error(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	response.JSON(ctx, fasthttp.StatusOK, user)
}

// ── /api/forgot-password ──────────────────────────────────────────────────────

func forgotPasswordHandler(ctx *fasthttp.RequestCtx) {
	if err := auth.ValidateInternalKey(ctx); err != nil {
		response.Error(ctx, fasthttp.StatusUnauthorized, err.Error())
		return
	}
	if string(ctx.Method()) != fasthttp.MethodPost {
		response.Error(ctx, fasthttp.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var input struct{ Email string `json:"email"` }
	if err := json.Unmarshal(ctx.PostBody(), &input); err != nil {
		response.Error(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		response.Error(ctx, fasthttp.StatusBadRequest, "Email address is required")
		return
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		response.Error(ctx, fasthttp.StatusInternalServerError, "Failed to generate token")
		return
	}
	rawToken := hex.EncodeToString(tokenBytes)
	hasher := sha256.New()
	hasher.Write([]byte(rawToken))
	hashedToken := hex.EncodeToString(hasher.Sum(nil))
	expires := time.Now().Add(1 * time.Hour)
	user, err := models.SetPasswordResetToken(ctx, email, hashedToken, expires)
	if err != nil {
		response.Error(ctx, fasthttp.StatusNotFound, err.Error())
		return
	}
	response.JSON(ctx, fasthttp.StatusOK, map[string]string{
		"message": "A password reset link has been generated.",
		"userName": user.Name, "rawToken": rawToken, "userEmail": user.Email,
	})
}

// ── /api/reset-password ───────────────────────────────────────────────────────

func resetPasswordHandler(ctx *fasthttp.RequestCtx) {
	if err := auth.ValidateInternalKey(ctx); err != nil {
		response.Error(ctx, fasthttp.StatusUnauthorized, err.Error())
		return
	}
	if string(ctx.Method()) != fasthttp.MethodPost {
		response.Error(ctx, fasthttp.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var input struct {
		Token    string `json:"token"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &input); err != nil {
		response.Error(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}
	token := strings.TrimSpace(input.Token)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if token == "" || email == "" || input.Password == "" {
		response.Error(ctx, fasthttp.StatusBadRequest, "Token, email, and password are required")
		return
	}
	if len(input.Password) < 8 {
		response.Error(ctx, fasthttp.StatusBadRequest, "Password must be at least 8 characters long")
		return
	}
	hasher := sha256.New()
	hasher.Write([]byte(token))
	hashedToken := hex.EncodeToString(hasher.Sum(nil))
	if err := models.ResetPasswordByToken(ctx, email, hashedToken, input.Password); err != nil {
		response.Error(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	response.JSON(ctx, fasthttp.StatusOK, map[string]string{
		"message": "Your password has been successfully reset. You can now sign in with your new password.",
	})
}
