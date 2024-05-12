package device

import (
	"GADS/common/models"
	"GADS/common/util"
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var latestDevices []*models.Device

// Get the latest devices information from MongoDB each second
func GetLatestDBDevices() {
	// Access the database and collection
	collection := util.MongoClient().Database("gads").Collection("devices")
	latestDevices = []*models.Device{}

	for {
		cursor, err := collection.Find(context.Background(), bson.D{{}}, options.Find())
		if err != nil {
			log.WithFields(log.Fields{
				"event": "get_db_devices",
			}).Error(fmt.Sprintf("Could not get db cursor when trying to get latest device info from db - %s", err))
		}

		if err := cursor.All(context.Background(), &latestDevices); err != nil {
			log.WithFields(log.Fields{
				"event": "get_db_devices",
			}).Error(fmt.Sprintf("Could not get devices latest info from db cursor - %s", err))
		}

		if err := cursor.Err(); err != nil {
			log.WithFields(log.Fields{
				"event": "get_db_devices",
			}).Error(fmt.Sprintf("Encountered db cursor error - %s", err))
		}

		cursor.Close(context.TODO())

		time.Sleep(1 * time.Second)
	}
}

func GetDeviceByUDID(udid string) *models.Device {
	for _, device := range latestDevices {
		if device.UDID == udid {
			return device
		}
	}

	return nil
}
