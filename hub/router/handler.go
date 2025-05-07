package router

import (
	"GADS/common/models"
	"GADS/hub/auth"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func HandleRequests(configData *models.HubConfig, uiFiles fs.FS) *gin.Engine {
	// Create the router and allow all origins
	// Allow particular headers as well
	r := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"Authorization", "Content-Type"}
	r.Use(cors.New(config))

	// Handle UI serving only if we have UI files embedded
	if uiFiles != nil {
		uiFS, err := fs.Sub(uiFiles, "hub-ui/build")
		if err != nil {
			log.Fatalf("Failed to get UI files filesystem: %v", err)
		}

		r.Use(func(c *gin.Context) {
			path := c.Request.URL.Path

			if path != "/" {
				_, err := uiFS.Open(strings.TrimPrefix(path, "/"))
				if err != nil {
					return
				}
			}

			fileServer := http.FileServer(http.FS(uiFS))
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
		})

		r.NoRoute(func(c *gin.Context) {
			indexFile, err := uiFS.Open("index.html")
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			defer indexFile.Close()

			stat, err := indexFile.Stat()
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), indexFile.(io.ReadSeeker))
		})
	}

	authGroup := r.Group("/")
	// Unauthenticated endpoints
	authGroup.POST("/authenticate", auth.LoginHandler)
	authGroup.GET("/available-devices", AvailableDevicesSSE)
	authGroup.GET("/admin/provider/:nickname/info", ProviderInfoSSE)
	authGroup.GET("/devices/control/:udid/in-use", DeviceInUseWS)
	authGroup.GET("devices/control/:udid/webrtc", DeviceWebRTCWS)
	authGroup.POST("/provider-update", ProviderUpdate)
	// Enable authentication on the endpoints below
	if configData.AuthEnabled {
		authGroup.Use(auth.AuthMiddleware())
	}
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
