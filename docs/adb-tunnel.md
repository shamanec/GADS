# Android adb-tunnel

## Overview

GADS allows you to connect Android devices to your local `adb` instance for debugging and development. Communication is authenticated and goes through the hub.

## Usage

1. Log in to the hub web interface and start remotely controlling an available Android device.
2. Start the Android client adb tunnel with `./GADS adb-tunnel --hub={GADS hub address} --username={GADS username} --password={GADS password} --udid={device-udid}`, e.g. `./GADS adb-tunnel --hub=http://192.168.1.24:10000 --username=admin --password=password --udid=ABC123`.
3. Wait for the tunnel connection to be established.
4. Run `adb devices` - you should see the device connected - you can now use the device through Android Studio for example for live development and debugging of applications.

## Usage for SSO users

SSO users do not have a local password, so they must authenticate with a JWT access token instead:

1. Sign in to the hub through SSO (open `/auth/sso/login` on the hub) and start remotely controlling an available Android device.
2. Open the browser DevTools, go to the Console tab and run:

   ```js
   copy(localStorage.getItem('accessToken'))
   ```

   This copies your current access token to the clipboard - the SSO sign-in stores it under the `accessToken` key of the hub's local storage.
3. Start the tunnel with `--token` instead of `--password`:

   ```bash
   ./GADS adb-tunnel --hub=http://192.168.1.24:10000 --udid=ABC123 --token=<paste-token-here>
   ```

   You can also pass the token via the `GADS_TOKEN` environment variable instead of `--token`.
4. Continue with steps 3-4 from the regular usage section above.

**Note:** the token is bound to your hub session, which ends after `--token-ttl` of inactivity, after `--max-session-age` since login or when the hub restarts - see [Authentication sessions](hub.md#authentication-sessions). After that, copy a fresh token from `localStorage.accessToken` and restart the tunnel.

## Notes

- You can only create tunnel to devices that are currently being remotely controlled by you.
- Stopping the remote control of the device through the hub interface will also drop the tunnel connection.
- Stopping the adb tunnel will not drop your remote control session.
