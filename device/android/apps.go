/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package android

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// GetInstalledApps returns the list of third-party package names installed on
// the device, obtained via `adb shell cmd package list packages -3`.
// The `-3` flag limits output to non-system packages.
func (d *AndroidDevice) GetInstalledApps() ([]string, error) {
	out, err := d.cmd.Run(context.Background(), "adb", "-s", d.info.UDID,
		"shell", "cmd", "package", "list", "packages", "-3")
	if err != nil {
		return nil, fmt.Errorf("GetInstalledApps %s: %w", d.info.UDID, err)
	}

	var apps []string
	result := strings.TrimSpace(string(out))
	// Each line has the form "package:<name>". Split on newlines (CRLF or LF).
	lines := regexp.MustCompile(`\r?\n`).Split(result, -1)
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			apps = append(apps, parts[1])
		}
	}
	return apps, nil
}

// InstallApp installs the APK at appPath on the device using `adb install -r`.
// The path is relative to the provider folder — the caller must ensure it
// exists on the host filesystem before calling.
func (d *AndroidDevice) InstallApp(appPath string) error {
	_, err := d.cmd.Run(context.Background(), "adb", "-s", d.info.UDID,
		"install", "-r", appPath)
	if err != nil {
		return fmt.Errorf("InstallApp %s: %w", d.info.UDID, err)
	}
	return nil
}

// UninstallApp removes the package identified by appID from the device using
// `adb uninstall`.
func (d *AndroidDevice) UninstallApp(appID string) error {
	_, err := d.cmd.Run(context.Background(), "adb", "-s", d.info.UDID,
		"uninstall", appID)
	if err != nil {
		return fmt.Errorf("UninstallApp %s: package %s: %w", d.info.UDID, appID, err)
	}
	return nil
}
