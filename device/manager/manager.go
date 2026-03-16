/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

// Package manager implements the DeviceManager — the provider-side orchestrator
// that detects connected devices, provisions them via platform-specific Setup
// calls, and keeps the hub informed of their state.
//
// This package replaces the global DBDeviceMap, Listener(), updateDevices(), and
// updateProviderHub() functions in provider/devices/common.go. It is designed to
// be additive (Phase 6); the switchover happens in Phase 7.
package manager

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"GADS/common/constants"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/device"
	"GADS/device/tizen"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// LoggerFactory is a function that creates a per-device structured logger.
// In production it is provider/logger.CreateCustomLogger; in tests it can be
// replaced with a no-op implementation.
type LoggerFactory func(logFilePath, collection string) (models.CustomLogger, error)

// Instance is the provider-level singleton DeviceManager, set during startup.
// Route handlers access it directly via manager.Instance.GetDevice(udid).
var Instance *DeviceManager

// DeviceManager manages the lifecycle of all devices known to this provider.
// It is the single source of truth for device state on the provider side,
// replacing the global DBDeviceMap and its associated mutexes.
type DeviceManager struct {
	// devices holds all ManagedDevice instances keyed by UDID.
	devices map[string]device.ManagedDevice
	// mu protects devices for concurrent read/write access.
	mu sync.RWMutex

	// cfg is the provider configuration (platform flags, folder paths, etc.).
	cfg *models.Provider

	// mongoStore is the raw MongoDB store used for DB-level operations that
	// aren't part of the device.DeviceStore interface (collection management,
	// workspace lookup, GetProviderDevices).
	mongoStore *db.MongoStore

	// factory creates platform-specific ManagedDevice instances.
	factory DeviceFactory

	// cmd is used by the detection helpers (ADB, SDB, ares-*).
	cmd device.CommandRunner

	// store satisfies the device.DeviceStore interface passed to each platform
	// device during construction.
	store device.DeviceStore

	// log is the provider-level logger (not per-device).
	log models.CustomLogger

	// logFn creates per-device loggers.
	logFn LoggerFactory

	// tizenRetryTracker tracks auto-connection retry state for Tizen devices.
	tizenRetryTracker *tizen.RetryTracker
}

// Start constructs the singleton DeviceManager, loads devices from the
// database, and launches the background goroutines that keep device state
// up to date and push updates to the hub. All parameters must be non-nil.
// It returns immediately; all background work runs in goroutines tied to ctx.
//
//   - cfg:        provider configuration
//   - mongoStore: raw MongoDB store for collection management and device queries
//   - log:        provider-level logger
//   - logFn:      creates per-device loggers
func Start(
	ctx context.Context,
	cfg *models.Provider,
	log models.CustomLogger,
	logFn LoggerFactory,
) {
	cmd := device.NewExecCommandRunner()
	store := device.NewMongoDeviceStore(db.GlobalMongoStore)
	httpClient := device.NewDefaultHTTPClient(30 * time.Second)
	factory := NewDefaultDeviceFactory(cmd, device.NewNetPortAllocator(), store, httpClient, cfg)

	Instance = &DeviceManager{
		devices:           make(map[string]device.ManagedDevice),
		cfg:               cfg,
		mongoStore:        db.GlobalMongoStore,
		factory:           factory,
		cmd:               cmd,
		store:             store,
		log:               log,
		logFn:             logFn,
		tizenRetryTracker: tizen.NewRetryTracker(),
	}
	Instance.loadFromDB()
	go Instance.deviceUpdateLoop(ctx)
	go Instance.hubSyncLoop(ctx)
}

// GetDevice returns the ManagedDevice for the given UDID and whether it exists.
func (m *DeviceManager) GetDevice(udid string) (device.ManagedDevice, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.devices[udid]
	return d, ok
}

// AllDevices returns a snapshot of all managed devices.
func (m *DeviceManager) AllDevices() []device.ManagedDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]device.ManagedDevice, 0, len(m.devices))
	for _, d := range m.devices {
		out = append(out, d)
	}
	return out
}

// AllDeviceInfos returns a snapshot of DeviceInfo pointers for all managed devices.
// Used by hub_sync to build the JSON payload.
func (m *DeviceManager) AllDeviceInfos() []*device.DeviceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*device.DeviceInfo, 0, len(m.devices))
	for _, d := range m.devices {
		out = append(out, d.Info())
	}
	return out
}

