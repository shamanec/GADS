package router

import (
	"GADS/hub/auth"
	"GADS/hub/util"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"path/filepath"
)

func HandleRequests(authentication bool) *gin.Engine {
	// Create the router and allow all origins
	// Allow particular headers as well
	r := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"X-Auth-Token", "Content-Type"}
	r.Use(cors.New(config))

	indexHtmlPath := filepath.Join(util.ConfigData.UIFilesTempDir, "index.html")

	// Configuration for SAP applications
	// Serve the static files from the built React app
	r.Use(static.Serve("/", static.LocalFile(util.ConfigData.UIFilesTempDir, true)))
	// For any missing route serve the index.htm from the static files
	// This will fix the issue with accessing particular endpoint in the browser manually or with refresh
	r.NoRoute(func(c *gin.Context) {
		c.File(indexHtmlPath)
	})

	authGroup := r.Group("/")
	// Unauthenticated endpoints
	authGroup.POST("/authenticate", auth.LoginHandler)
	// websockets - unauthenticated
	authGroup.GET("/available-devices", AvailableDevicesSSE)
	authGroup.POST("/devices/control/:udid/in-use", DeviceInUse)
	authGroup.GET("/admin/provider/:nickname/info", ProviderInfoSSE)
	// Enable authentication on the endpoints below
	if authentication {
		authGroup.Use(auth.AuthMiddleware())
	}
	authGroup.GET("/appium-logs", GetAppiumLogs)
	authGroup.GET("/appium-session-logs", GetAppiumSessionLogs)
	authGroup.GET("/health", HealthCheck)
	authGroup.POST("/devices/control/:udid", GetDevicePage)
	authGroup.POST("/logout", auth.LogoutHandler)
	authGroup.Any("/device/:udid/*path", DeviceProxyHandler)
	authGroup.Any("/provider/:name/*path", ProviderProxyHandler)
	authGroup.GET("/admin/providers", GetProviders)
	authGroup.POST("/admin/providers/add", AddProvider)
	authGroup.POST("/admin/providers/update", UpdateProvider)
	authGroup.POST("/admin/devices/add", AddNewDevice)
	authGroup.POST("/admin/user", AddUser)
	authGroup.PUT("/admin/user")    // TODO Update user
	authGroup.DELETE("/admin/user") // TODO Delete user

	return r
}
