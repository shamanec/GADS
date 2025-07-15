/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/hub/auth"
	"GADS/hub/devices"
	"GADS/provider/logger"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

// HealthCheck godoc
// @Summary      Health check endpoint
// @Description  Check if the GADS hub is running and healthy
// @Tags         System
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.HealthResponse
// @Security     BearerAuth
// @Router       /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// GetAppiumLogs godoc
// @Summary      Get Appium logs
// @Description  Retrieve Appium logs from a specific collection with optional limit
// @Tags         Logs
// @Accept       json
// @Produce      json
// @Param        collection  query     string  true   "Collection name"
// @Param        logLimit    query     int     false  "Log limit (max 1000, default 100)"
// @Success      200         {array}   models.LogEntry
// @Failure      400         {object}  models.ErrorResponse
// @Failure      500         {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /appium-logs [get]
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

	logs, err := db.GlobalMongoStore.GetAppiumLogs(collectionName, logLimit)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed to get logs - %s", err))
	}

	c.JSON(200, logs)
}

// GetProviderLogs godoc
// @Summary      Get provider logs
// @Description  Retrieve provider logs from a specific collection with optional limit
// @Tags         Logs
// @Accept       json
// @Produce      json
// @Param        collection  query     string  true   "Collection name"
// @Param        logLimit    query     int     false  "Log limit (max 1000, default 200)"
// @Success      200         {array}   models.LogEntry
// @Failure      400         {object}  models.ErrorResponse
// @Failure      500         {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/providers/logs [get]
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

	logs, err := db.GlobalMongoStore.GetProviderLogs(collectionName, logLimit)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed to get logs - %s", err))
		return
	}

	c.JSON(200, logs)
}

// GetAppiumSessionLogs godoc
// @Summary      Get Appium session logs
// @Description  Retrieve Appium logs for a specific session
// @Tags         Logs
// @Accept       json
// @Produce      json
// @Param        collection  query     string  true  "Collection name"
// @Param        session     query     string  true  "Appium session ID"
// @Success      200         {array}   models.LogEntry
// @Failure      400         {object}  models.ErrorResponse
// @Failure      500         {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /appium-session-logs [get]
func GetAppiumSessionLogs(c *gin.Context) {
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

	logs, err := db.GlobalMongoStore.GetAppiumSessionLogs(collectionName, sessionID)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed to get logs - %s", err))
	}

	c.JSON(200, logs)
}

// AddUser godoc
// @Summary      Add a new user
// @Description  Create a new user in the system
// @Tags         Admin - Users
// @Accept       json
// @Produce      json
// @Param        user  body      models.User  true  "User data"
// @Success      200   {object}  models.SuccessResponse
// @Failure      400   {object}  models.ErrorResponse
// @Failure      500   {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/user [post]
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

	if user.Username == "" || user.Password == "" || (user.Role == "user" && len(user.WorkspaceIDs) == 0) {
		BadRequest(c, "Empty or invalid body")
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		BadRequest(c, "Invalid role - `admin` and `user` are the accepted values")
		return
	}

	dbUser, err := db.GlobalMongoStore.GetUser(user.Username)
	if err != nil && err != mongo.ErrNoDocuments {
		InternalServerError(c, "Failed checking for user in db - "+err.Error())
		return
	}

	if dbUser.Username != "" {
		BadRequest(c, "User already exists")
		return
	}

	err = db.GlobalMongoStore.AddOrUpdateUser(user)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed adding/updating user - %s", err))
		return
	}

	OK(c, "Successfully added user")
}

