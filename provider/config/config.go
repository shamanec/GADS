package config

import (
	"GADS/common/db"
	"GADS/common/models"
	"fmt"
	"log"
	"strings"
)

var ProviderConfig = &models.Provider{}

func SetupConfig(nickname, folder, hubAddress string) {
	provider, err := db.GlobalMongoStore.GetProvider(nickname)
	if err != nil {
		log.Fatalf("Failed to get provider data from DB - %s", err)
	}
	if provider.Nickname == "" {
		log.Fatal("Provider with this nickname is not registered in the DB")
	}
	provider.ProviderFolder = folder
	provider.HubAddress = hubAddress
	if !strings.HasSuffix(provider.WdaBundleID, ".xctrunner") {
		provider.WdaBundleID = fmt.Sprintf("%s.xctrunner", provider.WdaBundleID)
	}

	ProviderConfig = &provider
}

func SetupSeleniumJar() error {
	return db.GlobalMongoStore.DownloadFile("selenium.jar", ProviderConfig.ProviderFolder)
}

func SetupIOSSupervisionProfileFile() error {
	return db.GlobalMongoStore.DownloadFile("supervision.p12", ProviderConfig.ProviderFolder)
}

func SetupWebDriverAgentFile() error {
	return db.GlobalMongoStore.DownloadFile("WebDriverAgent.ipa", ProviderConfig.ProviderFolder)
}
