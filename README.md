## Introduction

* GADS or Go Appium Docker Service is a web UI for [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider) orchestration and remote control of devices.  

## Features
1. Provider logs for debugging  
2. Devices remote control(most of which is wrapper around Appium)
  * Android
    - `minicap` video stream  
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
1. Add the IP addresses and ports of each created [GADS-devices-provider](https://github.com/shamanec/GADS-devices-provider) in the `config.json` file.  

## Thanks

| |About|
|---|---|
|[go-ios](https://github.com/danielpaulus/go-ios)|Many thanks for creating this tool to communicate with iOS devices on Linux, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode. Without it none of this would be possible|
|[iOS App Signer](https://github.com/DanTheMan827/ios-app-signer)|This is an app for OS X that can (re)sign apps and bundle them into ipa files that are ready to be installed on an iOS device.|
|[minicap](https://github.com/DeviceFarmer/minicap)|Stream screen capture data out of Android devices|  
|[Appium](https://github.com/appium)|It would be impossible to control the devices remotely without Appium for the control and WebDriverAgent for the iOS screen stream, kudos!|  

## WIP demo video  

https://user-images.githubusercontent.com/60219580/183677067-237c12d9-f06d-4b14-985c-17aedbb19ea6.mp4




