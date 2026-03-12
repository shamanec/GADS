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
	"os"
	"path/filepath"

	ios "github.com/danielpaulus/go-ios/ios"

	"GADS/provider/config"
	"GADS/provider/logger"
)

// pairRecordCachePath returns the file path for storing a cached pair record for the given device
func pairRecordCachePath(udid string) string {
	return filepath.Join(config.ProviderConfig.ProviderFolder, "pair_records", udid+".json")
}

// savePairRecordToFile persists an ios.PairRecord to disk so it survives provider restarts.
// The file is stored at {providerFolder}/pair_records/{udid}.json with mode 0600.
func savePairRecordToFile(udid string, record ios.PairRecord) error {
	dir := filepath.Dir(pairRecordCachePath(udid))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return os.WriteFile(pairRecordCachePath(udid), data, 0600)
}

// cachePairRecord reads the current pair record from usbmuxd and saves it to disk.
// No-op when pair cache is disabled.
func cachePairRecord(udid string) {
	if !config.ProviderConfig.UseIOSPairCache {
		return
	}
	record, err := ios.ReadPairRecord(udid)
	if err != nil || record.HostID == "" {
		return
	}
	if err := savePairRecordToFile(udid, record); err != nil {
		logger.ProviderLogger.LogWarn("ios_device_setup",
			fmt.Sprintf("Failed to cache pair record for device `%s` - %s", udid, err))
	} else {
		logger.ProviderLogger.LogInfo("ios_device_setup",
			fmt.Sprintf("Cached pair record for device `%s`", udid))
	}
}

// restorePairRecordToUsbmuxd loads a cached pair record from disk and pushes it back
// into usbmuxd's in-memory state via a SavePairRecord message.
// Returns nil on success; non-nil if no cache file exists or the save fails.
func restorePairRecordToUsbmuxd(udid string) error {
	data, err := os.ReadFile(pairRecordCachePath(udid))
	if err != nil {
		return err // no cached record
	}

	var record ios.PairRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("restorePairRecordToUsbmuxd: Failed to parse cached pair record - %s", err)
	}

	recordBytes := ios.ToPlistBytes(record)

	msg := ios.SavePair{
		BundleID:            "go.ios.control",
		ClientVersionString: "go-ios-1.0.0",
		MessageType:         "SavePairRecord",
		ProgName:            "go-ios",
		LibUSBMuxVersion:    3,
		PairRecordID:        udid,
		PairRecordData:      recordBytes,
	}

	muxConn, err := ios.NewUsbMuxConnectionSimple()
	if err != nil {
		return fmt.Errorf("restorePairRecordToUsbmuxd: Failed to connect to usbmuxd - %s", err)
	}
	defer muxConn.Close()

	if err := muxConn.Send(msg); err != nil {
		return fmt.Errorf("restorePairRecordToUsbmuxd: Failed to send SavePairRecord - %s", err)
	}

	resp, err := muxConn.ReadMessage()
	if err != nil {
		return fmt.Errorf("restorePairRecordToUsbmuxd: Failed to read SavePairRecord response - %s", err)
	}

	muxResponse := ios.MuxResponsefromBytes(resp.Payload)
	if !muxResponse.IsSuccessFull() {
		return fmt.Errorf("restorePairRecordToUsbmuxd: usbmuxd rejected SavePairRecord - %+v", muxResponse)
	}
	return nil
}
