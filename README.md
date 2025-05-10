<!--
  Title: GADS - Open Source Device Farm
  Description: Self-hosted device farm and test automation platform for iOS and Android. Open source alternative to AWS Device Farm and Firebase Test Lab with Appium integration.
  Author: shamanec
  Tags: device-farm, mobile-testing, ios-testing, android-testing, appium, test-automation, qa-tools, continuous-testing, mobile-device-management, selenium-grid
  -->

<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="/docs/gads-logo-light.png" alt="GADS - Open Source Mobile Device Farm Platform - Dark Theme Logo">
    <img src="/docs/gads-logo.png" width="256" alt="GADS - Open Source Mobile Device Farm Platform for iOS and Android Automated Testing"/>
  </picture>

  <h1>GADS - Open Source Device Farm for Mobile Testing</h1>

  [![GitHub Stars](https://img.shields.io/github/stars/shamanec/GADS?style=social)](https://github.com/shamanec/GADS/stargazers)
  [![GitHub Release](https://img.shields.io/github/v/release/shamanec/GADS)](https://github.com/shamanec/GADS/releases)
  [![GitHub Downloads](https://img.shields.io/github/downloads/shamanec/GADS/total)](https://github.com/shamanec/GADS/releases)
  [![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
  [![Discord](https://dcbadge.vercel.app/api/server/5amWvknKQd?style=flat&theme=clean&compact=true)](https://discord.gg/5amWvknKQd)

  🚀 **Self-Hosted Device Farm & Test Automation Platform** - Open Source Alternative to AWS Device Farm and Firebase Test Lab
</div>

## 🎯 What is GADS?

**GADS** is a free, open-source device farm platform that enables **remote device control** and **Appium test execution** on mobile devices. Perfect for QA teams, mobile developers, and organizations looking for a self-hosted alternative to expensive cloud testing services like AWS Device Farm and Firebase Test Lab.

The platform architecture consists of two main components:
- **Hub**: A web interface for remote device control and provider management.
- **Provider**: Handles device setup and makes them available for remote access.

### Why Choose GADS?
- 💰 **Free & Open Source**: Complete alternative to AWS Device Farm and Firebase Test Lab
- 📱 **Cross-Platform**: Full support for both iOS and Android devices
- 🎮 **Remote Control**: Real-time device control and testing capabilities
- 🔌 **Appium Compatible**: Works with industry-standard Appium testing framework
- 🛠 **Easy Setup**: Simple installation and configuration process

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
- 👥 **Workspace Management**
  - User access control per workspace
  - Default workspace for legacy support
  - [Detailed Workspace Documentation](./docs/workspaces.md)

### Provider Features 🔌
- 🛠️ **Easy Setup**
  - UI-based device management
- 🤖 **Automated Device Provisioning**
  - Per-device Appium server configuration
- 📡 **Remote Control**
  - iOS streaming via [WebDriverAgent](https://github.com/appium/WebDriverAgent)
  - Android streaming via [GADS-Android-stream](https://github.com/shamanec/GADS-Android-stream)
  - Android WebRTC video stream (Experimental) - [notes](./docs/provider.md#android-webrtc-video---experimental)
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

## 💡 Use Cases

- **Mobile App Testing**: Automate testing across multiple real devices
- **Manual QA**: Remote access to physical devices for manual testing
- **CI/CD Pipeline**: Integrate automated testing in your deployment workflow
- **Device Lab Management**: Centralized management of your organization's mobile devices
- **Cross-Browser Testing**: Test web applications across multiple mobile browsers

## 📊 Project Status

- **Project Stage**: Active Development
- **Contributors**: [View Contributors](https://github.com/shamanec/GADS/graphs/contributors)

## 🔍 Keywords

`device-farm`, `mobile-testing`, `ios-testing`, `android-testing`, `appium`, `test-automation`, `qa-tools`, `continuous-testing`, `mobile-device-management`, `selenium-grid`, `remote-device-control`, `mobile-qa`
