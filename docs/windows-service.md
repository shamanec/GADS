# GADS Hub and Provider Windows Services

## Overview

The GADS Hub and Provider Services can be configured to run as Windows services using WinSW (Windows Service Wrapper), allowing for easy management and automatic startup of the GADS application. This documentation provides step-by-step instructions to set up and run both the GADS Hub and Provider Services on Windows systems.

## Prerequisites

Before you begin, ensure that you have the following:

- A Windows-based operating system
- Administrator privileges to install and manage services
- The GADS application (`GADS.exe`) installed in a dedicated directory (e.g., `C:\GADS`)
- **Java JDK 17** or later installed and properly configured
- **Node.js and npm** installed (required for Appium)
- **Android SDK** installed (if the provider supports managing Android devices)
- **Appium** installed globally via npm (`npm install -g appium`)
- Network connectivity between Hub, Providers, and MongoDB

## Installation Steps

### 1. Download and Setup WinSW

WinSW (Windows Service Wrapper) is required to run GADS as a Windows service:

1. Download WinSW v3.0.0-alpha.11 from the [official GitHub repository](https://github.com/winsw/winsw/releases/tag/v3.0.0-alpha.11)
   
   > **Important:** We are using the v3.0.0-alpha.11 pre-release version because the service configuration templates in this documentation are only compatible with WinSW v3.x. While this is currently a pre-release version, it may become the latest stable release in the future.

2. Download the appropriate `WinSW.exe` file for your system architecture (x64, x86, or ARM64)
3. Rename the downloaded file to `winsw.exe` and copy it to the same directory as `GADS.exe`
4. Alternatively, add the path containing `winsw.exe` to your system PATH environment variable

### 2. Create Configuration Files

#### For GADS Hub

Create a `gads-hub.xml` file in the same directory as `GADS.exe`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<service>
    <!-- Basic service information -->
    <id>gads-hub</id>
    <name>GADS Hub Service</name>
    <description>GADS Hub Service for managing device connections and data flow</description>

    <!-- Service executable configuration -->
    <executable>GADS.exe</executable>
    <arguments>hub --host-address=%HUB_HOST_ADDRESS% --port=%HUB_PORT% --mongo-db=%HUB_MONGO_DB%</arguments>

    <!-- Environment variables -->
    <env name="HUB_HOST_ADDRESS" value="YOUR_HUB_HOST_ADDRESS"/>
    <env name="HUB_PORT" value="YOUR_HUB_PORT"/>
    <env name="HUB_MONGO_DB" value="YOUR_MONGO_DB_HOST:YOUR_MONGO_DB_PORT"/>

    <!-- Service behavior -->
    <startmode>Automatic</startmode>
    <logmode>rotate</logmode>
    <logpath>%BASE%</logpath>

    <!-- Service recovery options -->
    <onfailure action="restart" delay="10 sec"/>
    <resetfailure>1 hour</resetfailure>

    <!-- Service priority -->
    <priority>normal</priority>

    <!-- Working directory -->
    <workingdirectory>%BASE%</workingdirectory>

    <!-- Service stop timeout -->
    <stoptimeout>30 sec</stoptimeout>

    <!-- Pre-shutdown configuration -->
    <preshutdown>true</preshutdown>
    <preshutdownTimeout>3 min</preshutdownTimeout>
</service>
```

> **Note:** Adjust the paths in the environment variables to match your system configuration:
> - `HUB_HOST_ADDRESS`: The host address of the Hub service.
> - `HUB_PORT`: The port of the Hub service.
> - `HUB_MONGO_DB`: The MongoDB host and port of the Hub service.

#### For GADS Provider

Create a `gads-provider.xml` file in the same directory as `GADS.exe`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<service>
    <!-- Basic service information -->
    <id>gads-provider</id>
    <name>GADS Provider Service</name>
    <description>GADS Provider Service for managing device connections and providing services to the hub</description>

    <!-- Service executable configuration -->
    <executable>GADS.exe</executable>
    <arguments>provider --nickname=%PROVIDER_NICKNAME% --mongo-db=%PROVIDER_MONGO_DB% --hub=%PROVIDER_HUB% --log-level=%PROVIDER_LOG_LEVEL%</arguments>

    <!-- Environment variables -->
    <env name="NPM_PATH" value="C:\Program Files\nodejs"/>
    <env name="ANDROID_HOME" value="C:\Android\Sdk"/>
    <env name="JAVA_HOME" value="C:\Program Files\Java\jdk-17"/>
    <env name="PROVIDER_NICKNAME" value="YOUR_PROVIDER_NICKNAME"/>
    <env name="PROVIDER_MONGO_DB" value="YOUR_MONGO_DB_HOST:YOUR_MONGO_DB_PORT"/>
    <env name="PROVIDER_HUB" value="YOUR_HUB_URL"/>
    <env name="PROVIDER_LOG_LEVEL" value="debug"/>
    <env name="PATH" value="%NPM_PATH%;%ANDROID_HOME%\platform-tools;%JAVA_HOME%\bin;%PATH%"/>

    <!-- Service behavior -->
    <startmode>Automatic</startmode>
    <logmode>rotate</logmode>
    <logpath>%BASE%</logpath>

    <!-- Service recovery options -->
    <onfailure action="restart" delay="10 sec"/>
    <resetfailure>1 hour</resetfailure>

    <!-- Service priority -->
    <priority>normal</priority>

    <!-- Working directory -->
    <workingdirectory>%BASE%</workingdirectory>

    <!-- Service stop timeout -->
    <stoptimeout>30 sec</stoptimeout>

    <!-- Pre-shutdown configuration -->
    <preshutdown>true</preshutdown>
    <preshutdownTimeout>3 min</preshutdownTimeout>
</service>
```

> **Note:** Adjust the paths in the environment variables to match your system configuration:
> - `NPM_PATH`: Path to Node.js installation directory
> - `ANDROID_HOME`: Path to Android SDK installation directory
> - `JAVA_HOME`: Path to Java JDK installation directory

### 3. Install the Services

Open Command Prompt as Administrator and navigate to the directory containing `GADS.exe`, `winsw.exe`, and the XML configuration files.

#### Install Hub Service
```cmd
winsw.exe install gads-hub.xml
```

#### Install Provider Service
```cmd
winsw.exe install gads-provider.xml
```

### 4. Start the Services

You can start the services using either method:

#### Method 1: Using WinSW Commands
```cmd
winsw.exe start gads-hub.xml
winsw.exe start gads-provider.xml
```

#### Method 2: Using Windows Services Manager
1. Open `services.msc` (Windows Services Manager)
2. Find "GADS Hub Service" and "GADS Provider Service"
3. Right-click each service and select "Start"

## Managing the Services

You can manage the GADS Hub and Provider Services using the following commands:

### Check Service Status
```cmd
winsw.exe status gads-hub.xml
winsw.exe status gads-provider.xml
```

### Stop Services
```cmd
winsw.exe stop gads-hub.xml
winsw.exe stop gads-provider.xml
```

### Restart Services
```cmd
winsw.exe restart gads-hub.xml
winsw.exe restart gads-provider.xml
```

### View Service Logs
Log files are created in the same directory as the service executable. The log files are named:
- `gads-hub.out.log` and `gads-hub.err.log` for Hub service
- `gads-provider.out.log` and `gads-provider.err.log` for Provider service

### Using Windows Services Manager
1. Open `services.msc`
2. Find the GADS services
3. Right-click to start, stop, restart, or configure properties
4. Check the "Recovery" tab for automatic restart options

## Uninstallation

To remove the services:

1. First, stop the services:
```cmd
winsw.exe stop gads-hub.xml
winsw.exe stop gads-provider.xml
```

2. Then uninstall them:
```cmd
winsw.exe uninstall gads-hub.xml
winsw.exe uninstall gads-provider.xml
```

## Troubleshooting

### Common Issues

1. **Service fails to start**:
   - Check environment variables and file paths in XML configuration
   - Verify that all required dependencies (Java, Node.js, Android SDK) are properly installed
   - Check the service log files for detailed error information

2. **Permission denied**:
   - Ensure you're running Command Prompt as Administrator
   - Verify that the service account has proper permissions

3. **Port conflicts**:
   - Verify that the ports specified in the configuration are available
   - Use `netstat -an` to check for port usage

4. **Dependencies missing**:
   - Confirm JDK 17, Node.js, and Android SDK are properly installed
   - Verify that Appium is installed globally: `npm list -g appium`
   - Check that environment variables point to correct paths

### Verifying Installation

Before starting the services, verify your environment:

1. **Check Java Installation**:
```cmd
java -version
```

2. **Check Node.js and npm**:
```cmd
node --version
npm --version
```

3. **Check Appium Installation**:
```cmd
appium --version
```

4. **Verify Android SDK** (if using Android devices):
```cmd
adb version
```

### Log Files

Service logs are automatically created in the same directory as the executable:
- Standard output: `gads-hub.out.log` / `gads-provider.out.log`
- Error output: `gads-hub.err.log` / `gads-provider.err.log`

The `logmode>rotate</logmode>` setting ensures logs are automatically rotated to prevent excessive disk usage.

## Configuration Tips

1. **Multiple Providers**: You can run multiple provider services on the same machine by creating separate XML files with unique service IDs and nicknames.

2. **Custom Log Locations**: Modify the `<logpath>` element in the XML to specify a custom log directory.

3. **Service Dependencies**: Add `<depend>` elements to ensure services start in the correct order if needed.

4. **Memory Settings**: Add JVM options if needed using the `<env>` elements for Java-related configuration.

## Conclusion

Following these steps will help you successfully set up and run the GADS Hub and Provider Services as Windows services. This ensures automatic startup, proper logging, and easy management through Windows' native service management tools. For further assistance, please refer to the service logs or consult the community for support. 