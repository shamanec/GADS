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
	"GADS/hub/config"
	"GADS/hub/devices"
	"GADS/hub/signing"
	"GADS/provider/logger"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

const (
	defaultLockTTLMinutes        = 10
	maxLockTTLMinutes            = 360
	deviceInUsePingInterval      = 5 * time.Second
	deviceInUseInactivityTimeout = 30 * time.Minute
)

func normalizeLockTTLMinutes(ttl int) int {
	if ttl <= 0 {
		return defaultLockTTLMinutes
	}
	if ttl > maxLockTTLMinutes {
		return maxLockTTLMinutes
	}
	return ttl
}

func isDeviceInUseSessionExpired(lastActionTS int64, now time.Time) bool {
	return (now.UnixMilli() - lastActionTS) > deviceInUseInactivityTimeout.Milliseconds()
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
		return
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

	// Extract token from the request
	claims, err := auth.GetClaimsFromRequest(c)
	if err != nil || claims.Username == "" {
		c.Status(http.StatusUnauthorized)
		return
	}
	username = claims.Username
	userTenant = claims.Tenant

	// Verify if the device is already in use by another user
	device, exists := devices.HubDeviceStore.Get(udid)
	if !exists {
		c.Status(http.StatusNotFound)
		return
	}

	device.Mu.Lock()

	if device.IsLockedByOther(username, userTenant) {
		device.Mu.Unlock()
		c.Status(http.StatusConflict)
		return
	}

	// If the device is already held via an API lease by this user, preserve the API lock.
	// Calling AcquireLock here would overwrite LockSource to "ui", which would cause
	// HasActiveLease() to return false and the API lock to be released on WS disconnect.
	// For a pure UI session (no prior lock), reserve the device now to prevent a race
	// between passing the check above and completing the WebSocket upgrade below.
	if !device.HasActiveLease() {
		device.AcquireLock(username, userTenant, devices.LockSourceUI) //nolint:errcheck — AcquireLock only fails when locked by other, already checked above
	}
	device.Mu.Unlock()

	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		// Clear the reservation if the upgrade fails
		device.Mu.Lock()
		device.ReleaseLock()
		device.Mu.Unlock()

		logger.ProviderLogger.LogError("device_in_use_ws", fmt.Sprintf("Failed upgrading device in-use websocket - %s", err))
		return
	}

	// Add the created connection to the respective device in the map
	// So we can send different messages to it from other sources
	device.Mu.Lock()
	device.SetWSConnection(conn)
	device.Mu.Unlock()

	// If this function returns then we close the connection
	// And also set it to nil for the respective device in the map
	defer func() {
		conn.Close()
		device.Mu.Lock()
		device.ClearWSConnection()
		// Do not release the lock if automation is still running or if an API lease is still active.
		// The user intentionally locked the device via API (or owns the automation session)
		// and must remain the lock holder after closing remote control.
		if !device.IsRunningAutomation && !device.HasActiveLease() {
			device.ReleaseLock()
		}
		device.Mu.Unlock()
	}()

	// Create a context with cancel to use in the goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Goroutine: send pings every 5s and check for 30-min inactivity
	go func() {
		ticker := time.NewTicker(deviceInUsePingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				device.Mu.RLock()
				lastActionTS := device.LastActionTS
				device.Mu.RUnlock()

				if isDeviceInUseSessionExpired(lastActionTS, time.Now()) {
					ws.WriteFrame(conn, ws.NewCloseFrame(ws.NewCloseFrameBody(ws.StatusCode(4001), "session expired"))) //nolint:errcheck
					cancel()
					return
				}

				if err := wsutil.WriteServerText(conn, []byte("ping")); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	// Main loop: read client responses to keep the lock alive
	for {
		conn.SetReadDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck
		_, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			return
		}
		device.Mu.Lock()
		device.RefreshLock()
		device.Mu.Unlock()
	}
}

