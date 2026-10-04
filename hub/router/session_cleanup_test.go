package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExpiredSessionWaitsForCleanup(t *testing.T) {
	started, finish := make(chan struct{}), make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-finish
		w.Write([]byte(`{"value":null}`))
	}))
	defer provider.Close()
	defer func() {
		select {
		case <-finish:
		default:
			close(finish)
		}
	}()
	d, cleanup := newGridSessionDevice("cleanup-device", "session-a", strings.TrimPrefix(provider.URL, "http://"))
	defer cleanup()
	d.AppiumNewCommandTimeout = 1
	d.LastAutomationActionTS = time.Now().Add(-time.Minute).UnixMilli()
	sweepExpiredGridSessions()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("cleanup did not start")
	}
	// A concurrent release or failed-create callback must leave cleanup in
	// charge of the claim, not merely rely on device selection to reject it.
	d.Mu.Lock()
	d.ReleaseFromAutomation()
	d.Mu.Unlock()
	abortAutomationClaim(d)
	d.Mu.RLock()
	assert.True(t, d.IsRunningAutomation)
	assert.False(t, d.IsAvailableForAutomation, "session B must not claim A's device during DELETE")
	assert.Equal(t, "session-a", d.SessionID)
	d.Mu.RUnlock()
	for _, candidate := range []gridCandidate{{DeviceUDID: "cleanup-device"}, {PlatformName: "Android"}} {
		found, err := findAvailableDevice(candidate, []string{"ws1"}, "test-user", "tenant1")
		assert.Error(t, err)
		assert.Nil(t, found)
	}
	close(finish)
	assert.Eventually(t, func() bool {
		d.Mu.RLock()
		defer d.Mu.RUnlock()
		return d.IsAvailableForAutomation && d.SessionID == ""
	}, 3*time.Second, time.Millisecond)
	found, err := findAvailableDevice(gridCandidate{DeviceUDID: "cleanup-device"}, []string{"ws1"}, "test-user", "tenant1")
	assert.NoError(t, err)
	assert.Same(t, d, found)
}

func TestCleanupFailureKeepsDeviceReserved(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer provider.Close()
	d, cleanup := newGridSessionDevice("failed-cleanup-device", "session-a", strings.TrimPrefix(provider.URL, "http://"))
	defer cleanup()
	d.Mu.Lock()
	startSessionCleanup(d)
	d.Mu.Unlock()
	assert.Eventually(t, func() bool { d.Mu.RLock(); defer d.Mu.RUnlock(); return !d.SessionCleanupInProgress }, 3*time.Second, time.Millisecond)
	d.Mu.RLock()
	defer d.Mu.RUnlock()
	assert.False(t, d.IsAvailableForAutomation)
	assert.Equal(t, "session-a", d.SessionID)
}
