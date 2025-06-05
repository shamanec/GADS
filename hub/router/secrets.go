/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"GADS/common/db"
	"GADS/hub/auth"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// GetSecretKeys godoc
// @Summary      Get all secret keys
// @Description  Retrieve list of all secret keys in the system
// @Tags         Admin - Secret Keys
// @Accept       json
// @Produce      json
// @Param        status  query  string  false  "Filter by status (active/disabled)"
// @Success      200     {array}  models.SecretKeyResponse
// @Failure      500     {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/secret-keys [get]
func GetSecretKeys(c *gin.Context) {
	// Get secret keys from database
	store := auth.NewSecretStore(db.GlobalMongoStore.GetDefaultDatabase())
	keys, err := store.GetAllSecretKeys()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get secret keys"})
		return
	}

	// Don't expose the actual secret key values in the response for security
	var response []gin.H = []gin.H{}
	for _, key := range keys {
		response = append(response, gin.H{
			"id":                      key.ID.Hex(),
			"origin":                  key.Origin,
			"is_default":              key.IsDefault,
			"created_at":              key.CreatedAt,
			"updated_at":              key.UpdatedAt,
			"user_identifier_claim":   key.UserIdentifierClaim,
			"tenant_identifier_claim": key.TenantIdentifierClaim,
		})
	}

	c.JSON(http.StatusOK, gin.H{"secret_keys": response})
}

// AddSecretKey godoc
// @Summary      Add a new secret key
// @Description  Create a new secret key in the system
// @Tags         Admin - Secret Keys
// @Accept       json
// @Produce      json
// @Param        secretKey  body      models.SecretKeyRequest  true  "Secret key data"
// @Success      200        {object}  models.SecretKeyResponse
// @Failure      400        {object}  models.ErrorResponse
// @Failure      500        {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/secret-keys [post]
func AddSecretKey(c *gin.Context) {
	// Get username from user claims for audit
	username, exists := getUsernameFromContext(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
		return
	}

	// Parse request body
	var request struct {
		Origin                string `json:"origin" binding:"required"`
		Key                   string `json:"key" binding:"required"`
		IsDefault             bool   `json:"is_default"`
		Justification         string `json:"justification" binding:"required"`
		UserIdentifierClaim   string `json:"user_identifier_claim"`
		TenantIdentifierClaim string `json:"tenant_identifier_claim"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Create secret key
	secretKey := &auth.SecretKey{
		Origin:                request.Origin,
		Key:                   request.Key,
		IsDefault:             request.IsDefault,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
		UserIdentifierClaim:   request.UserIdentifierClaim,
		TenantIdentifierClaim: request.TenantIdentifierClaim,
	}

	// Add secret key to database
	store := auth.NewSecretStore(db.GlobalMongoStore.GetDefaultDatabase())
	err := store.AddSecretKey(secretKey, username, request.Justification)
	if err != nil {
		if err == auth.ErrDuplicateOrigin {
			c.JSON(http.StatusBadRequest, gin.H{"error": "An origin with this name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add secret key"})
		return
	}

	// If needed, refresh the JWT key cache
	cache := auth.GetSecretCache()
	if cache != nil {
		cache.Refresh()
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"id":                      secretKey.ID.Hex(),
		"origin":                  secretKey.Origin,
		"is_default":              secretKey.IsDefault,
		"created_at":              secretKey.CreatedAt,
		"updated_at":              secretKey.UpdatedAt,
		"user_identifier_claim":   secretKey.UserIdentifierClaim,
		"tenant_identifier_claim": secretKey.TenantIdentifierClaim,
	})
}

// UpdateSecretKey godoc
// @Summary      Update a secret key
// @Description  Update an existing secret key in the system
// @Tags         Admin - Secret Keys
// @Accept       json
// @Produce      json
// @Param        id         path      string                   true  "Secret key ID"
// @Param        secretKey  body      models.SecretKeyRequest  true  "Secret key data"
// @Success      200        {object}  models.SecretKeyResponse
// @Failure      400        {object}  models.ErrorResponse
// @Failure      404        {object}  models.ErrorResponse
// @Failure      500        {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/secret-keys/{id} [put]
func UpdateSecretKey(c *gin.Context) {
	// Get username from user claims for audit
	username, exists := getUsernameFromContext(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
		return
	}

	// Get ID from path
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// Parse request body
	var request struct {
		Key                   string `json:"key"`
		IsDefault             bool   `json:"is_default"`
		Justification         string `json:"justification" binding:"required"`
		UserIdentifierClaim   string `json:"user_identifier_claim"`
		TenantIdentifierClaim string `json:"tenant_identifier_claim"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get secret key from database
	store := auth.NewSecretStore(db.GlobalMongoStore.GetDefaultDatabase())
	secretKey, err := store.GetSecretKeyByID(objectID)
	if err != nil {
		if err == auth.ErrSecretKeyNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Secret key not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get secret key"})
		return
	}

	// Update secret key
	// Only update the key if a new value was provided
	if request.Key != "" {
		secretKey.Key = request.Key
	}
	secretKey.IsDefault = request.IsDefault
	secretKey.UpdatedAt = time.Now()
	if request.UserIdentifierClaim != "" {
		secretKey.UserIdentifierClaim = request.UserIdentifierClaim
	}
	if request.TenantIdentifierClaim != "" {
		secretKey.TenantIdentifierClaim = request.TenantIdentifierClaim
	}

	// Save secret key to database
	err = store.UpdateSecretKey(secretKey, username, request.Justification)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update secret key"})
		return
	}

	// If needed, refresh the JWT key cache
	cache := auth.GetSecretCache()
	if cache != nil {
		cache.Refresh()
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"id":                      secretKey.ID.Hex(),
		"origin":                  secretKey.Origin,
		"is_default":              secretKey.IsDefault,
		"created_at":              secretKey.CreatedAt,
		"updated_at":              secretKey.UpdatedAt,
		"user_identifier_claim":   secretKey.UserIdentifierClaim,
		"tenant_identifier_claim": secretKey.TenantIdentifierClaim,
	})
}