// AvailableDevicesSSE godoc
// @Summary      Available devices stream
// @Description  Server-sent events stream of available devices filtered by workspace
// @Tags         Hub - Devices selection
// @Accept       json
// @Produce      text/event-stream
// @Param        workspaceId  query  string  true  "Workspace ID"
// @Success      200          {object}  []devices.LocalHubDevice
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
		var deviceList = make([]*devices.LocalHubDevice, 0)

		for _, d := range devices.HubDeviceStore.AllSorted() {
			d.Mu.Lock()

			if d.Device.WorkspaceID != workspaceID {
				d.Mu.Unlock()
				continue
			}

			if d.LastUpdatedTimestamp < (time.Now().UnixMilli()-3000) && d.Connected {
				d.Available = false
			} else if d.ProviderState != "live" {
				d.Available = false
			} else {
				d.Available = true
			}

			d.InUse = d.IsLocked()

			d.Mu.Unlock()
			deviceList = append(deviceList, d)
		}

		jsonData, _ := json.Marshal(deviceList)
		c.SSEvent("", string(jsonData))
		c.Writer.Flush()
		time.Sleep(1 * time.Second)
		return true
	})
}

// uploadIPAFile stores an uploaded .ipa in GridFS under a unique generated name
// tagged with the given file type, so multiple builds coexist and each provider
// references a specific one by its GridFS id. noun is used in user-facing messages.
func uploadIPAFile(c *gin.Context, fileType, noun string) {
	file, err := c.FormFile("file")
	if err != nil {
		api.BadRequest(c, fmt.Sprintf("No file provided in form data - %s", err))
		return
	}
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".ipa") {
		api.BadRequest(c, fmt.Sprintf("Only .ipa files are allowed for %s uploads", noun))
		return
	}

	openedFile, err := file.Open()
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to open provided file - %s", err))
		return
	}
	defer openedFile.Close()

	metadata := bson.M{
		"type":          fileType,
		"description":   c.PostForm("description"),
		"uploaded_by":   c.GetString("username"),
		"original_name": file.Filename,
	}

	err = db.GlobalMongoStore.UploadFileWithMetadata(openedFile, uuid.NewString(), metadata, false)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to upload %s to MongoDB - %s", noun, err))
		return
	}

	api.OKMessage(c, fmt.Sprintf("`%s` uploaded successfully", file.Filename))
}

// UploadWebDriverAgentFile godoc
// @Summary      Upload a WebDriverAgent IPA
// @Description  Upload a WebDriverAgent IPA to MongoDB. Multiple IPAs can coexist; each is stored under a unique generated name with an optional description and the uploader recorded.
// @Tags         Hub - Admin - Files
// @Accept       multipart/form-data
// @Produce      json
// @Param        file         formData  file    true   "WebDriverAgent IPA file"
// @Param        description  formData  string  false  "Optional description to tell builds apart"
// @Success      200          {object}  models.SuccessResponse
// @Failure      400          {object}  models.ErrorResponse
// @Failure      500          {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/files/webdriveragent [post]
func UploadWebDriverAgentFile(c *gin.Context) {
	uploadIPAFile(c, "wda", "WebDriverAgent IPA")
}

// UploadBroadcastFile godoc
// @Summary      Upload a broadcast extension IPA
// @Description  Upload a GADS broadcast extension IPA to MongoDB. Multiple IPAs can coexist; each is stored under a unique generated name with an optional description and the uploader recorded.
// @Tags         Hub - Admin - Files
// @Accept       multipart/form-data
// @Produce      json
// @Param        file         formData  file    true   "Broadcast extension IPA file"
// @Param        description  formData  string  false  "Optional description to tell builds apart"
// @Success      200          {object}  models.SuccessResponse
// @Failure      400          {object}  models.ErrorResponse
// @Failure      500          {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/files/broadcast [post]
func UploadBroadcastFile(c *gin.Context) {
	uploadIPAFile(c, "broadcast", "broadcast extension IPA")
}

// tailString trims s and returns at most its last n characters, prefixing an
// ellipsis when it was truncated. Used to surface the tail of zsign's output.
func tailString(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}

