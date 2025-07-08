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
	"GADS/common/models"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCreateClientCredential_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/client-credentials", CreateClientCredential)

	req := models.CreateCredentialRequest{
		Name:        "Test Client",
		Description: "Test description",
	}
	jsonData, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/client-credentials", bytes.NewBuffer(jsonData))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "unauthorized", response.Error)
}

func TestListClientCredentials_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/client-credentials", ListClientCredentials)

	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/client-credentials", nil)

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "unauthorized", response.Error)
}

func TestOAuth2TokenEndpoint_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/oauth/token", OAuth2TokenEndpoint)

	// Teste com JSON inválido
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/oauth/token", bytes.NewBuffer([]byte("invalid json")))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error)
}

func TestOAuth2TokenEndpoint_MissingParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/oauth/token", OAuth2TokenEndpoint)

	req := models.OAuth2TokenRequest{
		ClientID: "test_client",
		// ClientSecret is missing intentionally
	}
	jsonData, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/oauth/token", bytes.NewBuffer(jsonData))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error)
}
