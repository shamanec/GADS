GADS - Go Appium Docker Service

## Introduction

* GADS or Go Appium Docker Service is a small webserver that allows you to configure and monitor Appium docker containers.  
* For the moment the service has only iOS containers integrated.  
* I will be attempting to have all services implemented using only Go. Right now its mostly like this but inside the containers shell scripts are used.  
* Right now no connection to Selenium Grid is made after starting Appium for a device - TODO  
* I will attempt to provide capability to do everything via UI and also REST calls  
* **NB** This is my first attempt at Go and web dev in general so a lot of the code is probably messy as hell. I will be doing my best to cleanup and improve all the time but for now this is just a working POC.  

1. Execute 'go run main.go'
2. Access on *localhost:10000*.

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
