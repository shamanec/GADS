package db

import (
	"GADS/common/models"
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AddWorkspace(workspace *models.Workspace) error {
	collection := mongoClient.Database("gads").Collection("workspaces")
	result, err := collection.InsertOne(mongoClientCtx, workspace)
	if err != nil {
		return err
	}
	workspace.ID = result.InsertedID.(primitive.ObjectID).Hex()
	return nil
}

func UpdateWorkspace(workspace *models.Workspace) error {
	collection := mongoClient.Database("gads").Collection("workspaces")

	objectID, err := primitive.ObjectIDFromHex(workspace.ID)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$set": bson.M{
			"name":        workspace.Name,
			"description": workspace.Description,
		},
	}
	_, err = collection.UpdateOne(mongoClientCtx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func DeleteWorkspace(id string) error {
	collection := mongoClient.Database("gads").Collection("workspaces")

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": objectID}
	_, err = collection.DeleteOne(mongoClientCtx, filter)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoStore) GetWorkspaces(ctx context.Context) ([]models.Workspace, error) {
	coll := m.Collection("workspaces")
	return GetDocuments[models.Workspace](ctx, coll, bson.D{{}})
}

func (m *MongoStore) WorkspaceHasDevices(ctx context.Context, workspaceId string) bool {
	coll := m.Collection("new_devices")
	filter := bson.M{"workspace_id": workspaceId}

	return HasDocuments(ctx, coll, filter)
}

func (m *MongoStore) WorkspaceHasUsers(ctx context.Context, workspaceId string) bool {
	coll := m.Collection("users")
	filter := bson.M{"workspace_id": workspaceId}

	return HasDocuments(ctx, coll, filter)
}

func (m *MongoStore) GetWorkspaceByID(ctx context.Context, workspaceId string) (models.Workspace, error) {
	coll := m.Collection("workspaces")
	objectID, err := primitive.ObjectIDFromHex(workspaceId)
	if err != nil {
		return models.Workspace{}, err
	}
	filter := bson.M{"_id": objectID}

	return GetDocument[models.Workspace](ctx, coll, filter)
}

func (m *MongoStore) GetWorkspaceByName(ctx context.Context, workspaceName string) (models.Workspace, error) {
	coll := m.Collection("workspaces")
	filter := bson.M{"name": workspaceName}

	return GetDocument[models.Workspace](ctx, coll, filter)
}

func (m *MongoStore) GetDefaultWorkspace(ctx context.Context) (models.Workspace, error) {
	coll := m.Collection("workspaces")
	filter := bson.M{"is_default": true}

	return GetDocument[models.Workspace](ctx, coll, filter)
}

func (m *MongoStore) GetWorkspacesPaginated(ctx context.Context, page, limit int, search string) ([]models.Workspace, int64) {
	coll := m.Collection("workspaces")
	// Calculate the number of documents to skip
	skip := (page - 1) * limit

	filter := bson.M{}
	if search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"} // Case-insensitive search
	}

	workspaces, err := GetDocuments[models.Workspace](ctx, coll, filter, options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)))
	if err != nil {
		return []models.Workspace{}, 0
	}
	workspaceCount, err := CountDocuments(ctx, coll, filter)
	if err != nil {
		return []models.Workspace{}, 0
	}

	return workspaces, workspaceCount
}

func (m *MongoStore) GetUserWorkspacesPaginated(ctx context.Context, username string, page, limit int, search string) ([]models.Workspace, int64) {
	return []models.Workspace{}, 0
}

func GetUserWorkspacesPaginated(username string, page, limit int, search string) ([]models.Workspace, int64) {
	var workspaces []models.Workspace
	collection := mongoClient.Database("gads").Collection("workspaces")

	// Calculate skip for pagination
	skip := (page - 1) * limit

	// Get user's workspace IDs from users collection
	userCollection := mongoClient.Database("gads").Collection("users")
	var user models.User
	err := userCollection.FindOne(mongoClientCtx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		return workspaces, 0
	}

	// Build filter for workspaces
	filter := bson.M{"_id": bson.M{"$in": user.WorkspaceIDs}}
	if search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"}
	}

	// Get workspaces with pagination
	cursor, err := collection.Find(mongoClientCtx, filter,
		options.Find().
			SetSkip(int64(skip)).
			SetLimit(int64(limit)))
	if err != nil {
		return workspaces, 0
	}
	defer cursor.Close(mongoClientCtx)

	for cursor.Next(mongoClientCtx) {
		var workspace models.Workspace
		if err := cursor.Decode(&workspace); err != nil {
			continue
		}
		workspaces = append(workspaces, workspace)
	}

	// Get total count
	count, err := collection.CountDocuments(mongoClientCtx, filter)
	if err != nil {
		return workspaces, 0
	}

	return workspaces, count
}
