package main

import (
	"GADS/device"
	"GADS/models"
	"GADS/router"
	"GADS/util"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

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
	uiFilesDir := flag.String("ui-files-dir", "",
		"Directory where the UI static files will be unpacked and served from."+
			"\nBy default app will try to use a temp dir on the host, use this flag only if you encounter issues with the temp folder."+
			"\nAlso you need to have created the folder in advance!")
	flag.Parse()

	osTempDir := os.TempDir()
	var uiFilesTempDir string
	// If a specific folder is provided, unpack the UI files there
	if *uiFilesDir != "" {
		_, err := os.Stat(*uiFilesDir)
		if err != nil {
			if os.IsNotExist(err) {
				log.Fatalf("The provided ui-files-dir `%s` does not exist - %s", *uiFilesDir, err)
			}
			log.Fatalf("Could not check if the provided ui-files-dir `%s` exists - %s", *uiFilesDir, err)
		}
		uiFilesTempDir = filepath.Join(*uiFilesDir, "gads-ui")
	} else {
		// If no folder is specified, use a temporary directory on the host
		uiFilesTempDir = filepath.Join(osTempDir, "gads-ui")
	}

	// Print out some useful information
	fmt.Printf("Using MongoDB instance on %s. You can change the instance with the --mongo-db flag\n", *mongoDB)
	fmt.Printf("Authentication enabled: %v\n", *authFlag)
	fmt.Printf("UI accessible on http://%s:%v. You can change the address and port with the --host-address and --port flags\n", *hostAddress, *port)
	fmt.Println("Adding admin user with:")
	fmt.Printf(" Name: %s. You can change the name with the --admin-username flag\n", *adminUser)
	fmt.Printf(" Password: %s. You can change the password with the --admin-password flag\n", *adminPassword)
	fmt.Printf(" Email: %s. You can change the email with the --admin-email flag\n", *adminEmail)
	fmt.Printf("UI static files will be unpacked in `%s`\n", uiFilesTempDir)

	config := util.ConfigJsonData{
		HostAddress:    *hostAddress,
		Port:           *port,
		MongoDB:        *mongoDB,
		AdminUsername:  *adminUser,
		AdminEmail:     *adminEmail,
		AdminPassword:  *adminPassword,
		OSTempDir:      osTempDir,
		UIFilesTempDir: uiFilesTempDir,
	}

	util.ConfigData = &config

	// Create a new connection to MongoDB
	util.InitMongo()
	err := util.AddOrUpdateUser(models.User{Username: util.ConfigData.AdminUsername, Email: util.ConfigData.AdminEmail, Password: util.ConfigData.AdminPassword, Role: "admin"})
	fmt.Println(err)

	// Start a goroutine that continuously gets the latest devices data from MongoDB
	go device.GetLatestDBDevices()

	defer util.MongoClientCtxCancel()

	setLogging()
	fmt.Println("")

	err = setupUIFiles()
	if err != nil {
		log.Fatalf("Failed to unpack UI files in folder `%s` - %s", uiFilesTempDir, err)
	}

	r := router.HandleRequests(*authFlag)

	// Start the GADS UI on the host IP address
	address := fmt.Sprintf("%s:%s", util.ConfigData.HostAddress, util.ConfigData.Port)
	//err = r.RunTLS(address, "./server.crt", "./server.key")
	err = r.Run(address)
	if err != nil {
		log.Fatalf("Gin Run failed - %s", err)
	}
}

func setupUIFiles() error {
	embeddedDir := "gads-ui/build"

	fmt.Printf("Attempting to unpack embedded UI static files from `%s` to `%s`\n", embeddedDir, util.ConfigData.UIFilesTempDir)

	err := os.RemoveAll(util.ConfigData.UIFilesTempDir)
	if err != nil {
		return err
	}

	// Ensure the target directory exists
	if err := os.MkdirAll(util.ConfigData.UIFilesTempDir, os.ModePerm); err != nil {
		return err
	}

	// Access the embedded directory as if it's the root
	fsSub, err := fs.Sub(uiFiles, embeddedDir)
	if err != nil {
		return err
	}

	// Walk the 'virtual' root of the embedded filesystem
	err = fs.WalkDir(fsSub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Path here is relative to the 'virtual' root, no need to strip directories
		outputPath := filepath.Join(util.ConfigData.UIFilesTempDir, path)

		if d.IsDir() {
			// Create directory
			return os.MkdirAll(outputPath, os.ModePerm)
		}

		// Read file data from the 'virtual' root
		data, err := fs.ReadFile(fsSub, path)
		if err != nil {
			return err
		}

		// Write file data
		return os.WriteFile(outputPath, data, os.ModePerm)
	})

	if err != nil {
		return err
	}

	return nil
}
