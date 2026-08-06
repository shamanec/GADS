/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package devices

// deviceFreedSignal is a coalescing wake-up for the grid session queue - any event
// that may have made a device available for automation pokes it. Capacity 1 is
// enough: a pending wake-up already covers every event that raced in before the
// dispatcher consumes it
var deviceFreedSignal = make(chan struct{}, 1)

// NotifyDeviceFreed signals that a device may have become available for automation.
// Non-blocking, so it is safe to call while holding a device mutex
func NotifyDeviceFreed() {
	select {
	case deviceFreedSignal <- struct{}{}:
	default:
	}
}

// DeviceFreedSignal returns the channel the grid session queue dispatcher waits on
func DeviceFreedSignal() <-chan struct{} {
	return deviceFreedSignal
}
