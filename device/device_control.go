package device

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Device struct {
	Container             *DeviceContainer `json:"container,omitempty"`
	Connected             bool             `json:"connected,omitempty"`
	Healthy               bool             `json:"healthy,omitempty"`
	LastHealthyTimestamp  int64            `json:"last_healthy_timestamp,omitempty"`
	UDID                  string           `json:"udid"`
	OS                    string           `json:"os"`
	AppiumPort            string           `json:"appium_port"`
	StreamPort            string           `json:"stream_port"`
	ContainerServerPort   string           `json:"container_server_port"`
	WDAPort               string           `json:"wda_port,omitempty"`
	Name                  string           `json:"name"`
	OSVersion             string           `json:"os_version"`
	ScreenSize            string           `json:"screen_size"`
	Model                 string           `json:"model"`
	Image                 string           `json:"image,omitempty"`
	Host                  string           `json:"host"`
	MinicapFPS            string           `json:"minicap_fps,omitempty"`
	MinicapHalfResolution string           `json:"minicap_half_resolution,omitempty"`
	UseMinicap            string           `json:"use_minicap,omitempty"`
}

type DeviceContainer struct {
	ContainerID     string `json:"id"`
	ContainerStatus string `json:"status"`
	ImageName       string `json:"image_name"`
	ContainerName   string `json:"container_name"`
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
	var err error
	udid := c.Param("udid")

	device := getDBDevice(udid)

	// If the device does not exist in the cached devices
	if device == (Device{}) {
		fmt.Println("error")
		return
	}

	res, err := http.Get("http://" + device.Host + ":10001/device/" + device.UDID + "/health")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if res.StatusCode != 200 {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			// handle error
		}

		fmt.Println(string(body))
		c.String(http.StatusInternalServerError, "Device not available")
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

	// This will generate only the device table, not the whole page
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
