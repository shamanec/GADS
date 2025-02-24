package common

import "sync"

// ResourceMutexManager manages different mutexes for various resources.
type ResourceMutexManager struct {
	StreamSettings            sync.Mutex
	ResetLocalDeviceFreePorts sync.Mutex
}

// Global instance of ResourceMutexManager
var MutexManager = &ResourceMutexManager{}
