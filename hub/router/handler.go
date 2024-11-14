package router

import (
	"GADS/hub/auth"
	"GADS/hub/devices"
	"path/filepath"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

func HandleRequests() *gin.Engine {
	// Create the router and allow all origins
	// Allow particular headers as well
	r := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"X-Auth-Token", "Content-Type"}
	r.Use(cors.New(config))

	indexHtmlPath := filepath.Join(devices.ConfigData.UIFilesTempDir, "index.html")

	// Configuration for SAP applications
	// Serve the static files from the built React app
	r.Use(static.Serve("/", static.LocalFile(devices.ConfigData.UIFilesTempDir, true)))
	// For any missing route serve the index.htm from the static files
	// This will fix the issue with accessing particular endpoint in the browser manually or with refresh
	r.NoRoute(func(c *gin.Context) {
		c.File(indexHtmlPath)
	})

	authGroup := r.Group("/")
	// Unauthenticated endpoints
	authGroup.POST("/authenticate", auth.LoginHandler)
	authGroup.GET("/available-devices", AvailableDevicesSSE)
	authGroup.GET("/admin/provider/:nickname/info", ProviderInfoSSE)
	authGroup.GET("/devices/control/:udid/in-use", DeviceInUseWS)
	authGroup.POST("/provider-update", ProviderUpdate)
	// Enable authentication on the endpoints below
	authGroup.Use(auth.AuthMiddleware())
	authGroup.GET("/appium-logs", GetAppiumLogs)
	authGroup.GET("/appium-session-logs", GetAppiumSessionLogs)
	authGroup.GET("/health", HealthCheck)
	authGroup.POST("/logout", auth.LogoutHandler)
	authGroup.Any("/device/:udid/*path", DeviceProxyHandler)
	authGroup.Any("/provider/:name/*path", ProviderProxyHandler)
	authGroup.GET("/admin/providers", GetProviders)
	authGroup.POST("/admin/providers/add", AddProvider)
	authGroup.POST("/admin/providers/update", UpdateProvider)
	authGroup.DELETE("/admin/providers/:nickname", DeleteProvider)
	authGroup.GET("/admin/providers/logs", GetProviderLogs)
	authGroup.POST("/admin/device", AddDevice)
	authGroup.PUT("/admin/device", UpdateDevice)
	authGroup.DELETE("/admin/device/:udid", DeleteDevice)
	authGroup.POST("/admin/device/:udid/release", ReleaseUsedDevice)
	authGroup.GET("/admin/devices", GetDevices)
	authGroup.POST("/admin/user", AddUser)
	authGroup.GET("/admin/users", GetUsers)
	authGroup.POST("/admin/upload-selenium-jar", UploadSeleniumJar)
	authGroup.PUT("/admin/user", UpdateUser)
	authGroup.DELETE("/admin/user/:nickname", DeleteUser)
	appiumGroup := r.Group("/grid")
	appiumGroup.Use(AppiumGridMiddleware())
	appiumGroup.Any("/*path")

	return r
}
