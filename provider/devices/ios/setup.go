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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"GADS/common/cli"
	"GADS/common/models"

	"github.com/Masterminds/semver"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/pelletier/go-toml/v2"
)

// Setup runs the full provisioning sequence for an iOS device. It blocks until
// the device is live or an error forces a reset. ctx is used to cancel
// in-progress work when the device disconnects.
//
// Provisioning steps:
//  1. Parse OS semver.
//  2. Get go-ios DeviceEntry.
//  3. Pair (with optional pair-record cache restore).
//  4. Check developer mode (iOS 16+).
//  5. Mount DDI.
//  6. Read hardware model and product type from plist.
//  7. Update screen dimensions (if not in DB).
//  8. Allocate userspace tunnel port (iOS 17.4+).
//  9. Create userspace tunnel and update DeviceEntry with RSD provider (iOS 17.4+).
//  10. Disable broadcast extension memory limit (if broadcast extension streaming).
//  11. Allocate ports (WDA, stream, WDA stream).
//  12. Forward ports via go-ios.
//  13. Install and/or launch WDA; wait for readiness.
//  14. Apply stream settings; update WDA MJPEG settings.
//  15. Optionally start Appium.
//  16. Collect installed apps.
//  17. Mark device state "live".
func (d *IOSDevice) Setup(ctx context.Context) error {
	d.setupMu.Lock()
	defer d.setupMu.Unlock()

	d.info.ProviderState = "preparing"
	d.log.LogInfo("ios_setup", fmt.Sprintf("Starting setup for device %s", d.info.UDID))

	// Reset the readiness channel in case Setup is called more than once.
	d.wdaReadyChan = make(chan bool, 1)

	// --- Step 1: parse semver ---
	sv, err := semver.NewVersion(d.info.OSVersion)
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: parse semver %q: %w", d.info.UDID, d.info.OSVersion, err))
	}
	d.semVer = sv

	// --- Step 2: get go-ios DeviceEntry ---
	entry, err := ios.GetDevice(d.info.UDID)
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: get device entry: %w", d.info.UDID, err))
	}
	d.goIOSEntry = entry

	// --- Step 3: pair ---
	if err := d.pair(); err != nil {
		return d.fail(fmt.Errorf("Setup %s: pair: %w", d.info.UDID, err))
	}

	// --- Step 4: developer mode check (iOS 16+) ---
	if d.semVer.Major() >= 16 {
		devModeEnabled, err := imagemounter.IsDevModeEnabled(d.goIOSEntry)
		if err != nil {
			return d.fail(fmt.Errorf("Setup %s: check developer mode: %w", d.info.UDID, err))
		}
		if !devModeEnabled {
			return d.fail(fmt.Errorf("Setup %s: developer mode is not enabled (iOS 16+ requires it)", d.info.UDID))
		}
	}

	// --- Step 5: mount DDI ---
	d.mountDDI()

	// --- Step 6: hardware model and product type ---
	plist, err := ios.GetValuesPlist(d.goIOSEntry)
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: get values plist: %w", d.info.UDID, err))
	}
	d.info.HardwareModel = plist["HardwareModel"].(string)
	productType := plist["ProductType"].(string)

	// --- Step 7: screen dimensions ---
	if d.info.ScreenHeight == "" || d.info.ScreenWidth == "" {
		if err := d.updateScreenSize(productType); err != nil {
			return d.fail(fmt.Errorf("Setup %s: screen size: %w", d.info.UDID, err))
		}
	}

	// --- Step 8: allocate userspace tunnel port ---
	// The tunnel port is set on the DeviceEntry before the tunnel is created.
	tunnelPort, err := d.ports.GetFreePort()
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: allocate tunnel port: %w", d.info.UDID, err))
	}
	tunnelPortInt, _ := strconv.Atoi(tunnelPort)
	d.goIOSEntry.UserspaceTUNPort = tunnelPortInt

	// --- Step 9: userspace tunnel (iOS 17.4+) ---
	v174 := semver.MustParse("17.4.0")
	if d.semVer.Compare(v174) >= 0 {
		tun, err := d.createTunnel()
		if err != nil {
			return d.fail(fmt.Errorf("Setup %s: create tunnel: %w", d.info.UDID, err))
		}
		d.goIOSTunnel = tun
		d.goIOSEntry.UserspaceTUNPort = tun.UserspaceTUNPort
		d.goIOSEntry.UserspaceTUN = tun.UserspaceTUN

		if err := d.goIosDeviceWithRsdProvider(); err != nil {
			return d.fail(fmt.Errorf("Setup %s: RSD provider: %w", d.info.UDID, err))
		}
	}

	time.Sleep(1 * time.Second)

	// --- Step 10: disable broadcast extension memory limit ---
	if d.info.StreamType == models.IOSWebRTCBroadcastExtensionID {
		pid, err := d.getProcessPid("gads-broadcast-extension")
		if err != nil {
			d.log.LogWarn("ios_setup", fmt.Sprintf("Could not get broadcast extension PID for %s: %v", d.info.UDID, err))
		} else {
			if err := d.disableProcessMemoryLimit(pid); err != nil {
				d.log.LogWarn("ios_setup", fmt.Sprintf("Could not disable broadcast extension memory limit for %s: %v", d.info.UDID, err))
			}
		}
	}

	// --- Step 11: allocate ports ---
	wdaPort, err := d.ports.GetFreePort()
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: allocate WDA port: %w", d.info.UDID, err))
	}
	d.info.WDAPort = wdaPort

	streamPort, err := d.ports.GetFreePort()
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: allocate stream port: %w", d.info.UDID, err))
	}
	d.info.StreamPort = streamPort

	wdaStreamPort, err := d.ports.GetFreePort()
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: allocate WDA stream port: %w", d.info.UDID, err))
	}
	d.info.WDAStreamPort = wdaStreamPort

	// --- Step 12: forward ports ---
	go d.goIosForward(ctx, d.info.WDAPort, "8100")
	go d.goIosForward(ctx, d.info.StreamPort, "8765")
	go d.goIosForward(ctx, d.info.WDAStreamPort, "9100")

	// --- Step 13: install/launch WDA and wait for readiness ---
	// For iOS < 17 and iOS >= 17.4 (tunnel devices): install WDA via zipconduit
	// and run via testmanagerd. For iOS 17.0-17.3: WDA is already installed,
	// just launch it.
	v17 := semver.MustParse("17.0.0")
	if d.semVer.Major() < 17 || d.semVer.Compare(v174) >= 0 {
		wdaPath := filepath.Join(d.cfg.ProviderFolder, "WebDriverAgent.ipa")
		if err := d.InstallApp(wdaPath); err != nil {
			return d.fail(fmt.Errorf("Setup %s: install WDA: %w", d.info.UDID, err))
		}
		go d.runWDA(ctx)
	} else if d.semVer.Compare(v17) >= 0 {
		if err := d.LaunchApp(d.cfg.WdaBundleID, true); err != nil {
			return d.fail(fmt.Errorf("Setup %s: launch WDA: %w", d.info.UDID, err))
		}
	}

	go d.checkWDAUp()
	select {
	case <-d.wdaReadyChan:
		d.log.LogInfo("ios_setup", fmt.Sprintf("WDA is up for device %s on port %s", d.info.UDID, d.info.WDAPort))
	case <-time.After(60 * time.Second):
		return d.fail(fmt.Errorf("Setup %s: WDA did not start within 60 seconds", d.info.UDID))
	}

	// --- Step 14: stream settings ---
	if err := d.applyStreamSettings(); err != nil {
		return d.fail(fmt.Errorf("Setup %s: apply stream settings: %w", d.info.UDID, err))
	}
	if err := d.UpdateStreamSettings(); err != nil {
		return d.fail(fmt.Errorf("Setup %s: update WDA stream settings: %w", d.info.UDID, err))
	}

	// --- Step 15: Appium (optional) ---
	if d.cfg.SetupAppiumServers {
		if err := d.setupAppium(ctx); err != nil {
			return d.fail(err)
		}
	}

	// --- Step 16: installed apps ---
	apps, err := d.GetInstalledApps()
	if err != nil {
		d.log.LogWarn("ios_setup", fmt.Sprintf("Could not list installed apps for %s: %v", d.info.UDID, err))
	} else {
		d.info.InstalledApps = apps
	}

	// --- Step 17: mark live ---
	d.info.ProviderState = "live"
	d.log.LogInfo("ios_setup", fmt.Sprintf("Device %s is live", d.info.UDID))
	return nil
}

