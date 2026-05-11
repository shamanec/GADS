package router

import (
	"GADS/common/models"
	"GADS/common/utils"
	"GADS/provider/config"
	"GADS/provider/devices"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/danielpaulus/go-ios/ios/instruments"
)

var controlNetClient = &http.Client{
	Timeout: time.Second * 120,
}

func androidRemoteServerRequest(dev devices.PlatformDevice, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	andDev, ok := dev.(*devices.AndroidDevice)
	if !ok {
		return nil, fmt.Errorf("device %s is not an Android device", dev.GetUDID())
	}
	url := fmt.Sprintf("http://localhost:%s/%s", andDev.GetAndroidRemoteServerPort(), endpoint)
	dev.GetLogger().LogDebug("androidRemoteServerRequest", fmt.Sprintf("Calling `%s` for device `%s`", url, dev.GetUDID()))
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return controlNetClient.Do(req)
}

func androidRemoteServerRequestJson(dev devices.PlatformDevice, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	andDev, ok := dev.(*devices.AndroidDevice)
	if !ok {
		return nil, fmt.Errorf("device %s is not an Android device", dev.GetUDID())
	}
	url := fmt.Sprintf("http://localhost:%s/%s", andDev.GetAndroidRemoteServerPort(), endpoint)
	dev.GetLogger().LogDebug("androidRemoteServerRequest", fmt.Sprintf("Calling `%s` for device `%s`", url, dev.GetUDID()))
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return controlNetClient.Do(req)
}

func appiumRequest(dev devices.PlatformDevice, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/session/%s/%s", dev.GetAppiumPort(), dev.GetAppiumSessionID(), endpoint)
	dev.GetLogger().LogDebug("appium_interact", fmt.Sprintf("Calling `%s` for device `%s`", url, dev.GetUDID()))
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return controlNetClient.Do(req)
}

func appiumRequestNoSession(dev devices.PlatformDevice, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/%s", dev.GetAppiumPort(), endpoint)
	dev.GetLogger().LogDebug("appium_interact", fmt.Sprintf("Calling `%s` for device `%s`", url, dev.GetUDID()))
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return controlNetClient.Do(req)
}

func wdaRequest(dev devices.PlatformDevice, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	iosDev, ok := dev.(*devices.IOSDevice)
	if !ok {
		return nil, fmt.Errorf("device %s is not an iOS device", dev.GetUDID())
	}
	url := fmt.Sprintf("http://localhost:%v/%s", iosDev.GetWDAPort(), endpoint)
	dev.GetLogger().LogDebug("wda_interact", fmt.Sprintf("Calling `%s` for device `%s`", url, dev.GetUDID()))
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return controlNetClient.Do(req)
}

func deviceLock(dev devices.PlatformDevice, lock string) (*http.Response, error) {
	if dev.GetOS() == "ios" {
		return wdaRequest(dev, http.MethodPost, "wda/"+lock, nil)
	} else {
		return androidRemoteServerRequest(dev, http.MethodPost, lock, nil)
	}
}

func deviceTap(dev devices.PlatformDevice, x float64, y float64) (*http.Response, error) {
	requestBody := struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}{
		X: x,
		Y: y,
	}
	actionJSON, err := json.MarshalIndent(requestBody, "", "  ")
	if err != nil {
		return nil, err
	}

	if dev.GetOS() == "ios" {
		return wdaRequest(dev, http.MethodPost, "wda/tap", bytes.NewReader(actionJSON))
	} else {
		return androidRemoteServerRequestJson(dev, http.MethodPost, "tap", bytes.NewReader([]byte(actionJSON)))
	}
}

