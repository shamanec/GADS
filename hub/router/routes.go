package router

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/hub/devices"
	"GADS/provider/logger"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

type AppiumLog struct {
	TS        int64  `json:"ts" bson:"ts"`
	Message   string `json:"msg" bson:"msg"`
	AppiumTS  string `json:"appium_ts" bson:"appium_ts"`
	LogType   string `json:"log_type" bson:"log_type"`
	SessionID string `json:"session_id" bson:"session_id"`
}

type ProviderLog struct {
	EventName string `json:"eventname" bson:"eventname"`
	Level     string `json:"level" bson:"level"`
	Message   string `json:"message" bson:"message"`
	Timestamp int64  `json:"timestamp" bson:"timestamp"`
}

func GetAppiumLogs(c *gin.Context) {
	logLimit, _ := strconv.Atoi(c.DefaultQuery("logLimit", "100"))
	if logLimit > 1000 {
		logLimit = 1000
	}

	collectionName := c.DefaultQuery("collection", "")
	if collectionName == "" {
		BadRequest(c, "Empty collection name provided")
		return
	}

	var logs []AppiumLog

	collection := db.MongoClient().Database("appium_logs").Collection(collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "ts", Value: -1}})
	findOptions.SetLimit(int64(logLimit))

	cursor, err := collection.Find(db.MongoCtx(), bson.D{{}}, findOptions)
	if err != nil {
		InternalServerError(c, "Failed to get cursor for collection")
	}
	defer cursor.Close(db.MongoCtx())

	if err := cursor.All(db.MongoCtx(), &logs); err != nil {
		InternalServerError(c, "Failed to read data from cursor")
	}
	if err := cursor.Err(); err != nil {
		InternalServerError(c, "Cursor error")
	}

	c.JSON(200, logs)
}

func GetProviderLogs(c *gin.Context) {
	logLimit, _ := strconv.Atoi(c.DefaultQuery("logLimit", "200"))
	if logLimit > 1000 {
		logLimit = 1000
	}

	collectionName := c.DefaultQuery("collection", "")
	if collectionName == "" {
		BadRequest(c, "Empty collection name provided")
		return
	}

	var logs []ProviderLog

	collection := db.MongoClient().Database("logs").Collection(collectionName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	findOptions.SetLimit(int64(logLimit))

	cursor, err := collection.Find(db.MongoCtx(), bson.D{{}}, findOptions)
	if err != nil {
		InternalServerError(c, "Failed to get cursor for collection")
	}
	defer cursor.Close(db.MongoCtx())

	if err := cursor.All(db.MongoCtx(), &logs); err != nil {
		InternalServerError(c, "Failed to read data from cursor")
	}
	if err := cursor.Err(); err != nil {
		InternalServerError(c, "Cursor error")
	}

	c.JSON(200, logs)
}

func GetAppiumSessionLogs(c *gin.Context) {
	var logs []AppiumLog

	collectionName := c.DefaultQuery("collection", "")
	if collectionName == "" {
		BadRequest(c, "Empty collection name provided")
		return
	}

	sessionID := c.DefaultQuery("session", "")
	if sessionID == "" {
		BadRequest(c, "Empty Appium session ID provided")
		return
	}

	collection := db.MongoClient().Database("appium_logs").Collection(collectionName)

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "ts", Value: -1}})
	filter := bson.D{{"session_id", sessionID}}

	cursor, err := collection.Find(db.MongoCtx(), filter, findOptions)
	if err != nil {
		InternalServerError(c, "Failed to get cursor for collection")
	}
	defer cursor.Close(db.MongoCtx())

	if err := cursor.All(db.MongoCtx(), &logs); err != nil {
		InternalServerError(c, "Failed to read data from cursor")
	}
	if err := cursor.Err(); err != nil {
		InternalServerError(c, "Cursor error")
	}

	c.JSON(200, logs)
}

