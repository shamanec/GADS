/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"GADS/device"
)

var appiumNetClient = &http.Client{
	Timeout: time.Second * 120,
}

// appiumRequest executes an HTTP request against the Appium server for the
// device, scoped to the current Appium session. method is an HTTP verb,
// endpoint is the path without a leading slash.
func appiumRequest(info *device.DeviceInfo, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/session/%s/%s", info.AppiumPort, info.AppiumSessionID, endpoint)
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return appiumNetClient.Do(req)
}

// appiumSource fetches the Appium page source for the device.
func appiumSource(info *device.DeviceInfo) (*http.Response, error) {
	return appiumRequest(info, http.MethodGet, "source", nil)
}
