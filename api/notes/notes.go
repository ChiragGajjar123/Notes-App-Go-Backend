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

// Handler handles all notes CRUD requests.
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
		getNotes(w, r, userID)
	case http.MethodPost:
		saveNote(w, r, userID)
	case http.MethodDelete:
		deleteNote(w, r, userID)
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func getNotes(w http.ResponseWriter, r *http.Request, userID bson.ObjectID) {
	archivedQuery := r.URL.Query().Get("archived")
	isArchived := strings.ToLower(archivedQuery) == "true"

	notes, err := models.FindNotes(r.Context(), userID, isArchived)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to fetch notes: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, notes)
}

type noteInput struct {
	ID         string   `json:"_id,omitempty"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
	IsPinned   bool     `json:"isPinned"`
	IsArchived bool     `json:"isArchived"`
	Color      string   `json:"color"`
}

func saveNote(w http.ResponseWriter, r *http.Request, userID bson.ObjectID) {
	var input noteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate title is required
	if strings.TrimSpace(input.Title) == "" {
		response.Error(w, http.StatusBadRequest, "Title is required")
		return
	}

	var noteID bson.ObjectID
	if input.ID != "" {
		var err error
		noteID, err = bson.ObjectIDFromHex(input.ID)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid Note ID format")
			return
		}
	}

	note := &models.Note{
		ID:         noteID,
		Title:      input.Title,
		Content:    input.Content,
		Category:   input.Category,
		Tags:       input.Tags,
		IsPinned:   input.IsPinned,
		IsArchived: input.IsArchived,
		Color:      input.Color,
		UserID:     userID,
	}

	savedNote, err := models.SaveNote(r.Context(), note)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to save note: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, savedNote)
}

func deleteNote(w http.ResponseWriter, r *http.Request, userID bson.ObjectID) {
	noteIDStr := r.URL.Query().Get("id")
	if noteIDStr == "" {
		response.Error(w, http.StatusBadRequest, "Missing note id parameter")
		return
	}

	noteID, err := bson.ObjectIDFromHex(noteIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid Note ID format")
		return
	}

	err = models.DeleteNote(r.Context(), noteID, userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to delete note: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
