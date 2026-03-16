/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package device

import (
	"context"
	"net/http"

	"GADS/common/models"
)

// RunningProcess represents a long-lived subprocess started by CommandRunner.Start.
// It provides access to the process's PID for lifecycle management (e.g. signalling,
// waiting for exit).
type RunningProcess struct {
	// PID is the OS process ID of the running subprocess.
	PID int

	// Done is closed when the subprocess exits. Callers can select on this
	// channel to detect unexpected termination.
	Done <-chan struct{}

	// stop is called internally to terminate the subprocess.
	stop func()
}

// Stop signals the subprocess to terminate. It is safe to call multiple times.
func (p *RunningProcess) Stop() {
	if p.stop != nil {
		p.stop()
	}
}

// CommandRunner abstracts os/exec for testability. Platform device types use
// this interface instead of calling exec.Command directly, so unit tests can
// inject a mock that returns canned output without spawning real processes.
type CommandRunner interface {
	// Run executes the named command with the given arguments, waits for it to
	// complete, and returns the combined stdout+stderr output. ctx can be used
	// to cancel the command.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)

	// Start launches the named command without waiting for it to finish.
	// It returns a RunningProcess handle that can be used to monitor or stop
	// the subprocess. ctx cancellation will also terminate the process.
	Start(ctx context.Context, name string, args ...string) (*RunningProcess, error)
}

// PortAllocator abstracts the allocation and release of host TCP ports.
// Device types use this interface to obtain free ports during setup and release
// them on reset, replacing direct access to the global UsedPorts map in
// providerutil.
type PortAllocator interface {
	// GetFreePort returns an unused host TCP port as a string (e.g. "10100").
	// The returned port is marked as in-use and will not be returned by a
	// subsequent call until FreePort is called.
	GetFreePort() (string, error)

	// FreePort marks the given port as available for reuse. It should be called
	// during device reset after all services using the port have been stopped.
	FreePort(port string)
}

// DeviceStore abstracts the database operations that device types need during
// their lifecycle. Production code uses MongoDeviceStore (device/deps_impl.go)
// which delegates to db.GlobalMongoStore. Tests inject an in-memory mock.
type DeviceStore interface {
	// AddOrUpdateDevice upserts the DeviceInfo record into the device registry
	// (MongoDB "new_devices" collection).
	AddOrUpdateDevice(info *DeviceInfo) error

	// GetDeviceStreamSettings retrieves the per-device stream configuration
	// (FPS, JPEG quality, scaling factor) for the given UDID.
	GetDeviceStreamSettings(udid string) (models.DeviceStreamSettings, error)

	// GetGlobalStreamSettings retrieves the provider-wide default stream settings,
	// creating a default record if none exists.
	GetGlobalStreamSettings() (models.StreamSettings, error)

	// GetTURNConfig retrieves the TURN server configuration used for WebRTC
	// streaming. Returns a disabled default config if none is stored.
	GetTURNConfig() (models.TURNConfig, error)
}

// HTTPClient abstracts outbound HTTP calls made by device types (to WDA, the
// GADS-Settings remote control server, Appium, etc.). Injecting this interface
// allows tests to intercept and verify HTTP requests without a real server.
type HTTPClient interface {
	// Do executes the given HTTP request and returns the response.
	Do(req *http.Request) (*http.Response, error)
}