func AddUser(c *gin.Context) {
	var user models.User

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("%s", err))
		return
	}

	err = json.Unmarshal(body, &user)
	if err != nil {
		BadRequest(c, fmt.Sprintf("%s", err))
		return
	}

	if user == (models.User{}) {
		BadRequest(c, "Empty or invalid body")
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		BadRequest(c, "Invalid role - `admin` and `user` are the accepted values")
		return
	}

	if user.Username == "" {
		BadRequest(c, "Empty username provided")
	}

	if user.Password == "" {
		BadRequest(c, "Empty password provided")
	}

	dbUser, err := db.GetUserFromDB(user.Username)
	if err != nil && err != mongo.ErrNoDocuments {
		InternalServerError(c, "Failed checking for user in db - "+err.Error())
		return
	}

	if dbUser != (models.User{}) {
		BadRequest(c, "User already exists")
		return
	}

	err = db.AddOrUpdateUser(user)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed adding/updating user - %s", err))
		return
	}

	OK(c, "Successfully added user")
}

func UpdateUser(c *gin.Context) {
	var user models.User

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("%s", err))
		return
	}

	err = json.Unmarshal(body, &user)
	if err != nil {
		BadRequest(c, fmt.Sprintf("%s", err))
		return
	}

	if user == (models.User{}) {
		BadRequest(c, "Empty or invalid body")
		return
	}

	dbUser, err := db.GetUserFromDB(user.Username)
	if err != nil && err != mongo.ErrNoDocuments {
		InternalServerError(c, "Failed checking for user in db - "+err.Error())
		return
	}

	if dbUser == (models.User{}) {
		BadRequest(c, "Cannot update non-existing user")
		return
	}

	if user.Password == "" {
		user.Password = dbUser.Password
	}

	err = db.AddOrUpdateUser(user)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed adding/updating user - %s", err))
		return
	}
}

func DeleteUser(c *gin.Context) {
	nickname := c.Param("nickname")

	err := db.DeleteUserDB(nickname)
	if err != nil {
		InternalServerError(c, "Failed to delete user - "+err.Error())
		return
	}

	OK(c, "Successfully deleted user")
}

func GetProviders(c *gin.Context) {
	providers := db.GetProvidersFromDB()
	if len(providers) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}
	OkJSON(c, providers)
}

func GetProviderInfo(c *gin.Context) {
	providerName := c.Param("name")
	providers := db.GetProvidersFromDB()
	for _, provider := range providers {
		if provider.Nickname == providerName {
			c.JSON(http.StatusOK, provider)
			return
		}
	}
	NotFound(c, fmt.Sprintf("No provider with name `%s` found", providerName))
}

func AddProvider(c *gin.Context) {
	var provider models.Provider
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("%s", err))
		return
	}

	err = json.Unmarshal(body, &provider)
	if err != nil {
		BadRequest(c, fmt.Sprintf("%s", err))
		return
	}

	// Validations
	if provider.Nickname == "" {
		BadRequest(c, "Missing or invalid nickname")
		return
	}
	providerDB, _ := db.GetProviderFromDB(provider.Nickname)
	if providerDB.Nickname == provider.Nickname {
		BadRequest(c, "Provider with this nickname already exists")
		return
	}

	if provider.OS == "" {
		BadRequest(c, "Missing or invalid OS")
		return
	}
	if provider.HostAddress == "" {
		BadRequest(c, "Missing or invalid host address")
		return
	}
	if provider.Port == 0 {
		BadRequest(c, "Missing or invalid port")
		return
	}
	if provider.UseSeleniumGrid && provider.SeleniumGrid == "" {
		BadRequest(c, "Missing or invalid Selenium Grid address")
		return
	}

	err = db.AddOrUpdateProvider(provider)
	if err != nil {
		InternalServerError(c, "Could not create provider")
		return
	}

	providersDB := db.GetProvidersFromDB()
	OkJSON(c, providersDB)
}

