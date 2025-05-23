/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"GADS/common/db"
	"GADS/provider/config"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type CustomLogger struct {
	*log.Logger
}

var logLevelMapping = map[string]logrus.Level{
	"debug": logrus.DebugLevel,
	"info":  logrus.InfoLevel,
	"error": logrus.ErrorLevel,
}

var ProviderLogger *CustomLogger
var logLevel string

func SetupLogging(level string) {
	logLevel = level

	var err error
	fmt.Println(fmt.Sprintf("Provider will be logging to `%s/provider.log`", config.ProviderConfig.ProviderFolder))
	ProviderLogger, err = CreateCustomLogger(fmt.Sprintf("%s/provider.log", config.ProviderConfig.ProviderFolder), config.ProviderConfig.Nickname)
	if err != nil {
		log.Fatalf("Failed to create custom logger for the provider instance - %s", err)
	}
}

func (l CustomLogger) LogDebug(eventName string, message string) {
	l.WithFields(log.Fields{
		"event": eventName,
	}).Debug(message)
}

func (l CustomLogger) LogInfo(eventName string, message string) {
	l.WithFields(log.Fields{
		"event": eventName,
	}).Info(message)
}

func (l CustomLogger) LogError(eventName string, message string) {
	l.WithFields(log.Fields{
		"event": eventName,
	}).Error(message)
}

func (l CustomLogger) LogWarn(eventName string, message string) {
	l.WithFields(log.Fields{
		"event": eventName,
	}).Warn(message)
}

func (l CustomLogger) LogFatal(eventName string, message string) {
	l.WithFields(log.Fields{
		"event": eventName,
	}).Fatal(message)
}

func (l CustomLogger) LogPanic(eventName string, message string) {
	l.WithFields(log.Fields{
		"event": eventName,
	}).Panic(message)
}

func CreateCustomLogger(logFilePath, collection string) (*CustomLogger, error) {
	// Create a new logger instance
	logger := log.New()
	ctx, _ := context.WithCancel(db.GlobalMongoStore.Ctx)

	// Configure the logger
	logger.SetFormatter(&log.JSONFormatter{})
	logger.SetLevel(logLevelMapping[logLevel])

	// Open the log file
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return &CustomLogger{}, fmt.Errorf("Could not set log output - %v", err)
	}

	// Set the output to both the log file and standard output (console)
	logger.SetOutput(io.MultiWriter(logFile, os.Stdout))

	logger.AddHook(&MongoDBHook{
		Client:     db.GlobalMongoStore.Client,
		DB:         "logs",
		Collection: collection,
		Ctx:        ctx,
	})

	return &CustomLogger{Logger: logger}, nil
}

type MongoDBHook struct {
	Client     *mongo.Client
	Ctx        context.Context
	DB         string
	Collection string
}

type logEntry struct {
	Level     string
	Message   string
	Timestamp int64
	Host      string
	EventName string
}

func (hook *MongoDBHook) Fire(entry *log.Entry) error {
	fields := entry.Data

	logEntry := logEntry{
		Level:     entry.Level.String(),
		Message:   entry.Message,
		Timestamp: time.Now().UnixMilli(),
		Host:      config.ProviderConfig.Nickname,
		EventName: fields["event"].(string),
	}

	document, err := bson.Marshal(logEntry)
	if err != nil {
		fmt.Printf("Logrus MongoDB hook failed - %s\n", err)
	}

	_, err = hook.Client.Database(hook.DB).Collection(hook.Collection).InsertOne(hook.Ctx, document)
	if err != nil {
		fmt.Printf("Logrus MongoDB hook failed - %s, \nData: %s\n", err, document)
	}

	return err
}

// Levels returns the log levels at which the hook should fire
func (hook *MongoDBHook) Levels() []log.Level {
	return log.AllLevels
}
