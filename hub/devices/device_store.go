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
	"GADS/common/models"
	"sort"
	"sync"
)

// DeviceStore is a concurrent-safe store for LocalHubDevice values.
// It uses a single RWMutex to protect the map (allowing concurrent reads),
// while each device has its own Mu for protecting its fields.
type DeviceStore struct {
	mu      sync.RWMutex
	devices map[string]*models.LocalHubDevice
}

func NewDeviceStore() *DeviceStore {
	return &DeviceStore{
		devices: make(map[string]*models.LocalHubDevice),
	}
}

// HubDeviceStore is the package-level singleton used by hub components.
var HubDeviceStore = NewDeviceStore()

// Get returns the device for the given UDID and whether it was found.
func (s *DeviceStore) Get(udid string) (*models.LocalHubDevice, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.devices[udid]
	return d, ok
}

// Set adds or replaces the device for the given UDID.
func (s *DeviceStore) Set(udid string, d *models.LocalHubDevice) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devices[udid] = d
}

// Delete removes the device with the given UDID from the store.
func (s *DeviceStore) Delete(udid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.devices, udid)
}

// All returns a snapshot slice of all devices. Callers may iterate freely;
// use each device's own Mu to protect field access.
func (s *DeviceStore) All() []*models.LocalHubDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*models.LocalHubDevice, 0, len(s.devices))
	for _, d := range s.devices {
		result = append(result, d)
	}
	return result
}

// AllSorted returns a snapshot slice of all devices ordered by UDID.
// Callers may iterate freely; use each device's own Mu to protect field access.
func (s *DeviceStore) AllSorted() []*models.LocalHubDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.devices))
	for k := range s.devices {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]*models.LocalHubDevice, 0, len(keys))
	for _, k := range keys {
		result = append(result, s.devices[k])
	}
	return result
}

// Len returns the number of devices in the store.
func (s *DeviceStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.devices)
}
