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
	"GADS/common/api"
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
// @Tags         Hub - System
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.SuccessResponse
// @Security     BearerAuth
// @Router       /health [get]
func HealthCheck(c *gin.Context) {
	api.OKMessage(c, "ok")
}

// GetAppiumLogs godoc
// @Summary      Get Appium logs
// @Description  Retrieve Appium logs from a specific collection with optional limit
// @Tags         Hub - Logs
// @Accept       json
// @Produce      json
// @Param        collection  query     string  true   "Collection name"
// @Param        logLimit    query     int     false  "Log limit (max 1000, default 100)"
// @Success      200         {object}   models.LogsResponse
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
		api.BadRequest(c, "Empty collection name provided")
		return
	}

	logs, err := db.GlobalMongoStore.GetAppiumLogs(collectionName, logLimit)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to get logs - %s", err))
	}

	api.OK(c, "Successfully retrieved Appium logs", logs)
}

// GetProviderLogs godoc
// @Summary      Get provider logs
// @Description  Retrieve provider logs from a specific collection with optional limit
// @Tags         Hub - Logs
// @Accept       json
// @Produce      json
// @Param        collection  query     string  true   "Collection name"
// @Param        logLimit    query     int     false  "Log limit (max 1000, default 200)"
// @Success      200         {object}   models.LogsResponse
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
		api.BadRequest(c, "Empty collection name provided")
		return
	}

	logs, err := db.GlobalMongoStore.GetProviderLogs(collectionName, logLimit)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to get logs - %s", err))
		return
	}

	api.OK(c, "Successfully retrieved provider logs", logs)
}

// AddUser godoc
// @Summary      Add a new user
// @Description  Create a new user in the system
// @Tags         Hub - Admin - Users
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
		api.InternalError(c, fmt.Sprintf("%s", err))
		return
	}

	err = json.Unmarshal(body, &user)
	if err != nil {
		api.BadRequest(c, fmt.Sprintf("%s", err))
		return
	}

	if user.Username == "" || user.Password == "" || (user.Role == "user" && len(user.WorkspaceIDs) == 0) {
		api.BadRequest(c, "Empty or invalid body")
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		api.BadRequest(c, "Invalid role - `admin` and `user` are the accepted values")
		return
	}

	dbUser, err := db.GlobalMongoStore.GetUser(user.Username)
	if err != nil && err != mongo.ErrNoDocuments {
		api.InternalError(c, "Failed checking for user in db - "+err.Error())
		return
	}

	if dbUser.Username != "" {
		api.BadRequest(c, "User already exists")
		return
	}

	err = db.GlobalMongoStore.AddOrUpdateUser(user)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed adding/updating user - %s", err))
		return
	}

	api.OKMessage(c, "Successfully added user")
}

// UpdateUser godoc
// @Summary      Update an existing user
// @Description  Update user information in the system
// @Tags         Hub - Admin - Users
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
		api.InternalError(c, fmt.Sprintf("%s", err))
		return
	}

	err = json.Unmarshal(body, &user)
	if err != nil {
		api.BadRequest(c, fmt.Sprintf("%s", err))
		return
	}

	if user.Username == "" || (user.Role == "user" && len(user.WorkspaceIDs) == 0) {
		api.BadRequest(c, "Username cannot be empty and non-admin users must have at least one workspace")
		return
	}

	dbUser, err := db.GlobalMongoStore.GetUser(user.Username)
	if err != nil && err != mongo.ErrNoDocuments {
		api.InternalError(c, "Failed checking for user in db - "+err.Error())
		return
	}

	if dbUser.Username == "" {
		api.BadRequest(c, "Cannot update non-existing user")
		return
	}

	err = db.GlobalMongoStore.AddOrUpdateUser(user)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed adding/updating user - %s", err))
		return
	}

	api.OKMessage(c, "Successfully updated user")
}

// DeleteUser godoc
// @Summary      Delete a user
// @Description  Remove a user from the system
// @Tags         Hub - Admin - Users
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
		api.InternalError(c, "Failed to delete user - "+err.Error())
		return
	}

	api.OKMessage(c, "Successfully deleted user")
}

// GetProviders godoc
// @Summary      Get all providers
// @Description  Retrieve list of all providers in the system
// @Tags         Hub - Admin - Providers
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.ProviderListResponse
// @Failure      400  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/providers [get]
func GetProviders(c *gin.Context) {
	providers, _ := db.GlobalMongoStore.GetAllProviders()
	if len(providers) == 0 {
		api.OK(c, "", []models.Provider{})
		return
	}
	api.OK(c, "", providers)
}

