package router

import (
	"GADS/common/models"
	"GADS/common/utils"
	"GADS/provider/config"
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

func androidRemoteServerRequest(device *models.Device, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/%s", device.AndroidRemoteServerPort, endpoint)
	device.Logger.LogDebug("androidRemoteServerRequest", fmt.Sprintf("Calling `%s` for device `%s`", url, device.UDID))
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return controlNetClient.Do(req)
}

func androidRemoteServerRequestJson(device *models.Device, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/%s", device.AndroidRemoteServerPort, endpoint)
	device.Logger.LogDebug("androidRemoteServerRequest", fmt.Sprintf("Calling `%s` for device `%s`", url, device.UDID))
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return controlNetClient.Do(req)
}

func appiumRequest(device *models.Device, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/session/%s/%s", device.AppiumPort, device.AppiumSessionID, endpoint)
	device.Logger.LogDebug("appium_interact", fmt.Sprintf("Calling `%s` for device `%s`", url, device.UDID))
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return controlNetClient.Do(req)
}

func appiumRequestNoSession(device *models.Device, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/%s", device.AppiumPort, endpoint)
	device.Logger.LogDebug("appium_interact", fmt.Sprintf("Calling `%s` for device `%s`", url, device.UDID))
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return controlNetClient.Do(req)
}

func wdaRequest(device *models.Device, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%v/%s", device.WDAPort, endpoint)
	device.Logger.LogDebug("wda_interact", fmt.Sprintf("Calling `%s` for device `%s`", url, device.UDID))
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return controlNetClient.Do(req)
}

func deviceLock(device *models.Device, lock string) (*http.Response, error) {
	if device.OS == "ios" {
		return wdaRequest(device, http.MethodPost, "wda/"+lock, nil)
	} else {
		return androidRemoteServerRequest(device, http.MethodPost, lock, nil)
	}
}

func deviceTap(device *models.Device, x float64, y float64) (*http.Response, error) {
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

	if device.OS == "ios" {
		return wdaRequest(device, http.MethodPost, "wda/tap", bytes.NewReader(actionJSON))
	} else {
		return androidRemoteServerRequestJson(device, http.MethodPost, "tap", bytes.NewReader([]byte(actionJSON)))
	}
}

func deviceTouchAndHold(device *models.Device, x float64, y float64, duration float64) (*http.Response, error) {
	if device.OS == "ios" {
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

	if device.OS == "ios" {
		return wdaRequest(device, http.MethodPost, "wda/touchAndHold", bytes.NewReader(actionJSON))
	} else {
		return androidRemoteServerRequestJson(device, http.MethodPost, "touchAndHold", bytes.NewReader([]byte(actionJSON)))
	}
}

func deviceScreenshot(device *models.Device) (string, error) {
	if device.OS == "android" {
		cmd := exec.Command("adb", "-s", device.UDID, "exec-out", "screencap", "-p")
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
		screenshotService, err := instruments.NewScreenshotService(device.GoIOSDeviceEntry)
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

func deviceSwipe(device *models.Device, x, y, endX, endY float64) (*http.Response, error) {
	if device.OS == "ios" {
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
		return wdaRequest(device, http.MethodPost, "wda/swipe", bytes.NewReader(actionJSON))
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
		return androidRemoteServerRequestJson(device, http.MethodPost, "swipe", bytes.NewReader([]byte(actionJSON)))
	}
}

func devicePinch(device *models.Device, x, y, scale float64) (*http.Response, error) {
	if device.OS == "ios" {
		velocity := scale / 0.3

		requestBody := struct {
			Scale    float64 `json:"scale"`
			Velocity float64 `json:"velocity"`
		}{
			Scale:    scale,
			Velocity: velocity,
		}

		actionJSON, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal iOS pinch payload: %w", err)
		}

		return wdaRequest(device, http.MethodPost, "wda/pinch", bytes.NewReader(actionJSON))
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

		return androidRemoteServerRequestJson(device, http.MethodPost, "pinch", bytes.NewReader(actionJSON))
	}
}

func deviceDoubleTap(device *models.Device, x, y float64) (*http.Response, error) {
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

	if device.OS == "ios" {
		return wdaRequest(device, http.MethodPost, "wda/doubleTap", bytes.NewReader(actionJSON))
	}

	return androidRemoteServerRequestJson(device, http.MethodPost, "doubleTap", bytes.NewReader(actionJSON))
}

func deviceHome(device *models.Device) (*http.Response, error) {
	if device.OS == "ios" {
		return wdaRequest(device, http.MethodPost, "wda/homescreen", nil)
	} else {
		return androidRemoteServerRequest(device, http.MethodPost, "home", nil)
	}
}

func activateApp(device *models.Device, appIdentifier string) (*http.Response, error) {
	if device.OS == "ios" {
		requestBody := struct {
			BundleId string `json:"bundleId"`
		}{
			BundleId: appIdentifier,
		}

		reqJson, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("appiumActivateApp: Failed to marshal request body json when activating app for device `%s` - %s", device.UDID, err)
		}

		return wdaRequest(device, http.MethodPost, "wda/apps/activate", bytes.NewReader(reqJson))
	}

	return nil, fmt.Errorf("App activation available only for iOS devices")
}

func deviceGetClipboard(device *models.Device) (*http.Response, error) {
	if device.OS == "ios" {
		requestBody := struct {
			ContentType string `json:"contentType"`
		}{
			ContentType: "plaintext",
		}
		reqJson, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("appiumGetClipboard: Failed to marshal request body json when getting clipboard for device `%s` - %s", device.UDID, err)
		}

		activateAppResp, err := activateApp(device, config.ProviderConfig.WdaBundleID)
		if err != nil {
			return activateAppResp, fmt.Errorf("appiumGetClipboard: Failed to activate app - %s", err)
		}
		defer activateAppResp.Body.Close()

		clipboardResp, err := wdaRequest(device, http.MethodPost, "wda/getPasteboard", bytes.NewReader(reqJson))
		if err != nil {
			return clipboardResp, fmt.Errorf("appiumGetClipboard: Failed to execute Appium request for device `%s` - %s", device.UDID, err)
		}

		_, err = deviceHome(device)
		if err != nil {
			device.Logger.LogWarn("appium_interact", "appiumGetClipboard: Failed to navigate to Home/Springboard using Appium")
		}

		return clipboardResp, nil
	} else {
		return androidRemoteServerRequest(device, http.MethodPost, "clipboard", nil)
	}
}

func executeTypeText(device *models.Device, text string) (*http.Response, error) {
	typeTextPayload := models.AppiumTypeText{
		Text: text,
	}
	typeJSON, err := json.MarshalIndent(typeTextPayload, "", "  ")
	if err != nil {
		return nil, err
	}

	if device.OS == "ios" {
		return wdaRequest(device, http.MethodPost, "wda/type", bytes.NewBuffer(typeJSON))
	} else {
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%v/type", device.AndroidIMEPort), bytes.NewBuffer(typeJSON))
		if err != nil {
			return nil, err
		}
		return netClient.Do(req)
	}
}

func getCenterCoordinates(device *models.Device) (float64, float64, error) {
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

func normalizeCoordinates(device *models.Device, x, y float64) (float64, float64, error) {
	if x == 0 && y == 0 {
		return getCenterCoordinates(device)
	}
	return x, y, nil
}

func executeCustomAction(device *models.Device, actionType string, params map[string]any) (*http.Response, error) {
	if params == nil {
		params = make(map[string]any)
	}

	switch actionType {
	case "tap":
		x := utils.GetFloat(params, "x", 0)
		y := utils.GetFloat(params, "y", 0)
		x, y, err := normalizeCoordinates(device, x, y)
		if err != nil {
			return nil, fmt.Errorf("normalizing coordinates: %w", err)
		}
		return deviceTap(device, x, y)

	case "double_tap":
		x := utils.GetFloat(params, "x", 0)
		y := utils.GetFloat(params, "y", 0)
		x, y, err := normalizeCoordinates(device, x, y)
		if err != nil {
			return nil, fmt.Errorf("normalizing coordinates: %w", err)
		}
		return deviceDoubleTap(device, x, y)

	case "swipe":
		x := utils.GetFloat(params, "x", 0)
		y := utils.GetFloat(params, "y", 0)
		endX := utils.GetFloat(params, "endX", 0)
		endY := utils.GetFloat(params, "endY", 0)
		return deviceSwipe(device, x, y, endX, endY)

	case "touch_and_hold":
		x := utils.GetFloat(params, "x", 0)
		y := utils.GetFloat(params, "y", 0)
		x, y, err := normalizeCoordinates(device, x, y)
		if err != nil {
			return nil, fmt.Errorf("normalizing coordinates: %w", err)
		}
		duration := utils.GetFloat(params, "duration", 1000)
		return deviceTouchAndHold(device, x, y, duration)

	case "pinch":
		x := utils.GetFloat(params, "x", 0)
		y := utils.GetFloat(params, "y", 0)
		x, y, err := normalizeCoordinates(device, x, y)
		if err != nil {
			return nil, fmt.Errorf("normalizing coordinates: %w", err)
		}
		scale := utils.GetFloat(params, "scale", 1.0)
		return devicePinch(device, x, y, scale)

	case "type_text":
		text := utils.GetString(params, "text", "")
		return executeTypeText(device, text)

	case "home":
		return deviceHome(device)

	case "lock":
		return deviceLock(device, "lock")

	case "unlock":
		return deviceLock(device, "unlock")

	case "pinch_in":
		x := utils.GetFloat(params, "x", 250)
		y := utils.GetFloat(params, "y", 500)
		return devicePinch(device, x, y, 0.5)

	case "pinch_out":
		x := utils.GetFloat(params, "x", 250)
		y := utils.GetFloat(params, "y", 500)
		return devicePinch(device, x, y, 2.0)

	default:
		return nil, fmt.Errorf("unsupported action type: %s", actionType)
	}
}
