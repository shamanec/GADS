package db

import (
	"GADS/common/models"
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

func (m *MongoStore) GetDevices(ctx context.Context) ([]models.Device, error) {
	coll := m.Collection("new_devices")
	return GetDocuments[models.Device](ctx, coll, bson.D{{}})
}

func (m *MongoStore) GetDeviceStreamSettings(ctx context.Context, udid string) (models.DeviceStreamSettings, error) {
	coll := m.Collection("device_stream_settings")
	filter := bson.D{{Key: "udid", Value: udid}}
	return GetDocument[models.DeviceStreamSettings](ctx, coll, filter)
}
