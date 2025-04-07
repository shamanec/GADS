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
	"log"
	"os"
	"runtime"
	"time"

	"github.com/spf13/pflag"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func StartProvider(flags *pflag.FlagSet) {
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

	// Create a connection to Mongo
	db.InitMongoClient(mongoDb)
	defer db.MongoCtxCancel()

	db.InitMongo("mongodb://localhost:27017/?keepAlive=true", "gads")
	defer db.GlobalMongoStore.Close()

	// Set up the provider configuration
	config.SetupConfig(nickname, providerFolder, hubAddress)
	config.ProviderConfig.OS = runtime.GOOS
	// Defer closing the Mongo connection on provider stopped
	defer db.CloseMongoConn()

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
		err := db.AddWorkspace(&defaultWorkspace)
		if err != nil {
			log.Fatalf("Failed to create default workspace - %s", err)
		}
		logger.ProviderLogger.LogInfo("provider_setup", "Created default workspace")
	}

	logger.ProviderLogger.LogInfo("provider_setup", "Checking if Appium is installed and available on the host")
	if !providerutil.AppiumAvailable() {
		log.Fatal("Appium is not available, set it up on the host as explained in the readme")
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

	if config.ProviderConfig.ProvideAndroid {
		err = config.SetupGADSWebRTCAndroidApkFile()
		if err != nil {
			logger.ProviderLogger.LogWarn("provider_setup", "There is no GADS Android WebRTC apk uploaded via the Admin UI!!!")
		}
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
	ctx, cancel := context.WithCancel(db.MongoCtx())
	defer cancel()

	for {
		coll := db.MongoClient().Database("gads").Collection("providers")
		filter := bson.D{{Key: "nickname", Value: config.ProviderConfig.Nickname}}

		update := bson.M{
			"$set": bson.M{
				"last_updated": time.Now().UnixMilli(),
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
