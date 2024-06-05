
Unless you are building from source, running the hub does not require any additional dependencies except a running MongoDB instance.

### IMPORTANT
This is only hub/UI, to actually have devices available you need to have at least one [provider](./provider.md) instance running on the same host(or another host on the same network) that will actually set up and provision devices.   
Follow the setup steps to create and run a provider instance.  
You can have multiple provider instances on different hosts providing devices.  

### Starting hub instance
Run `./GADS hub` with the following flags:  
- `--auth=` - `true/false` to enable actual user authentication (default is `false`)  
- `--host-address=` - local IP address of the host machine, e.g. `192.168.1.6` (default is `localhost`, I would advise against using the default value)  
- `--port=` - port on which the UI and backend service will run  
- `--mongo-db=` - IP address and port of the MongoDB instance, e.g `192.168.1.6:27017` (default is `localhost:27017`) - tested only on local network
- `--ui-files-dir=` - directory where the UI static files will be unpacked and served from. By default the app tries to use a temporary folder available on the host automatically. **NB** Use this flag only if you have issues with the default behaviour.

Then access the hub UI and API on `http://{host-address}:{port}`

### UI development
If you want to work on the React UI with hot reload you need to add a proxy in `package.json` to point to the Go backend
1. Open the `hub/gads-ui` folder.
2. Open the `package-json` file.
3. Add a new field `"proxy": "http://192.168.1.28:10000/"` providing the host and port of the Go backend service.
4. Run `npm start`

### Additional notes
#### Selenium Grid
Devices can be automatically connected to Selenium Grid 4 instance. You need to create the Selenium Grid hub instance yourself and then set it up in the provider configuration to connect to it.  
* Start your Selenium hub instance, e.g. `java -jar selenium.jar --host 192.168.1.6 --port 4444`
* When adding/updating provider configuration from `Admin > Provider administration` you need to supply the Selenium hub address, e.g. `http://192.168.1.6:4444`
* You also need to upload the respective Selenium jar file so the provider instances have access to it
  * Log in to the hub with admin user, go to `Admin > Files administration` and upload the Selenium jar file - v4.13 is recommended.  
  * The file will be stored in Mongo and providers will download it on start automatically.

**NB** At the time support for Selenium Grid was implemented latest Selenium version was 4.15. The latest version that actually worked with Appium relay nodes was 4.13. I haven't tested with lower versions. Use lower versions at your own risk. Versions > 4.15 might also work but it wasn't tested as well.