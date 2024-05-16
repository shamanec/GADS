# Provider
Currently the project assumes that [GADS UI](https://github.com/shamanec/GADS), MongoDB and device provider are on the same network. They can all be on the same machine as well.  
The provider supports Linux, macOS and Windows  
* Linux - iOS < 17, Android
* macOS - iOS, Android
* Windows - Android

# Setup
## Common
### Start MongoDB instance
**NB** You should have already done this when doing the set up for [GADS UI](https://github.com/shamanec/GADS)
The project uses MongoDB for storing info and syncing info with GADS UI.  
The MongoDB instance does not have to be on the same host as the provider or GADS UI. You just need to provide the correct instance IP address and port for the connection.  

**Prerequisites** You need to have Docker(Docker Desktop on macOS, Windows) installed.  
**NB** You don't have to use docker for MongoDB, you can spin it up any way you prefer.

1. Execute `docker run -d --restart=always --name mongodb -p 27017:27017 mongo:6.0`. This will pull the official MongoDB 6.0 image from Docker Hub and start a container binding port `27017` for db connection.  

or  

1. Start MongoDB instance in the way you prefer

### Add new provider configuration through UI
1. Open the running `GADS` UI.
2. Log in with an admin user
3. Navigate to `Admin` then `Providers administration`
4. Set all required data and add the new provider configuration in the DB.

### Create provider data folder - optional
**NB** This folder will be used to store logs, apps and get files needed by the provider. You can skip this step and then starting the provider will look for `apps` and `logs` folders relative to the folder where the provider binary is located. For example if you run the provider in `/Users/shamanec/Gads-provider` then it will look for `apps` and `logs` in `/Users/shamanec/Gads-provider/apps` and `/Users/shamanec/Gads-provider/logs` respectively. If you create a specific folder and provide it on startup - then the path will be relative to it.  

1. Create a folder on your machine that will be accessible to the provider - name it any way you want.
2. Open the newly created folder and inside create three more folders - `apps`, `logs`, `conf`

### Golang
* Install Go 1.21 or higher

### Appium
* Install Node > 16
* Install Appium with `npm install -g appium`
* Install Appium drivers
    * iOS devices - `appium driver install xcuitestdriver`
    * Android devices - `appium driver install uiautomator2`
* Add any additional Appium dependencies like `ANDROID_HOME`(Android SDK) environment variable, Java, etc.

### Android debug bridge - Android only
* Install `adb` (Android debug bridge). It should be available in PATH so it can be directly accessed via Terminal

### Trust devices to use adb - Android only
* On each device activate `Developer options`, open them and enable `Enable USB debugging`
* Connect each device to the host - a popup will appear on the device to pair - allow it.

### Enable Developer mode - iOS 16+ devices only
* Open Settings > Privacy & Security > Developer Mode
* Enable the toggle

### Set up go-ios - iOS only
1. Download the latest release of [go-ios](https://github.com/danielpaulus/go-ios) and unzip it
* On Macos - Add it to `/usr/local/bin` with `sudo cp ios /usr/local/bin` or to PATH
* On Linux - Add it to `/usr/local/bin` with `sudo cp ios /usr/local/bin` or to PATH
* On Windows - add it to system PATH so its available in Terminal

### GADS Android stream - Android only
1. Starting the provider will automatically download the latest GADS-stream release and put the `apk` file in the `./conf` folder. If you want to "update" it, just delete the current file and restart the provider.

### Supervise devices - iOS only, optional
**NB** You need a Mac machine to do this!  
1. Supervise your iOS devices as explained [here](#supervise-devices--ios-only)  
2. Copy your supervision certificate and add your supervision password as explained [here](#supervise-devices---ios-only)  

**NB** You can skip supervising the devices and you should trust manually on first pair attempt by the provider but it is preferable to have supervised the devices in advance and provided supervision file and password to make setup more autonomous.  

## Linux
### Usbmuxd
* Install usbmuxd - `sudo apt install usbmuxd`

### WebDriverAgent - iOS only
**NB** You need a Mac machine to do this!
1. [Create](#prepare-webdriveragent-file---linux) a `WebDriverAgent.ipa` or `WebDriverAgent.app`  
2. Copy the newly created `ipa/app` in the `/conf` folder with name `WebDriverAgent.ipa` or `WebDriverAgent.app` (exact name is important)

### Known limitations - iOS
1. It is not possible to execute **driver.executeScript("mobile: startPerfRecord")** with Appium to record application performance since Xcode tools are not available.
2. Anything else that might need Instruments and/or any other Xcode/OSX exclusive tools

## macOS
### Xcode - iOS only
* Install latest stable Xcode release.
* Install command line tools with `xcode-select --install`

### WebDriverAgent - iOS only
1. Download the latest release of [WebDriverAgent](https://github.com/appium/WebDriverAgent/releases)
2. Unzip the source code in any folder.
3. Open WebDriverAgent.xcodeproj in Xcode
4. Select signing profiles for WebDriverAgentLib and WebDriverAgentRunner.
5. Run the WebDriverAgentRunner with `Build > Test` on a device at least once to validate it builds and runs as expected.

or
*NB* Using my custom WebDriverAgent you can have faster tap/swipe interactions on iOS devices.  
*NB* The provider configuration should be set to use the custom WebDriverAgent in Mongo - either set it through GADS UI or using any db tool to update the provider config in Mongo for `use_custom_wda` with `true`  
1. Download the code of the `main` branch from my fork of [WebDriverAgent](https://github.com/shamanec/WebDriverAgent)
2. Unzip the code in any folder.
3. Open WebDriverAgent.xcodeproj in Xcode
4. Select signing profiles for WebDriverAgentLib and WebDriverAgentRunner.
5. Run the WebDriverAgentRunner with `Build > Test` on a device at least once to validate it builds and runs as expected.

## Windows
### iTunes - iOS only
* Install `iTunes` to be able to provision iOS < 17 devices

# Configuration setup
The provider can be initialy set up or updated via the GADS UI.  

## Common
### Devices config
No configuration needed, at the moment the provider will attempt to provision every device connected to it.

### Selenium Grid
Devices can be automatically connected to Selenium Grid 4 instance. You need to create the Selenium Grid hub instance yourself and then setup the provider to connect to it.  
To setup the provider download the Selenium server jar [release](https://github.com/SeleniumHQ/selenium/releases/tag/selenium-4.13.0) v4.13. Copy the downloaded jar and put it in the provider `./conf` folder.  
**NOTE** Currently versions above 4.13 don't work with Appium relay nodes and I haven't tested with lower versions. Use lower versions at your own risk.  

# Additional setup notes
## Prepare WebDriverAgent file - Linux, Windows
You need a Mac machine to at least build and sign WebDriverAgent, currently we cannot avoid this.  
You need a paid Apple Developer account to build and sign `WebDriverAgent`. With latest Apple changes it might be possible to do it with free accounts but maybe you'll have to sign the `ipa` file each week and other limitations might apply as well  

1. Download and install [iOS App Signer](https://dantheman827.github.io/ios-app-signer/)  
2. Download the code of the lates mainstream [WebDriverAgent](https://github.com/appium/WebDriverAgent/releases) release or alternatively the code from the `main` branch of my fork of [WebDriverAgent](https://github.com/shamanec/WebDriverAgent) for faster tap/swipe interactions.
2. Open `WebDriverAgent.xcodeproj` in Xcode.  
3. Ensure a team is selected before building the application. To do this go to: *Targets* and select each target one at a time. There should be a field for assigning teams certificates to the target.  
4. Remove your `WebDriverAgent` folder from `DerivedData` and run `Clean build folder` (just in case)  
5. Next build the application by selecting the `WebDriverAgentRunner` target and build for `Generic iOS Device`. Run `Product => Build for testing`. This will create a `Products/Debug-iphoneos` folder in the specified project directory.  
 `Example`: **/Users/<username>/Library/Developer/Xcode/DerivedData/WebDriverAgent-dzxbpamuepiwamhdbyvyfkbecyer/Build/Products/Debug-iphoneos**  
6. Open `iOS App Signer`  
7. Select `WebDriverAgentRunner-Runner.app`.  
8. Generate the WebDriverAgent *.ipa file.  

Alternatively:
7. Copy the `WebDriverAgentRunner-Runner.app` instead of bundling to IPA. `go-ios` allows us to install `app` as well as `ipa` so this might be less painful.

## Supervise the iOS devices - Linux, macOS, Windows - optional
This is a non-mandatory but a preferable step - it will reduce the needed device provisioning manual interactions  
1. Install Apple Configurator 2 on your Mac.
2. Attach your first device.
3. Set it up for supervision using a new(or existing) supervision identity. You can do that for free without having a paid MDM account.
4. Connect each consecutive device and supervise it using the same supervision identity.
5. Export your supervision identity file and choose a password.
6. Save your new supervision identity file in the project `./conf` folder as `supervision.p12`.

**Note** You can also `Trust` manually when connecting a device, might be required again after host/device restart.  

[] TODO - see if supervising can be automated with `go-ios` to skip this step and make set up more autonomous

# Running the provider
## Prebuilt binary
1. Download the prebuilt binary for your OS from the releases

## Build from source
1. Clone the project
2. Execute `go build .`

## Start the provider
1. Execute `./GADS-devices-provider` providing the flags:  
    a. `--nickname=` - this is used to get the correct provider configuration from MongoDB
    b. `--mongo-db=` - address and port of the MongoDB instance
    c. `--provider-folder=` - optional, folder where provider should store logs and apps and get needed files for setup. Can be 1) relative path to the folder where provider binary is located or 2) full path on the host. Default is the folder where the binary is currently located
    d. `--log-level=` - optional, how verbose should the provider logs be, use `debug` for more verbose output, default is `info`

Example default path: `./GADS-devices-provider --nickname=Provider1 --mongo-db=192.168.1.6:27017`  
Example relative path: `./GADS-devices-provider --nickname=Provider1 --mongo-db=192.168.1.6:27017 --provider-folder==./provider-data --log-level=debug`  
Example full path: `./GADS-devices-provider --nickname=Provider1 --mongo-db=192.168.1.6:27017 --provider-folder==/Users/shamanec/provider-data --log-level=debug`  

On start the provider will connect to MongoDB and read its respective configuration data.  

# Logging
Provider logs both to local files and in MongoDB.  

## Provider logs
Provider logs can be found in the `provider.log` file in the `/logs` folder relative to the supplied `provider-folder` flag on start. They will also be in MongoDB in DB `logs` and collection corresponding to the provider name.

## Device logs
On start a log folder and file is created for each device in the `/logs` folder relative to the supplied `provider-folder` flag on start. They will also be in MongoDB in DB `logs` and collection corresponding to the device UDID.
