/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package db

import (
	"GADS/common/models"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/exp/slices"
)

var (
	ErrInvalidPagination = errors.New("invalid pagination parameters")
	ErrAggregationFailed = errors.New("aggregation query failed")
)

func (m *MongoStore) AddWorkspace(workspace *models.Workspace) error {
	coll := m.GetCollection("workspaces")
	result, err := InsertDocumentWithResult[models.Workspace](m.Ctx, coll, *workspace)
	if err != nil {
		return err
	}
	workspace.ID = result.InsertedID.(primitive.ObjectID).Hex()
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
		"tenant":      workspace.Tenant,
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
		// Case-insensitive search in name, description, or tenant
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": search, "$options": "i"}},
			{"description": bson.M{"$regex": search, "$options": "i"}},
			{"tenant": bson.M{"$regex": search, "$options": "i"}},
		}
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

func (m *MongoStore) GetUserWorkspaces(username string) []models.Workspace {
	user, err := m.GetUser(username)
	if err != nil {
		return []models.Workspace{}
	}
	dbWorkspaces, err := m.GetWorkspaces()
	if err != nil {
		return []models.Workspace{}
	}

	var userWorkspaces []models.Workspace
	for _, dbWorkspace := range dbWorkspaces {
		if slices.Contains(user.WorkspaceIDs, dbWorkspace.ID) {
			userWorkspaces = append(userWorkspaces, dbWorkspace)
		}
	}
	return userWorkspaces
}

func (m *MongoStore) GetWorkspacesWithDeviceCount(page, limit int, search string) ([]models.WorkspaceWithDeviceCount, int64, error) {
	if page < 1 || limit < 1 || limit > 1000 {
		return nil, 0, ErrInvalidPagination
	}

	workspacesColl := m.GetCollection("workspaces")
	skip := int64((page - 1) * limit)

	matchFilter := bson.M{}
	if search != "" {
		regex := bson.M{"$regex": search, "$options": "i"}
		matchFilter["$or"] = []bson.M{
			{"name": regex},
			{"description": regex},
			{"tenant": regex},
		}
	}

	pipeline := []bson.M{
		{"$match": matchFilter},
		{
			"$lookup": bson.M{
				"from": "new_devices",
				"let":  bson.M{"workspace_id": bson.M{"$toString": "$_id"}},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{
								"$eq": []interface{}{"$workspace_id", "$$workspace_id"},
							},
						},
					},
					{"$project": bson.M{"_id": 1}},
				},
				"as": "devices",
			},
		},
		{
			"$addFields": bson.M{
				"device_count": bson.M{"$size": "$devices"},
			},
		},
		{"$unset": "devices"},
		{"$skip": skip},
		{"$limit": int64(limit)},
	}

	cursor, err := workspacesColl.Aggregate(m.Ctx, pipeline)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrAggregationFailed, err)
	}
	defer cursor.Close(m.Ctx)

	var workspaces []models.WorkspaceWithDeviceCount
	if err = cursor.All(m.Ctx, &workspaces); err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrAggregationFailed, err)
	}

	var totalCount int64
	if len(matchFilter) == 0 {
		totalCount, err = workspacesColl.CountDocuments(m.Ctx, bson.M{})
		if err != nil {
			return workspaces, 0, fmt.Errorf("failed to get total count: %v", err)
		}
	} else {
		countPipeline := []bson.M{
			{"$match": matchFilter},
			{"$count": "total"},
		}

		countCursor, err := workspacesColl.Aggregate(m.Ctx, countPipeline)
		if err != nil {
			return workspaces, 0, fmt.Errorf("failed to get total count: %v", err)
		}
		defer countCursor.Close(m.Ctx)

		var countResult []bson.M
		if err = countCursor.All(m.Ctx, &countResult); err != nil {
			return workspaces, 0, fmt.Errorf("failed to get total count: %v", err)
		}

		if len(countResult) > 0 {
			if count, ok := countResult[0]["total"].(int32); ok {
				totalCount = int64(count)
			}
		}
	}

	return workspaces, totalCount, nil
}