// Reset cancels in-progress services, closes the tunnel (if any), frees all
// allocated ports, and returns the device to the "init" state. It is
// idempotent.
func (d *IOSDevice) Reset(reason string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.info.IsResetting || d.info.ProviderState == "init" {
		return
	}

	d.log.LogInfo("ios_reset", fmt.Sprintf("Resetting device %s: %s", d.info.UDID, reason))
	d.info.IsResetting = true

	// Close the userspace tunnel if one was established.
	if d.goIOSTunnel.Address != "" {
		d.goIOSTunnel.Close()
	}

	d.ports.FreePort(d.info.WDAPort)
	d.ports.FreePort(d.info.WDAStreamPort)
	d.ports.FreePort(d.info.StreamPort)
	d.ports.FreePort(d.info.AppiumPort)

	d.info.WDAPort = ""
	d.info.WDAStreamPort = ""
	d.info.StreamPort = ""
	d.info.AppiumPort = ""

	d.info.ProviderState = "init"
	d.info.IsResetting = false
}

// pair runs the pairing sequence with optional pair-record cache support.
// If UseIOSPairCache is enabled and a cached record is available, it is
// restored to usbmuxd first (avoids the "Trust this computer?" prompt).
// On successful pairing the record is cached for next time.
func (d *IOSDevice) pair() (pairErr error) {
	if d.cfg.UseIOSPairCache {
		if err := d.restorePairRecord(); err == nil {
			d.log.LogInfo("ios_setup",
				fmt.Sprintf("Restored cached pair record for %s, skipping pairing", d.info.UDID))
			return nil
		}
	}

	defer func() {
		if pairErr == nil && d.cfg.UseIOSPairCache {
			d.cachePairRecord()
		}
	}()

	p12Path := filepath.Join(d.cfg.ProviderFolder, "supervision.p12")
	p12, err := os.ReadFile(p12Path)
	if err != nil {
		// No supervision profile — fall back to unsupervised pairing.
		d.log.LogWarn("ios_setup",
			fmt.Sprintf("No supervision.p12 for %s, unsupervised pairing", d.info.UDID))
		if err := ios.Pair(d.goIOSEntry); err != nil {
			return fmt.Errorf("unsupervised pair: %w", err)
		}
		return nil
	}

	if d.cfg.SupervisionPassword == "" {
		d.log.LogInfo("ios_setup",
			fmt.Sprintf("supervision.p12 present but no password for %s, unsupervised pairing", d.info.UDID))
		if err := ios.Pair(d.goIOSEntry); err != nil {
			return fmt.Errorf("unsupervised pair: %w", err)
		}
		return nil
	}

	if err := ios.PairSupervised(d.goIOSEntry, p12, d.cfg.SupervisionPassword); err != nil {
		d.log.LogWarn("ios_setup",
			fmt.Sprintf("Supervised pair failed for %s: %v, falling back to unsupervised", d.info.UDID, err))
		if err := ios.Pair(d.goIOSEntry); err != nil {
			return fmt.Errorf("unsupervised pair fallback: %w", err)
		}
	}
	return nil
}

