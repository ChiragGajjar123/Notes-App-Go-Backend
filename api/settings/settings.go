package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"notes-go-backend/pkg/auth"
	"notes-go-backend/pkg/models"
	"notes-go-backend/pkg/response"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Handler handles user settings theme requests.
func Handler(w http.ResponseWriter, r *http.Request) {
	// Validate internal auth headers
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
		getTheme(w, r, userID)
	case http.MethodPut:
		updateTheme(w, r, userID)
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func getTheme(w http.ResponseWriter, r *http.Request, userID bson.ObjectID) {
	userSettings, err := models.GetUserSettings(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to get settings: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, userSettings)
}

type updateThemeInput struct {
	Theme string `json:"theme"`
}

func updateTheme(w http.ResponseWriter, r *http.Request, userID bson.ObjectID) {
	var input updateThemeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	theme := strings.ToLower(strings.TrimSpace(input.Theme))
	if theme != "light" && theme != "dark" {
		response.Error(w, http.StatusBadRequest, "Theme must be either 'light' or 'dark'")
		return
	}

	updatedTheme, err := models.UpdateTheme(r.Context(), userID, theme)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update theme: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"theme": updatedTheme})
}
