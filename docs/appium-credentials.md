# 🔐 Appium Client Credentials

## Overview

GADS implements OAuth2 client credentials authentication for secure Appium test execution. This authentication mechanism ensures that only authorized clients can access and control devices through the Appium protocol.

> **📋 Need to create credentials?** See the [Client Credentials Management Guide](./client-credentials-management.md) for step-by-step instructions on creating and managing credentials.

## 🔧 Configuration

### Client ID Format

Client IDs in GADS follow a structured format to ensure uniqueness and traceability:
```
<prefix>_<timestamp>_<random_suffix>
```

Example: `gads_1704123456_abc12345`

### Configuring Custom Prefix

The client ID prefix can be customized using the `GADS_CLIENT_ID_PREFIX` environment variable:

```bash
# Default configuration (prefix: "gads")
./GADS hub --host-address=192.168.1.6 --port=10000

# Custom prefix for production environment
export GADS_CLIENT_ID_PREFIX=prod-mobile
./GADS hub --host-address=192.168.1.6 --port=10000

# Custom prefix for development environment
export GADS_CLIENT_ID_PREFIX=dev-testing
./GADS hub --host-address=192.168.1.6 --port=10000
```

### Why Use Custom Prefixes?

The configurable prefix provides valuable flexibility for:

1. **Environment-specific naming**: Different environments (dev, staging, prod) can use distinct prefixes to clearly identify resources and prevent cross-environment conflicts
2. **Multi-tenant scenarios**: Organizations running multiple GADS instances can differentiate them using custom prefixes
3. **Integration requirements**: Some organizations may have existing naming conventions or security policies that require specific prefixes

## 📱 Using Credentials with Appium

### Device-Specific Appium Endpoints

In GADS, each device has its own dedicated Appium server endpoint. This means you connect directly to a specific device using its unique ID in the URL:

```
http://gads-hub:10000/device/{device-id}/appium
```

For example:
```
http://gads-hub:10000/device/0123456789/appium
```

### Required Capability

Since the Appium server is already dedicated to a specific device, you only need to provide the `gads:clientSecret` capability for authentication:

```json
{
  "gads:clientSecret": "your-client-secret-here"
}
```

**Note**: Traditional Appium capabilities like `platformName`, `deviceName`, and `automationName` are not required because the device is already determined by the URL endpoint.

### Example: Java Client

```java
String deviceId = "0123456789"; // Your target device ID
String appiumUrl = String.format("http://gads-hub:10000/device/%s/appium", deviceId);

DesiredCapabilities caps = new DesiredCapabilities();
caps.setCapability("gads:clientSecret", System.getenv("GADS_CLIENT_SECRET"));

AppiumDriver driver = new RemoteWebDriver(
    new URL(appiumUrl), 
    caps
);
```

### Example: Python Client

```python
from appium import webdriver
import os

device_id = "0123456789"  # Your target device ID
appium_url = f"http://gads-hub:10000/device/{device_id}/appium"

caps = {
    "gads:clientSecret": os.environ.get("GADS_CLIENT_SECRET")
}

driver = webdriver.Remote(
    command_executor=appium_url,
    desired_capabilities=caps
)
```

### Example: JavaScript/Node.js Client

```javascript
const { remote } = require('webdriverio');

const deviceId = '0123456789'; // Your target device ID
const appiumPath = `/device/${deviceId}/appium`;

const caps = {
    'gads:clientSecret': process.env.GADS_CLIENT_SECRET
};

const driver = await remote({
    protocol: 'http',
    hostname: 'gads-hub',
    port: 10000,
    path: appiumPath,
    capabilities: caps
});
```

### Getting Device IDs

Device IDs can be obtained through:
1. The GADS web interface - visible in the device list
2. API endpoint: `GET /api/devices`
3. Device labels or configuration in your test infrastructure

## 🛡️ Security Considerations

### Secret Storage

- Client secrets are **never** stored in plain text
- Secrets are hashed using bcrypt with a cost factor of 10
- Each secret includes a unique salt for additional security
- Original secrets cannot be recovered from stored hashes

### Best Practices

1. **Secret Management**
   - Store client secrets in secure environment variables
   - Never commit secrets to version control
   - Rotate secrets regularly (recommended: every 90 days)
   - Use different secrets for different environments

2. **Access Control**
   - Limit credential scope to necessary devices only
   - Regularly audit credential usage
   - Revoke unused credentials promptly
   - Monitor failed authentication attempts

3. **Network Security**
   - Use HTTPS/TLS for all communications when possible