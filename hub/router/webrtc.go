/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"GADS/common/auth"
	"GADS/common/db"
	"GADS/hub/config"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetICEConfig godoc
// @Summary      Get WebRTC ICE configuration
// @Description  Retrieve ICE servers configuration (STUN + optional TURN) for WebRTC connections
// @Tags         Hub - WebRTC
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /ice-config [get]
func GetICEConfig(c *gin.Context) {
	// Always include STUN server (works for ~80-85% of network conditions)
	iceServers := []map[string]interface{}{
		{"urls": "stun:stun.l.google.com:19302"},
	}

	// TURN is optional — add it only when configured and enabled. STUN alone covers
	// most networks; TURN is the fallback for restrictive ones. A missing/failed TURN
	// config just yields a STUN-only response, never an error. Each outcome is logged so
	// an incomplete config (e.g. empty shared secret) is distinguishable from TURN being
	// intentionally disabled.
	turnConfig, err := db.GlobalMongoStore.GetTURNConfig()
	switch {
	case err != nil:
		slog.Warn(fmt.Sprintf("ice-config: could not load TURN config, serving STUN only - %s", err))
	case !turnConfig.Enabled:
		slog.Debug("ice-config: TURN disabled, serving STUN only")
	case turnConfig.Server == "" || turnConfig.SharedSecret == "":
		slog.Warn("ice-config: TURN is enabled but its config is incomplete (missing server or shared secret); serving STUN only")
	default:
		// Generate ephemeral credentials using TURN REST API
		ttl := turnConfig.TTL
		if ttl == 0 {
			ttl = 3600 // Default: 1 hour
		}
		username, password, _ := auth.GenerateTURNCredentials(turnConfig.SharedSecret, ttl, config.GlobalHubConfig.TURNUsernameSuffix)

		urls := []string{
			fmt.Sprintf("turn:%s:%d?transport=udp", turnConfig.Server, turnConfig.Port),
			fmt.Sprintf("turn:%s:%d?transport=tcp", turnConfig.Server, turnConfig.Port),
		}
		// TURN over TLS (turns:) is optional — only advertised when a TLS port is configured.
		if turnConfig.TLSPort > 0 {
			urls = append(urls, fmt.Sprintf("turns:%s:%d?transport=tcp", turnConfig.Server, turnConfig.TLSPort))
		}
		iceServers = append(iceServers, map[string]interface{}{
			"urls":       urls,
			"username":   username,
			"credential": password,
		})
		slog.Info(fmt.Sprintf("ice-config: advertising TURN %s:%d (tls=%t) to client", turnConfig.Server, turnConfig.Port, turnConfig.TLSPort > 0))
	}

	c.JSON(http.StatusOK, gin.H{"iceServers": iceServers})
}
