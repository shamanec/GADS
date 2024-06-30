package models

type ConfigJsonData struct {
	EnvConfig ProviderDB `json:"env-config" bson:"env-config"`
}

type ProviderDB struct {
	OS                   string   `json:"os" bson:"os"`
	Nickname             string   `json:"nickname" bson:"nickname"`
	HostAddress          string   `json:"host_address" bson:"host_address"`
	Port                 int      `json:"port" bson:"port"`
	UseSeleniumGrid      bool     `json:"use_selenium_grid" bson:"use_selenium_grid"`
	SeleniumGrid         string   `json:"selenium_grid" bson:"selenium_grid"`
	ProvideAndroid       bool     `json:"provide_android" bson:"provide_android"`
	ProvideIOS           bool     `json:"provide_ios" bson:"provide_ios"`
	WdaBundleID          string   `json:"wda_bundle_id" bson:"wda_bundle_id"`
	WdaRepoPath          string   `json:"wda_repo_path" bson:"wda_repo_path"`
	SupervisionPassword  string   `json:"supervision_password" bson:"supervision_password"`
	ProviderFolder       string   `json:"-" bson:"-"`
	LastUpdatedTimestamp int64    `json:"last_updated" bson:"last_updated"`
	ProvidedDevices      []Device `json:"provided_devices" bson:"provided_devices"`
	WebDriverBinary      string   `json:"-" bson:"-"`
	UseGadsIosStream     bool     `json:"use_gads_ios_stream" bson:"use_gads_ios_stream"`
	UseCustomWDA         bool     `json:"use_custom_wda" bson:"use_custom_wda"`
}

type ProviderData struct {
	ProviderData ProviderDB `json:"provider"`
	DeviceData   []*Device  `json:"device_data"`
}

type HubConfig struct {
	HostAddress          string `json:"host_address"`
	Port                 string `json:"port"`
	MongoDB              string `json:"mongo_db"`
	SeleniumGridInstance string `json:"selenium_grid_instance"`
	OSTempDir            string `json:"-"`
	UIFilesTempDir       string `json:"-"`
}
