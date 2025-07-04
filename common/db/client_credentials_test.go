/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package db

import (
	"GADS/hub/auth/clientcredentials"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestCreateClientCredential(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("success", func(mt *mtest.T) {
		// Mock the InsertOne response
		insertOneResult := mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "ok", Value: 1},
		)
		mt.AddMockResponses(insertOneResult)

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		credential, err := store.CreateClientCredential(
			"Test Client",
			"Test Description",
			"user123",
			"test-tenant",
		)

		require.NoError(t, err)
		assert.NotEmpty(t, credential.ID)
		assert.NotEmpty(t, credential.ClientID)
		assert.Contains(t, credential.ClientID, "gads_")
		assert.NotEmpty(t, credential.ClientSecret)
		assert.Equal(t, "Test Client", credential.Name)
		assert.Equal(t, "user123", credential.UserID)
		assert.Equal(t, "test-tenant", credential.Tenant)
		assert.True(t, credential.IsActive)
	})
}

func TestGetClientCredential(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("success", func(mt *mtest.T) {
		clientID := "gads_123456"
		clientSecretHash, _ := clientcredentials.HashSecret("test-secret")
		now := time.Now()

		// Mock the FindOne response
		findResponse := mtest.CreateCursorResponse(
			1,
			"gads_test.client_credentials",
			mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: primitive.NewObjectID()},
				{Key: "client_id", Value: clientID},
				{Key: "client_secret", Value: clientSecretHash},
				{Key: "name", Value: "Test Client"},
				{Key: "description", Value: "Test Description"},
				{Key: "user_id", Value: "user123"},
				{Key: "tenant", Value: "test-tenant"},
				{Key: "is_active", Value: true},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		)
		mt.AddMockResponses(findResponse)

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		retrieved, err := store.GetClientCredential(clientID)
		require.NoError(t, err)

		assert.Equal(t, clientID, retrieved.ClientID)
		assert.Equal(t, "Test Client", retrieved.Name)
		assert.Equal(t, "user123", retrieved.UserID)
		assert.Equal(t, "test-tenant", retrieved.Tenant)
		assert.Equal(t, clientSecretHash, retrieved.ClientSecret)
	})

	mt.Run("not_found", func(mt *mtest.T) {
		// Mock a not found response
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "not found",
		}))

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		_, err := store.GetClientCredential("non_existent_id")
		require.Error(t, err)
	})
}

func TestValidateClientCredentials(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("valid_credentials", func(mt *mtest.T) {
		clientID := "gads_123456"
		clientSecret := "test-secret"
		clientSecretHash, _ := clientcredentials.HashSecret(clientSecret)
		now := time.Now()

		// Mock the FindOne response for GetClientCredential
		findResponse := mtest.CreateCursorResponse(
			1,
			"gads_test.client_credentials",
			mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: primitive.NewObjectID()},
				{Key: "client_id", Value: clientID},
				{Key: "client_secret", Value: clientSecretHash},
				{Key: "name", Value: "Test Client"},
				{Key: "description", Value: "Test Description"},
				{Key: "user_id", Value: "user123"},
				{Key: "tenant", Value: "test-tenant"},
				{Key: "is_active", Value: true},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		)

		// Mock the UpdateOne response for UpdateClientCredentialLastUsed
		updateResponse := mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
			bson.E{Key: "ok", Value: 1},
		)

		// Add both responses
		mt.AddMockResponses(findResponse, updateResponse)

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		validated, err := store.ValidateClientCredentials(clientID, clientSecret)
		require.NoError(t, err)
		assert.Equal(t, clientID, validated.ClientID)
	})

	mt.Run("invalid_secret", func(mt *mtest.T) {
		clientID := "gads_123456"
		clientSecret := "test-secret"
		clientSecretHash, _ := clientcredentials.HashSecret("different-secret") // Different secret
		now := time.Now()

		// Mock the FindOne response
		findResponse := mtest.CreateCursorResponse(
			1,
			"gads_test.client_credentials",
			mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: primitive.NewObjectID()},
				{Key: "client_id", Value: clientID},
				{Key: "client_secret", Value: clientSecretHash},
				{Key: "name", Value: "Test Client"},
				{Key: "description", Value: "Test Description"},
				{Key: "user_id", Value: "user123"},
				{Key: "tenant", Value: "test-tenant"},
				{Key: "is_active", Value: true},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		)
		mt.AddMockResponses(findResponse)

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		_, err := store.ValidateClientCredentials(clientID, clientSecret)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid client credentials")
	})

	mt.Run("invalid_client_id", func(mt *mtest.T) {
		// Mock a not found response
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "not found",
		}))

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		_, err := store.ValidateClientCredentials("wrong-id", "any-secret")
		require.Error(t, err)
	})

	mt.Run("inactive_credential", func(mt *mtest.T) {
		clientID := "gads_123456"
		clientSecret := "test-secret"
		clientSecretHash, _ := clientcredentials.HashSecret(clientSecret)
		now := time.Now()

		// Mock the FindOne response with inactive credential
		findResponse := mtest.CreateCursorResponse(
			1,
			"gads_test.client_credentials",
			mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: primitive.NewObjectID()},
				{Key: "client_id", Value: clientID},
				{Key: "client_secret", Value: clientSecretHash},
				{Key: "name", Value: "Test Client"},
				{Key: "description", Value: "Test Description"},
				{Key: "user_id", Value: "user123"},
				{Key: "tenant", Value: "test-tenant"},
				{Key: "is_active", Value: false}, // Inactive
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		)
		mt.AddMockResponses(findResponse)

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		_, err := store.ValidateClientCredentials(clientID, clientSecret)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inactive")
	})
}

