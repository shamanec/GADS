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
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"GADS/common/models"
)

// fakeSetupDevice is a PlatformDevice whose Setup blocks until release is closed.
type fakeSetupDevice struct {
	RuntimeState
	setups  atomic.Int32
	release chan struct{}
}

func (d *fakeSetupDevice) Setup() error {
	d.setups.Add(1)
	<-d.release
	return nil
}

func (d *fakeSetupDevice) InstallApp(string) error                       { return nil }
func (d *fakeSetupDevice) UninstallApp(string) error                     { return nil }
func (d *fakeSetupDevice) GetInstalledApps() ([]models.DeviceApp, error) { return nil, nil }
func (d *fakeSetupDevice) GetInstalledAppBundleIDs() []string            { return nil }
func (d *fakeSetupDevice) LaunchApp(string) error                        { return nil }
func (d *fakeSetupDevice) KillApp(string) error                          { return nil }
func (d *fakeSetupDevice) AppiumCapabilities() models.AppiumServerCapabilities {
	return models.AppiumServerCapabilities{}
}

func waitSetupReturned(t *testing.T, d *fakeSetupDevice) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for d.setupRunning.Load() {
		if time.Now().After(deadline) {
			t.Fatal("Setup did not return")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStartSetupKeepsContextOfRunningSetup(t *testing.T) {
	d := &fakeSetupDevice{release: make(chan struct{})}

	startSetup(d)
	firstCtx := d.GetContext()

	// A Reset cancels the context while Setup still runs, and the sync loop
	// tries to start the device again on its next tick
	d.CtxCancel()
	startSetup(d)

	if d.GetContext() != firstCtx {
		t.Error("context replaced under a running Setup")
	}

	close(d.release)
	waitSetupReturned(t, d)
	if n := d.setups.Load(); n != 1 {
		t.Fatalf("Setup ran %d times while the first run was going, expected 1", n)
	}

	startSetup(d)
	if d.GetContext() == firstCtx {
		t.Error("next Setup did not get a fresh context")
	}
	waitSetupReturned(t, d)
	if n := d.setups.Load(); n != 2 {
		t.Errorf("Setup ran %d times after the first run returned, expected 2", n)
	}
}

func TestSetLiveUnlessReset(t *testing.T) {
	var r RuntimeState
	ctx, cancel := context.WithCancel(context.Background())
	r.ProviderState = "preparing"

	if err := r.setLiveUnlessReset(ctx); err != nil {
		t.Fatalf("setLiveUnlessReset() = %v, expected nil", err)
	}
	if r.ProviderState != "live" {
		t.Errorf("ProviderState = %q, expected live", r.ProviderState)
	}

	// A Reset cancelled the context and put the device back in init
	cancel()
	r.ProviderState = "init"
	err := r.setLiveUnlessReset(ctx)
	if !errors.Is(err, errResetDuringSetup) {
		t.Errorf("setLiveUnlessReset() = %v, expected errResetDuringSetup", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Error("error wraps context.Canceled, setup backoff would be skipped")
	}
	if r.ProviderState != "init" {
		t.Errorf("reset device marked %q", r.ProviderState)
	}
}
