package db

import (
	"GADS/common/models"
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

func (m *MongoStore) GetProvider(ctx context.Context, providerNickname string) (models.Provider, error) {
	coll := m.Collection("providers")
	filter := bson.D{{Key: "nickname", Value: providerNickname}}
	return GetDocument[models.Provider](ctx, coll, filter)
}

func (m *MongoStore) GetAllProviders(ctx context.Context) ([]models.Provider, error) {
	coll := m.Collection("providers")
	return GetDocuments[models.Provider](ctx, coll, bson.D{{}})
}
