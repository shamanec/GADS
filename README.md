## Introduction

* GADS or Go Appium Docker Service is a web UI for [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider) orchestration and remote control of devices.  

## Features
1. Provider logs for debugging  
2. Devices remote control(most of which is wrapper around Appium)
  * Android
    - `minicap` video stream - default option
    - `GADS-Android-stream` video stream - not as good as `minicap` but it is inhouse, can be used in case `minicap` fails for device    
    - basic device interaction - Home, Lock, Unlock, Type text, Clear text  
    - basic remote control - tap, swipe  
    - basic Appium inspector
  * iOS
    - `WDA mjpeg` video stream  
    - basic device interaction - Home, Lock, Unlock, Type text, Clear text  
    - basic remote control - tap, swipe  
    - basic Appium inspector  

3. TODO - simple provider container info and orchestration page  
4. TODO - more functionality for remote control  

Developed and tested on Ubuntu 18.04 LTS  

## Setup
Currently the project assumes that GADS UI, RethinkDB and device providers are on the same network. They can all be on the same machine as well.  

### Start RethinkDB instance
The project uses RethinkDB for syncing devices availability between providers and GADS UI.  
1. Execute `docker run -d --restart always --name gads-rethink -p 32770:8080 -p 32771:28015 rethinkdb:2.4.2`. This will pull the official RethinkDB 2.4.2 image from Docker Hub and start a container binding ports `32770` for the RethinkDB dashboard and `32771` for db connection.  
2. Open the RethinkDB dashboard on `http://localhost:32770/`  
3. Go to `Tables` and create a new database named `gads`  
4. Add a new table to `gads` database named `devices` with primary key `UDID` (you need to click `Show optional settings` for the primary key)  

### Setup config.json
1. Open the `config.json` file.  
2. Change the `gads_host_address` value to the IP of the host machine.  
3. Add the IP addresses of the device providers in the `device_providers` array.  
4. Change the `rethink_db` value to the IP address and port of the RethinkDB instance. Example: `192.168.1.2:32771`  

### Start the GADS UI
1. Execute `go build .`  in the main project folder  
2. Execute `./GADS`  
3. Access the UI on `http://localhost:10000`  

## Thanks

| |About|
|---|---|
|[go-ios](https://github.com/danielpaulus/go-ios)|Many thanks for creating this tool to communicate with iOS devices on Linux, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode. Without it none of this would be possible|
|[iOS App Signer](https://github.com/DanTheMan827/ios-app-signer)|This is an app for OS X that can (re)sign apps and bundle them into ipa files that are ready to be installed on an iOS device.|
|[minicap](https://github.com/DeviceFarmer/minicap)|Stream screen capture data out of Android devices|  
|[Appium](https://github.com/appium)|It would be impossible to control the devices remotely without Appium for the control and WebDriverAgent for the iOS screen stream, kudos!|  

## WIP demo video  

https://user-images.githubusercontent.com/60219580/183677067-237c12d9-f06d-4b14-985c-17aedbb19ea6.mp4




