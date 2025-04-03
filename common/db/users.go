package db

import (
	"GADS/common/models"
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

func (m *MongoStore) GetUser(ctx context.Context, username string) (models.User, error) {
	coll := m.Collection("users")
	filter := bson.D{{Key: "username", Value: username}}
	return GetDocument[models.User](ctx, coll, filter)
}

func (m *MongoStore) GetUsers(ctx context.Context) ([]models.User, error) {
	coll := m.Collection("users")
	return GetDocuments[models.User](ctx, coll, bson.D{{}})
}
