## Minicap setup
1. Setup Android SDK.  
2. Download and setup Android NDK.  
3. Clone https://github.com/shamanec/minicap.git in the main project folder in a 'minicap' folder(default).  
4. Open the 'minicap' folder.  
5. Execute 'git submodule init' and 'git submodule update'.  
6. Execute 'ndk-build'.  
7. Execute 'experimental/gradlew -p experimental assembleDebug'  
8. Execute 'ndk-build NDK_DEBUG=1 1>&2'

## Register devices in config.json
1. Open the *./configs/config.json* file.  
2. For each Android device add a new object inside the *android-devices-list* array in the json.  
3. For each device provide:  
  * unique Appium port  
  * unique minicap stream port  
  * device OS version  
  * device name - avoid using special characters and spaces except '_'. Example: "Huawei_P20_Pro"  
  * device UDID  

## Kill adb-server
1. You need to make sure that adb-server is not running before you start devices containers.  
2. Run 'adb kill-server'.  
