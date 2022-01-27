#!/bin/bash

# Start the WebDriverAgent on specific WDA and MJPEG ports
start-wda-go-ios() {
  echo "[$(date +'%d/%m/%Y %H:%M:%S')] Starting WebDriverAgent application on port $WDA_PORT"
  ./go-ios/ios runwda --bundleid=$WDA_BUNDLEID --testrunnerbundleid=$WDA_BUNDLEID --xctestconfig=WebDriverAgentRunner.xctest --env USE_PORT=$WDA_PORT --env MJPEG_SERVER_PORT=$MJPEG_PORT --udid $DEVICE_UDID >"/opt/logs/wda-logs.log" 2>&1 &
  sleep 2
}

# Kill the WebDriverAgent app if running on the device or just in case
kill-wda() {
  if ./go-ios/ios ps --udid $DEVICE_UDID | grep $WDA_BUNDLEID; then
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] Attempting to kill WDA app on device"
    ./go-ios/ios kill $WDA_BUNDLEID --udid=$DEVICE_UDID
    sleep 2
  else
    echo "WebDriverAgent is not currently running on the device, nothing to kill."
  fi
}

# Install the WebDriverAgent app on the device
install-wda() {
  echo "[$(date +'%d/%m/%Y %H:%M:%S')] Installing WDA application on device"
  ./go-ios/ios install --path=/opt/WebDriverAgent.ipa --udid=$DEVICE_UDID
}

# Start the WDA service on the device using the WDA bundleId
start-wda() {
  deviceIP=""
  echo "[$(date +'%d/%m/%Y %H:%M:%S')] WDA service is not running/accessible. Attempting to start/restart WDA service..."
  install-wda
  start-wda-go-ios
  #Parse the device IP address from the WebDriverAgent logs using the ServerURL
  #We are trying several times because it takes a few seconds to start the WDA but we want to avoid hardcoding specific seconds wait
  for i in {1..10}; do
    if [ -z "$deviceIP" ]; then
      deviceIP=$(grep "ServerURLHere-" "/opt/logs/wda-logs.log" | cut -d ':' -f 7)
      sleep 2
    else
      break
    fi
  done
  if [[ -z $deviceIP ]]; then
    echo "ERROR! Unable to parse WDA host device ip from log file!"
    docker-cli add-wda-url --wda_url="" --wda_stream_url=""
    # Below exit completely destroys container as there is no sense to continue with undefined WDA_HOST ip!
    exit -1
    else
      docker-cli add-wda-url --wda_url="http:$deviceIP:$WDA_PORT" --wda_stream_url="http:$deviceIP:$MJPEG_PORT"
  fi
}

# Hit WDA status URL and if service not available start it again
check-wda-status() {
  if curl -Is "http:$deviceIP:$WDA_PORT/status" | head -1 | grep -q '200 OK'; then
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] WDA is up and running. Nothing to do"
    sleep 10
  else
    kill-wda
    start-wda
  fi
}

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
  if [ ${ON_GRID} == "true" ]; then
    appium -p $APPIUM_PORT --udid "$DEVICE_UDID" \
      --log-timestamp \
      --default-capabilities \
      '{"mjpegServerPort": '${MJPEG_PORT}', "clearSystemFiles": "false", "webDriverAgentUrl":"'http:$deviceIP:${WDA_PORT}'", "preventWDAAttachments": "true", "simpleIsVisibleCheck": "false", "wdaLocalPort": "'${WDA_PORT}'", "platformVersion": "'${DEVICE_OS_VERSION}'", "automationName":"XCUITest", "platformName": "iOS", "deviceName": "'${DEVICE_NAME}'", "wdaLaunchTimeout": "120000", "wdaConnectionTimeout": "240000", "settings[mjpegServerScreenshotQuality]": 25, "settings[mjpegScalingFactor]": 50, "settings[mjpegServerFramerate]": 20}' \
      --nodeconfig /opt/nodeconfig.json >>"/opt/logs/appium-logs.log" 2>&1 &
  else
    appium -p $APPIUM_PORT --udid "$DEVICE_UDID" \
      --log-timestamp \
      --default-capabilities \
      '{"mjpegServerPort": '${MJPEG_PORT}', "clearSystemFiles": "false", "webDriverAgentUrl":"'http:$deviceIP:${WDA_PORT}'",  "preventWDAAttachments": "true", "simpleIsVisibleCheck": "false", "wdaLocalPort": "'${WDA_PORT}'", "platformVersion": "'${DEVICE_OS_VERSION}'", "automationName":"XCUITest", "platformName": "iOS", "deviceName": "'${DEVICE_NAME}'", "wdaLaunchTimeout": "120000", "wdaConnectionTimeout": "240000", "settings[mjpegServerScreenshotQuality]": 25, "settings[mjpegScalingFactor]": 50, "settings[mjpegServerFramerate]": 20}' >>"/opt/logs/appium-logs.log" 2>&1 &
  fi
}

# Mount the respective Apple Developer Disk Image for the current device OS version
# Skip mounting images if they are already mounted
mount-disk-images() {
  if ./go-ios/ios image list --udid=$DEVICE_UDID 2>&1 | grep "none"; then
    echo "Could not find Developer disk images on the device, mounting.."
    ./go-ios/ios image auto --basedir=/opt/DeveloperDiskImages --udid=$DEVICE_UDID
  else
    echo "Developer disk images are already mounted on the device, nothing to do."
  fi
}

# Pair device using the supervision identity
pair-device() {
  ./go-ios/ios pair --p12file="/opt/supervision.p12" --password="${SUPERVISION_PASSWORD}" --udid=$DEVICE_UDID
}

export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
# Only generate nodeconfig.json if the device will be registered on Selenium Grid
if [ ${ON_GRID} == "true" ]; then
  ./opt/nodeconfiggen.sh > /opt/nodeconfig.json
fi
touch /opt/logs/wda-sync.log
mount-disk-images >> "/opt/logs/wda-sync.log"
while true; do
  check-wda-status >> "/opt/logs/wda-sync.log"
  check-appium-status >> "/opt/logs/wda-sync.log"
done