func UpdateProvider(c *gin.Context) {
	var provider models.Provider
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("%s", err))
		return
	}

	err = json.Unmarshal(body, &provider)
	if err != nil {
		BadRequest(c, fmt.Sprintf("%s", err))
		return
	}

	// Validations
	if provider.Nickname == "" {
		BadRequest(c, "missing `nickname` field")
		return
	}
	if provider.OS == "" {
		BadRequest(c, "missing `os` field")
		return
	}
	if provider.HostAddress == "" {
		BadRequest(c, "missing `host_address` field")
		return
	}
	if provider.Port == 0 {
		BadRequest(c, "missing `port` field")
		return
	}
	if provider.UseSeleniumGrid && provider.SeleniumGrid == "" {
		BadRequest(c, "missing `selenium_grid` field")
		return
	}

	err = db.AddOrUpdateProvider(provider)
	if err != nil {
		InternalServerError(c, "Could not update provider")
		return
	}
	OK(c, "Provider updated successfully")
}

func DeleteProvider(c *gin.Context) {
	nickname := c.Param("nickname")

	err := db.DeleteProviderDB(nickname)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete provider from DB - %s", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Successfully deleted provider with nickname `%s` from DB", nickname)})
}

func ProviderInfoSSE(c *gin.Context) {
	nickname := c.Param("nickname")

	c.Stream(func(w io.Writer) bool {
		providerData, _ := db.GetProviderFromDB(nickname)

		jsonData, _ := json.Marshal(&providerData)

		c.SSEvent("", string(jsonData))
		c.Writer.Flush()
		time.Sleep(1 * time.Second)
		return true
	})
}

type WebRTCMessage struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

// Websocket signalling server for WebRTC
func DeviceWebRTCWS(c *gin.Context) {
	udid := c.Param("udid")

	// Accept the connection from the React UI
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		logger.ProviderLogger.LogError("device_in_use_ws", fmt.Sprintf("Failed upgrading device in-use websocket - %s", err))
		return
	}

	// Get the target device UDID
	devices.HubDevicesData.Mu.Lock()
	var deviceTest = devices.HubDevicesData.Devices[udid]
	devices.HubDevicesData.Mu.Unlock()

	// Connect to the respective device WebRTC signalling server websocket on the provider
	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s", deviceTest.Device.Host), Path: "/device/" + deviceTest.Device.UDID + "/webrtc"}
	providerConn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		log.Printf("Failed to dial provider signalling server websocket for device `%s` - %s\n", udid, err)
		return
	}
	defer providerConn.Close()

	go func() {
		for {
			msg, op, err := wsutil.ReadServerData(providerConn)
			if err != nil {
				log.Printf("WebRTC signalling webserver for device `%s` on provider `%s` disconnected - %s\n", udid, deviceTest.Device.Host, err)
				return
			}
			log.Printf("Received WebRTC message from provider signalling server for device `%s`, sending to hub UI - %s\n", udid, string(msg))
			err = wsutil.WriteServerMessage(conn, op, msg)
			if err != nil {
				log.Printf("Failed to write WebRTC message from provider signalling server to hub UI client for device `%s` - %s\n", udid, err)
				return
			}
		}
	}()

	for {
		msg, op, err := wsutil.ReadClientData(conn)
		if err != nil {
			log.Printf("Hub UI client for device `%s` disconnected, sending hangup message to provider signalling server - %s\n", udid, err)
			err = wsutil.WriteClientMessage(providerConn, op, []byte("hangup"))
			if err != nil {
				log.Printf("Failed to send hangup signal to provider signalling server for device `%s` - %s\n", udid, err)
			}
			return
		}
		log.Printf("Received WebRTC message from hub UI client for device `%s`, sending to provider signalling server - %s\n", udid, string(msg))

		var message WebRTCMessage
		err = json.Unmarshal(msg, &message)
		if err != nil {
			log.Printf("Failed to unmarshal WebRTC message from hub UI client for device `%s`, sending hangup message to provider signalling server - %s\n", udid, err)
			err = wsutil.WriteClientMessage(providerConn, op, []byte("hangup"))
			if err != nil {
				log.Printf("Failed to send hangup signal to provider signalling server for device `%s` - %s\n", udid, err)
			}
			return
		}
		switch message.Type {
		case "offer":
			log.Printf("Received an WebRTC offer from hub UI client for device `%s`, sending to provider signalling server\n", udid)
			err = wsutil.WriteClientMessage(providerConn, op, msg)
			if err != nil {
				log.Printf("Failed to send hub UI WebRTC offer for device `%s` to provider signalling server - %s\n", udid, err)
			}
			break
		case "candidate":
			err = wsutil.WriteClientMessage(providerConn, op, msg)
			if err != nil {
				log.Printf("Failed to send hub UI WebRTC ICE candidate for device `%s` to provider signalling server - %s\n", udid, err)
			}
			break
		}
	}
}

