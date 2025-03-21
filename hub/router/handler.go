package router

import (
	"GADS/common/models"
	"GADS/hub/auth"
	"path/filepath"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

func HandleRequests(configData *models.HubConfig) *gin.Engine {
	// Create the router and allow all origins
	// Allow particular headers as well
	r := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"X-Auth-Token", "Content-Type"}
	r.Use(cors.New(config))

	filesDir := filepath.Join(configData.FilesTempDir, "gads-ui")
	indexHtmlPath := filepath.Join(filesDir, "index.html")

	// Configuration for SAP applications
	// Serve the static files from the built React app
	r.Use(static.Serve("/", static.LocalFile(filesDir, true)))
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
	authGroup.GET("/admin/files", GetFiles)
	authGroup.POST("/admin/download-github-file", DownloadResourceFromGithubRepo)
	authGroup.POST("/admin/upload-file", UploadFile)
	authGroup.PUT("/admin/user", UpdateUser)
	authGroup.DELETE("/admin/user/:nickname", DeleteUser)
	authGroup.GET("/admin/global-settings", GetGlobalStreamSettings)
	authGroup.POST("/admin/global-settings", UpdateGlobalStreamSettings)
	authGroup.POST("/admin/workspaces", CreateWorkspace)
	authGroup.PUT("/admin/workspaces", UpdateWorkspace)
	authGroup.DELETE("/admin/workspaces/:id", DeleteWorkspace)
	authGroup.GET("/admin/workspaces", GetWorkspaces)
	authGroup.GET("/workspaces", GetUserWorkspaces)
	appiumGroup := r.Group("/grid")
	appiumGroup.Use(AppiumGridMiddleware())
	appiumGroup.Any("/*path")

	return r
}
