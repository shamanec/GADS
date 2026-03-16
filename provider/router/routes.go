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
	"GADS/common/api"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/common/utils"
	"GADS/device"
	"GADS/device/android"
	"GADS/device/manager"
	"GADS/device/tizen"
	"GADS/device/webos"
	"GADS/provider/config"
	"GADS/provider/logger"

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

	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		c.JSON(http.StatusNotFound, createAppiumErrorResponse(fmt.Sprintf("Device `%s` not found", udid)))
		return
	}

	target := "http://localhost:" + dev.Info().AppiumPort
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
	uploadDir := fmt.Sprintf("%s/", config.ProviderConfig.ProviderFolder)

	file, err := c.FormFile("file")
	if err != nil {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("No file provided in form data - %s", err), nil)
		return
	}

	allowedExtensions := []string{"ipa", "zip", "apk", "wgt", "ipk"}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	isAllowed := false
	for _, allowedExt := range allowedExtensions {
		if ext == "."+allowedExt {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Files with extension `%s` are not allowed", ext), nil)
		return
	}

	udid := c.Param("udid")
	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}

	if ext != ".zip" {
		dst := uploadDir + file.Filename
		os.Remove(dst)
		if err := c.SaveUploadedFile(file, dst); err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to save file to `%s` - %s", dst, err), nil)
			return
		}
		defer os.Remove(dst)

		if err := dev.InstallApp(dst); err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed installing app - %s", err), nil)
			return
		}
		if err := os.Remove(dst); err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, "App uploaded and installed successfully but failed to delete it", nil)
			return
		}
		api.GenericResponse(c, http.StatusOK, "App uploaded and installed successfully", nil)
		return
	}

	// zip handling
	f, err := file.Open()
	if err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to open provided zip file - %s", err), nil)
		return
	}
	defer f.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to read provided zip file - %s", err), nil)
		return
	}

	fileNames, err := utils.ListFilesInZip(buf.Bytes())
	if err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to get file list from provided zip file - %s", err), nil)
		return
	}
	if len(fileNames) < 1 {
		api.GenericResponse(c, http.StatusBadRequest, "Provided zip file is empty", nil)
		return
	}

	if strings.HasSuffix(fileNames[0], ".apk") || strings.HasSuffix(fileNames[0], ".ipa") {
		if err := utils.UnzipInMemory(buf.Bytes(), uploadDir); err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to unzip the file - %s", err), nil)
			return
		}
		appPath := uploadDir + "/" + fileNames[0]
		if err := dev.InstallApp(appPath); err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to install app - %s", err), nil)
			return
		}
		defer func() {
			if err := utils.DeleteFile(appPath); err != nil {
				logger.ProviderLogger.LogError("upload_and_install_app", fmt.Sprintf("Failed to delete app file - %s", err))
			}
		}()
	} else if strings.Contains(fileNames[0], ".app") {
		if err := utils.UnzipInMemory(buf.Bytes(), uploadDir); err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to unzip .app directory - %s", err), nil)
			return
		}
		appPath := uploadDir + "/" + fileNames[0]
		if err := dev.InstallApp(appPath); err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to install unzipped .app directory - %s", err), nil)
			return
		}
		defer func() {
			if err := utils.DeleteFolder(appPath); err != nil {
				logger.ProviderLogger.LogError("upload_and_install_app", "Failed to delete unzipped .app directory")
			}
		}()
	}
	api.GenericResponse(c, http.StatusOK, "App uploaded and installed successfully", nil)
}

func GetProviderData(c *gin.Context) {
	infos := DevManager.AllDeviceInfos()
	providerData := manager.ProviderPayload{
		Provider:   *config.ProviderConfig,
		DeviceData: infos,
	}
	api.GenericResponse(c, http.StatusOK, "", providerData)
}

type WdaOrientationResponse struct {
	Orientation string `json:"value"`
}

func DeviceInfo(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}
	info := dev.Info()

	// Update installed apps.
	apps, err := dev.GetInstalledApps()
	if err == nil {
		info.InstalledApps = apps
	}

	// Update current rotation.
	switch info.OS {
	case "android":
		if adev, ok := dev.(*android.AndroidDevice); ok {
			rotation, err := adev.GetCurrentRotation()
			if err == nil {
				info.CurrentRotation = rotation
			} else {
				info.CurrentRotation = "portrait"
			}
		}
	case "ios":
		if ctrl, ok := dev.(device.Controllable); ok {
			// Use a simple WDA orientation request via a direct HTTP call.
			resp, err := http.Get(fmt.Sprintf("http://localhost:%s/orientation", info.WDAPort))
			if err == nil {
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				var wdaResp WdaOrientationResponse
				if json.Unmarshal(body, &wdaResp) == nil {
					info.CurrentRotation = strings.ToLower(wdaResp.Orientation)
				} else {
					info.CurrentRotation = "portrait"
				}
			} else {
				info.CurrentRotation = "portrait"
			}
			_ = ctrl // suppress unused warning
		}
	}

	api.GenericResponse(c, http.StatusOK, "", info)
}

