package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"notes-go-backend/pkg/auth"
	"notes-go-backend/pkg/models"
	"notes-go-backend/pkg/ratelimit"
	"notes-go-backend/pkg/response"
)

type resetPasswordInput struct {
	Token    string `json:"token"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ResetPassword handles password reset execution.
func ResetPassword(w http.ResponseWriter, r *http.Request) {
	// Validate internal key
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

	// Enforce rate limit (3 attempts per hour)
	_, _, err := ratelimit.EnforceRateLimit(r.Context(), "reset-password", clientIP, ratelimit.PasswordResetLimit)
	if err != nil {
		response.Error(w, http.StatusTooManyRequests, err.Error())
		return
	}

	var input resetPasswordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	token := strings.TrimSpace(input.Token)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := input.Password

	if token == "" || email == "" || password == "" {
		response.Error(w, http.StatusBadRequest, "Token, email, and password are required")
		return
	}

	if len(password) < 8 {
		response.Error(w, http.StatusBadRequest, "Password must be at least 8 characters long")
		return
	}

	// SHA-256 hash the incoming raw token to compare with DB
	hasher := sha256.New()
	hasher.Write([]byte(token))
	hashedToken := hex.EncodeToString(hasher.Sum(nil))

	if err := models.ResetPasswordByToken(r.Context(), email, hashedToken, password); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Your password has been successfully reset. You can now sign in with your new password.",
	})
}
