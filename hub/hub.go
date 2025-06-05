/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

package hub

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/hub/auth"
	"GADS/hub/devices"
	"GADS/hub/router"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/swaggo/swag/example/basic/docs"
	"go.mongodb.org/mongo-driver/mongo"
)

var configData *models.HubConfig

func StartHub(flags *pflag.FlagSet, appVersion string, uiFiles fs.FS, resourceFiles embed.FS) {
	port, _ := flags.GetString("port")
	if port == "" {
		log.Fatalf("Please provide a port on which the hub instance should run through the --port flag, e.g. --port=10000")
	}
	hostAddress, _ := flags.GetString("host-address")
	fmt.Printf("Running hub version `%s`\n", appVersion)
	fmt.Printf("UI accessible on http://%s:%v. You can change the address and port with the --host-address and --port flags\n", hostAddress, port)

	mongoDB, _ := flags.GetString("mongo-db")
	fmt.Printf("Using MongoDB instance on %s. You can change the instance with the --mongo-db flag\n", mongoDB)

	authEnabled, _ := flags.GetBool("auth")
	fmt.Printf("Auth enabled: %v. You can enable/disable authentication on hub endpoints with the --auth flag\n", authEnabled)

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
		AuthEnabled:  authEnabled,
	}

	configData = &config

	db.InitMongo(mongoDB, "gads")
	defer db.GlobalMongoStore.Close()

	// Initialize the secret key cache
	secretStore := auth.NewSecretStore(db.GlobalMongoStore.GetDefaultDatabase())
	err := auth.InitSecretCache(secretStore, 5*time.Minute)
	if err != nil {
		log.Fatalf("Failed to initialize secret key cache: %v", err)
	}

	devices.InitHubDevicesData()
	// Start a goroutine that continuously gets the latest devices data from MongoDB
	go devices.GetLatestDBDevices()
	// Start a goroutine to clean hanging grid sessions
	go router.UpdateExpiredGridSessions()

	err = db.GlobalMongoStore.AddAdminUserIfMissing()
	if err != nil {
		log.Fatalf("Failed adding admin user on start - %s", err)
	}

	_, err = db.GlobalMongoStore.GetGlobalStreamSettings()
	if err != nil {
		log.Fatalf("Failed to get/update global stream settings - %s", err)
	}

	// Check if the default workspace exists
	defaultWorkspace, err := db.GlobalMongoStore.GetDefaultWorkspace()
	if err != nil {
		// Create default workspace if none exist
		defaultWorkspace = models.Workspace{
			Name:        "Default Workspace",
			Description: "This is the default workspace.",
			IsDefault:   true,
		}
		err := db.GlobalMongoStore.AddWorkspace(&defaultWorkspace)
		if err != nil {
			log.Fatalf("Failed to create default workspace - %s", err)
		}
	}

	// Associate users to default workspace if needed
	users, _ := db.GlobalMongoStore.GetUsers()
	for _, user := range users {
		// Skip admin users as they have access to all workspaces
		if user.Role == "admin" {
			continue
		}

		// If user has no workspaces at all then associate them with default workspace
		if len(user.WorkspaceIDs) == 0 {
			err := db.GlobalMongoStore.UpdateUserWorkspaces(user.Username, []string{defaultWorkspace.ID})
			if err != nil {
				log.Printf("Failed to associate user %s with default workspace - %s", user.Username, err)
				continue
			}
		}
	}

	// Associate devices to default workspace if needed
	devices, _ := db.GlobalMongoStore.GetDevices()
	for _, device := range devices {
		// If device has no workspace at all associate with the default workspace
		if device.WorkspaceID == "" {
			device.WorkspaceID = defaultWorkspace.ID
			err := db.GlobalMongoStore.AddOrUpdateDevice(&device)
			if err != nil {
				log.Printf("Failed to associate device %s with default workspace - %s", device.UDID, err)
				continue
			}
		} else {
			// If device has a workspace but it does not exist (for example it was default but default was deleted)
			// Then associate them with default workspace
			_, err := db.GlobalMongoStore.GetWorkspaceByID(device.WorkspaceID)
			if err != nil && err == mongo.ErrNoDocuments {
				device.WorkspaceID = defaultWorkspace.ID
				err := db.GlobalMongoStore.AddOrUpdateDevice(&device)
				if err != nil {
					log.Printf("Failed to associate device %s with default workspace - %s", device.UDID, err)
					continue
				}
			}
		}
	}

	err = setupResources(resourceFiles)
	if err != nil {
		log.Fatalf("Failed to unpack resource files in folder `%s` - %s", filesTempDir, err)
	}

	// Configure Swagger documentation
	docs.SwaggerInfo.Title = "GADS API"
	docs.SwaggerInfo.Description = "GADS API for device automation and management"
	docs.SwaggerInfo.Version = appVersion
	docs.SwaggerInfo.Host = fmt.Sprintf("%s:%s", hostAddress, port)
	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Schemes = []string{"http", "https"}

	r := router.HandleRequests(configData, uiFiles)

	// Start the GADS UI on the host IP address
	address := fmt.Sprintf("%s:%s", configData.HostAddress, configData.Port)
	//err = r.RunTLS(address, "./server.crt", "./server.key")
	err = r.Run(address)
	if err != nil {
		log.Fatalf("Gin Run failed - %s", err)
	}
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
