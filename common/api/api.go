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
