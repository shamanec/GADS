package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"

	"GADS/device"
	"GADS/proxy"
	"GADS/util"

	"github.com/gin-contrib/cors"
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

func setLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	// Create/open the log file and set it as logrus output
	project_log_file, err := os.OpenFile("./gads-project.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic(fmt.Sprintf("Could not create/open the gads-project log file for logrus - %s", err))
	}
	log.SetOutput(project_log_file)
}

func handleRequests() {
	// Create the router and allow all origins
	router := gin.Default()
	router.Use(cors.Default())

	// Other
	router.Static("/static", "./static")
	router.GET("/", GetInitialPage)
	router.GET("/logs", GetLogsPage)
	router.GET("/project-logs", GetLogs)
	router.GET("/logs-ws", util.LogsWS)

	// Devices endpoints
	router.GET("/devices", device.LoadDevices)
	router.GET("/available-devices", device.AvailableDeviceWS)
	router.GET("/available-devices2", device.AvailableDeviceWS2)
	router.POST("/devices/control/:udid", device.GetDevicePage)
	router.Any("/device/:udid/*path", proxy.DeviceProxyHandler)

	// Start the GADS UI on the host IP address
	address := fmt.Sprintf("%s:%s", util.ConfigData.GadsHostAddress, util.ConfigData.GadsPort)
	router.Run(address)
}

func main() {
	// Read the config.json and setup the data
	util.GetConfigJsonData()

	// Create a new connection to MongoDB
	util.InitMongo()

	// Start a goroutine that continiously gets the latest devices data from MongoDB
	go device.GetLatestDBDevices()

	// Start a goroutine that will send an html with the device selection to all clients connected to the socket
	// This creates near real-time updates of the device selection
	go device.GetDevices()
	go device.GetDevices2()

	setLogging()
	handleRequests()
}
