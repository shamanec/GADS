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
- [Devices Notes](#ios-phones)
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
-  **Prepare** [WebDriverAgent](#prepare-webdriveragent-on-macos).
-  (Optional) **Supervise** [your iOS devices](#supervise-devices).

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
- **Prepare** [WebDriverAgent](#prepare-webdriveragent-file---linux-windows).
- (Optional) **Supervise** [your iOS devices](#supervise-devices).

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
- **Prepare** [WebDriverAgent](#prepare-webdriveragent-file---linux-windows).
- (Optional) **Supervise** [your iOS devices](#supervise-devices).

#### ⚠️ Known Limitations - Windows, iOS
1. The command **driver.executeScript("mobile: startPerfRecord")** cannot be executed due to the unavailability of Xcode tools.
2. Any functionality requiring Instruments or other Xcode/macOS-exclusive tools is limited.



## Dependencies notes
### Appium
Appium is foundational in GADS - we use it both to create Appium servers to run UI tests against, but also to allow the interactions in the web remote control.  
Installation is pretty similar for all operating systems, you just have to find the proper steps for your setup.
- Install Node > 16
- Install Appium with `npm install -g appium`
- Install Appium drivers
  - iOS - `appium driver install xcuitest`
  - Android - `appium driver install uiautomator2`
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



## iOS Phones
### Enable Developer mode - iOS 16+ only
Developer mode needs to be enabled on iOS 16+ devices to allow `go-ios` usage against the device
- Open `Settings > Privacy & Security > Developer Mode`
- Enable the toggle

### Supervise devices
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

## Android
#### USB Debugging
* On each device activate `Developer options`, open them and enable `Enable USB debugging`
* Connect each device to the host - a popup will appear on the device to pair - allow it.

## Prepare WebDriverAgent - (read the full paragraph)
### WebDriverAgent ipa
You need to prepare and upload a signed `WebDriverAgent` ipa file from the hub UI in `Admin > Files`
By using my custom WebDriverAgent you can have faster tap/swipe interactions on iOS devices (the provider configuration should be set to use the custom WebDriverAgent through GADS UI).  
You can use mainstream WebDriverAgent as well  
  
### Prebuilt custom WebDriverAgent
- Download the prebuilt `WebDriverAgent.ipa` from my fork of [WebDriverAgent](https://github.com/shamanec/WebDriverAgent)
- Use any tool to resign it with your developer account (or provisioning profile + certificate)
  - [zsign](https://github.com/zhlynn/zsign)
  - [fastlane-sigh](https://docs.fastlane.tools/actions/sigh/)
  - [codesign](https://developer.apple.com/library/archive/documentation/Security/Conceptual/CodeSigningGuide/Procedures/Procedures.html)

### Prebuilt mainstream WebDriverAgent
- Download the latest prebuilt `WebDriverAgentRunner-Runner.zip` from the [WebDriverAgent](https://github.com/appium/WebDriverAgent/releases) releases
- Unzip the `.app` bundle in any folder.
- Navigate to the folder above and create an empty directory with the name `Payload`.
- Copy the `.app` bundle inside the `Payload` folder
- Compress the `Payload` directory into an archive (.zip file) and give it a new name with `.ipa` appended to the end of the file name.
- Use any tool to resign it with your developer account (or provisioning profile + certificate)
  - [zsign](https://github.com/zhlynn/zsign)
  - [fastlane-sigh](https://docs.fastlane.tools/actions/sigh/)
  - [codesign](https://developer.apple.com/library/archive/documentation/Security/Conceptual/CodeSigningGuide/Procedures/Procedures.html)

### Build WebDriverAgent IPA file manually using Xcode
- Download the code of the latest mainstream [WebDriverAgent](https://github.com/appium/WebDriverAgent/releases) release or alternatively the code from the `main` branch of my fork of [WebDriverAgent](https://github.com/shamanec/WebDriverAgent) for faster tap/swipe interactions.
- Open `WebDriverAgent.xcodeproj` in Xcode.
- Select signing profile for WebDriverAgentRunner. To do this go to: *Targets*, select WebDriverAgentRunner. There should be a field for assigning teams certificates to the target.
- Select `Build > Clean build folder` (just in case)
- Next build the application by selecting the `WebDriverAgentRunner` target and build for `Generic iOS Device`. Select `Product => Build for testing`. This will create a `Products/Debug-iphoneos` folder in the specified project directory.  
   `Example`: **/Users/<username>/Library/Developer/Xcode/DerivedData/WebDriverAgent-dzxbpamuepiwamhdbyvyfkbecyer/Build/Products/Debug-iphoneos**
- Navigate to the folder above and create an empty directory with the name `Payload`.
- Copy the `WebDriverAgentRunner-Runner.app` bundle/file inside the `Payload` folder
- Compress the `Payload` directory into an archive (.zip file) and give it a new name with `.ipa` appended to the end of the file name.

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

