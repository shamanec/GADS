package main

import (
	"html/template"
	"net/http"
	"os"

	"GADS/db"
	"GADS/device"
	"GADS/proxy"
	"GADS/util"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var project_log_file *os.File

func GetInitialPage(c *gin.Context) {
	var index = template.Must(template.ParseFiles("static/index.html"))
	err := index.Execute(c.Writer, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}

func setLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	project_log_file, err := os.OpenFile("./gads-project.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic(err)
	}
	log.SetOutput(project_log_file)
}

func handleRequests() {
	// myRouter.HandleFunc("/configuration/upload-app", util.UploadApp).Methods("POST")

	// myRouter.HandleFunc("/devices/control/{device_udid}", device.GetDevicePage)

	// myRouter.HandleFunc("/proxy/{udid}/{path:.*}", proxy.ProxyHandler)

	ginRouter := gin.Default()
	ginRouter.GET("/", GetInitialPage)
	ginRouter.GET("/logs", GetLogsPage)
	ginRouter.GET("/project-logs", GetLogs)

	ginRouter.GET("/devices", device.LoadDevices)
	ginRouter.GET("/available-devices", device.AvailableDeviceWS)
	ginRouter.POST("/devices/control/:udid", device.GetDevicePage)

	ginRouter.Any("/proxy/:udid/*path", proxy.ProxyHandler)

	ginRouter.Static("/static", "./static")

	ginRouter.Run(":10000")
}

func main() {
	util.GetConfigJsonData()

	db.NewConnection()
	go device.GetLatestDBDevices()
	go device.GetDevices()
	setLogging()
	handleRequests()
}
