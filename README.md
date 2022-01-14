## Introduction

<img src="https://drive.google.com/uc?export=view&id=1itoR-rv2pbR4gsOW6WmyhzpRocNszmsc" width="50%" height="50%">

* GADS or Go Appium Docker Service is a small webserver that allows you to configure and monitor Appium docker containers and essentially create your own device farm for Appium test execution.  
* For the moment the service has only iOS containers integrated. The project uses [go-ios](https://github.com/danielpaulus/go-ios) to pair devices, install and run WebDriverAgent  
* Right now no connection to Selenium Grid is made after starting Appium for a device - TODO  
* UI is simple but I am trying to make it intuitive so you can easily control most of the project config via the browser  
* Containers are automatically controlled and sustained - they are updated when a registered device is connected/disconnected, they also restart after host reboot which combined with the usbmuxd.service lets them keep working when a host failure occurs without needing intervention.  
* Appium server inside containers is also self-healing(kinda). WDA server is polled and restarted in case it fails making test execution (hopefully) for iOS devices pretty reliable.  
* **NB** This is my first attempt at Go and web dev in general so a lot of the code is probably messy as hell. I will be doing my best to cleanup and improve all the time but for now this is just a working POC.  
**NB** I've been doing this having only 2 iOS devices available. It looks like everything is pretty much working but I do not know how it would behave on a bigger scale.  

### Known limitations
1. It is not possible to execute **driver.executeScript("mobile: startPerfRecord")** to record application performance since Xcode tools are not available.  

## Dependencies  
The project has minimum dependencies:  
1. Install Docker.  
2. Install usbmuxd (from apt is sufficient)  
3. Install Go 1.17 (that is what I'm using, lower might also work)  

Developed and tested on Ubuntu 18.04 LTS  

## Prepare WebDriverAgent file

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

**Note** Donate to [iOS App Signer](https://dantheman827.github.io/ios-app-signer/) if you use it and like it.  
**Note2** You can use some other approach to generate the *.ipa file, this is what easily worked for me.  

## Supervise the iOS devices  
1. Install Apple Configurator 2 on your Mac.  
2. Attach your first device.  
3. Set it up for supervision using a new(or existing) supervision identity. You can do that for free without having a paid MDM account.  
4. Connect each consecutive device and supervise it using the same supervision identity.  
5. Export your supervision identity file and choose a password.  
6. Save your new supervision identity file in the project *./configs* (or other) folder.  

**Note** I'll see if I can add the option to manually accept devices pairing when no supervision is provided but it doesn't make sense in the context of self-sustaining device farm.  

## Update the environment file  
1. Set your sudo password - it is used by the commands that apply the systemd usbmuxd.service and the udev rules. It is used only locally so there should be no security risk unless you publicly commit it.  
2. Set the project dir - I'll rework it to identify it automatically but for now this is the implementation.  
3. Set Selenium Grid connection - true or false. True attempts to connect each Appium server to the Selenium Grid instance defined in *./configs/config.json*  
4. Set your supervision identity file path and the supervision identity password(same applies as step 1). The project assumes you are supervising your devices so that everything could happen automatically.  

## Run the project  
1. Clone the project.  
2. Cd into 'GADS' folder.  
3. Execute 'go run main.go'  
4. Open your browser and go to *localhost:10000*.  

You can access Swagger documentation on *http://localhost:10000/swagger/index.html*  

## Setup  
### Build Docker image
1. Open the Project Config page and select "Build image".
2. It doesn't report the progress dynamically so you need to refresh the config page until it is reported as available.  

### Register your devices  
1. Open the Project Config page.  
2. Tap on "Add device" button.  
3. Select a device from the dropdown.  
4. Provide a device name.  
5. Register it.  
6. Do that for all used devices.  

This will add a new object in the *./configs/config.json* file with the device name and UDID and will auto increment all used ports for the Appium connection for you.  

### Setup the usbmuxd.service and udev listener  
1. Open the Project Config page.  
2. Tap on "Setup listener" - you need to have your sudo password set up in the *./env.json* file.  

This will move *./configs/usbmuxd.service* to */lib/systemd/system* and enable the service - this starts usbmuxd automatically after reboot. It will also create and set udev rules in */etc/udev/rules.d* that will trigger the container updates when iOS device is connected/disconnected from the machine.  

### Update the project config  
1. Open the Project Config page.  
2. Tap on "Change config".  
3. Update your Selenium Grid values and the bundle ID of the used WebDriverAgent.  

### Provide the WebDriverAgent ipa  
1. Open the Project Config page.
2. Click on **Upload WDA**.
3. Select the ipa you created in step 7 and submit it.
4. The file will be uploaded in the **./ipa** folder and named **WebDriverAgent.ipa** making it ready to be used by the iOS containers.  

### Spin up containers  
1. Open the Project Config page.  
2. Tap on "Update iOS containers" button.  
3. Go to "iOS Containers" page - you should see a container created for each device registered in *./configs/config.json*.  
4. You can observe different logs for each device container for debugging purposes.  

### Upload application files to be used by Appium  
1. Open the Project Config page.  
2. Tap on "Upload app file" button.  
3. Select the AUT *.ipa file and submit it.  
4. It will be uploaded in the *./ipa* folder which is mounted to each container and then you can access this app and install it using Appium.  

**NB** For a way to perform most of these actions without the UI you can refer to the Swagger documentation.  

WORK IN PROGRESS

## Thanks

| |About|
|---|---|
|[go-ios](https://github.com/danielpaulus/go-ios)|Many thanks for creating this tool to communicate with iOS devices on Linux, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode|
|[iOS App Signer](https://github.com/DanTheMan827/ios-app-signer)|This is an app for OS X that can (re)sign apps and bundle them into ipa files that are ready to be installed on an iOS device.|

