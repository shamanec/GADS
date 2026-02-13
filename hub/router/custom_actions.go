package router

import (
	"GADS/common/api"
	"GADS/common/db"
	"GADS/common/models"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var validActionTypes = map[string]bool{
	"tap":            true,
	"double_tap":     true,
	"swipe":          true,
	"touch_and_hold": true,
	"pinch":          true,
	"type_text":      true,
	"pinch_in":       true,
	"pinch_out":      true,
}

func validateCustomAction(action *models.CustomAction) error {
	if action.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !validActionTypes[action.ActionType] {
		return fmt.Errorf("invalid action type: %s", action.ActionType)
	}
	return validateParameters(action.ActionType, action.Parameters)
}

func validateParameters(actionType string, params map[string]any) error {
	if params == nil {
		params = make(map[string]any)
	}

	switch actionType {
	case "tap", "double_tap":
		if _, ok := params["x"]; !ok {
			return fmt.Errorf("parameter 'x' is required for %s", actionType)
		}
		if _, ok := params["y"]; !ok {
			return fmt.Errorf("parameter 'y' is required for %s", actionType)
		}

		if x, ok := params["x"].(float64); ok && (x < 0 || x > 10000) {
			return fmt.Errorf("x must be between 0 and 10000")
		}
		if y, ok := params["y"].(float64); ok && (y < 0 || y > 10000) {
			return fmt.Errorf("y must be between 0 and 10000")
		}

	case "swipe":
		required := []string{"x", "y", "endX", "endY"}
		for _, p := range required {
			if _, ok := params[p]; !ok {
				return fmt.Errorf("parameter '%s' is required for swipe", p)
			}
		}

		for _, p := range required {
			if val, ok := params[p].(float64); ok && (val < 0 || val > 10000) {
				return fmt.Errorf("%s must be between 0 and 10000", p)
			}
		}

	case "type_text":
		text, ok := params["text"]
		if !ok {
			return fmt.Errorf("parameter 'text' is required for type_text")
		}
		if str, ok := text.(string); ok {
			if len(str) > 500 {
				return fmt.Errorf("text cannot exceed 500 characters")
			}
			params["text"] = sanitizeText(str)
		}

	case "touch_and_hold":
		if _, ok := params["x"]; !ok {
			return fmt.Errorf("parameter 'x' is required for touch_and_hold")
		}
		if _, ok := params["y"]; !ok {
			return fmt.Errorf("parameter 'y' is required for touch_and_hold")
		}

		if x, ok := params["x"].(float64); ok && (x < 0 || x > 10000) {
			return fmt.Errorf("x must be between 0 and 10000")
		}
		if y, ok := params["y"].(float64); ok && (y < 0 || y > 10000) {
			return fmt.Errorf("y must be between 0 and 10000")
		}

		if dur, ok := params["duration"]; ok {
			if d, ok := dur.(float64); ok && d > 10000 {
				return fmt.Errorf("duration cannot exceed 10000ms")
			}
		}

	case "pinch":
		if _, ok := params["scale"]; !ok {
			return fmt.Errorf("parameter 'scale' is required for pinch")
		}

		if scale, ok := params["scale"].(float64); ok && (scale <= 0 || scale > 10) {
			return fmt.Errorf("scale must be between 0.1 and 10")
		}

	case "pinch_in", "pinch_out":
		// No required parameters

	default:
		return fmt.Errorf("unsupported action type: %s", actionType)
	}

	return nil
}

func sanitizeText(text string) string {
	dangerous := []string{"'", "\"", "\\", ";", "<", ">", "&", "|", "$", "`", "\n", "\r"}
	cleaned := text
	for _, char := range dangerous {
		cleaned = strings.ReplaceAll(cleaned, char, "")
	}
	return cleaned
}

func migrateV1Action(action *models.CustomAction) {
	if len(action.Parameters) == 0 {
		switch action.ActionType {
		case "pinch_in":
			action.ActionType = "pinch"
			action.Parameters = map[string]any{"scale": 0.5}
		case "pinch_out":
			action.ActionType = "pinch"
			action.Parameters = map[string]any{"scale": 2.0}
		case "double_tap":
			action.Parameters = map[string]any{}
		}
	}
}

func GetCustomActions(c *gin.Context) {
	tenant := c.GetString("tenant")

	actions, err := db.GlobalMongoStore.GetCustomActions(tenant)
	if err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to get custom actions: %s", err), nil)
		return
	}

	for i := range actions {
		migrateV1Action(&actions[i])
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

	if err := validateCustomAction(&action); err != nil {
		api.GenericResponse(c, http.StatusBadRequest, err.Error(), nil)
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

	if err := validateCustomAction(&updates); err != nil {
		api.GenericResponse(c, http.StatusBadRequest, err.Error(), nil)
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
