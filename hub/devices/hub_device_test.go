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
	"net"
	"testing"
	"time"
)

// fakeConn is a minimal net.Conn that tracks whether Close was called.
type fakeConn struct {
	closed bool
}

func (f *fakeConn) Close() error                       { f.closed = true; return nil }
func (f *fakeConn) Read(b []byte) (int, error)         { return 0, nil }
func (f *fakeConn) Write(b []byte) (int, error)        { return 0, nil }
func (f *fakeConn) LocalAddr() net.Addr                { return nil }
func (f *fakeConn) RemoteAddr() net.Addr               { return nil }
func (f *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (f *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (f *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

// --- AcquireLock / ReleaseLock ---

func TestAcquireLock_Success(t *testing.T) {
	d := &LocalHubDevice{}
	d.Mu.Lock()
	err := d.AcquireLock("alice", "tenantA", LockSourceUI)
	d.Mu.Unlock()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d.InUseBy != "alice" || d.InUseByTenant != "tenantA" || d.LockSource != LockSourceUI {
		t.Error("lock fields not set correctly")
	}
	if d.InUseTS == 0 {
		t.Error("InUseTS should be set")
	}
}

func TestAcquireLock_FailsWhenLockedByOther(t *testing.T) {
	d := &LocalHubDevice{}
	d.InUseWSConnection = &fakeConn{}
	d.InUseBy = "alice"
	d.InUseByTenant = "tenantA"
	d.InUseTS = time.Now().UnixMilli()

	d.Mu.Lock()
	err := d.AcquireLock("bob", "tenantA", LockSourceUI)
	d.Mu.Unlock()

	if err == nil {
		t.Fatal("expected error when device locked by another user")
	}
}

func TestAcquireLock_SameUserSucceeds(t *testing.T) {
	d := &LocalHubDevice{}
	d.InUseBy = "alice"
	d.InUseByTenant = "tenantA"
	d.InUseTS = time.Now().UnixMilli()

	d.Mu.Lock()
	err := d.AcquireLock("alice", "tenantA", LockSourceUI)
	d.Mu.Unlock()

	if err != nil {
		t.Fatalf("same user should be allowed to re-acquire, got: %v", err)
	}
}

func TestReleaseLock_ClearsAllFields(t *testing.T) {
	d := &LocalHubDevice{}
	conn := &fakeConn{}
	d.InUseWSConnection = conn
	d.InUseBy = "alice"
	d.InUseByTenant = "tenantA"
	d.InUseTS = time.Now().UnixMilli()
	d.LockSource = LockSourceUI
	d.LeaseExpiresAt = time.Now().Add(10 * time.Minute).UnixMilli()

	d.Mu.Lock()
	d.ReleaseLock()
	d.Mu.Unlock()

	if d.InUseBy != "" || d.InUseByTenant != "" || d.InUseTS != 0 {
		t.Error("user fields should be cleared")
	}
	if d.LockSource != "" || d.LeaseExpiresAt != 0 {
		t.Error("lease fields should be cleared")
	}
	if d.InUseWSConnection != nil {
		t.Error("WS connection should be nil")
	}
	if !conn.closed {
		t.Error("WS connection should have been closed")
	}
}

// --- IsLocked ---

func TestIsLocked_UISession(t *testing.T) {
	d := &LocalHubDevice{}
	d.InUseWSConnection = &fakeConn{}

	d.Mu.RLock()
	locked := d.IsLocked()
	d.Mu.RUnlock()

	if !locked {
		t.Error("should be locked when WS connection present")
	}
}

func TestIsLocked_APILease(t *testing.T) {
	d := &LocalHubDevice{}
	d.LockSource = LockSourceAPI
	d.LeaseExpiresAt = time.Now().Add(5 * time.Minute).UnixMilli()

	d.Mu.RLock()
	locked := d.IsLocked()
	d.Mu.RUnlock()

	if !locked {
		t.Error("should be locked with active API lease")
	}
}

func TestIsLocked_ExpiredAPILease(t *testing.T) {
	d := &LocalHubDevice{}
	d.LockSource = LockSourceAPI
	d.LeaseExpiresAt = time.Now().Add(-1 * time.Minute).UnixMilli()

	d.Mu.RLock()
	locked := d.IsLocked()
	d.Mu.RUnlock()

	if locked {
		t.Error("should not be locked with expired API lease")
	}
}

func TestIsLocked_RecentTimestamp(t *testing.T) {
	d := &LocalHubDevice{}
	d.InUseTS = time.Now().UnixMilli()

	d.Mu.RLock()
	locked := d.IsLocked()
	d.Mu.RUnlock()

	if !locked {
		t.Error("should be locked with recent InUseTS")
	}
}

func TestIsLocked_StaleTimestamp(t *testing.T) {
	d := &LocalHubDevice{}
	d.InUseTS = time.Now().Add(-10 * time.Second).UnixMilli()

	d.Mu.RLock()
	locked := d.IsLocked()
	d.Mu.RUnlock()

	if locked {
		t.Error("should not be locked with stale InUseTS")
	}
}

func TestIsLocked_ZeroTimestamp(t *testing.T) {
	d := &LocalHubDevice{}

	d.Mu.RLock()
	locked := d.IsLocked()
	d.Mu.RUnlock()

	if locked {
		t.Error("should not be locked when InUseTS is zero")
	}
}

// --- IsLockedByOther ---

func TestIsLockedByOther_NotLocked(t *testing.T) {
	d := &LocalHubDevice{}

	d.Mu.RLock()
	result := d.IsLockedByOther("alice", "tenantA")
	d.Mu.RUnlock()

	if result {
		t.Error("should not be locked by other when InUseBy is empty")
	}
}

func TestIsLockedByOther_SameUser(t *testing.T) {
	d := &LocalHubDevice{}
	d.InUseBy = "alice"
	d.InUseByTenant = "tenantA"
	d.InUseWSConnection = &fakeConn{}

	d.Mu.RLock()
	result := d.IsLockedByOther("alice", "tenantA")
	d.Mu.RUnlock()

	if result {
		t.Error("should not report locked by other when same user")
	}
}

func TestIsLockedByOther_DifferentUser(t *testing.T) {
	d := &LocalHubDevice{}
	d.InUseBy = "alice"
	d.InUseByTenant = "tenantA"
	d.InUseWSConnection = &fakeConn{}

	d.Mu.RLock()
	result := d.IsLockedByOther("bob", "tenantA")
	d.Mu.RUnlock()

	if !result {
		t.Error("should be locked by other when different user holds WS session")
	}
}

func TestIsLockedByOther_DifferentTenant(t *testing.T) {
	d := &LocalHubDevice{}
	d.InUseBy = "alice"
	d.InUseByTenant = "tenantA"
	d.InUseWSConnection = &fakeConn{}

	d.Mu.RLock()
	result := d.IsLockedByOther("alice", "tenantB")
	d.Mu.RUnlock()

	if !result {
		t.Error("should be locked by other when same user but different tenant")
	}
}

// --- ReleaseLockIfNotHeld ---

func TestReleaseLockIfNotHeld_WithUISession(t *testing.T) {
	d := &LocalHubDevice{}
	conn := &fakeConn{}
	d.InUseWSConnection = conn
	d.InUseBy = "alice"
	d.InUseTS = time.Now().UnixMilli()

	d.Mu.Lock()
	d.ReleaseLockIfNotHeld()
	d.Mu.Unlock()

	if d.InUseBy == "" {
		t.Error("should not clear user when UI session is active")
	}
	if conn.closed {
		t.Error("should not close WS connection")
	}
}

func TestReleaseLockIfNotHeld_WithActiveLease(t *testing.T) {
	d := &LocalHubDevice{}
	d.LockSource = LockSourceAPI
	d.LeaseExpiresAt = time.Now().Add(5 * time.Minute).UnixMilli()
	d.InUseBy = "alice"
	d.InUseTS = time.Now().UnixMilli()

	d.Mu.Lock()
	d.ReleaseLockIfNotHeld()
	d.Mu.Unlock()

	if d.InUseBy == "" {
		t.Error("should not clear user when API lease is active")
	}
}

func TestReleaseLockIfNotHeld_NoHold(t *testing.T) {
	d := &LocalHubDevice{}
	d.InUseBy = "alice"
	d.InUseByTenant = "tenantA"
	d.InUseTS = time.Now().Add(-10 * time.Second).UnixMilli()

	d.Mu.Lock()
	d.ReleaseLockIfNotHeld()
	d.Mu.Unlock()

	if d.InUseBy != "" || d.InUseByTenant != "" {
		t.Error("should clear user fields when not held by UI or API")
	}
}

// --- RefreshLock ---

func TestRefreshLock_UpdatesTimestamp(t *testing.T) {
	d := &LocalHubDevice{}
	d.InUseTS = 1000

	before := time.Now().UnixMilli()
	d.Mu.Lock()
	d.RefreshLock()
	d.Mu.Unlock()

	if d.InUseTS < before {
		t.Error("RefreshLock should update InUseTS to now")
	}
}

// --- SetWSConnection / ClearWSConnection ---

func TestSetAndClearWSConnection(t *testing.T) {
	d := &LocalHubDevice{}
	conn := &fakeConn{}

	d.Mu.Lock()
	d.SetWSConnection(conn)
	d.Mu.Unlock()

	if d.InUseWSConnection == nil {
		t.Error("connection should be set")
	}

	d.Mu.Lock()
	d.ClearWSConnection()
	d.Mu.Unlock()

	if d.InUseWSConnection != nil {
		t.Error("connection should be nil after clear")
	}
	if conn.closed {
		t.Error("ClearWSConnection should not close the connection")
	}
}
