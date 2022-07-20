#!/bin/bash

# Hit the Appium status URL to see if it is available and start it if not
check-appium-status() {
  if curl -Is "http://127.0.0.1:4723/wd/hub/status" | head -1 | grep -q '200 OK'; then
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] Appium is already running. Nothing to do"
  else
    start-appium
  fi
}

# Start appium server for the device
# If the device is on Selenium Grid use created nodeconfig.json, if not - skip applying it in the command
start-appium() {
  if [ ${ON_GRID} == "true" ]; then
    appium -p 4723 --udid "$DEVICE_UDID" \
      --log-timestamp \
      --allow-cors \
      --session-override \
      --allow-insecure chromedriver_autodownload \
      --default-capabilities \
      '{"automationName":"UiAutomator2", "platformName": "Android", "deviceName": "'${DEVICE_NAME}'"}' \
      --nodeconfig /opt/nodeconfig.json >>/opt/logs/appium-logs.log 2>&1 &
  else
    appium -p 4723 --udid "$DEVICE_UDID" \
      --log-timestamp \
      --allow-cors \
      --session-override \
      --allow-insecure chromedriver_autodownload \
      --default-capabilities \
      '{"automationName":"UiAutomator2", "platformName": "Android", "deviceName": "'${DEVICE_NAME}'"}' >>/opt/logs/appium-logs.log 2>&1 &
  fi
}

export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
if [ ${ON_GRID} == "true" ]; then
  /opt/nodeconfiggen.sh > /opt/nodeconfig.json
fi
adb forward tcp:1313 localabstract:minicap
touch /opt/logs/minicap.log
touch /opt/logs/appium-logs.log
/opt/container_server 2>&1 &
cd /root/minicap/ && ./run.sh autosize >>/opt/logs/minicap.log 2>&1 &
docker-cli stream-minicap --port=4724 >>/opt/logs/minicap.log 2>&1 &
while true; do
  check-appium-status
  sleep 10
done