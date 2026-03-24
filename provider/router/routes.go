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
	"bytes"
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

	device := devices.DBDeviceMap[udid]

	target := "http://localhost:" + device.AppiumPort
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

func UploadAndInstallApp(c *gin.Context) {
	// Specify the upload directory
	uploadDir := fmt.Sprintf("%s/", config.ProviderConfig.ProviderFolder)

	// Read the file from the form data
	file, err := c.FormFile("file")
	if err != nil {
		api.BadRequest(c, fmt.Sprintf("No file provided in form data - %s", err))
		return
	}

	allowedExtensions := []string{"ipa", "zip", "apk", "wgt", "ipk"}
	// Check file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	isAllowed := false
	for _, allowedExt := range allowedExtensions {
		if ext == "."+allowedExt {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		api.BadRequest(c, fmt.Sprintf("Files with extension `%s` are not allowed", ext))
		return
	}

	udid := c.Param("udid")
	// Check if the target device is currently provisioned
	devices.DbDeviceMapMutex.RLock()
	dev, ok := devices.DBDeviceMap[udid]
	devices.DbDeviceMapMutex.RUnlock()

	if ok {
		// If the uploaded file is not a zip archive
		if ext != ".zip" {
			// Create file destination based on the provider dir and file name
			dst := uploadDir + file.Filename
			// First try to remove file if it already exists
			err = os.Remove(dst)
			// TODO handle error if it makes sense at all

			// Save the file to the target destination
			if err := c.SaveUploadedFile(file, dst); err != nil {
				api.InternalError(c, fmt.Sprintf("Failed to save file to `%s` - %s", dst, err))
				return
			}

			// Add a remove for the file in a defer func just in case
			defer func() {
				os.Remove(dst)
			}()

			// Try to install the app after saving the file
			err = devices.InstallApp(dev, file.Filename)
			if err != nil {
				api.InternalError(c, fmt.Sprintf("Failed installing app - %s", err))
				return
			}

			// Try to remove the file after installing it
			err = os.Remove(dst)
			if err != nil {
				api.InternalError(c, "App uploaded and installed successfully but failed to delete it")
				return
			}

			api.OKMessage(c, "App uploaded and installed successfully")
			return
		} else {
			// If the uploaded file is a zip archive
			// Open the zip to read it before extracting
			file, err := file.Open()
			defer file.Close()
			if err != nil {
				api.InternalError(c, fmt.Sprintf("Failed to open provided zip file - %s", err))
				return
			}

			// Read the file content into a byte slice
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, file); err != nil {
				api.InternalError(c, fmt.Sprintf("Failed to read provided zip file - %s", err))
				return
			}

			// Get a list of the files in the zip
			fileNames, err := utils.ListFilesInZip(buf.Bytes())
			if err != nil {
				api.InternalError(c, fmt.Sprintf("Failed to get file list from provided zip file - %s", err))
				return
			}

			// Validate there are files inside the zip
			if len(fileNames) < 1 {
				api.BadRequest(c, "Provided zip file is empty")
				return
			}

			// If we got an apk or ipa file - directly extract it
			if strings.HasSuffix(fileNames[0], ".apk") || strings.HasSuffix(fileNames[0], ".ipa") {
				// We use the file content we read above to unzip from memory without storing the zip file at all
				err = utils.UnzipInMemory(buf.Bytes(), uploadDir)
				if err != nil {
					api.InternalError(c, fmt.Sprintf("Failed to unzip the file - %s", err))
					return
				}

				// Attempt to install the unzipped app file
				err = devices.InstallApp(dev, fileNames[0])
				if err != nil {
					api.InternalError(c, fmt.Sprintf("Failed to install app - %s", err))
					return
				}

				// Delete the unzipped file when the function ends
				defer func() {
					err := utils.DeleteFile(uploadDir + "/" + fileNames[0])
					if err != nil {
						logger.ProviderLogger.LogError("upload_and_install_app", fmt.Sprintf("Failed to delete app file - %s", err))
					}
				}()
			} else if strings.Contains(fileNames[0], ".app") {
				// If the file name ends with .app, then its an iOS .app directory
				// We use the file content we read above to unzip from memory without storing the zip file at all
				err = utils.UnzipInMemory(buf.Bytes(), uploadDir)
				if err != nil {
					api.InternalError(c, fmt.Sprintf("Failed to unzip .app directory - %s", err))
					return
				}

				// Attempt to install the unzipped .app directory
				err = devices.InstallApp(dev, fileNames[0])
				if err != nil {
					api.InternalError(c, fmt.Sprintf("Failed to install unzipped .app directory - %s", err))
					return
				}

				// Delete the unzipped .app directory when the function ends
				defer func() {
					err := utils.DeleteFolder(uploadDir + "/" + fileNames[0])
					if err != nil {
						logger.ProviderLogger.LogError("upload_and_install_app", "Failed to delete unzipped .app directory")
					}
				}()
			}
			api.OKMessage(c, "App uploaded and installed successfully")
			return
		}
	}
	api.NotFound(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}

func GetProviderData(c *gin.Context) {
	var providerData models.ProviderData

	devices.DbDeviceMapMutex.RLock()
	deviceData := []models.Device{}
	for _, device := range devices.DBDeviceMap {
		deviceData = append(deviceData, *device)
	}
	devices.DbDeviceMapMutex.RUnlock()

	providerData.ProviderData = *config.ProviderConfig
	providerData.DeviceData = deviceData

	api.OK(c, "Successfully retrieved provider data", providerData)
}

type WdaOrientationResponse struct {
	Orientation string `json:"value"`
}

func DeviceInfo(c *gin.Context) {
	udid := c.Param("udid")

	devices.DbDeviceMapMutex.RLock()
	dev, ok := devices.DBDeviceMap[udid]
	devices.DbDeviceMapMutex.RUnlock()

	if ok {
		if dev.SupportedStreamTypes == nil {
			dev.SupportedStreamTypes = models.StreamTypesForOS(dev.OS)
		}

		devices.UpdateInstalledApps(dev)
		switch dev.OS {
		case "android":
			devices.UpdateCurrentRotation(dev)
		case "ios":
			wdaResp, err := wdaRequest(dev, http.MethodGet, "orientation", nil)
			if err != nil {
				dev.CurrentRotation = "portrait"
				api.OK(c, "", dev)
				return
			}
			defer wdaResp.Body.Close()

			responseBody, _ := io.ReadAll(wdaResp.Body)
			var responseJson WdaOrientationResponse
			err = json.Unmarshal(responseBody, &responseJson)
			if err != nil {
				dev.CurrentRotation = "portrait"
				api.OK(c, "", dev)
				return
			}
			dev.CurrentRotation = strings.ToLower(responseJson.Orientation)
		}

		api.OK(c, "Successfully retrieved device info", dev)
		return
	}
	api.NotFound(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}

func DeviceInstalledApps(c *gin.Context) {
	udid := c.Param("udid")
	var installedApps interface{}

	if dev, ok := devices.DBDeviceMap[udid]; ok {
		switch dev.OS {
		case "ios":
			installedApps = devices.GetInstalledAppsIOS(dev)
		case "android":
			installedApps = devices.GetInstalledAppsAndroidRemoteServer(dev)
		case "tizen":
			installedApps = devices.GetInstalledAppsTizen(dev)
		case "webos":
			installedApps = devices.GetInstalledAppsWebOS(dev)
		}
		api.OK(c, "Successfully retrieved device installed apps", installedApps)
		return
	}
	api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}

func DeviceChangeRotation(c *gin.Context) {
	udid := c.Param("udid")

	devices.DbDeviceMapMutex.RLock()
	dev, ok := devices.DBDeviceMap[udid]
	devices.DbDeviceMapMutex.RUnlock()
	if !ok {
		api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
		return
	}

	var requestBody models.DeviceRotation
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	if dev.OS == "android" {
		devices.ChangeRotationAndroid(dev, requestBody.Rotation)
	} else {
		requestBody := struct {
			Orientation string `json:"orientation"`
		}{
			Orientation: strings.ToUpper(requestBody.Rotation),
		}
		orientationJson, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			api.InternalError(c, err.Error())
			return
		}
		_, err = wdaRequest(dev, http.MethodPost, "orientation", bytes.NewReader(orientationJson))
		if err != nil {
			api.InternalError(c, err.Error())
		}
	}
}

func DevicesInfo(c *gin.Context) {
	deviceList := []*models.Device{}

	devices.DbDeviceMapMutex.RLock()
	for _, device := range devices.DBDeviceMap {
		deviceList = append(deviceList, device)
	}
	devices.DbDeviceMapMutex.RUnlock()

	api.OK(c, "Successfully retrieved devices info", deviceList)
}

type ProcessApp struct {
	App string `json:"app"`
}

func getInstalledAppIDs(device *models.Device) []string {
	var installedApps []string

	switch device.OS {
	case "ios":
		installedApps = devices.GetInstalledAppsBundleIdentifiersIOS(device)
	case "android":
		installedApps = devices.GetInstalledAppsBundleIdentifiersAndroid(device)
	case "tizen":
		tizenApps := devices.GetInstalledAppsTizen(device)
		for _, app := range tizenApps {
			installedApps = append(installedApps, app.AppID)
		}
	case "webos":
		webosApps := devices.GetInstalledAppsWebOS(device)
		for _, app := range webosApps {
			installedApps = append(installedApps, app.AppID)
		}
	}

	return installedApps
}

func UninstallApp(c *gin.Context) {
	udid := c.Param("udid")

	devices.DbDeviceMapMutex.RLock()
	dev, ok := devices.DBDeviceMap[udid]
	devices.DbDeviceMapMutex.RUnlock()

	var installedApps []string
	if ok {
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

		installedApps = getInstalledAppIDs(dev)

		if slices.Contains(installedApps, payloadJson.App) {
			err = devices.UninstallApp(dev, payloadJson.App)
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
		return
	}

	api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}

func LaunchApp(c *gin.Context) {
	udid := c.Param("udid")

	if dev, ok := devices.DBDeviceMap[udid]; ok {
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

		installedApps := getInstalledAppIDs(dev)

		if !slices.Contains(installedApps, payloadJson.App) {
			api.BadRequest(c, fmt.Sprintf("App `%s` is not installed on device", payloadJson.App))
			return
		}

		var launchErr error
		switch dev.OS {
		case "tizen":
			launchErr = devices.LaunchAppTizen(dev, payloadJson.App)
		case "webos":
			launchErr = devices.LaunchAppWebOS(dev, payloadJson.App)
		default:
			api.BadRequest(c, fmt.Sprintf("Launch app not supported for OS: %s", dev.OS))
			return
		}

		if launchErr != nil {
			api.InternalError(c, fmt.Sprintf("Failed to launch app `%s`: %v", payloadJson.App, launchErr))
			return
		}

		api.OK(c, fmt.Sprintf("Successfully launched app `%s`", payloadJson.App), installedApps)
		return
	}

	api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}

func CloseApp(c *gin.Context) {
	udid := c.Param("udid")

	if dev, ok := devices.DBDeviceMap[udid]; ok {
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

		installedApps := getInstalledAppIDs(dev)

		if !slices.Contains(installedApps, payloadJson.App) {
			api.BadRequest(c, fmt.Sprintf("App `%s` is not installed on device", payloadJson.App))
			return
		}

		var closeErr error
		switch dev.OS {
		case "tizen":
			closeErr = devices.CloseAppTizen(dev, payloadJson.App)
		case "webos":
			closeErr = devices.CloseAppWebOS(dev, payloadJson.App)
		default:
			api.BadRequest(c, fmt.Sprintf("Close app not supported for OS: %s", dev.OS))
			return
		}

		if closeErr != nil {
			api.InternalError(c, fmt.Sprintf("Failed to close app `%s`: %v", payloadJson.App, closeErr))
			return
		}

		api.OK(c, fmt.Sprintf("Successfully closed app `%s`", payloadJson.App), installedApps)
		return
	}

	api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}

func KillApp(c *gin.Context) {
	udid := c.Param("udid")
	bundleId := c.Param("bundleId")

	if dev, ok := devices.DBDeviceMap[udid]; ok {
		switch dev.OS {
		case "ios":
			err := devices.KillAppIOS(dev, bundleId)
			if err != nil {
				api.InternalError(c, fmt.Sprintf("Failed killing app with bundle id `%s`", bundleId))
				return
			}
			api.OKMessage(c, fmt.Sprintf("Successfully killed app with bundle id(package name) `%s`", bundleId))
		case "android":
			err := devices.KillAppAndroid(dev, bundleId)
			if err != nil {
				api.InternalError(c, fmt.Sprintf("Failed killing app with bundle id(package name) `%s`", bundleId))
				return
			}
			api.OKMessage(c, fmt.Sprintf("Successfully killed app with bundle id(package name) `%s`", bundleId))
		default:
			api.BadRequest(c, "Killing apps is not supported on Tizen/WebOS")
		}
		return
	}
	api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}

func ResetDevice(c *gin.Context) {
	udid := c.Param("udid")

	devices.DbDeviceMapMutex.RLock()
	device, ok := devices.DBDeviceMap[udid]
	devices.DbDeviceMapMutex.RUnlock()

	if ok {
		if device.IsResetting {
			api.Conflict(c, "Device setup is already being reset")
			return
		}
		if device.ProviderState != "live" {
			api.InternalError(c, fmt.Sprintf("Only devices in `live` state can be reset, current state is `%s`", device.ProviderState))
			return
		}

		devices.ResetLocalDevice(device, "Re-provisioning device")

		api.OKMessage(c, "Initiated device re-provisioning")
		return
	}

	api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}

func UpdateDeviceStreamSettings(c *gin.Context) {
	udid := c.Param("udid")

	devices.DbDeviceMapMutex.RLock()
	device, ok := devices.DBDeviceMap[udid]
	devices.DbDeviceMapMutex.RUnlock()

	if ok {
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

		if device.OS == "ios" {
			if streamSettings.TargetFPS != 0 && streamSettings.TargetFPS != device.StreamTargetFPS {
				device.StreamTargetFPS = streamSettings.TargetFPS
			}
			if streamSettings.JpegQuality != 0 && streamSettings.JpegQuality != device.StreamJpegQuality {
				device.StreamJpegQuality = streamSettings.JpegQuality
			}
			if streamSettings.ScalingFactor != 0 && streamSettings.ScalingFactor != device.StreamScalingFactor {
				device.StreamScalingFactor = streamSettings.ScalingFactor
			}

			err = devices.UpdateWebDriverAgentStreamSettings(device)
			if err != nil {
				api.InternalError(c, fmt.Sprintf("Failed to update stream settings - %s", err))
				return
			}
		} else {
			if streamSettings.TargetFPS != 0 && streamSettings.TargetFPS != device.StreamTargetFPS {
				device.StreamTargetFPS = streamSettings.TargetFPS
			}
			if streamSettings.JpegQuality != 0 && streamSettings.JpegQuality != device.StreamJpegQuality {
				device.StreamJpegQuality = streamSettings.JpegQuality
			}
			if streamSettings.ScalingFactor != 0 && streamSettings.ScalingFactor != device.StreamScalingFactor {
				device.StreamScalingFactor = streamSettings.ScalingFactor
			}

			if err = devices.UpdateGadsStreamSettings(device); err != nil {
				api.InternalError(c, fmt.Sprintf("Failed to update stream settings - %s", err))
				return
			}
		}

		deviceStreamSettings := models.DeviceStreamSettings{
			UDID:                udid,
			StreamTargetFPS:     device.StreamTargetFPS,
			StreamJpegQuality:   device.StreamJpegQuality,
			StreamScalingFactor: device.StreamScalingFactor,
		}

		err = db.GlobalMongoStore.UpdateDeviceStreamSettings(udid, deviceStreamSettings)
		if err != nil {
			api.InternalError(c, "Failed to update device stream settings in the DB")
			return
		}

		api.OKMessage(c, "Stream settings updated")
		return
	}

	api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}

func DeviceFiles(c *gin.Context) {
	udid := c.Param("udid")

	devices.DbDeviceMapMutex.RLock()
	device, ok := devices.DBDeviceMap[udid]
	devices.DbDeviceMapMutex.RUnlock()

	if ok {
		if device.OS == "android" {
			filesResp, err := androidRemoteServerRequest(device, http.MethodGet, "files", nil)
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
		} else {
			api.BadRequest(c, "Functionality not supported on iOS")
		}
	}

	api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}

func PushFileToSharedStorage(c *gin.Context) {
	udid := c.Param("udid")

	devices.DbDeviceMapMutex.RLock()
	device, ok := devices.DBDeviceMap[udid]
	devices.DbDeviceMapMutex.RUnlock()

	if ok {
		if device.OS == "ios" {
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
		adbCmd := exec.Command("adb", "-s", device.UDID, "push", tempPath, destPath)
		_, err = adbCmd.CombinedOutput()
		if err != nil {
			api.InternalError(c, fmt.Sprintf("Failed to push file `%s` to `%s` - %s", file.Filename, destPath, err))
			return
		}

		api.OKMessage(c, fmt.Sprintf("File `%s` successfully pushed to `%s`", file.Filename, destPath))
		return
	}

	api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}

func DeleteFileFromSharedStorage(c *gin.Context) {
	udid := c.Param("udid")
	filePath := c.PostForm("filePath")
	if filePath == "" {
		api.BadRequest(c, "Missing filePath in form data")
		return
	}

	devices.DbDeviceMapMutex.RLock()
	device, ok := devices.DBDeviceMap[udid]
	devices.DbDeviceMapMutex.RUnlock()

	if ok {
		if device.OS == "ios" {
			api.BadRequest(c, "Functionality not supported for iOS devices")
			return
		}

		err := devices.DeleteAndroidSharedStorageFile(device, filePath)
		if err != nil {
			api.InternalError(c, fmt.Sprintf("Failed to delete file on path `%s`", filePath))
			return
		}

		api.OKMessage(c, "Successfully deleted file")
		return
	}
	api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
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

	devices.DbDeviceMapMutex.RLock()
	device, ok := devices.DBDeviceMap[udid]
	devices.DbDeviceMapMutex.RUnlock()

	if ok {
		if device.OS == "ios" {
			api.BadRequest(c, "Functionality not supported for iOS devices")
			return
		}

		tempFilePath, err := devices.PullAndroidSharedStorageFile(device, filePath, fileName)
		defer os.Remove(tempFilePath)
		if err != nil {
			api.InternalError(c, fmt.Sprintf("Failed to pull file from path `%s` to a temporary directory", filePath))
			return
		}

		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
		c.Header("Access-Control-Expose-Headers", "Content-Disposition")
		c.File(tempFilePath)
		return
	}
	api.BadRequest(c, fmt.Sprintf("Did not find device with udid `%s`", udid))
}
