package config

import (
	"GADS/common/models"
	"GADS/provider/db"
)

var Config = &models.ConfigJsonData{}

func SetupConfig(nickname, folder string) {
	provider, err := db.GetProviderFromDB(nickname)
	if err != nil {
		panic("Could not get provider data from DB")
	}
	if provider.Nickname == "" {
		panic("Provider with this nickname is not registered in the DB")
	}
	provider.ProviderFolder = folder
	Config.EnvConfig = provider
}
