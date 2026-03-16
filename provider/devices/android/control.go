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
	"io"
	"net/http"
)

// remoteServerRequest builds and executes an HTTP request against the
// GADS-Settings remote control server running on this device. method is an
// HTTP verb (e.g. "POST"), endpoint is the path without a leading slash,
// and body may be nil.
func (d *AndroidDevice) remoteServerRequest(method, endpoint string, body io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/%s", d.remoteServerPort, endpoint)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("remoteServerRequest %s %s: build request: %w", d.info.UDID, endpoint, err)
	}
	return d.http.Do(req)
}

// remoteServerJSONRequest is like remoteServerRequest but sets Content-Type
// to application/json.
func (d *AndroidDevice) remoteServerJSONRequest(method, endpoint string, body io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/%s", d.remoteServerPort, endpoint)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("remoteServerJSONRequest %s %s: build request: %w", d.info.UDID, endpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")
	return d.http.Do(req)
}

// Tap sends a single-tap gesture at screen coordinates (x, y) to the
// GADS-Settings remote control server.
func (d *AndroidDevice) Tap(x, y float64) error {
	payload, err := json.Marshal(struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}{X: x, Y: y})
	if err != nil {
		return fmt.Errorf("Tap %s: marshal: %w", d.info.UDID, err)
	}
	resp, err := d.remoteServerJSONRequest(http.MethodPost, "tap", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Tap %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// DoubleTap sends a double-tap gesture at screen coordinates (x, y).
func (d *AndroidDevice) DoubleTap(x, y float64) error {
	payload, err := json.Marshal(struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}{X: x, Y: y})
	if err != nil {
		return fmt.Errorf("DoubleTap %s: marshal: %w", d.info.UDID, err)
	}
	resp, err := d.remoteServerJSONRequest(http.MethodPost, "doubleTap", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("DoubleTap %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Swipe performs a swipe gesture from (x, y) to (endX, endY). The duration
// sent to the remote server is 500 ms.
func (d *AndroidDevice) Swipe(x, y, endX, endY float64) error {
	payload, err := json.Marshal(struct {
		X        float64 `json:"x1"`
		Y        float64 `json:"y1"`
		EndX     float64 `json:"x2"`
		EndY     float64 `json:"y2"`
		Duration float64 `json:"duration"`
	}{X: x, Y: y, EndX: endX, EndY: endY, Duration: 500})
	if err != nil {
		return fmt.Errorf("Swipe %s: marshal: %w", d.info.UDID, err)
	}
	resp, err := d.remoteServerJSONRequest(http.MethodPost, "swipe", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Swipe %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// TouchAndHold performs a long-press at (x, y) for the given duration in
// milliseconds. Android and iOS both express duration in milliseconds here.
func (d *AndroidDevice) TouchAndHold(x, y, duration float64) error {
	payload, err := json.Marshal(struct {
		X        float64 `json:"x"`
		Y        float64 `json:"y"`
		Duration float64 `json:"duration"`
	}{X: x, Y: y, Duration: duration})
	if err != nil {
		return fmt.Errorf("TouchAndHold %s: marshal: %w", d.info.UDID, err)
	}
	resp, err := d.remoteServerJSONRequest(http.MethodPost, "touchAndHold", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("TouchAndHold %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Pinch performs a pinch gesture centred at (x, y) with the given scale.
// A scale < 1 zooms out; scale > 1 zooms in. Duration is fixed at 300 ms.
func (d *AndroidDevice) Pinch(x, y, scale float64) error {
	payload, err := json.Marshal(struct {
		CenterX   float64 `json:"centerX"`
		CenterY   float64 `json:"centerY"`
		Scale     float64 `json:"scale"`
		Duration  int     `json:"duration"`
		Direction string  `json:"direction"`
	}{CenterX: x, CenterY: y, Scale: scale, Duration: 300, Direction: "diagonal"})
	if err != nil {
		return fmt.Errorf("Pinch %s: marshal: %w", d.info.UDID, err)
	}
	resp, err := d.remoteServerJSONRequest(http.MethodPost, "pinch", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Pinch %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Home sends the Home button event to the remote control server.
func (d *AndroidDevice) Home() error {
	resp, err := d.remoteServerRequest(http.MethodPost, "home", nil)
	if err != nil {
		return fmt.Errorf("Home %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Lock sends a lock request to the remote control server.
func (d *AndroidDevice) Lock() error {
	resp, err := d.remoteServerRequest(http.MethodPost, "lock", nil)
	if err != nil {
		return fmt.Errorf("Lock %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Unlock sends an unlock request to the remote control server.
func (d *AndroidDevice) Unlock() error {
	resp, err := d.remoteServerRequest(http.MethodPost, "unlock", nil)
	if err != nil {
		return fmt.Errorf("Unlock %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Screenshot captures the device screen via `adb exec-out screencap -p` and
// returns the raw PNG bytes.
func (d *AndroidDevice) Screenshot() ([]byte, error) {
	out, err := d.cmd.Run(context.Background(), "adb", "-s", d.info.UDID, "exec-out", "screencap", "-p")
	if err != nil {
		return nil, fmt.Errorf("Screenshot %s: %w", d.info.UDID, err)
	}
	return out, nil
}

// GetClipboard retrieves the current clipboard contents from the remote
// control server.
func (d *AndroidDevice) GetClipboard() (string, error) {
	resp, err := d.remoteServerRequest(http.MethodPost, "clipboard", nil)
	if err != nil {
		return "", fmt.Errorf("GetClipboard %s: %w", d.info.UDID, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("GetClipboard %s: read body: %w", d.info.UDID, err)
	}
	return string(data), nil
}
