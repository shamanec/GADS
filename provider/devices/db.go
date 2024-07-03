package devices

import (
	"context"

	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"

	"go.mongodb.org/mongo-driver/bson"
)

func getDBProviderDevices() map[string]*models.Device {
	ctx, cancel := context.WithCancel(db.MongoCtx())
	defer cancel()

	var deviceDataMap = make(map[string]*models.Device)

	filter := bson.M{"provider": config.Config.EnvConfig.Nickname}

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
		deviceDataMap[dbDevice.UDID] = dbDevice
	}

	return deviceDataMap
}
