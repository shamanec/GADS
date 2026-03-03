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
)

// iosPairRecordPayload mirrors go-ios's unexported savePairRecordData for plist serialization.
// Field names must match exactly (no plist tags) so ToPlistBytes produces the same format.
type iosPairRecordPayload struct {
	DeviceCertificate []byte
	HostPrivateKey    []byte
	HostCertificate   []byte
	RootPrivateKey    []byte
	RootCertificate   []byte
	EscrowBag         []byte
	WiFiMACAddress    string
	HostID            string
	SystemBUID        string
}

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
		return fmt.Errorf("failed to parse cached pair record: %w", err)
	}

	// Serialize the record into the plist format that usbmuxd expects (same as go-ios's newSavePairRecordData).
	payload := iosPairRecordPayload{
		DeviceCertificate: record.DeviceCertificate,
		HostPrivateKey:    record.HostPrivateKey,
		HostCertificate:   record.HostCertificate,
		RootPrivateKey:    record.RootPrivateKey,
		RootCertificate:   record.RootCertificate,
		EscrowBag:         record.EscrowBag,
		WiFiMACAddress:    record.WiFiMACAddress,
		HostID:            record.HostID,
		SystemBUID:        record.SystemBUID,
	}
	recordBytes := ios.ToPlistBytes(payload)

	// Build the SavePairRecord message (mirrors go-ios's newSavePair).
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
		return fmt.Errorf("failed to connect to usbmuxd: %w", err)
	}
	defer muxConn.Close()

	if err := muxConn.Send(msg); err != nil {
		return fmt.Errorf("failed to send SavePairRecord: %w", err)
	}

	resp, err := muxConn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read SavePairRecord response: %w", err)
	}

	muxResponse := ios.MuxResponsefromBytes(resp.Payload)
	if !muxResponse.IsSuccessFull() {
		return fmt.Errorf("usbmuxd rejected SavePairRecord: %+v", muxResponse)
	}
	return nil
}