func DeviceInstalledApps(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}
	apps, err := dev.GetInstalledApps()
	if err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to get installed apps: %v", err), nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "", apps)
}

func DeviceChangeRotation(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}

	var requestBody models.DeviceRotation
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	info := dev.Info()
	if info.OS == "android" {
		if adev, ok := dev.(*android.AndroidDevice); ok {
			if err := adev.ChangeRotation(requestBody.Rotation); err != nil {
				api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
				return
			}
		}
	} else {
		// iOS: send orientation to WDA directly.
		orientPayload := struct {
			Orientation string `json:"orientation"`
		}{Orientation: strings.ToUpper(requestBody.Rotation)}
		orientJSON, err := json.Marshal(orientPayload)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%s/orientation", info.WDAPort), bytes.NewReader(orientJSON))
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := netClient.Do(req)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		resp.Body.Close()
	}
	api.GenericResponse(c, http.StatusOK, "Rotation changed", nil)
}

func DevicesInfo(c *gin.Context) {
	infos := DevManager.AllDeviceInfos()
	api.GenericResponse(c, http.StatusOK, "", infos)
}

type ProcessApp struct {
	App string `json:"app"`
}

func UninstallApp(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
		return
	}
	var payloadJson ProcessApp
	if err := json.Unmarshal(payload, &payloadJson); err != nil {
		api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
		return
	}

	installedApps, _ := dev.GetInstalledApps()
	if !slices.Contains(installedApps, payloadJson.App) {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("App `%s` is not installed on device", payloadJson.App), installedApps)
		return
	}

	if err := dev.UninstallApp(payloadJson.App); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to uninstall app `%s`", payloadJson.App), installedApps)
		return
	}
	// Remove from list.
	idx := slices.Index(installedApps, payloadJson.App)
	if idx != -1 {
		installedApps = append(installedApps[:idx], installedApps[idx+1:]...)
	}
	api.GenericResponse(c, http.StatusOK, fmt.Sprintf("Successfully uninstalled app `%s`", payloadJson.App), installedApps)
}

func LaunchApp(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
		return
	}
	var payloadJson ProcessApp
	if err := json.Unmarshal(payload, &payloadJson); err != nil {
		api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
		return
	}

	installedApps, _ := dev.GetInstalledApps()
	if !slices.Contains(installedApps, payloadJson.App) {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("App `%s` is not installed on device", payloadJson.App), installedApps)
		return
	}

	info := dev.Info()
	var launchErr error
	switch info.OS {
	case "tizen":
		if td, ok := dev.(*tizen.TizenDevice); ok {
			launchErr = td.LaunchApp(payloadJson.App)
		}
	case "webos":
		if wd, ok := dev.(*webos.WebOSDevice); ok {
			launchErr = wd.LaunchApp(payloadJson.App)
		}
	default:
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Launch app not supported for OS: %s", info.OS), nil)
		return
	}

	if launchErr != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to launch app `%s`: %v", payloadJson.App, launchErr), nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, fmt.Sprintf("Successfully launched app `%s`", payloadJson.App), installedApps)
}

func CloseApp(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
		return
	}
	var payloadJson ProcessApp
	if err := json.Unmarshal(payload, &payloadJson); err != nil {
		api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
		return
	}

	installedApps, _ := dev.GetInstalledApps()
	if !slices.Contains(installedApps, payloadJson.App) {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("App `%s` is not installed on device", payloadJson.App), installedApps)
		return
	}

	info := dev.Info()
	var closeErr error
	switch info.OS {
	case "tizen":
		if td, ok := dev.(*tizen.TizenDevice); ok {
			closeErr = td.CloseApp(payloadJson.App)
		}
	case "webos":
		if wd, ok := dev.(*webos.WebOSDevice); ok {
			closeErr = wd.CloseApp(payloadJson.App)
		}
	default:
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Close app not supported for OS: %s", info.OS), nil)
		return
	}

	if closeErr != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to close app `%s`: %v", payloadJson.App, closeErr), nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, fmt.Sprintf("Successfully closed app `%s`", payloadJson.App), installedApps)
}

func ResetDevice(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}
	info := dev.Info()
	if info.IsResetting {
		api.GenericResponse(c, http.StatusConflict, "Device setup is already being reset", nil)
		return
	}
	if dev.ProviderState() != "live" {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Only devices in `live` state can be reset, current state is `%s`", dev.ProviderState()), nil)
		return
	}
	dev.Reset("Re-provisioning device")
	api.GenericResponse(c, http.StatusOK, "Initiated device re-provisioning", nil)
}

