package provider

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/devices"
	"GADS/provider/logger"
	"GADS/provider/providerutil"
	"GADS/provider/router"
	"context"
	"fmt"
	"github.com/spf13/pflag"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

func StartProvider(flags *pflag.FlagSet) {
	logLevel, _ := flags.GetString("log-level")
	nickname, _ := flags.GetString("nickname")
	mongoDb, _ := flags.GetString("mongo-db")
	providerFolder, _ := flags.GetString("provider-folder")

	if nickname == "" {
		log.Fatalf("Please provide valid provider instance nickname via the --nickname flag, e.g. --nickname=Provider1")
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

	// Create a connection to Mongo
	db.InitMongoClient(mongoDb)
	defer db.MongoCtxCancel()
	// Set up the provider configuration
	config.SetupConfig(nickname, providerFolder)
	config.Config.EnvConfig.OS = runtime.GOOS
	// Defer closing the Mongo connection on provider stopped
	defer db.CloseMongoConn()

	// Setup logging for the provider itself
	logger.SetupLogging(logLevel)
	logger.ProviderLogger.LogInfo("provider_setup", fmt.Sprintf("Starting provider on port `%v`", config.Config.EnvConfig.Port))

	logger.ProviderLogger.LogInfo("provider_setup", "Checking if Appium is installed and available on the host")
	if !providerutil.AppiumAvailable() {
		log.Fatal("Appium is not available, set it up on the host as explained in the readme")
	}

	// Finalize grid configuration if Selenium Grid usage enabled
	if config.Config.EnvConfig.UseSeleniumGrid {
		configureSeleniumSettings()
	}

	// If running on macOS and iOS device provisioning is enabled
	if config.Config.EnvConfig.OS == "darwin" && config.Config.EnvConfig.ProvideIOS {
		logger.ProviderLogger.LogInfo("provider_setup", "Provider runs on macOS and is set up to provide iOS devices")
		// Add a trailing slash to WDA repo folder if its missing
		// To avoid issues with the configuration
		logger.ProviderLogger.LogDebug("provider_setup", "Handling trailing slash of provided WebDriverAgent repo path if needed")
		if !strings.HasSuffix(config.Config.EnvConfig.WdaRepoPath, "/") {
			logger.ProviderLogger.LogDebug("provider_setup", "Provided WebDriverAgent repo path has no trailing slash, adding it")
			config.Config.EnvConfig.WdaRepoPath = fmt.Sprintf("%s/", config.Config.EnvConfig.WdaRepoPath)
		}

		// Check if the provided WebDriverAgent repo path exists
		logger.ProviderLogger.LogDebug("provider_setup", "Checking if provided WebDriverAgent repo path exists on the host")
		_, err := os.Stat(config.Config.EnvConfig.WdaRepoPath)
		if err != nil {
			log.Fatalf("`%s` does not exist, you need to provide valid path to the WebDriverAgent repo in the provider configuration", config.Config.EnvConfig.WdaRepoPath)
		}

		// Check if xcodebuild is available - Xcode and command line tools should be installed
		if !providerutil.XcodebuildAvailable() {
			log.Fatal("xcodebuild is not available, you need to set it up on the host as explained in the readme")
		}

		// Build the WebDriverAgent using xcodebuild from the provided repo path
		err = providerutil.BuildWebDriverAgent()
		if err != nil {
			log.Fatalf("Could not build WebDriverAgent for testing - %s", err)
		}
	}

	if config.Config.EnvConfig.ProvideIOS {
		// Check if the `go-ios` binary is available on PATH as explained in the setup readme
		if !providerutil.GoIOSAvailable() {
			log.Fatal("`go-ios` is not available, you need to set it up on the host as explained in the readme")
		}

		// If on Linux or Windows and iOS devices provision enabled check for WebDriverAgent.ipa/app
		if config.Config.EnvConfig.OS != "darwin" {
			logger.ProviderLogger.LogInfo(
				"provider_setup",
				"Provider runs on Linux/Windows and is set up to provide iOS devices, checking if prepared WebDriverAgent binary exists in the provider folder as explained in the readme")
			err := configureWebDriverBinary(providerFolder)
			if err != nil {
				log.Fatalf("You should put signed WebDriverAgent.ipa/app file in the provider folder `%s` as explained in the readme", providerFolder)
			}
		}
	}

	// If we want to provide Android devices check if adb is available on PATH
	if config.Config.EnvConfig.ProvideAndroid {
		if !providerutil.AdbAvailable() {
			logger.ProviderLogger.LogError("provider", "adb is not available, you need to set up the host as explained in the readme")
			fmt.Println("adb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}

		// Try to remove potentially hanging ports forwarded by adb
		providerutil.RemoveAdbForwardedPorts()
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
	err := r.Run(fmt.Sprintf(":%v", config.Config.EnvConfig.Port))
	if err != nil {
		return err
	}
	return fmt.Errorf("HTTP server stopped due to an unknown reason")
}

// Check for and set up selenium jar file for creating Appium grid nodes in config
func configureSeleniumSettings() {
	seleniumJarFile := ""
	err := filepath.Walk(fmt.Sprintf("%s/", config.Config.EnvConfig.ProviderFolder), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.Contains(info.Name(), "selenium") && filepath.Ext(path) == ".jar" {
			seleniumJarFile = info.Name()
			return nil
		}
		return nil
	})
	if err != nil {
		return
	}
	if seleniumJarFile == "" {
		log.Fatalf("You have enabled Selenium Grid connection but no Selenium jar file was found in `%s` folder, you need to set it up as explained in the readme", config.Config.EnvConfig.ProviderFolder)
	}
	config.Config.EnvConfig.SeleniumJarFile = seleniumJarFile
}

// Check for and set up WebDriverAgent.ipa/app binary in config
func configureWebDriverBinary(providerFolder string) error {
	// Check for WDA ipa, then WDA app availability
	ipaPath := fmt.Sprintf("%s/WebDriverAgent.ipa", providerFolder)
	_, err := os.Stat(ipaPath)
	if err != nil {
		appPath := fmt.Sprintf("%s/WebDriverAgent.app", providerFolder)
		_, err = os.Stat(appPath)
		if os.IsNotExist(err) {
			return err
		}
		config.Config.EnvConfig.WebDriverBinary = "WebDriverAgent.app"
	} else {
		config.Config.EnvConfig.WebDriverBinary = "WebDriverAgent.ipa"
	}
	return nil
}

// Periodically send current provider data updates to MongoDB
func updateProviderInDB() {
	ctx, cancel := context.WithCancel(db.MongoCtx())
	defer cancel()

	for {
		coll := db.MongoClient().Database("gads").Collection("providers")
		filter := bson.D{{Key: "nickname", Value: config.Config.EnvConfig.Nickname}}

		var providedDevices []models.Device
		for _, mapDevice := range devices.DeviceMap {
			providedDevices = append(providedDevices, *mapDevice)
		}
		sort.Sort(models.ByUDID(providedDevices))

		update := bson.M{
			"$set": bson.M{
				"last_updated":     time.Now().UnixMilli(),
				"provided_devices": providedDevices,
			},
		}
		opts := options.Update().SetUpsert(true)
		_, err := coll.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			logger.ProviderLogger.LogError("update_provider", fmt.Sprintf("Failed to upsert provider in DB - %s", err))
		}
		time.Sleep(1 * time.Second)
	}
}
