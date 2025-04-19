package db

import (
	"GADS/common/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (m *MongoStore) AddWorkspace(workspace *models.Workspace) error {
	coll := m.GetCollection("workspaces")
	result, err := UpsertDocumentWithResult[models.Workspace](m.Ctx, coll, bson.D{{}}, *workspace)
	if err != nil {
		return err
	}
	workspace.ID = result.UpsertedID.(primitive.ObjectID).Hex()
	return nil
}

func (m *MongoStore) UpdateWorkspace(workspace *models.Workspace) error {
	coll := m.GetCollection("workspaces")
	objectID, err := primitive.ObjectIDFromHex(workspace.ID)
	if err != nil {
		return err
	}
	filter := bson.M{"_id": objectID}
	update := bson.M{
		"name":        workspace.Name,
		"description": workspace.Description,
	}
	return PartialDocumentUpdate(m.Ctx, coll, filter, update)
}

func (m *MongoStore) DeleteWorkspace(id string) error {
	coll := m.GetCollection("workspaces")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	filter := bson.M{"_id": objectID}
	return DeleteDocument(m.Ctx, coll, filter)
}

func (m *MongoStore) GetWorkspaces() ([]models.Workspace, error) {
	coll := m.GetCollection("workspaces")
	return GetDocuments[models.Workspace](m.Ctx, coll, bson.D{{}})
}

func (m *MongoStore) WorkspaceHasDevices(workspaceId string) bool {
	coll := m.GetCollection("new_devices")
	filter := bson.M{"workspace_id": workspaceId}

	return HasDocuments(m.Ctx, coll, filter)
}

func (m *MongoStore) WorkspaceHasUsers(workspaceId string) bool {
	coll := m.GetCollection("users")
	filter := bson.M{"workspace_id": workspaceId}

	return HasDocuments(m.Ctx, coll, filter)
}

func (m *MongoStore) GetWorkspaceByID(workspaceId string) (models.Workspace, error) {
	coll := m.GetCollection("workspaces")
	objectID, err := primitive.ObjectIDFromHex(workspaceId)
	if err != nil {
		return models.Workspace{}, err
	}
	filter := bson.M{"_id": objectID}

	return GetDocument[models.Workspace](m.Ctx, coll, filter)
}

func (m *MongoStore) GetWorkspaceByName(workspaceName string) (models.Workspace, error) {
	coll := m.GetCollection("workspaces")
	filter := bson.M{"name": workspaceName}

	return GetDocument[models.Workspace](m.Ctx, coll, filter)
}

func (m *MongoStore) GetDefaultWorkspace() (models.Workspace, error) {
	coll := m.GetCollection("workspaces")
	filter := bson.M{"is_default": true}

	return GetDocument[models.Workspace](m.Ctx, coll, filter)
}

func (m *MongoStore) GetWorkspacesPaginated(page, limit int, search string) ([]models.Workspace, int64) {
	coll := m.GetCollection("workspaces")
	// Calculate the number of documents to skip
	skip := (page - 1) * limit

	filter := bson.M{}
	if search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"} // Case-insensitive search
	}

	workspaces, err := GetDocuments[models.Workspace](m.Ctx, coll, filter, options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)))
	if err != nil {
		return []models.Workspace{}, 0
	}
	workspaceCount, err := CountDocuments(m.Ctx, coll, filter)
	if err != nil {
		return []models.Workspace{}, 0
	}

	return workspaces, workspaceCount
}

func (m *MongoStore) GetUserWorkspacesPaginated(username string, page, limit int, search string) ([]models.Workspace, int64) {
	return []models.Workspace{}, 0
}

func GetUserWorkspacesPaginated(username string, page, limit int, search string) ([]models.Workspace, int64) {
	var workspaces []models.Workspace
	collection := GlobalMongoStore.Client.Database("gads").Collection("workspaces")

	// Calculate skip for pagination
	skip := (page - 1) * limit

	// Get user's workspace IDs from users collection
	userCollection := GlobalMongoStore.Client.Database("gads").Collection("users")
	var user models.User
	err := userCollection.FindOne(GlobalMongoStore.Ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		return workspaces, 0
	}

	// Build filter for workspaces
	filter := bson.M{"_id": bson.M{"$in": user.WorkspaceIDs}}
	if search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"}
	}

	// Get workspaces with pagination
	cursor, err := collection.Find(GlobalMongoStore.Ctx, filter,
		options.Find().
			SetSkip(int64(skip)).
			SetLimit(int64(limit)))
	if err != nil {
		return workspaces, 0
	}
	defer cursor.Close(GlobalMongoStore.Ctx)

	for cursor.Next(GlobalMongoStore.Ctx) {
		var workspace models.Workspace
		if err := cursor.Decode(&workspace); err != nil {
			continue
		}
		workspaces = append(workspaces, workspace)
	}

	// Get total count
	count, err := collection.CountDocuments(GlobalMongoStore.Ctx, filter)
	if err != nil {
		return workspaces, 0
	}

	return workspaces, count
}
