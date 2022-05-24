## Dependencies  
The project itself has minimum dependencies:  
1. Install Docker.  
2. Install Go 1.17 (that is what I'm using, lower might also work)  

## Update the environment file  
1. Set your sudo password - it is used by the commands that apply the systemd usbmuxd.service and the udev rules. It is used only locally so there should be no security risk unless you publicly commit it.   
2. Set Selenium Grid connection - true or false. True attempts to connect each Appium server to the Selenium Grid instance defined in *./configs/config.json*  
4. Set your supervision identity password(same applies as step 1). The project assumes you are supervising your devices so that everything could happen automatically.  

## Run the project   
1. Execute 'go run main.go'  
2. Open your browser and go to *http://localhost:10000*.  

You can access Swagger documentation on *http://localhost:10000/swagger/index.html*  

## Setup  
### Build iOS Docker image
1. Cd into the project folder  
2. Execute *docker build -f Dockerfile -t ios-appium .*  

### Build Android Docker image
1. Cd into the project folder.  
2. Execute *docker build -f Dockerfile-Android -t android-appium .*

### Setup the usbmuxd.service and udev listener
1. Open the Project Config page.  
2. Tap on "Setup listener" - you need to have your sudo password set up in the *./configs/config.json* file.  

This will move *./configs/usbmuxd.service* to */lib/systemd/system* and enable the service - this starts usbmuxd automatically after reboot. It will also create and set udev rules in */etc/udev/rules.d* that will trigger the container updates when registered iOS/Android device is connected/disconnected from the machine.  

## Make iOS connection more stable
usbmuxd is being stopped by its own udev rules when the last iOS device is disconnected. This could potentially lead to problems creating containers when reconnecting devices after disconnecting all of them. To alleviate the issue you can:  
1. Open /lib/udev/rules.d/39-usbmuxd.rules  
2. Comment out or remove the last line:  
*SUBSYSTEM=="usb", ENV{DEVTYPE}=="usb_device", ENV{PRODUCT}=="5ac/12[9a][0-9a-f]/*", ACTION=="remove", RUN+="/usr/sbin/usbmuxd -x"*  

### Update the project config  
1. Open the Project Config page.  
2. Tap on "Change config".  
3. Update your Selenium Grid values and the bundle ID of the used WebDriverAgent.  

### Update host udev rules service
1. Open /lib/systemd/system/systemd-udevd.service ('sudo systemctl status udev.service' to find out if its a different file)
2. Add IPAddressAllow=127.0.0.1 at the bottom
3. Restart the machine.
4. This is to allow curl calls from the udev rules to the GADS server

### Setup udev rules using docker-cli instead of the server to start/stop containers
1. Find the CreateUdevRules() function in project_control.go
2. Comment the rule lines for Android/iOS for the server.
3. Uncomment the rule lines for Android/iOS for docker-cli.
4. Clone https://github.com/shamanec/GADS-docker-cli
5. Open the start-device-container.go
6. Change the project path to where your GADS project is located
7. Run 'go build -o docker-cli .' in the GADS-docker-cli main folder.
8. Copy the created cli tool 'docker-cli' to /usr/local/bin 

### Spin up containers  
If you have followed all the steps, registered the devices, built the images and added the udev rules just connect all your devices. Container should be automatically created for each of them.  

**NB** For a way to perform most of these actions without the UI you can refer to the Swagger documentation. 
