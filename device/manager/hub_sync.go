/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"GADS/common/models"
	"GADS/device"
)

// ProviderPayload is the JSON body posted to the hub's /provider-update endpoint.
// It replaces models.ProviderData in the new manager, using device.DeviceInfo
// instead of the legacy models.Device god-struct. The JSON field names are
// identical so the hub can decode either format.
type ProviderPayload struct {
	Provider   models.Provider    `json:"provider"`
	DeviceData []*device.DeviceInfo `json:"device_data"`
}

// syncToHub marshals the current device state and POSTs it to
// {hubURL}/provider-update. Returns a non-nil error if the request fails or
// the hub returns a non-200 status.
func syncToHub(client *http.Client, hubURL string, cfg models.Provider, infos []*device.DeviceInfo) error {
	payload := ProviderPayload{
		Provider:   cfg,
		DeviceData: infos,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("syncToHub: marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/provider-update", hubURL),
		bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("syncToHub: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("syncToHub: POST: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("syncToHub: hub returned %d", resp.StatusCode)
	}
	return nil
}
