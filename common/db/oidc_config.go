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

func (m *MongoStore) UpdateOIDCConfig(config models.OIDCConfig) error {
	globalSettings := models.GlobalSettings{
		Type:        "oidc-config",
		Settings:    config,
		LastUpdated: time.Now(),
	}
	coll := m.GetCollection("global_settings")
	filter := bson.D{{Key: "type", Value: "oidc-config"}}

	return UpsertDocument[models.GlobalSettings](m.Ctx, coll, filter, globalSettings)
}

// GetOIDCConfig returns the stored OIDC configuration, or a disabled one if there is none yet
func (m *MongoStore) GetOIDCConfig() (models.OIDCConfig, error) {
	var oidcConfig models.OIDCConfig
	coll := m.GetCollection("global_settings")
	filter := bson.D{{Key: "type", Value: "oidc-config"}}

	globalSettings, err := GetDocument[models.GlobalSettings](m.Ctx, coll, filter)
	if err == mongo.ErrNoDocuments {
		return models.OIDCConfig{GroupsClaim: "groups"}, nil
	}
	if err != nil {
		return oidcConfig, err
	}

	settingsBytes, err := bson.Marshal(globalSettings.Settings)
	if err != nil {
		return oidcConfig, fmt.Errorf("failed to marshal settings: %w", err)
	}

	err = bson.Unmarshal(settingsBytes, &oidcConfig)
	if err != nil {
		return oidcConfig, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return oidcConfig, nil
}
