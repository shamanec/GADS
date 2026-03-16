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
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"GADS/common/db"
	"GADS/common/models"

	"go.mongodb.org/mongo-driver/bson"
)

// ---------------------------------------------------------------------------
// ExecCommandRunner
// ---------------------------------------------------------------------------

// ExecCommandRunner is the production implementation of CommandRunner. It
// executes real OS processes via os/exec. Use this in provider code; inject
// a mock in tests.
type ExecCommandRunner struct{}

// NewExecCommandRunner returns a ready-to-use ExecCommandRunner.
func NewExecCommandRunner() *ExecCommandRunner {
	return &ExecCommandRunner{}
}

// Run executes name with args, waits for completion, and returns the combined
// stdout+stderr output as bytes. ctx cancellation terminates the process.
func (r *ExecCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return out.Bytes(), fmt.Errorf("run %s: %w", name, err)
	}
	return out.Bytes(), nil
}

// Start launches name with args without waiting for it to finish. The returned
// RunningProcess can be used to monitor or stop the subprocess. ctx
// cancellation also terminates the process.
func (r *ExecCommandRunner) Start(ctx context.Context, name string, args ...string) (*RunningProcess, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}

	done := make(chan struct{})
	go func() {
		// Wait releases resources and populates cmd.ProcessState regardless of
		// whether the process exits cleanly or is killed.
		_ = cmd.Wait()
		close(done)
	}()

	proc := &RunningProcess{
		PID:  cmd.Process.Pid,
		Done: done,
		stop: func() {
			// Process.Kill is safe to call even after the process has exited.
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		},
	}
	return proc, nil
}

// ---------------------------------------------------------------------------
// NetPortAllocator
// ---------------------------------------------------------------------------

// NetPortAllocator is the production implementation of PortAllocator. It finds
// free TCP ports by binding to port 0 and tracks allocated ports in an internal
// map so the same port is not handed out twice before being released.
//
// This replaces direct access to providerutil.UsedPorts.
type NetPortAllocator struct {
	mu    sync.Mutex
	ports map[string]bool
}

// NewNetPortAllocator returns a new NetPortAllocator with an empty port registry.
func NewNetPortAllocator() *NetPortAllocator {
	return &NetPortAllocator{
		ports: make(map[string]bool),
	}
}

// GetFreePort finds an unused TCP port on the host, marks it as allocated, and
// returns it as a string (e.g. "10234"). Up to 10 attempts are made with linear
// back-off before an error is returned.
func (a *NetPortAllocator) GetFreePort() (string, error) {
	const (
		maxAttempts = 10
		baseBackoff = 50 * time.Millisecond
	)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
		if err != nil {
			return "", fmt.Errorf("GetFreePort: resolve tcp addr: %w", err)
		}

		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return "", fmt.Errorf("GetFreePort: listen tcp: %w", err)
		}

		port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
		l.Close()

		a.mu.Lock()
		if !a.ports[port] {
			a.ports[port] = true
			a.mu.Unlock()
			return port, nil
		}
		a.mu.Unlock()

		// Port already allocated to another device — back off and retry.
		time.Sleep(time.Duration(attempt) * baseBackoff)
	}

	return "", fmt.Errorf("GetFreePort: no free port found after %d attempts", maxAttempts)
}

// FreePort marks the given port as available for reuse. It should be called
// during device reset after all services using the port have been stopped.
func (a *NetPortAllocator) FreePort(port string) {
	a.mu.Lock()
	delete(a.ports, port)
	a.mu.Unlock()
}

// ---------------------------------------------------------------------------
// MongoDeviceStore
// ---------------------------------------------------------------------------

// MongoDeviceStore is the production implementation of DeviceStore. It
// delegates to an existing *db.MongoStore instance, allowing device types to
// read and write device-related data without importing global state directly.
type MongoDeviceStore struct {
	store *db.MongoStore
}

// NewMongoDeviceStore wraps the given MongoStore. In production, pass
// db.GlobalMongoStore.
func NewMongoDeviceStore(store *db.MongoStore) *MongoDeviceStore {
	return &MongoDeviceStore{store: store}
}

// AddOrUpdateDevice upserts the DeviceInfo record into the "new_devices"
// collection, keyed by UDID. Only fields with a bson tag (i.e. persisted
// fields) are written; runtime-only fields (bson:"-") are ignored by the
// driver.
func (s *MongoDeviceStore) AddOrUpdateDevice(info *DeviceInfo) error {
	coll := s.store.GetCollection("new_devices")
	filter := bson.D{{Key: "udid", Value: info.UDID}}
	return db.UpsertDocument(s.store.Ctx, coll, filter, *info)
}

// GetDeviceStreamSettings retrieves the per-device stream configuration
// (FPS, JPEG quality, scaling factor) for the given UDID.
func (s *MongoDeviceStore) GetDeviceStreamSettings(udid string) (models.DeviceStreamSettings, error) {
	return s.store.GetDeviceStreamSettings(udid)
}

// GetGlobalStreamSettings retrieves the provider-wide default stream settings,
// creating a default record if none exists.
func (s *MongoDeviceStore) GetGlobalStreamSettings() (models.StreamSettings, error) {
	return s.store.GetGlobalStreamSettings()
}

// GetTURNConfig retrieves the TURN server configuration used for WebRTC
// streaming. Returns a disabled default config if none is stored.
func (s *MongoDeviceStore) GetTURNConfig() (models.TURNConfig, error) {
	return s.store.GetTURNConfig()
}

// ---------------------------------------------------------------------------
// DefaultHTTPClient
// ---------------------------------------------------------------------------

// DefaultHTTPClient is the production implementation of HTTPClient. It wraps a
// standard *http.Client with a configurable timeout. Use NewDefaultHTTPClient
// to construct one; for device control calls a 30-second timeout is typical.
type DefaultHTTPClient struct {
	client *http.Client
}

// NewDefaultHTTPClient returns a DefaultHTTPClient with the given request
// timeout. Pass 0 to use Go's default (no timeout).
func NewDefaultHTTPClient(timeout time.Duration) *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{Timeout: timeout},
	}
}

// Do executes the given HTTP request using the underlying http.Client.
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}