func TestDeactivateClientCredential(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("success", func(mt *mtest.T) {
		clientID := "gads_123456"

		// Mock the UpdateOne response
		updateResponse := mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
			bson.E{Key: "ok", Value: 1},
		)
		mt.AddMockResponses(updateResponse)

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		err := store.DeactivateClientCredential(clientID)
		require.NoError(t, err)
	})
}

func TestGetClientCredentialsByUser(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("success", func(mt *mtest.T) {
		userID := "user123"
		now := time.Now()
		objID1 := primitive.NewObjectID()
		objID2 := primitive.NewObjectID()

		// Create mock response for Find operation with two credentials
		firstBatch := mtest.CreateCursorResponse(
			1,
			"gads_test.client_credentials",
			mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: objID1},
				{Key: "client_id", Value: "gads_123456"},
				{Key: "client_secret", Value: "hashed_secret_1"},
				{Key: "name", Value: "Client 1"},
				{Key: "description", Value: "Desc 1"},
				{Key: "user_id", Value: userID},
				{Key: "tenant", Value: "tenant1"},
				{Key: "is_active", Value: true},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
			bson.D{
				{Key: "_id", Value: objID2},
				{Key: "client_id", Value: "gads_789012"},
				{Key: "client_secret", Value: "hashed_secret_2"},
				{Key: "name", Value: "Client 2"},
				{Key: "description", Value: "Desc 2"},
				{Key: "user_id", Value: userID},
				{Key: "tenant", Value: "tenant1"},
				{Key: "is_active", Value: true},
				{Key: "created_at", Value: now},
				{Key: "updated_at", Value: now},
			},
		)

		// Create end of cursor response
		killCursors := mtest.CreateCursorResponse(
			0,
			"gads_test.client_credentials",
			mtest.NextBatch,
		)

		mt.AddMockResponses(firstBatch, killCursors)

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		credentials, err := store.GetClientCredentialsByUser(userID)
		require.NoError(t, err)
		assert.Len(t, credentials, 2)

		for _, cred := range credentials {
			assert.Equal(t, userID, cred.UserID)
			assert.True(t, cred.IsActive)
		}
	})

	mt.Run("no_credentials", func(mt *mtest.T) {
		// Create empty cursor response
		emptyBatch := mtest.CreateCursorResponse(
			0,
			"gads_test.client_credentials",
			mtest.FirstBatch,
		)

		mt.AddMockResponses(emptyBatch)

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		credentials, err := store.GetClientCredentialsByUser("user_with_no_credentials")
		require.NoError(t, err)
		assert.Len(t, credentials, 0)
	})
}