// UpdateUser godoc
// @Summary      Update an existing user
// @Description  Update user information in the system
// @Tags         Admin - Users
// @Accept       json
// @Produce      json
// @Param        user  body      models.User  true  "User data"
// @Success      200   {object}  models.SuccessResponse
// @Failure      400   {object}  models.ErrorResponse
// @Failure      500   {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/user [put]
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

	if user.Username == "" || (user.Role == "user" && len(user.WorkspaceIDs) == 0) {
		BadRequest(c, "Username cannot be empty and non-admin users must have at least one workspace")
		return
	}

	dbUser, err := db.GlobalMongoStore.GetUser(user.Username)
	if err != nil && err != mongo.ErrNoDocuments {
		InternalServerError(c, "Failed checking for user in db - "+err.Error())
		return
	}

	if dbUser.Username == "" {
		BadRequest(c, "Cannot update non-existing user")
		return
	}

	err = db.GlobalMongoStore.AddOrUpdateUser(user)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed adding/updating user - %s", err))
		return
	}
}

// DeleteUser godoc
// @Summary      Delete a user
// @Description  Remove a user from the system
// @Tags         Admin - Users
// @Accept       json
// @Produce      json
// @Param        nickname  path      string  true  "User nickname"
// @Success      200       {object}  models.SuccessResponse
// @Failure      500       {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/user/{nickname} [delete]
func DeleteUser(c *gin.Context) {
	nickname := c.Param("nickname")

	err := db.GlobalMongoStore.DeleteUser(nickname)
	if err != nil {
		InternalServerError(c, "Failed to delete user - "+err.Error())
		return
	}

	OK(c, "Successfully deleted user")
}

// GetProviders godoc
// @Summary      Get all providers
// @Description  Retrieve list of all providers in the system
// @Tags         Admin - Providers
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.Provider
// @Security     BearerAuth
// @Router       /admin/providers [get]
func GetProviders(c *gin.Context) {
	providers, _ := db.GlobalMongoStore.GetAllProviders()
	if len(providers) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}
	OkJSON(c, providers)
}

func GetProviderInfo(c *gin.Context) {
	providerName := c.Param("name")
	providers, _ := db.GlobalMongoStore.GetAllProviders()
	for _, provider := range providers {
		if provider.Nickname == providerName {
			c.JSON(http.StatusOK, provider)
			return
		}
	}
	NotFound(c, fmt.Sprintf("No provider with name `%s` found", providerName))
}

// AddProvider godoc
// @Summary      Add a new provider
// @Description  Create a new provider in the system
// @Tags         Admin - Providers
// @Accept       json
// @Produce      json
// @Param        provider  body      models.Provider  true  "Provider data"
// @Success      200       {array}   models.Provider
// @Failure      400       {object}  models.ErrorResponse
// @Failure      500       {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/providers/add [post]
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
	providerDB, _ := db.GlobalMongoStore.GetProvider(provider.Nickname)
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

	provider.RegularizeProviderState()

	err = db.GlobalMongoStore.AddOrUpdateProvider(provider)
	if err != nil {
		InternalServerError(c, "Could not create provider")
		return
	}

	providersDB, _ := db.GlobalMongoStore.GetAllProviders()
	OkJSON(c, providersDB)
}

// UpdateProvider godoc
// @Summary      Update a provider
// @Description  Update an existing provider in the system
// @Tags         Admin - Providers
// @Accept       json
// @Produce      json
// @Param        provider  body      models.Provider  true  "Provider data"
// @Success      200       {object}  models.SuccessResponse
// @Failure      400       {object}  models.ErrorResponse
// @Failure      500       {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/providers/update [post]
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

	provider.RegularizeProviderState()

	err = db.GlobalMongoStore.AddOrUpdateProvider(provider)
	if err != nil {
		InternalServerError(c, "Could not update provider")
		return
	}
	OK(c, "Provider updated successfully")
}

// DeleteProvider godoc
// @Summary      Delete a provider
// @Description  Remove a provider from the system
// @Tags         Admin - Providers
// @Accept       json
// @Produce      json
// @Param        nickname  path      string  true  "Provider nickname"
// @Success      200       {object}  models.SuccessResponse
// @Failure      500       {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/providers/{nickname} [delete]
func DeleteProvider(c *gin.Context) {
	nickname := c.Param("nickname")

	err := db.GlobalMongoStore.DeleteProvider(nickname)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete provider from DB - %s", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Successfully deleted provider with nickname `%s` from DB", nickname)})
}