// loadFromDB reads the provider's device list from MongoDB and initialises
// each device (log directory, Appium log collection, logger, factory creation).
// Devices that fail to initialise are logged and skipped.
func (m *DeviceManager) loadFromDB() {
	dbDevices, err := device.GetProviderDevices(m.cfg.Nickname)
	if err != nil {
		m.log.LogError("manager_init", fmt.Sprintf("Failed to load provider devices from DB: %v", err))
		return
	}

	for i := range dbDevices {
		info := &dbDevices[i]

		// Assign to the default workspace if not already set.
		if info.WorkspaceID == "" {
			ws, err := m.mongoStore.GetDefaultWorkspace()
			if err != nil {
				m.log.LogWarn("manager_init",
					fmt.Sprintf("Failed to get default workspace for device %s: %v", info.UDID, err))
			} else {
				info.WorkspaceID = ws.ID
				if uErr := m.store.AddOrUpdateDevice(info); uErr != nil {
					m.log.LogWarn("manager_init",
						fmt.Sprintf("Failed to persist workspace for device %s: %v", info.UDID, uErr))
				}
			}
		}

		if err := m.initDevice(info); err != nil {
			m.log.LogError("manager_init",
				fmt.Sprintf("Failed to initialise device %s: %v", info.UDID, err))
		}
	}
}

// initDevice creates the per-device log directory, Appium log collection,
// logger, and platform-specific ManagedDevice, then adds it to the map.
func (m *DeviceManager) initDevice(info *device.DeviceInfo) error {
	info.Host = fmt.Sprintf("%s:%v", m.cfg.HostAddress, m.cfg.Port)

	// Create Appium log capped collection when Appium servers are enabled.
	if m.cfg.SetupAppiumServers {
		exists, err := m.mongoStore.CheckCollectionExistsWithDB("appium_logs_new", info.UDID)
		if err != nil {
			m.log.LogWarn("manager_init",
				fmt.Sprintf("Could not check Appium log collection for %s: %v", info.UDID, err))
		}
		if !exists {
			if err := m.mongoStore.CreateCappedCollectionWithDB("appium_logs_new", info.UDID, 30000, 30); err != nil {
				return fmt.Errorf("initDevice %s: create Appium log collection: %w", info.UDID, err)
			}
		}
		indexModel := mongo.IndexModel{
			Keys: bson.D{
				{Key: "timestamp", Value: constants.SortAscending},
				{Key: "session_id", Value: constants.SortAscending},
				{Key: "sequenceNumber", Value: constants.SortAscending},
			},
		}
		m.mongoStore.AddCollectionIndexWithDB("appium_logs_new", info.UDID, indexModel)
	}

	// Ensure per-device log directory exists.
	logDir := filepath.Join(m.cfg.ProviderFolder, "device_"+info.UDID)
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return fmt.Errorf("initDevice %s: create log dir: %w", info.UDID, err)
	}

	// Create per-device logger.
	logPath := filepath.Join(logDir, "device.log")
	devLog, err := m.logFn(logPath, info.UDID)
	if err != nil {
		return fmt.Errorf("initDevice %s: create logger: %w", info.UDID, err)
	}

	// Create the platform-specific ManagedDevice.
	dev, err := m.factory.Create(info, devLog)
	if err != nil {
		return fmt.Errorf("initDevice %s: factory: %w", info.UDID, err)
	}

	m.mu.Lock()
	m.devices[info.UDID] = dev
	m.mu.Unlock()
	return nil
}

// deviceUpdateLoop runs on a 1-second ticker. It detects physically connected
// devices, starts Setup for newly connected devices, resets disconnected ones,
// and runs Tizen auto-connection on a 30-second cadence.
func (m *DeviceManager) deviceUpdateLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var tizenChan <-chan time.Time
	if m.cfg.ProvideTizen {
		tizenTicker := time.NewTicker(30 * time.Second)
		defer tizenTicker.Stop()
		tizenChan = tizenTicker.C
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			connected := detectConnectedDevices(m.cfg, m.cmd)

			m.mu.RLock()
			snapshot := make(map[string]device.ManagedDevice, len(m.devices))
			for udid, d := range m.devices {
				snapshot[udid] = d
			}
			m.mu.RUnlock()

			for udid, dev := range snapshot {
				info := dev.Info()
				if info.Usage == "disabled" {
					continue
				}

				if slices.Contains(connected, udid) {
					info.Connected = true
					state := dev.ProviderState()
					if state != "preparing" && state != "live" {
						if err := device.ValidateDeviceUsageForOS(info.OS, info.Usage); err != nil {
							m.log.LogWarn("manager_update",
								fmt.Sprintf("Device %s has invalid config: %v — skipping setup", udid, err))
							continue
						}
						go func(d device.ManagedDevice) {
							d.Setup(ctx) //nolint:errcheck // failures handled internally via Reset
						}(dev)
					}
				} else {
					dev.Reset("device disconnected")
					info.Connected = false
				}
			}

		case <-tizenChan:
			m.handleTizenAutoConnect()
		}
	}
}

