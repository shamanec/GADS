package db

import "go.mongodb.org/mongo-driver/mongo"

func (m *MongoStore) GetDatabase(dbName string) *mongo.Database {
	return m.Client.Database(dbName)
}

func (m *MongoStore) GetDefaultDatabase() *mongo.Database {
	return m.GetDatabase("gads")
}
