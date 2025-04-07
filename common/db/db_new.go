package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStore struct {
	Client              *mongo.Client
	DefaultDatabaseName string // default database name if you want
	Ctx                 context.Context
	CtxCancel           context.CancelFunc
}

var (
	GlobalMongoStore *MongoStore
)

func InitMongo(uri, dbName string) error {
	store, err := newMongoStore(uri, dbName)
	if err != nil {
		return err
	}

	GlobalMongoStore = store
	return nil
}

func newMongoStore(uri, dbName string) (*MongoStore, error) {
	ctx, cancel := context.WithCancel(context.Background())

	clientOptions := options.Client().
		ApplyURI(connectionString).
		SetServerSelectionTimeout(maxTimeout).
		SetConnectTimeout(5 * time.Second).
		SetSocketTimeout(maxTimeout).
		SetMaxPoolSize(200).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(5 * time.Minute)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	if err = client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return &MongoStore{
		Client:              client,
		DefaultDatabaseName: dbName,
		Ctx:                 ctx,
		CtxCancel:           cancel,
	}, nil
}

// Close closes the Mongo connection
func (m *MongoStore) Close() error {
	m.CtxCancel()
	return m.Client.Disconnect(m.Ctx)
}
