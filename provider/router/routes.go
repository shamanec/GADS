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
	"GADS/common/utils"
	"GADS/provider/config"
	"GADS/provider/devices"
	"GADS/provider/logger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"GADS/common"

	"github.com/gin-gonic/gin"
)

func AppiumReverseProxy(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				fmt.Println("Appium Reverse Proxy panic:", err)
			} else {
				fmt.Println("Appium Reverse Proxy panic:", r)
			}

			c.JSON(http.StatusInternalServerError, createAppiumErrorResponse("Internal server error"))
		}
	}()

	udid := c.Param("udid")

	if !config.ProviderConfig.SetupAppiumServers {
		c.JSON(http.StatusServiceUnavailable, createAppiumErrorResponse("Appium server not available for device"))
		return
	}

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		c.JSON(http.StatusNotFound, createAppiumErrorResponse("Device not found"))
		return
	}

	target := "http://localhost:" + platDev.GetAppiumPort()
	path := c.Param("proxyPath")

	proxy := newAppiumProxy(target, path)
	proxy.ServeHTTP(c.Writer, c.Request)
}

func newAppiumProxy(target string, path string) *httputil.ReverseProxy {
	targetURL, _ := url.Parse(target)

	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = targetURL.Path + path
			req.Host = targetURL.Host
			req.Header.Del("Access-Control-Allow-Origin")
		},
	}
}

// allowedAppExtensions are the app file extensions that can be uploaded/installed
// on a device. Tizen .wgt / webOS .ipk are included for installs driven via API
// even though those platforms can't be remotely controlled from the UI.
var allowedAppExtensions = []string{".ipa", ".zip", ".apk", ".wgt", ".ipk"}

// installAppFromDisk installs an app file that is already saved on disk at
// uploadDir/fileName. Plain .apk/.ipa files are installed directly; a .zip is
// read back and unzipped in memory to extract an .apk/.ipa file or an iOS .app
// directory, which is then installed and the extracted artifact cleaned up. The
// caller owns the lifecycle of the uploadDir/fileName file itself.
func installAppFromDisk(platDev devices.PlatformDevice, uploadDir, fileName string) error {
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext != ".zip" {
		return platDev.InstallApp(fileName)
	}

	// Read the zip back from disk so we can unzip it in memory
	zipBytes, err := os.ReadFile(filepath.Join(uploadDir, fileName))
	if err != nil {
		return fmt.Errorf("Failed to read provided zip file - %s", err)
	}

	// Get a list of the files in the zip
	fileNames, err := utils.ListFilesInZip(zipBytes)
	if err != nil {
		return fmt.Errorf("Failed to get file list from provided zip file - %s", err)
	}
	if len(fileNames) < 1 {
		return fmt.Errorf("Provided zip file is empty")
	}

	innerName := fileNames[0]
	// If we got an apk or ipa file - directly extract and install it
	if strings.HasSuffix(innerName, ".apk") || strings.HasSuffix(innerName, ".ipa") {
		if err := utils.UnzipInMemory(zipBytes, uploadDir); err != nil {
			return fmt.Errorf("Failed to unzip the file - %s", err)
		}
		defer func() {
			if err := utils.DeleteFile(filepath.Join(uploadDir, innerName)); err != nil {
				logger.ProviderLogger.LogError("install_app", fmt.Sprintf("Failed to delete app file - %s", err))
			}
		}()
		return platDev.InstallApp(innerName)
	} else if idx := strings.Index(innerName, ".app"); idx != -1 {
		// iOS .app bundle (a directory). Derive the .app root folder from the entry
		// path so both the install and the cleanup target the whole bundle,
		// regardless of which entry the archive happens to list first (some zips
		// omit the explicit directory entry, so fileNames[0] may be a file inside).
		appRoot := innerName[:idx+len(".app")]
		if err := utils.UnzipInMemory(zipBytes, uploadDir); err != nil {
			return fmt.Errorf("Failed to unzip .app directory - %s", err)
		}
		defer func() {
			if err := utils.DeleteFolder(filepath.Join(uploadDir, appRoot)); err != nil {
				logger.ProviderLogger.LogError("install_app", fmt.Sprintf("Failed to delete unzipped .app directory - %s", err))
			}
		}()
		return platDev.InstallApp(appRoot)
	}

	return fmt.Errorf("Zip archive does not contain a supported .apk/.ipa/.app entry")
}

