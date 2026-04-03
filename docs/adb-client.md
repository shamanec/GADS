# Android adb-client

## Overview

GADS allows you to connect Android devices to your local `adb` instance for debugging and development. Communication is authenticated and goes through the hub.

## Usage

1. Log in to the hub web interface and start remotely controlling an available Android device.
2. Start the Android client adb tunnel with `./GADS adb-tunnel --hub={GADS hub address} --username={GADS username} --password={GADS password} --udid={device-udid}`, e.g. `./GADS adb-tunnel --hub=http://192.168.1.24:10000 --username=admin --password=password --udid=ABC123`.
3. Wait for the tunnel connection to be established.
4. Run `adb devices` - you should see the device connected - you can now use the device through Android Studio for example for live development and debugging of applications.

## Notes

- You can only create tunnel to devices that are currently being remotely controlled by you.
- Stopping the remote control of the device through the hub interface will also drop the tunnel connection.
- Stopping the adb client will not drop your remote control session.
