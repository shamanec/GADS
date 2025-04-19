package devices

import (
	"context"
	"log"

	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"

	"go.mongodb.org/mongo-driver/bson"
)

func getDBProviderDevices() map[string]*models.Device {
	ctx, cancel := context.WithCancel(db.MongoCtx())
	defer cancel()

	var deviceDataMap = make(map[string]*models.Device)

	filter := bson.M{"provider": config.ProviderConfig.Nickname}

	collection := db.MongoClient().Database("gads").Collection("new_devices")

	cursor, err := collection.Find(ctx, filter, nil)
	if err != nil {
		return nil
	}

	var deviceData []*models.Device

	if err := cursor.All(context.Background(), &deviceData); err != nil {
		return nil
	}

	if err := cursor.Err(); err != nil {
		return nil
	}

	cursor.Close(context.TODO())

	for _, dbDevice := range deviceData {
		// Ensure that devices are associated with the Default workspace if not specified
		if dbDevice.WorkspaceID == "" {
			if defaultWorkspace, err := db.GetDefaultWorkspace(); err == nil {
				dbDevice.WorkspaceID = defaultWorkspace.ID
				// Persist the workspace association in the database
				err := db.UpsertDeviceDB(dbDevice)
				if err != nil {
					log.Printf("Failed to associate device %s with default workspace - %s", dbDevice.UDID, err)
				}
			} else {
				return nil
			}
		}
		deviceDataMap[dbDevice.UDID] = dbDevice
	}

	return deviceDataMap
}
