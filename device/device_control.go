package device

import (
	"GADS/models"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

// Get specific device info from DB
func getDBDevice(udid string) *models.Device {
	for _, dbDevice := range latestDevices {
		if dbDevice.UDID == udid {
			return dbDevice
		}
	}
	return nil
}

// Load a specific device page
func GetDevicePage(c *gin.Context) {
	udid := c.Param("udid")

	device := getDBDevice(udid)
	if device.InUse {
		c.String(http.StatusInternalServerError, "Device is in use")
		return
	}
	// If the device does not exist in the cached devices
	if device == nil {
		c.String(http.StatusInternalServerError, "Device not found")
		return
	}

	// Create the device health URL
	url := fmt.Sprintf("http://%s/device/%s/health", device.Host, device.UDID)

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
	canvasWidth, canvasHeight := calculateCanvasDimensions(device)

	pageData := struct {
		Device       models.Device
		CanvasWidth  string
		CanvasHeight string
		ScreenHeight string
		ScreenWidth  string
	}{
		Device:       *device,
		CanvasWidth:  canvasWidth,
		CanvasHeight: canvasHeight,
		ScreenHeight: device.ScreenHeight,
		ScreenWidth:  device.ScreenWidth,
	}

	var tmpl = template.Must(template.ParseFiles("static/device_control_new.html"))
	err = tmpl.Execute(c.Writer, pageData)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}

// Calculate the device stream canvas dimensions
func calculateCanvasDimensions(device *models.Device) (canvasWidth string, canvasHeight string) {
	// Get the width and height provided
	widthString := device.ScreenWidth
	heightString := device.ScreenHeight

	// Convert them to ints
	width, _ := strconv.Atoi(widthString)
	height, _ := strconv.Atoi(heightString)

	screen_ratio := float64(width) / float64(height)

	canvasHeight = "850"
	canvasWidth = fmt.Sprintf("%f", 850*screen_ratio)

	return
}

func DeviceInUseWS(c *gin.Context) {
	udid := c.Param("udid")
	device := getDBDevice(udid)
	var mu sync.Mutex

	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	messageReceived := make(chan struct{})
	defer close(messageReceived)

	go func() {
		for {
			data, code, err := wsutil.ReadClientData(conn)
			if err != nil {
				fmt.Println(err)
				return
			}

			if code == 8 {
				close(messageReceived)
				return
			}

			if string(data) == "ping" {
				messageReceived <- struct{}{}
			}
		}
	}()

	for {
		select {
		case <-messageReceived:
			mu.Lock()
			device.InUse = true
			mu.Unlock()
		case <-time.After(2 * time.Second):
			mu.Lock()
			device.InUse = false
			mu.Unlock()
			return
		}
	}
}