func UpdateDeviceStreamSettings(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
		return
	}
	var streamSettings models.UpdateStreamSettings
	if err := json.Unmarshal(payload, &streamSettings); err != nil {
		api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
		return
	}

	info := dev.Info()
	common.MutexManager.StreamSettings.Lock()
	defer common.MutexManager.StreamSettings.Unlock()

	if streamSettings.TargetFPS != 0 && streamSettings.TargetFPS != info.StreamTargetFPS {
		info.StreamTargetFPS = streamSettings.TargetFPS
	}
	if streamSettings.JpegQuality != 0 && streamSettings.JpegQuality != info.StreamJpegQuality {
		info.StreamJpegQuality = streamSettings.JpegQuality
	}
	if streamSettings.ScalingFactor != 0 && streamSettings.ScalingFactor != info.StreamScalingFactor {
		info.StreamScalingFactor = streamSettings.ScalingFactor
	}

	if updater, ok := dev.(device.StreamSettingsUpdater); ok {
		if err := updater.UpdateStreamSettings(); err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to update stream settings - %s", err), nil)
			return
		}
	}

	deviceStreamSettings := models.DeviceStreamSettings{
		UDID:                udid,
		StreamTargetFPS:     info.StreamTargetFPS,
		StreamJpegQuality:   info.StreamJpegQuality,
		StreamScalingFactor: info.StreamScalingFactor,
	}
	if err := db.GlobalMongoStore.UpdateDeviceStreamSettings(udid, deviceStreamSettings); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to update device stream settings in the DB", nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "Stream settings updated", nil)
}

func DeviceFiles(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}
	adev, ok := dev.(*android.AndroidDevice)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Functionality not supported on iOS", nil)
		return
	}
	fileTree, err := adev.GetSharedStorageFileTree()
	if err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, "Failed to get shared storage file tree", nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "Successfully got shared storage file tree", fileTree)
}

func PushFileToSharedStorage(c *gin.Context) {
	udid := c.Param("udid")
	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}
	if dev.Info().OS == "ios" {
		api.GenericResponse(c, http.StatusBadRequest, "Functionality not supported for iOS devices", nil)
		return
	}

	destPath := c.PostForm("destPath")
	file, err := c.FormFile("file")
	if err != nil {
		api.GenericResponse(c, http.StatusBadRequest, "Missing file in form data", nil)
		return
	}

	tempPath := filepath.Join(os.TempDir(), file.Filename)
	defer os.Remove(tempPath)
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to save file `%s` to temp dir - %s", file.Filename, err), nil)
		return
	}

	adbCmd := exec.Command("adb", "-s", dev.Info().UDID, "push", tempPath, destPath)
	if _, err := adbCmd.CombinedOutput(); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to push file `%s` to `%s` - %s", file.Filename, destPath, err), nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, fmt.Sprintf("File `%s` successfully pushed to `%s`", file.Filename, destPath), nil)
}

func DeleteFileFromSharedStorage(c *gin.Context) {
	udid := c.Param("udid")
	filePath := c.PostForm("filePath")
	if filePath == "" {
		api.GenericResponse(c, http.StatusBadRequest, "Missing filePath in form data", nil)
		return
	}

	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}
	if dev.Info().OS == "ios" {
		api.GenericResponse(c, http.StatusBadRequest, "Functionality not supported for iOS devices", nil)
		return
	}
	adev, ok := dev.(*android.AndroidDevice)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Not an Android device", nil)
		return
	}
	if err := adev.DeleteFile(filePath); err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to delete file on path `%s`", filePath), nil)
		return
	}
	api.GenericResponse(c, http.StatusOK, "Successfully deleted file", nil)
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
		api.GenericResponse(c, http.StatusBadRequest, "Missing filePath or fileName in form data", nil)
		return
	}
	fileName := filepath.Base(filePath)

	dev, ok := DevManager.GetDevice(udid)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
		return
	}
	if dev.Info().OS == "ios" {
		api.GenericResponse(c, http.StatusBadRequest, "Functionality not supported for iOS devices", nil)
		return
	}
	adev, ok := dev.(*android.AndroidDevice)
	if !ok {
		api.GenericResponse(c, http.StatusBadRequest, "Not an Android device", nil)
		return
	}
	tempFilePath, err := adev.PullFile(filePath, fileName)
	defer os.Remove(tempFilePath)
	if err != nil {
		api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to pull file from path `%s` to a temporary directory", filePath), nil)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	c.Header("Access-Control-Expose-Headers", "Content-Disposition")
	c.File(tempFilePath)
}
