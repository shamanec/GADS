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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupTestMongo(t *testing.T) *MongoStore {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	require.NoError(t, err)

	store := &MongoStore{
		Client:       client,
		DatabaseName: "gads_test",
		Ctx:          ctx,
		CtxCancel:    cancel,
	}

	// Clean up test data
	coll := store.GetCollection("client_credentials")
	coll.DeleteMany(ctx, bson.M{})

	return store
}

func TestCreateClientCredential(t *testing.T) {
	store := setupTestMongo(t)

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
}

func TestGetClientCredential(t *testing.T) {
	store := setupTestMongo(t)

	// Create a test credential
	created, err := store.CreateClientCredential(
		"Test Client",
		"Test Description",
		"user123",
		"test-tenant",
	)
	require.NoError(t, err)

	// Retrieve it
	retrieved, err := store.GetClientCredential(created.ClientID)
	require.NoError(t, err)

	assert.Equal(t, created.ClientID, retrieved.ClientID)
	assert.Equal(t, created.Name, retrieved.Name)
	assert.Equal(t, created.UserID, retrieved.UserID)
	assert.Equal(t, created.Tenant, retrieved.Tenant)
	// Note: ClientSecret in retrieved will be the hash, not the original
}

func TestValidateClientCredentials(t *testing.T) {
	store := setupTestMongo(t)

	// Create a test credential
	created, err := store.CreateClientCredential(
		"Test Client",
		"Test Description",
		"user123",
		"test-tenant",
	)
	require.NoError(t, err)

	// Test valid credentials
	validated, err := store.ValidateClientCredentials(created.ClientID, created.ClientSecret)
	require.NoError(t, err)
	assert.Equal(t, created.ClientID, validated.ClientID)

	// Test invalid secret
	_, err = store.ValidateClientCredentials(created.ClientID, "wrong-secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client credentials")

	// Test invalid client ID
	_, err = store.ValidateClientCredentials("wrong-id", created.ClientSecret)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client credentials")
}

func TestDeactivateClientCredential(t *testing.T) {
	store := setupTestMongo(t)

	// Create a test credential
	created, err := store.CreateClientCredential(
		"Test Client",
		"Test Description",
		"user123",
		"test-tenant",
	)
	require.NoError(t, err)

	// Deactivate it
	err = store.DeactivateClientCredential(created.ClientID)
	require.NoError(t, err)

	// Try to validate - should fail
	_, err = store.ValidateClientCredentials(created.ClientID, created.ClientSecret)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inactive")
}

func TestGetClientCredentialsByUser(t *testing.T) {
	store := setupTestMongo(t)

	userID := "user123"

	// Create multiple credentials for the same user
	_, err := store.CreateClientCredential("Client 1", "Desc 1", userID, "tenant1")
	require.NoError(t, err)

	_, err = store.CreateClientCredential("Client 2", "Desc 2", userID, "tenant1")
	require.NoError(t, err)

	// Create credential for different user
	_, err = store.CreateClientCredential("Client 3", "Desc 3", "user456", "tenant1")
	require.NoError(t, err)

	// Get credentials for user123
	credentials, err := store.GetClientCredentialsByUser(userID)
	require.NoError(t, err)
	assert.Len(t, credentials, 2)

	for _, cred := range credentials {
		assert.Equal(t, userID, cred.UserID)
		assert.True(t, cred.IsActive)
	}
}

func TestGenerateClientID(t *testing.T) {
	id1, err := generateClientID()
	require.NoError(t, err)
	assert.Contains(t, id1, "gads_")

	id2, err := generateClientID()
	require.NoError(t, err)
	assert.Contains(t, id2, "gads_")

	// Should be unique
	assert.NotEqual(t, id1, id2)
}

func TestHashAndVerifyClientSecret(t *testing.T) {
	secret := "test-secret-123"
	hash := hashClientSecret(secret)

	assert.NotEqual(t, secret, hash)
	assert.True(t, verifyClientSecret(secret, hash))
	assert.False(t, verifyClientSecret("wrong-secret", hash))
}
