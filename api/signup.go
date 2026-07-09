package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"notes-go-backend/pkg/auth"
	"notes-go-backend/pkg/models"
	"notes-go-backend/pkg/ratelimit"
	"notes-go-backend/pkg/response"
)

type signupInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Handler handles user registration validation.
func Handler(w http.ResponseWriter, r *http.Request) {
	// Validate internal key (only internal NextAuth service should call signup)
	if err := auth.ValidateInternalKey(r); err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract Client IP passed by NextAuth proxy
	clientIP := r.Header.Get("X-Client-IP")
	if clientIP == "" {
		clientIP = ratelimit.GetClientIp(r)
	}

	// Enforce Rate Limiting for Signups
	_, _, err := ratelimit.EnforceRateLimit(r.Context(), "signup", clientIP, ratelimit.SignupLimit)
	if err != nil {
		response.Error(w, http.StatusTooManyRequests, err.Error())
		return
	}

	var input signupInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	name := strings.TrimSpace(input.Name)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := input.Password

	if name == "" || email == "" || password == "" {
		response.Error(w, http.StatusBadRequest, "Name, email, and password are required")
		return
	}

	// Basic validation of inputs
	if len(password) < 8 {
		response.Error(w, http.StatusBadRequest, "Password must be at least 8 characters long")
		return
	}

	user, err := models.RegisterUser(r.Context(), name, email, password)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, user)
}
