package hub

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/common/utils"
	"GADS/hub/devices"
	"GADS/hub/router"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var configData *models.HubConfig

func StartHub(flags *pflag.FlagSet, appVersion string, uiFiles embed.FS, resourceFiles embed.FS) {
	port, _ := flags.GetString("port")
	if port == "" {
		log.Fatalf("Please provide a port on which the hub instance should run through the --port flag, e.g. --port=10000")
	}
	hostAddress, _ := flags.GetString("host-address")
	fmt.Printf("Running hub version `%s`\n", appVersion)
	fmt.Printf("UI accessible on http://%s:%v. You can change the address and port with the --host-address and --port flags\n", hostAddress, port)

	mongoDB, _ := flags.GetString("mongo-db")
	fmt.Printf("Using MongoDB instance on %s. You can change the instance with the --mongo-db flag\n", mongoDB)

	fmt.Println("Default admin username is `admin`")
	fmt.Println("Default admin password is `password` unless you've changed it")

	filesDir, _ := flags.GetString("files-dir")
	osTempDir := os.TempDir()
	var filesTempDir string
	// If a specific folder is provided, unpack the UI files there
	if filesDir != "" {
		_, err := os.Stat(filesDir)
		if err != nil {
			if os.IsNotExist(err) {
				log.Fatalf("The provided files-dir `%s` does not exist - %s", filesDir, err)
			}
			log.Fatalf("Could not check if the provided files-dir `%s` exists - %s", filesDir, err)
		}
		filesTempDir = filesDir
	} else {
		// If no folder is specified, use a temporary directory on the host
		filesTempDir = osTempDir
	}

	config := models.HubConfig{
		HostAddress:  hostAddress,
		Port:         port,
		MongoDB:      mongoDB,
		OSTempDir:    osTempDir,
		FilesTempDir: filesTempDir,
		OS:           runtime.GOOS,
	}

	configData = &config

	// Create a new connection to MongoDB
	db.InitMongoClient(mongoDB)

	devices.InitHubDevicesData()
	// Start a goroutine that continuously gets the latest devices data from MongoDB
	go devices.GetLatestDBDevices()
	// Start a goroutine to clean hanging grid sessions
	go router.UpdateExpiredGridSessions()

	defer db.MongoCtxCancel()

	err := db.AddAdminUserIfMissing()
	if err != nil {
		log.Fatalf("Failed adding admin user on start - %s", err)
	}

	// Check if any workspaces exist
	workspaces := db.GetWorkspaces()
	if len(workspaces) == 0 {
		// Create default workspace if none exist
		defaultWorkspace := models.Workspace{
			ID:          utils.FormatWorkspaceID("Default"),
			Name:        "Default",
			Description: "This is the default workspace.",
		}
		err := db.AddWorkspace(&defaultWorkspace)
		if err != nil {
			log.Fatalf("Failed to create default workspace - %s", err)
		}
	}

	err = setupUIFiles(uiFiles)
	if err != nil {
		log.Fatalf("Failed to unpack UI files in folder `%s` - %s", filesTempDir, err)
	}

	err = setupResources(resourceFiles)
	if err != nil {
		log.Fatalf("Failed to unpack resource files in folder `%s` - %s", filesTempDir, err)
	}

	r := router.HandleRequests(configData)

	// Start the GADS UI on the host IP address
	address := fmt.Sprintf("%s:%s", configData.HostAddress, configData.Port)
	//err = r.RunTLS(address, "./server.crt", "./server.key")
	err = r.Run(address)
	if err != nil {
		log.Fatalf("Gin Run failed - %s", err)
	}
}

func setupUIFiles(uiFiles embed.FS) error {
	embeddedDir := "hub/gads-ui/build"
	targetDir := filepath.Join(configData.FilesTempDir, "gads-ui")

	fmt.Printf("Attempting to unpack embedded UI static files from `%s` to `%s`\n", embeddedDir, targetDir)

	err := os.RemoveAll(targetDir)
	if err != nil {
		return err
	}

	// Ensure the target directory exists
	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
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
		outputPath := filepath.Join(targetDir, path)

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

func setupResources(resourceFiles embed.FS) error {
	embeddedDir := "resources"
	targetDir := filepath.Join(configData.FilesTempDir, "resources")

	fmt.Printf("Attempting to unpack embedded resource files from `%s` to `%s`\n", embeddedDir, targetDir)

	err := os.RemoveAll(targetDir)
	if err != nil {
		return err
	}

	// Ensure the target directory exists
	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		return err
	}

	// Access the embedded directory as if it's the root
	fsSub, err := fs.Sub(resourceFiles, embeddedDir)
	if err != nil {
		return err
	}

	// Walk the 'virtual' root of the embedded filesystem
	err = fs.WalkDir(fsSub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Path here is relative to the 'virtual' root, no need to strip directories
		outputPath := filepath.Join(targetDir, path)

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
