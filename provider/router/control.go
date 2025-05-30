package router

import (
	"GADS/common/models"
	"GADS/provider/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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
		return androidRemoteServerRequest(device, http.MethodPost, "tap", bytes.NewReader([]byte(actionJSON)))
	}
}

func deviceTouchAndHold(device *models.Device, x float64, y float64, delay float64) (*http.Response, error) {
	requestBody := struct {
		X        float64 `json:"x"`
		Y        float64 `json:"y"`
		Delay    float64 `json:"delay"`
		Duration float64 `json:"duration"`
	}{
		X:        x,
		Y:        y,
		Delay:    delay,
		Duration: 1000,
	}
	actionJSON, err := json.MarshalIndent(requestBody, "", "  ")
	if err != nil {
		return nil, err
	}

	if device.OS == "ios" {
		return wdaRequest(device, http.MethodPost, "wda/tap", bytes.NewReader(actionJSON))
	} else {
		return androidRemoteServerRequest(device, http.MethodPost, "touchAndHold", bytes.NewReader([]byte(actionJSON)))
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
		return androidRemoteServerRequest(device, http.MethodPost, "swipe", bytes.NewReader([]byte(actionJSON)))
	}
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