// signAndUploadIPAFile resigns an unsigned .ipa with zsign using the provided
// provisioning profile and signing material (a .p12 + password, or a certificate
// + private key), then stores the signed result tagged with the given file type.
// noun is used in user-facing messages.
func signAndUploadIPAFile(c *gin.Context, fileType, noun string) {
	ipaFile, err := c.FormFile("file")
	if err != nil {
		api.BadRequest(c, fmt.Sprintf("No IPA file provided in form data - %s", err))
		return
	}
	if !strings.HasSuffix(strings.ToLower(ipaFile.Filename), ".ipa") {
		api.BadRequest(c, fmt.Sprintf("Only .ipa files are allowed for %s uploads", noun))
		return
	}

	profileFile, err := c.FormFile("profile")
	if err != nil {
		api.BadRequest(c, fmt.Sprintf("No provisioning profile provided in form data - %s", err))
		return
	}
	if !strings.HasSuffix(strings.ToLower(profileFile.Filename), ".mobileprovision") {
		api.BadRequest(c, "Only .mobileprovision files are allowed for the provisioning profile")
		return
	}

	method := c.PostForm("method")
	if method == "" {
		method = "p12"
	}

	// zsign works on files on disk. Stage every input in a temp dir that is
	// always cleaned up, sign into it, then store the signed output.
	tmpDir, err := os.MkdirTemp("", "gads-wda-sign-*")
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to create temp dir for signing - %s", err))
		return
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input.ipa")
	outputPath := filepath.Join(tmpDir, "signed.ipa")
	profilePath := filepath.Join(tmpDir, "profile.mobileprovision")
	if err := c.SaveUploadedFile(ipaFile, inputPath); err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to store uploaded IPA - %s", err))
		return
	}
	if err := c.SaveUploadedFile(profileFile, profilePath); err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to store provisioning profile - %s", err))
		return
	}

	binName, err := signing.BinaryName()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}
	zsignPath := filepath.Join(config.GlobalHubConfig.FilesTempDir, "resources", binName)
	if _, err := os.Stat(zsignPath); err != nil {
		api.InternalError(c, "The zsign binary is not available on the hub - resource files were not unpacked on startup")
		return
	}

	opts := signing.Options{
		ZsignPath:   zsignPath,
		InputIPA:    inputPath,
		OutputIPA:   outputPath,
		ProfilePath: profilePath,
	}

	switch method {
	case "p12":
		p12File, err := c.FormFile("p12")
		if err != nil {
			api.BadRequest(c, fmt.Sprintf("No .p12 signing identity provided in form data - %s", err))
			return
		}
		if !strings.HasSuffix(strings.ToLower(p12File.Filename), ".p12") {
			api.BadRequest(c, "Only .p12 files are allowed for the signing identity")
			return
		}
		p12Path := filepath.Join(tmpDir, "identity.p12")
		if err := c.SaveUploadedFile(p12File, p12Path); err != nil {
			api.InternalError(c, fmt.Sprintf("Failed to store .p12 identity - %s", err))
			return
		}
		opts.P12Path = p12Path
		opts.Password = c.PostForm("p12_password")
	case "certkey":
		certFile, err := c.FormFile("cert")
		if err != nil {
			api.BadRequest(c, fmt.Sprintf("No certificate provided in form data - %s", err))
			return
		}
		keyFile, err := c.FormFile("key")
		if err != nil {
			api.BadRequest(c, fmt.Sprintf("No private key provided in form data - %s", err))
			return
		}
		certPath := filepath.Join(tmpDir, "cert.pem")
		keyPath := filepath.Join(tmpDir, "key.pem")
		if err := c.SaveUploadedFile(certFile, certPath); err != nil {
			api.InternalError(c, fmt.Sprintf("Failed to store certificate - %s", err))
			return
		}
		if err := c.SaveUploadedFile(keyFile, keyPath); err != nil {
			api.InternalError(c, fmt.Sprintf("Failed to store private key - %s", err))
			return
		}
		opts.CertPath = certPath
		opts.KeyPath = keyPath
		opts.Password = c.PostForm("key_password")
	default:
		api.BadRequest(c, "Unknown signing method - use 'p12' or 'certkey'")
		return
	}

	zsignLog, signErr := signing.Sign(opts)
	if signErr != nil {
		api.BadRequest(c, fmt.Sprintf("Signing failed - %s\n\nzsign output:\n%s", signErr, tailString(zsignLog, 1500)))
		return
	}
	if fi, statErr := os.Stat(outputPath); statErr != nil || fi.Size() == 0 {
		api.BadRequest(c, fmt.Sprintf("Signing did not produce an output IPA - check the signing material and provisioning profile.\n\nzsign output:\n%s", tailString(zsignLog, 1500)))
		return
	}

	signedFile, err := os.Open(outputPath)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to open signed IPA - %s", err))
		return
	}
	defer signedFile.Close()

	metadata := bson.M{
		"type":          fileType,
		"description":   c.PostForm("description"),
		"uploaded_by":   c.GetString("username"),
		"original_name": ipaFile.Filename,
	}
	if err := db.GlobalMongoStore.UploadFileWithMetadata(signedFile, uuid.NewString(), metadata, false); err != nil {
		api.InternalError(c, fmt.Sprintf("IPA signed but failed to store it - %s", err))
		return
	}

	api.OKMessage(c, fmt.Sprintf("`%s` signed and uploaded successfully", ipaFile.Filename))
}

