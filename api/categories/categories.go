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

// Handler handles all categories CRUD requests.
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
		getCategories(w, r, userID)
	case http.MethodPost:
		createCategory(w, r, userID)
	case http.MethodPut:
		renameCategory(w, r, userID)
	case http.MethodDelete:
		deleteCategory(w, r, userID)
	default:
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func getCategories(w http.ResponseWriter, r *http.Request, userID bson.ObjectID) {
	categories, err := models.GetCategories(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to get categories: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, categories)
}

type createCatInput struct {
	Name string `json:"name"`
}

func createCategory(w http.ResponseWriter, r *http.Request, userID bson.ObjectID) {
	var input createCatInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	trimmedName := strings.TrimSpace(input.Name)
	if trimmedName == "" {
		response.Error(w, http.StatusBadRequest, "Category name is required")
		return
	}

	categories, err := models.CreateCategory(r.Context(), userID, trimmedName)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create category: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, categories)
}

type renameCatInput struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

func renameCategory(w http.ResponseWriter, r *http.Request, userID bson.ObjectID) {
	var input renameCatInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	oldName := strings.TrimSpace(input.OldName)
	newName := strings.TrimSpace(input.NewName)

	if oldName == "" || newName == "" {
		response.Error(w, http.StatusBadRequest, "Old and new category names are required")
		return
	}

	err := models.RenameCategory(r.Context(), userID, oldName, newName)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to rename category: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func deleteCategory(w http.ResponseWriter, r *http.Request, userID bson.ObjectID) {
	name := r.URL.Query().Get("name")
	if name == "" {
		response.Error(w, http.StatusBadRequest, "Missing category name parameter")
		return
	}

	err := models.DeleteCategory(r.Context(), userID, name)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to delete category: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