// ProviderInfoSSE godoc
// @Summary      Provider information stream
// @Description  Server-sent events stream of provider information updates
// @Tags         Admin - Providers
// @Accept       json
// @Produce      text/event-stream
// @Param        nickname  path  string  true  "Provider nickname"
// @Success      200       {object}  models.Provider
// @Security     BearerAuth
// @Router       /admin/provider/{nickname}/info [get]
func ProviderInfoSSE(c *gin.Context) {
	nickname := c.Param("nickname")

	c.Stream(func(w io.Writer) bool {
		providerData, _ := db.GlobalMongoStore.GetProvider(nickname)

		jsonData, _ := json.Marshal(&providerData)

		c.SSEvent("", string(jsonData))
		c.Writer.Flush()
		time.Sleep(1 * time.Second)
		return true
	})
}

// DeviceInUseWS godoc
// @Summary      Device in-use WebSocket
// @Description  WebSocket connection to manage device usage status and control
// @Tags         Devices Control
// @Accept       json
// @Produce      json
// @Param        udid   path   string  true   "Device UDID"
// @Param        token  query  string  true   "Bearer authentication token"
// @Success      101    {string}  string  "Switching Protocols"
// @Failure      400    {object}  models.ErrorResponse
// @Failure      401    {object}  models.ErrorResponse
// @Failure      404    {object}  models.ErrorResponse
// @Failure      409    {object}  models.ErrorResponse
// @Router       /devices/control/{udid}/in-use [get]
// This websocket connection is used to both set the device in use when remotely controlled
// As well as send live updates when needed - device info, release device, etc
func DeviceInUseWS(c *gin.Context) {
	udid := c.Param("udid")

	if udid == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	// Get the token from the request header
	tokenParam := c.Query("token")
	if tokenParam == "" {
		c.Status(http.StatusUnauthorized)
		return
	}

	var username string
	var userTenant string

	// Extract token from Bearer format
	tokenString, err := auth.ExtractTokenFromBearer(tokenParam)
	if err == nil {
		// Get origin from request
		origin := auth.GetOriginFromRequest(c)

		// Get claims from token with origin
		claims, err := auth.GetClaimsFromToken(tokenString, origin)
		if err != nil || claims.Username == "" {
			// Return 401 for any token validation error
			c.Status(http.StatusUnauthorized)
			return
		}

		username = claims.Username
		userTenant = claims.Tenant
	}

	// Verify if the device is already in use by another user
	devices.HubDevicesData.Mu.Lock()
	device, exists := devices.HubDevicesData.Devices[udid]
	if !exists {
		devices.HubDevicesData.Mu.Unlock()
		c.Status(http.StatusNotFound)
		return
	}

	// Check if device is in use by another user
	if device.InUseBy != "" {
		// Check if it's the same user (including tenant)
		isSameUser := device.InUseBy == username && device.InUseByTenant == userTenant

		// If not the same user AND there's an active WebSocket connection, always deny
		if !isSameUser && device.InUseWSConnection != nil {
			devices.HubDevicesData.Mu.Unlock()
			c.Status(http.StatusConflict)
			return
		}

		// If not the same user and device was used recently, also deny
		if !isSameUser && (time.Now().UnixMilli()-device.InUseTS) < 3000 {
			devices.HubDevicesData.Mu.Unlock()
			c.Status(http.StatusConflict)
			return
		}
	}

	// Reserve the device BEFORE upgrading the WebSocket
	// This prevents another user from passing the verification while we are upgrading the WebSocket
	devices.HubDevicesData.Devices[udid].InUseTS = time.Now().UnixMilli()
	devices.HubDevicesData.Devices[udid].InUseBy = username
	devices.HubDevicesData.Devices[udid].InUseByTenant = userTenant
	devices.HubDevicesData.Devices[udid].LastActionTS = time.Now().UnixMilli()
	devices.HubDevicesData.Mu.Unlock()

	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		// Clear the reservation if the upgrade fails
		devices.HubDevicesData.Mu.Lock()
		devices.HubDevicesData.Devices[udid].InUseTS = 0
		devices.HubDevicesData.Devices[udid].InUseBy = ""
		devices.HubDevicesData.Devices[udid].InUseByTenant = ""
		devices.HubDevicesData.Mu.Unlock()

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
		// Clear the connection
		devices.HubDevicesData.Devices[udid].InUseWSConnection = nil

		// Only clear user info if not running automation
		if !devices.HubDevicesData.Devices[udid].IsRunningAutomation {
			devices.HubDevicesData.Devices[udid].InUseTS = 0
			devices.HubDevicesData.Devices[udid].InUseBy = ""
			devices.HubDevicesData.Devices[udid].InUseByTenant = ""
		}
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
					if devices.HubDevicesData.Devices[udid].InUseBy != "" {
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
							devices.HubDevicesData.Devices[udid].InUseByTenant = ""
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
			devices.HubDevicesData.Devices[udid].InUseByTenant = userTenant
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
			// Only clear user info if not running automation
			if !devices.HubDevicesData.Devices[udid].IsRunningAutomation {
				devices.HubDevicesData.Devices[udid].InUseTS = 0
				devices.HubDevicesData.Devices[udid].InUseBy = ""
				devices.HubDevicesData.Devices[udid].InUseByTenant = ""
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

// AvailableDevicesSSE godoc
// @Summary      Available devices stream
// @Description  Server-sent events stream of available devices filtered by workspace
// @Tags         Devices Control
// @Accept       json
// @Produce      text/event-stream
// @Param        workspaceId  query  string  true  "Workspace ID"
// @Success      200          {object}  []models.LocalHubDevice
// @Failure      400          {object}  models.ErrorResponse
// @Router       /available-devices [get]
func AvailableDevicesSSE(c *gin.Context) {
	// Get workspace ID from query parameter
	workspaceID := c.Query("workspaceId")
	if workspaceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspaceId is required"})
		return
	}

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
			device := devices.HubDevicesData.Devices[key]

			// Filter by workspace
			if device.Device.WorkspaceID != workspaceID {
				continue
			}

			if device.Device.LastUpdatedTimestamp < (time.Now().UnixMilli()-3000) && device.Device.Connected {
				device.Available = false
			} else if device.Device.ProviderState != "live" {
				device.Available = false
			} else {
				device.Available = true
			}

			if device.InUseTS > (time.Now().UnixMilli() - 3000) {
				if !device.InUse {
					device.InUse = true
				}
			} else {
				if device.InUse {
					device.InUse = false
				}
			}
			deviceList = append(deviceList, device)
		}
		devices.HubDevicesData.Mu.Unlock()

		jsonData, _ := json.Marshal(deviceList)
		c.SSEvent("", string(jsonData))
		c.Writer.Flush()
		time.Sleep(1 * time.Second)
		return true
	})
}

