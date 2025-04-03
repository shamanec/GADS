package db

import (
	"GADS/common/models"
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

func (m *MongoStore) GetFiles(ctx context.Context) ([]models.DBFile, error) {
	coll := m.Collection("fs.files")
	return GetDocuments[models.DBFile](ctx, coll, bson.D{{}})
}
