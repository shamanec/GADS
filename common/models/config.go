/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package models

type Provider struct {
	OS               string `json:"os" bson:"os"`
	Nickname         string `json:"nickname" bson:"nickname"`
	HostAddress      string `json:"host_address" bson:"host_address"`
	Port             int    `json:"port" bson:"port"`
	ProvideAndroid   bool   `json:"provide_android" bson:"provide_android"`
	ProvideAndroidTv bool   `json:"provide_androidtv" bson:"provide_androidtv"`
	// Provide running Android emulators as ephemeral devices - discovered from `adb devices`,
	// never registered in the DB, identity derived from the AVD name.
	ProvideAndroidEmulators bool   `json:"provide_android_emulators" bson:"provide_android_emulators"`
	ProvideIOS              bool   `json:"provide_ios" bson:"provide_ios"`
	ProvideTizen            bool   `json:"provide_tizen" bson:"provide_tizen"`
	ProvideWebOS            bool   `json:"provide_webos" bson:"provide_webos"`
	ProvideRoku             bool   `json:"provide_roku" bson:"provide_roku"`
	WdaBundleID             string `json:"wda_bundle_id" bson:"wda_bundle_id"`
	WebDriverAgentIPA       string `json:"web_driver_agent_ipa" bson:"web_driver_agent_ipa"`
	BroadcastIPA            string `json:"broadcast_ipa" bson:"broadcast_ipa"`
	SupervisionPassword     string `json:"supervision_password" bson:"supervision_password"`
	ProviderFolder          string `json:"-" bson:"-"`
	LastUpdatedTimestamp    int64  `json:"last_updated" bson:"last_updated"`
	UseGadsIosStream        bool   `json:"use_gads_ios_stream" bson:"use_gads_ios_stream"`
	HubAddress              string `json:"hub_address" bson:"-"`
	SetupAppiumServers      bool   `json:"setup_appium_servers" bson:"setup_appium_servers"`
	TURNUsernameSuffix      string `json:"-" bson:"-"`
	UseIOSPairCache         bool   `json:"-" bson:"-"`
}

// ProviderDeviceSync is the lightweight struct sent from provider to hub each second
// for each device. It carries only the runtime fields the hub needs.
type ProviderDeviceSync struct {
	UDID          string `json:"udid"`
	Host          string `json:"host"`
	Connected     bool   `json:"connected"`
	ProviderState string `json:"provider_state"`
	// Appium session truth as reported by the device's appium-plugin. The marker
	// field distinguishes a provider that reports these from an older one whose
	// zero values would be indistinguishable from "no session" - the hub must not
	// act on the session fields unless the marker is true
	ReportsAppiumSessionState bool   `json:"reports_appium_session_state"`
	HasAppiumSession          bool   `json:"has_appium_session"`
	AppiumSessionID           string `json:"appium_session_id"`
	AppiumLastCommandTS       int64  `json:"appium_last_command_ts"`
	// Ephemeral device descriptor - set only for devices that exist solely in
	// provider memory (e.g. Android emulators) and have no DB record. The hub
	// upserts its device store entry from EphemeralDevice instead of the DB.
	// Both fields are omitted for regular devices so their payload is unchanged.
	Ephemeral       bool      `json:"ephemeral,omitempty"`
	EphemeralDevice *DBDevice `json:"ephemeral_device,omitempty"`
}

type ProviderData struct {
	ProviderData Provider             `json:"provider"`
	DeviceData   []ProviderDeviceSync `json:"device_data"`
}

type HubConfig struct {
	HostAddress        string `json:"host_address"`
	Port               string `json:"port"`
	MongoDB            string `json:"mongo_db"`
	OSTempDir          string `json:"-"`
	FilesTempDir       string `json:"-"`
	OS                 string `json:"os"`
	AuthEnabled        bool   `json:"auth_enabled"`
	TURNUsernameSuffix string `json:"-"`
}

type MinioConfig struct {
	Endpoint        string `json:"endpoint" bson:"endpoint"`
	AccessKeyID     string `json:"access_key_id" bson:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key" bson:"secret_access_key"`
	UseSSL          bool   `json:"use_ssl" bson:"use_ssl"`
	Enabled         bool   `json:"enabled" bson:"enabled"`
}

type TURNConfig struct {
	Server       string `json:"server" bson:"server"`
	Port         int    `json:"port" bson:"port"`
	SharedSecret string `json:"shared_secret" bson:"shared_secret"`
	TTL          int    `json:"ttl" bson:"ttl"` // Time-to-live in seconds (default: 3600)
	Enabled      bool   `json:"enabled" bson:"enabled"`
}

// RegularizeProviderState applies business rules to ensure provider configuration is consistent
// If SetupAppiumServers is false, ProvideTizen, ProvideWebOS, ProvideAndroidTv and ProvideRoku must also be false since they require Appium servers
func (p *Provider) RegularizeProviderState() {
	if !p.SetupAppiumServers {
		p.ProvideTizen = false
		p.ProvideWebOS = false
		p.ProvideAndroidTv = false
		p.ProvideRoku = false
	}
}