// UploadFile godoc
// @Summary      Upload a file
// @Description  Upload a file to MongoDB with custom filename
// @Tags         Admin - Files
// @Accept       multipart/form-data
// @Produce      json
// @Param        file      formData  file    true  "File to upload"
// @Param        fileName  formData  string  true  "Custom filename for MongoDB"
// @Success      200       {object}  models.SuccessResponse
// @Failure      400       {object}  models.ErrorResponse
// @Failure      500       {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/upload-file [post]
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

	openedFile, err := file.Open()
	defer openedFile.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf(fmt.Sprintf("Failed to open provided file - %s", err))})
		return
	}

	err = db.GlobalMongoStore.UploadFile(openedFile, fmt.Sprintf("%s", fileName), true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf(fmt.Sprintf("Failed to upload file to MongoDB - %s", err))})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("`%s` uploaded successfully", file.Filename)})
}

// AddDevice godoc
// @Summary      Add a new device
// @Description  Create a new device in the system
// @Tags         Admin - Devices
// @Accept       json
// @Produce      json
// @Param        device  body      models.Device  true  "Device data"
// @Success      200     {object}  models.SuccessResponse
// @Failure      400     {object}  models.ErrorResponse
// @Failure      500     {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/device [post]
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

	// Validate device configuration before processing
	err = models.ValidateDevice(&device)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device validation failed: %s", err.Error())})
		return
	}

	dbDevices, _ := db.GlobalMongoStore.GetDevices()
	for _, dbDevice := range dbDevices {
		if dbDevice.UDID == device.UDID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Device already exists in the DB"})
			return
		}
	}

	err = db.GlobalMongoStore.AddOrUpdateDevice(&device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert device in DB"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Added device in DB"})
}

