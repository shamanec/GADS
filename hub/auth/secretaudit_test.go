package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestSecretKeyAudit tests the secret key audit functionality
func TestSecretKeyAudit(t *testing.T) {
	// Setup test database
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// Use a separate test database
	dbName := "gads_test_" + primitive.NewObjectID().Hex()
	db := client.Database(dbName)
	defer db.Drop(ctx)

	// Create stores
	secretStore := NewSecretStore(db)
	auditStore := secretStore.GetSecretKeyAuditStore()

	// Create indexes for the audit collection
	err = auditStore.CreateMongoIndexes()
	assert.NoError(t, err, "Failed to create audit log indexes")

	// Test 1: Adding a Secret Key should create an audit log
	t.Run("AddSecretKey should create an audit log", func(t *testing.T) {
		// Create a new Secret Key
		key := &SecretKey{
			Origin:    "test-origin",
			Key:       "test-key-1",
			IsDefault: true,
		}

		// Add the key
		err := secretStore.AddSecretKey(key, "testuser@example.com", "Initial setup")
		assert.NoError(t, err, "Failed to add secret key")

		// Verify that the audit log was created
		filter := bson.M{
			"secret_key_id": key.ID,
			"action":        "create",
		}

		var log SecretKeyAuditLog
		err = db.Collection(secretKeyAuditLogsCollection).FindOne(ctx, filter).Decode(&log)
		assert.NoError(t, err, "Audit log not found")

		// Verify log fields
		assert.Equal(t, "testuser@example.com", log.Username)
		assert.Equal(t, "test-origin", log.Origin)
		assert.Equal(t, "create", log.Action)
		assert.True(t, log.IsDefault)
		assert.Equal(t, "Initial setup", log.Justification)
		assert.NotNil(t, log.NewKey)
		assert.Equal(t, "test-key-1", *log.NewKey)
	})

	// Test 2: Updating a Secret Key should create an audit log
	t.Run("UpdateSecretKey should create an audit log", func(t *testing.T) {
		// Find the key created in the previous test
		filter := bson.M{"origin": "test-origin"}
		var key SecretKey
		err := db.Collection(secretKeysCollection).FindOne(ctx, filter).Decode(&key)
		assert.NoError(t, err, "Failed to find secret key")

		// Update the key
		key.Key = "test-key-2"
		err = secretStore.UpdateSecretKey(&key, "admin@example.com", "Key rotation")
		assert.NoError(t, err, "Failed to update secret key")

		// Verify that the audit log was created
		filter = bson.M{
			"secret_key_id": key.ID,
			"action":        "update",
		}

		var log SecretKeyAuditLog
		err = db.Collection(secretKeyAuditLogsCollection).FindOne(ctx, filter).Decode(&log)
		assert.NoError(t, err, "Audit log not found")

		// Verify log fields
		assert.Equal(t, "admin@example.com", log.Username)
		assert.Equal(t, "update", log.Action)
		assert.Equal(t, "Key rotation", log.Justification)
		assert.NotNil(t, log.PreviousKey)
		assert.Equal(t, "test-key-1", *log.PreviousKey)
		assert.NotNil(t, log.NewKey)
		assert.Equal(t, "test-key-2", *log.NewKey)
	})

	// Test 3: Disabling a Secret Key should create an audit log
	t.Run("DisableSecretKey should create an audit log", func(t *testing.T) {
		// Create a new non-default Secret Key
		key := &SecretKey{
			Origin:    "test-origin-2",
			Key:       "test-key-3",
			IsDefault: false,
		}

		err := secretStore.AddSecretKey(key, "testuser@example.com", "Second key")
		assert.NoError(t, err, "Failed to add secret key")

		// Disable the key
		err = secretStore.DisableSecretKey(key.ID, "security@example.com", "Key compromised")
		assert.NoError(t, err, "Failed to disable secret key")

		// Verify that the audit log was created
		filter := bson.M{
			"secret_key_id": key.ID,
			"action":        "disable",
		}

		var log SecretKeyAuditLog
		err = db.Collection(secretKeyAuditLogsCollection).FindOne(ctx, filter).Decode(&log)
		assert.NoError(t, err, "Audit log not found")

		// Verify log fields
		assert.Equal(t, "security@example.com", log.Username)
		assert.Equal(t, "disable", log.Action)
		assert.Equal(t, "Key compromised", log.Justification)
		assert.NotNil(t, log.PreviousKey)
		assert.Equal(t, "test-key-3", *log.PreviousKey)
		assert.Nil(t, log.NewKey)
	})

	// Test 4: Get audit history with filters
	t.Run("GetHistory should return filtered logs", func(t *testing.T) {
		// Add a few more logs to test filtering
		for i := 0; i < 5; i++ {
			log := &SecretKeyAuditLog{
				Username:      "test@example.com",
				SecretKeyID:   primitive.NewObjectID(),
				Origin:        "test-origin-extra",
				Action:        "create",
				Timestamp:     time.Now().Add(time.Duration(-i) * time.Hour),
				Justification: "Test log",
			}

			err := auditStore.LogAction(log)
			assert.NoError(t, err, "Failed to add test audit log")
		}

		// Test filtering by origin
		logs, total, err := auditStore.GetHistory(1, 10, map[string]interface{}{
			"origin": "test-origin-extra",
		})
		assert.NoError(t, err, "Failed to get history")
		assert.Equal(t, int64(5), total, "Should have found 5 logs")
		assert.Len(t, logs, 5, "Should have returned 5 logs")

		// Test filtering by action
		logs, total, err = auditStore.GetHistory(1, 10, map[string]interface{}{
			"action": "update",
		})
		assert.NoError(t, err, "Failed to get history")
		assert.Equal(t, int64(1), total, "Should have found 1 log")
		assert.Len(t, logs, 1, "Should have returned 1 log")

		// Test pagination
		logs, total, err = auditStore.GetHistory(1, 2, map[string]interface{}{})
		assert.NoError(t, err, "Failed to get history")
		assert.Equal(t, 2, len(logs), "Should have returned 2 logs")

		// Second page
		logs2, _, err := auditStore.GetHistory(2, 2, map[string]interface{}{})
		assert.NoError(t, err, "Failed to get history")
		assert.Equal(t, 2, len(logs2), "Should have returned 2 logs")
		assert.NotEqual(t, logs[0].ID, logs2[0].ID, "Should return different logs on different pages")
	})

	// Test 5: Get a specific audit log by ID
	t.Run("GetAuditLogByID should return a specific log", func(t *testing.T) {
		// Add a log for testing
		id := primitive.NewObjectID()
		log := &SecretKeyAuditLog{
			ID:            id,
			Username:      "test@example.com",
			SecretKeyID:   primitive.NewObjectID(),
			Origin:        "test-origin-id",
			Action:        "create",
			Timestamp:     time.Now(),
			Justification: "Test log by ID",
		}

		err := auditStore.LogAction(log)
		assert.NoError(t, err, "Failed to add test audit log")

		// Get the log by ID
		retrievedLog, err := auditStore.GetAuditLogByID(id)
		assert.NoError(t, err, "Failed to get audit log by ID")
		assert.Equal(t, id, retrievedLog.ID, "Should return log with correct ID")
		assert.Equal(t, "test-origin-id", retrievedLog.Origin, "Should return log with correct origin")
		assert.Equal(t, "Test log by ID", retrievedLog.Justification, "Should return log with correct justification")
	})
}