// SignAndUploadWebDriverAgentFile godoc
// @Summary      Sign and upload a WebDriverAgent IPA
// @Description  Resign an unsigned WebDriverAgent IPA with zsign using the provided provisioning profile and signing material (either a .p12 + password, or a certificate + private key), then store the signed result like a normal WebDriverAgent upload.
// @Tags         Hub - Admin - Files
// @Accept       multipart/form-data
// @Produce      json
// @Param        file          formData  file    true   "Unsigned WebDriverAgent IPA file"
// @Param        profile       formData  file    true   "Provisioning profile (.mobileprovision)"
// @Param        method        formData  string  false  "Signing method: 'p12' (default) or 'certkey'"
// @Param        p12           formData  file    false  "Signing identity (.p12) - method p12"
// @Param        p12_password  formData  string  false  "Password for the .p12 identity"
// @Param        cert          formData  file    false  "Certificate (.cer/.pem) - method certkey"
// @Param        key           formData  file    false  "Private key (.key/.pem) - method certkey"
// @Param        key_password  formData  string  false  "Password for the private key, if encrypted"
// @Param        description   formData  string  false  "Optional description to tell builds apart"
// @Success      200           {object}  models.SuccessResponse
// @Failure      400           {object}  models.ErrorResponse
// @Failure      500           {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/files/webdriveragent/sign [post]
func SignAndUploadWebDriverAgentFile(c *gin.Context) {
	signAndUploadIPAFile(c, "wda", "WebDriverAgent IPA")
}

// SignAndUploadBroadcastFile godoc
// @Summary      Sign and upload a broadcast extension IPA
// @Description  Resign an unsigned GADS broadcast extension IPA with zsign using the provided provisioning profile and signing material (either a .p12 + password, or a certificate + private key), then store the signed result like a normal broadcast extension upload.
// @Tags         Hub - Admin - Files
// @Accept       multipart/form-data
// @Produce      json
// @Param        file          formData  file    true   "Unsigned broadcast extension IPA file"
// @Param        profile       formData  file    true   "Provisioning profile (.mobileprovision)"
// @Param        method        formData  string  false  "Signing method: 'p12' (default) or 'certkey'"
// @Param        p12           formData  file    false  "Signing identity (.p12) - method p12"
// @Param        p12_password  formData  string  false  "Password for the .p12 identity"
// @Param        cert          formData  file    false  "Certificate (.cer/.pem) - method certkey"
// @Param        key           formData  file    false  "Private key (.key/.pem) - method certkey"
// @Param        key_password  formData  string  false  "Password for the private key, if encrypted"
// @Param        description   formData  string  false  "Optional description to tell builds apart"
// @Success      200           {object}  models.SuccessResponse
// @Failure      400           {object}  models.ErrorResponse
// @Failure      500           {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/files/broadcast/sign [post]
func SignAndUploadBroadcastFile(c *gin.Context) {
	signAndUploadIPAFile(c, "broadcast", "broadcast extension IPA")
}

