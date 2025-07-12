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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// GetOrCreateDefaultTenant returns the default tenant string for GADS
// If it doesn't exist, generates a secure unique tenant and saves it
func (m *MongoStore) GetOrCreateDefaultTenant() (string, error) {
	coll := m.GetCollection("global_settings")
	filter := bson.D{{Key: "type", Value: "default-tenant"}}

	globalSettings, err := GetDocument[models.GlobalSettings](m.Ctx, coll, filter)
	if err == mongo.ErrNoDocuments {
		// Generate secure tenant string (32 bytes = 256 bits of entropy)
		bytes := make([]byte, 32)
		_, err := rand.Read(bytes)
		if err != nil {
			return "", fmt.Errorf("failed to generate tenant: %w", err)
		}
		tenant := base64.URLEncoding.EncodeToString(bytes)

		// Save to database
		settings := models.GlobalSettings{
			Type:        "default-tenant",
			Settings:    tenant,
			LastUpdated: time.Now(),
		}
		
		err = UpsertDocument(m.Ctx, coll, filter, settings)
		if err != nil {
			return "", fmt.Errorf("failed to save default tenant: %w", err)
		}
		
		return tenant, nil
	} else if err != nil {
		return "", fmt.Errorf("failed to get default tenant: %w", err)
	}

	// Extract tenant string from settings
	tenant, ok := globalSettings.Settings.(string)
	if !ok {
		return "", fmt.Errorf("invalid tenant format in database")
	}

	return tenant, nil
}