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
	"fmt"

	"GADS/common/models"

	"go.mongodb.org/mongo-driver/bson"
)

const deviceCollection = "new_devices"

// GetDevices returns all devices from the database.
func GetDevices() ([]models.DeviceInfo, error) {
	coll := GlobalMongoStore.GetCollection(deviceCollection)
	return GetDocuments[models.DeviceInfo](GlobalMongoStore.Ctx, coll, bson.D{{}})
}

// GetProviderDevices returns all devices assigned to the given provider.
func GetProviderDevices(providerNickname string) ([]models.DeviceInfo, error) {
	coll := GlobalMongoStore.GetCollection(deviceCollection)
	filter := bson.M{"provider": providerNickname}
	return GetDocuments[models.DeviceInfo](GlobalMongoStore.Ctx, coll, filter)
}

// AddOrUpdateDevice upserts a DeviceInfo record, keyed by UDID.
func AddOrUpdateDevice(info *models.DeviceInfo) error {
	coll := GlobalMongoStore.GetCollection(deviceCollection)
	filter := bson.D{{Key: "udid", Value: info.UDID}}
	return UpsertDocument(GlobalMongoStore.Ctx, coll, filter, *info)
}

// DeleteDevice removes a device by UDID.
func DeleteDevice(udid string) error {
	coll := GlobalMongoStore.GetCollection(deviceCollection)
	filter := bson.M{"udid": udid}
	return DeleteDocument(GlobalMongoStore.Ctx, coll, filter)
}

// EnsureDevicesHaveStreamType is a one-time migration that sets stream_type on
// old device records that only have the deprecated use_webrtc_video flag.
// It uses raw BSON to check the legacy field without needing it on DeviceInfo.
func EnsureDevicesHaveStreamType() error {
	devices, err := GetDevices()
	if err != nil {
		return fmt.Errorf("EnsureDevicesHaveStreamType: Could not get devices from DB - %s", err)
	}

	coll := GlobalMongoStore.GetCollection(deviceCollection)

	for _, dev := range devices {
		if dev.StreamType != "" {
			fmt.Printf("Device `%s` already has a stream type, not updating\n", dev.UDID)
			continue
		}

		fmt.Printf("Updating stream type for device `%s`\n", dev.UDID)

		// Read the raw document to check the legacy use_webrtc_video field.
		var raw bson.M
		err := coll.FindOne(GlobalMongoStore.Ctx, bson.M{"udid": dev.UDID}).Decode(&raw)
		if err != nil {
			fmt.Printf("Failed reading raw device `%s`: %v\n", dev.UDID, err)
			continue
		}

		useWebRTC, _ := raw["use_webrtc_video"].(bool)

		var streamType models.StreamingType
		switch dev.OS {
		case "android":
			if useWebRTC {
				streamType = models.AndroidWebRTCGetStreamStreamTypeID
			} else {
				streamType = models.MJPEGStreamTypeID
			}
		case "ios":
			streamType = models.MJPEGStreamTypeID
		default:
			continue
		}

		dev.StreamType = streamType
		if uErr := AddOrUpdateDevice(&dev); uErr != nil {
			fmt.Printf("Failed updating stream type for device `%s`: %v\n", dev.UDID, uErr)
		}
	}

	return nil
}
