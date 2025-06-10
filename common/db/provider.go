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
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func (m *MongoStore) GetProvider(providerNickname string) (models.Provider, error) {
	coll := m.GetCollection("providers")
	filter := bson.D{{Key: "nickname", Value: providerNickname}}
	return GetDocument[models.Provider](m.Ctx, coll, filter)
}

func (m *MongoStore) GetAllProviders() ([]models.Provider, error) {
	coll := m.GetCollection("providers")
	return GetDocuments[models.Provider](m.Ctx, coll, bson.D{{}})
}

func (m *MongoStore) AddOrUpdateProvider(provider models.Provider) error {
	coll := m.GetCollection("providers")
	filter := bson.D{{Key: "nickname", Value: provider.Nickname}}
	return UpsertDocument[models.Provider](m.Ctx, coll, filter, provider)
}

func (m *MongoStore) DeleteProvider(nickname string) error {
	coll := m.GetCollection("providers")
	filter := bson.M{"nickname": nickname}
	return DeleteDocument(m.Ctx, coll, filter)
}

func (m *MongoStore) UpdateProviderTimestamp(nickname string) error {
	coll := m.GetCollection("providers")
	filter := bson.M{"nickname": nickname}
	updates := bson.M{
		"last_updated": time.Now().UnixMilli(),
	}
	return PartialDocumentUpdate(m.Ctx, coll, filter, updates)
}

// This is a temporary function that will update all current provider configurations that do not have the new `setup_appium_servers` property.
// It will set it to true by default so we do not break the setup for people that already have it from a previous version
func (m *MongoStore) InitializeProviderSetupAppiumServers() (int64, error) {
	coll := m.GetCollection("providers")

	filter := bson.M{
		"setup_appium_servers": bson.M{"$exists": false},
	}

	update := bson.M{
		"$set": bson.M{"setup_appium_servers": true},
	}

	updateResult, err := coll.UpdateMany(m.Ctx, filter, update)
	if err != nil {
		return 0, err
	}

	return updateResult.ModifiedCount, nil
}