func GetProviderInfo(c *gin.Context) {
	providerName := c.Param("name")
	providers, _ := db.GlobalMongoStore.GetAllProviders()
	for _, provider := range providers {
		if provider.Nickname == providerName {
			api.OK(c, "Successfully retrieved providers data", provider)
			return
		}
	}
	api.NotFound(c, fmt.Sprintf("No provider with name `%s` found", providerName))
}

// AddProvider godoc
// @Summary      Add a new provider
// @Description  Create a new provider in the system
// @Tags         Hub - Admin - Providers
// @Accept       json
// @Produce      json
// @Param        provider  body      models.Provider  true  "Provider data"
// @Success      200       {object}  models.ProviderListResponse
// @Failure      400       {object}  models.ErrorResponse
// @Failure      500       {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/providers/add [post]
func AddProvider(c *gin.Context) {
	var provider models.Provider
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("%s", err))
		return
	}

	err = json.Unmarshal(body, &provider)
	if err != nil {
		api.BadRequest(c, fmt.Sprintf("%s", err))
		return
	}

	// Validations
	if provider.Nickname == "" {
		api.BadRequest(c, "Missing or invalid nickname")
		return
	}
	providerDB, _ := db.GlobalMongoStore.GetProvider(provider.Nickname)
	if providerDB.Nickname == provider.Nickname {
		api.BadRequest(c, "Provider with this nickname already exists")
		return
	}

	if provider.OS == "" {
		api.BadRequest(c, "Missing or invalid OS")
		return
	}
	if provider.HostAddress == "" {
		api.BadRequest(c, "Missing or invalid host address")
		return
	}
	if provider.Port == 0 {
		api.BadRequest(c, "Missing or invalid port")
		return
	}
	if provider.UseSeleniumGrid && provider.SeleniumGrid == "" {
		api.BadRequest(c, "Missing or invalid Selenium Grid address")
		return
	}

	provider.RegularizeProviderState()

	err = db.GlobalMongoStore.AddOrUpdateProvider(provider)
	if err != nil {
		api.InternalError(c, "Could not create provider")
		return
	}

	providersDB, _ := db.GlobalMongoStore.GetAllProviders()
	api.OK(c, "Successfully added provider", providersDB)
}

// UpdateProvider godoc
// @Summary      Update a provider
// @Description  Update an existing provider in the system
// @Tags         Hub - Admin - Providers
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
		api.InternalError(c, fmt.Sprintf("%s", err))
		return
	}

	err = json.Unmarshal(body, &provider)
	if err != nil {
		api.BadRequest(c, fmt.Sprintf("%s", err))
		return
	}

	// Validations
	if provider.Nickname == "" {
		api.BadRequest(c, "missing `nickname` field")
		return
	}
	if provider.OS == "" {
		api.BadRequest(c, "missing `os` field")
		return
	}
	if provider.HostAddress == "" {
		api.BadRequest(c, "missing `host_address` field")
		return
	}
	if provider.Port == 0 {
		api.BadRequest(c, "missing `port` field")
		return
	}
	if provider.UseSeleniumGrid && provider.SeleniumGrid == "" {
		api.BadRequest(c, "missing `selenium_grid` field")
		return
	}

	provider.RegularizeProviderState()

	err = db.GlobalMongoStore.AddOrUpdateProvider(provider)
	if err != nil {
		api.InternalError(c, "Could not update provider")
		return
	}
	api.OKMessage(c, "Provider updated successfully")
}

// DeleteProvider godoc
// @Summary      Delete a provider
// @Description  Remove a provider from the system
// @Tags         Hub - Admin - Providers
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
		api.InternalError(c, fmt.Sprintf("Failed to delete provider from DB - %s", err))
		return
	}

	api.OKMessage(c, fmt.Sprintf("Successfully deleted provider with nickname `%s` from DB", nickname))
}

// ProviderInfoSSE godoc
// @Summary      Provider information stream
// @Description  Server-sent events stream of provider information updates
// @Tags         Hub - Admin - Providers
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