func deviceTouchAndHold(dev devices.PlatformDevice, x float64, y float64, duration float64) (*http.Response, error) {
	if dev.GetOS() == "ios" {
		duration = float64(duration) / 1000
	}
	requestBody := struct {
		X        float64 `json:"x"`
		Y        float64 `json:"y"`
		Duration float64 `json:"duration"`
	}{
		X:        x,
		Y:        y,
		Duration: duration,
	}
	actionJSON, err := json.MarshalIndent(requestBody, "", "  ")
	if err != nil {
		return nil, err
	}

	if dev.GetOS() == "ios" {
		return wdaRequest(dev, http.MethodPost, "wda/touchAndHold", bytes.NewReader(actionJSON))
	} else {
		return androidRemoteServerRequestJson(dev, http.MethodPost, "touchAndHold", bytes.NewReader([]byte(actionJSON)))
	}
}

func deviceScreenshot(dev devices.PlatformDevice) (string, error) {
	if dev.GetOS() == "android" {
		cmd := exec.Command("adb", "-s", dev.GetUDID(), "exec-out", "screencap", "-p")
		var out bytes.Buffer
		cmd.Stdout = &out

		err := cmd.Run()
		if err != nil {
			return "", err
		}

		// Encode PNG bytes to Base64
		base64Screenshot := base64.StdEncoding.EncodeToString(out.Bytes())

		return base64Screenshot, nil
	} else {
		iosDev, ok := dev.(*devices.IOSDevice)
		if !ok {
			return "", fmt.Errorf("device %s is not an iOS device", dev.GetUDID())
		}
		screenshotService, err := instruments.NewScreenshotService(iosDev.GoIOSDeviceEntry)
		if err != nil {
			return "", err
		}
		imageBytes, err := screenshotService.TakeScreenshot()
		if err != nil {
			return "", err
		}

		base64Screenshot := base64.StdEncoding.EncodeToString(imageBytes)

		return base64Screenshot, nil
	}
}

func deviceSwipe(dev devices.PlatformDevice, x, y, endX, endY float64) (*http.Response, error) {
	if dev.GetOS() == "ios" {
		requestBody := struct {
			X     float64 `json:"startX"`
			Y     float64 `json:"startY"`
			EndX  float64 `json:"endX"`
			EndY  float64 `json:"endY"`
			Delay float64 `json:"delay"`
		}{
			X:     x,
			Y:     y,
			EndX:  endX,
			EndY:  endY,
			Delay: 1,
		}
		actionJSON, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, err
		}
		return wdaRequest(dev, http.MethodPost, "wda/swipe", bytes.NewReader(actionJSON))
	} else {
		requestBody := struct {
			X     float64 `json:"x1"`
			Y     float64 `json:"y1"`
			EndX  float64 `json:"x2"`
			EndY  float64 `json:"y2"`
			Delay float64 `json:"duration"`
		}{
			X:     x,
			Y:     y,
			EndX:  endX,
			EndY:  endY,
			Delay: 500,
		}
		actionJSON, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, err
		}
		return androidRemoteServerRequestJson(dev, http.MethodPost, "swipe", bytes.NewReader([]byte(actionJSON)))
	}
}

func devicePinch(dev devices.PlatformDevice, x, y, scale float64) (*http.Response, error) {
	if dev.GetOS() == "ios" {
		requestBody := struct {
			CenterX    float64 `json:"centerX"`
			CenterY    float64 `json:"centerY"`
			StartScale float64 `json:"startScale"`
			EndScale   float64 `json:"endScale"`
			Duration   float64 `json:"duration"`
		}{
			CenterX:    x,
			CenterY:    y,
			StartScale: 1.0,
			EndScale:   scale,
			Duration:   0.5,
		}

		actionJSON, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal iOS pinch payload: %w", err)
		}

		return wdaRequest(dev, http.MethodPost, "wda/pinch", bytes.NewReader(actionJSON))
	} else {
		requestBody := struct {
			CenterX   float64 `json:"centerX"`
			CenterY   float64 `json:"centerY"`
			Scale     float64 `json:"scale"`
			Duration  int     `json:"duration"`
			Direction string  `json:"direction"`
		}{
			CenterX:   x,
			CenterY:   y,
			Scale:     scale,
			Duration:  300,
			Direction: "diagonal",
		}

		actionJSON, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Android pinch payload: %w", err)
		}

		return androidRemoteServerRequestJson(dev, http.MethodPost, "pinch", bytes.NewReader(actionJSON))
	}
}

