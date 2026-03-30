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

	// Provider-only runtime fields (not on models.Device, not synced to hub)
	HardwareModel        string
	IsResetting          bool
	StreamTargetFPS      int
	StreamJpegQuality    int
	StreamScalingFactor  int
	AppiumLastPingTS     int64
	AppiumSessionID      string
	IsAppiumUp           bool
	HasAppiumSession     bool
	CurrentRotation      string
	SupportedStreamTypes []models.StreamType
	InstalledApps        []string
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
func (r *RuntimeState) GetAppiumSessionID() string   { return r.AppiumSessionID }
func (r *RuntimeState) SetAppiumSessionID(id string) { r.AppiumSessionID = id }
func (r *RuntimeState) SetAppiumUp(up bool)          { r.IsAppiumUp = up }
func (r *RuntimeState) SetAppiumLastPingTS(ts int64) { r.AppiumLastPingTS = ts }
func (r *RuntimeState) SetHasAppiumSession(has bool) { r.HasAppiumSession = has }
func (r *RuntimeState) GetIsResetting() bool         { return r.IsResetting }
func (r *RuntimeState) SetIsResetting(v bool)        { r.IsResetting = v }
func (r *RuntimeState) GetIsAppiumUp() bool          { return r.IsAppiumUp }
func (r *RuntimeState) GetHardwareModelValue() string       { return r.HardwareModel }
func (r *RuntimeState) SetHardwareModel(model string)       { r.HardwareModel = model }
func (r *RuntimeState) GetStreamTargetFPS() int             { return r.StreamTargetFPS }
func (r *RuntimeState) SetStreamTargetFPS(fps int)          { r.StreamTargetFPS = fps }
func (r *RuntimeState) GetStreamJpegQuality() int           { return r.StreamJpegQuality }
func (r *RuntimeState) SetStreamJpegQuality(q int)          { r.StreamJpegQuality = q }
func (r *RuntimeState) GetStreamScalingFactor() int         { return r.StreamScalingFactor }
func (r *RuntimeState) SetStreamScalingFactor(f int)        { r.StreamScalingFactor = f }
func (r *RuntimeState) GetCurrentRotationValue() string     { return r.CurrentRotation }
func (r *RuntimeState) SetCurrentRotation(rotation string)  { r.CurrentRotation = rotation }
func (r *RuntimeState) GetSupportedStreamTypes() []models.StreamType { return r.SupportedStreamTypes }
func (r *RuntimeState) SetSupportedStreamTypes(types []models.StreamType) { r.SupportedStreamTypes = types }
func (r *RuntimeState) GetInstalledAppIDs() []string        { return r.InstalledApps }
func (r *RuntimeState) SetInstalledAppIDs(apps []string)    { r.InstalledApps = apps }
func (r *RuntimeState) SetNewContext(ctx context.Context, cancel context.CancelFunc) {
	r.Context = ctx
	r.CtxCancel = cancel
}

// ToHubDevice builds a models.Device populated with DB + hub-visible runtime fields
// for JSON serialization to the hub. Only the 4 hub-synced runtime fields are included.
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

		// Hub-visible runtime fields
		Host:                db.Host,
		LastUpdatedTimestamp: db.LastUpdatedTimestamp,
		Connected:           db.Connected,
		ProviderState:       db.ProviderState,
	}
}

// ResetBase cancels the device context, frees the Appium port, and resets state to "init".
// Platform types should call this from their own Reset() method after doing platform-specific cleanup.
func (r *RuntimeState) ResetBase(reason string) bool {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	if !r.IsResetting && r.DBDevice.ProviderState != "init" {
		logger.ProviderLogger.LogInfo("provider", fmt.Sprintf("Resetting LocalDevice for device `%v` with reason: %s. Cancelling context, setting ProviderState to `init`, Healthy to `false` and updating the DB", r.DBDevice.UDID, reason))

		r.IsResetting = true
		if r.CtxCancel != nil {
			r.CtxCancel()
		}
		r.DBDevice.ProviderState = "init"
		r.IsResetting = false

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
