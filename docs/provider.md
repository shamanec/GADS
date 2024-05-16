The provider component is what actually sets up the Appium servers and all other dependencies for the connected devices and exposes the devices for testing or remote control.

- [Provider configuration](#provider-configuration)
- [Provider data folder](#provider-data-folder---optional)
- [Provider setup](#provider-setup)
  - [macOS](#macos)
  - [Linux](#linux)
  - [Windows](#windows)
- [Dependencies notes](#dependencies-notes)
- [Devices notes](#devices-notes)
- [Logging](#logging)
- [Additional notes](#additional-notes)
  - [Selenium Grid](#selenium-grid)

## Provider configuration

Provider configuration is added through the GADS UI
- Log in the hub UI with an admin user.
- Go to the `Admin` section.
- Open `Providers administration`
- On the `New provider` tab fill in all needed data and save.
- You should see a new provider tab with the nickname you provided. You can now start up a provider instance using the newly added configuration.

## Provider data folder - optional
The provider needs a persistent folder where logs, apps and other files might be stored.  

You can skip this step and then starting the provider will look for `apps` and `logs` folders relative to the folder where the provider binary is located. 
For example if you run the provider in `/Users/shamanec/Gads-provider` then it will look for `apps` and `logs` as `/Users/shamanec/Gads-provider/apps` and `/Users/shamanec/Gads-provider/logs` respectively.   

If you create a specific folder and provide it on startup - then the path will be relative to it.  
Refer to the `--provider-folder` flag in [Running a provider instance](#running-a-provider-instance)

1. Create a folder on your machine that will be accessible to the provider - name it any way you want.
2. Open the newly created folder and inside create three more folders - `apps`, `logs`, `conf`

## Provider setup
### macOS
#### Common
- Install [Appium](#appium)

#### Android
- Install [adb](#adb---android-debug-bridge) if providing Android devices
- Enable [USB Debugging](#usb-debugging) on each Android device

#### iOS
- Install [go-ios](#go-ios) if providing iOS devices
- Install [usbmuxd](#usbmuxd) if providing iOS devices
- Prepare [WebDriverAgent](#prepare-webdriveragent-on-macos)
- [Optional] [Supervise](#supervise-devices) your iOS devices

### Linux
#### Common
- Install [Appium](#appium)

#### Android
- Install [adb](#adb---android-debug-bridge) if providing Android devices
- Enabled [USB Debugging](#usb-debugging) on each Android device

#### iOS
- Install [go-ios](#go-ios) if providing iOS devices
- Install [usbmuxd](#usbmuxd) if providing iOS devices
- Prepare [WebDriverAgent](#prepare-webdriveragent-file---linux-windows) file
- [Optional] [Supervise](#supervise-devices) your iOS devices

#### Known limitations - Linux, iOS
1. It is not possible to execute **driver.executeScript("mobile: startPerfRecord")** with Appium to record application performance since Xcode tools are not available.
2. Anything else that might need Instruments and/or any other Xcode/macOS exclusive tools

### Windows
#### Common
- Install [Appium](#appium)

#### Android
- Install [adb](#adb---android-debug-bridge) if providing Android devices
- Enabled [USB Debugging](#usb-debugging) on each Android device

#### iOS
- Install [go-ios](#go-ios) if providing iOS devices
- Install [iTunes](#itunes) if providing iOS devices
- Prepare [WebDriverAgent](#prepare-webdriveragent-file---linux-windows) file
- [Optional] [Supervise](#supervise-devices) your iOS devices

#### Known limitations - Windows, iOS
1. It is not possible to execute **driver.executeScript("mobile: startPerfRecord")** with Appium to record application performance since Xcode tools are not available.
2. Anything else that might need Instruments and/or any other Xcode/macOS exclusive tools

### Running a provider instance
- Execute `./GADS provider` providing the following flags:  
  - `--nickname=` - mandatory, this is used to get the correct provider configuration from MongoDB
  - `--mongo-db=` - optional, IP address and port of the MongoDB instance (default is `localhost:27017`)
  - `--provider-folder=` - optional, folder where provider should store logs and apps and get needed files for setup. Can be 1) relative path to the folder where provider binary is located or 2) full path on the host. Default is the folder where the binary is currently located, default is `.`
  - `--log-level=` - optional, how verbose should the provider logs be (default is `info`, use `debug` for more log output)

### Dependencies notes
#### Appium
Appium is foundational in GADS - we use it both to provider Appium servers to run UI tests against, but also to allow the interactions in the web remote control.  
Installation is pretty similar for all operating systems, you just have to find the proper steps for your setup.
- Install Node > 16
- Install Appium with `npm install -g appium`
- Install Appium drivers
  - iOS - `appium driver install xcuitestdriver`
  - Android - `appium driver install uiautomator2`
- Add any additional Appium dependencies like `ANDROID_HOME`(Android SDK) environment variable, Java, etc.
- Test with `appium driver doctor uiautomator2` and `appium driver doctor xcuitest` to check for errors with the setup.

#### adb - Android Debug Bridge
`adb` (Android Debug Bridge) is mandatory when providing Android devices. You can skip installing it if no Android devices will be provided. 
- Install `adb` in a valid way for the provider OS. It should be available in PATH so it can be directly accessed via terminal

#### go-ios
`go-ios` is mandatory when providing iOS devices on any host OS
- Download the latest release of [go-ios](https://github.com/danielpaulus/go-ios) for the provider host OS and unzip it
- On macOS - Add it to `/usr/local/bin` with `sudo cp ios /usr/local/bin` or to PATH
- On Linux - Add it to `/usr/local/bin` with `sudo cp ios /usr/local/bin` or to PATH
- On Windows - add it to system PATH so that it is available in terminal

#### iTunes
`iTunes` is needed only on Windows and mandatory when providing iOS devices. Install it through an installation package or Microsoft Store, shouldn't really matter

#### usbmuxd
`usbmuxd` is used only on Linux and only when providing iOS devices.  
Example installation command for Ubuntu -  `sudo apt install usbmuxd`.

### Devices notes
#### iOS
##### Enable Developer mode - iOS 16+ only
Developer mode needs to be enabled on iOS 16+ devices to allow Xcode and `go-ios` usage against the device
- Open `Settings > Privacy & Security > Developer Mode`
- Enable the toggle

##### Supervise devices
This is an optional but a preferable step - it can make devices setup more autonomous - it can allow trusted pairing with devices without interacting with Trust popup  
**NB** You need a Mac machine to do this!

- Install Apple Configurator 2 on your Mac.
- Attach your first device.
- Set it up for supervision using a new(or existing) supervision identity. You can do that for free without having a paid MDM account.
- Connect each consecutive device and supervise it using the same supervision identity.
- Export your supervision identity file and choose a password.
- Save your new supervision identity file in the `./conf` folder as `supervision.p12`.

**NB** You can skip supervising the devices and you should trust manually on first pair attempt by the provider but if devices are supervised in advance setup can be more autonomous.

##### Prepare WebDriverAgent on macOS
- Download the latest release of [WebDriverAgent](https://github.com/appium/WebDriverAgent/releases)
- Unzip the source code in any folder.
- Open WebDriverAgent.xcodeproj in Xcode
- Select signing profiles for WebDriverAgentLib and WebDriverAgentRunner.
- Run the WebDriverAgentRunner with `Build > Test` on a device at least once to validate it builds and runs as expected.

or  
**NB** Using my custom WebDriverAgent you can have faster tap/swipe interactions on iOS devices.  
**NB** The provider configuration should be set to use the custom WebDriverAgent in Mongo - either set it through GADS UI or using any db tool to update the provider config in Mongo for `use_custom_wda` with `true`
- Download the code of the `main` branch from my fork of [WebDriverAgent](https://github.com/shamanec/WebDriverAgent)
- Unzip the code in any folder.
- Open WebDriverAgent.xcodeproj in Xcode
- Select signing profiles for WebDriverAgentLib and WebDriverAgentRunner.
- Run the WebDriverAgentRunner with `Build > Test` on a device at least once to validate it builds and runs as expected.

##### Prepare WebDriverAgent file - Linux, Windows
You need a Mac machine to at least build and sign WebDriverAgent, currently we cannot avoid this.  
You need a paid Apple Developer account to build and sign `WebDriverAgent` if you have more than 2 devices?.

- Download and install [iOS App Signer](https://dantheman827.github.io/ios-app-signer/)
- Download the code of the latest mainstream [WebDriverAgent](https://github.com/appium/WebDriverAgent/releases) release or alternatively the code from the `main` branch of my fork of [WebDriverAgent](https://github.com/shamanec/WebDriverAgent) for faster tap/swipe interactions.
- Open `WebDriverAgent.xcodeproj` in Xcode.
- Ensure a team is selected before building the application. To do this go to: *Targets* and select each target one at a time. There should be a field for assigning teams certificates to the target.
- Remove your `WebDriverAgent` folder from `DerivedData` and run `Clean build folder` (just in case)
- Next build the application by selecting the `WebDriverAgentRunner` target and build for `Generic iOS Device`. Run `Product => Build for testing`. This will create a `Products/Debug-iphoneos` folder in the specified project directory.  
   `Example`: **/Users/<username>/Library/Developer/Xcode/DerivedData/WebDriverAgent-dzxbpamuepiwamhdbyvyfkbecyer/Build/Products/Debug-iphoneos**
- Open `iOS App Signer`
- Select `WebDriverAgentRunner-Runner.app`.
- Generate the WebDriverAgent *.ipa file.

#### Android
##### USB Debugging
* On each device activate `Developer options`, open them and enable `Enable USB debugging`
* Connect each device to the host - a popup will appear on the device to pair - allow it.

### Logging
Provider logs both to local files and to MongoDB.

#### Provider logs
Provider logs can be found in the `provider.log` file in the `/logs` folder relative to the supplied `provider-folder` flag on start.  
They will also be stored in MongoDB in DB `logs` and collection corresponding to the provider nickname.

#### Device logs
On start a log folder and file is created for each device in the `/logs` folder relative to the supplied `provider-folder` flag on start.  
They will also be stored in MongoDB in DB `logs` and collection corresponding to the device UDID.

### Additional notes
#### Selenium Grid
Devices can be automatically connected to Selenium Grid 4 instance. You need to create the Selenium Grid hub instance yourself and then setup the provider to connect to it.  
To setup the provider download the Selenium server jar [release](https://github.com/SeleniumHQ/selenium/releases/tag/selenium-4.13.0) v4.13. Copy the downloaded jar and put it in the provider `./conf` folder.  
**NOTE** Currently versions above 4.13 don't work with Appium relay nodes and I haven't tested with lower versions. Use lower versions at your own risk.
