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

func (m *MongoStore) GetAppiumSessionLogs(collectionName, sessionID string) ([]models.AppiumPluginLog, error) {
	coll := m.GetCollectionWithDB(appiumLogDB, collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	filter := bson.D{{Key: "session_id", Value: sessionID}}

	return GetDocuments[models.AppiumPluginLog](m.Ctx, coll, filter, findOptions)
}

func (m *MongoStore) AddAppiumLog(collectionName string, log models.AppiumPluginLog) error {
	coll := m.GetCollectionWithDB(appiumLogDB, collectionName)

	return InsertDocument[models.AppiumPluginLog](m.Ctx, coll, log)
}

func (m *MongoStore) AddAppiumSessionLog(collectionName string, log models.AppiumPluginSessionLog) error {
	coll := m.GetCollectionWithDB(appiumSessionLogsDB, collectionName)

	return InsertDocument[models.AppiumPluginSessionLog](m.Ctx, coll, log)
}

func (m *MongoStore) ListAppiumSessions(
	collectionName string,
	platform, udid *string, // optional filters
	limit int64,
) ([]models.SessionLogsSummary, error) {
	// Build the $match filters for platform_name and/or device udid
	match := bson.D{}
	if platform != nil && *platform != "" {
		match = append(match, bson.E{Key: "platform_name", Value: *platform})
	}
	if udid != nil && *udid != "" {
		match = append(match, bson.E{Key: "udid", Value: *udid})
	}

	// Same first stages; no $facet at the end.
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		// We sort by session_id and the sequence_number that the plugin adds when logging
		{{Key: "$sort", Value: bson.D{
			{Key: "session_id", Value: 1},
			{Key: "sequence_number", Value: -1},
		}}},
		// First we group the logs by session_id which makes sure all the log results are per unique session
		// And do the same for platform_name, udid, build_id, device_name
		// *****
		// Since we already ordered by sequence_number in the sort above for end_sequence_number we get the first one currently in the list
		// And for start_sequence_number we the last one currently in the list
		// *****
		// And we the the total count of documents
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$session_id"},
			{Key: "session_id", Value: bson.D{{Key: "$first", Value: "$session_id"}}},
			{Key: "platform_name", Value: bson.D{{Key: "$first", Value: "$platform_name"}}},
			{Key: "udid", Value: bson.D{{Key: "$first", Value: "$udid"}}},
			{Key: "build_id", Value: bson.D{{Key: "$first", Value: "$build_id"}}},
			{Key: "device_name", Value: bson.D{{Key: "$first", Value: "$device_name"}}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		// Then we sort all the documents by the end_sequence_number to make sure they are ordered as we expect
		{{Key: "$sort", Value: bson.D{{Key: "end_sequence_number", Value: -1}}}},
		// We apply the requested limit of documents
		{{Key: "$limit", Value: limit}},
		{{Key: "$project", Value: bson.D{{Key: "_id", Value: 0}}}},
	}

	coll := m.GetCollectionWithDB(appiumSessionLogsDB, collectionName)
	cur, err := coll.Aggregate(m.Ctx, pipeline, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	defer cur.Close(m.Ctx)

	var items []models.SessionLogsSummary
	if err := cur.All(m.Ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (m *MongoStore) GetBuildReports(tenant string, limit int64) ([]models.BuildReport, error) {
	coll := m.GetCollectionWithDB(appiumSessionLogsDB, tenant)

	pipeline := mongo.Pipeline{
		// Match by tenant and only records with build_id
		{{Key: "$match", Value: bson.D{
			{Key: "tenant", Value: tenant},
			{Key: "build_id", Value: bson.D{{Key: "$ne", Value: ""}}},
		}}},

		// Group by build_id and build_run_id first
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "build_id", Value: "$build_id"},
				{Key: "build_run_id", Value: "$build_run_id"},
			}},
			{Key: "build_id", Value: bson.D{{Key: "$first", Value: "$build_id"}}},
			{Key: "build_run_id", Value: bson.D{{Key: "$first", Value: "$build_run_id"}}},
			{Key: "build_run_timestamp", Value: bson.D{{Key: "$first", Value: "$build_run_timestamp"}}},
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

		// Group by build_id to aggregate all runs
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$build_id"},
			{Key: "build_id", Value: bson.D{{Key: "$first", Value: "$build_id"}}},
			{Key: "total_runs", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "latest_run", Value: bson.D{{Key: "$max", Value: "$build_run_timestamp"}}},
			{Key: "build_runs", Value: bson.D{{Key: "$push", Value: bson.D{
				{Key: "build_run_id", Value: "$build_run_id"},
				{Key: "build_run_timestamp", Value: "$build_run_timestamp"},
				{Key: "session_count", Value: "$session_count"},
				{Key: "session_ids", Value: "$session_ids"},
				{Key: "test_names", Value: "$test_names"},
				{Key: "device_names", Value: "$device_names"},
				{Key: "first_action", Value: "$first_action"},
				{Key: "last_action", Value: "$last_action"},
			}}}},
		}}},

		// Sort build runs within each build by timestamp (newest first)
		{{Key: "$addFields", Value: bson.D{
			{Key: "build_runs", Value: bson.D{{Key: "$sortArray", Value: bson.D{
				{Key: "input", Value: "$build_runs"},
				{Key: "sortBy", Value: bson.D{{Key: "build_run_timestamp", Value: -1}}},
			}}}},
		}}},

		// Sort builds by latest run timestamp (newest first)
		{{Key: "$sort", Value: bson.D{{Key: "latest_run", Value: -1}}}},

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