// InstallStoredApp installs an app that was previously uploaded to the hub and
// stored in MongoDB GridFS, identified by its file id. The app is downloaded to
// the provider folder and installed on the device, then cleaned up.
func InstallStoredApp(c *gin.Context) {
	udid := c.Param("udid")
	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.NotFound(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	var payload struct {
		FileID   string `json:"file_id"`
		Filename string `json:"filename"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		api.BadRequest(c, fmt.Sprintf("Invalid request body - %s", err))
		return
	}
	if payload.FileID == "" {
		api.BadRequest(c, "No file_id provided")
		return
	}

	// The original filename is used only to preserve the correct extension
	ext := strings.ToLower(filepath.Ext(payload.Filename))
	if !slices.Contains(allowedAppExtensions, ext) {
		api.BadRequest(c, fmt.Sprintf("Files with extension `%s` are not allowed", ext))
		return
	}

	uploadDir := fmt.Sprintf("%s/", config.ProviderConfig.ProviderFolder)
	// Download under a unique local name that preserves the extension so the
	// install/zip handling works and concurrent installs don't collide
	localName := payload.FileID + ext
	if err := db.GlobalMongoStore.DownloadFileByID(payload.FileID, config.ProviderConfig.ProviderFolder, localName); err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to download app from MongoDB - %s", err))
		return
	}
	// Always clean up the downloaded file when done
	defer os.Remove(uploadDir + localName)

	if err := installAppFromDisk(platDev, uploadDir, localName); err != nil {
		api.InternalError(c, fmt.Sprintf("Failed installing app - %s", err))
		return
	}

	api.OKMessage(c, "App installed successfully")
}

func GetProviderData(c *gin.Context) {
	var providerData models.ProviderData

	allDevs := devices.DevManager.All()
	syncData := make([]models.ProviderDeviceSync, 0, len(allDevs))
	for _, dev := range allDevs {
		syncData = append(syncData, dev.ToSyncUpdate())
	}

	providerData.ProviderData = *config.ProviderConfig
	providerData.DeviceData = syncData

	api.OK(c, "Successfully retrieved provider data", providerData)
}

type WdaOrientationResponse struct {
	Orientation string `json:"value"`
}

// DeviceInfoResponse is the composite response for the DeviceInfo endpoint,
// combining DB fields with all runtime state from the provider.
type DeviceInfoResponse struct {
	models.DBDevice
	// Hub-synced runtime fields
	Host          string `json:"host"`
	Connected     bool   `json:"connected"`
	ProviderState string `json:"provider_state"`
	// Provider-only runtime fields
	HardwareModel        string              `json:"hardware_model"`
	IsResetting          bool                `json:"is_resetting"`
	StreamTargetFPS      int                 `json:"stream_target_fps,omitempty"`
	StreamJpegQuality    int                 `json:"stream_jpeg_quality,omitempty"`
	StreamScalingFactor  int                 `json:"stream_scaling_factor,omitempty"`
	AppiumLastPingTS     int64               `json:"appium_last_ts"`
	AppiumSessionID      string              `json:"appium_session_id"`
	IsAppiumUp           bool                `json:"is_appium_up"`
	HasAppiumSession     bool                `json:"has_appium_session"`
	CurrentRotation      string              `json:"current_rotation"`
	SupportedStreamTypes []models.StreamType `json:"supported_stream_types"`
	InstalledApps        []string            `json:"installed_apps"`
}

func DeviceInfo(c *gin.Context) {
	udid := c.Param("udid")

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.NotFound(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	resp := DeviceInfoResponse{
		DBDevice:        *platDev.GetDBDevice(),
		Host:            platDev.GetHost(),
		Connected:       platDev.IsConnected(),
		ProviderState:   platDev.GetProviderState(),
		HardwareModel:   platDev.GetHardwareModelValue(),
		IsResetting:     platDev.GetIsResetting(),
		AppiumSessionID: platDev.GetAppiumSessionID(),
		IsAppiumUp:      platDev.GetIsAppiumUp(),
		InstalledApps:   platDev.GetInstalledAppBundleIDs(),
	}

	if rcDev, rcOk := platDev.(devices.RemoteControllable); rcOk {
		resp.StreamTargetFPS = rcDev.GetStreamTargetFPS()
		resp.StreamJpegQuality = rcDev.GetStreamJpegQuality()
		resp.StreamScalingFactor = rcDev.GetStreamScalingFactor()
		resp.SupportedStreamTypes = rcDev.GetSupportedStreamTypes()
	}

	if resp.SupportedStreamTypes == nil {
		resp.SupportedStreamTypes = models.StreamTypesForOS(platDev.GetOS())
	}

	switch platDev.GetOS() {
	case "android":
		if rc, rcOk := platDev.(devices.RemoteControllable); rcOk {
			rotation, err := rc.GetCurrentRotation()
			if err == nil {
				resp.CurrentRotation = rotation
			}
		}
	case "ios":
		wdaResp, err := wdaRequest(platDev, http.MethodGet, "orientation", nil)
		if err != nil {
			resp.CurrentRotation = "portrait"
			api.OK(c, "", resp)
			return
		}
		defer wdaResp.Body.Close()

		responseBody, _ := io.ReadAll(wdaResp.Body)
		var responseJson WdaOrientationResponse
		err = json.Unmarshal(responseBody, &responseJson)
		if err != nil {
			resp.CurrentRotation = "portrait"
			api.OK(c, "", resp)
			return
		}
		resp.CurrentRotation = strings.ToLower(responseJson.Orientation)
	}

	api.OK(c, "Successfully retrieved device info", resp)
}

func DeviceInstalledApps(c *gin.Context) {
	udid := c.Param("udid")

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}
	installedApps, err := platDev.GetInstalledApps()
	if err != nil {
		platDev.GetLogger().LogError("device_apps", fmt.Sprintf("Failed to get installed apps - %s", err))
	}
	api.OK(c, "Successfully retrieved device installed apps", installedApps)
}

func DeviceGetRotation(c *gin.Context) {
	udid := c.Param("udid")

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.NotFound(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	rc, rcOk := platDev.(devices.RemoteControllable)
	if !rcOk {
		api.BadRequest(c, "Device does not support rotation")
		return
	}

	rotation, err := rc.GetCurrentRotation()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, "Successfully retrieved device rotation", gin.H{"rotation": rotation})
}

func DeviceChangeRotation(c *gin.Context) {
	udid := c.Param("udid")

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	var requestBody models.DeviceRotation
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if requestBody.Rotation != "portrait" && requestBody.Rotation != "landscape" {
		api.BadRequest(c, fmt.Sprintf("Invalid rotation `%s`, expected `portrait` or `landscape`", requestBody.Rotation))
		return
	}

	rc, rcOk := platDev.(devices.RemoteControllable)
	if !rcOk {
		api.BadRequest(c, "Device does not support rotation")
		return
	}

	if err := rc.ChangeRotation(requestBody.Rotation); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	currentRotation := requestBody.Rotation
	if rotation, err := rc.GetCurrentRotation(); err == nil {
		currentRotation = rotation
	}

	api.OK(c, "Device rotation request processed", gin.H{
		"rotation": currentRotation,
		"applied":  currentRotation == requestBody.Rotation,
	})
}

func DevicesInfo(c *gin.Context) {
	allDevs := devices.DevManager.All()
	syncList := make([]models.ProviderDeviceSync, 0, len(allDevs))
	for _, dev := range allDevs {
		syncList = append(syncList, dev.ToSyncUpdate())
	}

	api.OK(c, "Successfully retrieved devices info", syncList)
}

type ProcessApp struct {
	App string `json:"app"`
}

func UninstallApp(c *gin.Context) {
	udid := c.Param("udid")

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.BadRequest(c, "Invalid payload")
		return
	}

	var payloadJson ProcessApp
	err = json.Unmarshal(payload, &payloadJson)
	if err != nil {
		api.BadRequest(c, "Invalid payload")
		return
	}

	installedApps := platDev.GetInstalledAppBundleIDs()

	if slices.Contains(installedApps, payloadJson.App) {
		err = platDev.UninstallApp(payloadJson.App)
		if err != nil {
			api.InternalError(c, fmt.Sprintf("Failed to uninstall app `%s`", payloadJson.App))
			return
		}
		deletedAppIndex := slices.Index(installedApps, payloadJson.App)
		if deletedAppIndex != -1 {
			installedApps = append(installedApps[:deletedAppIndex], installedApps[deletedAppIndex+1:]...)
		}
		api.OK(c, fmt.Sprintf("Successfully uninstalled app `%s`", payloadJson.App), installedApps)
		return
	}
	api.BadRequest(c, fmt.Sprintf("App `%s` is not installed on device", payloadJson.App))
}

func LaunchApp(c *gin.Context) {
	udid := c.Param("udid")

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.BadRequest(c, "Invalid payload")
		return
	}

	var payloadJson ProcessApp
	err = json.Unmarshal(payload, &payloadJson)
	if err != nil {
		api.BadRequest(c, "Invalid payload")
		return
	}

	installedApps := platDev.GetInstalledAppBundleIDs()

	if !slices.Contains(installedApps, payloadJson.App) {
		api.BadRequest(c, fmt.Sprintf("App `%s` is not installed on device", payloadJson.App))
		return
	}

	launchErr := platDev.LaunchApp(payloadJson.App)
	if launchErr != nil {
		api.InternalError(c, fmt.Sprintf("Failed to launch app `%s`: %v", payloadJson.App, launchErr))
		return
	}

	api.OK(c, fmt.Sprintf("Successfully launched app `%s`", payloadJson.App), installedApps)
}

func CloseApp(c *gin.Context) {
	udid := c.Param("udid")

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.BadRequest(c, "Invalid payload")
		return
	}

	var payloadJson ProcessApp
	err = json.Unmarshal(payload, &payloadJson)
	if err != nil {
		api.BadRequest(c, "Invalid payload")
		return
	}

	installedApps := platDev.GetInstalledAppBundleIDs()

	if !slices.Contains(installedApps, payloadJson.App) {
		api.BadRequest(c, fmt.Sprintf("App `%s` is not installed on device", payloadJson.App))
		return
	}

	closeErr := platDev.KillApp(payloadJson.App)
	if closeErr != nil {
		api.InternalError(c, fmt.Sprintf("Failed to close app `%s`: %v", payloadJson.App, closeErr))
		return
	}

	api.OK(c, fmt.Sprintf("Successfully closed app `%s`", payloadJson.App), installedApps)
}

func KillApp(c *gin.Context) {
	udid := c.Param("udid")
	bundleId := c.Query("bundleId")

	if bundleId == "" {
		api.BadRequest(c, "No bundleId url param sent")
		return
	}

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	err := platDev.KillApp(bundleId)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed killing app with bundle id(package name) `%s`", bundleId))
		return
	}
	api.OKMessage(c, fmt.Sprintf("Successfully killed app with bundle id(package name) `%s`", bundleId))
}

func ResetDevice(c *gin.Context) {
	udid := c.Param("udid")

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	if platDev.GetIsResetting() {
		api.Conflict(c, "Device setup is already being reset")
		return
	}
	if platDev.GetProviderState() != "live" {
		api.InternalError(c, fmt.Sprintf("Only devices in `live` state can be reset, current state is `%s`", platDev.GetProviderState()))
		return
	}

	platDev.Reset("Re-provisioning device")

	api.OKMessage(c, "Initiated device re-provisioning")
}

func UpdateDeviceStreamSettings(c *gin.Context) {
	udid := c.Param("udid")

	platDev, deviceFound := devices.DevManager.Get(udid)
	if !deviceFound {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}
	rcDev, isRcDevice := platDev.(devices.RemoteControllable)
	if !isRcDevice {
		api.BadRequest(c, fmt.Sprintf("Device `%s` does not support stream settings", udid))
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.BadRequest(c, "Invalid payload")
		return
	}

	var streamSettings models.UpdateStreamSettings
	err = json.Unmarshal(payload, &streamSettings)
	if err != nil {
		api.BadRequest(c, "Invalid payload")
		return
	}

	common.MutexManager.StreamSettings.Lock()
	defer common.MutexManager.StreamSettings.Unlock()

	if streamSettings.TargetFPS != 0 && streamSettings.TargetFPS != rcDev.GetStreamTargetFPS() {
		rcDev.SetStreamTargetFPS(streamSettings.TargetFPS)
	}
	if streamSettings.JpegQuality != 0 && streamSettings.JpegQuality != rcDev.GetStreamJpegQuality() {
		rcDev.SetStreamJpegQuality(streamSettings.JpegQuality)
	}
	if streamSettings.ScalingFactor != 0 && streamSettings.ScalingFactor != rcDev.GetStreamScalingFactor() {
		rcDev.SetStreamScalingFactor(streamSettings.ScalingFactor)
	}

	if err = rcDev.UpdateStreamSettingsOnDevice(); err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to update stream settings - %s", err))
		return
	}

	deviceStreamSettings := models.DeviceStreamSettings{
		UDID:                udid,
		StreamTargetFPS:     rcDev.GetStreamTargetFPS(),
		StreamJpegQuality:   rcDev.GetStreamJpegQuality(),
		StreamScalingFactor: rcDev.GetStreamScalingFactor(),
	}

	err = db.GlobalMongoStore.UpdateDeviceStreamSettings(udid, deviceStreamSettings)
	if err != nil {
		api.InternalError(c, "Failed to update device stream settings in the DB")
		return
	}

	api.OKMessage(c, "Stream settings updated")
}

func DeviceFiles(c *gin.Context) {
	udid := c.Param("udid")

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	if platDev.GetOS() == "android" {
		filesResp, err := androidRemoteServerRequest(platDev, http.MethodGet, "files", nil)
		if err != nil {
			api.InternalError(c, "Failed to get shared storage file tree")
			return
		}
		defer filesResp.Body.Close()

		payload, err := io.ReadAll(filesResp.Body)
		if err != nil {
			api.InternalError(c, "Failed to read shared storage file tree response")
			return
		}
		var fileTree models.AndroidFileNode
		err = json.Unmarshal(payload, &fileTree)
		if err != nil {
			api.InternalError(c, "Failed to unmarshal storage file tree response")
			return
		}

		api.OK(c, "Successfully got shared storage file tree", fileTree)
		return
	}
	api.BadRequest(c, "Functionality not supported on iOS")
}

func PushFileToSharedStorage(c *gin.Context) {
	udid := c.Param("udid")

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	if platDev.GetOS() == "ios" {
		api.BadRequest(c, "Functionality not supported for iOS devices")
		return
	}

	destPath := c.PostForm("destPath")
	file, err := c.FormFile("file")
	if err != nil {
		api.BadRequest(c, "Missing file in form data")
		return
	}

	// Save uploaded file in a temporary folder so we can push it via adb
	tempPath := filepath.Join(os.TempDir(), file.Filename)

	// Remove the temporary file, we don't want to keep it on long running hosts
	defer os.Remove(tempPath)
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to save file `%s` to temp dir `%s` - %s", file.Filename, tempPath, err.Error()))
		return
	}

	// Push the file via adb to from the temporary folder to the target shared storage path
	adbCmd := exec.Command("adb", "-s", platDev.GetUDID(), "push", tempPath, destPath)
	_, err = adbCmd.CombinedOutput()
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to push file `%s` to `%s` - %s", file.Filename, destPath, err))
		return
	}

	api.OKMessage(c, fmt.Sprintf("File `%s` successfully pushed to `%s`", file.Filename, destPath))
}

func DeleteFileFromSharedStorage(c *gin.Context) {
	udid := c.Param("udid")
	filePath := c.PostForm("filePath")
	if filePath == "" {
		api.BadRequest(c, "Missing filePath in form data")
		return
	}

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	if platDev.GetOS() == "ios" {
		api.BadRequest(c, "Functionality not supported for iOS devices")
		return
	}

	device := platDev.GetDBDevice()
	err := devices.DeleteAndroidSharedStorageFile(device, filePath)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to delete file on path `%s`", filePath))
		return
	}

	api.OKMessage(c, "Successfully deleted file")
}

func createAppiumErrorResponse(message string) gin.H {
	return gin.H{
		"value": gin.H{
			"error":      "unknown error",
			"message":    message,
			"stacktrace": "",
		},
	}
}

func PullFileFromSharedStorage(c *gin.Context) {
	udid := c.Param("udid")
	filePath := c.PostForm("filePath")

	if filePath == "" {
		api.BadRequest(c, "Missing filePath or fileName in form data")
		return
	}
	fileName := filepath.Base(filePath)

	platDev, ok := devices.DevManager.Get(udid)
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	if platDev.GetOS() == "ios" {
		api.BadRequest(c, "Functionality not supported for iOS devices")
		return
	}

	device := platDev.GetDBDevice()
	tempFilePath, err := devices.PullAndroidSharedStorageFile(device, filePath, fileName)
	defer os.Remove(tempFilePath)
	if err != nil {
		api.InternalError(c, fmt.Sprintf("Failed to pull file from path `%s` to a temporary directory", filePath))
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	c.Header("Access-Control-Expose-Headers", "Content-Disposition")
	c.File(tempFilePath)
}