// This websocket connection is used to both set the device in use when remotely controlled
// As well as send live updates when needed - device info, release device, etc
func DeviceInUseWS(c *gin.Context) {
	udid := c.Param("udid")

	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		logger.ProviderLogger.LogError("device_in_use_ws", fmt.Sprintf("Failed upgrading device in-use websocket - %s", err))
		return
	}

	// Add the created connection to the respective device in the map
	// So we can send different messages to it from other sources
	devices.HubDevicesData.Mu.Lock()
	devices.HubDevicesData.Devices[udid].InUseWSConnection = conn
	devices.HubDevicesData.Mu.Unlock()

	// If this function returns then we close the connection
	// And also set it to nil for the respective device in the map
	defer func() {
		conn.Close()
		devices.HubDevicesData.Mu.Lock()
		devices.HubDevicesData.Devices[udid].InUseWSConnection = nil
		devices.HubDevicesData.Mu.Unlock()
	}()

	// Create a context with cancel to use in the goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a channel to send data from the goroutine listening for messages coming from the client(UI)
	// And then we receive from the channel in the for loop that sets the device in-use status
	messageReceived := make(chan string)
	defer close(messageReceived)

	// Loop getting messages from the client
	// To keep device in use
	go func() {
		for {
			select {
			case <-ctx.Done(): // If context was cancelled in the other goroutine or the for loop we return to stop the current goroutine
				return
			default:
				data, code, err := wsutil.ReadClientData(conn)
				if err != nil || code == 8 { // 8 is close code
					cancel() // Trigger context cancellation for all goroutines and the for loop if we got an error from the websocket/client and return to stop the current goroutine
					return
				}

				// If we got any data from the client that is not an empty string - this is the nickname of the person using the device
				// So we send it to the messageReceived channel
				if string(data) != "" {
					// Check if device is currently being used by someone
					if devices.HubDevicesData.Devices[udid].InUseBy != "automation" {
						// If it is being used check if the any action was performed in the last 30 minutes
						if (time.Now().UnixMilli() - devices.HubDevicesData.Devices[udid].LastActionTS) > (1800 * 1000) {
							// Send to the websocket a message that the session expired
							sessionExpiredMessage := models.DeviceInUseMessage{
								Type: "sessionExpired",
							}
							sessionExpiredJson, _ := json.Marshal(sessionExpiredMessage)
							wsutil.WriteServerText(conn, sessionExpiredJson)
							// Update the hub device to no longer be in use
							devices.HubDevicesData.Mu.Lock()
							devices.HubDevicesData.Devices[udid].InUseTS = 0
							devices.HubDevicesData.Devices[udid].InUseBy = ""
							devices.HubDevicesData.Mu.Unlock()
							// Cancel the current websocket goroutines and stuff
							cancel()
							return
						}
					}
					messageReceived <- string(data)
				}
			}
		}
	}()

	// Loop sending messages to client to keep the connection - like ping/pong
	go func() {
		for {
			select {
			case <-ctx.Done(): // If context was cancelled in the other goroutine or the for loop we return to stop the current goroutine
				return
			default:
				// Send a ping message to the client(UI) using the DeviceInUseMessage struct as json string
				deviceInUseMessage := models.DeviceInUseMessage{
					Type: "ping",
				}
				deviceInUseMessageJson, _ := json.Marshal(deviceInUseMessage)
				err := wsutil.WriteServerText(conn, deviceInUseMessageJson)
				if err != nil {
					cancel() // Trigger context cancellation for all goroutines and the for loop if we got an error from the websocket/client and return to stop the current goroutine
					return
				}
				// Wait 1 second between pings
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Create a new timer for the loop below
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	// We loop over the messageReceived channel, timer and the context cancellation signal
	for {
		select {
		// If a message is received over the channel with the username of the user occupying the device
		// We set the last timestamp for in use and set the name of the person using it
		// We reset the timer each time a message was received
		case userName := <-messageReceived:
			devices.HubDevicesData.Mu.Lock()
			devices.HubDevicesData.Devices[udid].InUseTS = time.Now().UnixMilli()
			devices.HubDevicesData.Devices[udid].InUseBy = userName
			devices.HubDevicesData.Mu.Unlock()
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(2 * time.Second)
		// If the timer limit is reached and the device is not in use by automation but a person
		// We reset the in use timestamp
		// And remove the name of the person that was using it
		case <-timer.C:
			devices.HubDevicesData.Mu.Lock()
			devices.HubDevicesData.Devices[udid].InUseTS = 0
			if devices.HubDevicesData.Devices[udid].InUseBy != "automation" {
				devices.HubDevicesData.Devices[udid].InUseBy = ""
			}
			devices.HubDevicesData.Mu.Unlock()
			return
		// If the context was cancelled from the read/write goroutines
		// We return to exit the loop
		case <-ctx.Done():
			return
		}
	}
}

func AvailableDevicesSSE(c *gin.Context) {
	c.Stream(func(w io.Writer) bool {

		devices.HubDevicesData.Mu.Lock()
		// Extract the keys from the map and order them
		var hubDeviceMapKeys []string
		for key := range devices.HubDevicesData.Devices {
			hubDeviceMapKeys = append(hubDeviceMapKeys, key)
		}
		sort.Strings(hubDeviceMapKeys)

		var deviceList = []*models.LocalHubDevice{}
		for _, key := range hubDeviceMapKeys {
			if devices.HubDevicesData.Devices[key].Device.LastUpdatedTimestamp < (time.Now().UnixMilli()-3000) && devices.HubDevicesData.Devices[key].Device.Connected {
				devices.HubDevicesData.Devices[key].Available = false
			} else if devices.HubDevicesData.Devices[key].Device.ProviderState != "live" {
				devices.HubDevicesData.Devices[key].Available = false
			} else {
				devices.HubDevicesData.Devices[key].Available = true
			}
			if devices.HubDevicesData.Devices[key].InUseTS > (time.Now().UnixMilli() - 3000) {
				if !devices.HubDevicesData.Devices[key].InUse {
					devices.HubDevicesData.Devices[key].InUse = true
				}
			} else {
				if devices.HubDevicesData.Devices[key].InUse {
					devices.HubDevicesData.Devices[key].InUse = false
				}
			}
			deviceList = append(deviceList, devices.HubDevicesData.Devices[key])
		}
		devices.HubDevicesData.Mu.Unlock()

		jsonData, _ := json.Marshal(deviceList)
		c.SSEvent("", string(jsonData))
		c.Writer.Flush()
		time.Sleep(1 * time.Second)
		return true
	})
}

// Custom upload function that allows us to upload any file to Mongo
// While providing the file name we want to use on upload regardless of the actual file name
func UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("No file provided in form data - %s", err)})
		return
	}
	fileName := c.PostForm("fileName")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fileName for MongoDB record was provided"})
		return
	}
	extension := c.PostForm("extension")
	if extension == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No expected extension was provided"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != extension {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Expected extension is `%s` but you provided file with `%s`", extension, ext)})
		return
	}

	openedFile, err := file.Open()
	defer openedFile.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf(fmt.Sprintf("Failed to open provided file - %s", err))})
		return
	}

	err = db.UploadFileGridFS(openedFile, fmt.Sprintf("%s", fileName), true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf(fmt.Sprintf("Failed to upload file to MongoDB - %s", err))})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("`%s` uploaded successfully", file.Filename)})
}

