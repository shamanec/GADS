#!/bin/bash

cat << EndOfMessage
{
  "capabilities":
      [
        {
          "browserName": "${DEVICE_NAME}",
          "version":"${DEVICE_OS_VERSION}",
          "maxInstances": 1,
          "platform":"Android",
	  "deviceName": "${DEVICE_NAME}",
          "deviceType": "phone",
          "platformName":"Android",
          "platformVersion":"${DEVICE_OS_VERSION}",
	  "udid": "${DEVICE_UDID}"
        }
      ],
  "configuration":
  {
    "url":"http://${DEVICES_HOST}:${APPIUM_PORT}/wd/hub",
    "port": ${APPIUM_PORT},
    "host": "${DEVICES_HOST}",
    "hubPort": ${SELENIUM_HUB_PORT},
    "hubHost": "${SELENIUM_HUB_HOST}",
    "timeout": 180,
    "maxSession": 1,
    "register": true,
    "registerCycle": 5000,
    "automationName": "UiAutomator2",
    "downPollingLimit": 10,
    "hubProtocol": "${HUB_PROTOCOL}"
  }
}
EndOfMessage