// UploadSupervisionProfile godoc
// @Summary      Upload the iOS supervision profile
// @Description  Upload the supervision profile (.p12). This is a single file - re-uploading replaces the existing one.
// @Tags         Hub - Admin - Files
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "Supervision profile .p12 file"
// @Success      200   {object}  models.SuccessResponse
// @Failure      400   {object}  models.ErrorResponse
// @Failure      500   {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/files/supervision [post]
func UploadSupervisionProfile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		api.BadRequest(c, fmt.Sprintf("No file provided in form data - %s", err))
		return
	}
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".p12") {
		api.BadRequest(c, "Only .p12 files are allowed for the supervision profile")
		return
	}

	openedFile, err := file.Open()
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to open provided file - %s", err))
		return
	}
	defer openedFile.Close()

	metadata := bson.M{
		"type":          "supervision",
		"uploaded_by":   c.GetString("username"),
		"original_name": file.Filename,
	}

	// The supervision profile is a single fixed-name file - force replace it.
	err = db.GlobalMongoStore.UploadFileWithMetadata(openedFile, "supervision.p12", metadata, true)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to upload supervision profile to MongoDB - %s", err))
		return
	}

	api.OKMessage(c, "Supervision profile uploaded successfully")
}

// DeleteFile godoc
// @Summary      Delete a file
// @Description  Delete a file stored in MongoDB GridFS by its id
// @Tags         Hub - Admin - Files
// @Produce      json
// @Param        id   path      string  true  "File id"
// @Success      200  {object}  models.SuccessResponse
// @Failure      400  {object}  models.ErrorResponse
// @Failure      500  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/files/{id} [delete]
func DeleteFile(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "No file id provided")
		return
	}
	if err := db.GlobalMongoStore.DeleteFileByID(id); err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to delete file - %s", err))
		return
	}
	api.OKMessage(c, "File deleted successfully")
}

// GetApps godoc
// @Summary      List uploaded device apps
// @Description  Retrieve the uploaded app files (apk/ipa/zip) stored in MongoDB GridFS for installing on devices
// @Tags         Hub - Apps
// @Produce      json
// @Success      200  {object}  models.FileListResponse
// @Failure      500  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /apps [get]
func GetApps(c *gin.Context) {
	files, err := db.GlobalMongoStore.GetFilesByType("app")
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to retrieve apps - %s", err))
		return
	}
	api.OK(c, "Successfully retrieved apps", files)
}

// UploadApp godoc
// @Summary      Upload a device app
// @Description  Upload an .apk/.ipa/.zip to MongoDB GridFS. Multiple builds can coexist; each is stored under a unique generated name with an optional description and the uploader recorded, to be installed on devices later.
// @Tags         Hub - Apps
// @Accept       multipart/form-data
// @Produce      json
// @Param        file         formData  file    true   "App file (.apk/.ipa/.zip)"
// @Param        description  formData  string  false  "Optional description to tell builds apart"
// @Success      200          {object}  models.SuccessResponse
// @Failure      400          {object}  models.ErrorResponse
// @Failure      500          {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /apps [post]
func UploadApp(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		api.BadRequest(c, fmt.Sprintf("No file provided in form data - %s", err))
		return
	}

	name := strings.ToLower(file.Filename)
	allowed := false
	for _, ext := range []string{".apk", ".ipa", ".zip", ".wgt", ".ipk"} {
		if strings.HasSuffix(name, ext) {
			allowed = true
			break
		}
	}
	if !allowed {
		api.BadRequest(c, "Only .apk, .ipa, .zip, .wgt or .ipk files are allowed for app uploads")
		return
	}

	openedFile, err := file.Open()
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to open provided file - %s", err))
		return
	}
	defer openedFile.Close()

	metadata := bson.M{
		"type":          "app",
		"description":   c.PostForm("description"),
		"uploaded_by":   c.GetString("username"),
		"original_name": file.Filename,
	}

	if err := db.GlobalMongoStore.UploadFileWithMetadata(openedFile, uuid.NewString(), metadata, false); err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to upload app to MongoDB - %s", err))
		return
	}

	api.OKMessage(c, fmt.Sprintf("`%s` uploaded successfully", file.Filename))
}

