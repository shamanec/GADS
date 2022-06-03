## Introduction

* GADS or Go Appium Docker Service is a small webserver that allows you to configure and monitor Appium docker containers and essentially create your own device farm for Appium test execution.   
* **NB** It is a work in progress and is in no way a full-fledged and finalized solution. This is my first attempt at Go and web dev in general so a lot of the code is probably messy as hell. I will be doing my best to cleanup and improve all the time but for now this is just a working POC.  
**NB** I've been doing this having only small number of devices available. It looks like everything is pretty much working but I do not know how it would behave on a bigger scale.  
* Currently being adopted and sponsored by <a href="https://1crew.com"><img src="https://1crew.com/StaticResources/1Crew_3D.png" alt="1crew" width="50"/><a/>  

## Features
* Mostly straighforward setup  
* Web UI to:  
  * See Appium config and other info  
  * Observe device containers data in real time, also see related logs  
  * Observe project logs with messages and timestamps  
* Endpoints to control the project without the UI  
* iOS Appium servers in Docker containers  
  - Most of the available functionality of the iOS devices is essentially a wrapper of the amazing [go-ios](https://github.com/danielpaulus/go-ios) project without which none of this would be possible  
  - Automatically spin up when registered device is connected/disconnected  
  - Self-healing checks to reinstall/restart WebDriverAgent if it fails  
  - Selenium Grid 3 connection  
  - Run iOS Appium tests on cheap hardware on much bigger scale with only one host machine and in isolation  
  - There are some limitations, you can check [Devices setup](./docs/devices-setup.md)  
* Android Appium servers in Docker containers  
  - Automatically spin up when registered device is connected/disconnected  
  - Selenium Grid 3 connection  
* Remote device control page:  
  1. Device video stream:  
    - for iOS the WDA mjpeg stream is used to stream video off the devices  
    - for Android minicap is used to stream video off the devices  
  2. Remote control:  
    - tap  
    - swipe  
    - type  
    - Appium itself is used to perform all of these actions  
  3. Information about the device(configuration, installed apps, available apps to install - WIP)  
  4. Simple web based 'Appium inspector':  
    - Create a new Appium session if none exists - not so useful when you have the actual and much better inspector from Appium  
    - Automatically connect to existing Appium session - useful for remote debugging while running/writing tests  
    - Search for elements using the usual Appium identifiers, visualize outline on stream upon element selection  
    - Get page source - visualize page source as tree similar to actual 'Appium Inspector', visualize element info upon selection, visualize outline on stream upon element selection  

Developed and tested on Ubuntu 18.04 LTS  

## Setup and docs  
[Devices setup](./docs/devices-setup.md)  
[General project setup](./docs/project-setup.md)  

## Thanks

| |About|
|---|---|
|[go-ios](https://github.com/danielpaulus/go-ios)|Many thanks for creating this tool to communicate with iOS devices on Linux, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode. Without it none of this would be possible|
|[iOS App Signer](https://github.com/DanTheMan827/ios-app-signer)|This is an app for OS X that can (re)sign apps and bundle them into ipa files that are ready to be installed on an iOS device.|
|[minicap](https://github.com/DeviceFarmer/minicap)|Stream screen capture data out of Android devices|