func deviceDoubleTap(dev devices.PlatformDevice, x, y float64) (*http.Response, error) {
	requestBody := struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}{
		X: x,
		Y: y,
	}
	actionJSON, err := json.MarshalIndent(requestBody, "", "  ")
	if err != nil {
		return nil, err
	}

	if dev.GetOS() == "ios" {
		return wdaRequest(dev, http.MethodPost, "wda/doubleTap", bytes.NewReader(actionJSON))
	}

	return androidRemoteServerRequestJson(dev, http.MethodPost, "doubleTap", bytes.NewReader(actionJSON))
}

func deviceHome(dev devices.PlatformDevice) (*http.Response, error) {
	if dev.GetOS() == "ios" {
		return wdaRequest(dev, http.MethodPost, "wda/homescreen", nil)
	} else {
		return androidRemoteServerRequest(dev, http.MethodPost, "home", nil)
	}
}

func deviceRecents(dev devices.PlatformDevice) error {
	if dev.GetOS() == "ios" {
		return fmt.Errorf("App switcher not supported on iOS via WDA")
	}
	cmd := exec.CommandContext(dev.GetContext(), "adb", "-s", dev.GetUDID(), "shell", "input", "keyevent", "KEYCODE_APP_SWITCH")
	return cmd.Run()
}

func activateApp(dev devices.PlatformDevice, appIdentifier string) (*http.Response, error) {
	if dev.GetOS() == "ios" {
		requestBody := struct {
			BundleId string `json:"bundleId"`
		}{
			BundleId: appIdentifier,
		}

		reqJson, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("appiumActivateApp: Failed to marshal request body json when activating app for device `%s` - %s", dev.GetUDID(), err)
		}

		return wdaRequest(dev, http.MethodPost, "wda/apps/activate", bytes.NewReader(reqJson))
	}

	return nil, fmt.Errorf("App activation available only for iOS devices")
}

func deviceGetClipboard(dev devices.PlatformDevice) (*http.Response, error) {
	if dev.GetOS() == "ios" {
		requestBody := struct {
			ContentType string `json:"contentType"`
		}{
			ContentType: "plaintext",
		}
		reqJson, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("appiumGetClipboard: Failed to marshal request body json when getting clipboard for device `%s` - %s", dev.GetUDID(), err)
		}

		activateAppResp, err := activateApp(dev, config.ProviderConfig.WdaBundleID)
		if err != nil {
			return activateAppResp, fmt.Errorf("appiumGetClipboard: Failed to activate app - %s", err)
		}
		defer activateAppResp.Body.Close()

		clipboardResp, err := wdaRequest(dev, http.MethodPost, "wda/getPasteboard", bytes.NewReader(reqJson))
		if err != nil {
			return clipboardResp, fmt.Errorf("appiumGetClipboard: Failed to execute Appium request for device `%s` - %s", dev.GetUDID(), err)
		}

		_, err = deviceHome(dev)
		if err != nil {
			dev.GetLogger().LogWarn("appium_interact", "appiumGetClipboard: Failed to navigate to Home/Springboard using Appium")
		}

		return clipboardResp, nil
	} else {
		return androidRemoteServerRequest(dev, http.MethodPost, "clipboard", nil)
	}
}

