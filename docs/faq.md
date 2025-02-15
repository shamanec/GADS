## FAQ

- *I have connected Android devices but provider is not picking them up at all, why?*
    - GADS uses `adb` to detect devices. First run `adb devices` in terminal to determine if you actually have devices discovered.
    - Make sure you have enabled [USB Debugging](./provider.md#usb-debugging) on each device
    - Make sure you have accepted the popup for debugging after connecting the devices to the host
- *I have connected device, it appears as live in UI but I cannot connect to it, why?*
    - Could be a number of things, make sure you run the provider with `--log-level=debug` and observe the provider logs
    - Observe the respective device `appium.log` file as well
- *When I start my provider WDA is in a install loop and I get the error: `Error accepting new connection accept tcp [::]:56505: use of closed network connection`*
    -   Make sure you've properly signed and created the uploaded [WebDriverAgent](./provider.md#prepare-webdriveragent-file---linux-windows) ipa
    -   In your provider config in GADS UI make sure you've provided proper bundle identifier for WebDriverAgent, e.g. `com.shamanec.WebDriverAgentRunner`
- *[macOS/Linux/Windows] I have a connected iOS device where WebDriverAgent installation/start up consistently fails, why?*
    - Make sure you've properly signed and created the uploaded [WebDriverAgent](./provider.md#prepare-webdriveragent-file---linux-windows) ipa
    - Observe the provider logs - if installation is failing, you will see the full `go-ios` command used by GADS to install the prepared WebDriverAgent ipa. Copy the command and try to run it from terminal without the provider. Observe and debug the output.
    - Observe the provider logs - if running of WDA is failing, you will see the full `go-ios` command used by GADS to run the installed WebDriverAgent. Copy the command and run it from terminal without the provider. Observe and debug the output
- *[Android] I can load the device in the UI but there is no video, why?*
    - **NB** The Android stream will not start properly if the device screen is off(device is locked)
    - Disconnect your device, find the `GADS-stream` app on it and uninstall it, reconnect the device - hopefully the new set up will be able to start it properly
    - You can also do the above through the UI - load the device, find the `GADS-stream` package in the installed apps and uninstall it. Go to `Admin > Providers Administration` and reset the device from the provider interface.
    - If above doesn't work - disconnect your device, tap on the `GADS-stream` app on it. If it asks for permissions - allow them and press the `Home` button on the device. Check in the notifications for something like `GADS-stream is recording the device screen`. Reconnect the device.
- *I can load the devices in UI but video is choppy/lags behind, is there something I can do?*
    - No, it is what it is. The Android stream was written by me and is as good as I was able to make it, don't think I can improve on streaming as well. For iOS we use the WebDriverAgent video stream so same applies there - we got what we got.
- *I can load the devices but interaction is slow/laggy, is there something I can do?*
    - No, it is what it is. GADS uses Appium under the hood for the interactions so we are as fast as it allows us to be.
- *I can load the device but interaction does not work at all/session expired popup appears, why?*
    - There is probably an issue with Appium setup or dependencies. It is quite possible to start Appium server successfully but everything fails due to missing environment variable like `ANDROID_HOME` or something in that line. Observe the respective device Appium logs either in UI or file
- *When I want to unlock my phone a session expired popup appears, why?*
  - Make sure your phone is not passcode protected since appium is only unlocking the device and it fails if it lands on the passcode screen 
