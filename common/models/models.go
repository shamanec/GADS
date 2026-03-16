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
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CustomLogger interface {
	LogDebug(eventName string, message string)
	LogInfo(eventName string, message string)
	LogError(eventName string, message string)
	LogWarn(eventName string, message string)
	LogFatal(eventName string, message string)
	LogPanic(eventName string, message string)
}

type User struct {
	Username     string   `json:"username" bson:"username" example:"john_doe"`
	Password     string   `json:"password" bson:"password,omitempty" example:"secure_password"`
	Role         string   `json:"role,omitempty" bson:"role" example:"user" enums:"admin,user"`
	ID           string   `json:"_id" bson:"_id,omitempty" example:"507f1f77bcf86cd799439011"`
	WorkspaceIDs []string `json:"workspace_ids" bson:"workspace_ids" example:"workspace_id_1,workspace_id_2"`
}

type DeviceStreamSettings struct {
	UDID                string `json:"udid" bson:"udid"`                                             // device UDID
	StreamTargetFPS     int    `json:"stream_target_fps,omitempty" bson:"stream_target_fps"`         // The target FPS for the MJPEG video streams
	StreamJpegQuality   int    `json:"stream_jpeg_quality,omitempty" bson:"stream_jpeg_quality"`     // The target JPEG quality for the MJPEG video streams
	StreamScalingFactor int    `json:"stream_scaling_factor,omitempty" bson:"stream_scaling_factor"` // The target scaling factor for the MJPEG video streams
}

type IOSModelData struct {
	Width  string
	Height string
	Model  string
}

type UpdateStreamSettings struct {
	TargetFPS     int `json:"target_fps,omitempty"`
	JpegQuality   int `json:"jpeg_quality,omitempty"`
	ScalingFactor int `json:"scaling_factor,omitempty"`
}

type DeviceInUseMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type DBFile struct {
	FileName   string             `json:"name" bson:"filename"`
	UploadDate primitive.DateTime `json:"upload_date" bson:"uploadDate"`
}

type GlobalSettings struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Type        string             `json:"type" bson:"type"`
	Settings    interface{}        `json:"settings" bson:"settings"`
	LastUpdated time.Time          `json:"last_updated" bson:"last_updated"`
}

type StreamSettings struct {
	TargetFPS            int `json:"target_fps,omitempty" bson:"target_fps"`
	JpegQuality          int `json:"jpeg_quality,omitempty" bson:"jpeg_quality"`
	ScalingFactorAndroid int `json:"scaling_factor_android,omitempty" bson:"scaling_factor_android"`
	ScalingFactoriOS     int `json:"scaling_factor_ios,omitempty" bson:"scaling_factor_ios"`
}

// ClientCredentials represents OAuth2 client credentials for API access
type ClientCredentials struct {
	ID           string     `json:"id" bson:"_id,omitempty"`
	ClientID     string     `json:"client_id" bson:"client_id"`
	ClientSecret string     `json:"-" bson:"client_secret"` // Never return in JSON (bcrypt hash)
	SecretLookup string     `json:"-" bson:"secret_lookup"` // SHA256 hash for efficient secret lookup
	Name         string     `json:"name" bson:"name"`
	Description  string     `json:"description" bson:"description"`
	UserID       string     `json:"user_id" bson:"user_id"`
	Tenant       string     `json:"tenant" bson:"tenant"`
	IsActive     bool       `json:"is_active" bson:"is_active"`
	CreatedAt    time.Time  `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" bson:"updated_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty" bson:"last_used_at,omitempty"`
}

type Workspace struct {
	ID          string `json:"id" bson:"_id,omitempty" example:"workspace_123"`
	Name        string `json:"name" bson:"name" example:"Development Team"`
	Description string `json:"description" bson:"description" example:"Workspace for development team testing"`
	IsDefault   bool   `json:"is_default" bson:"is_default" example:"false"`
	Tenant      string `json:"tenant" bson:"tenant,omitempty" example:"acme-corp"`
}

type WorkspaceWithDeviceCount struct {
	ID          string `json:"id" bson:"_id,omitempty" example:"workspace_123"`
	Name        string `json:"name" bson:"name" example:"Development Team"`
	Description string `json:"description" bson:"description" example:"Workspace for development team testing"`
	IsDefault   bool   `json:"is_default" bson:"is_default" example:"false"`
	Tenant      string `json:"tenant" bson:"tenant,omitempty" example:"acme-corp"`
	DeviceCount int    `json:"device_count" bson:"device_count" example:"5"`
}

type ProviderLog struct {
	EventName string `json:"eventname" bson:"eventname"`
	Level     string `json:"level" bson:"level"`
	Message   string `json:"message" bson:"message"`
	Timestamp int64  `json:"timestamp" bson:"timestamp"`
}

type TizenTVInfo struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Version   string      `json:"version"`
	Device    TizenDevice `json:"device"`
	Type      string      `json:"type"`
	URI       string      `json:"uri"`
	Remote    string      `json:"remote"`
	IsSupport string      `json:"isSupport"`
}

type TizenDevice struct {
	Type              string `json:"type"`
	DUID              string `json:"duid"`
	Model             string `json:"model"`
	ModelName         string `json:"modelName"`
	Description       string `json:"description"`
	NetworkType       string `json:"networkType"`
	SSID              string `json:"ssid"`
	IP                string `json:"ip"`
	FirmwareVersion   string `json:"firmwareVersion"`
	Name              string `json:"name"`
	ID                string `json:"id"`
	UDN               string `json:"udn"`
	Resolution        string `json:"resolution"`
	CountryCode       string `json:"countryCode"`
	MSFVersion        string `json:"msfVersion"`
	SmartHubAgreement string `json:"smartHubAgreement"`
	VoiceSupport      string `json:"VoiceSupport"`
	GamePadSupport    string `json:"GamePadSupport"`
	WifiMac           string `json:"wifiMac"`
	DeveloperMode     string `json:"developerMode"`
	DeveloperIP       string `json:"developerIP"`
	OS                string `json:"OS"`
}

type AndroidFileNode struct {
	Name     string                      `json:"name"`
	Children map[string]*AndroidFileNode `json:"children,omitempty"`
	IsFile   bool                        `json:"isFile"`
	FullPath string                      `json:"fullPath"`
	FileDate int64                       `json:"fileDate"`
}

type AppiumPluginConfiguration struct {
	ProviderUrl       string `json:"providerUrl"`
	UDID              string `json:"udid"`
	HeartBeatInterval string `json:"heartbeatIntervalMs"`
}
