package db

import (
	"GADS/common/models"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const customActionsCollection = "custom_actions"

func (m *MongoStore) GetCustomActions(tenant string) ([]models.CustomAction, error) {
	coll := m.GetCollection(customActionsCollection)
	filter := bson.M{"tenant": tenant}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	return GetDocuments[models.CustomAction](m.Ctx, coll, filter, opts)
}

func (m *MongoStore) GetCustomAction(id, tenant string) (models.CustomAction, error) {
	coll := m.GetCollection(customActionsCollection)
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.CustomAction{}, fmt.Errorf("invalid id format: %w", err)
	}

	filter := bson.M{
		"_id":    objectID,
		"tenant": tenant,
	}
	return GetDocument[models.CustomAction](m.Ctx, coll, filter)
}

func (m *MongoStore) CreateCustomAction(action *models.CustomAction) error {
	coll := m.GetCollection(customActionsCollection)
	now := time.Now()
	action.CreatedAt = now
	action.UpdatedAt = now

	result, err := InsertDocumentWithResult(m.Ctx, coll, action)
	if err != nil {
		return err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		action.ID = oid.Hex()
	}

	return nil
}

func (m *MongoStore) UpdateCustomAction(id, tenant string, action *models.CustomAction) error {
	coll := m.GetCollection(customActionsCollection)
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid id format: %w", err)
	}

	filter := bson.M{
		"_id":    objectID,
		"tenant": tenant,
	}

	action.UpdatedAt = time.Now()
	updates := bson.M{
		"name":        action.Name,
		"description": action.Description,
		"action_type": action.ActionType,
		"parameters":  action.Parameters,
		"is_favorite": action.IsFavorite,
		"updated_at":  action.UpdatedAt,
	}

	return PartialDocumentUpdate(m.Ctx, coll, filter, updates)
}

func (m *MongoStore) DeleteCustomAction(id, tenant string) error {
	coll := m.GetCollection(customActionsCollection)
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid id format: %w", err)
	}

	filter := bson.M{
		"_id":    objectID,
		"tenant": tenant,
	}

	return DeleteDocument(m.Ctx, coll, filter)
}

func (m *MongoStore) CountFavoriteActions(tenant string) (int64, error) {
	coll := m.GetCollection(customActionsCollection)
	filter := bson.M{
		"tenant":      tenant,
		"is_favorite": true,
	}

	return CountDocuments(m.Ctx, coll, filter)
}
