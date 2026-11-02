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
	"GADS/common/api"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/hub/auth"

	"github.com/gin-gonic/gin"
)

// GetOIDCConfig godoc
// @Summary      Get OIDC configuration
// @Description  Retrieve the configuration of sign-in through an OpenID Connect provider. The client secret is not returned
// @Tags         Hub - Admin - OIDC
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.OIDCConfigResponse
// @Failure      500  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/oidc-config [get]
func GetOIDCConfig(c *gin.Context) {
	config, err := db.GlobalMongoStore.GetOIDCConfig()
	if err != nil {
		api.InternalError(c, "Failed to retrieve OIDC configuration")
		return
	}

	api.OK(c, "OIDC configuration retrieved successfully", models.OIDCConfigView{
		Enabled:         config.Enabled,
		IssuerURL:       config.IssuerURL,
		ClientID:        config.ClientID,
		ClientSecretSet: config.ClientSecret != "",
		RedirectURI:     config.RedirectURI,
		AdminGroup:      config.AdminGroup,
		GroupsClaim:     config.GroupsClaim,
	})
}

// UpdateOIDCConfig godoc
// @Summary      Update OIDC configuration
// @Description  Update the configuration of sign-in through an OpenID Connect provider. It applies at once, an empty client secret keeps the stored one
// @Tags         Hub - Admin - OIDC
// @Accept       json
// @Produce      json
// @Param        config  body      models.OIDCConfig  true  "OIDC configuration"
// @Success      200     {object}  models.SuccessResponse
// @Failure      400     {object}  models.ErrorResponse
// @Failure      500     {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/oidc-config [post]
func UpdateOIDCConfig(c *gin.Context) {
	var config models.OIDCConfig

	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, "Invalid input")
		return
	}

	if config.ClientSecret == "" {
		stored, err := db.GlobalMongoStore.GetOIDCConfig()
		if err != nil {
			api.InternalError(c, "Failed to retrieve OIDC configuration")
			return
		}
		config.ClientSecret = stored.ClientSecret
	}
	if config.GroupsClaim == "" {
		config.GroupsClaim = "groups"
	}

	if err := config.Validate(); err != nil {
		api.BadRequest(c, "Invalid OIDC configuration - "+err.Error())
		return
	}

	err := db.GlobalMongoStore.UpdateOIDCConfig(config)
	if err != nil {
		api.InternalError(c, "Failed to save OIDC configuration")
		return
	}

	auth.ConfigureOIDC(config)

	api.OKMessage(c, "OIDC configuration updated successfully")
}
