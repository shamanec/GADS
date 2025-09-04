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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	appiumLogDB         = "appium_logs_new"
	appiumSessionLogsDB = "appium_session_logs"
	providerLogDB       = "logs"
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

func (m *MongoStore) AddAppiumLog(collectionName string, log models.AppiumPluginLog) error {
	coll := m.GetCollectionWithDB(appiumLogDB, collectionName)

	return InsertDocument[models.AppiumPluginLog](m.Ctx, coll, log)
}

func (m *MongoStore) AddAppiumSessionLog(collectionName string, log models.AppiumPluginSessionLog) error {
	coll := m.GetCollectionWithDB(appiumSessionLogsDB, collectionName)

	return InsertDocument[models.AppiumPluginSessionLog](m.Ctx, coll, log)
}

func (m *MongoStore) GetBuildReports(tenant string, limit int64) ([]models.BuildReport, error) {
	coll := m.GetCollectionWithDB(appiumSessionLogsDB, tenant)

	pipeline := mongo.Pipeline{
		// Match by tenant and only records with build_id
		{{Key: "$match", Value: bson.D{
			{Key: "tenant", Value: tenant},
			{Key: "build_id", Value: bson.D{{Key: "$ne", Value: ""}}},
		}}},

		// Group by build_id
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$build_id"},
			{Key: "build_id", Value: bson.D{{Key: "$first", Value: "$build_id"}}},
			{Key: "session_ids", Value: bson.D{{Key: "$addToSet", Value: "$session_id"}}},
			{Key: "test_names", Value: bson.D{{Key: "$addToSet", Value: "$test_name"}}},
			{Key: "device_names", Value: bson.D{{Key: "$addToSet", Value: "$device_name"}}},
			{Key: "first_action", Value: bson.D{{Key: "$min", Value: "$timestamp"}}},
			{Key: "last_action", Value: bson.D{{Key: "$max", Value: "$timestamp"}}},
		}}},

		// Add session count
		{{Key: "$addFields", Value: bson.D{
			{Key: "session_count", Value: bson.D{{Key: "$size", Value: "$session_ids"}}},
		}}},

		// Sort by latest action timestamp (newest first)
		{{Key: "$sort", Value: bson.D{{Key: "last_action", Value: -1}}}},

		// Limit results
		{{Key: "$limit", Value: limit}},

		// Clean up _id field
		{{Key: "$project", Value: bson.D{{Key: "_id", Value: 0}}}},
	}

	cursor, err := coll.Aggregate(m.Ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate build reports: %w", err)
	}
	defer cursor.Close(m.Ctx)

	var buildReports []models.BuildReport
	if err = cursor.All(m.Ctx, &buildReports); err != nil {
		return nil, fmt.Errorf("failed to decode build reports: %w", err)
	}

	return buildReports, nil
}

func (m *MongoStore) GetBuildSessions(tenant string, buildID string) ([]models.SessionReport, error) {
	coll := m.GetCollectionWithDB(appiumSessionLogsDB, tenant)

	pipeline := mongo.Pipeline{
		// Match by tenant and build_id
		{{Key: "$match", Value: bson.D{
			{Key: "tenant", Value: tenant},
			{Key: "build_id", Value: buildID},
		}}},

		// Group by session_id to get session-level data
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$session_id"},
			{Key: "session_id", Value: bson.D{{Key: "$first", Value: "$session_id"}}},
			{Key: "test_name", Value: bson.D{{Key: "$first", Value: "$test_name"}}},
			{Key: "device_name", Value: bson.D{{Key: "$first", Value: "$device_name"}}},
			{Key: "device_udid", Value: bson.D{{Key: "$first", Value: "$udid"}}},
			{Key: "platform_name", Value: bson.D{{Key: "$first", Value: "$platform_name"}}},
			{Key: "log_count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "first_action", Value: bson.D{{Key: "$min", Value: "$timestamp"}}},
			{Key: "last_action", Value: bson.D{{Key: "$max", Value: "$timestamp"}}},
			{Key: "failed_actions", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: []interface{}{
					bson.D{{Key: "$eq", Value: []interface{}{"$success", false}}},
					1, 0,
				}},
			}}}},
		}}},

		// Sort by first action timestamp (newest first)
		{{Key: "$sort", Value: bson.D{{Key: "first_action", Value: -1}}}},

		// Clean up _id field
		{{Key: "$project", Value: bson.D{{Key: "_id", Value: 0}}}},
	}

	cursor, err := coll.Aggregate(m.Ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate session reports: %w", err)
	}
	defer cursor.Close(m.Ctx)

	var sessionReports []models.SessionReport
	if err = cursor.All(m.Ctx, &sessionReports); err != nil {
		return nil, fmt.Errorf("failed to decode session reports: %w", err)
	}

	return sessionReports, nil
}

func (m *MongoStore) GetSessionLogs(tenant string, sessionID string) ([]models.SessionActionLog, error) {
	coll := m.GetCollectionWithDB(appiumSessionLogsDB, tenant)

	filter := bson.D{
		{Key: "tenant", Value: tenant},
		{Key: "session_id", Value: sessionID},
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "sequence_number", Value: 1}})

	cursor, err := coll.Find(m.Ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to find session logs: %w", err)
	}
	defer cursor.Close(m.Ctx)

	var sessionLogs []models.SessionActionLog
	if err = cursor.All(m.Ctx, &sessionLogs); err != nil {
		return nil, fmt.Errorf("failed to decode session logs: %w", err)
	}

	return sessionLogs, nil
}
