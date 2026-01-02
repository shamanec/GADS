/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package db

import (
	"GADS/common/constants"
	"GADS/common/models"
	"fmt"

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

func (m *MongoStore) GetProviderDevices(providerNickname string) ([]models.Device, error) {
	coll := m.GetCollection("new_devices")
	filter := bson.M{"provider": providerNickname}
	return GetDocuments[models.Device](m.Ctx, coll, filter)
}

// This function is meant to update the already existing devices to the new stream type structure
func (m *MongoStore) EnsureDevicesHaveStreamType() error {
	dbDevices, err := m.GetDevices()
	if err != nil {
		return fmt.Errorf("EnsureDevicesHaveStreamType: Could not get devices from DB - %s", err)
	}

	for index, dbDevice := range dbDevices {
		// Check if the device does not have stream type set
		if dbDevice.StreamType == "" {
			fmt.Printf("Updating stream type for device `%s`\n", dbDevice.UDID)
			if dbDevice.OS == "android" {
				// Check if the device was previously set to use webrtc and set it to the currently used GetStream sdk WebRTC, else use MJPEG
				if dbDevice.UseWebRTCVideo {
					dbDevices[index].StreamType = constants.AndroidWebRTCGetStreamStreamType.ID
				} else {
					dbDevices[index].StreamType = constants.MJPEGStreamType.ID
				}
			} else if dbDevice.OS == "ios" {
				// For iOS set MJPEG
				dbDevices[index].StreamType = constants.MJPEGStreamType.ID
			}

			err = m.AddOrUpdateDevice(&dbDevices[index])
			if err != nil {
				fmt.Printf("Failed updating stream type for device `%s`\n", dbDevice.UDID)
			}
		} else {
			fmt.Printf("Device `%s` already has a stream type, not updating\n", dbDevice.UDID)
		}
	}

	return nil
}