// mountDDI downloads (if needed) and mounts the Developer Disk Image for the
// device. Failures are non-fatal — logged as errors and the device is reset.
func (d *IOSDevice) mountDDI() {
	basedir := filepath.Join(d.cfg.ProviderFolder, "devimages")
	path, err := imagemounter.DownloadImageFor(d.goIOSEntry, basedir)
	if err != nil {
		d.log.LogError("ios_setup",
			fmt.Sprintf("Failed to download DDI for %s: %v", d.info.UDID, err))
		d.Reset("Failed to download DDI")
		return
	}
	if err := imagemounter.MountImage(d.goIOSEntry, path); err != nil {
		d.log.LogError("ios_setup",
			fmt.Sprintf("Failed to mount DDI for %s: %v", d.info.UDID, err))
		d.Reset("Failed to mount DDI")
	}
}

// applyStreamSettings loads per-device stream settings from the store,
// falling back to global settings if none are configured.
func (d *IOSDevice) applyStreamSettings() error {
	deviceSettings, err := d.store.GetDeviceStreamSettings(d.info.UDID)
	if err != nil {
		globalSettings, gErr := d.store.GetGlobalStreamSettings()
		if gErr != nil {
			return fmt.Errorf("applyStreamSettings %s: global: %w", d.info.UDID, gErr)
		}
		d.info.StreamTargetFPS = globalSettings.TargetFPS
		d.info.StreamJpegQuality = globalSettings.JpegQuality
		d.info.StreamScalingFactor = globalSettings.ScalingFactoriOS
		return nil
	}
	d.info.StreamTargetFPS = deviceSettings.StreamTargetFPS
	d.info.StreamJpegQuality = deviceSettings.StreamJpegQuality
	d.info.StreamScalingFactor = deviceSettings.StreamScalingFactor
	return nil
}