// handleTizenAutoConnect iterates over all Tizen devices and retries sdb
// connections for those that are not currently connected.
func (m *DeviceManager) handleTizenAutoConnect() {
	connected := tizen.GetConnectedDevices(m.cmd)

	m.mu.RLock()
	snapshot := make(map[string]device.ManagedDevice, len(m.devices))
	for udid, d := range m.devices {
		snapshot[udid] = d
	}
	m.mu.RUnlock()

	for udid, dev := range snapshot {
		if dev.Info().OS != "tizen" || dev.Info().Usage == "disabled" {
			continue
		}

		if !slices.Contains(connected, udid) {
			if m.tizenRetryTracker.ShouldAttemptConnection(udid) {
				tizen.AttemptConnection(m.tizenRetryTracker, m.cmd, udid, m.log)
			}
		}
	}
}

// hubSyncLoop runs on a 1-second ticker. It:
//  1. Syncs device config from the DB (handles added/removed/changed devices).
//  2. Applies any config changes (e.g. stream type change triggers a reset).
//  3. Builds the ProviderPayload and POSTs to the hub.
//
// If the hub fails 30 consecutive times, the provider is killed — matching the
// behaviour of the legacy updateProviderHub.
func (m *DeviceManager) hubSyncLoop(ctx context.Context) {
	client := &http.Client{Timeout: 5 * time.Second}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	failureCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.syncFromDB()

			infos := m.AllDeviceInfos()
			if err := syncToHub(client, m.cfg.HubAddress, *m.cfg, infos); err != nil {
				failureCount++
				m.log.LogError("manager_hub_sync",
					fmt.Sprintf("Failed to update hub (attempt %d/30): %v", failureCount, err))
				if failureCount >= 30 {
					log.Fatalf("Failed to reach hub 30 consecutive times — killing provider")
				}
			} else {
				failureCount = 0
			}
		}
	}
}

// syncFromDB reloads the device list from MongoDB and reconciles it with the
// in-memory map:
//   - Devices removed from DB are reset and removed from the map.
//   - Devices added to DB are initialised and added to the map.
//   - Config changes (Usage, StreamType, etc.) are applied; a stream-type change
//     triggers a reset so the device is re-provisioned with the new type.
func (m *DeviceManager) syncFromDB() {
	dbDevices, err := device.GetProviderDevices(m.cfg.Nickname)
	if err != nil {
		m.log.LogError("manager_db_sync",
			fmt.Sprintf("Failed to reload provider devices from DB: %v", err))
		return
	}

	// Build a UDID → index map to avoid copying mutex-containing Device values.
	dbIndex := make(map[string]int, len(dbDevices))
	for i := range dbDevices {
		dbIndex[dbDevices[i].UDID] = i
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove devices no longer in DB.
	for udid, dev := range m.devices {
		if _, ok := dbIndex[udid]; !ok {
			m.log.LogInfo("manager_db_sync",
				fmt.Sprintf("Device %s removed from DB — resetting", udid))
			dev.Reset("device removed from DB")
			delete(m.devices, udid)
		}
	}

	// Sync config changes and add new devices.
	for udid, idx := range dbIndex {
		dbDev := &dbDevices[idx] // pointer avoids copying the embedded mutex
		dev, exists := m.devices[udid]
		if !exists {
			// New device — initialise without holding the lock (initDevice also locks).
			m.mu.Unlock()
			if err := m.initDevice(dbDev); err != nil {
				m.log.LogError("manager_db_sync",
					fmt.Sprintf("Failed to initialise new device %s: %v", udid, err))
			}
			m.mu.Lock()
			continue
		}

		// Apply config changes.
		info := dev.Info()
		info.ScreenWidth = dbDev.ScreenWidth
		info.ScreenHeight = dbDev.ScreenHeight
		info.Name = dbDev.Name
		info.OSVersion = dbDev.OSVersion
		info.Usage = dbDev.Usage
		info.WorkspaceID = dbDev.WorkspaceID

		if !m.cfg.SetupAppiumServers && info.Usage != "disabled" {
			info.Usage = "control"
		}

		if dbDev.StreamType != info.StreamType {
			info.StreamType = dbDev.StreamType
			dev.Reset("stream type changed — re-provisioning")
		}
	}
}

