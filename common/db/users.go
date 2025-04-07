package db

import (
	"GADS/common/models"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (m *MongoStore) GetUser(username string) (models.User, error) {
	coll := m.Collection("users")
	filter := bson.D{{Key: "username", Value: username}}
	return GetDocument[models.User](m.Ctx, coll, filter)
}

func (m *MongoStore) GetUsers() ([]models.User, error) {
	coll := m.Collection("users")
	return GetDocuments[models.User](m.Ctx, coll, bson.D{{}})
}

func (m *MongoStore) AddOrUpdateUser(user models.User) error {
	coll := m.Collection("users")
	filter := bson.D{{Key: "username", Value: user.Username}}
	return UpsertDocument[models.User](m.Ctx, coll, filter, user)
}

func (m *MongoStore) DeleteUser(nickname string) error {
	coll := m.Collection("users")
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
