package router

import (
	"GADS/auth"
	"GADS/device"
	"GADS/util"
	"html/template"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

func handleIndex(c *gin.Context) {
	var tmpl = template.Must(template.ParseFiles("gads-ui/build/index.html"))
	err := tmpl.Execute(c.Writer, nil)
	if err != nil {
		return
	}
}

func HandleRequests(authentication bool) *gin.Engine {
	// Create the router and allow all origins
	// Allow particular headers as well
	r := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"X-Auth-Token", "Content-Type"}
	r.Use(cors.New(config))

	// Configuration for SAP applications
	// Serve the static files from the built React app
	r.Use(static.Serve("/", static.LocalFile("./gads-ui/build", true)))
	// For any missing route serve the index.html from the static files
	// This will fix the issue with accessing particular endpoint in the browser manually or with refresh
	r.NoRoute(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.RequestURI, "/api") {
			c.File("./gads-ui/build/index.html")
		}
	})

	authGroup := r.Group("/")
	// Unauthenticated endpoints
	authGroup.POST("/authenticate", auth.LoginHandler)
	// websockets - unauthenticated
	authGroup.GET("/logs-ws", util.LogsWS)
	authGroup.GET("/available-devices", device.AvailableDeviceWS)
	authGroup.GET("/devices/control/:udid/in-use", device.DeviceInUseWS)
	authGroup.GET("/admin/provider/:nickname/info-ws", ProviderInfoWS)
	// Enable authentication on the endpoints below
	if authentication {
		authGroup.Use(auth.AuthMiddleware())
	}
	authGroup.GET("/health", HealthCheck)
	authGroup.POST("/devices/control/:udid", device.GetDevicePage)
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
