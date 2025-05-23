/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package db

import "go.mongodb.org/mongo-driver/mongo"

func (m *MongoStore) GetDatabase(dbName string) *mongo.Database {
	return m.Client.Database(dbName)
}

func (m *MongoStore) GetDefaultDatabase() *mongo.Database {
	return m.GetDatabase("gads")
}
