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
	sessionLogsColl := m.GetCollectionWithDB(appiumSessionLogsDB, tenant)

	// First query: Get build reports from session logs
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

	cursor, err := sessionLogsColl.Aggregate(m.Ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate build reports: %w", err)
	}
	defer cursor.Close(m.Ctx)

	var buildReports []models.BuildReport
	if err = cursor.All(m.Ctx, &buildReports); err != nil {
		return nil, fmt.Errorf("failed to decode build reports: %w", err)
	}

	// If no builds found, return empty slice
	if len(buildReports) == 0 {
		return buildReports, nil
	}

	// Second query: Get test results counts from session logs where action = "Test Result"
	// Extract build IDs for the test results query
	buildIDs := make([]string, len(buildReports))
	for i, build := range buildReports {
		buildIDs[i] = build.BuildID
	}

	// Aggregate test results from session logs by build_id and test status
	testResultsPipeline := mongo.Pipeline{
		// Match by build IDs and test result actions
		{{Key: "$match", Value: bson.D{
			{Key: "tenant", Value: tenant},
			{Key: "build_id", Value: bson.D{{Key: "$in", Value: buildIDs}}},
			{Key: "action", Value: "Test Result"},
		}}},

		// Group by build_id and count test statuses
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$build_id"},
			{Key: "build_id", Value: bson.D{{Key: "$first", Value: "$build_id"}}},
			{Key: "total_sessions_with_results", Value: bson.D{{Key: "$addToSet", Value: "$session_id"}}},
			{Key: "passed_tests", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: []interface{}{
					bson.D{{Key: "$regexMatch", Value: bson.D{
						{Key: "input", Value: bson.D{{Key: "$toLower", Value: "$test_status"}}},
						{Key: "regex", Value: "pass"},
					}}},
					1, 0,
				}},
			}}}},
			{Key: "failed_tests", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: []interface{}{
					bson.D{{Key: "$regexMatch", Value: bson.D{
						{Key: "input", Value: bson.D{{Key: "$toLower", Value: "$test_status"}}},
						{Key: "regex", Value: "fail"},
					}}},
					1, 0,
				}},
			}}}},
			{Key: "skipped_tests", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: []interface{}{
					bson.D{{Key: "$regexMatch", Value: bson.D{
						{Key: "input", Value: bson.D{{Key: "$toLower", Value: "$test_status"}}},
						{Key: "regex", Value: "skip"},
					}}},
					1, 0,
				}},
			}}}},
		}}},

		// Calculate sessions with results count
		{{Key: "$addFields", Value: bson.D{
			{Key: "sessions_with_results_count", Value: bson.D{{Key: "$size", Value: "$total_sessions_with_results"}}},
		}}},

		// Clean up fields
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "total_sessions_with_results", Value: 0},
		}}},
	}

	testResultsCursor, err := sessionLogsColl.Aggregate(m.Ctx, testResultsPipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate test results: %w", err)
	}
	defer testResultsCursor.Close(m.Ctx)

	type TestCounts struct {
		BuildID                   string `bson:"build_id"`
		SessionsWithResultsCount  int    `bson:"sessions_with_results_count"`
		PassedTests               int    `bson:"passed_tests"`
		FailedTests               int    `bson:"failed_tests"`
		SkippedTests              int    `bson:"skipped_tests"`
	}

	var testCounts []TestCounts
	if err = testResultsCursor.All(m.Ctx, &testCounts); err != nil {
		return nil, fmt.Errorf("failed to decode test counts: %w", err)
	}

	// Create a map of test counts by build_id for quick lookup
	testCountsMap := make(map[string]TestCounts)
	for _, counts := range testCounts {
		testCountsMap[counts.BuildID] = counts
	}

	// Enhance build reports with test counts
	for i := range buildReports {
		if counts, exists := testCountsMap[buildReports[i].BuildID]; exists {
			buildReports[i].PassedTests = counts.PassedTests
			buildReports[i].FailedTests = counts.FailedTests
			buildReports[i].SkippedTests = counts.SkippedTests
			buildReports[i].NoResultTests = buildReports[i].SessionCount - counts.SessionsWithResultsCount
		} else {
			// No test results found, all sessions have no results
			buildReports[i].NoResultTests = buildReports[i].SessionCount
		}
	}

	return buildReports, nil
}

func (m *MongoStore) GetBuildSessions(tenant string, buildID string) ([]models.SessionReport, error) {
	sessionLogsColl := m.GetCollectionWithDB(appiumSessionLogsDB, tenant)

	// First query: Get session reports from action logs
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

	cursor, err := sessionLogsColl.Aggregate(m.Ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate session reports: %w", err)
	}
	defer cursor.Close(m.Ctx)

	var sessionReports []models.SessionReport
	if err = cursor.All(m.Ctx, &sessionReports); err != nil {
		return nil, fmt.Errorf("failed to decode session reports: %w", err)
	}

	// If no sessions found, return empty slice
	if len(sessionReports) == 0 {
		return sessionReports, nil
	}

	// Second query: Get test results from session logs where action = "Test Result"
	// Extract session IDs for the test results query
	sessionIDs := make([]string, len(sessionReports))
	for i, session := range sessionReports {
		sessionIDs[i] = session.SessionID
	}

	// Query test result session logs by session IDs
	testResultsFilter := bson.D{
		{Key: "tenant", Value: tenant},
		{Key: "session_id", Value: bson.D{{Key: "$in", Value: sessionIDs}}},
		{Key: "action", Value: "Test Result"},
	}

	testResultsCursor, err := sessionLogsColl.Find(m.Ctx, testResultsFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to find test results: %w", err)
	}
	defer testResultsCursor.Close(m.Ctx)

	type TestResultLog struct {
		SessionID   string `bson:"session_id"`
		TestStatus  string `bson:"test_status"`
		TestMessage string `bson:"test_message"`
		Timestamp   int64  `bson:"timestamp"`
	}

	var testResults []TestResultLog
	if err = testResultsCursor.All(m.Ctx, &testResults); err != nil {
		return nil, fmt.Errorf("failed to decode test results: %w", err)
	}

	// Create a map of test results by session_id for quick lookup
	testResultsMap := make(map[string]TestResultLog)
	for _, result := range testResults {
		testResultsMap[result.SessionID] = result
	}

	// Enhance session reports with test results
	for i := range sessionReports {
		if testResult, exists := testResultsMap[sessionReports[i].SessionID]; exists {
			sessionReports[i].Status = testResult.TestStatus
			sessionReports[i].Message = testResult.TestMessage
			sessionReports[i].Timestamp = testResult.Timestamp
		}
	}

	return sessionReports, nil
}

func (m *MongoStore) GetSessionLogs(tenant string, sessionID string) ([]models.AppiumPluginSessionLog, error) {
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

	var sessionLogs []models.AppiumPluginSessionLog
	if err = cursor.All(m.Ctx, &sessionLogs); err != nil {
		return nil, fmt.Errorf("failed to decode session logs: %w", err)
	}

	return sessionLogs, nil
}
