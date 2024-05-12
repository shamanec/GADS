package devices

import (
	"context"
	"fmt"
	"time"

	"GADS/provider/db"
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

	for _, device := range DeviceMap {
		filter := bson.M{"udid": device.UDID}
		if device.Connected {
			device.LastUpdatedTimestamp = time.Now().UnixMilli()
		}

		update := bson.M{
			"$set": device,
		}
		opts := options.Update().SetUpsert(true)

		_, err := db.MongoClient().Database("gads").Collection("devices").UpdateOne(ctx, filter, update, opts)

		if err != nil {
			logger.ProviderLogger.LogError("provider", fmt.Sprintf("upsertDevicesMongo: Failed upserting device data in Mongo - %s", err))
		}
	}
}
