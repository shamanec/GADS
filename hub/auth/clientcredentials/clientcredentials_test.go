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
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// MockCredentialStore implements a simple mock for testing
type MockCredentialStore struct {
	credentials map[string]models.ClientCredentials
	shouldError bool
}

func NewMockCredentialStore() *MockCredentialStore {
	return &MockCredentialStore{
		credentials: make(map[string]models.ClientCredentials),
		shouldError: false,
	}
}

func (m *MockCredentialStore) CreateClientCredential(name, description, userID, tenant string) (models.ClientCredentials, error) {
	if m.shouldError {
		return models.ClientCredentials{}, errors.New("mock error")
	}

	credential := models.ClientCredentials{
		ClientID:     "test_client_id",
		ClientSecret: "test_secret",
		Name:         name,
		Description:  description,
		UserID:       userID,
		Tenant:       tenant,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.credentials[credential.ClientID] = credential
	return credential, nil
}

func (m *MockCredentialStore) GetClientCredential(clientID string) (models.ClientCredentials, error) {
	if m.shouldError {
		return models.ClientCredentials{}, errors.New("mock error")
	}

	credential, exists := m.credentials[clientID]
	if !exists {
		return models.ClientCredentials{}, errors.New("credential not found")
	}

	return credential, nil
}

func (m *MockCredentialStore) GetClientCredentialsByUser(userID string) ([]models.ClientCredentials, error) {
	if m.shouldError {
		return nil, errors.New("mock error")
	}

	var result []models.ClientCredentials
	for _, cred := range m.credentials {
		if cred.UserID == userID && cred.IsActive {
			result = append(result, cred)
		}
	}

	return result, nil
}

func (m *MockCredentialStore) GetClientCredentialsByTenant(tenant string) ([]models.ClientCredentials, error) {
	if m.shouldError {
		return nil, errors.New("mock error")
	}

	var result []models.ClientCredentials
	for _, cred := range m.credentials {
		if cred.Tenant == tenant && cred.IsActive {
			result = append(result, cred)
		}
	}

	return result, nil
}

func (m *MockCredentialStore) UpdateClientCredential(clientID string, updates bson.M) error {
	if m.shouldError {
		return errors.New("mock error")
	}

	credential, exists := m.credentials[clientID]
	if !exists {
		return errors.New("credential not found")
	}

	if name, ok := updates["name"].(string); ok {
		credential.Name = name
	}
	if description, ok := updates["description"].(string); ok {
		credential.Description = description
	}

	m.credentials[clientID] = credential
	return nil
}

func (m *MockCredentialStore) DeactivateClientCredential(clientID string) error {
	if m.shouldError {
		return errors.New("mock error")
	}

	credential, exists := m.credentials[clientID]
	if !exists {
		return errors.New("credential not found")
	}

	credential.IsActive = false
	m.credentials[clientID] = credential
	return nil
}

func (m *MockCredentialStore) ValidateClientCredentials(clientID, clientSecret string) (models.ClientCredentials, error) {
	if m.shouldError {
		return models.ClientCredentials{}, errors.New("mock error")
	}

	credential, exists := m.credentials[clientID]
	if !exists || credential.ClientSecret != clientSecret || !credential.IsActive {
		return models.ClientCredentials{}, errors.New("invalid credentials")
	}

	if credential.ClientSecret != clientSecret {
		return models.ClientCredentials{}, errors.New("invalid credentials")
	}

	return credential, nil
}

// Tests

func TestCreateCredential(t *testing.T) {
	ctx := context.Background()
	mock := NewMockCredentialStore()

	// Test successful creation
	credential, err := CreateCredential(ctx, mock, "test_name", "test_description", "user123", "tenant1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if credential.Name != "test_name" {
		t.Errorf("Expected name 'test_name', got %s", credential.Name)
	}

	// Test empty name
	_, err = CreateCredential(ctx, mock, "", "test_description", "user123", "tenant1")
	if err == nil {
		t.Error("Expected error for empty name")
	}

	// Test empty userID
	_, err = CreateCredential(ctx, mock, "test_name", "test_description", "", "tenant1")
	if err == nil {
		t.Error("Expected error for empty userID")
	}
}

func TestGetCredential(t *testing.T) {
	ctx := context.Background()
	mock := NewMockCredentialStore()

	// Setup test credential
	testCred := models.ClientCredentials{
		ClientID: "test_id",
		UserID:   "user123",
		Tenant:   "tenant1",
		Name:     "test_name",
	}
	mock.credentials["test_id"] = testCred

	// Test successful get
	credential, err := GetCredential(ctx, mock, "test_id", "user123", "tenant1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if credential.ClientID != "test_id" {
		t.Errorf("Expected client ID 'test_id', got %s", credential.ClientID)
	}

	// Test wrong user
	_, err = GetCredential(ctx, mock, "test_id", "user999", "tenant1")
	if err == nil {
		t.Error("Expected error for wrong user")
	}

	// Test wrong tenant
	_, err = GetCredential(ctx, mock, "test_id", "user123", "tenant2")
	if err == nil {
		t.Error("Expected error for wrong tenant")
	}

	// Test empty userID
	_, err = GetCredential(ctx, mock, "test_id", "", "tenant1")
	if err == nil {
		t.Error("Expected error for empty userID")
	}
}

func TestListCredentials(t *testing.T) {
	ctx := context.Background()
	mock := NewMockCredentialStore()

	// Setup test credentials
	testCred1 := models.ClientCredentials{
		ClientID: "test_id_1",
		UserID:   "user123",
		Tenant:   "tenant1",
		IsActive: true,
	}
	testCred2 := models.ClientCredentials{
		ClientID: "test_id_2",
		UserID:   "user123",
		Tenant:   "tenant1",
		IsActive: true,
	}
	testCred3 := models.ClientCredentials{
		ClientID: "test_id_3",
		UserID:   "user123",
		Tenant:   "tenant2",
		IsActive: true,
	}
	testCred4 := models.ClientCredentials{
		ClientID: "test_id_4",
		UserID:   "user123",
		Tenant:   "tenant1",
		IsActive: false, // Inactive credential
	}

	mock.credentials["test_id_1"] = testCred1
	mock.credentials["test_id_2"] = testCred2
	mock.credentials["test_id_3"] = testCred3
	mock.credentials["test_id_4"] = testCred4

	// Test successful list for tenant1 - should return only 2 active credentials
	credentials, err := ListCredentials(ctx, mock, "user123", "tenant1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(credentials) != 2 {
		t.Errorf("Expected 2 credentials for tenant1, got %d", len(credentials))
	}

	// Test list for tenant2 - should return only 1 credential
	credentials, err = ListCredentials(ctx, mock, "user123", "tenant2")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(credentials) != 1 {
		t.Errorf("Expected 1 credential for tenant2, got %d", len(credentials))
	}

	// Test list for non-existent tenant - should return 0 credentials
	credentials, err = ListCredentials(ctx, mock, "user123", "tenant_nonexistent")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(credentials) != 0 {
		t.Errorf("Expected 0 credentials for non-existent tenant, got %d", len(credentials))
	}

	// Test empty userID
	_, err = ListCredentials(ctx, mock, "", "tenant1")
	if err == nil {
		t.Error("Expected error for empty userID")
	}

	// Test empty tenant
	_, err = ListCredentials(ctx, mock, "user123", "")
	if err == nil {
		t.Error("Expected error for empty tenant")
	}
}

func TestUpdateCredential(t *testing.T) {
	ctx := context.Background()
	mock := NewMockCredentialStore()

	// Setup test credential
	testCred := models.ClientCredentials{
		ClientID: "test_id",
		UserID:   "user123",
		Tenant:   "tenant1",
		Name:     "old_name",
	}
	mock.credentials["test_id"] = testCred

	// Test successful update
	err := UpdateCredential(ctx, mock, "test_id", "new_name", "new_description", "user123", "tenant1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify update
	updated := mock.credentials["test_id"]
	if updated.Name != "new_name" {
		t.Errorf("Expected name 'new_name', got %s", updated.Name)
	}
}

func TestRevokeCredential(t *testing.T) {
	ctx := context.Background()
	mock := NewMockCredentialStore()

	// Setup test credential
	testCred := models.ClientCredentials{
		ClientID: "test_id",
		UserID:   "user123",
		Tenant:   "tenant1",
		IsActive: true,
	}
	mock.credentials["test_id"] = testCred

	// Test successful revoke
	err := RevokeCredential(ctx, mock, "test_id", "user123", "tenant1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify revocation
	revoked := mock.credentials["test_id"]
	if revoked.IsActive {
		t.Error("Expected credential to be inactive after revocation")
	}
}

func TestValidateCredentials(t *testing.T) {
	ctx := context.Background()
	mock := NewMockCredentialStore()

	// Setup test credential
	testCred := models.ClientCredentials{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
		Tenant:       "tenant1",
		IsActive:     true,
	}
	mock.credentials["test_id"] = testCred

	// Test successful validation
	credential, err := ValidateCredentials(ctx, mock, "test_id", "test_secret", "tenant1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if credential.ClientID != "test_id" {
		t.Errorf("Expected client ID 'test_id', got %s", credential.ClientID)
	}

	// Test wrong secret
	_, err = ValidateCredentials(ctx, mock, "test_id", "wrong_secret", "tenant1")
	if err == nil {
		t.Error("Expected error for wrong secret")
	}

	// Test wrong tenant
	_, err = ValidateCredentials(ctx, mock, "test_id", "test_secret", "tenant2")
	if err == nil {
		t.Error("Expected error for wrong tenant")
	}

	// Test empty credentials
	_, err = ValidateCredentials(ctx, mock, "", "test_secret", "tenant1")
	if err == nil {
		t.Error("Expected error for empty client ID")
	}

	_, err = ValidateCredentials(ctx, mock, "test_id", "", "tenant1")
	if err == nil {
		t.Error("Expected error for empty client secret")
	}
}