// DeleteApp godoc
// @Summary      Delete an uploaded device app
// @Description  Delete an uploaded app file from MongoDB GridFS by its id. Only files tagged as apps can be deleted. Admins may delete any app; regular users may only delete apps they uploaded.
// @Tags         Hub - Apps
// @Produce      json
// @Param        id   path      string  true  "App file id"
// @Success      200  {object}  models.SuccessResponse
// @Failure      400  {object}  models.ErrorResponse
// @Failure      403  {object}  models.ErrorResponse
// @Failure      404  {object}  models.ErrorResponse
// @Failure      500  {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /apps/{id} [delete]
func DeleteApp(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "No file id provided")
		return
	}

	// Guard against deleting non-app files (e.g. signing/supervision) through this
	// all-user endpoint.
	file, err := db.GlobalMongoStore.GetFileByID(id)
	if err != nil || file.Metadata.Type != "app" {
		api.NotFound(c, fmt.Sprintf("No app found with id `%s`", id))
		return
	}

	// Admins can delete any uploaded app; regular users only their own uploads.
	if c.GetString("role") != "admin" && file.Metadata.UploadedBy != c.GetString("username") {
		api.Forbidden(c, "You can only delete apps you uploaded")
		return
	}

	if err := db.GlobalMongoStore.DeleteFileByID(id); err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to delete app - %s", err))
		return
	}
	api.OKMessage(c, "App deleted successfully")
}

