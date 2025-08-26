package router

import (
	"GADS/common/api"
	"GADS/common/db"

	"github.com/gin-gonic/gin"
)

func GetBuildReports(c *gin.Context) {
	// tenantInterface, exists := c.Get("tenant")
	// if !exists {
	// 	api.GenericResponse(c, 401, "Tenant not found in token", nil)
	// 	return
	// }

	// tenant, ok := tenantInterface.(string)
	// if !ok {
	// 	api.GenericResponse(c, 500, "Invalid tenant format", nil)
	// 	return
	// }

	buildReports, err := db.GlobalMongoStore.GetBuildReports("dge8WM7G7DTcbAjAwvtoHUNxRllTfa_xsFUl8f7778c=", 50)
	if err != nil {
		api.GenericResponse(c, 500, err.Error(), nil)
		return
	}

	api.GenericResponse(c, 200, "Got reports", buildReports)
}

func GetBuildSessions(c *gin.Context) {
	buildID := c.Param("build_id")
	if buildID == "" {
		api.GenericResponse(c, 400, "Build ID is required", nil)
		return
	}

	// tenantInterface, exists := c.Get("tenant")
	// if !exists {
	// 	api.GenericResponse(c, 401, "Tenant not found in token", nil)
	// 	return
	// }

	// tenant, ok := tenantInterface.(string)
	// if !ok {
	// 	api.GenericResponse(c, 500, "Invalid tenant format", nil)
	// 	return
	// }

	sessionReports, err := db.GlobalMongoStore.GetBuildSessions("dge8WM7G7DTcbAjAwvtoHUNxRllTfa_xsFUl8f7778c=", buildID)
	if err != nil {
		api.GenericResponse(c, 500, err.Error(), nil)
		return
	}

	api.GenericResponse(c, 200, "Got build sessions", sessionReports)
}
