package db

import (
	"GADS/common/models"
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AddWorkspace(workspace *models.Workspace) error {
	workspace.GenerateUUID()
	collection := mongoClient.Database("gads").Collection("workspaces")
	_, err := collection.InsertOne(mongoClientCtx, workspace)
	if err != nil {
		return err
	}
	return nil
}

func UpdateWorkspace(workspace *models.Workspace) error {
	collection := mongoClient.Database("gads").Collection("workspaces")
	filter := bson.M{"_id": workspace.ID}
	update := bson.M{"$set": workspace}
	_, err := collection.UpdateOne(mongoClientCtx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func DeleteWorkspace(id string) error {
	collection := mongoClient.Database("gads").Collection("workspaces")
	filter := bson.M{"_id": id}
	_, err := collection.DeleteOne(mongoClientCtx, filter)
	if err != nil {
		return err
	}
	return nil
}

func GetWorkspaces() []models.Workspace {
	var workspaces []models.Workspace
	collection := mongoClient.Database("gads").Collection("workspaces")

	cursor, err := collection.Find(mongoClientCtx, bson.M{})
	if err != nil {
		return workspaces
	}
	defer cursor.Close(mongoClientCtx)

	for cursor.Next(mongoClientCtx) {
		var workspace models.Workspace
		if err := cursor.Decode(&workspace); err != nil {
			continue
		}
		workspaces = append(workspaces, workspace)
	}

	return workspaces
}

func WorkspaceHasDevices(id string) bool {
	collection := mongoClient.Database("gads").Collection("new_devices")
	filter := bson.M{"workspace_id": id}
	count, err := collection.CountDocuments(mongoClientCtx, filter)
	if err != nil {
		return false
	}
	return count > 0
}

func GetWorkspaceByID(id string) (models.Workspace, error) {
	var workspace models.Workspace
	collection := mongoClient.Database("gads").Collection("workspaces")
	filter := bson.M{"_id": id}

	err := collection.FindOne(context.TODO(), filter).Decode(&workspace)
	if err != nil {
		return models.Workspace{}, err
	}
	return workspace, nil
}

func GetWorkspaceByName(name string) (models.Workspace, error) {
	var workspace models.Workspace
	collection := mongoClient.Database("gads").Collection("workspaces")
	filter := bson.M{"name": name}

	err := collection.FindOne(context.TODO(), filter).Decode(&workspace)
	if err != nil {
		return models.Workspace{}, err
	}
	return workspace, nil
}

func GetDefaultWorkspace() (models.Workspace, error) {
	var workspace models.Workspace
	collection := mongoClient.Database("gads").Collection("workspaces")
	filter := bson.M{"is_default": true}

	err := collection.FindOne(context.TODO(), filter).Decode(&workspace)
	if err != nil {
		return models.Workspace{}, err
	}
	return workspace, nil
}

func GetWorkspacesPaginated(page, limit int, search string) ([]models.Workspace, int64) {
	var workspaces []models.Workspace
	collection := mongoClient.Database("gads").Collection("workspaces")

	// Calculate the number of documents to skip
	skip := (page - 1) * limit

	filter := bson.M{}
	if search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"} // Case-insensitive search
	}

	cursor, err := collection.Find(mongoClientCtx, filter, options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)))
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

	// Get total count of workspaces
	count, err := collection.CountDocuments(mongoClientCtx, filter)
	if err != nil {
		return workspaces, 0
	}

	return workspaces, count
}
