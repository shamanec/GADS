package db

import (
	"slices"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (m *MongoStore) GetCollection(name string) *mongo.Collection {
	return m.GetDefaultDatabase().Collection(name)
}

func (m *MongoStore) GetCollectionWithDB(dbName, name string) *mongo.Collection {
	return m.GetDatabase(dbName).Collection(name)
}

func (m *MongoStore) CheckCollectionExists(collectionName string) (bool, error) {
	collections, err := m.GetCollectionNames()
	if err != nil {
		return false, err
	}
	if slices.Contains(collections, collectionName) {
		return true, nil
	}
	return false, nil
}

func (m *MongoStore) CheckCollectionExistsWithDB(dbName, collectionName string) (bool, error) {
	collections, err := m.GetCollectionNamesWithDB(dbName)
	if err != nil {
		return false, err
	}
	if slices.Contains(collections, collectionName) {
		return true, nil
	}
	return false, nil
}

func (m *MongoStore) GetCollectionNames() ([]string, error) {
	return m.Client.Database(m.DefaultDatabaseName).ListCollectionNames(m.Ctx, bson.M{})
}

func (m *MongoStore) GetCollectionNamesWithDB(dbName string) ([]string, error) {
	return m.Client.Database(dbName).ListCollectionNames(m.Ctx, bson.M{})
}

func (m *MongoStore) AddCollectionIndex(collectionName string, indexModel mongo.IndexModel) error {
	_, err := m.GetCollection(collectionName).Indexes().CreateOne(m.Ctx, indexModel)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoStore) AddCollectionIndexWithDB(dbName, collectionName string, indexModel mongo.IndexModel) error {
	_, err := m.Client.Database(dbName).Collection(collectionName).Indexes().CreateOne(m.Ctx, indexModel)
	if err != nil {
		return err
	}
	return nil
}
