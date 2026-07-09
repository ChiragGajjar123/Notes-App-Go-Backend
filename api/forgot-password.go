package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"notes-go-backend/pkg/auth"
	"notes-go-backend/pkg/models"
	"notes-go-backend/pkg/ratelimit"
	"notes-go-backend/pkg/response"
)

type forgotPasswordInput struct {
	Email string `json:"email"`
}

type forgotPasswordResponse struct {
	Message   string `json:"message"`
	UserName  string `json:"userName"`
	RawToken  string `json:"rawToken"`
	UserEmail string `json:"userEmail"`
}

// Handler handles forgot password request token generation.
func Handler(w http.ResponseWriter, r *http.Request) {
	// Validate internal key (only Next.js should call this)
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

	// 1. Enforce cooldown rate limit (1 request per 60s)
	_, _, err := ratelimit.EnforceRateLimit(r.Context(), "forgot-password-cooldown", clientIP, ratelimit.PasswordResetCooldownLimit)
	if err != nil {
		response.Error(w, http.StatusTooManyRequests, "Please wait 60 seconds before requesting another password reset.")
		return
	}

	// 2. Enforce overall rate limit (3 requests per hour)
	_, _, err = ratelimit.EnforceRateLimit(r.Context(), "forgot-password", clientIP, ratelimit.PasswordResetLimit)
	if err != nil {
		response.Error(w, http.StatusTooManyRequests, err.Error())
		return
	}

	var input forgotPasswordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		response.Error(w, http.StatusBadRequest, "Email address is required")
		return
	}

	// Generate random 32-byte secure token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	rawToken := hex.EncodeToString(tokenBytes)

	// SHA-256 hash the token for storage
	hasher := sha256.New()
	hasher.Write([]byte(rawToken))
	hashedToken := hex.EncodeToString(hasher.Sum(nil))

	expires := time.Now().Add(1 * time.Hour) // Valid for 1 hour

	user, err := models.SetPasswordResetToken(r.Context(), email, hashedToken, expires)
	if err != nil {
		response.Error(w, http.StatusNotFound, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, forgotPasswordResponse{
		Message:   "A password reset link has been generated.",
		UserName:  user.Name,
		RawToken:  rawToken,
		UserEmail: user.Email,
	})
}
