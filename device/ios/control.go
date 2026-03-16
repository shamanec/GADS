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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/danielpaulus/go-ios/ios/instruments"
)

// wdaRequest builds and executes an HTTP request against the WebDriverAgent
// server forwarded on d.wdaPort. method is an HTTP verb, endpoint is the path
// without a leading slash, and body may be nil.
func (d *IOSDevice) wdaRequest(method, endpoint string, body io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/%s", d.wdaPort, endpoint)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("wdaRequest %s %s: build request: %w", d.info.UDID, endpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")
	return d.http.Do(req)
}

// Tap sends a single-tap gesture at screen coordinates (x, y) to WDA.
func (d *IOSDevice) Tap(x, y float64) error {
	payload, err := json.Marshal(struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}{X: x, Y: y})
	if err != nil {
		return fmt.Errorf("Tap %s: marshal: %w", d.info.UDID, err)
	}
	resp, err := d.wdaRequest(http.MethodPost, "wda/tap", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Tap %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// DoubleTap sends a double-tap gesture at screen coordinates (x, y) to WDA.
func (d *IOSDevice) DoubleTap(x, y float64) error {
	payload, err := json.Marshal(struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}{X: x, Y: y})
	if err != nil {
		return fmt.Errorf("DoubleTap %s: marshal: %w", d.info.UDID, err)
	}
	resp, err := d.wdaRequest(http.MethodPost, "wda/doubleTap", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("DoubleTap %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Swipe performs a swipe from (x, y) to (endX, endY) with a fixed 1-second
// delay as expected by WDA's swipe endpoint.
func (d *IOSDevice) Swipe(x, y, endX, endY float64) error {
	payload, err := json.Marshal(struct {
		X     float64 `json:"startX"`
		Y     float64 `json:"startY"`
		EndX  float64 `json:"endX"`
		EndY  float64 `json:"endY"`
		Delay float64 `json:"delay"`
	}{X: x, Y: y, EndX: endX, EndY: endY, Delay: 1})
	if err != nil {
		return fmt.Errorf("Swipe %s: marshal: %w", d.info.UDID, err)
	}
	resp, err := d.wdaRequest(http.MethodPost, "wda/swipe", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Swipe %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// TouchAndHold performs a long-press at (x, y). duration is provided in
// milliseconds but WDA expects seconds, so it is divided by 1000.
func (d *IOSDevice) TouchAndHold(x, y, duration float64) error {
	// WDA expects duration in seconds; the interface contract uses milliseconds.
	durationSeconds := duration / 1000
	payload, err := json.Marshal(struct {
		X        float64 `json:"x"`
		Y        float64 `json:"y"`
		Duration float64 `json:"duration"`
	}{X: x, Y: y, Duration: durationSeconds})
	if err != nil {
		return fmt.Errorf("TouchAndHold %s: marshal: %w", d.info.UDID, err)
	}
	resp, err := d.wdaRequest(http.MethodPost, "wda/touchAndHold", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("TouchAndHold %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Pinch performs a pinch gesture centred at (x, y). scale < 1 zooms out,
// scale > 1 zooms in. WDA's pinch endpoint accepts startScale, endScale and
// duration; startScale is fixed at 1.0, duration at 0.5 s.
func (d *IOSDevice) Pinch(x, y, scale float64) error {
	payload, err := json.Marshal(struct {
		CenterX    float64 `json:"centerX"`
		CenterY    float64 `json:"centerY"`
		StartScale float64 `json:"startScale"`
		EndScale   float64 `json:"endScale"`
		Duration   float64 `json:"duration"`
	}{CenterX: x, CenterY: y, StartScale: 1.0, EndScale: scale, Duration: 0.5})
	if err != nil {
		return fmt.Errorf("Pinch %s: marshal: %w", d.info.UDID, err)
	}
	resp, err := d.wdaRequest(http.MethodPost, "wda/pinch", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Pinch %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Home navigates to the home screen via WDA.
func (d *IOSDevice) Home() error {
	resp, err := d.wdaRequest(http.MethodPost, "wda/homescreen", nil)
	if err != nil {
		return fmt.Errorf("Home %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Lock locks the device screen via WDA.
func (d *IOSDevice) Lock() error {
	resp, err := d.wdaRequest(http.MethodPost, "wda/lock", nil)
	if err != nil {
		return fmt.Errorf("Lock %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Unlock unlocks the device screen via WDA.
func (d *IOSDevice) Unlock() error {
	resp, err := d.wdaRequest(http.MethodPost, "wda/unlock", nil)
	if err != nil {
		return fmt.Errorf("Unlock %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// TypeText types the given text using WDA's type endpoint.
func (d *IOSDevice) TypeText(text string) error {
	payload, err := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: text})
	if err != nil {
		return fmt.Errorf("TypeText %s: marshal: %w", d.info.UDID, err)
	}
	resp, err := d.wdaRequest(http.MethodPost, "wda/type", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("TypeText %s: %w", d.info.UDID, err)
	}
	resp.Body.Close()
	return nil
}

// Screenshot captures the device screen using the go-ios instruments
// screenshot service and returns the raw image bytes (PNG).
func (d *IOSDevice) Screenshot() ([]byte, error) {
	svc, err := instruments.NewScreenshotService(d.goIOSEntry)
	if err != nil {
		return nil, fmt.Errorf("Screenshot %s: create service: %w", d.info.UDID, err)
	}
	data, err := svc.TakeScreenshot()
	if err != nil {
		return nil, fmt.Errorf("Screenshot %s: take screenshot: %w", d.info.UDID, err)
	}
	return data, nil
}

// GetClipboard retrieves the clipboard contents from WDA. It first activates
// the WDA app so it has foreground focus (required for pasteboard access),
// fetches the clipboard, then navigates home to restore the previous context.
func (d *IOSDevice) GetClipboard() (string, error) {
	// Activate WDA bundle to give it foreground access to the pasteboard.
	activatePayload, err := json.Marshal(struct {
		BundleID string `json:"bundleId"`
	}{BundleID: d.cfg.WdaBundleID})
	if err != nil {
		return "", fmt.Errorf("GetClipboard %s: marshal activate: %w", d.info.UDID, err)
	}
	activateResp, err := d.wdaRequest(http.MethodPost, "wda/apps/activate", bytes.NewReader(activatePayload))
	if err != nil {
		return "", fmt.Errorf("GetClipboard %s: activate WDA: %w", d.info.UDID, err)
	}
	activateResp.Body.Close()

	// Request the clipboard content.
	pastePayload, err := json.Marshal(struct {
		ContentType string `json:"contentType"`
	}{ContentType: "plaintext"})
	if err != nil {
		return "", fmt.Errorf("GetClipboard %s: marshal pasteboard: %w", d.info.UDID, err)
	}
	clipResp, err := d.wdaRequest(http.MethodPost, "wda/getPasteboard", bytes.NewReader(pastePayload))
	if err != nil {
		return "", fmt.Errorf("GetClipboard %s: getPasteboard: %w", d.info.UDID, err)
	}
	defer clipResp.Body.Close()

	// Navigate home so the user lands back on the home screen.
	homeResp, _ := d.wdaRequest(http.MethodPost, "wda/homescreen", nil)
	if homeResp != nil {
		homeResp.Body.Close()
	}

	data, err := io.ReadAll(clipResp.Body)
	if err != nil {
		return "", fmt.Errorf("GetClipboard %s: read body: %w", d.info.UDID, err)
	}
	return string(data), nil
}
