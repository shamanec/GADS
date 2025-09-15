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
	"fmt"
	"slices"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	return m.Client.Database(m.DatabaseName).ListCollectionNames(m.Ctx, bson.M{})
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

func (m *MongoStore) CreateCollection(collectionName string, opts *options.CreateCollectionOptions) error {
	return m.CreateCollectionWithDB(m.DatabaseName, collectionName, opts)
}

func (m *MongoStore) CreateCollectionWithDB(dbName, collectionName string, opts ...*options.CreateCollectionOptions) error {
	err := m.Client.Database(dbName).CreateCollection(m.Ctx, collectionName, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoStore) CreateCappedCollection(collectionName string, maxDocuments, mb int64) error {
	return m.CreateCappedCollectionWithDB(m.DatabaseName, collectionName, maxDocuments, mb)
}

func (m *MongoStore) CreateCappedCollectionWithDB(dbName, collectionName string, maxDocuments, mb int64) error {
	collections, err := m.GetCollectionNamesWithDB(dbName)
	if err != nil {
		return fmt.Errorf("Failed getting collection names - %s", err.Error())
	}
	if slices.Contains(collections, collectionName) {
		return nil
	}

	// Create capped collection options with limit of documents or 20 mb size limit
	// Seems reasonable for now, I have no idea what is a proper amount
	collectionOptions := options.CreateCollection()
	collectionOptions.SetCapped(true)
	collectionOptions.SetMaxDocuments(maxDocuments)
	collectionOptions.SetSizeInBytes(mb * 1024 * 1024)

	return m.CreateCollectionWithDB(dbName, collectionName, collectionOptions)
}

func (m *MongoStore) CreateAppiumTenantLogsCollection(dbName, collectionName string, maxDocuments, mb int64) error {
	err := m.CreateCappedCollectionWithDB(dbName, collectionName, maxDocuments, mb)
	if err != nil {
		return fmt.Errorf("failed creating capped logs collection for tenant `%s`", collectionName)
	}

	tenantBuildTimestampIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "tenant", Value: 1},
			{Key: "build_id", Value: 1},
			{Key: "timestamp", Value: -1},
		},
		Options: options.Index().SetName("tenant_build_timestamp_idx"),
	}
	err = m.AddCollectionIndexWithDB(dbName, collectionName, tenantBuildTimestampIndex)
	if err != nil {
		return fmt.Errorf("failed adding collection index on logs collection for tenant `%s`", collectionName)
	}
	return nil
}
