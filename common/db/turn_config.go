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
	"GADS/common/models"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (m *MongoStore) UpdateTURNConfig(config models.TURNConfig) error {
	globalSettings := models.GlobalSettings{
		Type:        "turn-config",
		Settings:    config,
		LastUpdated: time.Now(),
	}
	coll := m.GetCollection("global_settings")
	filter := bson.D{{Key: "type", Value: "turn-config"}}

	return UpsertDocument[models.GlobalSettings](m.Ctx, coll, filter, globalSettings)
}

func (m *MongoStore) GetTURNConfig() (models.TURNConfig, error) {
	var turnConfig models.TURNConfig
	coll := m.GetCollection("global_settings")
	filter := bson.D{{Key: "type", Value: "turn-config"}}

	globalSettings, err := GetDocument[models.GlobalSettings](m.Ctx, coll, filter)
	if err == mongo.ErrNoDocuments {
		// Return default config if not found (disabled by default)
		turnConfig = models.TURNConfig{
			Server:       "",
			Port:         3478,
			SharedSecret: "",
			TTL:          3600,
			Enabled:      false,
		}
		return turnConfig, nil
	} else if err != nil {
		return turnConfig, err
	}

	settingsBytes, err := bson.Marshal(globalSettings.Settings)
	if err != nil {
		return turnConfig, fmt.Errorf("failed to marshal settings: %v", err)
	}

	err = bson.Unmarshal(settingsBytes, &turnConfig)
	if err != nil {
		return turnConfig, fmt.Errorf("failed to unmarshal settings: %v", err)
	}

	return turnConfig, nil
}
