## Android setup
### Minicap setup
1. Setup Android SDK.  
2. Download and setup Android NDK.  
3. Clone https://github.com/shamanec/minicap.git in the main project folder in a 'minicap' folder(default).  
4. Open the 'minicap' folder.  
5. Execute 'git submodule init' and 'git submodule update'.  
6. Execute 'ndk-build'.  
7. Execute 'experimental/gradlew -p experimental assembleDebug'  
8. Execute 'ndk-build NDK_DEBUG=1 1>&2'

### Register devices in config.json
1. Open the *./configs/config.json* file.  
2. For each Android device add a new object inside the *devices-list* array in the json.  
3. For each device provide:  
  * *os* - should be "android"  
  * *appium_port* - unique Appium port  
  * *stream_port* - unique minicap port for the video stream  
  * *device_os_version* - "11" for example  
  * *device_name* - avoid using special characters and spaces except '_'. Example: "Huawei_P20_Pro"  
  * *device_udid* - UDID of the Android device, can get it with *adb devices*   

### Kill adb-server
1. You need to make sure that adb-server is not running before you start devices containers.  
2. Run 'adb kill-server'.  

## iOS setup
### Dependencies
1. Install usbmuxd (from apt is sufficient)  

### Known limitations
1. It is not possible to execute **driver.executeScript("mobile: startPerfRecord")** with Appium to record application performance since Xcode tools are not available.  
2. Anything else that might need Instruments and any other Xcode/OSX exclusive tools  

### Prepare WebDriverAgent file

You need an Apple Developer account to sign and build **WebDriverAgent**

1. Download and install [iOS App Signer](https://dantheman827.github.io/ios-app-signer/)  
2. Open **WebDriverAgent.xcodeproj** in Xcode.  
3. Ensure a team is selected before building the application. To do this go to: *Targets* and select each target one at a time. There should be a field for assigning teams certificates to the target.  
4. Remove your **WebDriverAgent** folder from *DerivedData* and run *Clean build folder* (just in case)  
5. Next build the application by selecting the *WebDriverAgentRunner* target and build for *Generic iOS Device*. Run *Product => Build for testing*. This will create a *Products/Debug-iphoneos* in the specified project directory.  
 *Example*: **/Users/<username>/Library/Developer/Xcode/DerivedData/WebDriverAgent-dzxbpamuepiwamhdbyvyfkbecyer/Build/Products/Debug-iphoneos**  
6. Open **iOS App Signer**  
7. Select **WebDriverAgentRunner-Runner.app**.  
8. Generate the WDA *.ipa file.  

**or zip it manually into an IPA yourself, I had some issues last time I did it :(**

### Provide the WebDriverAgent ipa  
1. Paste your WDA ipa in the **./apps** folder with name **WebDriverAgent.ipa** (exact name is important for the scripts)  

### Supervise the iOS devices  
1. Install Apple Configurator 2 on your Mac.  
2. Attach your first device.  
3. Set it up for supervision using a new(or existing) supervision identity. You can do that for free without having a paid MDM account.  
4. Connect each consecutive device and supervise it using the same supervision identity.  
5. Export your supervision identity file and choose a password.  
6. Save your new supervision identity file in the project *./configs* (or other) folder as *supervision.p12*.  

**Note** I'll see if I can add the option to manually accept devices pairing when no supervision is provided but it doesn't make sense in the context of self-sustaining device farm. You can try running it without providing supervision files, might just popup the manual accept alert.  

### Register your devices for the project
1. Open the *./configs/config.json* file.  
2. For each iOS device add a new object inside the *ios-devices-list* array in the json.  
3. For each device provide:  
  * *os* - should be "ios"  
  * *appium_port* - unique appium port  
  * *wda_mjpeg_port* - unique WDA Mjpeg stream port  
  * *wda_port* - unique WDA instance port  
  * *device_udid* - UDID of the iOS device, can get it with *go-ios* for example  
  * *device_os_version* - "15.2" for example  
  * *device_name* - avoid using special characters and spaces except '_'. Example: "Huawei_P20_Pro"  
  * *viewport_size* - this is needed to easily work with the stream and remote control. Example: "375x667". You can get it on https://whatismyviewport.com (ScreenSize: at the bottom)  