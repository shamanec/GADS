package utils

import (
	"GADS/common/auth"
	"GADS/common/db"
	"GADS/provider/config"
	"GADS/provider/logger"
	"fmt"

	"github.com/pion/webrtc/v3"
)

func GenerateWebRTCConfig() webrtc.Configuration {
	iceServers := []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
	}

	turnConfig, err := db.GlobalMongoStore.GetTURNConfig()
	switch {
	case err != nil:
		logger.ProviderLogger.LogWarn("webrtc_config", fmt.Sprintf("TURN config unavailable, WebRTC will use STUN only - %v", err))
	case !turnConfig.Enabled:
		logger.ProviderLogger.LogWarn("webrtc_config", "TURN disabled in config, WebRTC will use STUN only")
	case turnConfig.Server == "" || turnConfig.SharedSecret == "":
		logger.ProviderLogger.LogWarn("webrtc_config", "TURN enabled but server/shared secret missing, WebRTC will use STUN only")
	default:
		ttl := turnConfig.TTL
		if ttl == 0 {
			ttl = 3600
		}
		username, password, _ := auth.GenerateTURNCredentials(turnConfig.SharedSecret, ttl, config.ProviderConfig.TURNUsernameSuffix)
		urls := []string{
			fmt.Sprintf("turn:%s:%d?transport=udp", turnConfig.Server, turnConfig.Port),
			fmt.Sprintf("turn:%s:%d?transport=tcp", turnConfig.Server, turnConfig.Port),
		}
		// TURN over TLS (turns:) is optional — only advertised when a TLS port is configured.
		if turnConfig.TLSPort > 0 {
			urls = append(urls, fmt.Sprintf("turns:%s:%d?transport=tcp", turnConfig.Server, turnConfig.TLSPort))
		}
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       urls,
			Username:   username,
			Credential: password,
		})
		logger.ProviderLogger.LogInfo("webrtc_config", fmt.Sprintf("Applying TURN relay: server=%s:%d tls_port=%d suffix=%s", turnConfig.Server, turnConfig.Port, turnConfig.TLSPort, config.ProviderConfig.TURNUsernameSuffix))
	}

	return webrtc.Configuration{
		ICEServers: iceServers,
	}
}
