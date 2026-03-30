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
	// DB model - pointer to the *models.Device entry for hub-visible fields
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
	AppiumPort       string // port assigned to the device for the Appium server
}

// Common accessor implementations inherited by all platform types via embedding.

func (r *RuntimeState) GetUDID() string                { return r.DBDevice.UDID }
func (r *RuntimeState) GetOS() string                  { return r.DBDevice.OS }
func (r *RuntimeState) GetDBDevice() *models.Device    { return r.DBDevice }
func (r *RuntimeState) GetProviderState() string       { return r.DBDevice.ProviderState }
func (r *RuntimeState) SetProviderState(state string)  { r.DBDevice.ProviderState = state }
func (r *RuntimeState) IsConnected() bool              { return r.DBDevice.Connected }
func (r *RuntimeState) SetConnected(connected bool)    { r.DBDevice.Connected = connected }
func (r *RuntimeState) GetLogger() models.CustomLogger { return r.Logger }
func (r *RuntimeState) GetContext() context.Context     { return r.Context }
func (r *RuntimeState) GetAppiumPort() string      { return r.AppiumPort }
func (r *RuntimeState) SetAppiumPort(port string)   { r.AppiumPort = port }
func (r *RuntimeState) GetStreamPort() string        { return "" } // overridden by Android/iOS
func (r *RuntimeState) GetAppiumSessionID() string   { return r.DBDevice.AppiumSessionID }
func (r *RuntimeState) SetAppiumSessionID(id string) { r.DBDevice.AppiumSessionID = id }
func (r *RuntimeState) SetAppiumUp(up bool)          { r.DBDevice.IsAppiumUp = up }
func (r *RuntimeState) SetAppiumLastPingTS(ts int64) { r.DBDevice.AppiumLastPingTS = ts }
func (r *RuntimeState) SetHasAppiumSession(has bool) { r.DBDevice.HasAppiumSession = has }
func (r *RuntimeState) SetNewContext(ctx context.Context, cancel context.CancelFunc) {
	r.Context = ctx
	r.CtxCancel = cancel
}

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

// ResetBase cancels the device context, frees the Appium port, and resets state to "init".
// Platform types should call this from their own Reset() method after doing platform-specific cleanup.
func (r *RuntimeState) ResetBase(reason string) bool {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	if !r.DBDevice.IsResetting && r.DBDevice.ProviderState != "init" {
		logger.ProviderLogger.LogInfo("provider", fmt.Sprintf("Resetting LocalDevice for device `%v` with reason: %s. Cancelling context, setting ProviderState to `init`, Healthy to `false` and updating the DB", r.DBDevice.UDID, reason))

		r.DBDevice.IsResetting = true
		if r.CtxCancel != nil {
			r.CtxCancel()
		}
		r.DBDevice.ProviderState = "init"
		r.DBDevice.IsResetting = false

		// Free AppiumPort (common to all platforms)
		common.MutexManager.LocalDevicePorts.Lock()
		delete(providerutil.UsedPorts, r.AppiumPort)
		common.MutexManager.LocalDevicePorts.Unlock()
		return true
	}
	return false
}

// Reset is the default reset implementation. Platform types with ports or tunnels should override this.
func (r *RuntimeState) Reset(reason string) {
	r.ResetBase(reason)
}

// resetWithError logs an error, resets the device, and returns the error — used by Setup() step methods.
func (r *RuntimeState) resetWithError(step string, err error) error {
	logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Failed to %s for device `%s` - %v", step, r.GetUDID(), err))
	r.Reset(fmt.Sprintf("Failed to %s", step))
	return fmt.Errorf("%s: %w", step, err)
}
