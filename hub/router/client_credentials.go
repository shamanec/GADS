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
	"GADS/common/models"
	"GADS/hub/auth/clientcredentials"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CreateClientCredential godoc
// @Summary Create a new client credential
// @Description Create a new client credential for the authenticated user
// @Tags Client Credentials
// @Accept json
// @Produce json
// @Param request body models.CreateCredentialRequest true "Create credential request"
// @Success 201 {object} models.CreateCredentialResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /client-credentials [post]
func CreateClientCredential(c *gin.Context) {
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	tenant, _ := c.Get("tenant")
	tenantStr := ""
	if tenant != nil {
		tenantStr = tenant.(string)
	}

	var req models.CreateCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request format"})
		return
	}

	store := db.GlobalMongoStore
	credential, err := clientcredentials.CreateCredential(
		context.Background(),
		store,
		req.Name,
		req.Description,
		username.(string),
		tenantStr,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create credential"})
		return
	}

	response := models.CreateCredentialResponse{
		ClientID:         credential.ClientID,
		ClientSecret:     credential.ClientSecret,
		Tenant:           credential.Tenant,
		Name:             credential.Name,
		Description:      credential.Description,
		IsActive:         credential.IsActive,
		CreatedAt:        credential.CreatedAt.Format("2006-01-02T15:04:05Z"),
		CapabilityPrefix: capabilityPrefix,
	}

	c.JSON(http.StatusCreated, response)
}

// ListClientCredentials godoc
// @Summary List user's client credentials
// @Description Get all client credentials for the authenticated user
// @Tags Client Credentials
// @Produce json
// @Success 200 {object} models.ClientCredentialsListResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /client-credentials [get]
func ListClientCredentials(c *gin.Context) {
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	tenant, _ := c.Get("tenant")
	tenantStr := ""
	if tenant != nil {
		tenantStr = tenant.(string)
	}

	store := db.GlobalMongoStore
	credentials, err := clientcredentials.ListCredentials(
		context.Background(),
		store,
		username.(string),
		tenantStr,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list credentials"})
		return
	}

	credentialResponses := []models.CredentialResponse{}
	for _, cred := range credentials {
		credentialResponses = append(credentialResponses, models.CredentialResponse{
			ClientID:    cred.ClientID,
			Name:        cred.Name,
			Description: cred.Description,
			IsActive:    cred.IsActive,
			CreatedAt:   cred.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:   cred.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	response := models.ClientCredentialsListResponse{
		Credentials: credentialResponses,
		Total:       int64(len(credentialResponses)),
	}

	c.JSON(http.StatusOK, response)
}

// GetClientCredential godoc
// @Summary Get a specific client credential
// @Description Get a client credential by ID for the authenticated user
// @Tags Client Credentials
// @Produce json
// @Param id path string true "Client ID"
// @Success 200 {object} models.CredentialResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /client-credentials/{id} [get]
func GetClientCredential(c *gin.Context) {
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	tenant, _ := c.Get("tenant")
	tenantStr := ""
	if tenant != nil {
		tenantStr = tenant.(string)
	}

	clientID := c.Param("id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "client ID is required"})
		return
	}

	store := db.GlobalMongoStore
	credential, err := clientcredentials.GetCredential(
		context.Background(),
		store,
		clientID,
		username.(string),
		tenantStr,
	)
	if err != nil {
		if err.Error() == "access denied: not owner" || err.Error() == "access denied: wrong tenant" {
			c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "access denied"})
			return
		}
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "credential not found"})
		return
	}

	response := models.CredentialResponse{
		ClientID:    credential.ClientID,
		Name:        credential.Name,
		Description: credential.Description,
		IsActive:    credential.IsActive,
		CreatedAt:   credential.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   credential.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	c.JSON(http.StatusOK, response)
}

// UpdateClientCredential godoc
// @Summary Update a client credential
// @Description Update metadata for a client credential (name and description only)
// @Tags Client Credentials
// @Accept json
// @Produce json
// @Param id path string true "Client ID"
// @Param request body models.UpdateCredentialRequest true "Update credential request"
// @Success 200 {object} models.CredentialResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /client-credentials/{id} [put]
func UpdateClientCredential(c *gin.Context) {
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	tenant, _ := c.Get("tenant")
	tenantStr := ""
	if tenant != nil {
		tenantStr = tenant.(string)
	}

	clientID := c.Param("id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "client ID is required"})
		return
	}

	var req models.UpdateCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request format"})
		return
	}

	store := db.GlobalMongoStore
	err := clientcredentials.UpdateCredential(
		context.Background(),
		store,
		clientID,
		req.Name,
		req.Description,
		username.(string),
		tenantStr,
	)
	if err != nil {
		if err.Error() == "access denied: not owner" || err.Error() == "access denied: wrong tenant" {
			c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "access denied"})
			return
		}
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "credential not found"})
		return
	}

	credential, err := clientcredentials.GetCredential(
		context.Background(),
		store,
		clientID,
		username.(string),
		tenantStr,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to retrieve updated credential"})
		return
	}

	response := models.CredentialResponse{
		ClientID:    credential.ClientID,
		Name:        credential.Name,
		Description: credential.Description,
		IsActive:    credential.IsActive,
		CreatedAt:   credential.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   credential.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	c.JSON(http.StatusOK, response)
}

// RevokeClientCredential godoc
// @Summary Revoke a client credential
// @Description Revoke/deactivate a client credential
// @Tags Client Credentials
// @Produce json
// @Param id path string true "Client ID"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /client-credentials/{id} [delete]
func RevokeClientCredential(c *gin.Context) {
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	tenant, _ := c.Get("tenant")
	tenantStr := ""
	if tenant != nil {
		tenantStr = tenant.(string)
	}

	clientID := c.Param("id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "client ID is required"})
		return
	}

	store := db.GlobalMongoStore
	err := clientcredentials.RevokeCredential(
		context.Background(),
		store,
		clientID,
		username.(string),
		tenantStr,
	)
	if err != nil {
		if err.Error() == "access denied: not owner" || err.Error() == "access denied: wrong tenant" {
			c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "access denied"})
			return
		}
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "credential not found"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "credential revoked successfully"})
}