func executeTypeText(dev devices.PlatformDevice, text string) (*http.Response, error) {
	typeTextPayload := models.AppiumTypeText{
		Text: text,
	}
	typeJSON, err := json.MarshalIndent(typeTextPayload, "", "  ")
	if err != nil {
		return nil, err
	}

	if dev.GetOS() == "ios" {
		return wdaRequest(dev, http.MethodPost, "wda/type", bytes.NewBuffer(typeJSON))
	} else {
		andDev, ok := dev.(*devices.AndroidDevice)
		if !ok {
			return nil, fmt.Errorf("device %s is not an Android device", dev.GetUDID())
		}
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%v/type", andDev.GetAndroidIMEPort()), bytes.NewBuffer(typeJSON))
		if err != nil {
			return nil, err
		}
		return netClient.Do(req)
	}
}

func getCenterCoordinates(dev devices.PlatformDevice) (float64, float64, error) {
	device := dev.GetDBDevice()
	width, err := strconv.ParseFloat(device.ScreenWidth, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid screen width %q: %w", device.ScreenWidth, err)
	}

	height, err := strconv.ParseFloat(device.ScreenHeight, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid screen height %q: %w", device.ScreenHeight, err)
	}

	return width / 2, height / 2, nil
}

func normalizeCoordinates(dev devices.PlatformDevice, x, y float64) (float64, float64, error) {
	if x == 0 && y == 0 {
		return getCenterCoordinates(dev)
	}
	return x, y, nil
}

func executeCustomAction(dev devices.PlatformDevice, actionType string, params map[string]any) (*http.Response, error) {
	if params == nil {
		params = make(map[string]any)
	}

	switch actionType {
	case "tap":
		x := utils.GetFloat(params, "x", 0)
		y := utils.GetFloat(params, "y", 0)
		x, y, err := normalizeCoordinates(dev, x, y)
		if err != nil {
			return nil, fmt.Errorf("normalizing coordinates: %w", err)
		}
		return deviceTap(dev, x, y)

	case "double_tap":
		x := utils.GetFloat(params, "x", 0)
		y := utils.GetFloat(params, "y", 0)
		x, y, err := normalizeCoordinates(dev, x, y)
		if err != nil {
			return nil, fmt.Errorf("normalizing coordinates: %w", err)
		}
		return deviceDoubleTap(dev, x, y)

	case "swipe":
		x := utils.GetFloat(params, "x", 0)
		y := utils.GetFloat(params, "y", 0)
		endX := utils.GetFloat(params, "endX", 0)
		endY := utils.GetFloat(params, "endY", 0)
		return deviceSwipe(dev, x, y, endX, endY)

	case "touch_and_hold":
		x := utils.GetFloat(params, "x", 0)
		y := utils.GetFloat(params, "y", 0)
		x, y, err := normalizeCoordinates(dev, x, y)
		if err != nil {
			return nil, fmt.Errorf("normalizing coordinates: %w", err)
		}
		duration := utils.GetFloat(params, "duration", 1000)
		return deviceTouchAndHold(dev, x, y, duration)

	case "pinch":
		x := utils.GetFloat(params, "x", 0)
		y := utils.GetFloat(params, "y", 0)
		x, y, err := normalizeCoordinates(dev, x, y)
		if err != nil {
			return nil, fmt.Errorf("normalizing coordinates: %w", err)
		}
		scale := utils.GetFloat(params, "scale", 1.0)
		return devicePinch(dev, x, y, scale)

	case "type_text":
		text := utils.GetString(params, "text", "")
		return executeTypeText(dev, text)

	case "home":
		return deviceHome(dev)

	case "lock":
		return deviceLock(dev, "lock")

	case "unlock":
		return deviceLock(dev, "unlock")

	case "pinch_in":
		x := utils.GetFloat(params, "x", 250)
		y := utils.GetFloat(params, "y", 500)
		return devicePinch(dev, x, y, 0.5)

	case "pinch_out":
		x := utils.GetFloat(params, "x", 250)
		y := utils.GetFloat(params, "y", 500)
		return devicePinch(dev, x, y, 2.0)

	default:
		return nil, fmt.Errorf("unsupported action type: %s", actionType)
	}
}
