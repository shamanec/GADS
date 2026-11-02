/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOIDCConfigValidate(t *testing.T) {
	complete := OIDCConfig{
		Enabled:      true,
		IssuerURL:    "https://sso.example.com/realms/example",
		ClientID:     "gads",
		ClientSecret: "secret",
		RedirectURI:  "https://gads.example.com/auth/sso/callback",
	}

	testCases := []struct {
		name   string
		modify func(*OIDCConfig)
		valid  bool
	}{
		{name: "complete", modify: func(*OIDCConfig) {}, valid: true},
		{name: "disabled and empty", modify: func(c *OIDCConfig) { *c = OIDCConfig{} }, valid: true},
		{name: "no issuer", modify: func(c *OIDCConfig) { c.IssuerURL = "" }},
		{name: "no client ID", modify: func(c *OIDCConfig) { c.ClientID = "" }},
		{name: "no redirect URI", modify: func(c *OIDCConfig) { c.RedirectURI = "" }},
		{name: "no client secret", modify: func(c *OIDCConfig) { c.ClientSecret = "" }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := complete
			tc.modify(&config)
			if tc.valid {
				assert.NoError(t, config.Validate())
			} else {
				assert.Error(t, config.Validate())
			}
		})
	}
}
