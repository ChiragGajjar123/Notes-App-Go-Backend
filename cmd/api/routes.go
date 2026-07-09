package main

// This file wires the local dev server routes to the same logic
// used by each Vercel serverless function in api/*.go.
// Each Vercel function exports Handler; here we call the shared pkg/ logic directly.

import (
	"encoding/json"
	"net/http"
	"strings"

	"notes-go-backend/pkg/auth"
	"notes-go-backend/pkg/models"
	"notes-go-backend/pkg/ratelimit"
	"notes-go-backend/pkg/response"

	"go.mongodb.org/mongo-driver/v2/bson"

	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// ── /api/categories ──────────────────────────────────────────────────────────

func categoriesHandler(w http.ResponseWriter, r *http.Request) {
	userIDStr, err := auth.ValidateInternalRequest(r)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid User ID format")
		return
	}
	switch r.Method {
	case http.MethodGet:
		cats, err := models.GetCategories(r.Context(), userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to get categories: "+err.Error())
			return
		}
		response.JSON(w, http.StatusOK, cats)
	case http.MethodPost:
		var input struct{ Name string `json:"name"` }
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		name := strings.TrimSpace(input.Name)
		if name == "" {
			response.Error(w, http.StatusBadRequest, "Category name is required")
			return
		}
		cats, err := models.CreateCategory(r.Context(), userID, name)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to create category: "+err.Error())
			return
		}
		response.JSON(w, http.StatusOK, cats)
	case http.MethodPut:
		var input struct {
			OldName string `json:"oldName"`
			NewName string `json:"newName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if strings.TrimSpace(input.OldName) == "" || strings.TrimSpace(input.NewName) == "" {
			response.Error(w, http.StatusBadRequest, "Old and new category names are required")
			return
		}
		if err := models.RenameCategory(r.Context(), userID, strings.TrimSpace(input.OldName), strings.TrimSpace(input.NewName)); err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to rename category: "+err.Error())
			return
		}
		response.JSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	case http.MethodDelete:
		name := r.URL.Query().Get("name")
		if name == "" {
			response.Error(w, http.StatusBadRequest, "Missing category name parameter")
			return
		}
		if err := models.DeleteCategory(r.Context(), userID, name); err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to delete category: "+err.Error())
			return
		}
		response.JSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ── /api/notes ────────────────────────────────────────────────────────────────

func notesHandler(w http.ResponseWriter, r *http.Request) {
	userIDStr, err := auth.ValidateInternalRequest(r)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid User ID format")
		return
	}
	switch r.Method {
	case http.MethodGet:
		isArchived := strings.ToLower(r.URL.Query().Get("archived")) == "true"
		notes, err := models.FindNotes(r.Context(), userID, isArchived)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to fetch notes: "+err.Error())
			return
		}
		response.JSON(w, http.StatusOK, notes)
	case http.MethodPost:
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
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if strings.TrimSpace(input.Title) == "" {
			response.Error(w, http.StatusBadRequest, "Title is required")
			return
		}
		var noteID bson.ObjectID
		if input.ID != "" {
			noteID, err = bson.ObjectIDFromHex(input.ID)
			if err != nil {
				response.Error(w, http.StatusBadRequest, "Invalid Note ID format")
				return
			}
		}
		note := &models.Note{
			ID: noteID, Title: input.Title, Content: input.Content,
			Category: input.Category, Tags: input.Tags, IsPinned: input.IsPinned,
			IsArchived: input.IsArchived, Color: input.Color, UserID: userID,
		}
		saved, err := models.SaveNote(r.Context(), note)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to save note: "+err.Error())
			return
		}
		response.JSON(w, http.StatusOK, saved)
	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			response.Error(w, http.StatusBadRequest, "Missing note id parameter")
			return
		}
		noteID, err := bson.ObjectIDFromHex(idStr)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid Note ID format")
			return
		}
		if err := models.DeleteNote(r.Context(), noteID, userID); err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to delete note: "+err.Error())
			return
		}
		response.JSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ── /api/settings ─────────────────────────────────────────────────────────────

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	userIDStr, err := auth.ValidateInternalRequest(r)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid User ID format")
		return
	}
	switch r.Method {
	case http.MethodGet:
		s, err := models.GetUserSettings(r.Context(), userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to get settings: "+err.Error())
			return
		}
		response.JSON(w, http.StatusOK, s)
	case http.MethodPut:
		var input struct{ Theme string `json:"theme"` }
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		theme := strings.ToLower(strings.TrimSpace(input.Theme))
		if theme != "light" && theme != "dark" {
			response.Error(w, http.StatusBadRequest, "Theme must be either 'light' or 'dark'")
			return
		}
		updated, err := models.UpdateTheme(r.Context(), userID, theme)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to update theme: "+err.Error())
			return
		}
		response.JSON(w, http.StatusOK, map[string]string{"theme": updated})
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ── /api/signin ───────────────────────────────────────────────────────────────

func signinHandler(w http.ResponseWriter, r *http.Request) {
	if err := auth.ValidateInternalKey(r); err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	clientIP := r.Header.Get("X-Client-IP")
	if clientIP == "" {
		clientIP = ratelimit.GetClientIp(r)
	}
	if _, _, err := ratelimit.EnforceRateLimit(r.Context(), "login", clientIP, ratelimit.LoginLimit); err != nil {
		response.Error(w, http.StatusTooManyRequests, err.Error())
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" || input.Password == "" {
		response.Error(w, http.StatusBadRequest, "Email and password are required")
		return
	}
	user, err := models.AuthenticateUser(r.Context(), email, input.Password)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, user)
}

// ── /api/signup ───────────────────────────────────────────────────────────────

func signupHandler(w http.ResponseWriter, r *http.Request) {
	if err := auth.ValidateInternalKey(r); err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	clientIP := r.Header.Get("X-Client-IP")
	if clientIP == "" {
		clientIP = ratelimit.GetClientIp(r)
	}
	if _, _, err := ratelimit.EnforceRateLimit(r.Context(), "signup", clientIP, ratelimit.SignupLimit); err != nil {
		response.Error(w, http.StatusTooManyRequests, err.Error())
		return
	}
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	name := strings.TrimSpace(input.Name)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if name == "" || email == "" || input.Password == "" {
		response.Error(w, http.StatusBadRequest, "Name, email, and password are required")
		return
	}
	if len(input.Password) < 8 {
		response.Error(w, http.StatusBadRequest, "Password must be at least 8 characters long")
		return
	}
	user, err := models.RegisterUser(r.Context(), name, email, input.Password)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, user)
}

// ── /api/forgot-password ──────────────────────────────────────────────────────

func forgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if err := auth.ValidateInternalKey(r); err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	clientIP := r.Header.Get("X-Client-IP")
	if clientIP == "" {
		clientIP = ratelimit.GetClientIp(r)
	}
	if _, _, err := ratelimit.EnforceRateLimit(r.Context(), "forgot-password-cooldown", clientIP, ratelimit.PasswordResetCooldownLimit); err != nil {
		response.Error(w, http.StatusTooManyRequests, "Please wait 60 seconds before requesting another password reset.")
		return
	}
	if _, _, err := ratelimit.EnforceRateLimit(r.Context(), "forgot-password", clientIP, ratelimit.PasswordResetLimit); err != nil {
		response.Error(w, http.StatusTooManyRequests, err.Error())
		return
	}
	var input struct{ Email string `json:"email"` }
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		response.Error(w, http.StatusBadRequest, "Email address is required")
		return
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	rawToken := hex.EncodeToString(tokenBytes)
	hasher := sha256.New()
	hasher.Write([]byte(rawToken))
	hashedToken := hex.EncodeToString(hasher.Sum(nil))
	expires := time.Now().Add(1 * time.Hour)
	user, err := models.SetPasswordResetToken(r.Context(), email, hashedToken, expires)
	if err != nil {
		response.Error(w, http.StatusNotFound, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{
		"message": "A password reset link has been generated.",
		"userName": user.Name, "rawToken": rawToken, "userEmail": user.Email,
	})
}

// ── /api/reset-password ───────────────────────────────────────────────────────

func resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if err := auth.ValidateInternalKey(r); err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	clientIP := r.Header.Get("X-Client-IP")
	if clientIP == "" {
		clientIP = ratelimit.GetClientIp(r)
	}
	if _, _, err := ratelimit.EnforceRateLimit(r.Context(), "reset-password", clientIP, ratelimit.PasswordResetLimit); err != nil {
		response.Error(w, http.StatusTooManyRequests, err.Error())
		return
	}
	var input struct {
		Token    string `json:"token"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	token := strings.TrimSpace(input.Token)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if token == "" || email == "" || input.Password == "" {
		response.Error(w, http.StatusBadRequest, "Token, email, and password are required")
		return
	}
	if len(input.Password) < 8 {
		response.Error(w, http.StatusBadRequest, "Password must be at least 8 characters long")
		return
	}
	hasher := sha256.New()
	hasher.Write([]byte(token))
	hashedToken := hex.EncodeToString(hasher.Sum(nil))
	if err := models.ResetPasswordByToken(r.Context(), email, hashedToken, input.Password); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Your password has been successfully reset. You can now sign in with your new password.",
	})
}
