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
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/devices"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func appiumLockUnlock(device devices.PlatformDevice, lock string) (*http.Response, error) {
	endpoint := fmt.Sprintf("appium/device/%s", lock)
	return appiumRequest(device, http.MethodPost, endpoint, nil)
}

func appiumTap(device devices.PlatformDevice, x float64, y float64) (*http.Response, error) {
	// Generate the struct object for the Appium actions JSON request
	action := models.DevicePointerActions{
		Actions: []models.DevicePointerAction{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: models.DeviceActionParameters{
					PointerType: "touch",
				},
				Actions: []models.DeviceAction{
					{
						Type:     "pointerMove",
						Duration: 0,
						X:        x,
						Y:        y,
					},
					{
						Type:   "pointerDown",
						Button: 0,
					},
					{
						Type:     "pause",
						Duration: 10,
					},
					{
						Type:     "pointerUp",
						Duration: 0,
					},
				},
			},
		},
	}
	actionJSON, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return nil, err
	}

	return appiumRequest(device, http.MethodPost, "actions", bytes.NewReader(actionJSON))
}

func appiumTouchAndHold(device devices.PlatformDevice, x float64, y float64) (*http.Response, error) {
	action := models.DevicePointerActions{
		Actions: []models.DevicePointerAction{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: models.DeviceActionParameters{
					PointerType: "touch",
				},
				Actions: []models.DeviceAction{
					{
						Type:     "pointerMove",
						Duration: 0,
						X:        x,
						Y:        y,
					},
					{
						Type:   "pointerDown",
						Button: 0,
					},
					{
						Type:     "pause",
						Duration: 2000,
					},
					{
						Type:     "pointerUp",
						Duration: 0,
					},
				},
			},
		},
	}

	actionJSON, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return nil, err
	}

	return appiumRequest(device, http.MethodPost, "actions", bytes.NewReader(actionJSON))
}

func appiumSwipe(device devices.PlatformDevice, x, y, endX, endY float64) (*http.Response, error) {
	// Generate the struct object for the Appium actions JSON request
	action := models.DevicePointerActions{
		Actions: []models.DevicePointerAction{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: models.DeviceActionParameters{
					PointerType: "touch",
				},
				Actions: []models.DeviceAction{
					{
						Type:     "pointerMove",
						Duration: 0,
						X:        x,
						Y:        y,
					},
					{
						Type:     "pointerDown",
						Button:   0,
						Duration: 10,
					},
					{
						Type:     "pointerMove",
						Duration: 500,
						Origin:   "viewport",
						X:        endX,
						Y:        endY,
					},
					{
						Type:     "pointerUp",
						Duration: 0,
					},
				},
			},
		},
	}

	actionJSON, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return nil, err
	}

	return appiumRequest(device, http.MethodPost, "actions", bytes.NewReader(actionJSON))
}

func appiumSource(device devices.PlatformDevice) (*http.Response, error) {
	return appiumRequest(device, http.MethodGet, "source", nil)
}

func appiumScreenshot(device devices.PlatformDevice) (*http.Response, error) {
	return appiumRequest(device, http.MethodGet, "screenshot", nil)
}

func appiumHome(device devices.PlatformDevice) (*http.Response, error) {
	switch device.GetOS() {
	case "android":
		requestBody := models.AndroidKeycodePayload{
			Keycode: 3,
		}

		typeJSON, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, err
		}

		return appiumRequest(device, http.MethodPost, "appium/device/press_keycode", bytes.NewReader(typeJSON))
	case "ios":
		return wdaRequest(device, http.MethodPost, "wda/homescreen", nil)
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.GetOS())
	}
}

func appiumActivateApp(device devices.PlatformDevice, appIdentifier string) (*http.Response, error) {
	switch device.GetOS() {
	case "ios":
		requestBody := struct {
			BundleId string `json:"bundleId"`
		}{
			BundleId: appIdentifier,
		}

		reqJson, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("appiumActivateApp: Failed to marshal request body json when activating app for device `%s` - %s", device.GetUDID(), err)
		}

		return wdaRequest(device, http.MethodPost, "wda/apps/activate", bytes.NewReader(reqJson))
	case "android":
		requestBody := struct {
			AppId string `json:"appId"`
		}{
			AppId: appIdentifier,
		}

		reqJson, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("appiumActivateApp: Failed to marshal request body json when activating app for device `%s` - %s", device.GetUDID(), err)
		}

		return appiumRequest(device, http.MethodPost, "appium/device/activate_app", bytes.NewReader(reqJson))
	default:
		return nil, fmt.Errorf("appiumActivateApp: Bad device OS for device `%s` - %s", device.GetUDID(), device.GetOS())
	}
}

func appiumGetClipboard(device devices.PlatformDevice) (*http.Response, error) {
	requestBody := struct {
		ContentType string `json:"contentType"`
	}{
		ContentType: "plaintext",
	}
	reqJson, err := json.MarshalIndent(requestBody, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("appiumGetClipboard: Failed to marshal request body json when getting clipboard for device `%s` - %s", device.GetUDID(), err)
	}

	switch device.GetOS() {
	case "ios":
		activateAppResp, err := appiumActivateApp(device, config.ProviderConfig.WdaBundleID)
		if err != nil {
			return activateAppResp, fmt.Errorf("appiumGetClipboard: Failed to activate app - %s", err)
		}
		defer activateAppResp.Body.Close()

		clipboardResp, err := wdaRequest(device, http.MethodPost, "wda/getPasteboard", bytes.NewReader(reqJson))
		if err != nil {
			return clipboardResp, fmt.Errorf("appiumGetClipboard: Failed to execute Appium request for device `%s` - %s", device.GetUDID(), err)
		}

		_, err = appiumHome(device)
		if err != nil {
			device.GetLogger().LogWarn("appium_interact", "appiumGetClipboard: Failed to navigate to Home/Springboard using Appium")
		}

		return clipboardResp, nil
	case "android":
		return appiumRequest(device, http.MethodPost, "appium/device/get_clipboard", bytes.NewReader(reqJson))
	default:
		return nil, fmt.Errorf("appiumGetClipboard: Bad device OS for device `%s` - %s", device.GetUDID(), device.GetOS())
	}
}
