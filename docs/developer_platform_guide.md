# Platform Support Development Guide

This guide explains how to implement support for new platforms in the GADS provider component. It follows established patterns used for iOS, Android, and Tizen TV support.

## Table of Contents
- [Platform Support Development Guide](#platform-support-development-guide)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Implementation Checklist](#implementation-checklist)
  - [Core Components](#core-components)
    - [1. Configuration](#1-configuration)
    - [2. Platform Detection](#2-platform-detection)
    - [3. Device Setup](#3-device-setup)
    - [4. Device Detection](#4-device-detection)
  - [Best Practices](#best-practices)
  - [Testing](#testing)

## Overview

The provider component uses a modular architecture for platform support. Each platform implementation requires:
1. Device detection and management
2. Platform-specific setup and configuration
3. Appium integration
4. Device state management

## Implementation Checklist

- [ ] Add platform-specific configuration in `config/config.go`
- [ ] Create platform detection utilities in `providerutil`
- [ ] Implement device setup logic in `devices/`
- [ ] Add platform validation in `provider.go`
- [ ] Update documentation

## Core Components

### 1. Configuration

Add a new platform flag in the provider configuration: 

```go
type ProviderConfig struct {
    // Existing fields...
    ProvideNewPlatform bool json:"provide_new_platform"
}
```

### 2. Platform Detection

Create a utility function in `providerutil/providerutil.go`:

```go
func NewPlatformToolAvailable() bool {
    logger.ProviderLogger.LogInfo("provider_setup", "Checking if new_platform_tool is set up and available on the host PATH")

    cmd := exec.Command("new_platform_tool", "--version")
    err := cmd.Run()
    if err != nil {
        logger.ProviderLogger.LogDebug("provider_setup", 
            fmt.Sprintf("newPlatformToolAvailable: tool is not available or command failed - %s", err))
        return false
    }
    return true
}
```

### 3. Device Setup

Create a new file `devices/new_platform.go`:

```go
package devices

func setupNewPlatformDevice(device *models.Device) {
    device.SetupMutex.Lock()
	defer device.SetupMutex.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)

    device.ProviderState = "preparing"
    logger.ProviderLogger.LogInfo("new_platform_setup", 
        fmt.Sprintf("Running setup for new platform device `%v`", device.UDID))

    // Platform-specific setup steps
    err := performPlatformSetup(device)
    if err != nil {
        resetLocalDevice(device)
        return
    }

    // Start Appium
    go startAppium(device)
    go checkAppiumUp(device)

    select {
    case <-device.AppiumReadyChan:
        logger.ProviderLogger.LogInfo("new_platform_setup", 
            fmt.Sprintf("Successfully started Appium for device `%v` on port %v", 
                device.UDID, device.AppiumPort))
    case <-time.After(30 * time.Second):
        logger.ProviderLogger.LogError("new_platform_setup", 
            fmt.Sprintf("Did not successfully start Appium for device `%v` in 30 seconds", 
                device.UDID))
        resetLocalDevice(device)
        return
    }

    wg.Wait()
    device.ProviderState = "live"
}
```

### 4. Device Detection

Add platform detection in `devices/common.go`:

```go
func getConnectedDevicesNewPlatform() []string {
    var devices []string
    cmd := exec.Command("new_platform_tool", "list")
    output, err := cmd.CombinedOutput()
    if err != nil {
        logger.ProviderLogger.LogError("device_setup", 
            fmt.Sprintf("Failed to get connected new platform devices - %s", err))
        return devices
    }
    
    // Parse device list output
    return parseDeviceList(output)
}
```

## Best Practices

1. **Error Handling**
   - Always use the logger for errors and important events
   - Reset device state on setup failures
   - Provide meaningful error messages

2. **Device State Management**
   - Follow the existing state flow: init → preparing → live
   - Use channels for async operations
   - Clean up resources on device disconnection

3. **Code Organization**
   - Keep platform-specific code in separate files
   - Reuse common utilities when possible
   - Follow existing naming conventions

Reference implementation examples:

iOS Device Setup:
```go:provider/devices/ios.go
startLine: 13
endLine: 40
```

Tizen Device Setup:
```go:provider/devices/tizen.go
startLine: 13
endLine: 40
```

## Testing

1. **Basic Connectivity**
```go
func TestNewPlatformDetection(t *testing.T) {
    devices := getConnectedDevicesNewPlatform()
    assert.NotNil(t, devices)
}
```

2. **Device Setup**
```go
func TestNewPlatformSetup(t *testing.T) {
    device := &models.Device{
        UDID: "test-device",
        OS:   "new_platform",
    }
    setupNewPlatformDevice(device)
    assert.Equal(t, "live", device.ProviderState)
}
```