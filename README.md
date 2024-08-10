- [Intro](#intro)  
- [Features](#features)  
  - [Hub](#hub-features)
  - [Provider](#provider-features)
- [Setup](#setup) 
  - [Common setup](#common-setup)
    - [MongoDB](#mongodb)
  - [Hub setup](./docs/hub.md)
  - [Provider setup](./docs/provider.md)
- [FAQ](./docs/faq.md)
- [Thanks](#thanks)
- [Demo video](#demo-video)

[![](https://dcbadge.vercel.app/api/server/5amWvknKQd)](https://discord.gg/5amWvknKQd)

<img src="/docs/gads-logo.png" width="256"/>

## Intro
GADS is an application for remote control and Appium test execution on mobile devices  

The app consists of two main components  - `hub` and `provider`  
The role of the `hub` is to serve a web interface for the remote control of devices and provider management, as well as act as proxy for providers.  
The role of the `provider` is to set up and provide the mobile devices for remote control/testing  
Supports both Android and iOS devices  
Supports Linux, macOS and Windows - notes below

## Features
### Hub features
- Web interface
  - Authentication
    - Login, session expiry
    - Add users (for admins)
  - Devices control (most of interaction is wrapped around Appium APIs)
    - Live video
      - **NB** Videos are essentially MJPEG streams so they are very bandwidth hungry
    - Basic remote control - tap, swipe, touch&hold, home, lock, unlock, type text to active element, get clipboard
    - Install/Uninstall apps
    - Take high quality screenshots
    - Reservation - loading a device sets it `In use` and can't be used by another person until it is released
- Backend
  - Serving the web interface
  - Proxy the communication to the provider instances
  - Experimental Appium grid replacement for Selenium Grid
    - Integrated with UI to reserve devices currently running Appium tests

### Provider features
- Straightforward dependencies setup
- Devices administration via the hub UI
- Automatic provisioning when registered devices are connected
  - Dependencies automatically installed on devices 
  - Appium server set up and started for each device
- Remote control APIs for the hub
  - iOS MJPEG video stream using [WebDriverAgent](https://github.com/appium/WebDriverAgent)
  - Android MJPEG video stream using [GADS-Android-stream](https://github.com/shamanec/GADS-Android-stream)
  - Interaction wrapped around Appium - tap, swipe, touch&hold, type text, lock and unlock device, get clipboard
- Appium test execution - each device has its Appium server proxied on a provider endpoint for easier access
- Optionally Selenium Grid 4 nodes can be registered for each device Appium server
- macOS
  - Supports both Android / iOS
- Linux
  - Supports both Android / iOS < 17 && iOS >= 17.4
  - Has some limitations to Appium execution with iOS devices due to actual Xcode tools being unavailable on Linux
- Windows 10
  - Supports Android / iOS < 17 && ios >= 17.4
  - Has some limitations to Appium execution with iOS devices due to actual Xcode tools being unavailable on Windows

Developed and tested on Ubuntu 18.04 LTS, Ubuntu 20.04 LTS, Windows 10, macOS Ventura 13.5.1

## Setup
Currently the project assumes that GADS hub, device providers, MongoDB and Selenium Grid are on the same network. They can all be on the same machine as well.
- Download the latest binary for your OS from [releases](https://github.com/shamanec/GADS/releases).

or build the project from source 
- Clone the project.
- Open the `hub/gads-ui` folder in Terminal.
- Execute `npm install`
- Execute `npm run build`
- Go back to the main repo folder.
- Execute `go build .`

### Common setup
#### MongoDB
The project uses MongoDB for storing logs and for synchronization of some data between hub and providers.
You can either run MongoDB in a docker container:  
- You need to have Docker(Docker Desktop on macOS, Windows) installed.
- Execute `docker run -d --restart=always --name mongodb -p 27017:27017 mongo:6.0`. This will pull the official MongoDB 6.0 image from Docker Hub and start a container binding ports `27017` for the MongoDB instance.
- You can use MongoDB Compass or another tool to access the db if needed.

or  
- Start MongoDB instance in the way you prefer

#### Hub setup
[Docs](./docs/hub.md)  

#### Provider setup
[Docs](./docs/provider.md)

### Thanks

| | About                                                                                                                                                              |
|---|--------------------------------------------------------------------------------------------------------------------------------------------------------------------| 
|[go-ios](https://github.com/danielpaulus/go-ios)| Many thanks for creating this CLI tool to communicate with iOS devices, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode |
|[Appium](https://github.com/appium)| It would be impossible to control the devices remotely without Appium for the control and WebDriverAgent for the iOS screen stream, kudos!                         |  

### Videos
#### Start hub
https://github.com/user-attachments/assets/7a6dab5a-52d1-4c48-882d-48b67e180c89

#### Add provider configuration
https://github.com/user-attachments/assets/07c94ecf-217e-4185-9465-8b8054ddef7e

#### Add devices and start provider
https://github.com/user-attachments/assets/a1b323da-0169-463e-9a37-b0364fc52480

#### Run Appium tests in parallel with TestNG
https://github.com/user-attachments/assets/cb2da413-6a72-4ead-9433-c4d2b41d5f4b

#### Remote control
https://github.com/user-attachments/assets/2d6b29fc-3e83-46be-88c4-d7a563205975




