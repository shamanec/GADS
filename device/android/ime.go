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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TypeText sends text to the GADS IME server running on the device. The IME
// server is forwarded to d.imePort on the host.
//
// The GADS IME approach is used instead of WDA (iOS) because Android does not
// have a cross-app typing API — the IME server intercepts keystrokes at the
// system level.
func (d *AndroidDevice) TypeText(text string) error {
	payload, err := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: text})
	if err != nil {
		return fmt.Errorf("TypeText %s: marshal: %w", d.info.UDID, err)
	}

	url := fmt.Sprintf("http://localhost:%s/type", d.imePort)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("TypeText %s: build request: %w", d.info.UDID, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.http.Do(req)
	if err != nil {
		return fmt.Errorf("TypeText %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// setupIME enables the GADS keyboard IME and sets it as the active input
// method on the device. A 1-second pause between enable and set is required
// because Android's IME switching can be asynchronous.
func (d *AndroidDevice) setupIME(ctx context.Context) error {
	if err := d.enableIME(ctx); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	return d.setIMEAsActive(ctx)
}

// enableIME runs `adb shell ime enable` for the GADS keyboard IME service.
func (d *AndroidDevice) enableIME(ctx context.Context) error {
	_, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID,
		"shell", "ime", "enable", "com.gads.settings/.GADSKeyboardIME")
	if err != nil {
		return fmt.Errorf("enableIME %s: %w", d.info.UDID, err)
	}
	return nil
}

// setIMEAsActive runs `adb shell ime set` to make the GADS keyboard IME the
// current active input method on the device.
func (d *AndroidDevice) setIMEAsActive(ctx context.Context) error {
	_, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID,
		"shell", "ime", "set", "com.gads.settings/.GADSKeyboardIME")
	if err != nil {
		return fmt.Errorf("setIMEAsActive %s: %w", d.info.UDID, err)
	}
	return nil
}
