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
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"

	"GADS/common/db"
	"GADS/common/models"
	"GADS/provider/config"
	"GADS/provider/logger"
)

// Emulators default to the H264 WebRTC stream as the most capable path. If live
// testing shows the app_process H264 capture does not work on emulators, flip this
// to models.MJPEGStreamTypeId (MJPEG through the GADS-Settings service).
const emulatorDefaultStreamType = models.AndroidWebRTCGadsH264StreamTypeId

// emulatorSerialRegex matches adb console-port serials like `emulator-5554`.
var emulatorSerialRegex = regexp.MustCompile(`^emulator-\d+$`)

// emulatorAvdNames caches serial → AVD name so `adb emu avd name` runs once per
// emulator boot instead of on every 1s tick. Accessed only from the updateDevices
// goroutine, so no locking is needed.
var emulatorAvdNames = map[string]string{}

// emulatorDuplicateWarned tracks serials already reported as a second running
// instance of an AVD so the warning is logged once per boot, not every tick.
var emulatorDuplicateWarned = map[string]bool{}

// reconcileAndroidEmulators translates connected emulator serials into ephemeral
// devices keyed by AVD-derived UDIDs and returns the connected list with those
// serials replaced by the synthesized UDIDs, so the regular matching in
// updateDevices works unchanged. Ephemeral devices whose emulator disappeared
// from adb are removed from DevManager entirely - unlike DB devices, which stay
// visible as disconnected, an ephemeral device's existence is its connection.
func reconcileAndroidEmulators(connectedDevices []string) []string {
	reconciled := make([]string, 0, len(connectedDevices))
	// Every emulator serial present in the adb listing this tick, used to expire
	// devices and caches for emulators that are gone
	seenSerials := map[string]bool{}
	// AVD names already mapped to a serial this tick, to catch a second running
	// instance of the same AVD (possible with `emulator -read-only`)
	claimedAvds := map[string]string{}

	for _, entry := range connectedDevices {
		if !emulatorSerialRegex.MatchString(entry) {
			reconciled = append(reconciled, entry)
			continue
		}
		serial := entry
		seenSerials[serial] = true

		// An emulator registered in the DB under its raw serial keeps the regular
		// DB-device flow - it was set up that way before ephemeral support existed
		if platDev, exists := DevManager.Get(serial); exists && !platDev.IsEphemeral() {
			reconciled = append(reconciled, serial)
			continue
		}

		avdName, err := resolveEmulatorAvdName(serial)
		if err != nil {
			logger.ProviderLogger.LogDebugf("emulator_discovery", "Could not resolve AVD name for `%s`, will retry on next tick - %v", serial, err)
			continue
		}

		if claimedSerial, claimed := claimedAvds[avdName]; claimed {
			if !emulatorDuplicateWarned[serial] {
				emulatorDuplicateWarned[serial] = true
				logger.ProviderLogger.LogWarnf("emulator_discovery", "AVD `%s` is running as both `%s` and `%s` - ignoring `%s`, only one instance of an AVD can be provided", avdName, claimedSerial, serial, serial)
			}
			continue
		}
		claimedAvds[avdName] = serial

		udid, err := ensureEmulatorDevice(serial, avdName)
		if err != nil {
			logger.ProviderLogger.LogErrorf("emulator_discovery", "Failed to initialize device for emulator `%s` (AVD `%s`) - %v", serial, avdName, err)
			continue
		}
		reconciled = append(reconciled, udid)
	}

	for _, platDev := range DevManager.All() {
		if !platDev.IsEphemeral() {
			continue
		}
		if !seenSerials[platDev.GetSerial()] {
			logger.ProviderLogger.LogInfof("emulator_discovery", "Emulator `%s` (device `%s`) is no longer connected, removing device", platDev.GetSerial(), platDev.GetUDID())
			platDev.Reset("Emulator is no longer connected.")
			DevManager.Delete(platDev.GetUDID())
		}
	}

	for serial := range emulatorAvdNames {
		if !seenSerials[serial] {
			delete(emulatorAvdNames, serial)
			delete(emulatorDuplicateWarned, serial)
		}
	}

	return reconciled
}

