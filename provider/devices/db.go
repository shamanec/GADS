/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package devices

import (
	"log"

	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"
)

func getDBProviderDevices() map[string]*models.Device {
	var deviceDataMap = make(map[string]*models.Device)

	deviceData, err := db.GlobalMongoStore.GetProviderDevices(config.ProviderConfig.Nickname)
	if err != nil {
		return nil
	}

	for _, dbDevice := range deviceData {
		// Ensure that devices are associated with the Default workspace if not specified
		if dbDevice.WorkspaceID == "" {
			if defaultWorkspace, err := db.GlobalMongoStore.GetDefaultWorkspace(); err == nil {
				dbDevice.WorkspaceID = defaultWorkspace.ID
				// Persist the workspace association in the database
				err := db.GlobalMongoStore.AddOrUpdateDevice(&dbDevice)
				if err != nil {
					log.Printf("Failed to associate device %s with default workspace - %s", dbDevice.UDID, err)
				}
			} else {
				return nil
			}
		}
		deviceDataMap[dbDevice.UDID] = &dbDevice
	}

	return deviceDataMap
}
