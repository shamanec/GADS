<!--
  Title: GADS - Open Source Device Farm
  Description: Self-hosted device farm and test automation platform for iOS, Android, and Samsung Tizen OS smart TVs. Open source alternative to AWS Device Farm and Firebase Test Lab with Appium integration.
  Author: shamanec
  Tags: device-farm, mobile-testing, ios-testing, android-testing, appium, test-automation, qa-tools, continuous-testing, mobile-device-management, selenium-grid
  -->

<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="/docs/gads-logo-light.png" alt="GADS - Open Source Mobile Device Farm Platform - Dark Theme Logo">
    <img src="/docs/gads-logo.png" width="256" alt="GADS - Open Source Mobile Device Farm Platform for iOS and Android Automated Testing"/>
  </picture>

  <h1>GADS - Device Farm for Mobile & Smart TV Testing</h1>

  [![GitHub Stars](https://img.shields.io/github/stars/shamanec/GADS?style=social)](https://github.com/shamanec/GADS/stargazers)
  [![GitHub Release](https://img.shields.io/github/v/release/shamanec/GADS)](https://github.com/shamanec/GADS/releases)
  [![GitHub Downloads](https://img.shields.io/github/downloads/shamanec/GADS/total)](https://github.com/shamanec/GADS/releases)
  [![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
  [![Discord](https://dcbadge.vercel.app/api/server/5amWvknKQd?style=flat&theme=clean&compact=true)](https://discord.gg/5amWvknKQd)

  🚀 **Self-Hosted Device Farm & Test Automation Platform** - Alternative to AWS Device Farm and Firebase Test Lab for Mobile & Smart TV Testing
</div>

## 🎯 What is GADS?

**GADS** is a free, open-source device farm platform that enables **remote device control** and **Appium test execution** on mobile devices (iOS/Android) and smart TVs (Samsung Tizen OS). Perfect for QA teams, mobile developers, and organizations looking for a self-hosted alternative to expensive cloud testing services like AWS Device Farm and Firebase Test Lab.

The platform architecture consists of two main components:
- **Hub**: A web interface for remote device control and provider management.
- **Provider**: Handles device setup and makes them available for remote access.

### Why Choose GADS?
- 💰 **Free**: Self-hosted alternative to AWS Device Farm and Firebase Test Lab
- 📱 **Cross-Platform**: Full support for iOS and Android devices, plus automated testing for Samsung Tizen OS smart TVs
- 🎮 **Remote Control**: Real-time device control and testing capabilities
- 🔌 **Appium Compatible**: Works with industry-standard Appium testing framework
- 🔑 **Flexible Authentication**: Support for multiple JWT issuers with origin-based keys
- 🛠 **Easy Setup**: Simple installation and configuration process

## ✨ Key Features

### Hub Features 🎯
- 🔐 **Authentication System**
  - User login with session management
  - Admin user management
  - Origin-based secret key management
  - Multiple JWT provider support
  - [Detailed Secret Keys Documentation](./docs/secret-keys.md)
- 📱 **Device Control**
  - Real-time video streaming (MJPEG/WebRTC)
  - Remote interactions: tap, swipe, text input, get clipboard
  - Keyboard typing
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
  - Per-device Appium server configuration (optional)
- 📡 **Remote Control**
  - iOS streaming via [WebDriverAgent](https://github.com/appium/WebDriverAgent)
  - Android MJPEG video streaming
  - Android WebRTC video stream (Experimental) - [notes](./docs/provider.md#android-webrtc-video---experimental)
  - Non-Appium based interaction
  - Keyboard typing (highly performant on Android, usable on iOS)
- 🧪 **Testing Integration**
  - Individual Appium server endpoints (optional)
  - Optional Selenium Grid 4 node registration
  - Automated testing for Samsung Tizen OS smart TVs (no remote control, testing only)

## 💻 Platform Support

| OS         | Android Support | iOS Support | Tizen OS Support    | Notes                                                             |
|------------|----------------|-------------|----------------------|-------------------------------------------------------------------|
| **macOS**  | ✅             | ✅           | ✅ (automation only) | Full support for mobile, Tizen OS supports only automated testing |
| **Linux**  | ✅             | ⚠️           | ✅ (automation only) | Limited iOS support due to Xcode dependency                       |
| **Windows**| ✅             | ⚠️           | ✅ (automation only) | Limited iOS support due to Xcode dependency                       |

**Important**: Tizen OS support is focused on **automated testing only**. Manual interaction and real-time device control available for mobile devices are not supported for smart TVs.

## License

This repository is **dual-licensed**:

- **Open Source Components** (AGPL-3.0):
  All source code in this repository, excluding explicitly listed proprietary components, is licensed under the [GNU Affero General Public License v3.0 (AGPL-3.0)](https://www.gnu.org/licenses/agpl-3.0.html).

- **Proprietary Components**:
  The `hub-ui` directory is licensed under a separate proprietary license. See [`PROPRIETARY-LICENSE.txt`](./PROPRIETARY-LICENSE.txt) for more information.

Please refer to the [`LICENSE-OVERVIEW`](./LICENSE-OVERVIEW.txt) file for a detailed overview.

### Using GADS

GADS, including both open source and obfuscated proprietary components, is freely available for use under the terms specified in the license. Users can utilize all functionalities provided by GADS, including those powered by the proprietary components.

### Important Notes on Proprietary Components

- While the proprietary components are included in the distribution, their source code is not available for viewing, modification, or redistribution.
- These components are provided in an obfuscated form to protect our intellectual property.
- Users are granted the right to use these components as part of GADS, but not to decompile, reverse engineer, or attempt to extract the original source code.

### Contributions and Modifications

- Contributions and modifications to the open-source portions of GADS are welcome.
- Please note that it is not possible to contribute to or modify the proprietary components due to their obfuscated nature.

## 🚀 Getting Started

> ### **Prerequisites**
> Before getting started, make sure you have the following:
> - A **MongoDB** instance (v6.0 recommended)
> - Network connectivity between Hub, Providers, MongoDB, and Selenium Grid
> ---

### ⚡ Quick Start

#### Option 1: Download the latest binary

1. Go to the [releases page](https://github.com/shamanec/GADS/releases) and download the latest binary for your platform.

#### Option 2: Build from source for non-UI related development
**IMPORTANT** You can freely use the Go code to your ends or provide new features/bug fixes on mainstream project but any changes to the UI should be requested from the core team.  

```bash
# Clone the repository
git clone https://github.com/shamanec/GADS

# Build the application without UI
cd ../..
go build .
```

#### Option 3: Build from source for UI related development
**IMPORTANT** You can freely use the Go code to your ends or provide new features/bug fixes on mainstream project but any changes to the UI should be requested from the core team.  

1. Clone the repository
```bash
git clone https://github.com/shamanec/GADS
```
2. Download the prebuilt UI files zip from the latest [release](https://github.com/shamanec/GADS/releases)
3. Unzip the file from step into your GADS folder in a new folder named `hub-ui`, your folder structure should look like `../GADS/hub-ui/build/*`
4. Build the application
```bash
cd ../..
go build -tags ui .
```
> **Note**: Optionally before building you can update the docs.go (OpenAPI spec) by running `swag init -g hub/hub.go -o docs`

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
For detailed instructions on how to create a service for Windows using WinSW, please refer to the [Windows Service Documentation](./docs/windows-service.md).

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

### Mobile Testing 📱
- **Mobile App Testing**: Automate testing across multiple real iOS and Android devices
- **Manual QA**: Remote access to physical devices for manual testing and debugging
- **Cross-Browser Testing**: Test web applications across multiple mobile browsers
- **Device Lab Management**: Centralized management of your organization's mobile devices

### Smart TV Testing 📺
- **Tizen OS App Testing**: Automated testing of applications for Samsung smart TVs
- **TV-Specific Testing**: Validate TV app functionality, performance, and compatibility
- **Remote-First Testing**: Test TV apps without physical access to devices

## 📊 Project Status

- **Project Stage**: Active Development
- **Contributors**: [View Contributors](https://github.com/shamanec/GADS/graphs/contributors)

## 🔍 Keywords

`device-farm`, `mobile-testing`, `ios-testing`, `android-testing`, `appium`, `test-automation`, `qa-tools`, `continuous-testing`, `mobile-device-management`, `selenium-grid`, `remote-device-control`, `mobile-qa`, `tizen-testing`, `smart-tv-testing`
