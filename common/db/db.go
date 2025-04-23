package db

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStore struct {
	Client       *mongo.Client
	DatabaseName string // default database name if you want
	Ctx          context.Context
	CtxCancel    context.CancelFunc
}

var (
	GlobalMongoStore *MongoStore
	connectionString string
	maxErrorCount    = 30                                         // Maximum number of errors before logging fatal
	maxTimeout       = time.Duration(maxErrorCount) * time.Second // Maximum timeout value for server selection and socket
)

func InitMongo(dbAddress, dbName string) error {
	connectionString = fmt.Sprintf("mongodb://%s/?keepAlive=true", dbAddress)
	store, err := NewMongoStore(dbName)
	if err != nil {
		return err
	}
	GlobalMongoStore = store

	return nil
}

func NewMongoStore(dbName string) (*MongoStore, error) {
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
		cancel()
		log.Fatalf("Could not connect to Mongo server at `%s` - %s", connectionString, err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		cancel()
		return nil, err
	}

	newStore := &MongoStore{
		Client:       client,
		DatabaseName: dbName,
		Ctx:          ctx,
		CtxCancel:    cancel,
	}
	go newStore.checkDBConnection()

	return newStore, nil
}

// Close closes the Mongo connection
func (m *MongoStore) Close() error {
	m.CtxCancel()
	return m.Client.Disconnect(m.Ctx)
}

func (m *MongoStore) checkDBConnection() {
	errorCounter := 0
	for {
		ctx, cancel := context.WithTimeout(m.Ctx, 1*time.Second)
		err := m.Client.Ping(ctx, nil)
		cancel()

		if err != nil {
			errorCounter++
			log.WithFields(log.Fields{
				"error_count": errorCounter,
				"error":       err,
			}).Warn("Failed to ping MongoDB")

			if errorCounter >= maxErrorCount {
				log.Fatalf("Lost connection to MongoDB server and failed to reconnect after %d attempts!", maxErrorCount)
			}
		} else {
			if errorCounter > 0 {
				log.Info("MongoDB connection restored")
				errorCounter = 0
			}
			time.Sleep(1 * time.Second)
		}
	}
}
