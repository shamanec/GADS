package hub

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/hub/devices"
	"GADS/hub/router"
	"embed"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"io/fs"
	"os"
	"path/filepath"
)

func setLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	// Create/open the log file and set it as logrus output
	projectLogFile, err := os.OpenFile("./gads-project.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		log.Fatalf("Could not create/open the gads-project log file for logrus - %s", err)
	}
	log.SetOutput(projectLogFile)
}

//go:embed gads-ui/build
var uiFiles embed.FS

func StartHub(flags *pflag.FlagSet) {
	port, _ := flags.GetString("port")
	if port == "" {
		log.Fatalf("Please provide a port on which the hub instance should run through the --port flag, e.g. --port=10000")
	}
	hostAddress, _ := flags.GetString("host-address")
	fmt.Printf("UI accessible on http://%s:%v. You can change the address and port with the --host-address and --port flags\n", hostAddress, port)

	mongoDB, _ := flags.GetString("mongo-db")
	fmt.Printf("Using MongoDB instance on %s. You can change the instance with the --mongo-db flag\n", mongoDB)

	fmt.Println("Default admin username is `admin`")
	fmt.Println("Default admin password is `password` unless you've changed it")

	uiFilesDir, _ := flags.GetString("ui-files-dir")
	osTempDir := os.TempDir()
	var uiFilesTempDir string
	// If a specific folder is provided, unpack the UI files there
	if uiFilesDir != "" {
		_, err := os.Stat(uiFilesDir)
		if err != nil {
			if os.IsNotExist(err) {
				log.Fatalf("The provided ui-files-dir `%s` does not exist - %s", uiFilesDir, err)
			}
			log.Fatalf("Could not check if the provided ui-files-dir `%s` exists - %s", uiFilesDir, err)
		}
		uiFilesTempDir = filepath.Join(uiFilesDir, "gads-ui")
	} else {
		// If no folder is specified, use a temporary directory on the host
		uiFilesTempDir = filepath.Join(osTempDir, "gads-ui")
	}
	fmt.Printf("UI static files will be unpacked in `%s`\n", uiFilesTempDir)

	config := models.HubConfig{
		HostAddress:    hostAddress,
		Port:           port,
		MongoDB:        mongoDB,
		OSTempDir:      osTempDir,
		UIFilesTempDir: uiFilesTempDir,
	}

	devices.ConfigData = &config

	// Create a new connection to MongoDB
	db.InitMongoClient(mongoDB)

	// Start a goroutine that continuously gets the latest devices data from MongoDB
	go devices.GetLatestDBDevices()

	defer db.MongoCtxCancel()

	setLogging()
	fmt.Println("")

	err := db.AddAdminUserIfMissing()
	if err != nil {
		log.Fatalf("Failed adding admin user on start - %s", err)
	}

	err = setupUIFiles()
	if err != nil {
		log.Fatalf("Failed to unpack UI files in folder `%s` - %s", uiFilesTempDir, err)
	}

	r := router.HandleRequests()

	// Start the GADS UI on the host IP address
	address := fmt.Sprintf("%s:%s", devices.ConfigData.HostAddress, devices.ConfigData.Port)
	//err = r.RunTLS(address, "./server.crt", "./server.key")
	err = r.Run(address)
	if err != nil {
		log.Fatalf("Gin Run failed - %s", err)
	}
}

func setupUIFiles() error {
	embeddedDir := "gads-ui/build"

	fmt.Printf("Attempting to unpack embedded UI static files from `%s` to `%s`\n", embeddedDir, devices.ConfigData.UIFilesTempDir)

	err := os.RemoveAll(devices.ConfigData.UIFilesTempDir)
	if err != nil {
		return err
	}

	// Ensure the target directory exists
	if err := os.MkdirAll(devices.ConfigData.UIFilesTempDir, os.ModePerm); err != nil {
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
		outputPath := filepath.Join(devices.ConfigData.UIFilesTempDir, path)

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
