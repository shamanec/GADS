## config.json

Initially there were two separate *.txt files for devices info and environment variables where the data was provided line by line. Switching to json data made the scripts a bit more complex but it allowed me to combine the devices info and environment variables into a single file and in my opinion it looks cooler and neater this way. The **config.json** consists of a single object that contains the key-value pairs for the environment variables and a single array named 'devicesList' which contains the vars for each device. Everything in the file is pretty much self-explanatory but I'll add provide some info regardless:

1. Environment
 * *selenium_hub_host*, *selenium_hub_port*, *selenium_hub_protocol_type* and *devices_host* can be left as is if you are not going to connect to Selenium Grid.
 * *selenium_hub_host* should be the IP address of the Selenium Grid if you are connecting to one.
 * *selenium_hub_port* should be the port of the Selenium Grid if you are connecting to one.
 * *selenium_hub_protocol_type* should be 'http' or 'https' if you are connecting to Selenium Grid depending on your setup.
 * *devices_host* should be the IP address of the current machine that will provide the devices if you are connecting to Selenium Grid.
 * *wda_bundle_id* should be the bundle ID of the WebDriverAgent.ipa that you built or can be left as is if you are trying to use mine.

2. Devices
 * *appium_port* is of type number and should be the port on which you want the particular device to register Appium server.
 * *device_name* is of type string and can be anything but I would avoid using spaces and special characters if possible. You can stick to the examples like 'iPhone_11'
 * *device_os_version* is of type string and should be the respective device OS version.
 * *device_udid* is of type string and should be the respective device UDID. You can get them by executing **./ios list --details** in the main project folder.
 * *wda_mjpeg_port* is of type number and should be the port on which you want to get a stream off WDA - I am not actually using it but could be useful for someone.
 * *wda_port* is of type number and should be the port on which you want the WDA to listen on - it is used by the Appium server to connect to the specific *webDriverAgentUrl* since we are not doing the building WDA dynamically as on OSX systems.

All looks straightforward and you should not have issues with updating the file but for ease of use you can do it via the main script. You can update the environment vars by executing **./services.sh control** and selecting option **4) Setup environment vars**. You can add more devices (that are connected to the machine) by executing **./services.sh control** and selecting option **9) Add a device**.

## wdaSync.sh

 * This is the cornerstone of keeping the WebDriverAgent up and running on the device as long as possible or in an ideal scenario - indefinitely as long as the device is working and connected to the machine.
 * Please refer to the diagram below:  
DIAGRAM TO BE UPDATED BECAUSE IT WAS REMOVED FROM HOSTING SERVICE, REALLY SORRY ABOUT THAT :D   

 * The script uses [go-ios](https://github.com/danielpaulus/go-ios) to install and run the WebDriverAgent
 * The script also uses the *go-ios* to mount the Developer Disk Images to the device - you should already have them prepared as described in the main project Readme.md
 * The script checks if WDA is up and running by calling **curl -Is "http:$deviceIP:$WDA_PORT/status"**
 * The script checks if Appium is up and running by calling **curl -Is "http://127.0.0.1:${APPIUM_PORT}/wd/hub/status"**
 * Appium is launched using the *webDriverAgentUrl* capability to connect to the already installed and started WDA agent instead of attempting to install it which obviously will not work without Xcode :D
 * Appium is launched with extended *wdaLaunchTimeout* and *wdaConnectionTimeout* capabilities to give the script time to 'restart' WDA in case it crashes and it's no longer available - this in theory should allow for continious test execution without failing tests if the WDA crashes.
