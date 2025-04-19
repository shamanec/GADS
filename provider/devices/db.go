package devices

import (
	"log"

	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"
)

func getDBProviderDevices() map[string]*models.Device {
	var deviceDataMap = make(map[string]*models.Device)

	deviceData, err := db.GlobalMongoStore.GetProviderDevices(config.ProviderConfig.Nickname)
	if err != nil {
		return nil
	}

	for _, dbDevice := range deviceData {
		// Ensure that devices are associated with the Default workspace if not specified
		if dbDevice.WorkspaceID == "" {
			if defaultWorkspace, err := db.GlobalMongoStore.GetDefaultWorkspace(); err == nil {
				dbDevice.WorkspaceID = defaultWorkspace.ID
				// Persist the workspace association in the database
				err := db.GlobalMongoStore.AddOrUpdateDevice(&dbDevice)
				if err != nil {
					log.Printf("Failed to associate device %s with default workspace - %s", dbDevice.UDID, err)
				}
			} else {
				return nil
			}
		}
		deviceDataMap[dbDevice.UDID] = &dbDevice
	}

	return deviceDataMap
}
