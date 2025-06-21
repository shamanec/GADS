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

			api.GenericResponse(c, http.StatusInternalServerError, "Internal server error", nil)
		}
	}()

	udid := c.Param("udid")

	if !config.ProviderConfig.SetupAppiumServers {
		api.GenericResponse(c, http.StatusServiceUnavailable, "Appium server not available for device", nil)
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
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("No file provided in form data - %s", err), nil)
		return
	}

	allowedExtensions := []string{"ipa", "zip", "apk"}
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
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Files with extension `%s` are not allowed", ext), nil)
		return
	}

	udid := c.Param("udid")
	// Check if the target device is currently provisioned
	if dev, ok := devices.DBDeviceMap[udid]; ok {
		// If the uploaded file is not a zip archive
		if ext != ".zip" {
			// Create file destination based on the provider dir and file name
			dst := uploadDir + file.Filename
			// First try to remove file if it already exists
			err = os.Remove(dst)
			// TODO handle error if it makes sense at all

			// Save the file to the target destination
			if err := c.SaveUploadedFile(file, dst); err != nil {
				api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to save file to `%s` - %s", dst, err), nil)
				return
			}

			// Add a remove for the file in a defer func just in case
			defer func() {
				os.Remove(dst)
			}()

			// Try to install the app after saving the file
			err = devices.InstallApp(dev, file.Filename)
			if err != nil {
				api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed installing app - %s", err), nil)
				return
			}

			// Try to remove the file after installing it
			err = os.Remove(dst)
			if err != nil {
				api.GenericResponse(c, http.StatusInternalServerError, "App uploaded and installed successfully but failed to delete it", nil)
				return
			}

			api.GenericResponse(c, http.StatusOK, "App uploaded and installed successfully", nil)
			return
		} else {
			// If the uploaded file is a zip archive
			// Open the zip to read it before extracting
			file, err := file.Open()
			defer file.Close()
			if err != nil {
				api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to open provided zip file - %s", err), nil)
				return
			}

			// Read the file content into a byte slice
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, file); err != nil {
				api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to read provided zip file - %s", err), nil)
				return
			}

			// Get a list of the files in the zip
			fileNames, err := utils.ListFilesInZip(buf.Bytes())
			if err != nil {
				api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to get file list from provided zip file - %s", err), nil)
				return
			}

			// Validate there are files inside the zip
			if len(fileNames) < 1 {
				api.GenericResponse(c, http.StatusBadRequest, "Provided zip file is empty", nil)
				return
			}

			// If we got an apk or ipa file - directly extract it
			if strings.HasSuffix(fileNames[0], ".apk") || strings.HasSuffix(fileNames[0], ".ipa") {
				// We use the file content we read above to unzip from memory without storing the zip file at all
				err = utils.UnzipInMemory(buf.Bytes(), uploadDir)
				if err != nil {
					api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to unzip the file - %s", err), nil)
					return
				}

				// Attempt to install the unzipped app file
				err = devices.InstallApp(dev, fileNames[0])
				if err != nil {
					api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to install app - %s", err), nil)
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
					api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to unzip .app directory - %s", err), nil)
					return
				}

				// Attempt to install the unzipped .app directory
				err = devices.InstallApp(dev, fileNames[0])
				if err != nil {
					api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to install unzipped .app directory - %s", err), nil)
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
			api.GenericResponse(c, http.StatusOK, "App uploaded and installed successfully", nil)
			return
		}
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
}

func GetProviderData(c *gin.Context) {
	var providerData models.ProviderData

	deviceData := []models.Device{}
	for _, device := range devices.DBDeviceMap {
		deviceData = append(deviceData, *device)
	}

	providerData.ProviderData = *config.ProviderConfig
	providerData.DeviceData = deviceData

	api.GenericResponse(c, http.StatusOK, "", providerData)
}

func DeviceInfo(c *gin.Context) {
	udid := c.Param("udid")

	if dev, ok := devices.DBDeviceMap[udid]; ok {
		devices.UpdateInstalledApps(dev)
		api.GenericResponse(c, http.StatusOK, "", dev)
		return
	}
	api.GenericResponse(c, http.StatusNotFound, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
}

func DeviceInstalledApps(c *gin.Context) {
	udid := c.Param("udid")
	var installedApps []string

	if dev, ok := devices.DBDeviceMap[udid]; ok {
		if dev.OS == "ios" {
			installedApps = devices.GetInstalledAppsIOS(dev)
		} else {
			installedApps = devices.GetInstalledAppsAndroid(dev)
		}
		api.GenericResponse(c, http.StatusOK, "", installedApps)
		return
	}
	api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), installedApps)
}

