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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"GADS/common/cli"
	"GADS/common/models"

	"github.com/Masterminds/semver"
	"github.com/pelletier/go-toml/v2"
)

// Setup runs the full provisioning sequence for an Android device. It must be
// called from a goroutine — it blocks until the device is live or an error
// forces a reset. ctx is used to cancel in-progress work when the device
// disconnects.
//
// Provisioning steps:
//  1. Parse OS semver for version-gated behaviour.
//  2. Collect hardware model.
//  3. Update screen dimensions from ADB (if not set in DB).
//  4. Disable auto-rotation.
//  5. Allocate host ports for stream, IME, and remote control server.
//  6. Get installed apps; uninstall legacy GADS packages.
//  7. Install GADS-Settings APK and push to /data/local/tmp.
//  8. Start the remote control server.
//  9. Start the video stream service.
//  10. Forward device ports to host ports.
//  11. If WebRTC: push TURN credentials to the stream service.
//  12. Setup GADS IME.
//  13. Apply stream settings and push to device.
//  14. Optionally start Appium.
//  15. Mark device state "live".
func (d *AndroidDevice) Setup(ctx context.Context) error {
	d.setupMu.Lock()
	defer d.setupMu.Unlock()

	d.info.ProviderState = "preparing"
	d.log.LogInfo("android_setup", fmt.Sprintf("Starting setup for device %s", d.info.UDID))

	// --- Step 1: parse semver ---
	sv, err := semver.NewVersion(d.info.OSVersion)
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: parse semver %q: %w", d.info.UDID, d.info.OSVersion, err))
	}
	d.semVer = sv

	// --- Step 2: hardware model ---
	d.getHardwareModel(ctx)

	// --- Step 3: screen dimensions ---
	if d.info.ScreenHeight == "" || d.info.ScreenWidth == "" {
		if err := d.updateScreenSize(ctx); err != nil {
			return d.fail(fmt.Errorf("Setup %s: screen size: %w", d.info.UDID, err))
		}
	}

	// --- Step 4: disable auto-rotation ---
	if err := d.disableAutoRotation(ctx); err != nil {
		return d.fail(fmt.Errorf("Setup %s: disable auto-rotation: %w", d.info.UDID, err))
	}

	// --- Step 5: allocate ports ---
	streamPort, err := d.ports.GetFreePort()
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: allocate stream port: %w", d.info.UDID, err))
	}
	d.info.StreamPort = streamPort

	imePort, err := d.ports.GetFreePort()
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: allocate IME port: %w", d.info.UDID, err))
	}
	d.imePort = imePort

	remotePort, err := d.ports.GetFreePort()
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: allocate remote server port: %w", d.info.UDID, err))
	}
	d.remoteServerPort = remotePort

	// --- Step 6: installed apps / uninstall legacy GADS packages ---
	installedApps, err := d.GetInstalledApps()
	if err != nil {
		d.log.LogWarn("android_setup", fmt.Sprintf("Could not list installed apps for %s: %v", d.info.UDID, err))
		installedApps = []string{}
	}
	d.info.InstalledApps = installedApps

	legacyPackages := []string{
		"com.gads.settings",
		"com.gads.webrtc",
		"com.shamanec.stream",
		"com.gads.gads_ime",
	}
	for _, pkg := range legacyPackages {
		if slices.Contains(installedApps, pkg) {
			if err := d.UninstallApp(pkg); err != nil {
				return d.fail(fmt.Errorf("Setup %s: uninstall %s: %w", d.info.UDID, pkg, err))
			}
			time.Sleep(3 * time.Second)
		}
	}

	// --- Step 7: install GADS-Settings and push to /data/local/tmp ---
	apkPath := filepath.Join(d.cfg.ProviderFolder, "gads-settings.apk")
	if _, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID, "install", "-r", apkPath); err != nil {
		return d.fail(fmt.Errorf("Setup %s: install GADS-Settings: %w", d.info.UDID, err))
	}
	if _, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID, "push", apkPath, "/data/local/tmp/gads-settings"); err != nil {
		return d.fail(fmt.Errorf("Setup %s: push GADS-Settings: %w", d.info.UDID, err))
	}
	time.Sleep(2 * time.Second)

	// --- Step 8: start remote control server ---
	// Kill any existing instance first, then start fresh.
	_, _ = d.cmd.Run(ctx, "adb", "-s", d.info.UDID, "shell", "pkill", "-f", "RemoteControlServerKt")
	time.Sleep(1 * time.Second)

	remoteProc, err := d.cmd.Start(ctx, "adb", "-s", d.info.UDID, "shell",
		"CLASSPATH=/data/local/tmp/gads-settings app_process / com.gads.settings.RemoteControlServerKt 1994")
	if err != nil {
		return d.fail(fmt.Errorf("Setup %s: start remote control server: %w", d.info.UDID, err))
	}
	go func() {
		<-remoteProc.Done
		d.Reset("GADS remote control server exited unexpectedly")
	}()
	time.Sleep(2 * time.Second)

	// --- Step 9: start video stream ---
	if err := d.startStream(ctx); err != nil {
		return d.fail(err)
	}
	time.Sleep(2 * time.Second)

	// --- Step 10: forward ports ---
	if err := d.forwardPort(ctx, d.info.StreamPort, "1991"); err != nil {
		return d.fail(fmt.Errorf("Setup %s: forward stream port: %w", d.info.UDID, err))
	}
	if err := d.forwardPort(ctx, d.imePort, "1993"); err != nil {
		return d.fail(fmt.Errorf("Setup %s: forward IME port: %w", d.info.UDID, err))
	}
	if err := d.forwardPort(ctx, d.remoteServerPort, "1994"); err != nil {
		return d.fail(fmt.Errorf("Setup %s: forward remote server port: %w", d.info.UDID, err))
	}

	// --- Step 11: push TURN config (WebRTC only) ---
	if models.IsWebRTCStreamType(d.info.StreamType) {
		if err := d.updateWebRTCTURNConfig(); err != nil {
			// Non-fatal: WebRTC will fall back to STUN-only.
			d.log.LogWarn("android_setup",
				fmt.Sprintf("Could not send TURN config to %s: %v (STUN-only fallback)", d.info.UDID, err))
		}
	}

	// --- Step 12: setup GADS IME ---
	if err := d.setupIME(ctx); err != nil {
		return d.fail(fmt.Errorf("Setup %s: IME: %w", d.info.UDID, err))
	}

	// --- Step 13: apply and push stream settings ---
	if err := d.applyStreamSettings(); err != nil {
		return d.fail(fmt.Errorf("Setup %s: apply stream settings: %w", d.info.UDID, err))
	}
	if err := d.UpdateStreamSettings(); err != nil {
		return d.fail(fmt.Errorf("Setup %s: update stream settings: %w", d.info.UDID, err))
	}

	// --- Step 14: Appium (optional) ---
	if d.cfg.SetupAppiumServers {
		if err := d.setupAppium(ctx, installedApps); err != nil {
			return d.fail(err)
		}
	}

	// --- Step 15: mark live ---
	d.info.ProviderState = "live"
	d.log.LogInfo("android_setup", fmt.Sprintf("Device %s is live", d.info.UDID))
	return nil
}

