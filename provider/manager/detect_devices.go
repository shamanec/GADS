/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package manager

import (
	"context"
	"strings"

	"GADS/common/models"
	"GADS/provider/devices"

	"github.com/danielpaulus/go-ios/ios"
)

// detectConnectedDevices returns the union of all platform-specific device IDs
// currently connected to the host, filtered by which platforms the provider is
// configured to support.
func getConnectedDevices(cfg *models.Provider, cmd devices.CommandRunner) []string {
	var ids []string

	if cfg.ProvideAndroid {
		ids = append(ids, detectAndroid(cmd)...)
	}
	if cfg.ProvideIOS {
		ids = append(ids, detectIOS()...)
	}
	if cfg.ProvideTizen {
		ids = append(ids, detectTizen(cmd)...)
	}
	if cfg.ProvideWebOS {
		ids = append(ids, detectWebOS(cmd)...)
	}

	return ids
}

// detectAndroid runs `adb devices` and returns the serial numbers of all
// devices currently in the "device" state (i.e. authorised and ready).
func detectAndroid(cmd devices.CommandRunner) []string {
	out, err := cmd.Run(context.Background(), "adb", "devices")
	if err != nil {
		return nil
	}

	var ids []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "List of devices") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		// Only include authorised, ready devices (state == "device").
		if len(fields) >= 2 && fields[1] == "device" {
			ids = append(ids, fields[0])
		}
	}
	return ids
}

// detectIOS uses the go-ios library to list USB-attached iOS devices. Returns
// their serial numbers (UDIDs).
func detectIOS() []string {
	deviceList, err := ios.ListDevices()
	if err != nil {
		return nil
	}

	var ids []string
	for _, d := range deviceList.DeviceList {
		ids = append(ids, d.Properties.SerialNumber)
	}
	return ids
}

// detectTizen runs `sdb devices` and returns the device IDs of all connected
// Tizen devices reported as ready.
func detectTizen(cmd devices.CommandRunner) []string {
	out, err := cmd.Run(context.Background(), "sdb", "devices")
	if err != nil {
		return nil
	}

	var ids []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "List of devices attached") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "device" {
			ids = append(ids, fields[0])
		}
	}
	return ids
}

// detectWebOS runs `ares-setup-device --list` and returns the IP addresses of
// all registered WebOS TV devices. For WebOS, the device UDID IS the IP address.
func detectWebOS(cmd devices.CommandRunner) []string {
	out, err := cmd.Run(context.Background(), "ares-setup-device", "--list")
	if err != nil {
		return nil
	}

	var ids []string
	for _, line := range strings.Split(string(out), "\n") {
		// Skip empty lines, header, separator lines, and emulator lines.
		if line == "" || strings.Contains(line, "name") ||
			strings.Contains(line, "----") || strings.Contains(line, "emulator") {
			continue
		}

		// Output format: name  user@IP:PORT  ssh  tv
		// Find the field that contains "@" and ":" (deviceinfo field).
		for _, field := range strings.Fields(line) {
			if strings.Contains(field, "@") && strings.Contains(field, ":") {
				// Extract IP from user@IP:PORT.
				parts := strings.Split(field, "@")
				if len(parts) == 2 {
					ipPort := strings.Split(parts[1], ":")
					if len(ipPort) >= 1 {
						ids = append(ids, ipPort[0])
					}
				}
				break
			}
		}
	}
	return ids
}
