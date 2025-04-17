package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetDocuments finds all documents matching 'filter' in the given collection.
// 'T' is the Go struct representing the document.
// 'coll' is the *mongo.Collection you want to query.
// 'opts' optional find options (sorting, projection, etc.).
func GetDocuments[T any](ctx context.Context, coll *mongo.Collection, filter interface{}, opts ...*options.FindOptions) ([]T, error) {
	cursor, err := coll.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []T
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// If needed, you can do post-processing or handle 'cursor.Err()' again,
	// but 'cursor.All()' typically covers it.
	return results, nil
}

// GetDocument finds a single document by filter and decodes it into T.
func GetDocument[T any](ctx context.Context, coll *mongo.Collection, filter interface{}, opts ...*options.FindOneOptions) (T, error) {
	var result T
	err := coll.FindOne(ctx, filter, opts...).Decode(&result)
	if err != nil {
		return result, err
	}
	return result, nil
}

// CountDocuments returns the number of documents found in the provided collection with the possibility of applying filter or options.FindOptions
func CountDocuments(ctx context.Context, coll *mongo.Collection, filter interface{}, opts ...*options.FindOptions) (int64, error) {
	count, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// HasDocuments returns boolean regarding if a collection has documents respecting the provided filter and options
func HasDocuments(ctx context.Context, coll *mongo.Collection, filter interface{}, opts ...*options.FindOptions) bool {
	count, err := CountDocuments(ctx, coll, filter, opts...)
	if err != nil {
		return false
	}

	return count > 0
}

// UpsertDocument updates a document in the target collection respecting the provided filter
func UpsertDocument[T any](ctx context.Context, coll *mongo.Collection, filter interface{}, doc T) error {
	update := bson.M{"$set": doc}
	opts := options.Update().SetUpsert(true)

	_, err := coll.UpdateOne(ctx, filter, update, opts)
	return err
}

func UpsertDocumentWithResult[T any](ctx context.Context, coll *mongo.Collection, filter interface{}, doc T) (*mongo.UpdateResult, error) {
	update := bson.M{"$set": doc}
	opts := options.Update().SetUpsert(true)

	result, err := coll.UpdateOne(ctx, filter, update, opts)
	return result, err
}

// PartialDocumentUpdate updates only specific fields of the document that are provided via `updates` interface, usually bson.M{} maps
func PartialDocumentUpdate(ctx context.Context, coll *mongo.Collection, filter interface{}, updates interface{}, opts ...*options.UpdateOptions) error {
	update := bson.M{"$set": updates}

	_, err := coll.UpdateOne(ctx, filter, update, opts...)
	return err
}

// DeleteDocument removes a document from the target collection respecting the provided filter
func DeleteDocument(ctx context.Context, coll *mongo.Collection, filter interface{}) error {
	_, err := coll.DeleteOne(ctx, filter)
	return err
}