// No proper Swagger documentation for websockets
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
// @Tags         Hub - Devices selection
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
		api.BadRequest(c, "workspaceId is required")
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
// @Tags         Hub - Admin - Files
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
		api.BadRequest(c, fmt.Sprintf("No file provided in form data - %s", err))
		return
	}
	fileName := c.PostForm("fileName")
	if fileName == "" {
		api.BadRequest(c, "No fileName for MongoDB record was provided")
		return
	}

	openedFile, err := file.Open()
	defer openedFile.Close()
	if err != nil {
		api.InternalError(c, fmt.Sprintf(fmt.Sprintf("Failed to open provided file - %s", err)))
		return
	}

	err = db.GlobalMongoStore.UploadFile(openedFile, fmt.Sprintf("%s", fileName), true)
	if err != nil {
		api.InternalError(c, fmt.Sprintf(fmt.Sprintf("Failed to upload file to MongoDB - %s", err)))
		return
	}

	api.OKMessage(c, fmt.Sprintf("`%s` uploaded successfully", file.Filename))
}

// AddDevice godoc
// @Summary      Add a new device
// @Description  Create a new device in the system
// @Tags         Hub - Admin - Devices
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
		api.InternalError(c, fmt.Sprintf("Failed to read request body - %s", err))
		return
	}
	defer c.Request.Body.Close()

	var device models.Device
	err = json.Unmarshal(reqBody, &device)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to unmarshal request body to struct - %s", err))
		return
	}

	// Validate device configuration before processing
	err = models.ValidateDevice(&device)
	if err != nil {
		api.BadRequest(c, fmt.Sprintf("Device validation failed: %s", err.Error()))
		return
	}

	dbDevices, _ := db.GlobalMongoStore.GetDevices()
	for _, dbDevice := range dbDevices {
		if dbDevice.UDID == device.UDID {
			api.BadRequest(c, "Device already exists in the DB")
			return
		}
	}

	err = db.GlobalMongoStore.AddOrUpdateDevice(&device)
	if err != nil {
		api.InternalError(c, "Failed to upsert device in DB")
		return
	}

	api.OKMessage(c, "Added device in DB")
}

// UpdateDevice godoc
// @Summary      Update a device
// @Description  Update an existing device in the system
// @Tags         Hub - Admin - Devices
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
		api.InternalError(c, fmt.Sprintf("Failed to read request body - %s", err))
		return
	}
	defer c.Request.Body.Close()

	var reqDevice models.Device
	err = json.Unmarshal(reqBody, &reqDevice)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to unmarshal request body to struct - %s", err))
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
			if reqDevice.StreamType != dbDevice.StreamType {
				dbDevice.StreamType = reqDevice.StreamType
			}

			if reqDevice.WorkspaceID != "" && reqDevice.WorkspaceID != dbDevice.WorkspaceID {
				dbDevice.WorkspaceID = reqDevice.WorkspaceID
			}

			// Validate device configuration before saving to DB
			err = models.ValidateDevice(&dbDevice)
			if err != nil {
				api.BadRequest(c, fmt.Sprintf("Device validation failed: %s", err.Error()))
				return
			}

			err = db.GlobalMongoStore.AddOrUpdateDevice(&dbDevice)
			if err != nil {
				api.InternalError(c, "Failed to upsert device in DB")
				return
			}
			api.OKMessage(c, "Successfully updated device in DB")
			return
		}
	}

	api.NotFound(c, fmt.Sprintf("Device with udid `%s` does not exist in the DB", reqDevice.UDID))
}

// DeleteDevice godoc
// @Summary      Delete a device
// @Description  Remove a device from the system
// @Tags         Hub - Admin - Devices
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
		api.InternalError(c, fmt.Sprintf("Failed to delete device from DB - %s", err))
		return
	}

	api.OKMessage(c, fmt.Sprintf("Successfully deleted device with udid `%s` from DB", udid))
}

type AdminDeviceData struct {
	Devices           []models.Device     `json:"devices"`
	Providers         []string            `json:"providers"`
	DeviceStreamTypes []models.StreamType `json:"device_stream_types"`
}

// GetDevices godoc
// @Summary      Get all devices
// @Description  Retrieve list of all devices with provider information
// @Tags         Hub - Admin - Devices
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

	for index, dbDevice := range dbDevices {
		dbDevices[index].SupportedStreamTypes = models.StreamTypesForOS(dbDevice.OS)
	}

	var adminDeviceData = AdminDeviceData{
		Devices:   dbDevices,
		Providers: providerNames,
		DeviceStreamTypes: []models.StreamType{
			models.MJPEGStreamType,
			models.IOSWebRTCFFMpegStreamType,
			models.AndroidWebRTCGadsH264StreamType,
			models.AndroidWebRTCGetStreamStreamType,
			models.IOSWebRTCBroadcastExtensionStreamType,
		},
	}

	api.OK(c, "Successfully retrieved devices data", adminDeviceData)
}

