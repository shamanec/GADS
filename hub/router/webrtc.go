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
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetICEConfig godoc
// @Summary      Get WebRTC ICE configuration
// @Description  Retrieve ICE servers configuration (STUN + TURN) for WebRTC connections
// @Tags         WebRTC
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      412  {object}  models.ErrorResponse
// @Failure      500  {object}  models.ErrorResponse
// @Router       /ice-config [get]
func GetICEConfig(c *gin.Context) {
	// Always include STUN server as fallback
	iceServers := []map[string]interface{}{
		// {"urls": "stun:stun.l.google.com:19302"},
	}

	// Fetch TURN configuration from MongoDB
	turnConfig, err := db.GlobalMongoStore.GetTURNConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve TURN configuration",
		})
		return
	}

	// Check if TURN is configured and enabled
	if !turnConfig.Enabled {
		c.JSON(http.StatusPreconditionFailed, gin.H{
			"error":   "TURN server not configured",
			"message": "WebRTC requires TURN server to work in restricted networks. Please configure TURN server in Admin → Global Settings.",
			"action":  "configure_turn",
		})
		return
	}

	// Validate TURN configuration is complete
	if turnConfig.Server == "" {
		c.JSON(http.StatusPreconditionFailed, gin.H{
			"error":   "TURN server configuration incomplete",
			"message": "TURN server address is required. Please configure in Admin → Global Settings.",
			"action":  "configure_turn",
		})
		return
	}

	if turnConfig.SharedSecret == "" {
		c.JSON(http.StatusPreconditionFailed, gin.H{
			"error":   "TURN server configuration incomplete",
			"message": "TURN shared secret is required for security. Please configure in Admin → Global Settings.",
			"action":  "configure_turn",
		})
		return
	}

	// Generate ephemeral credentials using TURN REST API
	ttl := turnConfig.TTL
	if ttl == 0 {
		ttl = 3600 // Default: 1 hour
	}
	username, password, _ := auth.GenerateTURNCredentials(turnConfig.SharedSecret, ttl)

	// Add TURN server to ICE servers list
	turnServer := map[string]interface{}{
		"urls": []string{
			fmt.Sprintf("turn:%s:%d?transport=udp", turnConfig.Server, turnConfig.Port),
			fmt.Sprintf("turn:%s:%d?transport=tcp", turnConfig.Server, turnConfig.Port),
		},
		"username":   username,
		"credential": password,
	}
	iceServers = append(iceServers, turnServer)

	c.JSON(http.StatusOK, gin.H{"iceServers": iceServers})
}
