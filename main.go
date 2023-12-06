package main

import (
	"flag"
	"fmt"
	"html/template"
	"os"

	"GADS/device"
	"GADS/models"
	"GADS/router"
	"GADS/util"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

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

func main() {
	auth_flag := flag.Bool("auth", false, "If authentication should be turned on")
	flag.Parse()

	// Read the config.json and setup the data
	util.GetConfigJsonData()

	// Create a new connection to MongoDB
	util.InitMongo()
	err := util.AddOrUpdateUser(models.User{Username: util.ConfigData.AdminUsername, Email: util.ConfigData.AdminEmail, Password: util.ConfigData.AdminPassword, Role: "admin"})
	fmt.Println(err)

	// Start a goroutine that continiously gets the latest devices data from MongoDB
	go device.GetLatestDBDevices()

	// Start a goroutine that will send an html with the device selection to all clients connected to the socket
	// This creates near real-time updates of the device selection
	go device.GetDevices()

	defer util.MongoClientCtxCancel()

	setLogging()

	r := router.HandleRequests(*auth_flag)

	// Start the GADS UI on the host IP address
	address := fmt.Sprintf("%s:%s", util.ConfigData.GadsHostAddress, util.ConfigData.GadsPort)
	r.Run(address)
}
