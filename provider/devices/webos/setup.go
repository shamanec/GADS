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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"GADS/common/cli"
	"GADS/common/models"
	"github.com/pelletier/go-toml/v2"
)

// Setup provisions the WebOS TV device: kills stale Appium processes, sets the
// IP address, starts Appium, and marks the device live. It acquires setupMu
// so that concurrent calls for the same device are serialised. On failure the
// device is reset internally; the returned error is informational.
func (d *WebOSDevice) Setup(ctx context.Context) error {
	d.setupMu.Lock()
	defer d.setupMu.Unlock()

	d.info.ProviderState = "preparing"
	d.log.LogInfo("webos_setup", fmt.Sprintf("Running setup for WebOS device %s", d.info.UDID))

	// For WebOS, the UDID is the device IP address used by the ares-* tools.
	d.info.IPAddress = d.info.UDID

	if err := d.setupAppium(ctx); err != nil {
		return d.fail(err)
	}

	d.info.ProviderState = "live"
	d.log.LogInfo("webos_setup", fmt.Sprintf("Device %s is live", d.info.UDID))
	return nil
}

// Reset releases allocated resources and returns the device to the "init"
// state so the provider loop can attempt setup again. It is idempotent.
func (d *WebOSDevice) Reset(reason string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.info.IsResetting || d.info.ProviderState == "init" {
		return
	}

	d.log.LogInfo("webos_reset", fmt.Sprintf("Resetting device %s: %s", d.info.UDID, reason))
	d.info.IsResetting = true

	d.ports.FreePort(d.info.AppiumPort)
	d.info.AppiumPort = ""

	d.info.ProviderState = "init"
	d.info.IsResetting = false
}

// setupAppium kills stale Appium processes, allocates a port, starts Appium,
// and waits up to 30 seconds for it to become healthy.
func (d *WebOSDevice) setupAppium(ctx context.Context) error {
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
				d.log.LogInfo("webos_setup",
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

// startAppium launches the Appium server for this WebOS TV device using the
// webos/lgtv automation driver. It runs in a goroutine and resets the device
// if the process exits unexpectedly.
func (d *WebOSDevice) startAppium(ctx context.Context) {
	chromeDriverPath := filepath.Join(d.cfg.ProviderFolder, "drivers/chromedriver")
	absolutePath, err := filepath.Abs(chromeDriverPath)
	if err != nil {
		d.log.LogError("webos_setup",
			fmt.Sprintf("Failed to resolve ChromeDriver path for %s: %v", d.info.UDID, err))
		d.Reset("Failed to resolve ChromeDriver path")
		return
	}

	caps := models.AppiumServerCapabilities{
		AutomationName:         "webos",
		PlatformName:           "lgtv",
		UDID:                   d.info.UDID,
		DeviceHost:             d.info.UDID,
		DeviceName:             d.info.Name,
		ChromeDriverExecutable: absolutePath,
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
		d.log.LogError("webos_setup", fmt.Sprintf("Failed to start Appium for %s: %v", d.info.UDID, err))
		d.Reset("Appium failed to start")
		return
	}
	<-proc.Done
	d.Reset("Appium process exited unexpectedly")
}

// createGridTOML writes a Selenium Grid TOML node config for this device.
func (d *WebOSDevice) createGridTOML() error {
	url := fmt.Sprintf("http://%s:%v/device/%s/appium",
		d.cfg.HostAddress, d.cfg.Port, d.info.UDID)
	caps := fmt.Sprintf(
		`{"appium:deviceName": "%s", "platformName": "lgtv", "appium:automationName": "webos", "appium:udid": "%s"}`,
		d.info.Name, d.info.UDID)

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

// startGridNode starts the Selenium Grid node for this device.
func (d *WebOSDevice) startGridNode(ctx context.Context) {
	time.Sleep(5 * time.Second)
	proc, err := d.cmd.Start(ctx, "java",
		"-jar", filepath.Join(d.cfg.ProviderFolder, "selenium.jar"),
		"node",
		"--host", d.cfg.HostAddress,
		"--config", filepath.Join(d.cfg.ProviderFolder, d.info.UDID+".toml"),
		"--grid-url", d.cfg.SeleniumGrid,
	)
	if err != nil {
		d.log.LogError("webos_setup",
			fmt.Sprintf("Failed to start Selenium Grid node for %s: %v", d.info.UDID, err))
		d.Reset("Selenium Grid node failed to start")
		return
	}
	<-proc.Done
	d.Reset("Selenium Grid node exited unexpectedly")
}

// fail logs the error, calls Reset, and returns the error for use as a
// one-liner: `return d.fail(fmt.Errorf(...))`.
func (d *WebOSDevice) fail(err error) error {
	d.log.LogError("webos_setup", err.Error())
	d.Reset(err.Error())
	return err
}