func AddDevice(c *gin.Context) {
	reqBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read request body - %s", err)})
		return
	}
	defer c.Request.Body.Close()

	var device models.Device
	err = json.Unmarshal(reqBody, &device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to unmarshal request body to struct - %s", err)})
		return
	}

	dbDevices := db.GetDBDeviceNew()
	for _, dbDevice := range dbDevices {
		if dbDevice.UDID == device.UDID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Device already exists in the DB"})
			return
		}
	}

	err = db.UpsertDeviceDB(&device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert device in DB"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Added device in DB"})
}

func UpdateDevice(c *gin.Context) {
	reqBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read request body - %s", err)})
		return
	}
	defer c.Request.Body.Close()

	var reqDevice models.Device
	err = json.Unmarshal(reqBody, &reqDevice)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to unmarshal request body to struct - %s", err)})
		return
	}

	dbDevices := db.GetDBDeviceNew()
	for _, dbDevice := range dbDevices {
		if dbDevice.UDID == reqDevice.UDID {
			// Update only the relevant data and only if something has changed
			if dbDevice.Provider != reqDevice.Provider {
				dbDevice.Provider = reqDevice.Provider
			}
			if reqDevice.OS != "" && dbDevice.OS != reqDevice.OS {
				dbDevice.OS = reqDevice.OS
			}
			if dbDevice.ScreenHeight != reqDevice.ScreenHeight {
				dbDevice.ScreenHeight = reqDevice.ScreenHeight
			}

			if dbDevice.ScreenWidth != reqDevice.ScreenWidth {
				dbDevice.ScreenWidth = reqDevice.ScreenWidth
			}
			if reqDevice.OSVersion != "" && dbDevice.OSVersion != reqDevice.OSVersion {
				dbDevice.OSVersion = reqDevice.OSVersion
			}
			if reqDevice.Name != "" && reqDevice.Name != dbDevice.Name {
				dbDevice.Name = reqDevice.Name
			}

			if reqDevice.Usage != "" && reqDevice.Usage != dbDevice.Usage {
				dbDevice.Usage = reqDevice.Usage
			}
			if reqDevice.UseWebRTCVideo != dbDevice.UseWebRTCVideo {
				dbDevice.UseWebRTCVideo = reqDevice.UseWebRTCVideo
			}
			if reqDevice.WebRTCVideoCodec != dbDevice.WebRTCVideoCodec {
				dbDevice.WebRTCVideoCodec = reqDevice.WebRTCVideoCodec
			}
			err = db.UpsertDeviceDB(&dbDevice)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert device in DB"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Successfully updated device in DB"})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Device with udid `%s` does not exist in the DB", reqDevice.UDID)})
}

