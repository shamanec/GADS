package db

import (
	"GADS/common/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (m *MongoStore) GetAppiumLogs(collectionName string, logLimit int) ([]models.AppiumLog, error) {
	coll := m.GetCollectionWithDB("appium_logs", collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "ts", Value: -1}})
	findOptions.SetLimit(int64(logLimit))

	return GetDocuments[models.AppiumLog](m.Ctx, coll, bson.D{{}}, findOptions)
}

func (m *MongoStore) GetProviderLogs(collectionName string, logLimit int) ([]models.ProviderLog, error) {
	coll := m.GetCollectionWithDB("logs", collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	findOptions.SetLimit(int64(logLimit))

	return GetDocuments[models.ProviderLog](m.Ctx, coll, bson.D{{}}, findOptions)
}

func (m *MongoStore) GetAppiumSessionLogs(collectionName, sessionID string) ([]models.AppiumLog, error) {
	coll := m.GetCollectionWithDB("appium_logs", collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "ts", Value: -1}})
	filter := bson.D{{"session_id", sessionID}}

	return GetDocuments[models.AppiumLog](m.Ctx, coll, filter, findOptions)
}

func (m *MongoStore) WriteAppiumLog(collectionName string, log models.AppiumLogEntry) error {
	coll := m.GetCollectionWithDB("appium_logs_new", collectionName)
	return InsertDocument[models.AppiumLogEntry](m.Ctx, coll, log)
}
