/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package tizen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"GADS/common/utils"
)

// TizenApp represents an installed application on a Tizen TV device.
type TizenApp struct {
	AppID       string `json:"appId"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	IsDevApp    bool   `json:"isDevApp"`    // true if installed via dev/sideload
	IsSystemApp bool   `json:"isSystemApp"` // always false (cannot determine reliably)
}

// GetInstalledApps returns the app IDs of all installed apps on the device,
// satisfying the device.AppManager interface. For richer app metadata
// (title, version, dev/system flags) use GetTizenApps instead.
func (d *TizenDevice) GetInstalledApps() ([]string, error) {
	apps, err := d.GetTizenApps()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(apps))
	for _, a := range apps {
		ids = append(ids, a.AppID)
	}
	return ids, nil
}

// GetTizenApps returns the full list of installed apps with metadata by
// running `sdb -s {udid} shell 0 vd_applist` and parsing its output.
func (d *TizenDevice) GetTizenApps() ([]TizenApp, error) {
	apps := []TizenApp{}

	out, err := d.cmd.Run(context.Background(), "sdb", "-s", d.info.UDID, "shell", "0", "vd_applist")
	if err != nil {
		return apps, fmt.Errorf("GetTizenApps %s: %w", d.info.UDID, err)
	}

	lines := strings.Split(string(out), "\n")
	var currentApp TizenApp
	var appType string
	var installedSourceType string
	inAppBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// A separator line (many dashes, no '=') signals the start of a new app block.
		if strings.HasPrefix(trimmed, "----") && len(trimmed) > 50 && !strings.Contains(line, "=") {
			if inAppBlock && currentApp.AppID != "" {
				currentApp.IsDevApp = (appType == "user" || installedSourceType == "0")
				currentApp.IsSystemApp = false
				apps = append(apps, currentApp)
			}
			currentApp = TizenApp{}
			appType = ""
			installedSourceType = ""
			inAppBlock = true
			continue
		}

		if inAppBlock && strings.Contains(line, "=") {
			parts := strings.Split(line, "=")
			if len(parts) >= 2 {
				key := strings.TrimSpace(strings.ReplaceAll(parts[0], "-", ""))
				value := strings.Trim(strings.TrimSpace(parts[1]), "-")

				switch key {
				case "app_tizen_id":
					currentApp.AppID = value
				case "app_title":
					currentApp.Title = value
				case "app_version":
					currentApp.Version = value
				case "type":
					appType = value
				case "installed_source_type":
					installedSourceType = value
				}
			}
		}
	}

	// Flush the last block if present.
	if inAppBlock && currentApp.AppID != "" {
		currentApp.IsDevApp = (appType == "user" || installedSourceType == "0")
		currentApp.IsSystemApp = false
		apps = append(apps, currentApp)
	}

	d.log.LogInfo("tizen_apps", fmt.Sprintf("Found %d installed apps on device %s", len(apps), d.info.UDID))
	return apps, nil
}

// InstallApp extracts the provided .wgt file, re-packages it with the Samsung
// certificate found under ~/SamsungCertificate/, and installs it via `tizen install`.
func (d *TizenDevice) InstallApp(appName string) error {
	certName, err := d.getTizenCertificateName()
	if err != nil {
		return fmt.Errorf("InstallApp %s: %w", d.info.UDID, err)
	}

	if !strings.HasSuffix(appName, ".wgt") {
		return fmt.Errorf("InstallApp %s: unsupported format %q (expected .wgt)", d.info.UDID, appName)
	}

	appPath := filepath.Join(d.cfg.ProviderFolder, appName)
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("tizen_temp_%s", d.info.UDID))

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("InstallApp %s: create temp dir: %w", d.info.UDID, err)
	}
	defer os.RemoveAll(tempDir)

	d.log.LogInfo("tizen_apps", fmt.Sprintf("Extracting .wgt for device %s", d.info.UDID))
	if err := utils.ExtractZipToDir(appPath, tempDir); err != nil {
		return fmt.Errorf("InstallApp %s: extract .wgt: %w", d.info.UDID, err)
	}

	d.log.LogInfo("tizen_apps", fmt.Sprintf("Packaging app with certificate %s for device %s", certName, d.info.UDID))
	if _, err := d.cmd.Run(context.Background(), "tizen", "package", "-t", "wgt", "-s", certName, "--", tempDir); err != nil {
		return fmt.Errorf("InstallApp %s: tizen package: %w", d.info.UDID, err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("InstallApp %s: read temp dir: %w", d.info.UDID, err)
	}

	var wgtFile string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".wgt") {
			wgtFile = filepath.Join(tempDir, entry.Name())
			break
		}
	}
	if wgtFile == "" {
		return fmt.Errorf("InstallApp %s: no .wgt found after packaging", d.info.UDID)
	}

	d.log.LogInfo("tizen_apps", fmt.Sprintf("Installing app on device %s", d.info.UDID))
	if _, err := d.cmd.Run(context.Background(), "tizen", "install", "-n", wgtFile, "-s", d.info.UDID); err != nil {
		return fmt.Errorf("InstallApp %s: tizen install: %w", d.info.UDID, err)
	}

	d.log.LogInfo("tizen_apps", fmt.Sprintf("Successfully installed app on device %s", d.info.UDID))
	return nil
}

// UninstallApp removes the app identified by appID from the Tizen device via
// `tizen uninstall -s {udid} -p {appID}`.
func (d *TizenDevice) UninstallApp(appID string) error {
	if _, err := d.cmd.Run(context.Background(), "tizen", "uninstall", "-s", d.info.UDID, "-p", appID); err != nil {
		return fmt.Errorf("UninstallApp %s: %w", d.info.UDID, err)
	}
	d.log.LogInfo("tizen_apps", fmt.Sprintf("Uninstalled app %s from device %s", appID, d.info.UDID))
	return nil
}

// LaunchApp launches the app identified by appID via `tizen run`.
func (d *TizenDevice) LaunchApp(appID string) error {
	if _, err := d.cmd.Run(context.Background(), "tizen", "run", "-s", d.info.UDID, "-p", appID); err != nil {
		return fmt.Errorf("LaunchApp %s: %w", d.info.UDID, err)
	}
	d.log.LogInfo("tizen_apps", fmt.Sprintf("Launched app %s on device %s", appID, d.info.UDID))
	return nil
}

// CloseApp closes the app identified by appID via `sdb shell 0 was_kill`.
func (d *TizenDevice) CloseApp(appID string) error {
	if _, err := d.cmd.Run(context.Background(), "sdb", "-s", d.info.UDID, "shell", "0", "was_kill", appID); err != nil {
		return fmt.Errorf("CloseApp %s: %w", d.info.UDID, err)
	}
	d.log.LogInfo("tizen_apps", fmt.Sprintf("Closed app %s on device %s", appID, d.info.UDID))
	return nil
}

// getTizenCertificateName locates the first directory under ~/SamsungCertificate/
// and returns its name, which is used as the certificate identifier for `tizen package`.
func (d *TizenDevice) getTizenCertificateName() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getTizenCertificateName: get home dir: %w", err)
	}

	certDir := filepath.Join(homeDir, "SamsungCertificate")
	entries, err := os.ReadDir(certDir)
	if err != nil {
		return "", fmt.Errorf("getTizenCertificateName: read %s (configure certificate per docs): %w", certDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			d.log.LogInfo("tizen_apps", fmt.Sprintf("Using Tizen certificate: %s", entry.Name()))
			return entry.Name(), nil
		}
	}

	return "", fmt.Errorf("getTizenCertificateName: no certificate directory in %s (configure per docs)", certDir)
}