// Reset cancels the device context, frees all allocated ports, and transitions
// the device back to the "init" state so it can be provisioned again.
// reason is logged for diagnostics. Reset is idempotent — calling it while
// already resetting or in "init" state is a no-op.
func (d *AndroidDevice) Reset(reason string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.info.IsResetting || d.info.ProviderState == "init" {
		return
	}

	d.log.LogInfo("android_reset", fmt.Sprintf("Resetting device %s: %s", d.info.UDID, reason))
	d.info.IsResetting = true

	// Free allocated ports so other devices can reuse them.
	d.ports.FreePort(d.info.StreamPort)
	d.ports.FreePort(d.imePort)
	d.ports.FreePort(d.remoteServerPort)
	d.ports.FreePort(d.info.AppiumPort)

	d.info.StreamPort = ""
	d.imePort = ""
	d.remoteServerPort = ""
	d.info.AppiumPort = ""

	d.info.ProviderState = "init"
	d.info.IsResetting = false
}

// applyStreamSettings loads stream settings from the store (per-device first,
// then global fallback) and stores them in d.info.
func (d *AndroidDevice) applyStreamSettings() error {
	deviceSettings, err := d.store.GetDeviceStreamSettings(d.info.UDID)
	if err != nil {
		// Fall back to global settings when per-device settings are absent.
		globalSettings, gErr := d.store.GetGlobalStreamSettings()
		if gErr != nil {
			return fmt.Errorf("applyStreamSettings %s: global: %w", d.info.UDID, gErr)
		}
		d.info.StreamTargetFPS = globalSettings.TargetFPS
		d.info.StreamJpegQuality = globalSettings.JpegQuality
		d.info.StreamScalingFactor = globalSettings.ScalingFactorAndroid
		return nil
	}
	d.info.StreamTargetFPS = deviceSettings.StreamTargetFPS
	d.info.StreamJpegQuality = deviceSettings.StreamJpegQuality
	d.info.StreamScalingFactor = deviceSettings.StreamScalingFactor
	return nil
}

