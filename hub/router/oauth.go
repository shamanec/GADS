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
	"GADS/hub/auth"
	"GADS/hub/auth/clientcredentials"
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// OAuth2TokenEndpoint godoc
// @Summary OAuth2 Client Credentials Token Endpoint
// @Description Generate an access token using OAuth2 client credentials flow
// @Tags OAuth2
// @Accept application/x-www-form-urlencoded
// @Accept json
// @Produce json
// @Param client_id formData string true "Client ID"
// @Param client_secret formData string true "Client Secret"
// @Param tenant formData string true "Tenant"
// @Success 200 {object} models.AuthResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /oauth/token [post]
func OAuth2TokenEndpoint(c *gin.Context) {
	var req models.OAuth2TokenRequest

	contentType := c.GetHeader("Content-Type")
	if contentType == "application/x-www-form-urlencoded" {
		clientID := c.PostForm("client_id")
		clientSecret := c.PostForm("client_secret")
		tenant := c.PostForm("tenant")
		if clientID == "" || clientSecret == "" {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid_request"})
			return
		}

		req = models.OAuth2TokenRequest{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Tenant:       tenant,
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid_request"})
			return
		}
	}

	origin := auth.GetOriginFromRequest(c)
	store := db.GlobalMongoStore

	credential, err := clientcredentials.ValidateCredentials(
		context.Background(),
		store,
		req.ClientID,
		req.ClientSecret,
		req.Tenant,
	)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid_client"})
		return
	}

	if !credential.IsActive {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid_client"})
		return
	}

	user, err := store.GetUser(credential.UserID)
	userRole := "user"
	if err == nil {
		userRole = user.Role
	}

	token, err := generateAccessToken(credential, origin, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "server_error"})
		return
	}

	response := models.AuthResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		Username:    credential.UserID,
		Role:        userRole,
	}

	c.JSON(http.StatusOK, response)
}

func generateAccessToken(credential *models.ClientCredentials, origin string, userRole string) (string, error) {
	token, err := auth.GenerateJWT(
		credential.UserID,
		userRole,
		credential.Tenant,
		[]string{userRole},
		time.Hour,
		origin,
	)
	if err != nil {
		return "", err
	}
	return token, nil
}
