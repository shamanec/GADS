## Introduction

* GADS is a web UI for remote control of devices provisioned by [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider).  

## Features
1. Provider logs for debugging  
2. Devices control (most of interaction is wrapped around Appium API)
  * Android
    - [GADS-Android-stream](https://github.com/shamanec/GADS-Android-stream) video stream  
  * iOS
    - [WebDriverAgent](https://github.com/appium/WebDriverAgent) video stream   
  * Both
    - Basic functionalities - Home, Lock, Unlock, Type text, Clear text  
    - Basic remote control - tap, swipe  
    - Basic web Appium inspector - see elements tree with info only
    - Take high quality screenshots
    - Simple logs display - Appium/WebDriverAgent logs when provider is in `debug`, some simple interaction logs
    - Reservation - loading a device sets it `In use` and can't be used by another person until it is released
    - Appium session refresh mechanism if a session timed out or was closed

Developed and tested on Ubuntu 18.04 LTS, Windows 10, macOS Ventura 13.5.1  

## Setup
Currently the project assumes that GADS UI, MongoDB and device providers are on the same network. They can all be on the same machine as well.  

### Go
1. Install Go version 1.21.0 or higher

### Start MongoDB instance - this can be done as provider step as well
The project uses MongoDB for syncing devices info between providers and GADS UI.  

#### Install Docker 
1. You need to have Docker(Docker Desktop on macOS, Windows) installed.  

#### Start a MongoDB container instance
1. Execute `docker run -d --restart=always --name mongodb -p 27017:27017 mongo:6.0`. This will pull the official MongoDB 6.0 image from Docker Hub and start a container binding ports `27017` for the MongoDB instance.  
2. You can use MongoDB Compass or another tool to access the db if needed.

### Setup the GADS UI
Download the latest release and the appropriate [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider) release or clone the latest main code from both repos.

#### Setup config.json
1. Open the `config.json` file.  
2. Change the `gads_host_address` value to the IP of the host machine.  
3. Change the `gads_port` value to the port you wish the service to run on - default is 10000.  
4. Change the `mongo_db` value to the IP address and port of the MongoDB instance. Example: `192.168.1.2:32771`  

#### Start the GADS UI
1. Open terminal and execute `go build .` in the main project folder  
2. Execute `./GADS`  
3. Access the UI on `http://{gads_host_address}:{gads_port}`

#### Start a provider instance
This is only the UI, to actually have devices available you need to have at least one [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider) instance running on the same host(or another host on the same network) that will actually set up and provision the devices. Follow the setup steps in the linked repository to create a provider instance.

## Thanks

| |About|
|---|---| 
|[Appium](https://github.com/appium)|It would be impossible to control the devices remotely without Appium for the control and WebDriverAgent for the iOS screen stream, kudos!|  

## Demo video  
iOS

https://github.com/shamanec/GADS/assets/60219580/a97a3d2c-ddfd-4930-bed3-67061a07b2b8

Android  

https://github.com/shamanec/GADS/assets/60219580/b3aec708-1630-489e-b1a3-9e345de051ad


