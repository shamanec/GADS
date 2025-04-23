package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"GADS/common/db"
	"GADS/common/models"

	"go.mongodb.org/mongo-driver/mongo"
)

type AppiumLogger struct {
	localFile       *os.File
	mongoCollection *mongo.Collection
}

func NewAppiumLogger(logFilePath, udid string) (*AppiumLogger, error) {
	file, err := os.OpenFile(logFilePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	collection := db.GlobalMongoStore.Client.Database("appium_logs").Collection(udid)

	return &AppiumLogger{
		localFile:       file,
		mongoCollection: collection,
	}, nil
}

func (logger *AppiumLogger) Log(device *models.Device, logLine string) {
	var logData models.AppiumLog

	// If we have an actual message we will log it
	// or skip the line to not spam DB with empty entries
	// The message is after the type of log in the string like [HTTP] so we just split
	// Assuming every line has the log type formatted like this
	messageSplit := strings.Split(logLine, "] ")
	if len(messageSplit) < 2 {
		return
	}
	if messageSplit[1] == "" {
		return
	}
	logData.Message = messageSplit[1]

	// We get the timestamp provided by Appium
	// For additional info
	timestampSplit := strings.Split(logLine, " -")
	logData.AppiumTS = timestampSplit[0]

	// Set the current provider timestamp as well for additional info in case its needed(might be obsolete)
	logData.SystemTS = time.Now().UnixMilli()

	// We get the Appium log type, e.g. Appium, HTTP, XCUITestDriver
	// Its on each line in square brackets, e.g. [Appium], [HTTP]
	// Should be always there but just in case we have a fallback to `Unknown`
	re := regexp.MustCompile(`\[([^\[\]]*)]`)
	match := re.FindStringSubmatch(logLine)
	if match != nil {
		if len(match) < 2 {
			logData.Type = "Unknown"
		} else {
			logData.Type = match[1]
		}
	} else {
		logData.Type = "Unknown"
	}

	// If we have a new session created
	// Add it to the local device object
	// This way we can keep a session in UI alive when a session from outside is created, e.g. Appium Inspector, test automation
	if strings.Contains(logLine, "session created successfully") {
		sessionId := ""
		firstSplit := strings.Split(logLine, ", session ")
		if len(firstSplit) >= 2 {
			firstSplitValue := firstSplit[1]
			sessionId = strings.Split(firstSplitValue, " ")[0]
		}
		if len(sessionId) != 36 {
			device.AppiumSessionID = ""
		} else {
			device.AppiumSessionID = sessionId
		}
	}

	// If a session is being removed due to timeout or deletion
	// Remove the session ID from the local device
	if strings.Contains(logLine, "Removing session") {
		// firstSplit := strings.Split(logLine, "Removing session ")[1]
		// removedSessionId := strings.Split(firstSplit, " ")[0]
		device.AppiumSessionID = ""
	}

	// Set the log session ID to the local device session ID
	// This provides additional info as well as allows us to filter Appium logs per session
	logData.SessionID = device.AppiumSessionID

	// Log to file
	err := appiumLogToFile(logger, logData)
	if err != nil {
		fmt.Printf("Failed writing Appium log to file - %s \n Log data:\n%s\n", err, logData)
	}

	// Log to Mongo
	err = appiumLogToMongo(logger, logData)
	if err != nil {
		fmt.Printf("Failed writing Appium log to Mongo - %s \n Log data:\n%s\n", err, logData)
	}
}

func appiumLogToFile(logger *AppiumLogger, logData models.AppiumLog) error {
	jsonData, err := json.Marshal(logData)
	if err != nil {
		return err
	}

	if _, err := logger.localFile.WriteString(string(jsonData) + "\n"); err != nil {
		return err
	}

	return nil
}

func appiumLogToMongo(logger *AppiumLogger, logData models.AppiumLog) error {
	_, err := logger.mongoCollection.InsertOne(context.TODO(), logData)
	if err != nil {
		return err
	}

	return nil
}

func (logger *AppiumLogger) Close() {
	if err := logger.localFile.Close(); err != nil {
		log.Println("Error closing the log file:", err)
	}
}
