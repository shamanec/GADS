<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="/docs/gads-logo-light.png">
    <img src="/docs/gads-logo.png" width="256" alt="GADS Logo"/>
  </picture>

  <h1>GADS - Mobile Device Management & Testing Platform</h1>

  [![Discord](https://dcbadge.vercel.app/api/server/5amWvknKQd)](https://discord.gg/5amWvknKQd)

  🚀 **Remote control and automated testing** for iOS & Android devices
</div>

## 📖 Overview

**GADS** is a platform for **remote device control** and **Appium test execution** on mobile devices. It consists of two main components:

- **Hub**: A web interface for remote device control and provider management.
- **Provider**: Handles device setup and makes them available for remote access.

## ✨ Key Features

### Hub Features 🎯
- 🔐 **Authentication System**
  - User login with session management
  - Admin user management
- 📱 **Device Control**
  - Real-time video streaming (MJPEG)
  - Remote interactions: tap, swipe, text input
  - App installation/uninstallation
  - High-quality screenshots
  - Device reservation system
- 🔄 **Backend Capabilities**
  - Web interface serving
  - Provider communication proxy
  - Experimental **Selenium Grid** replacement

### Provider Features 🔌
- 🛠️ **Easy Setup**
  - UI-based device management
- 🤖 **Automated Device Provisioning**
  - Per-device Appium server configuration
- 📡 **Remote Control**
  - iOS streaming via [WebDriverAgent](https://github.com/appium/WebDriverAgent)
  - Android streaming via [GADS-Android-stream](https://github.com/shamanec/GADS-Android-stream)
  - Comprehensive Appium-based device interaction
- 🧪 **Testing Integration**
  - Individual Appium server endpoints
  - Optional Selenium Grid 4 node registration

## 💻 Platform Support

| OS        | Android Support | iOS Support  | Notes |
|-----------|-----------------|--------------|-------|
| **macOS** | ✅               | ✅            | Full support |
| **Linux** | ✅               | ⚠️            | Limited iOS support due to Xcode dependency |
| **Windows** | ✅             | ⚠️            | Limited iOS support due to Xcode dependency |

## 🚀 Getting Started

> ### **Prerequisites**
> Before getting started, make sure you have the following:
> - A **MongoDB** instance (v6.0 recommended)
> - Network connectivity between Hub, Providers, MongoDB, and Selenium Grid
> ---

### ⚡ Quick Start

#### Option 1: Download the latest binary

1. Go to the [releases page](https://github.com/shamanec/GADS/releases) and download the latest binary for your platform.

#### Option 2: Build from source

```bash
# Clone the repository
git clone https://github.com/shamanec/GADS

# Build the UI
cd hub/gads-ui
npm install
npm run build

# Build the application
cd ../..
go build .
```

### 🛠️ Common setup
#### 🌱 MongoDB
The project uses MongoDB for storing logs and for synchronization of some data between hub and providers.
You can either run MongoDB in a docker container:  
- You need to have Docker(Docker Desktop on macOS, Windows) installed.
- Execute `docker run -d --restart=always --name mongodb -p 27017:27017 mongo:6.0`. This will pull the official MongoDB 6.0 image from Docker Hub and start a container binding ports `27017` for the MongoDB instance.
- You can use MongoDB Compass or another tool to access the db if needed.

or  
- Start MongoDB instance in the way you prefer

#### ⚙️ Hub setup
For detailed instructions on setting up the Hub, refer to the [Hub Setup Docs](./docs/hub.md)  

#### 📱 Provider setup
For detailed instructions on setting up the Provider, refer to the [Provider Setup Docs.](./docs/provider.md)

## Running GADS as a System Service
To ensure that GADS runs continuously and can be managed easily, it is recommended to execute it as a service on your operating system. Running GADS as a service allows it to start automatically on boot, restart on failure, and be managed through standard service commands.

### 🐧 Linux
For detailed instructions on how to create a service for Linux using systemd, please refer to the [Linux Service Documentation](./docs/linux-service.md).

### 🖥️ Windows
*Note: Service implementation for Windows is yet to be documented.*

### 🍏 macOS
*Note: Service implementation for macOS is yet to be documented.*

## ❓ FAQ

The **FAQ** (Frequently Asked Questions) section has been created to provide quick answers to the most common questions about GADS. If you have any questions regarding installation, setup, or functionality, check out the answers in our documentation.

For more details, refer to the [full FAQ](./docs/faq.md).

## 🙏 Thanks

| | About                                                                                                                                                              |
|---|--------------------------------------------------------------------------------------------------------------------------------------------------------------------| 
|[go-ios](https://github.com/danielpaulus/go-ios)| Many thanks for creating this CLI tool to communicate with iOS devices, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode |
|[Appium](https://github.com/appium)| It would be impossible to control the devices remotely without Appium for the control and WebDriverAgent for the iOS screen stream, kudos!                         |  

## 🎥 Videos
#### Start hub
https://github.com/user-attachments/assets/7a6dab5a-52d1-4c48-882d-48b67e180c89

#### Add provider configuration
https://github.com/user-attachments/assets/07c94ecf-217e-4185-9465-8b8054ddef7e

#### Add devices and start provider
https://github.com/user-attachments/assets/a1b323da-0169-463e-9a37-b0364fc52480

#### Run Appium tests in parallel with TestNG
https://github.com/user-attachments/assets/cb2da413-6a72-4ead-9433-c4d2b41d5f4b

#### Remote control
https://github.com/user-attachments/assets/2d6b29fc-3e83-46be-88c4-d7a563205975




