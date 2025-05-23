/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package auth

import (
	"context"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const secretKeyAuditLogsCollection = "secret_key_audit_logs"

// SecretKeyAuditLog represents an audit record for Secret Key operations
type SecretKeyAuditLog struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Username      string             `bson:"username" json:"user"`
	SecretKeyID   primitive.ObjectID `bson:"secret_key_id" json:"secret_key_id"`
	Origin        string             `bson:"origin" json:"origin"`
	Action        string             `bson:"action" json:"action"` // "create", "update", "disable"
	Timestamp     time.Time          `bson:"timestamp" json:"timestamp"`
	IsDefault     bool               `bson:"is_default" json:"is_default"`
	PreviousKey   *string            `bson:"previous_key,omitempty" json:"-"` // Not exposed in API for security reasons
	NewKey        *string            `bson:"new_key,omitempty" json:"-"`      // Not exposed in API for security reasons
	Justification string             `bson:"justification,omitempty" json:"justification,omitempty"`
}

// SecretKeyAuditStore manages audit records for Secret Keys
type SecretKeyAuditStore struct {
	db *mongo.Database
}

// NewSecretKeyAuditStore creates a new AuditStore instance
func NewSecretKeyAuditStore(database *mongo.Database) *SecretKeyAuditStore {
	return &SecretKeyAuditStore{
		db: database,
	}
}

// LogAction records a Secret Key change action
func (a *SecretKeyAuditStore) LogAction(log *SecretKeyAuditLog) error {
	// Set timestamp if not defined
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	// Insert log into collection
	_, err := a.db.Collection(secretKeyAuditLogsCollection).InsertOne(context.Background(), log)
	return err
}

// GetHistory retrieves history with pagination and filters
func (a *SecretKeyAuditStore) GetHistory(page, limit int, filters map[string]interface{}) ([]SecretKeyAuditLog, int64, error) {
	// Apply filters
	filter := bson.M{}
	for key, value := range filters {
		if key == "from_date" {
			if _, exists := filter["timestamp"]; !exists {
				filter["timestamp"] = bson.M{}
			}
			filter["timestamp"].(bson.M)["$gte"] = value
		} else if key == "to_date" {
			if _, exists := filter["timestamp"]; !exists {
				filter["timestamp"] = bson.M{}
			}
			filter["timestamp"].(bson.M)["$lte"] = value
		} else {
			filter[key] = value
		}
	}

	// Prepare search options with pagination and sort by timestamp (most recent first)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	findOptions.SetSkip(int64((page - 1) * limit))
	findOptions.SetLimit(int64(limit))

	// Perform search
	cursor, err := a.db.Collection(secretKeyAuditLogsCollection).Find(context.Background(), filter, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.Background())

	// Get results
	var auditLogs []SecretKeyAuditLog
	if err := cursor.All(context.Background(), &auditLogs); err != nil {
		return nil, 0, err
	}

	// Count total records (without pagination)
	total, err := a.db.Collection(secretKeyAuditLogsCollection).CountDocuments(context.Background(), filter)
	if err != nil {
		return nil, 0, err
	}

	return auditLogs, total, nil
}

// GetAuditLogByID retrieves a specific record
func (a *SecretKeyAuditStore) GetAuditLogByID(id primitive.ObjectID) (*SecretKeyAuditLog, error) {
	filter := bson.M{"_id": id}
	var auditLog SecretKeyAuditLog

	err := a.db.Collection(secretKeyAuditLogsCollection).FindOne(context.Background(), filter).Decode(&auditLog)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, mongo.ErrNoDocuments
		}
		return nil, err
	}

	return &auditLog, nil
}

// CreateMongoIndexes creates indexes needed for performance
func (a *SecretKeyAuditStore) CreateMongoIndexes() error {
	// Define indexes to be created
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "timestamp", Value: -1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{Key: "origin", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{Key: "username", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{Key: "action", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetBackground(true),
		},
	}

	// Create indexes
	_, err := a.db.Collection(secretKeyAuditLogsCollection).Indexes().CreateMany(context.Background(), indexes)
	return err
}

// FormatHistoryResponse formats the API response with pagination
func FormatHistoryResponse(logs []SecretKeyAuditLog, total int64, page, limit int) map[string]interface{} {
	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return map[string]interface{}{
		"items": logs,
		"total": total,
		"page":  page,
		"limit": limit,
		"pages": totalPages,
	}
}
