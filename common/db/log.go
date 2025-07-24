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
	appiumLogDB   = "appium_logs_new"
	providerLogDB = "logs"
)

func (m *MongoStore) GetAppiumLogs(collectionName string, logLimit int) ([]models.AppiumPluginLog, error) {
	coll := m.GetCollectionWithDB(appiumLogDB, collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{
		{Key: "timestamp", Value: -1},
		{Key: "sequenceNumber", Value: -1},
	})
	findOptions.SetLimit(int64(logLimit))

	return GetDocuments[models.AppiumPluginLog](m.Ctx, coll, bson.D{{}}, findOptions)
}

func (m *MongoStore) GetProviderLogs(collectionName string, logLimit int) ([]models.ProviderLog, error) {
	coll := m.GetCollectionWithDB(providerLogDB, collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	findOptions.SetLimit(int64(logLimit))

	return GetDocuments[models.ProviderLog](m.Ctx, coll, bson.D{{}}, findOptions)
}

func (m *MongoStore) GetAppiumSessionLogs(collectionName, sessionID string) ([]models.AppiumPluginLog, error) {
	coll := m.GetCollectionWithDB(appiumLogDB, collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "ts", Value: -1}})
	filter := bson.D{{Key: "session_id", Value: sessionID}}

	return GetDocuments[models.AppiumPluginLog](m.Ctx, coll, filter, findOptions)
}

func (m *MongoStore) AddAppiumLog(collectionName string, log models.AppiumPluginLog) error {
	coll := m.GetCollectionWithDB(appiumLogDB, collectionName)

	return InsertDocument[models.AppiumPluginLog](m.Ctx, coll, log)
}
