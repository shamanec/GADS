package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"

	"GADS/auth"
	"GADS/device"
	"GADS/models"
	"GADS/proxy"
	"GADS/router"
	"GADS/util"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func GetInitialPage(c *gin.Context) {
	var index = template.Must(template.ParseFiles("static/index.html"))
	err := index.Execute(c.Writer, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Could not create the initial page html - %s", err.Error()))
	}
}

func GetSeleniumGridPage(c *gin.Context) {
	var index = template.Must(template.ParseFiles("static/selenium_grid.html"))
	err := index.Execute(c.Writer, util.ConfigData)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Could not create the selenium grid page html - %s", err.Error()))
	}
}

func setLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	// Create/open the log file and set it as logrus output
	project_log_file, err := os.OpenFile("./gads-project.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic(fmt.Sprintf("Could not create/open the gads-project log file for logrus - %s", err))
	}
	log.SetOutput(project_log_file)
}

func handleIndex(c *gin.Context) {
	var tmpl = template.Must(template.ParseFiles("gads-ui/build/index.html"))
	err := tmpl.Execute(c.Writer, nil)
	if err != nil {
		return
	}
}

func handleRequests() {
	// Create the router and allow all origins
	// Also set use of gin session
	r := gin.Default()
	r.Use(cors.Default())

	// Create cookie store
	store := cookie.NewStore([]byte(util.ConfigData.CookiesSecret))
	store.Options(sessions.Options{MaxAge: 60 * 60})
	r.Use(sessions.Sessions("Access-Token", store))

	// Serve static files from the React build folder
	// router.Static("/static", "./gads-ui/build/static")
	// router.Static("/static", "./gads-ui/build/static")
	// router.GET("/", handleIndex)

	// Authenticated endpoints
	authGroup := r.Group("/")
	authGroup.Use(auth.AuthMiddleware())
	authGroup.GET("/logs", GetLogsPage)
	authGroup.GET("/devices", device.LoadDevices)
	authGroup.GET("/", GetInitialPage)
	authGroup.GET("/selenium-grid", GetSeleniumGridPage)
	authGroup.POST("/devices/control/:udid", device.GetDevicePage)
	authGroup.POST("/logout", auth.LogoutHandler)
	authGroup.Any("/device/:udid/*path", proxy.DeviceProxyHandler)
	authGroup.Static("/static", "./static")
	authGroup.POST("/admin/user", router.AddUser)
	authGroup.PUT("/admin/user")    // TODO Update user
	authGroup.DELETE("/admin/user") // TODO Delete user

	// Unauthenticated endpoints
	r.POST("/login", auth.LoginHandler)

	// websockets - unauthenticated
	r.GET("/logs-ws", util.LogsWS)
	r.GET("/available-devices", device.AvailableDeviceWS)
	r.GET("/devices/control/:udid/in-use", device.DeviceInUseWS)

	// Start the GADS UI on the host IP address
	address := fmt.Sprintf("%s:%s", util.ConfigData.GadsHostAddress, util.ConfigData.GadsPort)
	r.Run(address)
}

func main() {
	// Read the config.json and setup the data
	util.GetConfigJsonData()

	// Create a new connection to MongoDB
	util.InitMongo()
	err := util.AddOrUpdateUser(models.User{Username: util.ConfigData.AdminUsername, Password: util.ConfigData.AdminPassword, Role: "admin"})
	fmt.Println(err)

	// Start a goroutine that continiously gets the latest devices data from MongoDB
	go device.GetLatestDBDevices()

	// Start a goroutine that will send an html with the device selection to all clients connected to the socket
	// This creates near real-time updates of the device selection
	go device.GetDevices()

	defer util.MongoClientCtxCancel()

	setLogging()
	handleRequests()
}
