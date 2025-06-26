/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package clientcredentials

import (
	"GADS/common/models"
	"context"
	"errors"
	"log"
)

// CredentialStore defines the interface for client credential storage operations
type CredentialStore interface {
	CreateClientCredential(name, description, userID, tenant string) (models.ClientCredentials, error)
	GetClientCredential(clientID string) (models.ClientCredentials, error)
	GetClientCredentialsByUser(userID string) ([]models.ClientCredentials, error)
	UpdateClientCredential(clientID string, updates map[string]interface{}) error
	DeactivateClientCredential(clientID string) error
	ValidateClientCredentials(clientID, clientSecret string) (models.ClientCredentials, error)
}

func CreateCredential(ctx context.Context, store CredentialStore, name, description string, userID string, tenant string) (*models.ClientCredentials, error) {
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}
	if userID == "" {
		return nil, errors.New("user ID cannot be empty")
	}

	// Use existing DB function directly
	credential, err := store.CreateClientCredential(name, description, userID, tenant)
	if err != nil {
		log.Printf("Error creating client credential: %v", err)
		return nil, err
	}

	log.Printf("Client credential created: %s", credential.ClientID)
	return &credential, nil
}

func GetCredential(ctx context.Context, store CredentialStore, clientID string, userID string, tenant string) (*models.ClientCredentials, error) {
	if clientID == "" {
		return nil, errors.New("client ID cannot be empty")
	}
	if userID == "" {
		return nil, errors.New("user ID cannot be empty")
	}

	// Use existing DB function
	credential, err := store.GetClientCredential(clientID)
	if err != nil {
		return nil, err
	}

	// Check tenant access
	if credential.Tenant != tenant {
		return nil, errors.New("access denied: wrong tenant")
	}

	// Check if user owns credential
	if credential.UserID != userID {
		return nil, errors.New("access denied: not owner")
	}

	return &credential, nil
}

// ListCredentials retrieves all active client credentials for a specific user and tenant
func ListCredentials(ctx context.Context, store CredentialStore, userID string, tenant string) ([]models.ClientCredentials, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}
	if tenant == "" {
		return nil, errors.New("tenant cannot be empty")
	}

	log.Printf("Listing credentials for user %s in tenant %s", userID, tenant)

	// Get all credentials for the user
	userCredentials, err := store.GetClientCredentialsByUser(userID)
	if err != nil {
		return nil, err
	}

	// Filter by tenant to ensure tenant isolation
	var filteredCredentials []models.ClientCredentials
	for _, cred := range userCredentials {
		if cred.Tenant == tenant && cred.IsActive {
			filteredCredentials = append(filteredCredentials, cred)
		}
	}

	return filteredCredentials, nil
}

func UpdateCredential(ctx context.Context, store CredentialStore, clientID, name, description string, userID string, tenant string) error {
	_, err := GetCredential(ctx, store, clientID, userID, tenant)
	if err != nil {
		return err
	}

	// Prepare updates
	updates := map[string]interface{}{
		"name":        name,
		"description": description,
	}

	// Use existing DB function
	err = store.UpdateClientCredential(clientID, updates)
	if err != nil {
		log.Printf("Error updating credential %s: %v", clientID, err)
		return err
	}

	log.Printf("Credential updated: %s", clientID)
	return nil
}

func RevokeCredential(ctx context.Context, store CredentialStore, clientID string, userID string, tenant string) error {
	_, err := GetCredential(ctx, store, clientID, userID, tenant)
	if err != nil {
		return err
	}

	// Use existing DB function
	err = store.DeactivateClientCredential(clientID)
	if err != nil {
		log.Printf("Error revoking credential %s: %v", clientID, err)
		return err
	}

	log.Printf("Credential revoked: %s", clientID)
	return nil
}

func ValidateCredentials(ctx context.Context, store CredentialStore, clientID, clientSecret, tenant string) (*models.ClientCredentials, error) {
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("invalid credentials")
	}

	// Use existing DB function (already includes secret validation)
	credential, err := store.ValidateClientCredentials(clientID, clientSecret)
	if err != nil {
		log.Printf("Credential validation failed for %s: %v", clientID, err)
		return nil, errors.New("invalid credentials")
	}

	// Check tenant
	if credential.Tenant != tenant {
		log.Printf("Tenant mismatch for credential %s", clientID)
		return nil, errors.New("invalid credentials")
	}

	return &credential, nil
}
