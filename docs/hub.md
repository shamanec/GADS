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

### Experimental Appium grid

Using Selenium Grid 4 is a bit of a hassle and some versions do not work properly with Appium relay nodes.  
For this reason I created an experimental grid implementation into the hub itself.  
I haven't even read the Selenium Grid implementation and made up something myself - it might not work properly but could be the better alternative if it does work properly.  
The experimental grid was tested only using latest Appium and Selenium Java client versions and with TestNG. Tests can be executed sequentially or in parallel using TestNG with `methods` or `classes` with multiple threads. I assume it should support any type of session creation with any Appium language client

- The grid is accessible on your hub instance e.g. `http://192.168.1.6:10000/grid` and should be used as Appium/Selenium driver URL target. You just try to start a session as you usually do with Selenium Grid
- The grid allows targeting devices by UDID
- The grid allows targeting devices by `platformName`(iOS or Android) or `appium:automationName`(XCUITest or UiAutomator2) capabilities during session creation
  - Additionally the grid allows filtering by `appium:platformVersion` capability which supports exact version e.g. `17.5.1` or a major version e.g. `17`, `11` etc

### Android devices remote control debugging

GADS allows you to create an adb tunnel to a remotely controlled Android device for local development and debugging - find more information on usage [here](./adb-tunnel.md)
