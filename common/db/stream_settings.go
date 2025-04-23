package db

import (
	"GADS/common/models"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (m *MongoStore) UpdateGlobalStreamSettings(settings models.StreamSettings) error {
	globalSettings := models.GlobalSettings{
		Type:        "stream-settings",
		Settings:    settings,
		LastUpdated: time.Now(),
	}
	coll := m.GetCollection("global_settings")
	filter := bson.D{{Key: "type", Value: "stream-settings"}}

	return UpsertDocument[models.GlobalSettings](m.Ctx, coll, filter, globalSettings)
}

func (m *MongoStore) UpdateDeviceStreamSettings(udid string, settings models.DeviceStreamSettings) error {
	coll := m.GetCollection("device_stream_settings")
	filter := bson.D{{Key: "udid", Value: udid}}

	return UpsertDocument[models.DeviceStreamSettings](m.Ctx, coll, filter, settings)
}

func (m *MongoStore) GetGlobalStreamSettings() (models.StreamSettings, error) {
	var streamSettings models.StreamSettings
	coll := m.GetCollection("global_settings")
	filter := bson.D{{Key: "type", Value: "stream-settings"}}

	globalSettings, err := GetDocument[models.GlobalSettings](m.Ctx, coll, filter)
	if err == mongo.ErrNoDocuments {
		streamSettings = models.StreamSettings{
			TargetFPS:            15,
			JpegQuality:          75,
			ScalingFactorAndroid: 50,
			ScalingFactoriOS:     50,
		}

		err = m.UpdateGlobalStreamSettings(streamSettings)
		if err != nil {
			return streamSettings, err
		}
	} else if err != nil {
		return streamSettings, err
	}

	settingsBytes, err := bson.Marshal(globalSettings.Settings)
	if err != nil {
		return streamSettings, fmt.Errorf("failed to marshal settings: %v", err)
	}

	err = bson.Unmarshal(settingsBytes, &streamSettings)
	if err != nil {
		return streamSettings, fmt.Errorf("failed to unmarshal settings: %v", err)
	}

	return streamSettings, nil
}
