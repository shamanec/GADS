package device

import (
	"GADS/util"
	"context"
	"time"

	log "github.com/sirupsen/logrus"
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
			log.Fatal(err)
		}

		if err := cursor.All(util.MongoCtx(), &latestDevices); err != nil {
			log.Fatal(err)
		}
		if err := cursor.Err(); err != nil {
			log.Fatal(err)
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
