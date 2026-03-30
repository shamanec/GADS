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
	"sort"
	"sync"
)

// DeviceStore is a concurrent-safe store for PlatformDevice values.
// It encapsulates the map and mutex so callers never touch the lock directly.
// Each device has its own Mutex (on RuntimeState) for per-device field protection.
type DeviceStore struct {
	mu      sync.RWMutex
	devices map[string]PlatformDevice
}

// NewDeviceStore creates an empty DeviceStore ready for use.
func NewDeviceStore() *DeviceStore {
	return &DeviceStore{
		devices: make(map[string]PlatformDevice),
	}
}

// Get returns the device for the given UDID and whether it was found.
func (s *DeviceStore) Get(udid string) (PlatformDevice, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.devices[udid]
	return d, ok
}

// Set adds or replaces the device for the given UDID.
func (s *DeviceStore) Set(udid string, dev PlatformDevice) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devices[udid] = dev
}

// Delete removes the device with the given UDID from the store.
func (s *DeviceStore) Delete(udid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.devices, udid)
}

// All returns a snapshot slice of all devices. The caller may iterate freely
// without holding the store lock. Use each device's own Mutex to protect field access.
func (s *DeviceStore) All() []PlatformDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PlatformDevice, 0, len(s.devices))
	for _, d := range s.devices {
		result = append(result, d)
	}
	return result
}

// AllSorted returns a snapshot slice of all devices ordered by UDID.
func (s *DeviceStore) AllSorted() []PlatformDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.devices))
	for k := range s.devices {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]PlatformDevice, 0, len(keys))
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

// ForEach calls fn for each device in the store while holding a read lock.
// Do not call store methods from within fn to avoid deadlock.
func (s *DeviceStore) ForEach(fn func(udid string, dev PlatformDevice)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for udid, dev := range s.devices {
		fn(udid, dev)
	}
}

// UDIDs returns a slice of all device UDIDs currently in the store.
func (s *DeviceStore) UDIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, 0, len(s.devices))
	for udid := range s.devices {
		result = append(result, udid)
	}
	return result
}