// setupAppium kills stale processes, allocates a port, starts Appium, and
// waits up to 30 seconds for it to become healthy.
func (d *IOSDevice) setupAppium(ctx context.Context) error {
	if err := cli.KillDeviceAppiumProcess(d.info.UDID); err != nil {
		return fmt.Errorf("setupAppium %s: kill existing processes: %w", d.info.UDID, err)
	}

	appiumPort, err := d.ports.GetFreePort()
	if err != nil {
		return fmt.Errorf("setupAppium %s: allocate port: %w", d.info.UDID, err)
	}
	d.info.AppiumPort = appiumPort

	go d.startAppium(ctx)

	timeout := time.After(30 * time.Second)
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-timeout:
			return fmt.Errorf("setupAppium %s: timed out waiting for Appium", d.info.UDID)
		case <-tick.C:
			if d.info.IsAppiumUp {
				d.log.LogInfo("ios_setup",
					fmt.Sprintf("Appium is up for device %s on port %s", d.info.UDID, d.info.AppiumPort))
				goto appiumDone
			}
		}
	}
appiumDone:

	if d.cfg.UseSeleniumGrid {
		if err := d.createGridTOML(); err != nil {
			return fmt.Errorf("setupAppium %s: create grid TOML: %w", d.info.UDID, err)
		}
		go d.startGridNode(ctx)
	}
	return nil
}

// startAppium launches the Appium server for this iOS device using XCUITest
// automation. It runs in a goroutine and resets the device if the process exits.
func (d *IOSDevice) startAppium(ctx context.Context) {
	caps := models.AppiumServerCapabilities{
		UDID:                  d.info.UDID,
		WdaURL:                "http://localhost:" + d.info.WDAPort,
		WdaLocalPort:          d.info.WDAPort,
		WdaLaunchTimeout:      "120000",
		WdaConnectionTimeout:  "240000",
		ClearSystemFiles:      "false",
		PreventWdaAttachments: "true",
		SimpleIsVisibleCheck:  "false",
		AutomationName:        "XCUITest",
		PlatformName:          "iOS",
		DeviceName:            d.info.Name,
	}
	capsJSON, _ := json.Marshal(caps)

	pluginCfg := models.AppiumPluginConfiguration{
		ProviderUrl:       fmt.Sprintf("http://%s:%v", d.cfg.HostAddress, d.cfg.Port),
		HeartBeatInterval: "2000",
		UDID:              d.info.UDID,
	}
	pluginCfgJSON, _ := json.Marshal(pluginCfg)

	proc, err := d.cmd.Start(ctx, "appium",
		"-p", d.info.AppiumPort,
		"--log-timestamp",
		"--use-plugin=gads",
		fmt.Sprintf("--plugin-gads-config=%s", string(pluginCfgJSON)),
		"--session-override",
		"--log-no-colors",
		"--relaxed-security",
		"--default-capabilities", string(capsJSON),
	)
	if err != nil {
		d.log.LogError("ios_setup", fmt.Sprintf("Failed to start Appium for %s: %v", d.info.UDID, err))
		d.Reset("Appium failed to start")
		return
	}
	<-proc.Done
	d.Reset("Appium process exited unexpectedly")
}

