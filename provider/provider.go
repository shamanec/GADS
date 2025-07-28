/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package provider

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/devices"
	"GADS/provider/logger"
	"GADS/provider/providerutil"
	"GADS/provider/router"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/pflag"
)

func StartProvider(flags *pflag.FlagSet, resourceFiles embed.FS) {
	logLevel, _ := flags.GetString("log-level")
	nickname, _ := flags.GetString("nickname")
	mongoDb, _ := flags.GetString("mongo-db")
	providerFolder, _ := flags.GetString("provider-folder")
	hubAddress, _ := flags.GetString("hub")

	if nickname == "" {
		log.Fatalf("Please provide valid provider instance nickname via the --nickname flag, e.g. --nickname=Provider1")
	}

	if hubAddress == "" {
		log.Fatalf("Please provide valid GADS hub instance address via the --hub flag, e.g. --hub=http://192.168.1.6:10000")
	}

	if providerFolder == "." {
		providerFolder = fmt.Sprintf("./%s", nickname)
	}

	fmt.Println("Preparing...")

	// Create the provider folder if needed
	err := os.MkdirAll(providerFolder, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create provider folder `%s` - %s", providerFolder, err)
	}

	db.InitMongo(mongoDb, "gads")
	defer db.GlobalMongoStore.Close()

	// Set up the provider configuration
	config.SetupConfig(nickname, providerFolder, hubAddress)
	config.ProviderConfig.OS = runtime.GOOS

	// Setup logging for the provider itself
	logger.SetupLogging(logLevel)
	logger.ProviderLogger.LogInfo("provider_setup", fmt.Sprintf("Starting provider on port `%v`", config.ProviderConfig.Port))

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
		logger.ProviderLogger.LogInfo("provider_setup", "Created default workspace")
	}

	if config.ProviderConfig.SetupAppiumServers {
		logger.ProviderLogger.LogInfo("provider_setup", "Checking if Appium is installed and available on the host")
		if !providerutil.AppiumAvailable() {
			log.Fatal("Appium is not available, set it up on the host as explained in the readme")
		}

		logger.ProviderLogger.LogInfo("provider_setup", "Checking if GADS Appium plugin is installed on the host NPM")
		if !providerutil.CheckAppiumPluginInstalledNPM() {
			logger.ProviderLogger.LogInfo("provider_setup", "Installing GADS Appium plugin globally on host NPM")
			err = providerutil.InstallAppiumPluginNPM()
			if err != nil {
				log.Fatalf("Failed to install GADS Appium plugin on NPM - %s", err)
			}
			logger.ProviderLogger.LogInfo("provider_setup", "Successfully installed GADS Appium plugin globally on host NPM")
		} else {
			logger.ProviderLogger.LogInfo("provider_setup", "GADS Appium plugin already installed on host NPM")
		}
		logger.ProviderLogger.LogInfo("provider_setup", "Checking if GADS plugin is installed on Appium")
		if !providerutil.CheckAppiumPluginInstalled() {
			logger.ProviderLogger.LogInfo("provider_setup", "Installing GADS plugin on Appium")
			err = providerutil.InstallAppiumPlugin()
			if err != nil {
				log.Fatalf("Failed to install GADS plugin on Appium - %s", err)
			}
			logger.ProviderLogger.LogInfo("provider_setup", "Successfully installed GADS plugin on Appium")
		} else {
			logger.ProviderLogger.LogInfo("provider_setup", "GADS plugin already installed on Appium")
		}
	} else {
		logger.ProviderLogger.LogInfo("provider_setup", "Provider is not configured to set up Appium servers, skipped Appium availability check")
	}

	// Download supervision profile file from MongoDB if a supervision password was supplied
	if config.ProviderConfig.SupervisionPassword != "" {
		err = config.SetupIOSSupervisionProfileFile()
		if err != nil {
			log.Fatalf("You've set up a supervision profile password but there is something wrong with providing the supervision profile file from MongoDB - %s", err)
		}
	}

	if config.ProviderConfig.ProvideIOS {
		err = config.SetupWebDriverAgentFile()
		if err != nil {
			log.Fatalf("Could not provide WebDriverAgent.ipa file from MongoDB - %s", err)
		}
	}

	err = extractProviderResourceFiles(config.ProviderConfig.ProviderFolder, resourceFiles)
	if err != nil {
		log.Fatalf("Failed to extract embedded resource files - %s", err)
	}

	// Finalize grid configuration if Selenium Grid usage enabled
	if config.ProviderConfig.UseSeleniumGrid {
		err = config.SetupSeleniumJar()
		if err != nil {
			log.Fatalf("Selenium Grid connection is enabled but there is something wrong with providing the selenium jar file from MongoDB - %s", err)
		}
	}

	// If we want to provide Android devices check if adb is available on PATH
	if config.ProviderConfig.ProvideAndroid {
		if !providerutil.AdbAvailable() {
			logger.ProviderLogger.LogError("provider", "adb is not available, you need to set up the host as explained in the readme")
			fmt.Println("adb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}

		// Try to remove potentially hanging ports forwarded by adb
		providerutil.RemoveAdbForwardedPorts()
	}

	config.ProviderConfig.RegularizeProviderState()

	// If we want to provide Tizen devices check if sdb is available on PATH
	if config.ProviderConfig.ProvideTizen {
		if !providerutil.SdbAvailable() {
			logger.ProviderLogger.LogError("provider", "sdb is not available, you need to set up the host as explained in the readme")
			fmt.Println("sdb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	// If we want to provide WebOS devices check if ares-setup-device is available on PATH
	if config.ProviderConfig.ProvideWebOS {
		if !providerutil.AresAvailable() {
			logger.ProviderLogger.LogError("provider", "ares-setup-device is not available, you need to set up the host as explained in the readme")
			fmt.Println("ares-setup-device is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	// Start a goroutine that will start updating devices on provider start
	go devices.Listener()

	// Start the provider server
	err = startHTTPServer()
	if err != nil {
		log.Fatal("HTTP server stopped")
	}
}

func startHTTPServer() error {
	// Handle the endpoints
	r := router.HandleRequests()
	// Start periodically updating the provider data in the DB
	go updateProviderInDB()
	// Start the provider
	address := fmt.Sprintf("%s:%v", config.ProviderConfig.HostAddress, config.ProviderConfig.Port)
	err := r.Run(address)
	if err != nil {
		return err
	}
	return fmt.Errorf("HTTP server stopped due to an unknown reason")
}

// Periodically send current provider data updates to MongoDB
func updateProviderInDB() {
	for {
		err := db.GlobalMongoStore.UpdateProviderTimestamp(config.ProviderConfig.Nickname)
		if err != nil {
			logger.ProviderLogger.LogError("update_provider", fmt.Sprintf("Failed to upsert provider in DB - %s", err))
		}

		time.Sleep(1 * time.Second)
	}
}

func extractProviderResourceFiles(destination string, resourceFiles embed.FS) error {
	files := []string{"gads-settings.apk"}
	for _, file := range files {
		data, err := resourceFiles.ReadFile("resources/" + file)
		if err != nil {
			return err
		}

		outPath := filepath.Join(destination, file)

		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return err
		}

		err = os.WriteFile(outPath, data, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}
