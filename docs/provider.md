# Setup Guide for Provider Component

The provider component is responsible for setting up the Appium servers and managing dependencies for connected devices. It exposes these devices for testing or remote control.

## Table of Contents
- [Provider Configuration](#provider-configuration)
- [Provider Data Folder](#provider-data-folder---optional)
- [Provider Setup](#provider-setup)
  - [macOS](#macos)
  - [Linux](#linux)
  - [Windows](#windows)
- [Dependencies Notes](#dependencies-notes)
- [Device Notes](#device-notes)
  - [iOS Phones](#ios-phones)
  - [Android](#android-phone)
  - [Tizen TV](#tizen-tv)
  - [WebOS TV](#webos-tv)
- [Starting Provider Instance](#starting-a-provider-instance)
- [Logging](#logging)

## Provider Configuration

**Provider configuration is added through the GADS UI:**
1. Log in to the hub UI with an admin user.
2. Navigate to the `Admin` section.
3. Open `Providers`.
4. On the `New provider` tab, fill in all necessary data and save.
5. You should see a new provider component with the configuration you provided. You can now start up a provider instance using this configuration.

## Provider Data Folder - Optional

The provider requires a persistent folder to store logs, apps, and other files. 

You can skip this step, and the provider will create a folder named after the provider instance nickname relative to where the provider binary is located. For example, if you run the provider in `/Users/shamanec/GADS` with the nickname `Provider1`, it will create `/Users/shamanec/GADS/Provider1` to store all related data.

To specify a folder, create it on your machine and provide it at startup using the `--provider-folder` flag.

## Provider Setup

### macOS

#### Common
- **Install** [Appium](#appium)

#### Android
-  **Install** [ADB (Android Debug Bridge)](#adb---android-debug-bridge) if providing Android devices.
-  **Enable** [USB Debugging](#usb-debugging) on each Android device.

#### iOS
-  **Prepare** [WebDriverAgent](#build-webdriveragent-ipa-file-manually-using-xcode).
- (Optional) **Supervise** [your iOS devices](#supervise-devices).

#### Tizen
- **Install** [SDB (Smart Development Bridge)](#sdb---tizen-only)
- **Enable** [Developer Mode](#developer-mode-tizen) on each Tizen TV

#### WebOS
- **Install** [WebOS CLI](#webos-cli---webos-only)
- **Enable** [Developer Mode](#developer-mode---webos) on each WebOS TV

<br>

---

### Linux
#### Common
- **Install** [Appium](#appium)

#### Android
- **Install** [ADB (Android Debug Bridge)](#adb---android-debug-bridge) if providing Android devices.
- **Enable** [USB Debugging](#usb-debugging) on each Android device.

#### iOS
- **Install** [usbmuxd](#usbmuxd) if providing iOS devices.
- **Prepare** [WebDriverAgent](#prebuilt-custom-webdriveragent).
- (Optional) **Supervise** [your iOS devices](#supervise-devices).

#### Tizen
- **Install** [SDB (Smart Development Bridge)](#sdb---tizen-only)
- **Enable** [Developer Mode](#developer-mode-tizen) on each Tizen TV

#### WebOS
- **Install** [WebOS CLI](#webos-cli---webos-only)
- **Enable** [Developer Mode](#developer-mode---webos) on each WebOS TV

#### ⚠️ Known Limitations - Linux, iOS
1. The command **driver.executeScript("mobile: startPerfRecord")** cannot be executed due to the unavailability of Xcode tools.
2. Any functionality requiring Instruments or other Xcode/macOS-exclusive tools is limited.

<br>

---

### Windows

#### Common
- **Install** [Appium](#appium)

#### Android
- **Install** [ADB (Android Debug Bridge)](#adb---android-debug-bridge) if providing Android devices.
- **Enable** [USB Debugging](#usb-debugging) on each Android device.

#### iOS
- **Install** [iTunes](#itunes) if providing iOS devices.
- **Prepare** [WebDriverAgent](#prebuilt-custom-webdriveragent).
- (Optional) **Supervise** [your iOS devices](#supervise-devices).

#### Tizen
- **Install** [SDB (Smart Development Bridge)](#sdb---tizen-only)
- **Enable** [Developer Mode](#developer-mode-tizen) on each Tizen TV

#### WebOS
- **Install** [WebOS CLI](#webos-cli---webos-only)
- **Enable** [Developer Mode](#developer-mode---webos) on each WebOS TV

#### ⚠️ Known Limitations - Windows, iOS
1. The command **driver.executeScript("mobile: startPerfRecord")** cannot be executed due to the unavailability of Xcode tools.
2. Any functionality requiring Instruments or other Xcode/macOS-exclusive tools is limited.



## Dependencies notes
### Appium - optional 
If you want the configured devices to each have a respective Appium server set up registered in Selenium Grid or the GADS Appium grid for test execution you need to enable this in the provider configuration in the Admin UI!!!  
**NOTE** Appium has to be installed and set up on the provider host machine if you want to take advantage of this.  
Installation is pretty similar for all operating systems, you just have to find the proper steps for your setup.
- Install Node > 16
- Install Appium with `npm install -g appium`
- Install Appium drivers
  - iOS - `appium driver install xcuitest`
  - Android - `appium driver install uiautomator2`
  - Tizen TV - `appium driver install --source=npm appium-tizen-tv-driver`
  - WebOS TV - `appium driver install --source=npm appium-lg-webos-driver`
- Add any additional Appium dependencies like `ANDROID_HOME`(Android SDK) environment variable, Java, etc.
- Test with `appium driver doctor uiautomator2` and `appium driver doctor xcuitest` to check for errors with the setup.

<br>

---

### adb - Android Only

`adb` (Android Debug Bridge) is mandatory when providing Android devices. You can skip installing it if no Android devices will be provided. 
- Install `adb` in a valid way for the provider OS. It should be available in PATH so it can be directly accessed via terminal. <br>
Example installation on macOS - `brew install adb`

<br>

---

### usbmuxd - Linux -> iOS
`usbmuxd` is used only on **Linux** and only when providing **iOS devices**.  
Example installation command for Ubuntu -  `sudo apt install usbmuxd`.

--- 

### iTunes - Windows -> iOS
`iTunes` is needed only on **Windows** and mandatory when providing **iOS devices**. Install it through an installation package or Microsoft Store, shouldn't really matter

### WebDriverAgent -> iOS

#### WebDriverAgent ipa
You need to prepare and upload a signed `WebDriverAgent` ipa file from the hub UI in `Admin > Files`  
GADS supports only WebDriverAgent from my [fork](https://github.com/shamanec/WebDriverAgent).  
The fork has optimizations for the mjpeg video stream and additional endpoints for faster tap/swipe interactions that are not available in the mainstream repo.  
Additionally those endpoints require different coordinates for interaction from mainstream WDA which forces separate handling for the remote control which is too much work.  
Fork is kept up to date with latest mainstream.  
  
#### Prebuilt custom WebDriverAgent
- Download the prebuilt `WebDriverAgent.ipa` from my fork of [WebDriverAgent](https://github.com/shamanec/WebDriverAgent)
- Use any tool to re-sign it with your developer account (or provisioning profile + certificate)
  - [zsign](https://github.com/zhlynn/zsign)
  - [fastlane-sigh](https://docs.fastlane.tools/actions/sigh/)
  - [codesign](https://developer.apple.com/library/archive/documentation/Security/Conceptual/CodeSigningGuide/Procedures/Procedures.html)
  - Re-sign from hub UI - TODO

#### Build WebDriverAgent IPA file manually using Xcode
- Download the code from the `main` branch of my fork of [WebDriverAgent](https://github.com/shamanec/WebDriverAgent).
- Open `WebDriverAgent.xcodeproj` in Xcode.
- Select signing profile for WebDriverAgentRunner. To do this go to: *Targets*, select WebDriverAgentRunner. There should be a field for assigning teams certificates to the target.
- Select `Build > Clean build folder` (just in case)
- Next build the application by selecting the `WebDriverAgentRunner` target and build for `Generic iOS Device`. Select `Product => Build for testing`. This will create a `Products/Debug-iphoneos` folder in the specified project directory.  
   `Example`: **/Users/<username>/Library/Developer/Xcode/DerivedData/WebDriverAgent-dzxbpamuepiwamhdbyvyfkbecyer/Build/Products/Debug-iphoneos**
- Navigate to the folder above and create an empty directory with the name `Payload`.
- Copy the `.app` bundle inside the `Payload` folder
- Compress the `Payload` directory into an archive (.zip file) and give it a new name with `.ipa` appended to the end of the file name.
- **NB** iOS 17-17.3 Windows/Linux WebDriverAgent additional step
  - Open the `.app` bundle, navigate to `Frameworks` and delete the `XC*.framework` folders before moving it to `Payload`
  - IPA has to be re-signed after that once again uzing any applicable tool

## Device Notes

### iOS Phones
#### Enable Developer mode - iOS 16+ only
Developer mode needs to be enabled on iOS 16+ devices to allow `go-ios` usage against the device
- Open `Settings > Privacy & Security > Developer Mode`
- Enable the toggle

#### Supervise devices
This is an optional but a preferable step - it can make devices setup more autonomous - it can allow trusted pairing with devices without interacting with Trust popup  
**NB** You need a Mac machine to do this!

- Install Apple Configurator 2 on your Mac.
- Attach your first device.
- Set it up for supervision using a new (or existing) supervision identity. You can do that for free without having a paid MDM account.
- Connect each consecutive device and supervise it using the same supervision identity.
- Export your supervision identity file and choose a password.
- Save your new supervision identity as `*.p12` file.
- Log in to the hub with admin and upload the `*.p12` file via `Admin > Files`.

**NB** Make sure to remember the supervision password, you need to set it up in the provider config for each provider that will use a supervision profile.  
**NB** Provider will fall back to manual pairing if something is wrong with the supervision profile, provided password or supervised pairing.  
**NB** You can skip supervising the devices and you should trust manually on first pair attempt by the provider but if devices are supervised in advance setup can be more autonomous.

### Android Phones
#### USB Debugging
* On each device activate `Developer options`, open them and enable `Enable USB debugging`
* Connect each device to the host - a popup will appear on the device to pair - allow it.

#### Android WebRTC video - EXPERIMENTAL
GADS has experimental WebRTC video streaming for Android that can be used instead of MJPEG. The quality can be lower because it is controlled by WebRTC itself but it can potentially work better on external networks with lower bandwidth consumption.

##### WebRTC device setup
* Go to `Admin > Devices` in the hub UI
* Set `Use WebRTC video?` to `true` for the target device
* Select a preferred video [codec](#webrtc-video-codecs)

##### WebRTC video codecs
Many Android phones support hardware encoding for H264/VP8.  
Some devices like Huawei do not - for them software encoding is enforced.  
You can test the performance and select H264, VP8 or VP9 per device to achieve the best quality and performance of the video stream.  
Note that it is possible that on some devices it might not work at all, in this case you should disable WebRTC and use the MJPEG stream instead.  

**NB** It is complex to handle both device encoder and browser decoder limitations, I would suggest using Chrome/Safari, but I assume that most of the time also Firefox should manage.  
**NB** WebRTC video has some initial delay/latency while calculating the bitrate and connection capabilities when you access the device control.  

## Starting a provider instance
- Execute `./GADS provider` providing the following flags:  
  - `--nickname=` - mandatory, this is used to get the correct provider configuration from MongoDB
  - `--mongo-db=` - optional, IP address and port of the MongoDB instance (default is `localhost:27017`)
  - `--provider-folder=` - optional, folder where provider should store logs and apps and other needed files. Can be relative path to the folder where provider binary is located or full path on the host - `./test`, `.`, `./test/test1`, `/Users/shamanec/Desktop/test` are all valid. Default is the folder where the binary is currently located - `.`
  - `--log-level=` - optional, how verbose should the provider logs be (default is `info`, use `debug` for more log output)
  - `--hub=` - mandatory, the address of the hub instance so the provider can push data to it automatically, e.g `http://192.168.68.109:10000`

## Logging
Provider logs both to local files and to MongoDB.
Provider logs can be found in the `provider.log` file in the used provider folder - default or provided by the `--provider-folder` flag.  
They will also be stored in MongoDB in DB `logs` and collection corresponding to the provider nickname.

## Device logs
On start a log folder and file is created for each device relative to the used provider folder - default or provided by the `--provider-folder` flag.  
They will also be stored in MongoDB in DB `logs` and collection corresponding to the device UDID.

### SDB - Tizen Only
`sdb` (Smart Development Bridge) is mandatory when providing Tizen TV devices. You can skip installing it if no Tizen devices will be provided.
- Download and install [Tizen Studio CLI](https://developer.tizen.org/development/tizen-studio/download)
- Set up environment variables:
  ```bash
  # Add to your ~/.bashrc or equivalent
  export TIZEN_HOME=/path/to/tizen-studio
  export PATH=${PATH}:${TIZEN_HOME}/tools:${TIZEN_HOME}/tools/ide/bin
  ```
- Ensure `sdb` is available in PATH by running `sdb version` in terminal
- Restart your terminal or run `source ~/.bashrc` to apply changes

**Note**: Replace `/path/to/tizen-studio` with your actual Tizen Studio installation path. Common locations are:
- macOS: `/Users/<username>/tizen-studio`
- Linux: `/home/<username>/tizen-studio`
- Windows: `C:\tizen-studio`

## Tizen TV
### Developer Mode
* On each TV, navigate to Settings and enter the Apps menu
* Select the "Developer mode" option
* Enable Developer mode and enter the IP address of your development machine
* Accept any security prompts that appear
* The TV will restart to apply the changes

### Device Connection
* Ensure the TV and the Appium host machine are on the same local network
* After enabling developer mode, connect to the TV using SDB:  
  ```bash
  sdb connect <tv-ip-address>  
  ```
* Verify the connection by running:  
  ```bash
  sdb devices
  ```

* The TV should appear in the list of connected devices with status "device"
* First connection will require accepting a pairing request on the TV
* For app testing:
  - Only correctly-signed debug versions of apps can be tested
  - Apps must be built with the appropriate Tizen TV SDK certificates

### Known Limitations
* Video streaming is not available for Tizen TV devices
* Some remote control features may be limited due to TV-specific interactions
* Screen dimensions are fixed based on TV resolution

### WebOS CLI - WebOS Only
`WebOS CLI` is mandatory when providing WebOS TV devices. You can skip installing it if no WebOS devices will be provided.
- Download the [WebOS TV CLI](https://webostv.developer.lge.com/develop/tools/webos-tv-cli-installation) (v1.12.4 recommended)
- Extract the downloaded CLI archive and place the extracted contents in `${LG_WEBOS_TV_SDK_HOME}/CLI`
- Set up environment variables:
  ```bash
  # Add to your ~/.bashrc or equivalent
  export LG_WEBOS_TV_SDK_HOME=/path/to/webOS_TV_SDK
  export WEBOS_CLI_TV=${LG_WEBOS_TV_SDK_HOME}/CLI
  export PATH=${PATH}:${WEBOS_CLI_TV}/bin
  ```
- Ensure `ares` commands are available in PATH by running `ares -V` in terminal
- Restart your terminal or run `source ~/.bashrc` to apply changes

**Note**: Replace `/path/to/webOS_TV_SDK` with your actual WebOS TV SDK installation path. Common locations are:
- macOS: `/Users/<username>/webOS_TV_SDK`
- Linux: `/home/<username>/webOS_TV_SDK`
- Windows: `C:\webOS_TV_SDK`

## WebOS TV
### Developer Mode - WebOS
* Install the Developer Mode app from LG Content Store
* Sign in with your LG Developer account (create one at https://webostv.developer.lge.com if needed)
* Enable Developer Mode by clicking the Dev Mode Status button
* The TV will reboot automatically

### Device Connection
* Ensure the TV and the provider host machine are on the same network
* Add the TV as a device using the WebOS CLI:
  ```bash
  ares-setup-device --add target -i "host=10.123.45.67" -i "port=9922" -i "username=prisoner" -i "default=true"
  ```
  > **⚠️ IMPORTANT**: The device name (e.g., `target` in the example above) must be:
  > - Descriptive and meaningful for your setup
  > - **EXACTLY the same** as the device name registered in GADS
  > - If the names don't match, there will be configuration issues with the provisioned Appium server for the TV
  
  - Default port is 9922
  - Default username is "prisoner"
  - Leave password empty
* For first-time connections, you'll need to accept the pairing request on the TV
* Verify the connection by running:
  ```bash
  ares-setup-device --list
  ```
* The TV should appear in the list with its IP:PORT identifier

### Chromedriver Requirements
* WebOS TVs require Chromedriver 2.36 for compatibility
* GADS will manage the Chromedriver installation automatically
* The driver path will be configured in Appium capabilities

### Device UDID Format
* WebOS devices use the format `IP:PORT` as their UDID (e.g., `192.168.1.100:9922`)
* This UDID must be registered in the GADS database before the device can be used

### Known Limitations
* Video streaming is not available for WebOS TV devices
* Remote control features are limited compared to mobile devices
* Only web-based TV apps can be automated (native apps have limited support)
* Developer Mode has a 1000-hour time limit and needs periodic renewal

