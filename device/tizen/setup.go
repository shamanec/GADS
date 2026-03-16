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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"GADS/common/cli"
	"GADS/common/models"
	"github.com/pelletier/go-toml/v2"
)

// Setup provisions the Tizen TV device: kills stale Appium processes, fetches
// TV metadata, starts Appium, and marks the device live. It acquires setupMu
// so that concurrent calls for the same device are serialised.
func (d *TizenDevice) Setup(ctx context.Context) {
	d.setupMu.Lock()
	defer d.setupMu.Unlock()

	d.info.ProviderState = "preparing"
	d.log.LogInfo("tizen_setup", fmt.Sprintf("Running setup for Tizen device %s", d.info.UDID))

	if err := d.setupAppium(ctx); err != nil {
		d.fail(err)
		return
	}

	d.info.ProviderState = "live"
	d.log.LogInfo("tizen_setup", fmt.Sprintf("Device %s is live", d.info.UDID))
}

// Reset releases allocated resources and returns the device to the "init"
// state so the provider loop can attempt setup again. It is idempotent.
func (d *TizenDevice) Reset(reason string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.info.IsResetting || d.info.ProviderState == "init" {
		return
	}

	d.log.LogInfo("tizen_reset", fmt.Sprintf("Resetting device %s: %s", d.info.UDID, reason))
	d.info.IsResetting = true

	d.ports.FreePort(d.appiumPort)
	d.appiumPort = ""

	d.info.ProviderState = "init"
	d.info.IsResetting = false
}

// setupAppium kills stale Appium processes, fetches TV info to populate device
// metadata, starts Appium, and waits up to 30 seconds for it to become healthy.
func (d *TizenDevice) setupAppium(ctx context.Context) error {
	if err := cli.KillDeviceAppiumProcess(d.info.UDID); err != nil {
		return fmt.Errorf("setupAppium %s: kill existing processes: %w", d.info.UDID, err)
	}

	appiumPort, err := d.ports.GetFreePort()
	if err != nil {
		return fmt.Errorf("setupAppium %s: allocate port: %w", d.info.UDID, err)
	}
	d.appiumPort = appiumPort

	if err := d.getTVInfo(); err != nil {
		return fmt.Errorf("setupAppium %s: get TV info: %w", d.info.UDID, err)
	}

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
				d.log.LogInfo("tizen_setup",
					fmt.Sprintf("Appium is up for device %s on port %s", d.info.UDID, d.appiumPort))
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

// startAppium launches the Appium server for this Tizen TV device using the
// TizenTV automation driver. It runs in a goroutine and resets the device if
// the process exits unexpectedly.
func (d *TizenDevice) startAppium(ctx context.Context) {
	chromeDriverPath := filepath.Join(d.cfg.ProviderFolder, "drivers/chromedriver")
	absolutePath, err := filepath.Abs(chromeDriverPath)
	if err != nil {
		d.log.LogError("tizen_setup",
			fmt.Sprintf("Failed to resolve ChromeDriver path for %s: %v", d.info.UDID, err))
		d.Reset("Failed to resolve ChromeDriver path")
		return
	}

	caps := models.AppiumServerCapabilities{
		AutomationName:         "TizenTV",
		PlatformName:           "TizenTV",
		UDID:                   d.info.UDID,
		DeviceAddress:          d.info.DeviceAddress,
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
		"-p", d.appiumPort,
		"--log-timestamp",
		"--use-plugin=gads",
		fmt.Sprintf("--plugin-gads-config=%s", string(pluginCfgJSON)),
		"--session-override",
		"--log-no-colors",
		"--relaxed-security",
		"--default-capabilities", string(capsJSON),
	)
	if err != nil {
		d.log.LogError("tizen_setup", fmt.Sprintf("Failed to start Appium for %s: %v", d.info.UDID, err))
		d.Reset("Appium failed to start")
		return
	}
	<-proc.Done
	d.Reset("Appium process exited unexpectedly")
}

// createGridTOML writes a Selenium Grid TOML node config for this device.
func (d *TizenDevice) createGridTOML() error {
	url := fmt.Sprintf("http://%s:%v/device/%s/appium",
		d.cfg.HostAddress, d.cfg.Port, d.info.UDID)
	caps := fmt.Sprintf(
		`{"appium:deviceName": "%s", "platformName": "TizenTV", "appium:automationName": "TizenTV", "appium:udid": "%s"}`,
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
func (d *TizenDevice) startGridNode(ctx context.Context) {
	time.Sleep(5 * time.Second)
	proc, err := d.cmd.Start(ctx, "java",
		"-jar", filepath.Join(d.cfg.ProviderFolder, "selenium.jar"),
		"node",
		"--host", d.cfg.HostAddress,
		"--config", filepath.Join(d.cfg.ProviderFolder, d.info.UDID+".toml"),
		"--grid-url", d.cfg.SeleniumGrid,
	)
	if err != nil {
		d.log.LogError("tizen_setup",
			fmt.Sprintf("Failed to start Selenium Grid node for %s: %v", d.info.UDID, err))
		d.Reset("Selenium Grid node failed to start")
		return
	}
	<-proc.Done
	d.Reset("Selenium Grid node exited unexpectedly")
}

// getTVInfo fetches device metadata from the Tizen REST API at port 8001 and
// populates HardwareModel, OSVersion, IPAddress, DeviceAddress, and screen
// dimensions on d.info.
func (d *TizenDevice) getTVInfo() error {
	host, err := tvHost(d.info.UDID)
	if err != nil {
		return fmt.Errorf("getTVInfo %s: %w", d.info.UDID, err)
	}

	url := fmt.Sprintf("http://%s:8001/api/v2/", host)
	resp, err := http.Get(url) //nolint:noctx // short-lived metadata fetch
	if err != nil {
		return fmt.Errorf("getTVInfo %s: HTTP GET: %w", d.info.UDID, err)
	}
	defer resp.Body.Close()

	var tvInfo models.TizenTVInfo
	if err := json.NewDecoder(resp.Body).Decode(&tvInfo); err != nil {
		return fmt.Errorf("getTVInfo %s: decode: %w", d.info.UDID, err)
	}

	d.info.HardwareModel = tvInfo.Device.ModelName
	d.info.OSVersion = tvInfo.Version
	d.info.IPAddress = tvInfo.Device.IP
	d.info.DeviceAddress = d.info.UDID

	if tvInfo.Device.Resolution != "" {
		dims := strings.Split(tvInfo.Device.Resolution, "x")
		if len(dims) == 2 {
			d.info.ScreenWidth = dims[0]
			d.info.ScreenHeight = dims[1]
		}
	}

	return nil
}

// fail logs the error, calls Reset, and returns the error for use as a
// one-liner: `return d.fail(fmt.Errorf(...))`.
func (d *TizenDevice) fail(err error) error {
	d.log.LogError("tizen_setup", err.Error())
	d.Reset(err.Error())
	return err
}
