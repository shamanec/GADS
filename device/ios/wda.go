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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"GADS/common/models"

	"github.com/danielpaulus/go-ios/ios/testmanagerd"
)

// runWDA launches WebDriverAgent via go-ios testmanagerd and blocks until WDA
// exits or the context is cancelled. On failure or unexpected exit it resets
// the device. Must be run in a goroutine from Setup.
func (d *IOSDevice) runWDA(ctx context.Context) {
	testCfg := testmanagerd.TestConfig{
		BundleId:           d.cfg.WdaBundleID,
		TestRunnerBundleId: d.cfg.WdaBundleID,
		XctestConfigName:   "WebDriverAgentRunner.xctest",
		Env:                nil,
		Args:               nil,
		TestsToRun:         nil,
		TestsToSkip:        nil,
		XcTest:             false,
		Device:             d.goIOSEntry,
		Listener:           testmanagerd.NewTestListener(io.Discard, io.Discard, os.TempDir()),
	}

	_, err := testmanagerd.RunTestWithConfig(ctx, testCfg)
	if err != nil {
		d.Reset("WebDriverAgent exited with error: " + err.Error())
	}
}

// checkWDAUp polls the WDA /status endpoint until it returns HTTP 200 or the
// 60-second timeout elapses. On success it sends true to d.wdaReadyChan; on
// timeout it resets the device.
func (d *IOSDevice) checkWDAUp() {
	client := &http.Client{Timeout: 30 * time.Second}
	url := fmt.Sprintf("http://localhost:%s/status", d.wdaPort)
	req, _ := http.NewRequest(http.MethodGet, url, nil)

	for range 60 {
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			d.wdaReadyChan <- true
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	d.Reset("WebDriverAgent did not respond within 60 seconds")
}

// updateWDAStreamSettings pushes MJPEG stream settings (FPS, quality, scaling)
// to WDA via the /appium/settings endpoint. This must be called after WDA is
// confirmed up and before streaming begins.
func (d *IOSDevice) updateWDAStreamSettings() error {
	settings := models.WDAMjpegSettings{
		Settings: models.WDAMjpegProperties{
			MjpegServerFramerate:         d.info.StreamTargetFPS,
			MjpegServerScreenshotQuality: d.info.StreamJpegQuality,
			MjpegServerScalingFactor:     d.info.StreamScalingFactor,
		},
	}

	payload, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("updateWDAStreamSettings %s: marshal: %w", d.info.UDID, err)
	}

	url := fmt.Sprintf("http://localhost:%s/appium/settings", d.wdaPort)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("updateWDAStreamSettings %s: build request: %w", d.info.UDID, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.http.Do(req)
	if err != nil {
		return fmt.Errorf("updateWDAStreamSettings %s: %w", d.info.UDID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("updateWDAStreamSettings %s: unexpected status %d", d.info.UDID, resp.StatusCode)
	}
	return nil
}