// setupAppium kills any stale Appium process, allocates a port, uninstalls
// conflicting UIAutomator2 server packages, starts Appium, and waits up to
// 30 seconds for it to become healthy.
func (d *AndroidDevice) setupAppium(ctx context.Context, installedApps []string) error {
	if err := cli.KillDeviceAppiumProcess(d.info.UDID); err != nil {
		return fmt.Errorf("setupAppium %s: kill existing processes: %w", d.info.UDID, err)
	}

	appiumPort, err := d.ports.GetFreePort()
	if err != nil {
		return fmt.Errorf("setupAppium %s: allocate port: %w", d.info.UDID, err)
	}
	d.info.AppiumPort = appiumPort

	// Clean up stale UIAutomator2 packages that may conflict with fresh installs.
	stalePackages := []string{
		"io.appium.settings",
		"io.appium.uiautomator2.server",
		"io.appium.uiautomator2.server.test",
	}
	for _, pkg := range stalePackages {
		if slices.Contains(installedApps, pkg) {
			if err := d.UninstallApp(pkg); err != nil {
				d.log.LogWarn("android_setup", fmt.Sprintf("Failed to uninstall %s from %s: %v", pkg, d.info.UDID, err))
			}
		}
	}

	go d.startAppium(ctx)

	// Wait up to 30 seconds for the Appium plugin to report healthy.
	timeout := time.After(30 * time.Second)
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-timeout:
			return fmt.Errorf("setupAppium %s: timed out waiting for Appium to start", d.info.UDID)
		case <-tick.C:
			if d.info.IsAppiumUp {
				d.log.LogInfo("android_setup", fmt.Sprintf("Appium is up for device %s on port %s", d.info.UDID, d.info.AppiumPort))
				break
			}
			continue
		}
		break
	}

	// Start Selenium Grid node if configured.
	if d.cfg.UseSeleniumGrid {
		if err := d.createGridTOML(); err != nil {
			return fmt.Errorf("setupAppium %s: create grid TOML: %w", d.info.UDID, err)
		}
		go d.startGridNode(ctx)
	}
	return nil
}

// startAppium launches the Appium server process for this device. It runs in
// a goroutine started by setupAppium and resets the device if the process exits
// unexpectedly.
func (d *AndroidDevice) startAppium(ctx context.Context) {
	caps := models.AppiumServerCapabilities{
		UDID:           d.info.UDID,
		AutomationName: "UiAutomator2",
		PlatformName:   "Android",
		DeviceName:     d.info.Name,
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
		d.log.LogError("android_setup", fmt.Sprintf("Failed to start Appium for %s: %v", d.info.UDID, err))
		d.Reset("Appium failed to start")
		return
	}
	<-proc.Done
	d.Reset("Appium process exited unexpectedly")
}

// createGridTOML writes a Selenium Grid node TOML file for this device to the
// provider folder. The file is named <udid>.toml.
func (d *AndroidDevice) createGridTOML() error {
	url := fmt.Sprintf("http://%s:%v/device/%s/appium",
		d.cfg.HostAddress, d.cfg.Port, d.info.UDID)
	caps := fmt.Sprintf(
		`{"appium:deviceName": "%s", "platformName": "%s", "appium:platformVersion": "%s", "appium:automationName": "UiAutomator2", "appium:udid": "%s"}`,
		d.info.Name, d.info.OS, d.info.OSVersion, d.info.UDID)

	gridPort, err := d.ports.GetFreePort()
	if err != nil {
		return fmt.Errorf("createGridTOML %s: allocate port: %w", d.info.UDID, err)
	}
	gridPortInt := 0
	if _, err := fmt.Sscanf(gridPort, "%d", &gridPortInt); err != nil {
		return fmt.Errorf("createGridTOML %s: parse port %q: %w", d.info.UDID, gridPort, err)
	}

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

// startGridNode starts the Selenium Grid node for this device. It waits 5
// seconds before starting to let Appium fully initialise, then resets the
// device if the node process exits.
func (d *AndroidDevice) startGridNode(ctx context.Context) {
	time.Sleep(5 * time.Second)
	proc, err := d.cmd.Start(ctx, "java",
		"-jar", filepath.Join(d.cfg.ProviderFolder, "selenium.jar"),
		"node",
		"--host", d.cfg.HostAddress,
		"--config", filepath.Join(d.cfg.ProviderFolder, d.info.UDID+".toml"),
		"--grid-url", d.cfg.SeleniumGrid,
	)
	if err != nil {
		d.log.LogError("android_setup", fmt.Sprintf("Failed to start Selenium Grid node for %s: %v", d.info.UDID, err))
		d.Reset("Selenium Grid node failed to start")
		return
	}
	<-proc.Done
	d.Reset("Selenium Grid node exited unexpectedly")
}

// fail logs the error, calls Reset, and returns the error so Setup can use it
// as a one-liner: `return d.fail(fmt.Errorf(...))`.
func (d *AndroidDevice) fail(err error) error {
	d.log.LogError("android_setup", err.Error())
	d.Reset(err.Error())
	return err
}
