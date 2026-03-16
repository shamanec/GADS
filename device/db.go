/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package device

import (
	"fmt"

	"GADS/common/db"

	"go.mongodb.org/mongo-driver/bson"
)

const deviceCollection = "new_devices"

// GetDevices returns all devices from the database.
func GetDevices() ([]DeviceInfo, error) {
	coll := db.GlobalMongoStore.GetCollection(deviceCollection)
	return db.GetDocuments[DeviceInfo](db.GlobalMongoStore.Ctx, coll, bson.D{{}})
}

// GetProviderDevices returns all devices assigned to the given provider.
func GetProviderDevices(providerNickname string) ([]DeviceInfo, error) {
	coll := db.GlobalMongoStore.GetCollection(deviceCollection)
	filter := bson.M{"provider": providerNickname}
	return db.GetDocuments[DeviceInfo](db.GlobalMongoStore.Ctx, coll, filter)
}

// AddOrUpdateDevice upserts a DeviceInfo record, keyed by UDID.
func AddOrUpdateDevice(info *DeviceInfo) error {
	coll := db.GlobalMongoStore.GetCollection(deviceCollection)
	filter := bson.D{{Key: "udid", Value: info.UDID}}
	return db.UpsertDocument(db.GlobalMongoStore.Ctx, coll, filter, *info)
}

// DeleteDevice removes a device by UDID.
func DeleteDevice(udid string) error {
	coll := db.GlobalMongoStore.GetCollection(deviceCollection)
	filter := bson.M{"udid": udid}
	return db.DeleteDocument(db.GlobalMongoStore.Ctx, coll, filter)
}

// EnsureDevicesHaveStreamType is a one-time migration that sets stream_type on
// old device records that only have the deprecated use_webrtc_video flag.
// It uses raw BSON to check the legacy field without needing it on DeviceInfo.
func EnsureDevicesHaveStreamType() error {
	devices, err := GetDevices()
	if err != nil {
		return fmt.Errorf("EnsureDevicesHaveStreamType: Could not get devices from DB - %s", err)
	}

	coll := db.GlobalMongoStore.GetCollection(deviceCollection)

	for _, dev := range devices {
		if dev.StreamType != "" {
			fmt.Printf("Device `%s` already has a stream type, not updating\n", dev.UDID)
			continue
		}

		fmt.Printf("Updating stream type for device `%s`\n", dev.UDID)

		// Read the raw document to check the legacy use_webrtc_video field.
		var raw bson.M
		err := coll.FindOne(db.GlobalMongoStore.Ctx, bson.M{"udid": dev.UDID}).Decode(&raw)
		if err != nil {
			fmt.Printf("Failed reading raw device `%s`: %v\n", dev.UDID, err)
			continue
		}

		useWebRTC, _ := raw["use_webrtc_video"].(bool)

		var streamType StreamingType
		switch dev.OS {
		case "android":
			if useWebRTC {
				streamType = AndroidWebRTCGetStreamStreamTypeID
			} else {
				streamType = MJPEGStreamTypeID
			}
		case "ios":
			streamType = MJPEGStreamTypeID
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
