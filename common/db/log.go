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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	appiumLogDB   = "appium_logs"
	providerLogDB = "logs"
)

func (m *MongoStore) GetAppiumLogs(collectionName string, logLimit int) ([]models.AppiumLog, error) {
	coll := m.GetCollectionWithDB(appiumLogDB, collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "ts", Value: -1}})
	findOptions.SetLimit(int64(logLimit))

	return GetDocuments[models.AppiumLog](m.Ctx, coll, bson.D{{}}, findOptions)
}

func (m *MongoStore) GetProviderLogs(collectionName string, logLimit int) ([]models.ProviderLog, error) {
	coll := m.GetCollectionWithDB(providerLogDB, collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	findOptions.SetLimit(int64(logLimit))

	return GetDocuments[models.ProviderLog](m.Ctx, coll, bson.D{{}}, findOptions)
}

func (m *MongoStore) GetAppiumSessionLogs(collectionName, sessionID string) ([]models.AppiumLog, error) {
	coll := m.GetCollectionWithDB(appiumLogDB, collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "ts", Value: -1}})
	filter := bson.D{{"session_id", sessionID}}

	return GetDocuments[models.AppiumLog](m.Ctx, coll, filter, findOptions)
}