// ReleaseUsedDevice godoc
// @Summary      Release a device in use
// @Description  Force release a device that is currently in use
// @Tags         Hub - Admin - Devices
// @Tags         Hub - Devices selection
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
		api.InternalError(c, "Failed to send release device message - "+err.Error())
		return
	}

	devices.HubDevicesData.Devices[udid].InUseWSConnection.Close()
	devices.HubDevicesData.Devices[udid].InUseTS = 0
	devices.HubDevicesData.Devices[udid].InUseBy = ""
	devices.HubDevicesData.Devices[udid].InUseByTenant = ""

	api.OKMessage(c, "Message to release device was successfully sent")
}

// syncDeviceFields synchronizes operational fields from provider to hub device
// Only updates fields that are different between the two devices
func syncDeviceFields(target *models.Device, source *models.Device) {
	if target.Connected != source.Connected {
		target.Connected = source.Connected
	}
	if target.ProviderState != source.ProviderState {
		target.ProviderState = source.ProviderState
	}
	if target.LastUpdatedTimestamp != source.LastUpdatedTimestamp {
		target.LastUpdatedTimestamp = source.LastUpdatedTimestamp
	}
	if target.Host != source.Host {
		target.Host = source.Host
	}
}

// ProviderUpdate godoc
// @Summary      Provider update
// @Description  Receive updates from providers about device status
// @Tags         Hub - Admin - Providers
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

			// Update only operational fields from provider
			syncDeviceFields(&hubDevice.Device, &providerDevice)
		}
		devices.HubDevicesData.Mu.Unlock()
	}

	api.OKMessage(c, "Provider data updated in hub")
}

// GetUsers godoc
// @Summary      Get all users
// @Description  Retrieve list of all users in the system
// @Tags         Hub - Admin - Users
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.UserListResponse
// @Failure      500  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/users [get]
func GetUsers(c *gin.Context) {
	users, _ := db.GlobalMongoStore.GetUsers()
	// Clean up the passwords, not that the project is very secure but let's not send them
	for i := range users {
		users[i].Password = ""
	}

	api.OK(c, "Successfully retrieved users data", users)
}

// GetFiles godoc
// @Summary      Get all files
// @Description  Retrieve list of all files stored in the system
// @Tags         Hub - Admin - Files
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.FileListResponse
// @Security     BearerAuth
// @Router       /admin/files [get]
func GetFiles(c *gin.Context) {
	files, _ := db.GlobalMongoStore.GetFiles()

	api.OK(c, "Successfully retrieved files data", files)
}

// GetGlobalStreamSettings godoc
// @Summary      Get global stream settings
// @Description  Retrieve global streaming settings from the database
// @Tags         Hub - Admin - Settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.StreamSettingsResponse
// @Failure      500  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/global-settings [get]
func GetGlobalStreamSettings(c *gin.Context) {
	// Retrieve global stream settings from the database
	streamSettings, err := db.GlobalMongoStore.GetGlobalStreamSettings()
	if err != nil {
		api.InternalError(c, "Failed to retrieve global stream settings")
		return
	}

	// Return the stream settings as a JSON response
	api.OK(c, "Successfully retrieved global stream settings", streamSettings)
}

// UpdateGlobalStreamSettings godoc
// @Summary      Update global stream settings
// @Description  Update global streaming settings in the database
// @Tags         Hub - Admin - Settings
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
		api.BadRequest(c, "Invalid input")
		return
	}

	err := db.GlobalMongoStore.UpdateGlobalStreamSettings(settings)
	if err != nil {
		api.InternalError(c, "Failed to save settings")
		return
	}

	api.OKMessage(c, "Settings updated successfully")
}

// GetMinioConfig godoc
// @Summary      Get MinIO configuration
// @Description  Retrieve MinIO configuration settings from the database
// @Tags         Hub - Admin - Settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.MinioConfigResponse
// @Failure      500  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/minio-config [get]
func GetMinioConfig(c *gin.Context) {
	minioConfig, err := db.GlobalMongoStore.GetMinioConfig()
	if err != nil {
		api.InternalError(c, "Failed to retrieve MinIO configuration")
		return
	}

	api.OK(c, "MinIO configuration retrieved successfully", minioConfig)
}

