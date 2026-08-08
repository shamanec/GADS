/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package devices

import "testing"

func TestParseAvdNameOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "name followed by OK",
			output:   "Pixel_8_API_35\nOK\n",
			expected: "Pixel_8_API_35",
		},
		{
			name:     "windows line endings",
			output:   "Pixel_8_API_35\r\nOK\r\n",
			expected: "Pixel_8_API_35",
		},
		{
			name:     "leading blank line",
			output:   "\nPixel_8_API_35\nOK\n",
			expected: "Pixel_8_API_35",
		},
		{
			name:     "only OK",
			output:   "OK\n",
			expected: "",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseAvdNameOutput(test.output); got != test.expected {
				t.Errorf("parseAvdNameOutput(%q) = %q, expected %q", test.output, got, test.expected)
			}
		})
	}
}

func TestEmulatorSerialRegex(t *testing.T) {
	matching := []string{"emulator-5554", "emulator-5556", "emulator-12345"}
	nonMatching := []string{"R58M123ABC", "192.168.1.10:5555", "emulator-", "emulator-5554x", "myemulator-5554"}

	for _, serial := range matching {
		if !emulatorSerialRegex.MatchString(serial) {
			t.Errorf("expected `%s` to match the emulator serial pattern", serial)
		}
	}
	for _, serial := range nonMatching {
		if emulatorSerialRegex.MatchString(serial) {
			t.Errorf("expected `%s` to NOT match the emulator serial pattern", serial)
		}
	}
}
