package devices

import (
	"context"

	"GADS/common/db"
	"GADS/common/models"
	"GADS/common/utils"
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
			dbDevice.WorkspaceID = utils.FormatWorkspaceID("Default")
		}
		deviceDataMap[dbDevice.UDID] = dbDevice
	}

	return deviceDataMap
}
