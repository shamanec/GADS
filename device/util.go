package device

import (
	"GADS/util"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var latestDevices []Device

// Get the latest devices information from MongoDB each second
func GetLatestDBDevices() {
	// Access the database and collection
	collection := util.MongoClient().Database("gads").Collection("devices")

	for {
		cursor, err := collection.Find(context.Background(), bson.D{{}}, options.Find())
		if err != nil {
			util.ErrorLog("gads-ui", "get_devices", fmt.Sprintf("Could not get db cursor when trying to get latest device info from db - %s", err))
		}

		if err := cursor.All(util.MongoCtx(), &latestDevices); err != nil {
			util.ErrorLog("gads-ui", "get_devices", fmt.Sprintf("Could not get devices latest info from db cursor - %s", err))
		}

		if err := cursor.Err(); err != nil {
			util.ErrorLog("gads-ui", "get_devices", fmt.Sprintf("Encountered db cursor error - %s", err))
		}

		cursor.Close(util.MongoCtx())

		time.Sleep(1 * time.Second)
	}
}

func GetDeviceByUDID(udid string) *Device {
	for _, device := range latestDevices {
		if device.UDID == udid {
			return &device
		}
	}

	return nil
}
