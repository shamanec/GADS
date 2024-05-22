package router

import (
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

	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/devices"
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
	device := devices.DeviceMap[udid]

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
	uploadDir := fmt.Sprintf("%s/apps/", config.Config.EnvConfig.ProviderFolder)

	// Read the file from the form data
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "File extension `" + ext + "` not allowed"})
		return
	}

	udid := c.Param("udid")
	if dev, ok := devices.DeviceMap[udid]; ok {
		// Save the uploaded file to the specified directory
		dst := uploadDir + file.Filename
		// First try to remove file if it already exists
		err = os.Remove(dst)
		if err != nil {
			// TODO handle error properly
		}

		// Save the file
		if err := c.SaveUploadedFile(file, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		defer func() {
			os.Remove(dst)
		}()

		// Try to install the app after saving the file
		err = devices.InstallApp(dev, file.Filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed installing app"})
			return
		}

		err = os.Remove(dst)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "App installed but failed to delete it"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "App uploaded and installed successfully", "status": "success"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "Device currently not available"})
}

func GetProviderData(c *gin.Context) {
	var providerData models.ProviderData

	deviceData := []*models.Device{}
	for _, device := range devices.DeviceMap {
		deviceData = append(deviceData, device)
	}

	providerData.ProviderData = config.Config.EnvConfig
	providerData.DeviceData = deviceData

	c.JSON(http.StatusOK, providerData)
}

func DeviceInfo(c *gin.Context) {
	udid := c.Param("udid")

	if dev, ok := devices.DeviceMap[udid]; ok {
		devices.UpdateInstalledApps(dev)
		c.JSON(http.StatusOK, dev)
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Did not find device with udid `%s`", udid)})
}

func DevicesInfo(c *gin.Context) {
	deviceList := []*models.Device{}

	for _, device := range devices.DeviceMap {
		deviceList = append(deviceList, device)
	}

	c.JSON(http.StatusOK, deviceList)
}

type ProcessApp struct {
	App string `json:"app"`
}

func UninstallApp(c *gin.Context) {
	udid := c.Param("udid")

	if dev, ok := devices.DeviceMap[udid]; ok {
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

		if slices.Contains(dev.InstalledApps, payloadJson.App) {
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

func InstallApp(c *gin.Context) {
	udid := c.Param("udid")

	if dev, ok := devices.DeviceMap[udid]; ok {
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
		err = devices.InstallApp(dev, payloadJson.App)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to install app `%s`", payloadJson.App)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Successfully installed app `%s`", payloadJson.App)})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with udid `%s` does not exist", udid)})
}

func ResetDevice(c *gin.Context) {
	udid := c.Param("udid")

	if device, ok := devices.DeviceMap[udid]; ok {
		device.IsResetting = true
		device.CtxCancel()
		device.ProviderState = "init"
		device.IsResetting = false

		c.JSON(http.StatusOK, gin.H{"message": "Initiate setup reset on device"})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with udid `%s` does not exist", udid)})
}