// UpdateDevice godoc
// @Summary      Update a device
// @Description  Update an existing device in the system
// @Tags         Admin - Devices
// @Accept       json
// @Produce      json
// @Param        device  body      models.Device  true  "Device data"
// @Success      200     {object}  models.SuccessResponse
// @Failure      400     {object}  models.ErrorResponse
// @Failure      404     {object}  models.ErrorResponse
// @Failure      500     {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/device [put]
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

	dbDevices, _ := db.GlobalMongoStore.GetDevices()
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

			if reqDevice.WorkspaceID != "" && reqDevice.WorkspaceID != dbDevice.WorkspaceID {
				dbDevice.WorkspaceID = reqDevice.WorkspaceID
			}

			// Validate device configuration before saving to DB
			err = models.ValidateDevice(&dbDevice)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device validation failed: %s", err.Error())})
				return
			}

			err = db.GlobalMongoStore.AddOrUpdateDevice(&dbDevice)
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

// DeleteDevice godoc
// @Summary      Delete a device
// @Description  Remove a device from the system
// @Tags         Admin - Devices
// @Accept       json
// @Produce      json
// @Param        udid  path      string  true  "Device UDID"
// @Success      200   {object}  models.SuccessResponse
// @Failure      500   {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/device/{udid} [delete]
func DeleteDevice(c *gin.Context) {
	udid := c.Param("udid")

	err := db.GlobalMongoStore.DeleteDevice(udid)
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

// GetDevices godoc
// @Summary      Get all devices
// @Description  Retrieve list of all devices with provider information
// @Tags         Admin - Devices
// @Accept       json
// @Produce      json
// @Success      200  {object}  AdminDeviceData
// @Failure      500  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/devices [get]
func GetDevices(c *gin.Context) {
	dbDevices, _ := db.GlobalMongoStore.GetDevices()
	providers, _ := db.GlobalMongoStore.GetAllProviders()

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

// ReleaseUsedDevice godoc
// @Summary      Release a device in use
// @Description  Force release a device that is currently in use
// @Tags         Admin - Devices
// @Accept       json
// @Produce      json
// @Param        udid  path      string  true  "Device UDID"
// @Success      200   {object}  models.SuccessResponse
// @Failure      500   {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/device/{udid}/release [post]
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
	devices.HubDevicesData.Devices[udid].InUseByTenant = ""

	c.JSON(200, gin.H{"message": "Message to release device was successfully sent"})
}

// syncDeviceFields synchronizes device fields from source to target device
// Only updates fields that are different between the two devices
func syncDeviceFields(target *models.Device, source *models.Device) {
	if target.Usage != source.Usage {
		target.Usage = source.Usage
	}
	if target.Name != source.Name {
		target.Name = source.Name
	}
	if target.OSVersion != source.OSVersion {
		target.OSVersion = source.OSVersion
	}
	if target.ScreenWidth != source.ScreenWidth {
		target.ScreenWidth = source.ScreenWidth
	}
	if target.ScreenHeight != source.ScreenHeight {
		target.ScreenHeight = source.ScreenHeight
	}
	if target.Provider != source.Provider {
		target.Provider = source.Provider
	}
}

// ProviderUpdate godoc
// @Summary      Provider update
// @Description  Receive updates from providers about device status
// @Tags         Providers
// @Accept       json
// @Produce      json
// @Param        providerData  body      models.ProviderData  true  "Provider device data"
// @Success      200           {object}  models.SuccessResponse
// @Router       /provider-update [post]
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

			providerDeviceHasChanged := providerDevice.Provider != hubDevice.Device.Provider

			// If device is "live" on provider, provider data takes precedence for operational fields
			// Otherwise, DB data takes precedence to prevent incorrect overrides
			if providerDevice.ProviderState == "live" && !providerDeviceHasChanged {
				// For live devices, provider operational data takes precedence
				// Keep provider values for Usage and Provider fields
				// But still respect DB configuration for device metadata
				syncDeviceFields(&hubDevice.Device, &providerDevice)
			} else if !providerDeviceHasChanged {
				// For non-live devices, DB data takes precedence for all fields
				syncDeviceFields(&providerDevice, &hubDevice.Device)
			}

			hubDevice.Device = providerDevice
		}
		devices.HubDevicesData.Mu.Unlock()
	}

	c.JSON(http.StatusOK, gin.H{})
}

