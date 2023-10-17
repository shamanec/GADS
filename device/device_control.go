package device

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Device struct {
	Connected            bool   `json:"connected,omitempty" bson:"connected,omitempty"`
	Healthy              bool   `json:"healthy,omitempty" bson:"healthy,omitempty"`
	LastHealthyTimestamp int64  `json:"last_healthy_timestamp,omitempty" bson:"last_healthy_timestamp,omitempty"`
	UDID                 string `json:"udid" bson:"_id"`
	OS                   string `json:"os" bson:"os"`
	Name                 string `json:"name" bson:"name"`
	OSVersion            string `json:"os_version" bson:"os_version"`
	ScreenSize           string `json:"screen_size" bson:"screen_size"`
	Model                string `json:"model" bson:"model"`
	Image                string `json:"image,omitempty" bson:"image,omitempty"`
	HostAddress          string `json:"host_address" bson:"host_address"`
}

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

// Get specific device info from DB
func getDBDevice(udid string) Device {
	for _, dbDevice := range latestDevices {
		if dbDevice.UDID == udid {
			return dbDevice
		}
	}
	return Device{}
}

// Load a specific device page
func GetDevicePage(c *gin.Context) {
	udid := c.Param("udid")

	device := getDBDevice(udid)
	// If the device does not exist in the cached devices
	if device == (Device{}) {
		c.String(http.StatusInternalServerError, "Device not found")
		return
	}

	// Create the device health URL
	url := fmt.Sprintf("http://%s:10001/device/%s/health", device.HostAddress, device.UDID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed creating http request to check device health from provider - %s", err.Error()))
		return
	}

	response, err := netClient.Do(req)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed performing http request to check device health from provider - %s", err.Error()))
		return
	}

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		c.String(http.StatusInternalServerError, fmt.Sprintf("Device not healthy, health check response: %s", string(body)))
		return
	}

	// Calculate the width and height for the canvas
	canvasWidth, canvasHeight := calculateCanvasDimensions(device.ScreenSize)

	pageData := struct {
		Device       Device
		CanvasWidth  string
		CanvasHeight string
	}{
		Device:       device,
		CanvasWidth:  canvasWidth,
		CanvasHeight: canvasHeight,
	}

	var tmpl = template.Must(template.ParseFiles("static/device_control_new.html"))
	err = tmpl.Execute(c.Writer, pageData)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}

// Calculate the device stream canvas dimensions
func calculateCanvasDimensions(size string) (canvasWidth string, canvasHeight string) {
	// Get the width and height provided
	dimensions := strings.Split(size, "x")
	widthString := dimensions[0]
	heightString := dimensions[1]

	// Convert them to ints
	width, _ := strconv.Atoi(widthString)
	height, _ := strconv.Atoi(heightString)

	screen_ratio := float64(width) / float64(height)

	canvasHeight = "850"
	canvasWidth = fmt.Sprintf("%f", 850*screen_ratio)

	return
}
