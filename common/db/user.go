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

func (m *MongoStore) GetUser(username string) (models.User, error) {
	coll := m.GetCollection("users")
	filter := bson.D{{Key: "username", Value: username}}
	return GetDocument[models.User](m.Ctx, coll, filter)
}

func (m *MongoStore) GetUsers() ([]models.User, error) {
	coll := m.GetCollection("users")
	return GetDocuments[models.User](m.Ctx, coll, bson.D{{}})
}

func (m *MongoStore) AddOrUpdateUser(user models.User) error {
	coll := m.GetCollection("users")
	filter := bson.D{{Key: "username", Value: user.Username}}
	return UpsertDocument[models.User](m.Ctx, coll, filter, user)
}

// InsertUser adds a new user and fails with a duplicate key error if the username is taken
func (m *MongoStore) InsertUser(user models.User) error {
	coll := m.GetCollection("users")
	return InsertDocument[models.User](m.Ctx, coll, user)
}

// UpdateSSOUser applies the role and subject of an SSO sign-in. It only matches
// while the user is still an SSO user not bound to another subject, so a user
// deleted or changed during the sign-in is neither recreated nor taken over.
// It reports whether the user matched
func (m *MongoStore) UpdateSSOUser(username, subject, role string) (bool, error) {
	coll := m.GetCollection("users")
	filter := bson.M{
		"username":    username,
		"auth_source": models.AuthSourceOIDC,
		"$or": bson.A{
			bson.M{"oidc_subject": subject},
			bson.M{"oidc_subject": bson.M{"$exists": false}},
			bson.M{"oidc_subject": ""},
		},
	}
	update := bson.M{"$set": bson.M{"role": role, "oidc_subject": subject}}

	result, err := coll.UpdateOne(m.Ctx, filter, update)
	if err != nil {
		return false, fmt.Errorf("failed to update SSO user: %w", err)
	}
	return result.MatchedCount == 1, nil
}

// CreateUserIndexes makes usernames unique, so two concurrent first sign-ins of the same
// SSO user cannot create the account twice
func (m *MongoStore) CreateUserIndexes() error {
	coll := m.GetCollection("users")

	usernameIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "username", Value: 1}},
		Options: &options.IndexOptions{
			Unique: &[]bool{true}[0],
		},
	}

	_, err := coll.Indexes().CreateOne(m.Ctx, usernameIndex)
	if err != nil {
		return fmt.Errorf("failed to create users indexes: %w", err)
	}
	return nil
}

func (m *MongoStore) DeleteUser(nickname string) error {
	coll := m.GetCollection("users")
	filter := bson.M{"username": nickname}
	return DeleteDocument(m.Ctx, coll, filter)
}

func (m *MongoStore) AddAdminUserIfMissing() error {
	dbUser, err := GlobalMongoStore.GetUser("admin")
	if err != nil && err != mongo.ErrNoDocuments {
		return fmt.Errorf("AddAdminUserIfMissing: Failed to check if admin user is in the DB - %s", err)
	}

	if dbUser.Username != "" {
		return nil // User exists
	}

	err = GlobalMongoStore.AddOrUpdateUser(models.User{Username: "admin", Password: "password", Role: "admin"})
	if err != nil {
		return fmt.Errorf("Failed to add/update admin user - %s", err)
	}
	return nil
}

func (m *MongoStore) UpdateUserWorkspaces(username string, workspaceIDs []string) error {
	coll := m.GetCollection("users")
	filter := bson.M{"username": username}
	updates := bson.M{
		"workspace_ids": workspaceIDs,
	}
	return PartialDocumentUpdate(m.Ctx, coll, filter, updates)
}

// UpdateUserPassword updates only the password field for the given user, leaving
// role and workspaces untouched (a full upsert of a partial User struct would
// clobber those fields).
func (m *MongoStore) UpdateUserPassword(username, newPassword string) error {
	coll := m.GetCollection("users")
	filter := bson.M{"username": username}
	updates := bson.M{
		"password": newPassword,
	}
	return PartialDocumentUpdate(m.Ctx, coll, filter, updates)
}
