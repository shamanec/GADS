/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package api

import "github.com/gin-gonic/gin"

type APIResponse struct {
	Message string      `json:"message,omitempty"`
	Result  interface{} `json:"result,omitempty"`
}

func GenericResponse(c *gin.Context, statusCode int, message string, result interface{}) {
	c.JSON(statusCode, APIResponse{
		Message: message,
		Result:  result,
	})
}
