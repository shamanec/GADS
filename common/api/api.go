/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package api

import (
	"GADS/common/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GenericResponse(c *gin.Context, statusCode int, message string, result interface{}) {
	c.JSON(statusCode, models.APIResponse{
		Message: message,
		Result:  result,
	})
}

func InternalServerErrorResponse(c *gin.Context, message string, result interface{}) {
	GenericResponse(c, http.StatusInternalServerError, message, result)
}

func OKResponse(c *gin.Context, message string, result interface{}) {
	GenericResponse(c, http.StatusOK, message, result)
}

func NotFoundResponse(c *gin.Context, message string, result interface{}) {
	GenericResponse(c, http.StatusNotFound, message, result)
}

func BadRequestResponse(c *gin.Context, message string, result interface{}) {
	GenericResponse(c, http.StatusBadRequest, message, result)
}

func ForbiddenResponse(c *gin.Context, message string, result interface{}) {
	GenericResponse(c, http.StatusForbidden, message, result)
}
