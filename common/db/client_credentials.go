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
	"GADS/common/models"
	"GADS/hub/auth/clientcredentials"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetClientCredential retrieves a client credential by client_id
func (m *MongoStore) GetClientCredential(clientID string) (models.ClientCredentials, error) {
	coll := m.GetCollection("client_credentials")
	filter := bson.D{{Key: "client_id", Value: clientID}}
	return GetDocument[models.ClientCredentials](m.Ctx, coll, filter)
}

// GetClientCredentialsByUser retrieves all client credentials for a specific user
func (m *MongoStore) GetClientCredentialsByUser(userID string) ([]models.ClientCredentials, error) {
	coll := m.GetCollection("client_credentials")
	filter := bson.D{{Key: "user_id", Value: userID}, {Key: "is_active", Value: true}}
	return GetDocuments[models.ClientCredentials](m.Ctx, coll, filter)
}

// GetClientCredentialsByTenant retrieves all client credentials for a specific tenant
func (m *MongoStore) GetClientCredentialsByTenant(tenant string) ([]models.ClientCredentials, error) {
	coll := m.GetCollection("client_credentials")
	filter := bson.D{{Key: "tenant", Value: tenant}, {Key: "is_active", Value: true}}
	return GetDocuments[models.ClientCredentials](m.Ctx, coll, filter)
}

// CreateClientCredential creates a new client credential with generated client_id and secret
func (m *MongoStore) CreateClientCredential(name, description, userID, tenant string) (models.ClientCredentials, error) {
	clientID := clientcredentials.GenerateClientIDWithPrefix(getClientIDPrefix())

	clientSecret, err := clientcredentials.GenerateClientSecret()
	if err != nil {
		return models.ClientCredentials{}, fmt.Errorf("failed to generate client secret: %w", err)
	}

	hashedSecret, err := clientcredentials.HashSecret(clientSecret)
	if err != nil {
		return models.ClientCredentials{}, fmt.Errorf("failed to hash client secret: %w", err)
	}

	credential := models.ClientCredentials{
		ClientID:     clientID,
		ClientSecret: hashedSecret, // Store hashed version
		Name:         name,
		Description:  description,
		UserID:       userID,
		Tenant:       tenant,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	coll := m.GetCollection("client_credentials")
	result, err := InsertDocumentWithResult(m.Ctx, coll, credential)
	if err != nil {
		return models.ClientCredentials{}, fmt.Errorf("failed to create client credential: %w", err)
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		credential.ID = oid.Hex()
	}

	// Return credential with unhashed secret for client to save
	credential.ClientSecret = clientSecret
	return credential, nil
}

// UpdateClientCredential updates an existing client credential
func (m *MongoStore) UpdateClientCredential(clientID string, updates map[string]interface{}) error {
	coll := m.GetCollection("client_credentials")
	filter := bson.D{{Key: "client_id", Value: clientID}}

	// Add updated timestamp
	updates["updated_at"] = time.Now()

	return PartialDocumentUpdate(m.Ctx, coll, filter, updates)
}

// UpdateClientCredentialLastUsed updates the last_used_at timestamp
func (m *MongoStore) UpdateClientCredentialLastUsed(clientID string) error {
	coll := m.GetCollection("client_credentials")
	filter := bson.D{{Key: "client_id", Value: clientID}}
	updates := bson.M{
		"last_used_at": time.Now(),
		"updated_at":   time.Now(),
	}
	return PartialDocumentUpdate(m.Ctx, coll, filter, updates)
}

// DeactivateClientCredential marks a client credential as inactive
func (m *MongoStore) DeactivateClientCredential(clientID string) error {
	return m.UpdateClientCredential(clientID, map[string]interface{}{
		"is_active": false,
	})
}

// DeleteClientCredential permanently removes a client credential
func (m *MongoStore) DeleteClientCredential(clientID string) error {
	coll := m.GetCollection("client_credentials")
	filter := bson.M{"client_id": clientID}
	return DeleteDocument(m.Ctx, coll, filter)
}

// ValidateClientCredentials validates client_id and secret for authentication
func (m *MongoStore) ValidateClientCredentials(clientID, clientSecret string) (models.ClientCredentials, error) {
	credential, err := m.GetClientCredential(clientID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return models.ClientCredentials{}, fmt.Errorf("invalid client credentials")
		}
		return models.ClientCredentials{}, err
	}

	if !credential.IsActive {
		return models.ClientCredentials{}, fmt.Errorf("client credential is inactive")
	}

	// Verify secret hash
	if !clientcredentials.ValidateSecret(clientSecret, credential.ClientSecret) {
		return models.ClientCredentials{}, fmt.Errorf("invalid client credentials")
	}

	// Update last used timestamp
	go m.UpdateClientCredentialLastUsed(clientID)

	return credential, nil
}

// CreateClientCredentialIndexes creates database indexes for performance
func (m *MongoStore) CreateClientCredentialIndexes() error {
	coll := m.GetCollection("client_credentials")

	// Unique index on client_id
	clientIDIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "client_id", Value: 1}},
		Options: &options.IndexOptions{
			Unique: &[]bool{true}[0],
		},
	}

	// Compound index on user_id and is_active for fast user queries
	userActiveIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "is_active", Value: 1},
		},
	}

	// Compound index on tenant and is_active for tenant queries
	tenantActiveIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "tenant", Value: 1},
			{Key: "is_active", Value: 1},
		},
	}

	indexes := []mongo.IndexModel{clientIDIndex, userActiveIndex, tenantActiveIndex}

	_, err := coll.Indexes().CreateMany(m.Ctx, indexes)
	return err
}

// Helper functions

func getClientIDPrefix() string {
	prefix := os.Getenv("GADS_CLIENT_ID_PREFIX")
	if prefix == "" {
		return "gads"
	}
	return prefix
}
