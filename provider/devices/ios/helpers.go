/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package ios

import (
	"context"
	"fmt"
	"strconv"

	"GADS/common/constants"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/forward"
	"github.com/danielpaulus/go-ios/ios/tunnel"
)

// goIosForward starts a go-ios port-forward from devicePort to hostPort. It
// blocks until ctx is cancelled, then closes the forwarder. Must be run in a
// goroutine from Setup.
func (d *IOSDevice) goIosForward(ctx context.Context, hostPort, devicePort string) {
	hostPortInt, _ := strconv.Atoi(hostPort)
	devicePortInt, _ := strconv.Atoi(devicePort)

	cl, err := forward.Forward(d.goIOSEntry, uint16(hostPortInt), uint16(devicePortInt))
	if err != nil {
		d.log.LogError("ios_setup",
			fmt.Sprintf("goIosForward %s: device:%s→host:%s: %v", d.info.UDID, devicePort, hostPort, err))
		d.Reset("Port forwarding failed")
		return
	}
	<-ctx.Done()
	cl.Close()
}

// updateScreenSize looks up the screen dimensions for productType in the
// IOSDeviceInfoMap constant table and writes them to d.info. The updated
// dimensions are persisted to the DB via the store.
func (d *IOSDevice) updateScreenSize(productType string) error {
	dims, ok := constants.IOSDeviceInfoMap[productType]
	if !ok {
		return fmt.Errorf("updateScreenSize %s: product type %q not found in IOSDeviceInfoMap", d.info.UDID, productType)
	}
	d.info.ScreenHeight = dims.Height
	d.info.ScreenWidth = dims.Width

	if err := d.store.AddOrUpdateDevice(d.info); err != nil {
		return fmt.Errorf("updateScreenSize %s: persist: %w", d.info.UDID, err)
	}
	return nil
}

// createTunnel establishes a userspace tunnel for devices running iOS 17.4+.
// The tunnel replaces standard USB port forwarding; all go-ios service calls
// after this point must use the tunnel-based DeviceEntry.
func (d *IOSDevice) createTunnel() (tunnel.Tunnel, error) {
	tun, err := tunnel.ConnectUserSpaceTunnelLockdown(
		d.goIOSEntry,
		d.goIOSEntry.UserspaceTUNPort,
	)
	if err != nil {
		return tunnel.Tunnel{}, fmt.Errorf("createTunnel %s: %w", d.info.UDID, err)
	}
	tun.UserspaceTUN = true
	tun.UserspaceTUNPort = d.goIOSEntry.UserspaceTUNPort
	return tun, nil
}

// goIosDeviceWithRsdProvider refreshes d.goIOSEntry with an RSD-provider-aware
// entry. This is required after a userspace tunnel is established so that all
// subsequent go-ios library calls route through the tunnel correctly.
func (d *IOSDevice) goIosDeviceWithRsdProvider() error {
	rsdSvc, err := ios.NewWithAddrPortDevice(
		d.goIOSTunnel.Address,
		d.goIOSTunnel.RsdPort,
		d.goIOSEntry,
	)
	if err != nil {
		return fmt.Errorf("goIosDeviceWithRsdProvider %s: new addr port device: %w", d.info.UDID, err)
	}
	defer rsdSvc.Close()

	rsdProvider, err := rsdSvc.Handshake()
	if err != nil {
		return fmt.Errorf("goIosDeviceWithRsdProvider %s: handshake: %w", d.info.UDID, err)
	}

	newEntry, err := ios.GetDeviceWithAddress(d.info.UDID, d.goIOSTunnel.Address, rsdProvider)
	if err != nil {
		return fmt.Errorf("goIosDeviceWithRsdProvider %s: get device: %w", d.info.UDID, err)
	}
	newEntry.UserspaceTUN = d.goIOSEntry.UserspaceTUN
	newEntry.UserspaceTUNPort = d.goIOSEntry.UserspaceTUNPort
	d.goIOSEntry = newEntry
	return nil
}