func TestClientIDGeneration(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("default_prefix_and_uniqueness", func(mt *mtest.T) {
		// Mock two InsertOne responses
		insertResponse1 := mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "ok", Value: 1},
		)
		insertResponse2 := mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "ok", Value: 1},
		)
		mt.AddMockResponses(insertResponse1, insertResponse2)

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		// Test default prefix
		cred1, err := store.CreateClientCredential("Test 1", "Desc 1", "user1", "tenant1")
		require.NoError(t, err)
		assert.Contains(t, cred1.ClientID, "gads_")

		// Create another to test uniqueness
		cred2, err := store.CreateClientCredential("Test 2", "Desc 2", "user1", "tenant1")
		require.NoError(t, err)
		assert.Contains(t, cred2.ClientID, "gads_")

		// Should be unique
		assert.NotEqual(t, cred1.ClientID, cred2.ClientID)
	})
}

func TestClientIDPrefixConfiguration(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("custom_prefix", func(mt *mtest.T) {
		// Mock InsertOne response
		insertResponse := mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "ok", Value: 1},
		)
		mt.AddMockResponses(insertResponse)

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		// Test custom prefix
		t.Setenv("GADS_CLIENT_ID_PREFIX", "myorg")
		cred, err := store.CreateClientCredential("Test", "Desc", "user1", "tenant1")
		require.NoError(t, err)
		assert.Contains(t, cred.ClientID, "myorg_")
		assert.NotContains(t, cred.ClientID, "gads_")
	})

	mt.Run("empty_prefix_fallback", func(mt *mtest.T) {
		// Mock InsertOne response
		insertResponse := mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "ok", Value: 1},
		)
		mt.AddMockResponses(insertResponse)

		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		// Test empty prefix falls back to default
		t.Setenv("GADS_CLIENT_ID_PREFIX", "")
		cred, err := store.CreateClientCredential("Test", "Desc", "user1", "tenant1")
		require.NoError(t, err)
		assert.Contains(t, cred.ClientID, "gads_")
	})
}

func TestClientSecretValidation(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("end_to_end_validation", func(mt *mtest.T) {
		// Mock InsertOne response for CreateClientCredential
		insertResponse := mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "ok", Value: 1},
		)
		
		store := &MongoStore{
			Client:       mt.Client,
			DatabaseName: "gads_test",
			Ctx:          context.Background(),
		}

		// First, create a credential and capture the generated secret
		mt.AddMockResponses(insertResponse)
		cred, err := store.CreateClientCredential("Test", "Desc", "user1", "tenant1")
		require.NoError(t, err)
		
		// The stored hash should validate with the returned secret
		generatedSecret := cred.ClientSecret
		storedHash, _ := clientcredentials.HashSecret(generatedSecret)
		
		// Mock FindOne for successful validation
		findResponse := mtest.CreateCursorResponse(
			1,
			"gads_test.client_credentials",
			mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: primitive.NewObjectID()},
				{Key: "client_id", Value: cred.ClientID},
				{Key: "client_secret", Value: storedHash},
				{Key: "name", Value: "Test"},
				{Key: "description", Value: "Desc"},
				{Key: "user_id", Value: "user1"},
				{Key: "tenant", Value: "tenant1"},
				{Key: "is_active", Value: true},
				{Key: "created_at", Value: time.Now()},
				{Key: "updated_at", Value: time.Now()},
			},
		)
		
		// Mock UpdateOne for last used timestamp
		updateResponse := mtest.CreateSuccessResponse(
			bson.E{Key: "n", Value: 1},
			bson.E{Key: "nModified", Value: 1},
			bson.E{Key: "ok", Value: 1},
		)
		
		mt.AddMockResponses(findResponse, updateResponse)
		
		// Original secret should validate
		validated, err := store.ValidateClientCredentials(cred.ClientID, generatedSecret)
		require.NoError(t, err)
		assert.Equal(t, cred.ClientID, validated.ClientID)
		
		// Mock FindOne for failed validation (wrong secret)
		mt.AddMockResponses(findResponse)
		
		// Wrong secret should not validate
		_, err = store.ValidateClientCredentials(cred.ClientID, "wrong-secret")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid client credentials")
	})
}
