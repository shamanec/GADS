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

func (m *MongoStore) UpdateMinioConfig(config models.MinioConfig) error {
	globalSettings := models.GlobalSettings{
		Type:        "minio-config",
		Settings:    config,
		LastUpdated: time.Now(),
	}
	coll := m.GetCollection("global_settings")
	filter := bson.D{{Key: "type", Value: "minio-config"}}

	return UpsertDocument[models.GlobalSettings](m.Ctx, coll, filter, globalSettings)
}

func (m *MongoStore) GetMinioConfig() (models.MinioConfig, error) {
	var minioConfig models.MinioConfig
	coll := m.GetCollection("global_settings")
	filter := bson.D{{Key: "type", Value: "minio-config"}}

	globalSettings, err := GetDocument[models.GlobalSettings](m.Ctx, coll, filter)
	if err == mongo.ErrNoDocuments {
		// Return default config if not found
		minioConfig = models.MinioConfig{
			Endpoint:        "localhost:9000",
			AccessKeyID:     "minioadmin",
			SecretAccessKey: "minioadmin",
			UseSSL:          false,
			Enabled:         false,
		}

		err = m.UpdateMinioConfig(minioConfig)
		if err != nil {
			return minioConfig, err
		}
	} else if err != nil {
		return minioConfig, err
	} else {
		settingsBytes, err := bson.Marshal(globalSettings.Settings)
		if err != nil {
			return minioConfig, fmt.Errorf("failed to marshal settings: %v", err)
		}

		err = bson.Unmarshal(settingsBytes, &minioConfig)
		if err != nil {
			return minioConfig, fmt.Errorf("failed to unmarshal settings: %v", err)
		}
	}

	return minioConfig, nil
}