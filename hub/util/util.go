package util

import (
	"GADS/common/db"
	"GADS/common/models"
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strconv"
	"time"
)

var ConfigData *models.HubConfig

func CalculateCanvasDimensions(device *models.Device) (canvasWidth string, canvasHeight string) {
	// Get the width and height provided
	widthString := device.ScreenWidth
	heightString := device.ScreenHeight

	// Convert them to ints
	width, _ := strconv.Atoi(widthString)
	height, _ := strconv.Atoi(heightString)

	screen_ratio := float64(width) / float64(height)

	canvasHeight = "850"
	canvasWidth = fmt.Sprintf("%f", 850*screen_ratio)

	return
}

var LatestDevices []*models.Device

// Get the latest devices information from MongoDB each second
func GetLatestDBDevices() {
	// Access the database and collection
	collection := db.MongoClient().Database("gads").Collection("devices")
	LatestDevices = []*models.Device{}

	for {
		cursor, err := collection.Find(context.Background(), bson.D{{}}, options.Find())
		if err != nil {
			log.WithFields(log.Fields{
				"event": "get_db_devices",
			}).Error(fmt.Sprintf("Could not get db cursor when trying to get latest device info from db - %s", err))
		}

		if err := cursor.All(context.Background(), &LatestDevices); err != nil {
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
	for _, device := range LatestDevices {
		if device.UDID == udid {
			return device
		}
	}

	return nil
}
