#!/bin/bash

# Hit the Appium status URL to see if it is available and start it if not
check-appium-status() {
  if curl -Is "http://127.0.0.1:${APPIUM_PORT}/wd/hub/status" | head -1 | grep -q '200 OK'; then
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] Appium is already running. Nothing to do"
  else
    start-appium
  fi
}

# Start appium server for the device
# If the device is on Selenium Grid use created nodeconfig.json, if not - skip applying it in the command
start-appium() {
    appium -p $APPIUM_PORT --udid "$DEVICE_UDID" \
      --log-timestamp \
      --allow-cors \
      --session-override \
      --default-capabilities \
      '{"automationName":"UiAutomator2", "platformName": "Android", "deviceName": "Test"}' >> /opt/logs/appium-logs.log 2>&1 &
}

export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
adb forward tcp:1313 localabstract:minicap
touch /opt/logs/minicap.log
touch /opt/logs/appium-logs.log
cd /root/minicap/ && ./run.sh autosize >> /opt/minicap.log 2>&1 &
docker-cli stream-minicap --port=$STREAM_PORT >> /opt/minicap.log 2>&1 &
while true; do
  check-appium-status
  sleep 10
done