// ensureEmulatorDevice returns the synthesized UDID for the given emulator,
// creating and registering the ephemeral device if this AVD has no device yet.
func ensureEmulatorDevice(serial string, avdName string) (string, error) {
	udid := fmt.Sprintf("emu_%s_%s", config.ProviderConfig.Nickname, avdName)

	if platDev, exists := DevManager.Get(udid); exists {
		// Same AVD re-appeared on a different console port - retarget the transport id
		if andDev, ok := platDev.(*AndroidDevice); ok && andDev.Serial != serial {
			logger.ProviderLogger.LogInfof("emulator_discovery", "Emulator for AVD `%s` moved from `%s` to `%s`", avdName, andDev.Serial, serial)
			andDev.Serial = serial
		}
		return udid, nil
	}

	osVersion, err := getEmulatorOSVersion(serial)
	if err != nil {
		return "", fmt.Errorf("could not detect OS version - %v", err)
	}

	usage := "enabled"
	if !config.ProviderConfig.SetupAppiumServers {
		// Same rule syncDevicesToDB applies to DB devices - no Appium servers means control-only
		usage = "control"
	}

	// Ephemeral devices always live in the default workspace - there is no DB
	// record an admin could move to another one
	workspaceID := ""
	if defaultWorkspace, err := db.GlobalMongoStore.GetDefaultWorkspace(); err == nil {
		workspaceID = defaultWorkspace.ID
	} else {
		logger.ProviderLogger.LogWarnf("emulator_discovery", "Could not resolve default workspace for emulator device `%s` - %v", udid, err)
	}

	dbDevice := &models.DBDevice{
		UDID:        udid,
		OS:          "android",
		Name:        avdName,
		OSVersion:   osVersion,
		Provider:    config.ProviderConfig.Nickname,
		Usage:       usage,
		DeviceType:  "emulator",
		StreamType:  emulatorDefaultStreamType,
		WorkspaceID: workspaceID,
	}

	platDev, err := createPlatformDevice(dbDevice)
	if err != nil {
		return "", err
	}
	andDev, ok := platDev.(*AndroidDevice)
	if !ok {
		return "", fmt.Errorf("device for emulator `%s` is not an AndroidDevice", serial)
	}
	// Mark the device before registering it so it is never visible in DevManager
	// unmarked - the DB sync loop would otherwise remove it as not-in-DB
	andDev.Serial = serial
	andDev.Ephemeral = true
	DevManager.Set(udid, platDev)

	logger.ProviderLogger.LogInfof("emulator_discovery", "Discovered emulator `%s` (AVD `%s`, Android %s), providing it as device `%s`", serial, avdName, osVersion, udid)
	return udid, nil
}

// resolveEmulatorAvdName returns the AVD name backing the emulator with the given
// serial, cached per serial for the lifetime of the emulator connection.
func resolveEmulatorAvdName(serial string) (string, error) {
	if avdName, cached := emulatorAvdNames[serial]; cached {
		return avdName, nil
	}

	out, err := exec.Command("adb", "-s", serial, "emu", "avd", "name").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("`adb emu avd name` failed - %v (%s)", err, strings.TrimSpace(string(out)))
	}
	avdName := parseAvdNameOutput(string(out))
	if avdName == "" {
		return "", fmt.Errorf("`adb emu avd name` returned no name (output: `%s`)", strings.TrimSpace(string(out)))
	}

	emulatorAvdNames[serial] = avdName
	return avdName, nil
}

// parseAvdNameOutput extracts the AVD name from `adb emu avd name` output - the
// name on its own line followed by an `OK` line. Returns "" if no name is found.
func parseAvdNameOutput(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != "OK" {
			return line
		}
	}
	return ""
}

// getEmulatorOSVersion returns a semver-parsable Android version for the emulator.
// Preview builds report a codename (e.g. `Baklava`) instead of a number as the
// release version - fall back to the numeric API level for those.
func getEmulatorOSVersion(serial string) (string, error) {
	release, err := getEmulatorProp(serial, "ro.build.version.release")
	if err != nil {
		return "", err
	}
	if _, err := semver.NewVersion(release); err == nil {
		return release, nil
	}

	sdk, err := getEmulatorProp(serial, "ro.build.version.sdk")
	if err != nil {
		return "", fmt.Errorf("release version `%s` is not semver-parsable and reading the API level failed - %v", release, err)
	}
	return sdk, nil
}

func getEmulatorProp(serial string, property string) (string, error) {
	out, err := exec.Command("adb", "-s", serial, "shell", "getprop", property).Output()
	if err != nil {
		return "", fmt.Errorf("`getprop %s` failed - %v", property, err)
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", fmt.Errorf("`getprop %s` returned an empty value", property)
	}
	return value, nil
}
