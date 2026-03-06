package utils

import (
	"GADS/common/auth"
	"GADS/common/db"
	"GADS/provider/config"
	"fmt"

	"github.com/pion/webrtc/v3"
)

func GenerateWebRTCConfig() webrtc.Configuration {
	iceServers := []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
	}

	turnConfig, err := db.GlobalMongoStore.GetTURNConfig()
	if err == nil && turnConfig.Enabled && turnConfig.Server != "" && turnConfig.SharedSecret != "" {
		ttl := turnConfig.TTL
		if ttl == 0 {
			ttl = 3600
		}
		username, password, _ := auth.GenerateTURNCredentials(turnConfig.SharedSecret, ttl, config.ProviderConfig.TURNUsernameSuffix)
		turnIceServer := webrtc.ICEServer{
			URLs: []string{
				fmt.Sprintf("turn:%s:%d?transport=udp", turnConfig.Server, turnConfig.Port),
				fmt.Sprintf("turn:%s:%d?transport=tcp", turnConfig.Server, turnConfig.Port),
			},
			Username:   username,
			Credential: password,
		}
		iceServers = append(iceServers, turnIceServer)
	}

	return webrtc.Configuration{
		ICEServers: iceServers,
	}
}
