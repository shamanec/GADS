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
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetICEConfig godoc
// @Summary      Get WebRTC ICE configuration
// @Description  Retrieve ICE servers configuration (STUN + optional TURN) for WebRTC connections
// @Tags         WebRTC
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /ice-config [get]
func GetICEConfig(c *gin.Context) {
	// Always include STUN server (works for ~80-85% of network conditions)
	iceServers := []map[string]interface{}{
		{"urls": "stun:stun.l.google.com:19302"},
	}

	// Try to add TURN server if configured and enabled
	turnConfig, err := db.GlobalMongoStore.GetTURNConfig()
	if err == nil && turnConfig.Enabled && turnConfig.Server != "" && turnConfig.SharedSecret != "" {
		// Generate ephemeral credentials using TURN REST API
		ttl := turnConfig.TTL
		if ttl == 0 {
			ttl = 3600 // Default: 1 hour
		}
		username, password, _ := auth.GenerateTURNCredentials(turnConfig.SharedSecret, ttl, config.GlobalHubConfig.TURNUsernameSuffix)

		// Add TURN server as fallback for restrictive networks
		turnServer := map[string]interface{}{
			"urls": []string{
				fmt.Sprintf("turn:%s:%d?transport=udp", turnConfig.Server, turnConfig.Port),
				fmt.Sprintf("turn:%s:%d?transport=tcp", turnConfig.Server, turnConfig.Port),
			},
			"username":   username,
			"credential": password,
		}
		iceServers = append(iceServers, turnServer)
	}

	c.JSON(http.StatusOK, gin.H{"iceServers": iceServers})
}
