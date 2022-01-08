GADS - Go Appium Docker Service

<img src="https://drive.google.com/uc?export=view&id=1itoR-rv2pbR4gsOW6WmyhzpRocNszmsc" width="50%" height="50%">

## Introduction

* GADS or Go Appium Docker Service is a small webserver that allows you to configure and monitor Appium docker containers.  
* For the moment the service has only iOS containers integrated. The project uses [go-ios](https://github.com/danielpaulus/go-ios) to install and run WebDriverAgent  
* Right now no connection to Selenium Grid is made after starting Appium for a device - TODO  
* I will attempt to provide capability to do everything via UI and also REST  
* UI is simple but I am trying to make it intuitive so you can easily control most of the project config via the browser  
* **NB** This is my first attempt at Go and web dev in general so a lot of the code is probably messy as hell. I will be doing my best to cleanup and improve all the time but for now this is just a working POC.  

## Dependencies  
The project has minimum dependencies:  
1. Install Docker.  
2. Install usbmuxd (from apt is sufficient)  
3. Install Go 1.17 (that is what I'm using, lower might also work)  


## Run the project  
1. Clone the project.
2. Cd into 'GADS' folder.
3. Execute 'go run main.go'
4. Open your browser and go to *localhost:10000*.

## Prepare WebDriverAgent file

You need an Apple Developer account to sign and build **WebDriverAgent**

1. Open **WebDriverAgent.xcodeproj** in Xcode.
2. Ensure a team is selected before building the application. To do this go to: *Targets* and select each target one at a time. There should be a field for assigning teams certificates to the target.
3. Remove your **WebDriverAgent** folder from *DerivedData* and run *Clean build folder* (just in case)
4. Next build the application by selecting the *WebDriverAgentRunner* target and build for *Generic iOS Device*. Run *Product => Build for testing*. This will create a *Products/Debug-iphoneos* in the specified project directory.  
 *Example*: **/Users/<username>/Library/Developer/Xcode/DerivedData/WebDriverAgent-dzxbpamuepiwamhdbyvyfkbecyer/Build/Products/Debug-iphoneos**
5. Open **WebDriverAgentRunner-Runner.app**.
6. Zip all the files inside.
7. Open the **Configuration** page in the UI.
8. Click on **Upload WDA**.
9. Select the zip you created in step 6 and submit it.
10. **WebDriverAgent** folder will be created inside the main project folder and the file will be unzipped inside. This folder will be mounted to iOS containers and used to install WebDriverAgent on the devices.

WORK IN PROGRESS
