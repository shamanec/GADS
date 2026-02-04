package router

import (
	"GADS/common/api"
	"GADS/common/db"
	"GADS/common/models"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetCustomActions(c *gin.Context) {
	tenant := c.GetString("tenant")

	actions, err := db.GlobalMongoStore.GetCustomActions(tenant)
	if err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to get custom actions: %s", err), nil)
		return
	}

	api.GenericResponse(c, http.StatusOK, "", actions)
}

func CreateCustomAction(c *gin.Context) {
	tenant := c.GetString("tenant")
	username := c.GetString("username")

	var action models.CustomAction
	if err := json.NewDecoder(c.Request.Body).Decode(&action); err != nil {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %s", err), nil)
		return
	}

	if action.Name == "" || action.ActionType == "" {
		api.GenericResponse(c, http.StatusBadRequest, "name and action_type are required", nil)
		return
	}

	validActionTypes := map[string]bool{
		"pinch_in":   true,
		"pinch_out":  true,
		"double_tap": true,
	}
	if !validActionTypes[action.ActionType] {
		api.GenericResponse(c, http.StatusBadRequest, "invalid action_type: must be pinch_in, pinch_out, or double_tap", nil)
		return
	}

	if action.IsFavorite {
		count, err := db.GlobalMongoStore.CountFavoriteActions(tenant)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to count favorites: %s", err), nil)
			return
		}
		if count >= 5 {
			api.GenericResponse(c, http.StatusBadRequest, "maximum of 5 favorite actions allowed", nil)
			return
		}
	}

	action.Tenant = tenant
	action.CreatedBy = username

	if err := db.GlobalMongoStore.CreateCustomAction(&action); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to create custom action: %s", err), nil)
		return
	}

	api.GenericResponse(c, http.StatusCreated, "", action)
}

func UpdateCustomAction(c *gin.Context) {
	tenant := c.GetString("tenant")
	id := c.Param("id")

	existing, err := db.GlobalMongoStore.GetCustomAction(id, tenant)
	if err != nil {
		api.GenericResponse(c, http.StatusNotFound, "custom action not found", nil)
		return
	}

	var updates models.CustomAction
	if err := json.NewDecoder(c.Request.Body).Decode(&updates); err != nil {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %s", err), nil)
		return
	}

	if updates.Name == "" || updates.ActionType == "" {
		api.GenericResponse(c, http.StatusBadRequest, "name and action_type are required", nil)
		return
	}

	validActionTypes := map[string]bool{
		"pinch_in":   true,
		"pinch_out":  true,
		"double_tap": true,
	}
	if !validActionTypes[updates.ActionType] {
		api.GenericResponse(c, http.StatusBadRequest, "invalid action_type: must be pinch_in, pinch_out, or double_tap", nil)
		return
	}

	if updates.IsFavorite && !existing.IsFavorite {
		count, err := db.GlobalMongoStore.CountFavoriteActions(tenant)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to count favorites: %s", err), nil)
			return
		}
		if count >= 5 {
			api.GenericResponse(c, http.StatusBadRequest, "maximum of 5 favorite actions allowed", nil)
			return
		}
	}

	updates.Tenant = tenant
	if err := db.GlobalMongoStore.UpdateCustomAction(id, tenant, &updates); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to update custom action: %s", err), nil)
		return
	}

	updated, _ := db.GlobalMongoStore.GetCustomAction(id, tenant)
	api.GenericResponse(c, http.StatusOK, "", updated)
}

func DeleteCustomAction(c *gin.Context) {
	tenant := c.GetString("tenant")
	id := c.Param("id")

	if err := db.GlobalMongoStore.DeleteCustomAction(id, tenant); err != nil {
		api.GenericResponse(c, http.StatusNotFound, "custom action not found", nil)
		return
	}

	api.GenericResponse(c, http.StatusOK, "custom action deleted", nil)
}
