## Introduction

* GADS is a web UI for [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider) orchestration and remote control of devices.  

## Features
1. Provider logs for debugging  
2. Devices remote control(most of which is wrapper around Appium)
  * Android
    - `GADS-Android-stream` video stream   
    - basic device interaction - Home, Lock, Unlock, Type text, Clear text  
    - basic remote control - tap, swipe  
    - basic Appium inspector
  * iOS
    - `WebDriverAgent MJPEG` video stream  
    - basic device interaction - Home, Lock, Unlock, Type text, Clear text  
    - basic remote control - tap, swipe  
    - basic Appium inspector  

3. TODO - simple provider container info and orchestration page  
4. TODO - more functionality for remote control  

Developed and tested on Ubuntu 18.04 LTS, Windows 10, macOS Ventura 13.5.1  

## Setup
Currently the project assumes that GADS UI, MongoDB and device providers are on the same network. They can all be on the same machine as well.  

### Go
1. Install Go version 1.21.0 or higher

### Start MongoDB instance
The project uses MongoDB for syncing devices info between providers and GADS UI.  
1. Execute `docker run -d --restart-always --name mongodb -p 27017:27017 mongo:6.0`. This will pull the official MongoDB 6.0 image from Docker Hub and start a container binding ports `27017` for the MongoDB instance.  
2. You can use MongoDB Compass or another tool to access the db.

### Setup config.json
1. Open the `config.json` file.  
2. Change the `gads_host_address` value to the IP of the host machine.  
3. Change the `gads_port` value to the port you wish the service to run on - default is 10000.  
4. Change the `mongo_db` value to the IP address and port of the MongoDB instance. Example: `192.168.1.2:32771`  

### Start the GADS UI
1. Execute `go build .`  in the main project folder  
2. Execute `./GADS`  
3. Access the UI on `http://{gads_host_address}:{gads_port}`

### Start a provider instance
This is only a UI, to actually have devices available you need to have at least one [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider) instance running on the same host(or another host on the same network) that will actually set up and provide the devices. Follow the setup steps in the linked repo to create a provider instance.

## Thanks

| |About|
|---|---|
|[go-ios](https://github.com/danielpaulus/go-ios)|Many thanks for creating this tool to communicate with iOS devices on Linux, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode. Without it none of this would be possible|  
|[iOS App Signer](https://github.com/DanTheMan827/ios-app-signer)|This is an app for OS X that can (re)sign apps and bundle them into ipa files that are ready to be installed on an iOS device.|  
|[Appium](https://github.com/appium)|It would be impossible to control the devices remotely without Appium for the control and WebDriverAgent for the iOS screen stream, kudos!|  

## WIP demo video  

https://user-images.githubusercontent.com/60219580/183677067-237c12d9-f06d-4b14-985c-17aedbb19ea6.mp4




