## Introduction

* GADS or Go Appium Docker Service is a web UI for [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider) orchestration and remote control of devices.  

* Currently being adopted and sponsored by <a href="https://1crew.com"><img src="https://1crew.com/StaticResources/1Crew_3D.png" alt="1crew" width="50"/><a/>  

## Features
1. Provider logs for debugging  
2. Devices remote control
  * Android
    - `minicap` video stream  
    - basic device interaction - Home, Lock, Unlock, Type text, Clear text  
    - basic remote control - tap, swipe  
    - basic Appium inspector
  * iOS
    - `WDA mjpeg` video stream  
    - basice device interaciton - Home, Lock, Unlock, Type text, Clear text  
    - basic remote control - tap, swipe  
    - basic Appium inspector  

Developed and tested on Ubuntu 18.04 LTS  

## Setup
1. Add the IP addresses and ports of each created [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider) in the `config.json` file.  

## Thanks

| |About|
|---|---|
|[go-ios](https://github.com/danielpaulus/go-ios)|Many thanks for creating this tool to communicate with iOS devices on Linux, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode. Without it none of this would be possible|
|[iOS App Signer](https://github.com/DanTheMan827/ios-app-signer)|This is an app for OS X that can (re)sign apps and bundle them into ipa files that are ready to be installed on an iOS device.|
|[minicap](https://github.com/DeviceFarmer/minicap)|Stream screen capture data out of Android devices|  

## WIP demo video  

https://user-images.githubusercontent.com/60219580/171872161-a70c66ad-1b0f-4dee-a479-14be61799257.mp4  
