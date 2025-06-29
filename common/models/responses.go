/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package models

// SuccessResponse represents a successful API response
type SuccessResponse struct {
	Message string `json:"message" example:"Operation completed successfully"`
}

// ErrorResponse represents an error API response
type ErrorResponse struct {
	Error string `json:"error" example:"Something went wrong"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Message string `json:"message" example:"ok"`
}

// SecretKeyResponse represents a secret key response (without exposing the actual key)
type SecretKeyResponse struct {
	ID                    string `json:"id" example:"507f1f77bcf86cd799439011"`
	Origin                string `json:"origin" example:"web.example.com"`
	IsDefault             bool   `json:"is_default" example:"false"`
	CreatedAt             string `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt             string `json:"updated_at" example:"2023-01-01T00:00:00Z"`
	UserIdentifierClaim   string `json:"user_identifier_claim" example:"username"`
	TenantIdentifierClaim string `json:"tenant_identifier_claim" example:"tenant"`
}

// SecretKeyRequest represents the request to create/update a secret key
type SecretKeyRequest struct {
	Origin                string `json:"origin" example:"web.example.com"`
	Key                   string `json:"key" example:"your_secret_key_here"`
	IsDefault             bool   `json:"is_default" example:"false"`
	Justification         string `json:"justification" example:"Adding new key for web client"`
	UserIdentifierClaim   string `json:"user_identifier_claim,omitempty" example:"username"`
	TenantIdentifierClaim string `json:"tenant_identifier_claim,omitempty" example:"tenant"`
}

// JustificationRequest represents a request that requires justification
type JustificationRequest struct {
	Justification string `json:"justification" example:"Reason for this action"`
}

// WorkspacesResponse represents the response for workspace listing endpoints
type WorkspacesResponse struct {
	Workspaces []Workspace `json:"workspaces"`
	Total      int64       `json:"total" example:"25"`
}

// WorkspacesWithDeviceCountResponse represents the response for workspace listing endpoints with device count
type WorkspacesWithDeviceCountResponse struct {
	Workspaces []WorkspaceWithDeviceCount `json:"workspaces"`
	Total      int64                      `json:"total" example:"25"`
}

// AuthResponse represents the response for authentication endpoints
type AuthResponse struct {
	AccessToken string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	TokenType   string `json:"token_type" example:"Bearer"`
	ExpiresIn   int    `json:"expires_in" example:"3600"`
	Username    string `json:"username" example:"john_doe"`
	Role        string `json:"role" example:"user"`
}

// UserInfoResponse represents the response for user info endpoints
type UserInfoResponse struct {
	Username            string   `json:"username" example:"john_doe"`
	Role                string   `json:"role" example:"user"`
	Tenant              string   `json:"tenant,omitempty" example:"acme-corp"`
	Scopes              []string `json:"scopes" example:"user,admin"`
	UserIdentifierClaim string   `json:"user_identifier_claim,omitempty" example:"username"`
}

// SecretKeyHistoryResponse represents paginated secret key audit history
type SecretKeyHistoryResponse struct {
	Logs       []interface{} `json:"logs"`
	Total      int64         `json:"total" example:"50"`
	Page       int           `json:"page" example:"1"`
	Limit      int           `json:"limit" example:"10"`
	TotalPages int           `json:"total_pages" example:"5"`
}

// SecretKeyAuditLogResponse represents a single audit log entry
type SecretKeyAuditLogResponse struct {
	ID            string `json:"id" example:"507f1f77bcf86cd799439011"`
	Username      string `json:"user" example:"admin"`
	SecretKeyID   string `json:"secret_key_id" example:"507f1f77bcf86cd799439012"`
	Origin        string `json:"origin" example:"web.example.com"`
	Action        string `json:"action" example:"create"`
	Timestamp     string `json:"timestamp" example:"2023-01-01T00:00:00Z"`
	IsDefault     bool   `json:"is_default" example:"false"`
	Justification string `json:"justification,omitempty" example:"Adding new secret key for web client"`
}

// LogEntry represents a log entry structure
type LogEntry struct {
	Timestamp string `json:"timestamp" example:"2023-01-01T00:00:00.123Z"`
	Level     string `json:"level" example:"INFO"`
	Message   string `json:"message" example:"Device connected successfully"`
	Source    string `json:"source,omitempty" example:"appium"`
}

// FileEntry represents a file entry structure
type FileEntry struct {
	Name       string `json:"name" example:"test-app.apk"`
	UploadDate string `json:"upload_date" example:"2023-01-01T00:00:00Z"`
	Size       int64  `json:"size,omitempty" example:"1048576"`
}

// Client Credentials Request structures
type CreateCredentialRequest struct {
	Name        string `json:"name" binding:"required" example:"My API Client"`
	Description string `json:"description" example:"Client credentials for my application"`
}

type UpdateCredentialRequest struct {
	Name        string `json:"name" example:"Updated API Client"`
	Description string `json:"description" example:"Updated description for my application"`
}

type OAuth2TokenRequest struct {
	ClientID     string `json:"client_id" binding:"required" example:"cc_1234567890abcdef"`
	ClientSecret string `json:"client_secret" binding:"required" example:"cs_abcdef1234567890"`
	Tenant       string `json:"tenant,omitempty" example:"acme-corp"`
}

// Client Credentials Response structures
type CredentialResponse struct {
	ClientID    string `json:"client_id" example:"cc_1234567890abcdef"`
	Name        string `json:"name" example:"My API Client"`
	Description string `json:"description" example:"Client credentials for my application"`
	IsActive    bool   `json:"is_active" example:"true"`
	CreatedAt   string `json:"created_at" example:"2023-01-01T00:00:00Z"`
	UpdatedAt   string `json:"updated_at" example:"2023-01-01T00:00:00Z"`
}

type CreateCredentialResponse struct {
	ClientID         string `json:"client_id" example:"cc_1234567890abcdef"`
	ClientSecret     string `json:"client_secret" example:"cs_abcdef1234567890"`
	Tenant           string `json:"tenant" example:"acme-corp"`
	Name             string `json:"name" example:"My API Client"`
	Description      string `json:"description" example:"Client credentials for my application"`
	IsActive         bool   `json:"is_active" example:"true"`
	CreatedAt        string `json:"created_at" example:"2023-01-01T00:00:00Z"`
	CapabilityPrefix string `json:"capability_prefix" example:"gads"`
}

type ClientCredentialsListResponse struct {
	Credentials []CredentialResponse `json:"credentials"`
	Total       int64                `json:"total" example:"5"`
}
