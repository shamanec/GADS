package router

import (
	"GADS/common/db"
	"GADS/hub/auth"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetSecretKeys returns all secret keys
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
			"id":         key.ID.Hex(),
			"origin":     key.Origin,
			"is_default": key.IsDefault,
			"created_at": key.CreatedAt,
			"updated_at": key.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"secret_keys": response})
}

// AddSecretKey adds a new secret key
func AddSecretKey(c *gin.Context) {
	// Parse request body
	var request struct {
		Origin    string `json:"origin" binding:"required"`
		Key       string `json:"key" binding:"required"`
		IsDefault bool   `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Create secret key
	secretKey := &auth.SecretKey{
		Origin:    request.Origin,
		Key:       request.Key,
		IsDefault: request.IsDefault,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Add secret key to database
	store := auth.NewSecretStore(db.GlobalMongoStore.GetDefaultDatabase())
	err := store.AddSecretKey(secretKey)
	if err != nil {
		if err == auth.ErrDuplicateOrigin {
			c.JSON(http.StatusBadRequest, gin.H{"error": "An origin with this name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add secret key"})
		return
	}

	// Refresh cache
	if auth.GetSecretCache() != nil {
		auth.GetSecretCache().Refresh()
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"id":         secretKey.ID.Hex(),
		"origin":     secretKey.Origin,
		"is_default": secretKey.IsDefault,
		"created_at": secretKey.CreatedAt,
		"updated_at": secretKey.UpdatedAt,
	})
}

// UpdateSecretKey updates a secret key
func UpdateSecretKey(c *gin.Context) {
	// Get ID from path
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// Parse request body
	var request struct {
		Key       string `json:"key"`
		IsDefault bool   `json:"is_default"`
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
	if request.Key != "" {
		secretKey.Key = request.Key
	}
	secretKey.IsDefault = request.IsDefault
	secretKey.UpdatedAt = time.Now()

	// Save secret key to database
	err = store.UpdateSecretKey(secretKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update secret key"})
		return
	}

	// Refresh cache
	if auth.GetSecretCache() != nil {
		auth.GetSecretCache().Refresh()
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"id":         secretKey.ID.Hex(),
		"origin":     secretKey.Origin,
		"is_default": secretKey.IsDefault,
		"created_at": secretKey.CreatedAt,
		"updated_at": secretKey.UpdatedAt,
	})
}

// DisableSecretKey disables a secret key
func DisableSecretKey(c *gin.Context) {
	// Get ID from path
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// Disable secret key
	store := auth.NewSecretStore(db.GlobalMongoStore.GetDefaultDatabase())
	err = store.DisableSecretKey(objectID)
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

	// Refresh cache
	if auth.GetSecretCache() != nil {
		auth.GetSecretCache().Refresh()
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{"message": "Secret key disabled"})
}
