/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package ios

import (
	"fmt"
	"strings"

	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
)

// GetInstalledApps returns the list of user-installed app bundle IDs on the
// device, obtained via the go-ios installationproxy service.
func (d *IOSDevice) GetInstalledApps() ([]string, error) {
	svc, err := installationproxy.New(d.goIOSEntry)
	if err != nil {
		return nil, fmt.Errorf("GetInstalledApps %s: create proxy: %w", d.info.UDID, err)
	}

	resp, err := svc.BrowseUserApps()
	if err != nil {
		return nil, fmt.Errorf("GetInstalledApps %s: browse: %w", d.info.UDID, err)
	}

	apps := make([]string, 0, len(resp))
	for _, app := range resp {
		apps = append(apps, app.CFBundleIdentifier())
	}
	return apps, nil
}

// InstallApp installs the IPA at appPath on the device using the go-ios
// zipconduit service. On Windows the "./" prefix must be stripped from the
// path before sending.
func (d *IOSDevice) InstallApp(appPath string) error {
	if d.cfg.OS == "windows" {
		appPath = strings.TrimPrefix(appPath, "./")
	}

	conn, err := zipconduit.New(d.goIOSEntry)
	if err != nil {
		return fmt.Errorf("InstallApp %s: create zipconduit: %w", d.info.UDID, err)
	}
	if err := conn.SendFile(appPath); err != nil {
		return fmt.Errorf("InstallApp %s: send file: %w", d.info.UDID, err)
	}
	return nil
}

// UninstallApp removes the app identified by bundleID using the go-ios
// installationproxy service.
func (d *IOSDevice) UninstallApp(bundleID string) error {
	svc, err := installationproxy.New(d.goIOSEntry)
	if err != nil {
		return fmt.Errorf("UninstallApp %s: create proxy: %w", d.info.UDID, err)
	}
	if err := svc.Uninstall(bundleID); err != nil {
		return fmt.Errorf("UninstallApp %s: uninstall %s: %w", d.info.UDID, bundleID, err)
	}
	return nil
}

// LaunchApp starts the app with the given bundleID on the device using
// go-ios instruments process control. If killExisting is true, any running
// instance is terminated first.
func (d *IOSDevice) LaunchApp(bundleID string, killExisting bool) error {
	pControl, err := instruments.NewProcessControl(d.goIOSEntry)
	if err != nil {
		return fmt.Errorf("LaunchApp %s: create process control: %w", d.info.UDID, err)
	}

	opts := map[string]any{}
	if killExisting {
		opts["KillExisting"] = 1
	}
	if _, err := pControl.LaunchAppWithArgs(bundleID, nil, nil, opts); err != nil {
		return fmt.Errorf("LaunchApp %s: launch %s: %w", d.info.UDID, bundleID, err)
	}
	return nil
}
