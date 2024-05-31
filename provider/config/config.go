package config

import (
	"GADS/common/db"
	"GADS/common/models"
	"log"
)

var Config = &models.ConfigJsonData{}

func SetupConfig(nickname, folder string) {
	provider, err := db.GetProviderFromDB(nickname)
	if err != nil {
		log.Fatalf("Failed to gte provider data from DB - %s", err)
	}
	if provider.Nickname == "" {
		log.Fatal("Provider with this nickname is not registered in the DB")
	}
	provider.ProviderFolder = folder
	Config.EnvConfig = provider
}
