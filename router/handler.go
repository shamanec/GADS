package router

import (
	"GADS/auth"
	"GADS/device"
	"GADS/util"
	"fmt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func HandleRequests(authentication bool) *gin.Engine {
	// Create the router and allow all origins
	// Also set use of gin session
	r := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"X-Auth-Token", "Content-Type"}
	r.Use(cors.New(config))

	// Serve static files from the React build folder
	// router.Static("/static", "./gads-ui/build/static")
	// router.Static("/static", "./gads-ui/build/static")
	// router.GET("/", handleIndex)

	// Authenticated endpoints
	authGroup := r.Group("/")
	if authentication {
		fmt.Printf("Authentication is %v", authentication)
		authGroup.Use(auth.AuthMiddleware())
	}
	authGroup.GET("/logs", GetLogsPage)
	authGroup.GET("/devices", device.LoadDevices)
	authGroup.GET("/", GetInitialPage)
	authGroup.GET("/selenium-grid", GetSeleniumGridPage)
	authGroup.POST("/devices/control/:udid", device.GetDevicePage)
	authGroup.POST("/logout", auth.LogoutHandler)
	authGroup.Any("/device/:udid/*path", DeviceProxyHandler)
	authGroup.Static("/static", "./static")
	authGroup.POST("/admin/user", AddUser)
	authGroup.PUT("/admin/user")    // TODO Update user
	authGroup.DELETE("/admin/user") // TODO Delete user

	// Unauthenticated endpoints
	r.POST("/authenticate", auth.LoginHandler)

	// websockets - unauthenticated
	r.GET("/logs-ws", util.LogsWS)
	r.GET("/available-devices", device.AvailableDeviceWS)
	r.GET("/devices/control/:udid/in-use", device.DeviceInUseWS)

	return r
}
