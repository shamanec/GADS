package db

import (
	"GADS/common/models"

	"go.mongodb.org/mongo-driver/bson"
)

func (m *MongoStore) GetFiles() ([]models.DBFile, error) {
	coll := m.GetCollection("fs.files")
	return GetDocuments[models.DBFile](m.Ctx, coll, bson.D{{}})
}
