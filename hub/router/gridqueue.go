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
	"GADS/hub/devices"
	"strings"
	"sync"
	"time"
)

// How long a session request may wait for a device to free up. Absent
// `gads:queueTimeout` keeps the historical 10 second wait; explicit values are
// clamped to the maximum
const (
	defaultQueueWait   = 10 * time.Second
	maxQueueWait       = 300 * time.Second
	queueSweepInterval = 500 * time.Millisecond
)

// sessionWaiter is one queued `POST /grid/session` request waiting for a device.
// Once enqueued the dispatcher owns it: it either delivers a claimed device on the
// channel or closes the channel on timeout, client disconnect or a fail-fast error
type sessionWaiter struct {
	candidate gridCandidate
	user      gridUser
	deadline  time.Time
	// The request context's Done channel - nil means the waiter cannot be cancelled
	ctxDone <-chan struct{}
	deliver chan *devices.LocalHubDevice
	// Written by the dispatcher under the queue mutex and read by the handler only
	// after `deliver` is closed - the channel close orders the accesses
	lastErr    error
	clientGone bool
}

// sessionQueue hands free devices to session requests in FIFO order. A single
// dispatcher goroutine walks the waiters on device-freed notifications, enqueue
// kicks and a coarse tick (device availability can also appear via provider
// updates, which do not notify)
type sessionQueue struct {
	mu              sync.Mutex
	waiters         []*sessionWaiter
	kick            chan struct{}
	startDispatcher sync.Once
}

var gridQueue = &sessionQueue{kick: make(chan struct{}, 1)}

// queueWaitDuration resolves how long a session request may wait for a device -
// `gads:queueTimeout` seconds when the client sent it (clamped to [0, maxQueueWait],
// 0 means a single immediate attempt), otherwise the historical 10 second default
func queueWaitDuration(candidate gridCandidate) time.Duration {
	if candidate.QueueTimeout == nil {
		return defaultQueueWait
	}
	wait := time.Duration(*candidate.QueueTimeout) * time.Second
	if wait < 0 {
		return 0
	}
	if wait > maxQueueWait {
		return maxQueueWait
	}
	return wait
}

// isFailFastDeviceError reports whether a findAvailableDevice error is a static
// condition that waiting cannot fix - an unknown/unassigned UDID or a device whose
// usage forbids automation. The create handler maps these to their own responses
func isFailFastDeviceError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "No device with udid") ||
		strings.Contains(err.Error(), "is not enabled for automation")
}

// Acquire finds and claims a device matching the candidate, waiting up to `wait`
// behind earlier requests when none is free right now. It returns the claimed
// device, the most recent matching error (for the timeout response), and whether
// the client went away while waiting
func (q *sessionQueue) Acquire(candidate gridCandidate, user gridUser, wait time.Duration, ctxDone <-chan struct{}) (*devices.LocalHubDevice, error, bool) {
	q.startDispatcher.Do(func() { go q.dispatchLoop() })

	waiter := &sessionWaiter{
		candidate: candidate,
		user:      user,
		deadline:  time.Now().Add(wait),
		ctxDone:   ctxDone,
		deliver:   make(chan *devices.LocalHubDevice, 1),
	}

	q.mu.Lock()
	// A request may only skip the queue when nobody is already waiting - otherwise
	// a device freed a moment ago would go to the newcomer instead of the first
	// waiter in line
	if len(q.waiters) == 0 {
		device, err := findAvailableDevice(candidate, user.AllowedWorkspaceIDs, user.UserID, user.Tenant)
		if device != nil || isFailFastDeviceError(err) || wait <= 0 {
			q.mu.Unlock()
			return device, err, false
		}
		waiter.lastErr = err
	}
	q.waiters = append(q.waiters, waiter)
	q.mu.Unlock()
	q.wake()

	device, ok := <-waiter.deliver
	if ok {
		return device, nil, false
	}
	return nil, waiter.lastErr, waiter.clientGone
}

// wake pokes the dispatcher without blocking - a pending kick already covers this one
func (q *sessionQueue) wake() {
	select {
	case q.kick <- struct{}{}:
	default:
	}
}

func (q *sessionQueue) dispatchLoop() {
	ticker := time.NewTicker(queueSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-q.kick:
		case <-devices.DeviceFreedSignal():
		case <-ticker.C:
		}
		q.dispatch()
	}
}

// dispatch walks the waiters in FIFO order once. Only the queue mutex is held while
// walking - device claiming happens inside findAvailableDevice under each device's
// own mutex, so no device mutex is ever held across the walk
func (q *sessionQueue) dispatch() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.waiters) == 0 {
		return
	}

	now := time.Now()
	remaining := make([]*sessionWaiter, 0, len(q.waiters))
	for _, waiter := range q.waiters {
		select {
		case <-waiter.ctxDone:
			waiter.clientGone = true
			close(waiter.deliver)
			continue
		default:
		}

		device, err := findAvailableDevice(waiter.candidate, waiter.user.AllowedWorkspaceIDs, waiter.user.UserID, waiter.user.Tenant)
		if device != nil {
			waiter.deliver <- device
			continue
		}
		if err != nil {
			waiter.lastErr = err
		}
		// The deadline check runs after the attempt so even a zero-wait waiter that
		// had to queue behind others gets one real matching attempt
		if isFailFastDeviceError(err) || now.After(waiter.deadline) {
			close(waiter.deliver)
			continue
		}
		remaining = append(remaining, waiter)
	}
	q.waiters = remaining
}

// depth reports the number of queued waiters - surfaced on /grid/status and used
// by tests to synchronize on enqueue order
func (q *sessionQueue) depth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.waiters)
}
