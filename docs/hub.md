# Hub Setup

Unless you are building from source, running the hub does not require any additional dependencies except a running MongoDB instance.

## IMPORTANT

This is only hub/UI, to actually have devices available you need to have at least one [**provider**](./provider.md) instance running on the same host (or another host on the same network) that will actually set up and provision devices.  
Follow the setup steps to create and run a provider instance.  
You can have multiple provider instances on different hosts providing devices.

## Starting hub instance

Run `./GADS hub` with the following flags:

- `--host-address=` - local IP address of the host machine, e.g. `192.168.1.6` (default is `localhost`, I would advise against using the default value)
- `--port=` - port on which the UI and backend service will be served
- `--auth=` - enable/disable authentication. When disabled you can access any UI page/hub endpoint without login token validation, note that this is **highly insecure** and should be used only for development - `true/false`
- `--mongo-db=` - IP address and port of the MongoDB instance, e.g `192.168.1.6:27017` (default is `localhost:27017`) - tested only on local network
- `--files-dir=` - directory where the UI static files will be unpacked and served from. By default the app tries to use a temporary folder available on the host automatically. **NB** Use this flag only if you have issues with the default behaviour.

Then access the hub UI and API on `http://{host-address}:{port}`

## UI development

If you want to work on the React UI with hot reload you need to add a proxy in `package.json` to point to the Go backend

1. Open the `hub/gads-ui` folder.
2. Open the `package-json` file.
3. Add a new field `"proxy": "http://192.168.1.28:10000/"` providing the host and port of the Go backend service.
4. Run `npm start`

## Additional notes

### Users administration

You can add/delete users and change their roles/passwords via the `Admin` panel.  
There are no limitations on usernames and passwords - only the default `admin` user cannot be deleted and its role changed(you can change its password though)

### Providers administration

For each provider instance you need to create a provider configuration via the `Admin` panel.  
All fields have tooltips to help you with the required information.

### Devices administration

Device configurations are added via the `Admin` panel.  
You have to provide all the required information and assign each device to a provider.  
Changes to the device configuration require the respective provider instance restarted.  
All fields have tooltips to help you with the required information.

**Android emulators** are the exception - providers with `Provide Android emulators?` enabled discover and report running emulators automatically as ephemeral devices. They never appear in `Admin > Devices` (there is nothing to configure), show up in the device selection list with an emulator badge while running, and are removed within ~15 seconds after the emulator or its provider stops. See the [provider documentation](./provider.md#android-emulators) for details.

### Appium grid

Using Selenium Grid 4 is a bit of a hassle and some versions do not work properly with Appium relay nodes.  
For this reason the hub embeds its own grid implementation - no Selenium Grid required. Point your Appium/Selenium driver URL at the hub, e.g. `http://192.168.1.6:10000/grid`, and create sessions as you usually would with any Appium language client.

**Session requests**
- Requests must use the W3C `capabilities` format (`alwaysMatch`/`firstMatch`). Legacy `desiredCapabilities`-only requests are rejected with `400 invalid argument` - Appium 2+ does not accept them either
- Every session request must carry the `gads:clientSecret` capability for authentication - see [Appium client credentials](./appium-credentials.md). All `gads:*` capabilities are stripped before the request is forwarded, so the secret never appears in Appium logs

**Device targeting**
- By UDID via `appium:udid`
- By `platformName` (iOS or Android) or `appium:automationName` (XCUITest or UiAutomator2)
  - Additionally the grid allows filtering by `appium:platformVersion` capability which supports exact version e.g. `17.5.1` or a major version e.g. `17`, `11` etc
- Devices whose usage is set to `Control` or `Disabled`, and devices on a provider configured without Appium servers, are never dispatched - requests pinned to one by UDID fail immediately with the reason instead of queueing

**Queueing**
- When no matching device is free the request waits in a FIFO queue - first come, first served
- The default wait is 10 seconds; the `gads:queueTimeout` capability (seconds, clamped to 300) sets it per request, and `0` means fail immediately when nothing is free

**Session behavior**
- `appium:newCommandTimeout` is honored by the hub as well (default 60 seconds); an explicit `0` disables idle expiry entirely
- Only the exact `DELETE /grid/session/{id}` ends a session - DELETEs on subpaths (`/window`, `/cookie`, `/actions`) are proxied as ordinary commands
- BiDi is not supported: sessions requesting `webSocketUrl: true` still create fine, but the `webSocketUrl` capability is always removed from the response

**Response enrichment** - a successful session response includes extra `gads:*` capabilities telling you which device you actually got:
- `gads:deviceUdid`, `gads:deviceName`, `gads:provider`
- `gads:controlUrl` - a direct link to the hub's remote control UI for the device serving your test; the session owner can also attach from the device list via the `Use` button (after confirming) and watch the test live

**Observability**
- `GET /grid/status` reports overall grid readiness; without credentials that is all it reports. Send your client secret as `Authorization: Bearer <secret>` to also get the per-device availability list and a readiness flag scoped to your tenant's workspaces
- `GET /automation-sessions` (authenticated) lists the currently active automation sessions

### Android devices remote control debugging

GADS allows you to create an adb tunnel to a remotely controlled Android device for local development and debugging - find more information on usage [here](./adb-tunnel.md)
