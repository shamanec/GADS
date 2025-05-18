package auth

import (
	"context"
	"errors"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const secretKeysCollection = "secret_keys"

// SecretKey represents a JWT secret key for a specific origin
type SecretKey struct {
	ID                    primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Origin                string             `bson:"origin" json:"origin"`
	Key                   string             `bson:"key" json:"key"`
	IsDefault             bool               `bson:"is_default" json:"is_default"`
	CreatedAt             time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt             time.Time          `bson:"updated_at" json:"updated_at"`
	Disabled              bool               `bson:"disabled" json:"disabled"`
	DisabledAt            *time.Time         `bson:"disabled_at,omitempty" json:"disabled_at,omitempty"`
	UserIdentifierClaim   string             `bson:"user_identifier_claim,omitempty" json:"user_identifier_claim,omitempty"`
	TenantIdentifierClaim string             `bson:"tenant_identifier_claim,omitempty" json:"tenant_identifier_claim,omitempty"`
}

// Errors
var (
	ErrSecretKeyNotFound    = errors.New("secret key not found")
	ErrDefaultKeyNotFound   = errors.New("default secret key not found")
	ErrDuplicateOrigin      = errors.New("an origin with this name already exists")
	ErrMultipleDefaultKeys  = errors.New("multiple default keys found")
	ErrCannotDisableDefault = errors.New("cannot disable the default secret key")
)

// SecretStore provides methods to manage JWT secret keys
type SecretStore struct {
	db         *mongo.Database
	auditStore *SecretKeyAuditStore
}

// NewSecretStore creates a new SecretStore instance
func NewSecretStore(database *mongo.Database) *SecretStore {
	return &SecretStore{
		db:         database,
		auditStore: NewSecretKeyAuditStore(database),
	}
}

// AddSecretKey adds a new secret key for an origin
func (s *SecretStore) AddSecretKey(secretKey *SecretKey, username, justification string) error {
	// Check if origin already exists
	filter := bson.M{"origin": secretKey.Origin, "disabled": false}
	count, err := s.db.Collection(secretKeysCollection).CountDocuments(context.Background(), filter)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrDuplicateOrigin
	}

	// If this is a default key, ensure no other default keys exist
	if secretKey.IsDefault {
		err = s.ensureNoOtherDefaultKeys()
		if err != nil {
			return err
		}
	}

	// Set timestamps
	now := time.Now()
	secretKey.CreatedAt = now
	secretKey.UpdatedAt = now
	secretKey.Disabled = false

	result, err := s.db.Collection(secretKeysCollection).InsertOne(context.Background(), secretKey)
	if err != nil {
		return err
	}

	secretKey.ID = result.InsertedID.(primitive.ObjectID)

	// Add audit log
	auditLog := &SecretKeyAuditLog{
		Username:      username,
		SecretKeyID:   secretKey.ID,
		Origin:        secretKey.Origin,
		Action:        "create",
		Timestamp:     now,
		IsDefault:     secretKey.IsDefault,
		NewKey:        &secretKey.Key,
		Justification: justification,
	}

	if err := s.auditStore.LogAction(auditLog); err != nil {
		// Log error but continue - failure to audit shouldn't block the main operation
		log.Printf("Error logging secret key audit event: %v", err)
	}

	return nil
}

// UpdateSecretKey updates an existing secret key
func (s *SecretStore) UpdateSecretKey(secretKey *SecretKey, username, justification string) error {
	// Get current state for audit
	currentKey, err := s.GetSecretKeyByID(secretKey.ID)
	if err != nil {
		return err
	}

	// If this key is being set as default, ensure no other default keys exist
	if secretKey.IsDefault {
		err := s.ensureNoOtherDefaultKeys(secretKey.ID)
		if err != nil {
			return err
		}
	}

	// Set update timestamp
	secretKey.UpdatedAt = time.Now()

	filter := bson.M{"_id": secretKey.ID}
	update := bson.M{"$set": secretKey}

	result, err := s.db.Collection(secretKeysCollection).UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrSecretKeyNotFound
	}

	// Add audit log
	auditLog := &SecretKeyAuditLog{
		Username:      username,
		SecretKeyID:   secretKey.ID,
		Origin:        secretKey.Origin,
		Action:        "update",
		Timestamp:     secretKey.UpdatedAt,
		IsDefault:     secretKey.IsDefault,
		PreviousKey:   &currentKey.Key,
		NewKey:        &secretKey.Key,
		Justification: justification,
	}

	if err := s.auditStore.LogAction(auditLog); err != nil {
		// Log error but continue - failure to audit shouldn't block the main operation
		log.Printf("Error logging secret key audit event: %v", err)
	}

	return nil
}

