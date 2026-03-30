/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package devices

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"GADS/common/cli"
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"
	"GADS/provider/providerutil"
)

// setupAppiumForDevice handles the full Appium setup for any platform device:
// kill existing process, allocate port, start Appium, wait for ready, optionally start Selenium Grid node.
func setupAppiumForDevice(d PlatformDevice) error {
	device := d.GetDBDevice()

	if err := cli.KillDeviceAppiumProcess(device.UDID); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Failed attempt to kill existing Appium processes for device `%s` - %v", device.UDID, err))
		d.Reset("Failed to kill existing Appium processes.")
		return err
	}

	appiumPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not allocate free Appium port for device `%v` - %v", device.UDID, err))
		d.Reset("Failed to allocate free Appium port for device.")
		return err
	}
	device.AppiumPort = appiumPort

	caps := d.AppiumCapabilities()
	go startAppium(device, caps)

	timeout := time.After(30 * time.Second)
	tick := time.Tick(200 * time.Millisecond)
AppiumLoop:
	for {
		select {
		case <-timeout:
			logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Did not successfully start Appium for device `%v` in 30 seconds", device.UDID))
			d.Reset("Failed to start Appium for device.")
			return fmt.Errorf("appium did not start in time")
		case <-tick:
			if device.IsAppiumUp {
				logger.ProviderLogger.LogInfo("device_setup", fmt.Sprintf("Successfully started Appium for device `%v` on port %v", device.UDID, device.AppiumPort))
				break AppiumLoop
			}
		}
	}

	if config.ProviderConfig.UseSeleniumGrid {
		if err := createGridTOML(device, caps.AutomationName); err != nil {
			logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Selenium Grid use is enabled but couldn't create TOML for device `%s` - %s", device.UDID, err))
			d.Reset("Failed to create TOML for device.")
			return err
		}
		go startGridNode(device)
	}

	return nil
}

// startAppium starts the Appium server process with the given capabilities.
// It runs as a goroutine and blocks until the process exits.
func startAppium(device *models.Device, capabilities models.AppiumServerCapabilities) {
	capabilitiesJson, _ := json.Marshal(capabilities)

	pluginConfig := models.AppiumPluginConfiguration{
		ProviderUrl:       fmt.Sprintf("http://%s:%v", config.ProviderConfig.HostAddress, config.ProviderConfig.Port),
		HeartBeatInterval: "2000",
		UDID:              device.UDID,
	}
	pluginConfigJson, _ := json.Marshal(pluginConfig)

	cmd := exec.CommandContext(
		device.Context,
		"appium",
		"-p",
		device.AppiumPort,
		"--log-timestamp",
		"--use-plugin=gads",
		fmt.Sprintf("--plugin-gads-config=%s", string(pluginConfigJson)),
		"--session-override",
		"--log-no-colors",
		"--relaxed-security",
		"--default-capabilities", string(capabilitiesJson))

	logger.ProviderLogger.LogDebug("device_setup", fmt.Sprintf("Starting Appium on device `%s` with command `%s`", device.UDID, cmd.Args))

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error executing `%s` for device `%v` - %v", cmd.Args, device.UDID, err))
		ResetLocalDevice(device, "Failed to execute Appium command.")
		return
	}

	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf(
			"startAppium: Error waiting for `%s` command to finish, it errored out or device `%v` was disconnected - %v",
			cmd.Args, device.UDID, err))

		ResetLocalDevice(device, "Appium command errored out or device was disconnected.")
	}
}

// createGridTOML creates a Selenium Grid TOML configuration file for the device.
func createGridTOML(device *models.Device, automationName string) error {
	url := fmt.Sprintf("http://%s:%v/device/%s/appium", config.ProviderConfig.HostAddress, config.ProviderConfig.Port, device.UDID)
	configs := fmt.Sprintf(`{"appium:deviceName": "%s", "platformName": "%s", "appium:platformVersion": "%s", "appium:automationName": "%s", "appium:udid": "%s"}`, device.Name, device.OS, device.OSVersion, automationName, device.UDID)

	port, _ := providerutil.GetFreePort()
	portInt, _ := strconv.Atoi(port)
	conf := models.AppiumTomlConfig{
		Server: models.AppiumTomlServer{
			Port: portInt,
		},
		Node: models.AppiumTomlNode{
			DetectDrivers: false,
		},
		Relay: models.AppiumTomlRelay{
			URL:            url,
			StatusEndpoint: "/status",
			Configs: []string{
				"1",
				configs,
			},
		},
	}

	res, err := toml.Marshal(conf)
	if err != nil {
		return fmt.Errorf("Failed marshalling TOML Appium config - %s", err)
	}

	file, err := os.Create(fmt.Sprintf("%s/%s.toml", config.ProviderConfig.ProviderFolder, device.UDID))
	if err != nil {
		return fmt.Errorf("Failed creating TOML Appium config file - %s", err)
	}
	defer file.Close()

	_, err = io.WriteString(file, string(res))
	if err != nil {
		return fmt.Errorf("Failed writing to TOML Appium config file - %s", err)
	}

	return nil
}

// startGridNode starts a Selenium Grid node for the device.
// It runs as a goroutine and blocks until the process exits.
func startGridNode(device *models.Device) {
	time.Sleep(5 * time.Second)
	cmd := exec.CommandContext(device.Context,
		"java",
		"-jar",
		fmt.Sprintf("%s/selenium.jar", config.ProviderConfig.ProviderFolder),
		"node",
		"--host",
		config.ProviderConfig.HostAddress,
		"--config",
		fmt.Sprintf("%s/%s.toml", config.ProviderConfig.ProviderFolder, device.UDID),
		"--grid-url",
		config.ProviderConfig.SeleniumGrid,
	)

	logger.ProviderLogger.LogInfo("device_setup", fmt.Sprintf("Starting Selenium grid node for device `%s` with command `%s`", device.UDID, cmd.Args))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error creating stdoutpipe while starting Selenium Grid node for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to create stdoutpipe while starting Selenium Grid node.")
		return
	}

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not start Selenium Grid node for device `%v` - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to start Selenium Grid node.")
		return
	}

	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		device.Logger.LogDebug("grid-node", strings.TrimSpace(line))
	}

	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error waiting for Selenium Grid node command to finish, it errored out or device `%v` was disconnected - %v", device.UDID, err))
		ResetLocalDevice(device, "Failed to wait for Selenium Grid node command to finish.")
	}
}