func DevicesInfo(c *gin.Context) {
	deviceList := []*models.Device{}

	for _, device := range devices.DBDeviceMap {
		deviceList = append(deviceList, device)
	}
	api.GenericResponse(c, http.StatusOK, "", deviceList)
}

type ProcessApp struct {
	App string `json:"app"`
}

func UninstallApp(c *gin.Context) {
	udid := c.Param("udid")

	var installedApps []string
	if dev, ok := devices.DBDeviceMap[udid]; ok {
		payload, err := io.ReadAll(c.Request.Body)
		if err != nil {
			api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
			return
		}

		var payloadJson ProcessApp
		err = json.Unmarshal(payload, &payloadJson)
		if err != nil {
			api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
			return
		}

		if dev.OS == "ios" {
			installedApps = devices.GetInstalledAppsIOS(dev)
		} else {
			installedApps = devices.GetInstalledAppsAndroid(dev)
		}

		if slices.Contains(installedApps, payloadJson.App) {
			err = devices.UninstallApp(dev, payloadJson.App)
			if err != nil {
				api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to uninstall app `%s`", payloadJson.App), installedApps)
				return
			}
			deletedAppIndex := slices.Index(installedApps, payloadJson.App)
			if deletedAppIndex != -1 {
				installedApps = append(installedApps[:deletedAppIndex], installedApps[deletedAppIndex+1:]...)
			}
			api.GenericResponse(c, http.StatusOK, fmt.Sprintf("Successfully uninstalled app `%s`", payloadJson.App), installedApps)
			return
		}
		api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("App `%s` is not installed on device", payloadJson.App), installedApps)
		return
	}

	api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
}

func ResetDevice(c *gin.Context) {
	udid := c.Param("udid")

	if device, ok := devices.DBDeviceMap[udid]; ok {
		if device.IsResetting {
			api.GenericResponse(c, http.StatusConflict, "Device setup is already being reset", nil)
			return
		}
		if device.ProviderState != "live" {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Only devices in `live` state can be reset, current state is `%s`", device.ProviderState), nil)
			return
		}

		devices.ResetLocalDevice(device, "Re-provisioning device")

		api.GenericResponse(c, http.StatusOK, "Initiated device re-provisioning", nil)
		return
	}

	api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
}

func UpdateDeviceStreamSettings(c *gin.Context) {
	udid := c.Param("udid")

	if device, ok := devices.DBDeviceMap[udid]; ok {
		payload, err := io.ReadAll(c.Request.Body)
		if err != nil {
			api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
			return
		}

		var streamSettings models.UpdateStreamSettings
		err = json.Unmarshal(payload, &streamSettings)
		if err != nil {
			api.GenericResponse(c, http.StatusBadRequest, "Invalid payload", nil)
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
				api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to update stream settings - %s", err), nil)
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
				api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to update stream settings - %s", err), nil)
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
			api.GenericResponse(c, http.StatusInternalServerError, "Failed to update device stream settings in the DB", nil)
			return
		}

		api.GenericResponse(c, http.StatusOK, "Stream settings updated", nil)
		return
	}

	api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
}