func DeleteDevice(c *gin.Context) {
	udid := c.Param("udid")

	err := db.DeleteDeviceDB(udid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete device from DB - %s", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Successfully deleted device with udid `%s` from DB", udid)})
}

type AdminDeviceData struct {
	Devices   []models.Device `json:"devices"`
	Providers []string        `json:"providers"`
}

func GetDevices(c *gin.Context) {
	dbDevices := db.GetDBDeviceNew()
	providers := db.GetProvidersFromDB()

	var providerNames []string = []string{}
	for _, provider := range providers {
		providerNames = append(providerNames, provider.Nickname)
	}

	if len(dbDevices) == 0 || len(providerNames) == 0 {
		dbDevices = []models.Device{}
	}

	var adminDeviceData = AdminDeviceData{
		Devices:   dbDevices,
		Providers: providerNames,
	}

	c.JSON(http.StatusOK, adminDeviceData)
}

func ReleaseUsedDevice(c *gin.Context) {
	udid := c.Param("udid")

	// Send a release device message on the device in use websocket connection
	deviceInUseMessage := models.DeviceInUseMessage{
		Type: "releaseDevice",
	}
	deviceInUseMessageJson, _ := json.Marshal(deviceInUseMessage)

	devices.HubDevicesData.Mu.Lock()
	defer devices.HubDevicesData.Mu.Unlock()
	err := wsutil.WriteServerText(devices.HubDevicesData.Devices[udid].InUseWSConnection, deviceInUseMessageJson)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to send release device message - " + err.Error()})
		return
	}

	devices.HubDevicesData.Devices[udid].InUseWSConnection.Close()
	devices.HubDevicesData.Devices[udid].InUseTS = 0
	devices.HubDevicesData.Devices[udid].InUseBy = ""

	c.JSON(200, gin.H{"message": "Message to release device was successfully sent"})
}