// GetUsers godoc
// @Summary      Get all users
// @Description  Retrieve list of all users in the system
// @Tags         Admin - Users
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.User
// @Failure      500  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/users [get]
func GetUsers(c *gin.Context) {
	users, _ := db.GlobalMongoStore.GetUsers()
	// Clean up the passwords, not that the project is very secure but let's not send them
	for i := range users {
		users[i].Password = ""
	}

	c.JSON(http.StatusOK, users)
}

// GetFiles godoc
// @Summary      Get all files
// @Description  Retrieve list of all files stored in the system
// @Tags         Admin - Files
// @Accept       json
// @Produce      json
// @Success      200  {array}  models.FileEntry
// @Security     BearerAuth
// @Router       /admin/files [get]
func GetFiles(c *gin.Context) {
	files, _ := db.GlobalMongoStore.GetFiles()

	c.JSON(http.StatusOK, files)
}

// DownloadResourceFromGithubRepo godoc
// @Summary      Download resource from GitHub repository
// @Description  Download a resource file from the GADS GitHub repository
// @Tags         Admin - Files
// @Accept       json
// @Produce      text/plain
// @Param        fileName  query  string  true  "Name of the file to download"
// @Success      200       {string}  string  "File downloaded successfully"
// @Failure      500       {string}  string  "Internal server error"
// @Security     BearerAuth
// @Router       /admin/download-github-file [post]
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

// GetGlobalStreamSettings godoc
// @Summary      Get global stream settings
// @Description  Retrieve global streaming settings from the database
// @Tags         Admin - Settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.StreamSettings
// @Failure      500  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/global-settings [get]
func GetGlobalStreamSettings(c *gin.Context) {
	// Retrieve global stream settings from the database
	streamSettings, err := db.GlobalMongoStore.GetGlobalStreamSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve global stream settings"})
		return
	}

	// Return the stream settings as a JSON response
	c.JSON(http.StatusOK, streamSettings)
}

// UpdateGlobalStreamSettings godoc
// @Summary      Update global stream settings
// @Description  Update global streaming settings in the database
// @Tags         Admin - Settings
// @Accept       json
// @Produce      json
// @Param        settings  body      models.StreamSettings  true  "Stream settings"
// @Success      200       {object}  models.SuccessResponse
// @Failure      400       {object}  models.ErrorResponse
// @Failure      500       {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/global-settings [post]
func UpdateGlobalStreamSettings(c *gin.Context) {
	var settings models.StreamSettings

	// Bind the JSON input to the settings struct
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	err := db.GlobalMongoStore.UpdateGlobalStreamSettings(settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}
