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
	"fmt"

	"GADS/common/models"
	"GADS/provider/devices"
	"GADS/provider/devices/android"
	"GADS/provider/devices/ios"
	"GADS/provider/devices/tizen"
	"GADS/provider/devices/webos"
)

// DeviceFactory creates platform-specific ManagedDevice instances from a
// DeviceInfo. Implementations are responsible for injecting dependencies.
type DeviceFactory interface {
	// Create builds and returns a ManagedDevice for the platform identified by
	// info.OS. log is the per-device logger. Returns an error if info.OS is
	// not recognised or a required dependency is unavailable.
	Create(info *models.DeviceInfo, log models.CustomLogger) (devices.ManagedDevice, error)
}

// DefaultDeviceFactory is the production DeviceFactory. It creates
// platform-specific device instances wired with the shared dependency set
// passed to NewDefaultDeviceFactory.
type DefaultDeviceFactory struct {
	cmd   devices.CommandRunner
	ports devices.PortAllocator
	store devices.DeviceStore
	http  devices.HTTPClient
	cfg   *models.Provider
}

// NewDefaultDeviceFactory constructs a DefaultDeviceFactory with shared
// dependency implementations. All parameters must be non-nil.
func NewDefaultDeviceFactory(
	cmd devices.CommandRunner,
	ports devices.PortAllocator,
	store devices.DeviceStore,
	httpClient devices.HTTPClient,
	cfg *models.Provider,
) *DefaultDeviceFactory {
	return &DefaultDeviceFactory{
		cmd:   cmd,
		ports: ports,
		store: store,
		http:  httpClient,
		cfg:   cfg,
	}
}

// Create builds a ManagedDevice for info.OS. Returns an error for unknown OS values.
func (f *DefaultDeviceFactory) Create(info *models.DeviceInfo, log models.CustomLogger) (devices.ManagedDevice, error) {
	switch info.OS {
	case "android":
		return android.New(info, f.cmd, f.ports, f.store, f.http, log, f.cfg), nil
	case "ios":
		return ios.New(info, f.cmd, f.ports, f.store, f.http, log, f.cfg), nil
	case "tizen":
		return tizen.New(info, f.cmd, f.ports, f.store, log, f.cfg), nil
	case "webos":
		return webos.New(info, f.cmd, f.ports, f.store, log, f.cfg), nil
	default:
		return nil, fmt.Errorf("DeviceFactory: unsupported OS %q for device %s", info.OS, info.UDID)
	}
}
