# GADS Hub and Provider Services

## Overview

The GADS Hub and Provider Services are designed to run as Linux services, allowing for easy management and monitoring of the GADS application. This documentation provides step-by-step instructions to set up and run both the GADS Hub and Provider Services on your system.

## Prerequisites

Before you begin, ensure that you have the following:

- A Linux-based operating system
- Root access to install packages and create necessary directories
- `logrotate` installed on your system
- The GADS application installed in the `/root/GADS` directory
- **Android SDK** installed (if the provider supports managing Android devices)
- Specific environment variables defined for the provider to function properly, such as `ANDROID_HOME` necessary for Appium to work correctly when Android devices are supported.

> **Important:** If the GADS application is installed in a different directory, you must adjust the `WorkingDirectory` in the service files accordingly.

## Installation Steps

### 1. Install Logrotate

First, ensure that `logrotate` is installed on your system. If it is not already installed, you can install it using the following command:

```sh
sudo apt install logrotate -y
```

### 2. Create Configuration Files

#### For GADS Provider

- Create the `gads` configuration file in the `/etc/sysconfig` directory. If the directory does not exist, create it.

```sh
# /etc/sysconfig/gads
# ANDROID_HOME is only required if the provider is managing Android devices. 
# Adjust this value and other environment variables as necessary to match the configuration of the environment where the provider is installed.
ANDROID_HOME=/root/.tools/android-sdk
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
PROVIDER_NICKNAME=YOUR_PROVIDER_NICKNAME
PROVIDER_MONGO_DB=YOUR_MONGO_DB_HOST:YOUR_MONGO_DB_PORT
PROVIDER_HUB=YOUR_HUB_URL
PROVIDER_LOG_LEVEL=debug
```

- Create the `gads-provider.service` file in the `/etc/systemd/system/` directory.

```sh
# /etc/systemd/system/gads-provider.service
[Unit]
Description=GADS Provider Service
After=network.target

[Service]
User=root
Group=root
EnvironmentFile=/etc/sysconfig/gads
Type=simple
WorkingDirectory=/root/GADS  # Adjust this path if GADS is installed elsewhere
ExecStart=/bin/bash -c 'trap "" SIGHUP; exec ./GADS provider --nickname=${PROVIDER_NICKNAME} --mongo-db=${PROVIDER_MONGO_DB} --hub=${PROVIDER_HUB} --log-level=${PROVIDER_LOG_LEVEL} >> /var/log/gads-provider.log 2>&1'
Restart=always
RestartSec=5s
KillSignal=SIGTERM

[Install]
WantedBy=multi-user.target
```

- Create the `gads-provider.logrotate` logrotate configuration file in `/etc/logrotate.d/` and add the following content:

```sh
# /etc/logrotate.d/gads-provider
/var/log/gads-provider.log {
    daily
    rotate 7
    dateext
    dateformat -%Y-%m-%d
    extension .log
    copytruncate
    missingok
    notifempty
}
```

#### For GADS Hub

- Create the `gads` configuration file in the `/etc/sysconfig` directory. If the directory does not exist, create it.

```sh
# /etc/sysconfig/gads
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
HUB_HOST_ADDRESS=YOUR_HUB_HOST_ADDRESS
HUB_PORT=YOUR_HUB_PORT
HUB_MONGO_DB=YOUR_MONGO_DB_HOST:YOUR_MONGO_DB_PORT
```

- Create the `gads-hub.service` file in the `/etc/systemd/system/` directory.

```sh
# /etc/systemd/system/gads-hub.service
[Unit]
Description=GADS Hub Service
After=network.target

[Service]
User=root
Group=root
EnvironmentFile=/etc/sysconfig/gads
Type=simple
WorkingDirectory=/root/GADS  # Adjust this path if GADS is installed elsewhere
ExecStart=/bin/bash -c 'trap "" SIGHUP; exec ./GADS hub --host-address=${HUB_HOST_ADDRESS} --port=${HUB_PORT} --mongo-db=${HUB_MONGO_DB} >> /var/log/gads-hub.log 2>&1'
Restart=always
RestartSec=5s
KillSignal=SIGTERM

[Install]
WantedBy=multi-user.target
```

- Create the `gads-hub.logrotate` logrotate configuration file in `/etc/logrotate.d/` and add the following content:

```sh
# /etc/logrotate.d/gads-hub
/var/log/gads-hub.log {
    daily
    rotate 7
    dateext
    dateformat -%Y-%m-%d
    extension .log
    copytruncate
    missingok
    notifempty
}
```

### 3. Enable and Start the Services

After setting up the configuration files, run the following commands to manage the GADS Hub and Provider Services:

- Reload the systemd manager configuration:

```sh
systemctl daemon-reload
```

- Enable the services to start on boot:

```sh
systemctl enable gads-provider.service
systemctl enable gads-hub.service
```

- Start the services:

```sh
systemctl start gads-provider.service
systemctl start gads-hub.service
```

### 4. Managing the Services

You can manage the GADS Hub and Provider Services using the following commands:

- To restart the services:

```sh
systemctl restart gads-provider.service
systemctl restart gads-hub.service
```

- To check the status of the services:

```sh
systemctl status gads-provider.service
systemctl status gads-hub.service
```

- To view the logs in real-time:

```sh
tail -f /var/log/gads-provider.log
tail -f /var/log/gads-hub.log
```

- To view the journal logs for the services:

```sh
journalctl -fu gads-provider.service
journalctl -fu gads-hub.service
```

- To edit the service configurations:

```sh
systemctl edit gads-provider.service
systemctl edit gads-hub.service
```

## Conclusion

Following these steps will help you successfully set up and run the GADS Hub and Provider Services on your Linux system. For further assistance, please refer to the service logs or consult the community for support.