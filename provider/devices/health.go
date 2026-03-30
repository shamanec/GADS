/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package devices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

)

// Check if a device is healthy by checking Appium and WebDriverAgent(for iOS) services
func GetDeviceHealth(dev PlatformDevice) (bool, error) {
	return dev.IsConnected(), nil
}

func checkAppiumSession(dev PlatformDevice) error {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%s/sessions", dev.GetAppiumPort()), nil)
	if err != nil {
		dev.SetAppiumSessionID("")
		return fmt.Errorf("checkAppiumSession: Failed creating request - %s", err)
	}

	response, err := netClient.Do(req)
	if err != nil {
		dev.SetAppiumSessionID("")
		return fmt.Errorf("checkAppiumSession: Failed executing request `%s` - %s", req.URL, err)
	}
	responseBody, _ := io.ReadAll(response.Body)

	var responseJson AppiumGetSessionsResponse
	err = json.Unmarshal(responseBody, &responseJson)
	if err != nil {
		dev.SetAppiumSessionID("")
		return fmt.Errorf("checkAppiumSession: Failed unmarshaling response json - %s", err)
	}

	if len(responseJson.Value) == 0 {
		sessionID, err := createAppiumSession(dev)
		if err != nil {
			dev.SetAppiumSessionID("")
			return fmt.Errorf("checkAppiumSession: Could not create new Appium session - %s", err)
		}
		dev.SetAppiumSessionID(sessionID)
		return nil
	}

	dev.SetAppiumSessionID(responseJson.Value[0].ID)
	return nil
}

func createAppiumSession(dev PlatformDevice) (string, error) {
	var automationName = "UiAutomator2"
	var platformName = "Android"
	var waitForIdleTimeout = 10
	if dev.GetOS() == "ios" {
		automationName = "XCUITest"
		platformName = "iOS"
		waitForIdleTimeout = 0
	}

	data := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"alwaysMatch": map[string]interface{}{
				"appium:automationName":     automationName,
				"platformName":              platformName,
				"appium:newCommandTimeout":  1800,
				"appium:waitForIdleTimeout": waitForIdleTimeout,
			},
			"firstMatch": []map[string]interface{}{},
		},
		"desiredCapabilities": map[string]interface{}{
			"appium:automationName":     automationName,
			"platformName":              platformName,
			"appium:newCommandTimeout":  1800,
			"appium:waitForIdleTimeout": waitForIdleTimeout,
		},
	}

	jsonString, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("createAppiumSession: Failed marshalling payload json - %s", err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%v/session", dev.GetAppiumPort()), bytes.NewBuffer(jsonString))
	if err != nil {
		return "", fmt.Errorf("createAppiumSession: Failed creating request for Appium session - %s", err)
	}

	response, err := netClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("createAppiumSession: Failed executing request `%s` - %s", req.URL, err)
	}
	defer response.Body.Close()

	responseBody, _ := io.ReadAll(response.Body)
	var responseJson AppiumCreateSessionResponse
	err = json.Unmarshal(responseBody, &responseJson)
	if err != nil {
		return "", fmt.Errorf("createAppiumSession: Failed unmarshalling Appium session response - %s", err)
	}

	return responseJson.Value.SessionID, nil
}

type AppiumGetSessionsResponse struct {
	Value []struct {
		ID string `json:"id"`
	} `json:"value"`
}

type AppiumCreateSessionResponse struct {
	Value struct {
		SessionID string `json:"sessionId"`
	} `json:"value"`
}
