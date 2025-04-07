package db

import (
	"GADS/common/models"

	"go.mongodb.org/mongo-driver/bson"
)

func (m *MongoStore) GetProvider(providerNickname string) (models.Provider, error) {
	coll := m.GetCollection("providers")
	filter := bson.D{{Key: "nickname", Value: providerNickname}}
	return GetDocument[models.Provider](m.Ctx, coll, filter)
}

func (m *MongoStore) GetAllProviders() ([]models.Provider, error) {
	coll := m.GetCollection("providers")
	return GetDocuments[models.Provider](m.Ctx, coll, bson.D{{}})
}

func (m *MongoStore) AddOrUpdateProvider(provider models.Provider) error {
	coll := m.GetCollection("providers")
	filter := bson.D{{Key: "nickname", Value: provider.Nickname}}
	return UpsertDocument[models.Provider](m.Ctx, coll, filter, provider)
}

func (m *MongoStore) DeleteProvider(nickname string) error {
	coll := m.GetCollection("providers")
	filter := bson.M{"nickname": nickname}
	return DeleteDocument(m.Ctx, coll, filter)
}
