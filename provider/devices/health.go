package devices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"GADS/common/models"
)

// Check if a device is healthy by checking Appium and WebDriverAgent(for iOS) services
func GetDeviceHealth(device *models.Device) (bool, error) {
	err := checkAppiumSession(device)
	if err != nil {
		return false, err
	}

	return device.Connected, nil
}

func checkAppiumSession(device *models.Device) error {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%s/sessions", device.AppiumPort), nil)
	if err != nil {
		device.AppiumSessionID = ""
		return fmt.Errorf("checkAppiumSession: Failed creating request - %s", err)
	}

	response, err := netClient.Do(req)
	if err != nil {
		device.AppiumSessionID = ""
		return fmt.Errorf("checkAppiumSession: Failed executing request `%s` - %s", req.URL, err)
	}
	responseBody, _ := io.ReadAll(response.Body)

	var responseJson AppiumGetSessionsResponse
	err = json.Unmarshal(responseBody, &responseJson)
	if err != nil {
		device.AppiumSessionID = ""
		return fmt.Errorf("checkAppiumSession: Failed unmarshaling response json - %s", err)
	}

	if len(responseJson.Value) == 0 {
		sessionID, err := createAppiumSession(device)
		if err != nil {
			device.AppiumSessionID = ""
			return fmt.Errorf("checkAppiumSession: Could not create new Appium session - %s", err)
		}
		device.AppiumSessionID = sessionID
		return nil
	}

	device.AppiumSessionID = responseJson.Value[0].ID
	return nil
}

func createAppiumSession(device *models.Device) (string, error) {
	var automationName = "UiAutomator2"
	var platformName = "Android"
	var waitForIdleTimeout = 10
	if device.OS == "ios" {
		automationName = "XCUITest"
		platformName = "iOS"
		waitForIdleTimeout = 0
	}

	data := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"alwaysMatch": map[string]interface{}{
				"appium:automationName":     automationName,
				"platformName":              platformName,
				"appium:newCommandTimeout":  120,
				"appium:waitForIdleTimeout": waitForIdleTimeout,
			},
			"firstMatch": []map[string]interface{}{},
		},
		"desiredCapabilities": map[string]interface{}{
			"appium:automationName":     automationName,
			"platformName":              platformName,
			"appium:newCommandTimeout":  120,
			"appium:waitForIdleTimeout": waitForIdleTimeout,
		},
	}

	jsonString, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("createAppiumSession: Failed marshalling payload json - %s", err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%v/session", device.AppiumPort), bytes.NewBuffer(jsonString))
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
