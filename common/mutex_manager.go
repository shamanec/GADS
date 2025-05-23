/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package common

import "sync"

// ResourceMutexManager manages different mutexes for various resources.
type ResourceMutexManager struct {
	StreamSettings   sync.Mutex
	LocalDevicePorts sync.Mutex
}

// Global instance of ResourceMutexManager
var MutexManager = &ResourceMutexManager{}
