package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func successResponse(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{"message": message})
}

func errorResponse(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{"error": message})
}

func OK(c *gin.Context, message string) {
	successResponse(c, http.StatusOK, message)
}

func BadRequest(c *gin.Context, message string) {
	errorResponse(c, http.StatusBadRequest, message)
}

func InternalServerError(c *gin.Context, message string) {
	errorResponse(c, http.StatusInternalServerError, message)
}
