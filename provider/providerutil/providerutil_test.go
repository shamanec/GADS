/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package providerutil

import "testing"

func TestReleasePorts(t *testing.T) {
	UsedPorts = map[string]bool{"50001": true, "50002": true, "50003": true}
	t.Cleanup(func() { UsedPorts = make(map[string]bool) })

	streamPort, imePort, emptyPort := "50001", "50002", ""
	inUse := ReleasePorts(&streamPort, &imePort, &emptyPort)

	if inUse != 1 {
		t.Errorf("ReleasePorts() = %d ports still allocated, expected 1", inUse)
	}
	if UsedPorts["50001"] || UsedPorts["50002"] {
		t.Errorf("released ports are still allocated: %v", UsedPorts)
	}
	if !UsedPorts["50003"] {
		t.Errorf("port not passed to ReleasePorts was freed: %v", UsedPorts)
	}
	// Fields are cleared so a later release with stale values cannot free a port
	// that was handed to another device in the meantime
	if streamPort != "" || imePort != "" {
		t.Errorf("port fields not cleared: stream=%q ime=%q", streamPort, imePort)
	}

	UsedPorts["50001"] = true // re-allocated to another device
	ReleasePorts(&streamPort)
	if !UsedPorts["50001"] {
		t.Errorf("repeated release freed a port re-allocated to another device")
	}
}