func ProviderUpdate(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	defer c.Request.Body.Close()
	if err != nil {
		// handle error if needed
	}

	var providerDeviceData models.ProviderData

	err = json.Unmarshal(bodyBytes, &providerDeviceData)
	if err != nil {
		// handle error if needed
	}

	for _, providerDevice := range providerDeviceData.DeviceData {
		devices.HubDevicesData.Mu.Lock()
		hubDevice, ok := devices.HubDevicesData.Devices[providerDevice.UDID]
		if ok {
			// If device is not connected reset all fields that might allow it to get stuck in Running automation state
			// If its not connected, then its not running automation or is available for automation
			if !providerDevice.Connected {
				hubDevice.IsAvailableForAutomation = false
				hubDevice.IsRunningAutomation = false
				hubDevice.InUseBy = ""
				hubDevice.SessionID = ""
				devices.HubDevicesData.Mu.Unlock()
				continue
			}
			// Set a timestamp to indicate last time info about the device was updated from the provider
			providerDevice.LastUpdatedTimestamp = time.Now().UnixMilli()

			// Check all DB related values so if you make a change in the DB for a device
			// The provider pushing updates will not overwrite with something wrong
			if providerDevice.Usage != hubDevice.Device.Usage {
				providerDevice.Usage = hubDevice.Device.Usage
			}
			if providerDevice.Name != hubDevice.Device.Name {
				providerDevice.Name = hubDevice.Device.Name
			}
			if providerDevice.OSVersion != hubDevice.Device.OSVersion {
				providerDevice.OSVersion = hubDevice.Device.OSVersion
			}
			if providerDevice.ScreenWidth != hubDevice.Device.ScreenWidth {
				providerDevice.ScreenWidth = hubDevice.Device.ScreenWidth
			}
			if providerDevice.ScreenHeight != hubDevice.Device.ScreenHeight {
				providerDevice.ScreenHeight = hubDevice.Device.ScreenHeight
			}
			if providerDevice.Provider != hubDevice.Device.Provider {
				providerDevice.Provider = hubDevice.Device.Provider
			}

			hubDevice.Device = providerDevice
		}
		devices.HubDevicesData.Mu.Unlock()
	}

	c.JSON(http.StatusOK, gin.H{})
}

func GetUsers(c *gin.Context) {
	users := db.GetUsers()
	// Clean up the passwords, not that the project is very secure but let's not send them
	for i := range users {
		users[i].Password = ""
	}

	c.JSON(http.StatusOK, users)
}

func GetFiles(c *gin.Context) {
	files := db.GetDBFiles()

	c.JSON(http.StatusOK, files)
}

func DownloadResourceFromGithubRepo(c *gin.Context) {
	fileName := c.Query("fileName")
	fmt.Println("Filename " + fileName)

	// Create the file
	out, err := os.Create(fileName)
	if err != nil {
		c.String(http.StatusInternalServerError, "Create"+err.Error())
		return
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(fmt.Sprintf("https://raw.githubusercontent.com/shamanec/GADS/wda-signing/resources/%s", fileName))
	if err != nil {
		c.String(http.StatusInternalServerError, "Get"+err.Error())
		return
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		c.String(resp.StatusCode, "Statuscode"+err.Error())
		return
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
}

func GetGlobalStreamSettings(c *gin.Context) {
	// Retrieve global stream settings from the database
	streamSettings, err := db.GetGlobalStreamSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve global stream settings"})
		return
	}

	// Return the stream settings as a JSON response
	c.JSON(http.StatusOK, streamSettings)
}

func UpdateGlobalStreamSettings(c *gin.Context) {
	var settings models.StreamSettings

	// Bind the JSON input to the settings struct
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	err := db.UpdateGlobalStreamSettings(settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}
