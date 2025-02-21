package router

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/common/util"
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
	"path/filepath"
	"slices"
	"strings"

	"GADS/common"

	"github.com/gin-gonic/gin"
)

type JsonErrorResponse struct {
	EventName    string `json:"event"`
	ErrorMessage string `json:"error_message"`
}

type JsonResponse struct {
	Message string `json:"message"`
}

func AppiumReverseProxy(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				fmt.Println("Appium Reverse Proxy panic:", err)
			} else {
				fmt.Println("Appium Reverse Proxy panic:", r)
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal Server Error",
			})
		}
	}()

	udid := c.Param("udid")
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
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("No file provided in form data - %s", err)})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Files with extension `%s` are not allowed", ext)})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save file to `%s` - %s", dst, err)})
				return
			}

			// Add a remove for the file in a defer func just in case
			defer func() {
				os.Remove(dst)
			}()

			// Try to install the app after saving the file
			err = devices.InstallApp(dev, file.Filename)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed installing app - %s", err)})
				return
			}

			// Try to remove the file after installing it
			err = os.Remove(dst)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "App uploaded and installed successfully but failed to delete it"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "App uploaded and installed successfully", "status": "success"})
			return
		} else {
			// If the uploaded file is a zip archive
			// Open the zip to read it before extracting
			file, err := file.Open()
			defer file.Close()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf(fmt.Sprintf("Failed to open provided zip file - %s", err))})
				return
			}

			// Read the file content into a byte slice
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, file); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read provided zip file - %s", err)})
				return
			}

			// Get a list of the files in the zip
			fileNames, err := util.ListFilesInZip(buf.Bytes())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get file list from provided zip file - %s", err)})
				return
			}

			// Validate there are files inside the zip
			if len(fileNames) < 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Provided zip file is empty")})
				return
			}

			// If we got an apk or ipa file - directly extract it
			if strings.HasSuffix(fileNames[0], ".apk") || strings.HasSuffix(fileNames[0], ".ipa") {
				// We use the file content we read above to unzip from memory without storing the zip file at all
				err = util.UnzipInMemory(buf.Bytes(), uploadDir)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to unzip the file - %s", err)})
					return
				}

				// Attempt to install the unzipped app file
				err = devices.InstallApp(dev, fileNames[0])
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to install app - %s", err)})
					return
				}

				// Delete the unzipped file when the function ends
				defer func() {
					err := util.DeleteFile(uploadDir + "/" + fileNames[0])
					if err != nil {
						logger.ProviderLogger.LogError("upload_and_install_app", fmt.Sprintf("Failed to delete app file - %s", err))
					}
				}()
			} else if strings.Contains(fileNames[0], ".app") {
				// If the file name ends with .app, then its an iOS .app directory
				// We use the file content we read above to unzip from memory without storing the zip file at all
				err = util.UnzipInMemory(buf.Bytes(), uploadDir)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to unzip .app directory - %s", err)})
					return
				}

				// Attempt to install the unzipped .app directory
				err = devices.InstallApp(dev, fileNames[0])
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to install unzipped .app directory - %s", err)})
					return
				}

				// Delete the unzipped .app directory when the function ends
				defer func() {
					err := util.DeleteFolder(uploadDir + "/" + fileNames[0])
					if err != nil {
						logger.ProviderLogger.LogError("upload_and_install_app", "Failed to delete unzipped .app directory")
					}
				}()
			}
			c.JSON(http.StatusOK, gin.H{"message": "App uploaded and installed successfully", "status": "success"})
			return
		}
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Device currently not available"})
	}
}

func GetProviderData(c *gin.Context) {
	var providerData models.ProviderData

	deviceData := []models.Device{}
	for _, device := range devices.DBDeviceMap {
		deviceData = append(deviceData, *device)
	}

	providerData.ProviderData = *config.ProviderConfig
	providerData.DeviceData = deviceData

	c.JSON(http.StatusOK, providerData)
}

func DeviceInfo(c *gin.Context) {
	udid := c.Param("udid")

	if dev, ok := devices.DBDeviceMap[udid]; ok {
		devices.UpdateInstalledApps(dev)
		dev.UsesCustomWDA = config.ProviderConfig.UseCustomWDA
		c.JSON(http.StatusOK, dev)
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Did not find device with udid `%s`", udid)})
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
		c.JSON(http.StatusOK, installedApps)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Did not find device with udid `%s`", udid)})
}

func DevicesInfo(c *gin.Context) {
	deviceList := []*models.Device{}

	for _, device := range devices.DBDeviceMap {
		deviceList = append(deviceList, device)
	}

	c.JSON(http.StatusOK, deviceList)
}

type ProcessApp struct {
	App string `json:"app"`
}

func UninstallApp(c *gin.Context) {
	udid := c.Param("udid")

	if dev, ok := devices.DBDeviceMap[udid]; ok {
		payload, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		var payloadJson ProcessApp
		err = json.Unmarshal(payload, &payloadJson)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		var installedApps []string
		if dev.OS == "ios" {
			installedApps = devices.GetInstalledAppsIOS(dev)
		} else {
			installedApps = devices.GetInstalledAppsAndroid(dev)
		}

		if slices.Contains(installedApps, payloadJson.App) {
			err = devices.UninstallApp(dev, payloadJson.App)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to uninstall app `%s`", payloadJson.App)})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Successfully uninstalled app `%s`", payloadJson.App)})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("App `%s` is not installed on device", payloadJson.App)})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with udid `%s` does not exist", udid)})
}

func ResetDevice(c *gin.Context) {
	udid := c.Param("udid")

	if device, ok := devices.DBDeviceMap[udid]; ok {
		if device.IsResetting {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Device setup is already being reset"})
			return
		}
		if device.ProviderState != "live" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Only devices in `live` state can be reset, current state is `" + device.ProviderState + "`"})
			return
		}
		device.IsResetting = true
		device.CtxCancel()
		device.ProviderState = "init"
		device.IsResetting = false

		c.JSON(http.StatusOK, gin.H{"message": "Initiate setup reset on device"})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with udid `%s` does not exist", udid)})
}

func UpdateDeviceStreamSettings(c *gin.Context) {
	udid := c.Param("udid")

	if device, ok := devices.DBDeviceMap[udid]; ok {
		payload, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		var streamSettings models.UpdateStreamSettings
		err = json.Unmarshal(payload, &streamSettings)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
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

			err = devices.UpdateWebDriverAgentStreamSettings(device, false)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update stream settings on iOS device " + err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update stream settings on Android device " + err.Error()})
				return
			}
		}

		deviceStreamSettings := models.DeviceStreamSettings{
			UDID:                udid,
			StreamTargetFPS:     device.StreamTargetFPS,
			StreamJpegQuality:   device.StreamJpegQuality,
			StreamScalingFactor: device.StreamScalingFactor,
		}

		err = db.UpdateDeviceStreamSettings(udid, deviceStreamSettings)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update device stream settings in the database"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Stream settings updated"})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with udid `%s` does not exist", udid)})
}
