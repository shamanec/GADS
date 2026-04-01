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
// @Tags Hub - OAuth2
// @Accept application/x-www-form-urlencoded
// @Accept json
// @Produce json
// @Param client_id formData string true "Client ID"
// @Param client_secret formData string true "Client Secret"
// @Success 200 {object} models.AuthResponse
// @Failure 400 {object} models.OAuthErrorResponse
// @Failure 401 {object} models.OAuthErrorResponse
// @Router /oauth/token [post]
func OAuth2TokenEndpoint(c *gin.Context) {
	var req models.OAuth2TokenRequest

	contentType := c.GetHeader("Content-Type")
	if contentType == "application/x-www-form-urlencoded" {
		clientID := c.PostForm("client_id")
		clientSecret := c.PostForm("client_secret")
		if clientID == "" || clientSecret == "" {
			c.JSON(http.StatusBadRequest, models.OAuthErrorResponse{Error: "invalid_request"})
			return
		}

		req = models.OAuth2TokenRequest{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, models.OAuthErrorResponse{Error: "invalid_request"})
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
	)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.OAuthErrorResponse{Error: "invalid_client"})
		return
	}

	if !credential.IsActive {
		c.JSON(http.StatusUnauthorized, models.OAuthErrorResponse{Error: "invalid_client"})
		return
	}

	user, err := store.GetUser(credential.UserID)
	userRole := "user"
	if err == nil {
		userRole = user.Role
	}

	token, err := generateAccessToken(credential, origin, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.OAuthErrorResponse{Error: "server_error"})
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
		[]string{userRole},
		time.Hour,
		origin,
	)
	if err != nil {
		return "", err
	}
	return token, nil
}
