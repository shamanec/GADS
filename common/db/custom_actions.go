package db

import (
	"GADS/common/models"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const customActionsCollection = "custom_actions"
const userFavoriteActionsCollection = "user_favorite_actions"

func (m *MongoStore) GetCustomActions() ([]models.CustomAction, error) {
	coll := m.GetCollection(customActionsCollection)
	filter := bson.M{}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	return GetDocuments[models.CustomAction](m.Ctx, coll, filter, opts)
}

func (m *MongoStore) GetCustomAction(id string) (models.CustomAction, error) {
	coll := m.GetCollection(customActionsCollection)
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.CustomAction{}, fmt.Errorf("invalid id format: %w", err)
	}

	filter := bson.M{"_id": objectID}
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

func (m *MongoStore) UpdateCustomAction(id string, action *models.CustomAction) error {
	coll := m.GetCollection(customActionsCollection)
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid id format: %w", err)
	}

	filter := bson.M{"_id": objectID}

	action.UpdatedAt = time.Now()
	updates := bson.M{
		"name":        action.Name,
		"description": action.Description,
		"action_type": action.ActionType,
		"parameters":  action.Parameters,
		"updated_at":  action.UpdatedAt,
	}

	return PartialDocumentUpdate(m.Ctx, coll, filter, updates)
}

func (m *MongoStore) DeleteCustomAction(id string) error {
	coll := m.GetCollection(customActionsCollection)
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid id format: %w", err)
	}

	filter := bson.M{"_id": objectID}

	if err := DeleteDocument(m.Ctx, coll, filter); err != nil {
		return err
	}

	// Cleanup favoritos associados
	if err := m.DeleteFavoritesByActionID(id); err != nil {
		fmt.Printf("Warning: failed to cleanup favorites for action %s: %v\n", id, err)
	}

	return nil
}

func (m *MongoStore) GetUserFavoriteActionIDs(username string) ([]string, error) {
	coll := m.GetCollection(userFavoriteActionsCollection)
	filter := bson.M{"username": username}

	var favorites []models.UserFavoriteAction
	cursor, err := coll.Find(m.Ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(m.Ctx)

	if err = cursor.All(m.Ctx, &favorites); err != nil {
		return nil, err
	}

	ids := make([]string, len(favorites))
	for i, fav := range favorites {
		ids[i] = fav.ActionID
	}
	return ids, nil
}

func (m *MongoStore) AddUserFavoriteAction(username, actionID string) error {
	count, err := m.CountUserFavoriteActions(username)
	if err != nil {
		return err
	}
	if count >= 5 {
		return fmt.Errorf("user already has maximum of 5 favorite actions")
	}

	coll := m.GetCollection(userFavoriteActionsCollection)
	favorite := models.UserFavoriteAction{
		Username: username,
		ActionID: actionID,
	}

	_, err = InsertDocumentWithResult(m.Ctx, coll, favorite)
	return err
}

func (m *MongoStore) RemoveUserFavoriteAction(username, actionID string) error {
	coll := m.GetCollection(userFavoriteActionsCollection)
	filter := bson.M{
		"username":  username,
		"action_id": actionID,
	}
	return DeleteDocument(m.Ctx, coll, filter)
}

func (m *MongoStore) CountUserFavoriteActions(username string) (int64, error) {
	coll := m.GetCollection(userFavoriteActionsCollection)
	filter := bson.M{"username": username}
	return CountDocuments(m.Ctx, coll, filter)
}

func (m *MongoStore) DeleteFavoritesByActionID(actionID string) error {
	coll := m.GetCollection(userFavoriteActionsCollection)
	filter := bson.M{"action_id": actionID}
	_, err := coll.DeleteMany(m.Ctx, filter)
	return err
}

// CreateUserFavoriteActionIndexes creates database indexes for user favorite actions
func (m *MongoStore) CreateUserFavoriteActionIndexes() error {
	coll := m.GetCollection(userFavoriteActionsCollection)

	// Unique compound index on (username, action_id)
	// Prevents duplicate favorites for the same user
	uniqueIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "username", Value: 1},
			{Key: "action_id", Value: 1},
		},
		Options: &options.IndexOptions{
			Unique: &[]bool{true}[0],
		},
	}

	// Index on username for fast user favorite queries
	usernameIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "username", Value: 1},
		},
	}

	// Index on action_id for cleanup when actions are deleted
	actionIDIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "action_id", Value: 1}},
	}

	indexes := []mongo.IndexModel{uniqueIndex, usernameIndex, actionIDIndex}

	_, err := coll.Indexes().CreateMany(m.Ctx, indexes)
	return err
}