// AddDevice godoc
// @Summary      Add a new device
// @Description  Create a new device in the system
// @Tags         Hub - Admin - Devices
// @Accept       json
// @Produce      json
// @Param        device  body      models.DBDevice  true  "Device data"
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

	var device models.DBDevice
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
// @Param        device  body      models.DBDevice  true  "Device data"
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

	var reqDevice models.DBDevice
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
	Devices           []models.DBDevice   `json:"devices"`
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
		dbDevices = []models.DBDevice{}
	}

	var adminDeviceData = AdminDeviceData{
		Devices:   dbDevices,
		Providers: providerNames,
		DeviceStreamTypes: []models.StreamType{
			models.MJPEGStreamType,
			models.IOSWebRTCFFMpegStreamType,
			models.AndroidWebRTCGadsH264StreamType,
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

	releaseDevice, ok := devices.HubDeviceStore.Get(udid)
	if !ok {
		api.NotFound(c, fmt.Sprintf("Device with udid `%s` not found", udid))
		return
	}

	releaseDevice.Mu.Lock()
	defer releaseDevice.Mu.Unlock()

	if releaseDevice.InUseWSConnection != nil {
		ws.WriteFrame(releaseDevice.InUseWSConnection, ws.NewCloseFrame(ws.NewCloseFrameBody(ws.StatusCode(4000), "released by admin"))) //nolint:errcheck
	}

	releaseDevice.ReleaseLock()

	api.OKMessage(c, "Device was successfully released")
}

type lockDeviceResponse struct {
	UDID        string `json:"udid"`
	LockedBy    string `json:"locked_by"`
	Tenant      string `json:"tenant"`
	ExpiresAtMS int64  `json:"expires_at_ms"`
}

// LockDevice godoc
// @Summary      Lock a device via REST API
// @Description  Acquire an exclusive lock on a device. Authenticate via Authorization header (Bearer token) or ?token= query param (raw token, no Bearer prefix). Optional ?ttl_minutes= (default 10, max 360). If locked by another user returns 409. Admins can take over any lock.
// @Tags         Hub - Devices
// @Produce      json
// @Param        udid         path   string  true   "Device UDID"
// @Param        token        query  string  false  "Raw JWT token (alternative to Authorization header)"
// @Param        ttl_minutes  query  int     false  "Lock TTL in minutes (default 10, max 360)"
// @Success      200   {object}  lockDeviceResponse
// @Failure      401   {object}  models.ErrorResponse
// @Failure      404   {object}  models.ErrorResponse
// @Failure      409   {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /devices/control/{udid}/lock [post]
func LockDevice(c *gin.Context) {
	udid := c.Param("udid")

	claims, err := auth.GetClaimsFromRequest(c)
	if err != nil || claims.Username == "" {
		c.Status(http.StatusUnauthorized)
		return
	}

	ttl, _ := strconv.Atoi(c.DefaultQuery("ttl_minutes", strconv.Itoa(defaultLockTTLMinutes)))
	ttl = normalizeLockTTLMinutes(ttl)

	device, ok := devices.HubDeviceStore.Get(udid)
	if !ok {
		api.NotFound(c, fmt.Sprintf("Device with udid `%s` not found", udid))
		return
	}

	device.Mu.Lock()
	defer device.Mu.Unlock()

	if device.IsLockedByOther(claims.Username, claims.Tenant) {
		if claims.Role != "admin" {
			api.Conflict(c, fmt.Sprintf("Device `%s` is already locked by another user", udid))
			return
		}
		// Admin takeover: kick the current holder out via close frame if UI session
		if device.InUseWSConnection != nil {
			ws.WriteFrame(device.InUseWSConnection, ws.NewCloseFrame(ws.NewCloseFrameBody(ws.StatusCode(4000), "released by admin"))) //nolint:errcheck
		}
		device.ReleaseLock()
	}

	device.AcquireLock(claims.Username, claims.Tenant, devices.LockSourceAPI) //nolint:errcheck — IsLockedByOther already checked above
	expiresAt := time.Now().Add(time.Duration(ttl) * time.Minute).UnixMilli()
	device.LeaseExpiresAt = expiresAt

	c.JSON(http.StatusOK, lockDeviceResponse{
		UDID:        udid,
		LockedBy:    claims.Username,
		Tenant:      claims.Tenant,
		ExpiresAtMS: expiresAt,
	})
}

// UnlockDevice godoc
// @Summary      Unlock a device via REST API
// @Description  Release a lock on a device. No-op if device is already free. Returns 409 if locked by another user (admins can always unlock). Authenticate via Authorization header or ?token= query param.
// @Tags         Hub - Devices
// @Produce      json
// @Param        udid   path   string  true   "Device UDID"
// @Param        token  query  string  false  "Raw JWT token (alternative to Authorization header)"
// @Success      200   {object}  models.SuccessResponse
// @Failure      401   {object}  models.ErrorResponse
// @Failure      404   {object}  models.ErrorResponse
// @Failure      409   {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /devices/control/{udid}/unlock [post]
func UnlockDevice(c *gin.Context) {
	udid := c.Param("udid")

	claims, err := auth.GetClaimsFromRequest(c)
	if err != nil || claims.Username == "" {
		c.Status(http.StatusUnauthorized)
		return
	}

	device, ok := devices.HubDeviceStore.Get(udid)
	if !ok {
		api.NotFound(c, fmt.Sprintf("Device with udid `%s` not found", udid))
		return
	}

	device.Mu.Lock()
	defer device.Mu.Unlock()

	if !device.IsLocked() {
		api.OKMessage(c, "Device is not locked")
		return
	}

	if device.IsLockedByOther(claims.Username, claims.Tenant) {
		if claims.Role != "admin" {
			api.Conflict(c, fmt.Sprintf("Device `%s` is locked by another user", udid))
			return
		}
		// Admin kick-out via close frame if UI session
		if device.InUseWSConnection != nil {
			ws.WriteFrame(device.InUseWSConnection, ws.NewCloseFrame(ws.NewCloseFrameBody(ws.StatusCode(4000), "released by admin"))) //nolint:errcheck
		}
	}

	device.ReleaseLock()
	api.OKMessage(c, "Device successfully unlocked")
}

// syncDeviceFields synchronizes operational fields from provider to hub device.
func syncDeviceFields(target *devices.LocalHubDevice, source *models.ProviderDeviceSync) {
	if target.Connected != source.Connected {
		target.Connected = source.Connected
	}
	if target.ProviderState != source.ProviderState {
		target.ProviderState = source.ProviderState
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

	for i := range providerDeviceData.DeviceData {
		providerDevice := &providerDeviceData.DeviceData[i]
		hubDevice, ok := devices.HubDeviceStore.Get(providerDevice.UDID)
		if !ok {
			continue
		}
		hubDevice.Mu.Lock()
		// If device is not connected reset all fields that might allow it to get stuck in Running automation state
		if !providerDevice.Connected {
			hubDevice.Connected = false
			hubDevice.ProviderState = providerDevice.ProviderState
			hubDevice.Host = providerDevice.Host
			hubDevice.IsAvailableForAutomation = false
			hubDevice.IsRunningAutomation = false
			hubDevice.ReleaseLockIfNotHeld()
			hubDevice.SessionID = ""
			hubDevice.Mu.Unlock()
			continue
		}
		// Stamp when we last heard from the provider about this device
		hubDevice.LastUpdatedTimestamp = time.Now().UnixMilli()

		syncDeviceFields(hubDevice, providerDevice)
		hubDevice.Mu.Unlock()
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
