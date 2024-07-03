package devices

import (
	"context"
	"fmt"
	"time"

	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Update all devices data in Mongo each second
func updateDevicesMongo() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		upsertDevicesMongo()
	}
}

// Upsert all devices data in Mongo
func upsertDevicesMongo() {
	ctx, cancel := context.WithCancel(db.MongoCtx())
	defer cancel()

	for _, device := range DBDeviceMap {
		filter := bson.M{"udid": device.UDID}
		if device.Connected {
			device.LastUpdatedTimestamp = time.Now().UnixMilli()
		}

		update := bson.M{
			"$set": device,
		}
		opts := options.Update().SetUpsert(true)

		coll := db.MongoClient().Database("gads").Collection("new_devices")

		_, err := coll.UpdateOne(ctx, filter, update, opts)

		if err != nil {
			logger.ProviderLogger.LogError("provider", fmt.Sprintf("upsertDevicesMongo: Failed upserting device data in Mongo - %s", err))
		}
	}
}

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