func DeviceFiles(c *gin.Context) {
	udid := c.Param("udid")

	if device, ok := devices.DBDeviceMap[udid]; ok {
		if device.OS == "android" {
			filesResp, err := androidRemoteServerRequest(device, http.MethodGet, "files", nil)
			if err != nil {
				api.GenericResponse(c, http.StatusInternalServerError, "Failed to get shared storage file tree", nil)
				return
			}
			defer filesResp.Body.Close()

			payload, err := io.ReadAll(filesResp.Body)
			if err != nil {
				api.GenericResponse(c, http.StatusInternalServerError, "Failed to read shared storage file tree response", nil)
				return
			}
			var fileTree models.AndroidFileNode
			err = json.Unmarshal(payload, &fileTree)
			if err != nil {
				api.GenericResponse(c, http.StatusInternalServerError, "Failed to unmarshal storage file tree response", nil)
				return
			}

			api.GenericResponse(c, http.StatusOK, "Successfully got shared storage file tree", fileTree)
			return
		} else {
			api.GenericResponse(c, http.StatusBadRequest, "Functionality not supported on iOS", nil)
		}
	}

	api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
}

func PushFileToSharedStorage(c *gin.Context) {
	udid := c.Param("udid")

	if device, ok := devices.DBDeviceMap[udid]; ok {
		if device.OS == "ios" {
			api.GenericResponse(c, http.StatusBadRequest, "Functionality not supported for iOS devices", nil)
			return
		}

		destPath := c.PostForm("destPath")
		file, err := c.FormFile("file")
		if err != nil {
			api.GenericResponse(c, http.StatusBadRequest, "Missing file in form data", nil)
			return
		}

		// Save uploaded file in a temporary folder so we can push it via adb
		tempPath := filepath.Join(os.TempDir(), file.Filename)

		// Remove the temporary file, we don't want to keep it on long running hosts
		defer os.Remove(tempPath)
		if err := c.SaveUploadedFile(file, tempPath); err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to save file `%s` to temp dir `%s` - %s", file.Filename, tempPath, err.Error), nil)
			return
		}

		// Push the file via adb to from the temporary folder to the target shared storage path
		adbCmd := exec.Command("adb", "-s", device.UDID, "push", tempPath, destPath)
		_, err = adbCmd.CombinedOutput()
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to push file `%s` to `%s` - %s", file.Filename, destPath, err), nil)
			return
		}

		api.GenericResponse(c, http.StatusOK, fmt.Sprintf("File `%s` successfully pushed to `%s`", file.Filename, destPath), nil)
	}

	api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
}

func DeleteFileFromSharedStorage(c *gin.Context) {
	udid := c.Param("udid")
	filePath := c.PostForm("filePath")
	if filePath == "" {
		api.GenericResponse(c, http.StatusBadRequest, "Missing filePath in form data", nil)
		return
	}

	if device, ok := devices.DBDeviceMap[udid]; ok {
		if device.OS == "ios" {
			api.GenericResponse(c, http.StatusBadRequest, "Functionality not supported for iOS devices", nil)
			return
		}

		err := devices.DeleteAndroidSharedStorageFile(device, filePath)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to delete file on path `%s`", filePath), nil)
			return
		}

		api.GenericResponse(c, http.StatusOK, "Successfully deleted file", nil)
		return
	}
	api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
}

func PullFileFromSharedStorage(c *gin.Context) {
	udid := c.Param("udid")
	filePath := c.PostForm("filePath")

	if filePath == "" {
		api.GenericResponse(c, http.StatusBadRequest, "Missing filePath or fileName in form data", nil)
		return
	}
	fileName := filepath.Base(filePath)

	if device, ok := devices.DBDeviceMap[udid]; ok {
		if device.OS == "ios" {
			api.GenericResponse(c, http.StatusBadRequest, "Functionality not supported for iOS devices", nil)
			return
		}

		tempFilePath, err := devices.PullAndroidSharedStorageFile(device, filePath, fileName)
		defer os.Remove(tempFilePath)
		if err != nil {
			api.GenericResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to pull file from path `%s` to a temporary directory", filePath), nil)
			return
		}

		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
		c.Header("Access-Control-Expose-Headers", "Content-Disposition")
		c.File(tempFilePath)
		return
	}
	api.GenericResponse(c, http.StatusBadRequest, fmt.Sprintf("Did not find device with udid `%s`", udid), nil)
}
