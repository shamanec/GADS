package device

import (
	"GADS/db"
	"GADS/util"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var latestDevices []Device

// Get the latest devices information from RethinkDB each second
func GetLatestDBDevices() {
	for {
		// Get a cursor of the whole "devices" table
		cursor, err := r.Table("devices").Run(db.DBSession)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "get_devices_db",
			}).Error("Could not get devices from DB, err: " + err.Error())
			latestDevices = []Device{}
		}
		defer cursor.Close()

		// Retrieve all documents from the DB into the Device slice
		err = cursor.All(&latestDevices)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "get_devices_db",
			}).Error("Could not get devices from DB, err: " + err.Error())
			latestDevices = []Device{}
		}
		time.Sleep(1 * time.Second)
	}
}

func CheckWDASession(wdaURL string) (string, error) {
	response, err := http.Get("http://" + wdaURL + "/status")
	if err != nil {
		return "", err
	}

	responseBody, _ := io.ReadAll(response.Body)

	var responseJson map[string]interface{}
	err = json.Unmarshal(responseBody, &responseJson)
	if err != nil {
		return "", err
	}

	if responseJson["sessionId"] == "" || responseJson["sessionId"] == nil {
		sessionId, err := createWDASession(wdaURL)
		if err != nil {
			return "", err
		}

		if sessionId == "" {
			return "", err
		}
	}

	return fmt.Sprintf("%v", responseJson["sessionId"]), nil
}

func createWDASession(wdaURL string) (string, error) {
	requestString := `{
		"capabilities": {
			"firstMatch": [
				{
					"arguments": [],
					"environment": {},
					"eventloopIdleDelaySec": 0,
					"shouldWaitForQuiescence": true,
					"shouldUseTestManagerForVisibilityDetection": false,
					"maxTypingFrequency": 60,
					"shouldUseSingletonTestManager": true,
					"shouldTerminateApp": true,
					"forceAppLaunch": true,
					"useNativeCachingStrategy": true,
					"forceSimulatorSoftwareKeyboardPresence": false
				}
			],
			"alwaysMatch": {}
		}
	}`

	response, err := http.Post("http://"+wdaURL+"/session", "application/json", strings.NewReader(requestString))
	if err != nil {
		return "", err
	}

	responseBody, _ := io.ReadAll(response.Body)

	var responseJson map[string]interface{}
	err = json.Unmarshal(responseBody, &responseJson)
	if err != nil {
		return "", err
	}

	if responseJson["sessionId"] == "" || responseJson["sessionId"] == nil {
		if err != nil {
			return "", errors.New("Could not get `sessionId` while creating a new WebDriverAgent session")
		}
	}

	return fmt.Sprintf("%v", responseJson["sessionId"]), nil
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

func checkAppiumSession(appiumURL string) (string, error) {
	response, err := http.Get("http://" + appiumURL + "/sessions")
	if err != nil {
		return "", err
	}
	responseBody, _ := io.ReadAll(response.Body)

	var responseJson AppiumGetSessionsResponse
	err = util.UnmarshalJSONString(string(responseBody), &responseJson)
	if err != nil {
		return "", err
	}

	if len(responseJson.Value) == 0 {
		sessionID, err := createAppiumSession(appiumURL)
		if err != nil {
			return "", err
		}
		return sessionID, nil
	}

	return responseJson.Value[0].ID, nil
}

func createAppiumSession(appiumURL string) (string, error) {
	requestString := `{
		"capabilities": {
			"alwaysMatch": {
				"appium:automationName": "UiAutomator2",
				"platformName": "Android",
				"appium:ensureWebviewsHavePages": true,
				"appium:nativeWebScreenshot": true,
				"appium:newCommandTimeout": 0,
				"appium:connectHardwareKeyboard": true
			},
			"firstMatch": [
				{}
			]
		},
		"desiredCapabilities": {
			"appium:automationName": "UiAutomator2",
			"platformName": "Android",
			"appium:ensureWebviewsHavePages": true,
			"appium:nativeWebScreenshot": true,
			"appium:newCommandTimeout": 0,
			"appium:connectHardwareKeyboard": true
		}
	}`

	response, err := http.Post("http://"+appiumURL+"/session", "application/json", strings.NewReader(requestString))
	if err != nil {
		return "", err
	}

	responseBody, _ := io.ReadAll(response.Body)
	var responseJson AppiumCreateSessionResponse
	err = util.UnmarshalJSONString(string(responseBody), &responseJson)
	if err != nil {
		return "", err
	}

	return responseJson.Value.SessionID, nil
}

func GetDeviceByUDID(udid string) *Device {
	for _, device := range latestDevices {
		if device.UDID == udid {
			return &device
		}
	}

	return nil
}
