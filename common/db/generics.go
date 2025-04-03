package db

import (
	"context"

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
