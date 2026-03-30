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
	"fmt"
	"sync"

	"github.com/Masterminds/semver"

	"GADS/common"
	"GADS/common/models"
	"GADS/provider/logger"
	"GADS/provider/providerutil"
)

// RuntimeState holds runtime fields shared across all platform device types.
// Each concrete device type (AndroidDevice, IOSDevice, etc.) embeds this struct
// to inherit common state and accessor methods.
type RuntimeState struct {
	// DB model - pointer to the *models.Device entry (shared with DBDeviceMap for backward compat)
	DBDevice *models.Device

	// Infrastructure
	Context          context.Context
	CtxCancel        context.CancelFunc
	Mutex            sync.Mutex
	SetupMutex       sync.Mutex
	Logger           models.CustomLogger
	SemVer           *semver.Version
	InitialSetupDone bool
	AppiumReadyChan  chan bool
}

// Common accessor implementations inherited by all platform types via embedding.

func (r *RuntimeState) GetUDID() string              { return r.DBDevice.UDID }
func (r *RuntimeState) GetOS() string                 { return r.DBDevice.OS }
func (r *RuntimeState) GetDBDevice() *models.Device   { return r.DBDevice }
func (r *RuntimeState) GetProviderState() string      { return r.DBDevice.ProviderState }
func (r *RuntimeState) SetProviderState(state string) { r.DBDevice.ProviderState = state }
func (r *RuntimeState) IsConnected() bool             { return r.DBDevice.Connected }
func (r *RuntimeState) SetConnected(connected bool)   { r.DBDevice.Connected = connected }

// ToHubDevice builds a models.Device populated with runtime fields for JSON serialization to the hub.
// Fields are assigned individually to avoid copying the sync.Mutex embedded in models.Device.
func (r *RuntimeState) ToHubDevice() models.Device {
	db := r.DBDevice
	return models.Device{
		// DB-persisted fields
		UDID:         db.UDID,
		OS:           db.OS,
		Name:         db.Name,
		OSVersion:    db.OSVersion,
		IPAddress:    db.IPAddress,
		Provider:     db.Provider,
		Usage:        db.Usage,
		ScreenWidth:  db.ScreenWidth,
		ScreenHeight: db.ScreenHeight,
		DeviceType:   db.DeviceType,
		WorkspaceID:  db.WorkspaceID,
		StreamType:   db.StreamType,

		// Runtime fields the hub needs (json-tagged, bson:"-")
		Host:                 db.Host,
		HardwareModel:        db.HardwareModel,
		LastUpdatedTimestamp:  db.LastUpdatedTimestamp,
		Connected:            db.Connected,
		IsResetting:          db.IsResetting,
		ProviderState:        db.ProviderState,
		StreamTargetFPS:      db.StreamTargetFPS,
		StreamJpegQuality:    db.StreamJpegQuality,
		StreamScalingFactor:  db.StreamScalingFactor,
		SupportedStreamTypes: db.SupportedStreamTypes,
		AppiumLastPingTS:     db.AppiumLastPingTS,
		AppiumSessionID:      db.AppiumSessionID,
		IsAppiumUp:           db.IsAppiumUp,
		HasAppiumSession:     db.HasAppiumSession,
		CurrentRotation:      db.CurrentRotation,
		InstalledApps:        db.InstalledApps,
	}
}

// Reset cancels the device context, closes tunnels, frees ports, and resets state to "init".
func (r *RuntimeState) Reset(reason string) {
	r.DBDevice.Mutex.Lock()
	defer r.DBDevice.Mutex.Unlock()
	if !r.DBDevice.IsResetting && r.DBDevice.ProviderState != "init" {
		logger.ProviderLogger.LogInfo("provider", fmt.Sprintf("Resetting LocalDevice for device `%v` with reason: %s. Cancelling context, setting ProviderState to `init`, Healthy to `false` and updating the DB", r.DBDevice.UDID, reason))

		r.DBDevice.IsResetting = true
		r.DBDevice.CtxCancel()
		r.DBDevice.ProviderState = "init"
		r.DBDevice.IsResetting = false
		if r.DBDevice.GoIOSTunnel.Address != "" {
			r.DBDevice.GoIOSTunnel.Close()
		}

		// Free any used ports from the map where we keep them
		common.MutexManager.LocalDevicePorts.Lock()
		delete(providerutil.UsedPorts, r.DBDevice.WDAPort)
		delete(providerutil.UsedPorts, r.DBDevice.StreamPort)
		delete(providerutil.UsedPorts, r.DBDevice.AppiumPort)
		delete(providerutil.UsedPorts, r.DBDevice.WDAStreamPort)
		common.MutexManager.LocalDevicePorts.Unlock()
	}
}
