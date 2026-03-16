/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package webos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"GADS/common/utils"
)

// WebOSApp represents an installed application on a WebOS TV device.
type WebOSApp struct {
	AppID     string `json:"appId"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	IsDevApp  bool   `json:"isDevApp"`  // always true (ares-install lists only dev apps)
	SystemApp bool   `json:"systemApp"` // parsed from output (always false for CLI apps)
}

// GetInstalledApps returns the list of apps installed on the WebOS device by
// running `ares-install --device {name} --listfull` and parsing its output.
func (d *WebOSDevice) GetInstalledApps() ([]WebOSApp, error) {
	apps := []WebOSApp{}

	out, err := d.cmd.Run(context.Background(), "ares-install", "--device", d.info.Name, "--listfull")
	if err != nil {
		return apps, fmt.Errorf("GetInstalledApps %s: %w", d.info.UDID, err)
	}

	lines := strings.Split(string(out), "\n")
	var currentApp WebOSApp

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			if currentApp.AppID != "" {
				currentApp.IsDevApp = true
				apps = append(apps, currentApp)
				currentApp = WebOSApp{}
			}
			continue
		}

		if strings.Contains(line, " : ") {
			parts := strings.SplitN(line, " : ", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "id":
					currentApp.AppID = value
				case "title":
					currentApp.Title = value
				case "version":
					currentApp.Version = value
				case "systemApp":
					currentApp.SystemApp = (value == "true")
				}
			}
		}
	}

	// Flush the last app if present.
	if currentApp.AppID != "" {
		currentApp.IsDevApp = true
		apps = append(apps, currentApp)
	}

	d.log.LogInfo("webos_apps", fmt.Sprintf("Found %d installed apps on device %s", len(apps), d.info.UDID))
	return apps, nil
}

// InstallApp installs a .ipk file directly, or extracts + packages a zip and
// installs the resulting .ipk via the ares-install and ares-package CLI tools.
func (d *WebOSDevice) InstallApp(appName string) error {
	appPath := filepath.Join(d.cfg.ProviderFolder, appName)

	if strings.HasSuffix(appName, ".ipk") {
		d.log.LogInfo("webos_apps", fmt.Sprintf("Installing .ipk directly on device %s", d.info.UDID))
		if _, err := d.cmd.Run(context.Background(), "ares-install", "--device", d.info.Name, appPath); err != nil {
			return fmt.Errorf("InstallApp %s: ares-install .ipk: %w", d.info.UDID, err)
		}
		d.log.LogInfo("webos_apps", fmt.Sprintf("Successfully installed app on device %s", d.info.UDID))
		return nil
	}

	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("webos_temp_%s", d.info.UDID))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("InstallApp %s: create temp dir: %w", d.info.UDID, err)
	}
	defer os.RemoveAll(tempDir)

	d.log.LogInfo("webos_apps", fmt.Sprintf("Extracting source for device %s", d.info.UDID))
	if err := utils.ExtractZipToDir(appPath, tempDir); err != nil {
		return fmt.Errorf("InstallApp %s: extract: %w", d.info.UDID, err)
	}

	d.log.LogInfo("webos_apps", fmt.Sprintf("Packaging app for device %s", d.info.UDID))
	if _, err := d.cmd.Run(context.Background(), "ares-package", tempDir); err != nil {
		return fmt.Errorf("InstallApp %s: ares-package: %w", d.info.UDID, err)
	}

	// ares-package writes the .ipk to the current working directory.
	entries, err := os.ReadDir(".")
	if err != nil {
		return fmt.Errorf("InstallApp %s: read cwd: %w", d.info.UDID, err)
	}

	var ipkFile string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".ipk") {
			ipkFile = entry.Name()
			break
		}
	}
	if ipkFile == "" {
		return fmt.Errorf("InstallApp %s: no .ipk found after packaging", d.info.UDID)
	}
	defer os.Remove(ipkFile)

	d.log.LogInfo("webos_apps", fmt.Sprintf("Installing packaged app on device %s", d.info.UDID))
	if _, err := d.cmd.Run(context.Background(), "ares-install", "--device", d.info.Name, ipkFile); err != nil {
		return fmt.Errorf("InstallApp %s: ares-install packaged: %w", d.info.UDID, err)
	}

	d.log.LogInfo("webos_apps", fmt.Sprintf("Successfully installed app on device %s", d.info.UDID))
	return nil
}

// UninstallApp removes the app identified by appID via `ares-install --remove`.
func (d *WebOSDevice) UninstallApp(appID string) error {
	if _, err := d.cmd.Run(context.Background(), "ares-install", "--device", d.info.Name, "--remove", appID); err != nil {
		return fmt.Errorf("UninstallApp %s: %w", d.info.UDID, err)
	}
	d.log.LogInfo("webos_apps", fmt.Sprintf("Uninstalled app %s from device %s", appID, d.info.UDID))
	return nil
}

// LaunchApp launches the app identified by appID via `ares-launch`.
func (d *WebOSDevice) LaunchApp(appID string) error {
	if _, err := d.cmd.Run(context.Background(), "ares-launch", "--device", d.info.Name, appID); err != nil {
		return fmt.Errorf("LaunchApp %s: %w", d.info.UDID, err)
	}
	d.log.LogInfo("webos_apps", fmt.Sprintf("Launched app %s on device %s", appID, d.info.UDID))
	return nil
}

// CloseApp closes the app identified by appID via `ares-launch --close`.
func (d *WebOSDevice) CloseApp(appID string) error {
	if _, err := d.cmd.Run(context.Background(), "ares-launch", "--device", d.info.Name, "--close", appID); err != nil {
		return fmt.Errorf("CloseApp %s: %w", d.info.UDID, err)
	}
	d.log.LogInfo("webos_apps", fmt.Sprintf("Closed app %s on device %s", appID, d.info.UDID))
	return nil
}
