package router

import (
	"GADS/common/api"
	"GADS/common/db"

	"github.com/gin-gonic/gin"
)

func GetBuildReports(c *gin.Context) {
	// tenantInterface, exists := c.Get("tenant")
	// if !exists {
	// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant not found in token"})
	// 	return
	// }

	// tenant, ok := tenantInterface.(string)
	// if !ok {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid tenant format"})
	// 	return
	// }

	buildReports, err := db.GlobalMongoStore.GetBuildReports("dge8WM7G7DTcbAjAwvtoHUNxRllTfa_xsFUl8f7778c=", 50)
	if err != nil {
		api.GenericResponse(c, 500, err.Error(), nil)
		return
	}

	api.GenericResponse(c, 200, "Got reports", buildReports)
}
