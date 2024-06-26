package router

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/hub/devices"
	"GADS/provider/logger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
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
		user.Username = "New user"
	}

	dbUser, err := db.GetUserFromDB(user.Username)
	if err != nil && err != mongo.ErrNoDocuments {
		InternalServerError(c, "Failed checking for user in db - "+err.Error())
		return
	} else {
		fmt.Println("User does not exist, creating")
		// ADD LOGGER HERE
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
	var provider models.ProviderDB
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
	if provider.ProvideIOS {
		if provider.WdaBundleID == "" && (provider.OS == "windows" || provider.OS == "linux") {
			BadRequest(c, "Missing or invalid WebDriverAgent bundle ID")
			return
		}
		if provider.WdaRepoPath == "" && provider.OS == "darwin" {
			BadRequest(c, "Missing or invalid WebDriverAgent repo path")
			return
		}
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
	var provider models.ProviderDB
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
	if provider.ProvideIOS {
		if provider.WdaBundleID == "" && (provider.OS == "windows" || provider.OS == "linux") {
			BadRequest(c, "missing `wda_bundle_id` field")
			return
		}
		if provider.WdaRepoPath == "" && provider.OS == "darwin" {
			BadRequest(c, "missing `wda_repo_path` field")
			return
		}
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

func ProviderInfoSSE(c *gin.Context) {
	nickname := c.Param("nickname")

	c.Stream(func(w io.Writer) bool {
		providerData, _ := db.GetProviderFromDB(nickname)
		dbDevices := db.GetDBDevicesUDIDs()

		for i, connectedDevice := range providerData.ConnectedDevices {
			if slices.Contains(dbDevices, connectedDevice.UDID) {
				providerData.ConnectedDevices[i].IsConfigured = true
			}
		}

		jsonData, _ := json.Marshal(&providerData)

		c.SSEvent("", string(jsonData))
		c.Writer.Flush()
		time.Sleep(1 * time.Second)
		return true
	})
}

func DeviceInUseWS(c *gin.Context) {
	udid := c.Param("udid")

	var mu sync.Mutex
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		logger.ProviderLogger.LogError("device_in_use_ws", fmt.Sprintf("Failed upgrading device in-use websocket - %s", err))
		return
	}
	defer conn.Close()

	messageReceived := make(chan string)
	defer close(messageReceived)

	go func() {
		for {
			data, code, err := wsutil.ReadClientData(conn)
			if err != nil {
				fmt.Println(err)
				return
			}

			if code == 8 {
				close(messageReceived)
				return
			}

			if string(data) != "" {
				messageReceived <- string(data)
			}
		}
	}()

	//var timeout = time.After(2 * time.Second)
	for {
		select {
		case userName := <-messageReceived:
			mu.Lock()
			devices.HubDevicesMap[udid].InUseTS = time.Now().UnixMilli()
			devices.HubDevicesMap[udid].InUseBy = userName
			mu.Unlock()
		case <-time.After(2 * time.Second):
			mu.Lock()
			devices.HubDevicesMap[udid].InUseTS = 0
			if devices.HubDevicesMap[udid].InUseBy != "automation" {
				devices.HubDevicesMap[udid].InUseBy = ""
			}
			mu.Unlock()
			return
		}
	}
}

var availableMu sync.Mutex

func AvailableDevicesSSE(c *gin.Context) {
	c.Stream(func(w io.Writer) bool {

		availableMu.Lock()
		// Extract the keys from the map and order them
		var hubDeviceMapKeys []string
		for key := range devices.HubDevicesMap {
			hubDeviceMapKeys = append(hubDeviceMapKeys, key)
		}
		sort.Strings(hubDeviceMapKeys)

		var deviceList = []*models.LocalHubDevice{}
		for _, key := range hubDeviceMapKeys {
			deviceList = append(deviceList, devices.HubDevicesMap[key])
		}
		availableMu.Unlock()

		jsonData, _ := json.Marshal(deviceList)
		c.SSEvent("", string(jsonData))
		c.Writer.Flush()
		time.Sleep(1 * time.Second)
		return true
	})
}

func UploadSeleniumJar(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("No file provided in form data - %s", err)})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))

	if ext != ".jar" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only .jar files are accepted. Got - " + ext})
		return
	}

	openedFile, err := file.Open()
	defer openedFile.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf(fmt.Sprintf("Failed to open provided file - %s", err))})
		return
	}

	err = db.UploadFileGridFS(openedFile, "selenium.jar", true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf(fmt.Sprintf("Failed to upload file to MongoDB - %s", err))})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Selenium jar uploaded successfully"})
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

	err = db.UpsertDeviceDB(device)
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
			if reqDevice.ScreenHeight != "" && dbDevice.ScreenHeight != reqDevice.ScreenHeight {
				dbDevice.ScreenHeight = reqDevice.ScreenHeight
			}
			if reqDevice.ScreenWidth != "" && dbDevice.ScreenWidth != reqDevice.ScreenWidth {
				dbDevice.ScreenWidth = reqDevice.ScreenWidth
			}
			if reqDevice.OSVersion != "" && dbDevice.OSVersion != reqDevice.OSVersion {
				dbDevice.OSVersion = reqDevice.OSVersion
			}
			if reqDevice.Name != "" && reqDevice.Name != dbDevice.Name {
				dbDevice.Name = reqDevice.Name
			}
			err = db.UpsertDeviceDB(dbDevice)
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

	err = db.DeleteDeviceDB(device.UDID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete device from DB - %s", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Successfully deleted device with udid `%s` from DB", device.UDID)})
}

type AdminDeviceData struct {
	Devices   []models.Device `json:"devices"`
	Providers []string        `json:"providers"`
}

func GetDevices(c *gin.Context) {
	dbDevices := db.GetDBDeviceNew()
	providers := db.GetProvidersFromDB()

	var providerNames []string
	for _, provider := range providers {
		providerNames = append(providerNames, provider.Nickname)
	}

	if len(dbDevices) == 0 {
		dbDevices = []models.Device{}
	}

	var adminDeviceData = AdminDeviceData{
		Devices:   dbDevices,
		Providers: providerNames,
	}

	c.JSON(http.StatusOK, adminDeviceData)
}
