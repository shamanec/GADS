
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
- `--admin-username=` - username of the default admin user (default is `admin`)  
- `--admin-password=` - password of the default admin user (default is `password`)  
- `--admin-email=` - email of the default admin user (default is `admin@gads.ui`)  
- `--ui-files-dir=` - directory where the UI static files will be unpacked and served from. By default the app tries to use a temporary folder available on the host automatically. **NB** Use this flag only if you have issues with the default behaviour.

Then access the hub UI and API on `http://{host-address}:{port}`

### UI development
If you want to work on the React UI with hot reload you need to add a proxy in `package.json` to point to the Go backend
1. Open the `hub/gads-ui` folder.
2. Open the `package-json` file.
3. Add a new field `"proxy": "http://192.168.1.28:10000/"` providing the host and port of the Go backend service.
4. Run `npm start`