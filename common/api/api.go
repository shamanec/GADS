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

// OK sends a 200 response with result payload.
func OK[T any](c *gin.Context, message string, result T) {
	c.JSON(http.StatusOK, models.APIResponse[T]{Success: true, Message: message, Result: result})
}

// OKMessage sends a 200 response with a message only (no result payload).
func OKMessage(c *gin.Context, message string) {
	c.JSON(http.StatusOK, models.APIResponse[any]{Success: true, Message: message})
}

// Created sends a 201 response with result payload.
func Created[T any](c *gin.Context, message string, result T) {
	c.JSON(http.StatusCreated, models.APIResponse[T]{Success: true, Message: message, Result: result})
}

// ErrorResponse sends an error response with the given status code.
func ErrorResponse(c *gin.Context, status int, message string) {
	c.JSON(status, models.APIResponse[any]{Success: false, Message: message})
}

// BadRequest sends a 400 error response.
func BadRequest(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusBadRequest, message)
}

// Unauthorized sends a 401 error response.
func Unauthorized(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusUnauthorized, message)
}

// Forbidden sends a 403 error response.
func Forbidden(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusForbidden, message)
}

// NotFound sends a 404 error response.
func NotFound(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusNotFound, message)
}

// Conflict sends a 409 error response.
func Conflict(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusConflict, message)
}

// InternalError sends a 500 error response.
func InternalError(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusInternalServerError, message)
}