// UpdateMinioConfig godoc
// @Summary      Update MinIO configuration
// @Description  Update MinIO configuration settings in the database
// @Tags         Hub - Admin - Settings
// @Accept       json
// @Produce      json
// @Param        config  body      models.MinioConfig  true  "MinIO configuration"
// @Success      200     {object}  models.SuccessResponse
// @Failure      400     {object}  models.ErrorResponse
// @Failure      500     {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/minio-config [post]
func UpdateMinioConfig(c *gin.Context) {
	var config models.MinioConfig

	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, "Invalid input")
		return
	}

	// Basic validation
	if config.Enabled {
		if config.Endpoint == "" {
			api.BadRequest(c, "Endpoint is required when MinIO is enabled")
			return
		}

		if config.AccessKeyID == "" {
			api.BadRequest(c, "Access Key ID is required when MinIO is enabled")
			return
		}

		if config.SecretAccessKey == "" {
			api.BadRequest(c, "Secret Access Key is required when MinIO is enabled")
			return
		}
	}

	err := db.GlobalMongoStore.UpdateMinioConfig(config)
	if err != nil {
		api.InternalError(c, "Failed to save MinIO configuration")
		return
	}

	api.OKMessage(c, "MinIO configuration updated successfully")
}

// GetTURNConfig godoc
// @Summary      Get TURN server configuration
// @Description  Retrieve the TURN server configuration from MongoDB global settings
// @Tags         Hub - Admin - Settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.TURNConfigResponse
// @Failure      500  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/turn-config [get]
func GetTURNConfig(c *gin.Context) {
	turnConfig, err := db.GlobalMongoStore.GetTURNConfig()
	if err != nil {
		api.InternalError(c, "Failed to retrieve TURN configuration")
		return
	}

	api.OK(c, "TURN configuration retrieved", turnConfig)
}

// UpdateTURNConfig godoc
// @Summary      Update TURN server configuration
// @Description  Update the TURN server configuration stored in MongoDB global settings
// @Tags         Hub - Admin - Settings
// @Accept       json
// @Produce      json
// @Param        config  body      models.TURNConfig  true  "TURN configuration"
// @Success      200     {object}  models.SuccessResponse
// @Failure      400     {object}  models.ErrorResponse
// @Failure      500     {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/turn-config [post]
func UpdateTURNConfig(c *gin.Context) {
	var config models.TURNConfig

	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, "Invalid input")
		return
	}

	// Validation: if enabled, validate required fields
	if config.Enabled {
		if config.Server == "" {
			api.BadRequest(c, "Server is required when TURN is enabled")
			return
		}

		if config.Port <= 0 || config.Port > 65535 {
			api.BadRequest(c, "Port must be between 1 and 65535")
			return
		}

		if config.SharedSecret == "" {
			api.BadRequest(c, "Shared secret is required when TURN is enabled")
			return
		}

		// Set default TTL if not provided
		if config.TTL == 0 {
			config.TTL = 3600 // Default: 1 hour
		}
	}

	err := db.GlobalMongoStore.UpdateTURNConfig(config)
	if err != nil {
		api.InternalError(c, "Failed to save TURN configuration")
		return
	}

	api.OKMessage(c, "TURN configuration updated successfully")
}

// GetSystemStatus godoc
// @Summary      Get system status messages
// @Description  Retrieve system status messages for administrators
// @Tags         Hub - Admin - System
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.SysStatusResponse
// @Security     BearerAuth
// @Router       /admin/system-status [get]
func GetSystemStatus(c *gin.Context) {
	var messages []models.SystemStatusMessage

	// Check if any devices are configured
	devices, _ := db.GlobalMongoStore.GetDevices()
	if len(devices) == 0 {
		messages = append(messages, models.SystemStatusMessage{
			Type:    "no_devices",
			Message: "No devices configured.",
			Action:  "Add devices in Admin -> Devices",
		})
	}

	// Check if any providers are configured
	providers, _ := db.GlobalMongoStore.GetAllProviders()
	if len(providers) == 0 {
		messages = append(messages, models.SystemStatusMessage{
			Type:    "no_providers",
			Message: "No providers configured.",
			Action:  "Add providers in Admin -> Providers",
		})
	}

	response := models.SystemStatusResponse{
		Messages: messages,
	}

	api.OK(c, "System status retrieved successfully", response)
}
