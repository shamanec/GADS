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
	"testing"

	"GADS/common/models"
)

func TestApplySetupTimeConfig(t *testing.T) {
	device := models.DBDevice{
		UDID:               "ANDROID123",
		StreamType:         models.AndroidWebRTCGadsH264StreamTypeId,
		AudioStreamEnabled: true,
		AudioInputType:     "microphone",
	}

	tests := []struct {
		name            string
		mutate          func(*models.DBDevice)
		wantReprovision bool
	}{
		// An unchanged configuration must not reprovision - the sync loop runs every second
		{name: "no changes", mutate: func(d *models.DBDevice) {}, wantReprovision: false},
		{name: "audio input type", mutate: func(d *models.DBDevice) { d.AudioInputType = "internal" }, wantReprovision: true},
		{name: "audio stream disabled", mutate: func(d *models.DBDevice) { d.AudioStreamEnabled = false }, wantReprovision: true},
		{name: "stream type", mutate: func(d *models.DBDevice) { d.StreamType = models.MJPEGStreamTypeId }, wantReprovision: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inMemory := device
			fromDB := device
			test.mutate(&fromDB)

			if got := applySetupTimeConfig(&inMemory, &fromDB); got != test.wantReprovision {
				t.Fatalf("expected reprovision %v, got %v", test.wantReprovision, got)
			}
			if inMemory != fromDB {
				t.Fatalf("expected the DB configuration to be copied, got %+v", inMemory)
			}
		})
	}
}
