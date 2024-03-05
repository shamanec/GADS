package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"GADS/device"
	"GADS/models"
	"GADS/router"
	"GADS/util"

	log "github.com/sirupsen/logrus"
)

func setLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	// Create/open the log file and set it as logrus output
	projectLogFile, err := os.OpenFile("./gads-project.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic(fmt.Sprintf("Could not create/open the gads-project log file for logrus - %s", err))
	}
	log.SetOutput(projectLogFile)
}

//go:embed gads-ui/build
var uiFiles embed.FS

func main() {
	authFlag := flag.Bool("auth", false, "If authentication should be turned on")
	hostAddress := flag.String("host-address", "localhost", "The IP address of the host machine, defaults to `localhost`")
	port := flag.String("port", "10000", "The port on which the UI should be accessed")
	mongoDB := flag.String("mongo-db", "localhost:27017", "The address of the MongoDB instance")
	adminUser := flag.String("admin-username", "admin", "Username for the default admin user")
	adminPassword := flag.String("admin-password", "password", "Password for the default admin user")
	adminEmail := flag.String("admin-email", "admin@gads.ui", "Email for the default admin user")
	flag.Parse()

	// Print out some useful information
	fmt.Printf("Using MongoDB instance on %s. You can change the instance with the --mongo-db flag\n", *mongoDB)
	fmt.Printf("Authentication enabled: %v\n", *authFlag)
	fmt.Printf("UI accessible on http://%s:%v. You can change the address and port with the --host-address and --port flags\n", *hostAddress, *port)
	fmt.Println("Adding admin user with:")
	fmt.Printf(" Name: %s. You can change the name with the --admin-username flag\n", *adminUser)
	fmt.Printf(" Password: %s. You can change the password with the --admin-password flag\n", *adminPassword)
	fmt.Printf(" Email: %s. You can change the email with the --admin-email flag\n", *adminEmail)

	config := util.ConfigJsonData{
		HostAddress:   *hostAddress,
		Port:          *port,
		MongoDB:       *mongoDB,
		AdminUsername: *adminUser,
		AdminEmail:    *adminEmail,
		AdminPassword: *adminPassword,
	}

	util.ConfigData = &config

	// Create a new connection to MongoDB
	util.InitMongo()
	err := util.AddOrUpdateUser(models.User{Username: util.ConfigData.AdminUsername, Email: util.ConfigData.AdminEmail, Password: util.ConfigData.AdminPassword, Role: "admin"})
	fmt.Println(err)

	// Start a goroutine that continuously gets the latest devices data from MongoDB
	go device.GetLatestDBDevices()

	// Start a goroutine that will get latest devices data from DB and sends it to all connected clients
	// This creates near real-time updates of the device selection
	go device.GetDevices()

	defer util.MongoClientCtxCancel()

	setLogging()
	fmt.Println("")

	r := router.HandleRequests(*authFlag, uiFiles)

	// Start the GADS UI on the host IP address
	address := fmt.Sprintf("%s:%s", util.ConfigData.HostAddress, util.ConfigData.Port)
	err = r.Run(address)
	if err != nil {
		log.Fatalf("Gin Run failed - %s", err)
	}
}
