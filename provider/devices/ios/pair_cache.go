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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielpaulus/go-ios/ios"
)

// pairRecordCachePath returns the filesystem path where the pair record for
// udid is stored: {providerFolder}/pair_records/{udid}.json.
func (d *IOSDevice) pairRecordCachePath() string {
	return filepath.Join(d.cfg.ProviderFolder, "pair_records", d.info.UDID+".json")
}

// cachePairRecord reads the current pair record from usbmuxd and persists it
// to disk. This allows the provider to restore the record on reconnect,
// avoiding the "Trust this computer?" dialog. It is a no-op when
// UseIOSPairCache is disabled on the provider config.
func (d *IOSDevice) cachePairRecord() {
	if !d.cfg.UseIOSPairCache {
		return
	}
	record, err := ios.ReadPairRecord(d.info.UDID)
	if err != nil || record.HostID == "" {
		return
	}
	if err := d.savePairRecordToFile(record); err != nil {
		d.log.LogWarn("ios_pair_cache",
			fmt.Sprintf("Failed to cache pair record for device %s: %v", d.info.UDID, err))
		return
	}
	d.log.LogInfo("ios_pair_cache",
		fmt.Sprintf("Cached pair record for device %s", d.info.UDID))
}

// savePairRecordToFile persists record to the pair record cache path with
// mode 0600. The directory is created if it does not exist.
func (d *IOSDevice) savePairRecordToFile(record ios.PairRecord) error {
	dir := filepath.Dir(d.pairRecordCachePath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("savePairRecordToFile %s: mkdir: %w", d.info.UDID, err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("savePairRecordToFile %s: marshal: %w", d.info.UDID, err)
	}
	return os.WriteFile(d.pairRecordCachePath(), data, 0600)
}

// restorePairRecord loads the cached pair record from disk and pushes it into
// usbmuxd via a SavePairRecord message. Returns nil on success; non-nil if no
// cache file exists or the restore fails.
func (d *IOSDevice) restorePairRecord() error {
	data, err := os.ReadFile(d.pairRecordCachePath())
	if err != nil {
		return err // no cached record — not an error worth logging
	}

	var record ios.PairRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("restorePairRecord %s: unmarshal: %w", d.info.UDID, err)
	}

	recordBytes := ios.ToPlistBytes(record)
	msg := ios.SavePair{
		BundleID:            "gads.ios.control",
		ClientVersionString: "gads-1.0.0",
		MessageType:         "SavePairRecord",
		ProgName:            "gads",
		LibUSBMuxVersion:    3,
		PairRecordID:        d.info.UDID,
		PairRecordData:      recordBytes,
	}

	muxConn, err := ios.NewUsbMuxConnectionSimple()
	if err != nil {
		return fmt.Errorf("restorePairRecord %s: connect to usbmuxd: %w", d.info.UDID, err)
	}
	defer muxConn.Close()

	if err := muxConn.Send(msg); err != nil {
		return fmt.Errorf("restorePairRecord %s: send SavePairRecord: %w", d.info.UDID, err)
	}

	resp, err := muxConn.ReadMessage()
	if err != nil {
		return fmt.Errorf("restorePairRecord %s: read response: %w", d.info.UDID, err)
	}

	muxResp := ios.MuxResponsefromBytes(resp.Payload)
	if !muxResp.IsSuccessFull() {
		return fmt.Errorf("restorePairRecord %s: usbmuxd rejected record: %+v", d.info.UDID, muxResp)
	}
	return nil
}