// DisableSecretKey disables a secret key without deleting it
func (s *SecretStore) DisableSecretKey(id primitive.ObjectID, username, justification string) error {
	// Check if it's the default key
	key, err := s.GetSecretKeyByID(id)
	if err != nil {
		return err
	}

	// Don't allow disabling the default key
	if key.IsDefault {
		return ErrCannotDisableDefault
	}

	now := time.Now()
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{
		"disabled":    true,
		"disabled_at": now,
		"updated_at":  now,
	}}

	result, err := s.db.Collection(secretKeysCollection).UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrSecretKeyNotFound
	}

	// Add audit log
	auditLog := &SecretKeyAuditLog{
		Username:      username,
		SecretKeyID:   key.ID,
		Origin:        key.Origin,
		Action:        "disable",
		Timestamp:     now,
		IsDefault:     key.IsDefault,
		PreviousKey:   &key.Key,
		Justification: justification,
	}

	if err := s.auditStore.LogAction(auditLog); err != nil {
		// Log error but continue - failure to audit shouldn't block the main operation
		log.Printf("Error logging secret key audit event: %v", err)
	}

	return nil
}

// GetSecretKeyByOrigin returns the secret key for a specific origin
func (s *SecretStore) GetSecretKeyByOrigin(origin string) (*SecretKey, error) {
	filter := bson.M{"origin": origin, "disabled": false}
	var secretKey SecretKey

	err := s.db.Collection(secretKeysCollection).FindOne(context.Background(), filter).Decode(&secretKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrSecretKeyNotFound
		}
		return nil, err
	}

	return &secretKey, nil
}

// GetSecretKeyByID returns a secret key by its ID
func (s *SecretStore) GetSecretKeyByID(id primitive.ObjectID) (*SecretKey, error) {
	filter := bson.M{"_id": id}
	var secretKey SecretKey

	err := s.db.Collection(secretKeysCollection).FindOne(context.Background(), filter).Decode(&secretKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrSecretKeyNotFound
		}
		return nil, err
	}

	return &secretKey, nil
}

// GetDefaultSecretKey returns the default secret key
func (s *SecretStore) GetDefaultSecretKey() (*SecretKey, error) {
	filter := bson.M{"is_default": true, "disabled": false}
	var secretKey SecretKey

	err := s.db.Collection(secretKeysCollection).FindOne(context.Background(), filter).Decode(&secretKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrDefaultKeyNotFound
		}
		return nil, err
	}

	return &secretKey, nil
}

// GetAllSecretKeys returns all active secret keys
func (s *SecretStore) GetAllSecretKeys() ([]*SecretKey, error) {
	filter := bson.M{"disabled": false}
	opts := options.Find().SetSort(bson.D{{Key: "origin", Value: 1}})

	cursor, err := s.db.Collection(secretKeysCollection).Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var secretKeys []*SecretKey
	if err := cursor.All(context.Background(), &secretKeys); err != nil {
		return nil, err
	}

	return secretKeys, nil
}

// ensureNoOtherDefaultKeys ensures no other keys are set as default
// If exceptID is provided, that key is excluded from the check
func (s *SecretStore) ensureNoOtherDefaultKeys(exceptID ...primitive.ObjectID) error {
	filter := bson.M{"is_default": true, "disabled": false}

	// If exceptID is provided, exclude it from the check
	if len(exceptID) > 0 && !exceptID[0].IsZero() {
		filter["_id"] = bson.M{"$ne": exceptID[0]}
	}

	// Find existing default keys
	var defaultKeys []SecretKey
	cursor, err := s.db.Collection(secretKeysCollection).Find(context.Background(), filter)
	if err != nil {
		return err
	}
	defer cursor.Close(context.Background())

	if err := cursor.All(context.Background(), &defaultKeys); err != nil {
		return err
	}

	// Update all existing default keys to non-default
	if len(defaultKeys) > 0 {
		for _, key := range defaultKeys {
			updateFilter := bson.M{"_id": key.ID}
			update := bson.M{"$set": bson.M{
				"is_default": false,
				"updated_at": time.Now(),
			}}
			_, err := s.db.Collection(secretKeysCollection).UpdateOne(context.Background(), updateFilter, update)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GetSecretKeyAuditStore returns the audit store
func (s *SecretStore) GetSecretKeyAuditStore() *SecretKeyAuditStore {
	return s.auditStore
}
