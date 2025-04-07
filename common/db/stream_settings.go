package db

import (
	"GADS/common/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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
