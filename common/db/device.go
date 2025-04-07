package db

import (
	"GADS/common/models"

	"go.mongodb.org/mongo-driver/bson"
)

func (m *MongoStore) GetDevices() ([]models.Device, error) {
	coll := m.GetCollection("new_devices")
	return GetDocuments[models.Device](m.Ctx, coll, bson.D{{}})
}

func (m *MongoStore) GetDeviceStreamSettings(udid string) (models.DeviceStreamSettings, error) {
	coll := m.GetCollection("device_stream_settings")
	filter := bson.D{{Key: "udid", Value: udid}}
	return GetDocument[models.DeviceStreamSettings](m.Ctx, coll, filter)
}

func (m *MongoStore) DeleteDevice(udid string) error {
	coll := m.GetCollection("new_devices")
	filter := bson.M{"udid": udid}
	return DeleteDocument(m.Ctx, coll, filter)
}

func (m *MongoStore) AddOrUpdateDevice(device *models.Device) error {
	coll := m.GetCollection("new_devices")
	filter := bson.D{{Key: "udid", Value: device.UDID}}
	return UpsertDocument[models.Device](m.Ctx, coll, filter, *device)
}