// DisableSecretKey godoc
// @Summary      Disable a secret key
// @Description  Disable an existing secret key in the system
// @Tags         Admin - Secret Keys
// @Accept       json
// @Produce      json
// @Param        id            path      string                        true  "Secret key ID"
// @Param        justification body      models.JustificationRequest  true  "Justification for disabling"
// @Success      200           {object}  models.SuccessResponse
// @Failure      400           {object}  models.ErrorResponse
// @Failure      404           {object}  models.ErrorResponse
// @Failure      500           {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/secret-keys/{id} [delete]
func DisableSecretKey(c *gin.Context) {
	// Get username from user claims for audit
	username, exists := getUsernameFromContext(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
		return
	}

	// Get ID from path
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// Parse request body to get justification (optional)
	var request struct {
		Justification string `json:"justification" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body or missing justification"})
		return
	}

	// Disable secret key
	store := auth.NewSecretStore(db.GlobalMongoStore.GetDefaultDatabase())
	err = store.DisableSecretKey(objectID, username, request.Justification)
	if err != nil {
		if err == auth.ErrSecretKeyNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Secret key not found"})
			return
		}
		if err == auth.ErrCannotDisableDefault {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot disable the default secret key"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable secret key"})
		return
	}

	// If needed, refresh the JWT key cache
	cache := auth.GetSecretCache()
	if cache != nil {
		cache.Refresh()
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{"message": "Secret key disabled"})
}

// GetSecretKeyHistory godoc
// @Summary      Get secret key history
// @Description  Retrieve audit history of secret key changes
// @Tags         Admin - Secret Keys
// @Accept       json
// @Produce      json
// @Param        page      query  int     false  "Page number (default 1)"
// @Param        limit     query  int     false  "Items per page (default 10, max 100)"
// @Param        origin    query  string  false  "Filter by origin"
// @Param        action    query  string  false  "Filter by action"
// @Param        username  query  string  false  "Filter by username"
// @Param        from_date query  string  false  "Filter from date (RFC3339 format)"
// @Param        to_date   query  string  false  "Filter to date (RFC3339 format)"
// @Success      200       {object}  models.SecretKeyHistoryResponse
// @Failure      500       {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/secret-keys/history [get]
func GetSecretKeyHistory(c *gin.Context) {
	// Extract pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 || limit > 100 {
		limit = 10
	}

	// Prepare filters
	filters := make(map[string]interface{})

	// Add optional filters
	if origin := c.Query("origin"); origin != "" {
		filters["origin"] = origin
	}

	if action := c.Query("action"); action != "" {
		filters["action"] = action
	}

	if username := c.Query("username"); username != "" {
		filters["username"] = username
	}

	// Date filters
	if fromDateStr := c.Query("from_date"); fromDateStr != "" {
		if fromDate, err := time.Parse(time.RFC3339, fromDateStr); err == nil {
			filters["from_date"] = fromDate
		}
	}

	if toDateStr := c.Query("to_date"); toDateStr != "" {
		if toDate, err := time.Parse(time.RFC3339, toDateStr); err == nil {
			filters["to_date"] = toDate
		}
	}

	// Get history
	store := auth.NewSecretStore(db.GlobalMongoStore.GetDefaultDatabase())
	auditStore := store.GetSecretKeyAuditStore()
	logs, total, err := auditStore.GetHistory(page, limit, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit history"})
		return
	}

	// Format response
	response := auth.FormatHistoryResponse(logs, total, page, limit)
	c.JSON(http.StatusOK, response)
}

// GetSecretKeyHistoryByID godoc
// @Summary      Get secret key history by ID
// @Description  Retrieve a specific audit record by ID
// @Tags         Admin - Secret Keys
// @Accept       json
// @Produce      json
// @Param        id  path      string  true  "Audit log ID"
// @Success      200 {object}  models.SecretKeyAuditLogResponse
// @Failure      400 {object}  models.ErrorResponse
// @Failure      404 {object}  models.ErrorResponse
// @Failure      500 {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/secret-keys/history/{id} [get]
func GetSecretKeyHistoryByID(c *gin.Context) {
	// Extract ID from URL
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing log ID"})
		return
	}

	// Convert to ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log ID format"})
		return
	}

	// Fetch audit record
	store := auth.NewSecretStore(db.GlobalMongoStore.GetDefaultDatabase())
	auditStore := store.GetSecretKeyAuditStore()
	log, err := auditStore.GetAuditLogByID(objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Audit log not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit log"})
		}
		return
	}

	// Respond with the record
	c.JSON(http.StatusOK, log)
}

// getUsernameFromContext extracts username from JWT claims or context
func getUsernameFromContext(c *gin.Context) (string, bool) {
	// First, try to get username from context (as configured in AuthMiddleware)
	if username, exists := c.Get("username"); exists {
		if usernameStr, ok := username.(string); ok && usernameStr != "" {
			return usernameStr, true
		}
	}

	// If not found in context, try to get from JWT claims
	if userClaims, exists := c.Get("user"); exists {
		if claims, ok := userClaims.(*auth.JWTClaims); ok {
			// Priority to the Username field
			if claims.Username != "" {
				return claims.Username, true
			}

			// If Username is empty, try to use the Subject field for compatibility
			if claims.Subject != "" {
				return claims.Subject, true
			}
		}
	}

	return "", false
}
