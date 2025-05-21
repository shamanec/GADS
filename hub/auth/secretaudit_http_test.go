package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupTestRouter(secretStore *SecretStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create a mock middleware that puts the URL parameter ID in the context
	router.Use(func(c *gin.Context) {
		// Simulate the behavior that getURLParam expects
		idParam := c.Param("id")
		if idParam != "" {
			vars := map[string]string{"id": idParam}
			c.Set("urlParams", vars)
		}
		c.Next()
	})

	// Add routes
	router.GET("/admin/secret-keys/history", gin.WrapF(AdminSecretKeyHistoryHandler(secretStore)))
	router.GET("/admin/secret-keys/history/:id", gin.WrapF(AdminSecretKeyHistoryByIDHandler(secretStore)))

	return router
}

func setupTestDatabase() (*mongo.Database, *SecretStore, func()) {
	// Setup test database
	ctx := context.Background()
	client, _ := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))

	// Use a separate test database
	dbName := "gads_test_" + primitive.NewObjectID().Hex()
	db := client.Database(dbName)

	// Create stores
	secretStore := NewSecretStore(db)

	// Create indexes for the audit collection
	auditStore := secretStore.GetSecretKeyAuditStore()
	auditStore.CreateMongoIndexes()

	// Cleanup function to be called at the end of the test
	cleanup := func() {
		db.Drop(ctx)
		client.Disconnect(ctx)
	}

	return db, secretStore, cleanup
}

func TestAdminSecretKeyHistoryHandler(t *testing.T) {
	// Setup test
	db, secretStore, cleanup := setupTestDatabase()
	defer cleanup()

	// Add some test records to the audit collection
	logs := []SecretKeyAuditLog{
		{
			ID:            primitive.NewObjectID(),
			Username:      "user1@example.com",
			SecretKeyID:   primitive.NewObjectID(),
			Origin:        "test-origin-1",
			Action:        "create",
			Timestamp:     time.Now().Add(-1 * time.Hour),
			IsDefault:     true,
			Justification: "Test log 1",
		},
		{
			ID:            primitive.NewObjectID(),
			Username:      "user2@example.com",
			SecretKeyID:   primitive.NewObjectID(),
			Origin:        "test-origin-2",
			Action:        "update",
			Timestamp:     time.Now().Add(-2 * time.Hour),
			IsDefault:     false,
			Justification: "Test log 2",
		},
	}

	// Insert test records
	for _, log := range logs {
		_, _ = db.Collection(secretKeyAuditLogsCollection).InsertOne(context.Background(), log)
	}

	// Create test router
	router := setupTestRouter(secretStore)

	t.Run("Should return all logs", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/secret-keys/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		// Verify the response
		items, ok := response["items"].([]interface{})
		assert.True(t, ok)
		assert.Equal(t, 2, len(items))
		assert.Equal(t, float64(2), response["total"])
		assert.Equal(t, float64(1), response["page"])
		assert.Equal(t, float64(10), response["limit"])
	})

	t.Run("Should filter by origin", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/secret-keys/history?origin=test-origin-1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		// Verify the response
		items, ok := response["items"].([]interface{})
		assert.True(t, ok)
		assert.Equal(t, 1, len(items))
		assert.Equal(t, float64(1), response["total"])
	})

	t.Run("Should filter by action", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/secret-keys/history?action=update", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		// Verify the response
		items, ok := response["items"].([]interface{})
		assert.True(t, ok)
		assert.Equal(t, 1, len(items))
		assert.Equal(t, float64(1), response["total"])
	})
}

func TestAdminSecretKeyHistoryByIDHandler(t *testing.T) {
	// Setup test
	db, secretStore, cleanup := setupTestDatabase()
	defer cleanup()

	// Create a known ID for testing
	logID := primitive.NewObjectID()

	// Add a test record with the known ID
	log := SecretKeyAuditLog{
		ID:            logID,
		Username:      "user1@example.com",
		SecretKeyID:   primitive.NewObjectID(),
		Origin:        "test-origin",
		Action:        "create",
		Timestamp:     time.Now(),
		IsDefault:     true,
		Justification: "Test log for ID lookup",
	}

	// Insert test record
	_, _ = db.Collection(secretKeyAuditLogsCollection).InsertOne(context.Background(), log)

	// Create test router
	router := setupTestRouter(secretStore)

	t.Run("Should return a specific log by ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/secret-keys/history/"+logID.Hex(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response SecretKeyAuditLog
		json.Unmarshal(w.Body.Bytes(), &response)

		// Verify the response
		assert.Equal(t, logID.Hex(), response.ID.Hex())
		assert.Equal(t, "user1@example.com", response.Username)
		assert.Equal(t, "test-origin", response.Origin)
		assert.Equal(t, "create", response.Action)
		assert.Equal(t, "Test log for ID lookup", response.Justification)
	})

	t.Run("Should return 404 for invalid ID", func(t *testing.T) {
		invalidID := primitive.NewObjectID().Hex()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/secret-keys/history/"+invalidID, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		// Verify the response
		assert.Equal(t, "Audit log not found", response["error"])
	})
}
