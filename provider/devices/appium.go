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
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"GADS/common/cli"
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"
	"GADS/provider/providerutil"
)

// setupAppiumForDevice handles the full Appium setup for any platform device:
// kill existing process, allocate port, start Appium, wait for ready, optionally start Selenium Grid node.
func setupAppiumForDevice(d PlatformDevice) error {
	udid := d.GetUDID()

	if err := cli.KillDeviceAppiumProcess(udid); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Failed attempt to kill existing Appium processes for device `%s` - %v", udid, err))
		d.Reset("Failed to kill existing Appium processes.")
		return err
	}

	appiumPort, err := providerutil.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not allocate free Appium port for device `%v` - %v", udid, err))
		d.Reset("Failed to allocate free Appium port for device.")
		return err
	}
	d.SetAppiumPort(appiumPort)

	caps := d.AppiumCapabilities()
	go startAppium(d, caps)

	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
AppiumLoop:
	for {
		select {
		case <-timeout:
			logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Did not successfully start Appium for device `%v` in 30 seconds", udid))
			d.Reset("Failed to start Appium for device.")
			return fmt.Errorf("appium did not start in time")
		case <-ticker.C:
			if d.GetIsAppiumUp() {
				logger.ProviderLogger.LogInfo("device_setup", fmt.Sprintf("Successfully started Appium for device `%v` on port %v", udid, appiumPort))
				break AppiumLoop
			}
		}
	}

	return nil
}

// startAppium starts the Appium server process with the given capabilities.
// It runs as a goroutine and blocks until the process exits.
func startAppium(d PlatformDevice, capabilities models.AppiumServerCapabilities) {
	udid := d.GetUDID()
	appiumPort := d.GetAppiumPort()
	capabilitiesJson, _ := json.Marshal(capabilities)

	pluginConfig := models.AppiumPluginConfiguration{
		ProviderUrl:       fmt.Sprintf("http://%s:%v", config.ProviderConfig.HostAddress, config.ProviderConfig.Port),
		HeartBeatInterval: "2000",
		UDID:              udid,
	}
	pluginConfigJson, _ := json.Marshal(pluginConfig)

	cmd := exec.CommandContext(
		d.GetContext(),
		"appium",
		"-p",
		appiumPort,
		"--log-timestamp",
		"--use-plugin=gads",
		fmt.Sprintf("--plugin-gads-config=%s", string(pluginConfigJson)),
		"--session-override",
		"--log-no-colors",
		"--relaxed-security",
		"--default-capabilities", string(capabilitiesJson))

	logger.ProviderLogger.LogDebug("device_setup", fmt.Sprintf("Starting Appium on device `%s` with command `%s`", udid, cmd.Args))

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error executing `%s` for device `%v` - %v", cmd.Args, udid, err))
		d.Reset("Failed to execute Appium command.")
		return
	}

	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf(
			"startAppium: Error waiting for `%s` command to finish, it errored out or device `%v` was disconnected - %v",
			cmd.Args, udid, err))

		d.Reset("Appium command errored out or device was disconnected.")
	}
}
