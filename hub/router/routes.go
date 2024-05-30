package router

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/hub/devices"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"html/template"
	"io"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

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

func AddNewDevice(c *gin.Context) {
	var device models.Device

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	err = json.Unmarshal(payload, &device)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	err = db.UpsertDeviceDB(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert device in DB"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Added device in DB for the current provider"})
}

func getDBDevice(udid string) *models.Device {
	for _, dbDevice := range devices.LatestDevices {
		if dbDevice.UDID == udid {
			return dbDevice
		}
	}
	return nil
}

var inUseMap = make(map[string]int64)

func DeviceInUse(c *gin.Context) {
	udid := c.Param("udid")
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()

	inUseMap[udid] = time.Now().UnixMilli()

	c.String(200, "")
}

func GetDevicePage(c *gin.Context) {
	udid := c.Param("udid")

	reqDevice := getDBDevice(udid)
	if reqDevice.InUse {
		c.String(http.StatusInternalServerError, "Device is in use")
		return
	}
	// If the reqDevice does not exist in the cached devices
	if reqDevice == nil {
		c.String(http.StatusInternalServerError, "Device not found")
		return
	}

	// Create the reqDevice health URL
	url := fmt.Sprintf("http://%s/device/%s/health", reqDevice.Host, reqDevice.UDID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed creating http request to check reqDevice health from provider - %s", err.Error()))
		return
	}

	response, err := netClient.Do(req)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed performing http request to check reqDevice health from provider - %s", err.Error()))
		return
	}

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		c.String(http.StatusInternalServerError, fmt.Sprintf("Device not healthy, health check response: %s", string(body)))
		return
	}

	// Calculate the width and height for the canvas
	canvasWidth, canvasHeight := devices.CalculateCanvasDimensions(reqDevice)

	pageData := struct {
		Device       models.Device
		CanvasWidth  string
		CanvasHeight string
		ScreenHeight string
		ScreenWidth  string
	}{
		Device:       *reqDevice,
		CanvasWidth:  canvasWidth,
		CanvasHeight: canvasHeight,
		ScreenHeight: reqDevice.ScreenHeight,
		ScreenWidth:  reqDevice.ScreenWidth,
	}

	var tmpl = template.Must(template.ParseFiles("static/device_control_new.html"))
	err = tmpl.Execute(c.Writer, pageData)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}

func AvailableDevicesSSE(c *gin.Context) {
	c.Stream(func(w io.Writer) bool {
		for _, device := range devices.LatestDevices {

			if device.Connected && device.LastUpdatedTimestamp >= (time.Now().UnixMilli()-3000) {
				device.Available = true

				deviceInUseTS, ok := inUseMap[device.UDID]
				if ok {
					if deviceInUseTS >= (time.Now().UnixMilli() - 3000) {
						device.InUse = true
					}
				} else {
					device.InUse = false
					delete(inUseMap, device.UDID)
				}
				continue
			}
			device.InUse = false
			device.Available = false
		}

		jsonData, _ := json.Marshal(&devices.LatestDevices)
		c.SSEvent("", string(jsonData))
		c.Writer.Flush()
		time.Sleep(1 * time.Second)
		return true
	})
}
