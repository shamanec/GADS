## Introduction

* GADS is a web UI for remote control of devices provisioned by [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider).

**NB** New React based UI - work in progress

## Features
1. Authentication  
  a. Log in, session expiry  
  b. Add users (for admins)  
2. Devices control (most of interaction is wrapped around Appium API)
  * Android
    - [GADS-Android-stream](https://github.com/shamanec/GADS-Android-stream) video stream  
  * iOS
    - [WebDriverAgent](https://github.com/appium/WebDriverAgent) video stream   
  * Both
    - Basic functionalities - Home, Lock, Unlock, Type text
    - Basic remote control - tap, swipe, touch&hold
    - Take high quality screenshots
    - Reservation - loading a device sets it `In use` and can't be used by another person until it is released
    - Appium session refresh mechanism if a session timed out or was closed

Developed and tested on Ubuntu 18.04 LTS, Windows 10, macOS Ventura 13.5.1  

## TODO
* Basic browser Appium inspector
* Provider and devices log display
* Extend features - better administration, more control options - e.g. simulate location

## Setup
Currently the project assumes that GADS UI, MongoDB, Selenium Grid and device providers are on the same network. They can all be on the same machine as well.  

### Deps
1. Install Go version 1.21.0 or higher
2. Install Node > 16.

#### Install Docker 
1. You need to have Docker(Docker Desktop on macOS, Windows) installed.  

#### Start a MongoDB container instance
1. Execute `docker run -d --restart=always --name mongodb -p 27017:27017 mongo:6.0`. This will pull the official MongoDB 6.0 image from Docker Hub and start a container binding ports `27017` for the MongoDB instance.  
2. You can use MongoDB Compass or another tool to access the db if needed.

### Setup the GADS UI
Clone the project and the [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider) code from both repos.

#### Setup config.json
1. Open the `config.json` file.  
2. Change the `gads_host_address` value to the IP of the host machine.  
3. Change the `gads_port` value to the port you wish the service to run on - default is 10000.  
4. Change the `mongo_db` value to the IP address and port of the MongoDB instance. Example: `192.168.1.2:32771` 
5. ~~If you are registering devices to Selenium Grid via providers, you can visualize the grid directly in the UI - change `selenium_grid_instance` to the instance url, e.g. `http://192.168.1.6:4444` and access it via the UI~~

**NB** You can change the default admin username and password values in `config.json` if you wish.  

#### Build the UI
1. Open the `gads-ui` folder in Terminal.
2. Execute `npm install`
3. Execute `npm run build`

#### Start the UI and backend service
1. Open terminal and execute `go build .` in the main project folder  
2. Execute `./GADS --auth=true`  
3. Access the UI on `http://{gads_host_address}:{gads_port}`

#### UI development
If you want to work on the UI you need to add a proxy in `package.json` to point to the Go backend 
1. Open the `gads-ui` folder.
2. Open the `package-json` file.
3. Add a new field `"proxy": "http://192.168.1.28:10000/"` providing the host and port of the Go backend service.
4. Run `npm start`

#### Start a provider instance
This is only the UI, to actually have devices available you need to have at least one [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider) instance running on the same host(or another host on the same network) that will actually set up and provision the devices. Follow the setup steps in the linked repository to create a provider instance.

## Thanks

| |About|
|---|---| 
|[Appium](https://github.com/appium)|It would be impossible to control the devices remotely without Appium for the control and WebDriverAgent for the iOS screen stream, kudos!|  

## Demo video  
https://github.com/shamanec/GADS/assets/60219580/3142fb7b-74a6-49bd-83c9-7e8512dee5fc



