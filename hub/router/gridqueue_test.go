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
	"GADS/common/models"
	"GADS/hub/devices"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// acquireResult carries one Acquire outcome from a waiter goroutine to the test
type acquireResult struct {
	device     *devices.LocalHubDevice
	err        error
	clientGone bool
}

// acquireAsync runs gridQueue.Acquire in a goroutine and returns the channel its
// result lands on
func acquireAsync(candidate gridCandidate, user gridUser, wait time.Duration, ctxDone <-chan struct{}) <-chan acquireResult {
	results := make(chan acquireResult, 1)
	go func() {
		device, err, clientGone := gridQueue.Acquire(candidate, user, wait, ctxDone)
		results <- acquireResult{device: device, err: err, clientGone: clientGone}
	}()
	return results
}

// waitAcquire receives an Acquire outcome with a hard timeout so a broken queue
// fails the test instead of hanging it
func waitAcquire(t *testing.T, results <-chan acquireResult) acquireResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("Acquire did not return within 5 seconds")
		return acquireResult{}
	}
}

// freeDevice releases the device from automation the way the real code paths do,
// which also pokes the queue dispatcher
func freeDevice(device *devices.LocalHubDevice) {
	device.Mu.Lock()
	device.ReleaseFromAutomation()
	device.Mu.Unlock()
}

func TestQueueWaitDuration(t *testing.T) {
	seconds := func(s int64) *int64 { return &s }

	tests := []struct {
		name         string
		queueTimeout *int64
		want         time.Duration
	}{
		{"absent keeps the historical 10s default", nil, 10 * time.Second},
		{"explicit value used as-is", seconds(90), 90 * time.Second},
		{"clamped to the 300s maximum", seconds(400), 300 * time.Second},
		{"zero means no waiting", seconds(0), 0},
		{"negative clamped to no waiting", seconds(-5), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, queueWaitDuration(gridCandidate{QueueTimeout: tt.queueTimeout}))
		})
	}
}

func TestGridQueueFIFOOrder(t *testing.T) {
	device, cleanup := newGridSessionDevice("queue-fifo-device", "", "fake-host")
	defer cleanup()

	user := gridUser{UserID: "test-user", Tenant: "tenant1", AllowedWorkspaceIDs: []string{"ws1"}}
	candidate := gridCandidate{DeviceUDID: "queue-fifo-device"}

	// Enqueue two waiters for the same busy device, synchronizing on queue depth
	// so the enqueue order is deterministic
	resultsA := acquireAsync(candidate, user, 5*time.Second, nil)
	assert.Eventually(t, func() bool { return gridQueue.depth() == 1 }, 2*time.Second, 10*time.Millisecond)
	resultsB := acquireAsync(candidate, user, 5*time.Second, nil)
	assert.Eventually(t, func() bool { return gridQueue.depth() == 2 }, 2*time.Second, 10*time.Millisecond)

	// Free the device - the FIRST enqueued waiter must get it while the second
	// keeps waiting
	freeDevice(device)
	resultA := waitAcquire(t, resultsA)
	assert.Same(t, device, resultA.device)
	assert.False(t, resultA.clientGone)
	assert.Eventually(t, func() bool { return gridQueue.depth() == 1 }, 2*time.Second, 10*time.Millisecond)

	// Free it again - now the second waiter's turn
	freeDevice(device)
	resultB := waitAcquire(t, resultsB)
	assert.Same(t, device, resultB.device)

	// Acquire claims the device on delivery
	device.Mu.RLock()
	defer device.Mu.RUnlock()
	assert.False(t, device.IsAvailableForAutomation)
}

func TestGridQueueTimeout(t *testing.T) {
	_, cleanup := newGridSessionDevice("queue-timeout-device", "", "fake-host")
	defer cleanup()

	user := gridUser{UserID: "test-user", Tenant: "tenant1", AllowedWorkspaceIDs: []string{"ws1"}}
	candidate := gridCandidate{DeviceUDID: "queue-timeout-device"}

	start := time.Now()
	device, err, clientGone := gridQueue.Acquire(candidate, user, 1*time.Second, nil)
	elapsed := time.Since(start)

	assert.Nil(t, device)
	assert.False(t, clientGone)
	// The timeout response carries the last matching error, same as the old retry loop
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "not available for automation")
	}
	assert.GreaterOrEqual(t, elapsed, 1*time.Second)
	assert.Less(t, elapsed, 3*time.Second)
}

func TestGridQueueZeroWaitFailsImmediately(t *testing.T) {
	_, cleanup := newGridSessionDevice("queue-zero-wait-device", "", "fake-host")
	defer cleanup()

	user := gridUser{UserID: "test-user", Tenant: "tenant1", AllowedWorkspaceIDs: []string{"ws1"}}
	candidate := gridCandidate{DeviceUDID: "queue-zero-wait-device"}

	start := time.Now()
	device, err, clientGone := gridQueue.Acquire(candidate, user, 0, nil)

	assert.Nil(t, device)
	assert.False(t, clientGone)
	assert.Error(t, err)
	assert.Less(t, time.Since(start), 500*time.Millisecond)
}

func TestGridQueueCancelledWaiterLeavesTheQueue(t *testing.T) {
	device, cleanup := newGridSessionDevice("queue-cancel-device", "", "fake-host")
	defer cleanup()

	user := gridUser{UserID: "test-user", Tenant: "tenant1", AllowedWorkspaceIDs: []string{"ws1"}}
	candidate := gridCandidate{DeviceUDID: "queue-cancel-device"}

	ctxDoneA := make(chan struct{})
	resultsA := acquireAsync(candidate, user, 5*time.Second, ctxDoneA)
	assert.Eventually(t, func() bool { return gridQueue.depth() == 1 }, 2*time.Second, 10*time.Millisecond)
	resultsB := acquireAsync(candidate, user, 5*time.Second, nil)
	assert.Eventually(t, func() bool { return gridQueue.depth() == 2 }, 2*time.Second, 10*time.Millisecond)

	// Cancel the first waiter - it must report the client gone without a device
	close(ctxDoneA)
	resultA := waitAcquire(t, resultsA)
	assert.True(t, resultA.clientGone)
	assert.Nil(t, resultA.device)

	// The freed device must skip the cancelled waiter and go to the second one
	freeDevice(device)
	resultB := waitAcquire(t, resultsB)
	assert.Same(t, device, resultB.device)
}

func TestGridCreateSessionQueueTimeoutOverHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prevGridDB := gridDB
	defer func() { gridDB = prevGridDB }()
	gridDB = &fakeGridStore{
		credential: models.ClientCredentials{UserID: "test-user", Tenant: "tenant1", IsActive: true},
		workspaces: []models.Workspace{{ID: "ws1", Tenant: "tenant1"}},
	}

	_, cleanup := newGridSessionDevice("queue-http-device", "", "fake-host")
	defer cleanup()

	// The only matching device is busy and `gads:queueTimeout: 1` bounds the wait -
	// the request must come back as a 500 `session not created` after about a second
	sessionBody := `{"capabilities":{"alwaysMatch":{"platformName":"Android","gads:clientSecret":"test-secret","appium:udid":"queue-http-device","gads:queueTimeout":1}}}`

	router := newGridTestRouter()
	req, _ := http.NewRequest("POST", "/grid/session", bytes.NewBufferString(sessionBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	start := time.Now()
	router.ServeHTTP(w, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.GreaterOrEqual(t, elapsed, 1*time.Second)
	assert.Less(t, elapsed, 3*time.Second)

	var response SeleniumSessionErrorResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "session not created", response.Value.Error)
	assert.Contains(t, response.Value.Message, "not available for automation")
}