// createGridTOML writes a Selenium Grid TOML file for this device.
func (d *IOSDevice) createGridTOML() error {
	url := fmt.Sprintf("http://%s:%v/device/%s/appium",
		d.cfg.HostAddress, d.cfg.Port, d.info.UDID)
	caps := fmt.Sprintf(
		`{"appium:deviceName": "%s", "platformName": "%s", "appium:platformVersion": "%s", "appium:automationName": "XCUITest", "appium:udid": "%s"}`,
		d.info.Name, d.info.OS, d.info.OSVersion, d.info.UDID)

	gridPort, err := d.ports.GetFreePort()
	if err != nil {
		return fmt.Errorf("createGridTOML %s: allocate port: %w", d.info.UDID, err)
	}
	gridPortInt := 0
	fmt.Sscanf(gridPort, "%d", &gridPortInt)

	conf := models.AppiumTomlConfig{
		Server: models.AppiumTomlServer{Port: gridPortInt},
		Node:   models.AppiumTomlNode{DetectDrivers: false},
		Relay: models.AppiumTomlRelay{
			URL:            url,
			StatusEndpoint: "/status",
			Configs:        []string{"1", caps},
		},
	}
	data, err := toml.Marshal(conf)
	if err != nil {
		return fmt.Errorf("createGridTOML %s: marshal: %w", d.info.UDID, err)
	}

	tomlPath := filepath.Join(d.cfg.ProviderFolder, d.info.UDID+".toml")
	f, err := os.Create(tomlPath)
	if err != nil {
		return fmt.Errorf("createGridTOML %s: create file: %w", d.info.UDID, err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("createGridTOML %s: write: %w", d.info.UDID, err)
	}
	return nil
}

// startGridNode starts the Selenium Grid node process for this device.
func (d *IOSDevice) startGridNode(ctx context.Context) {
	time.Sleep(5 * time.Second)
	proc, err := d.cmd.Start(ctx, "java",
		"-jar", filepath.Join(d.cfg.ProviderFolder, "selenium.jar"),
		"node",
		"--host", d.cfg.HostAddress,
		"--config", filepath.Join(d.cfg.ProviderFolder, d.info.UDID+".toml"),
		"--grid-url", d.cfg.SeleniumGrid,
	)
	if err != nil {
		d.log.LogError("ios_setup",
			fmt.Sprintf("Failed to start Selenium Grid node for %s: %v", d.info.UDID, err))
		d.Reset("Selenium Grid node failed to start")
		return
	}
	<-proc.Done
	d.Reset("Selenium Grid node exited unexpectedly")
}

// getProcessPid returns the PID of the first running process named processName
// on the device, using go-ios instruments.
func (d *IOSDevice) getProcessPid(processName string) (uint64, error) {
	svc, err := instruments.NewDeviceInfoService(d.goIOSEntry)
	if err != nil {
		return 0, fmt.Errorf("getProcessPid %s: create service: %w", d.info.UDID, err)
	}
	defer svc.Close()

	processes, err := svc.ProcessList()
	if err != nil {
		return 0, fmt.Errorf("getProcessPid %s: list processes: %w", d.info.UDID, err)
	}
	for _, p := range processes {
		if p.Pid > 1 && p.Name == processName {
			return p.Pid, nil
		}
	}
	return 0, fmt.Errorf("getProcessPid %s: process %q not found", d.info.UDID, processName)
}

// disableProcessMemoryLimit disables the Jetsam memory limit for the given
// PID via go-ios instruments process control.
func (d *IOSDevice) disableProcessMemoryLimit(pid uint64) error {
	pControl, err := instruments.NewProcessControl(d.goIOSEntry)
	if err != nil {
		return fmt.Errorf("disableProcessMemoryLimit %s: create control: %w", d.info.UDID, err)
	}
	disabled, err := pControl.DisableMemoryLimit(pid)
	if err != nil {
		return fmt.Errorf("disableProcessMemoryLimit %s: pid %d: %w", d.info.UDID, pid, err)
	}
	if !disabled {
		return fmt.Errorf("disableProcessMemoryLimit %s: pid %d: reported not disabled", d.info.UDID, pid)
	}
	return nil
}

// fail logs the error, calls Reset, and returns the error.
func (d *IOSDevice) fail(err error) error {
	d.log.LogError("ios_setup", err.Error())
	d.Reset(err.Error())
	return err
}
