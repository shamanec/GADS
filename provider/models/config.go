package models

type ConfigJsonData struct {
	EnvConfig ProviderDB `json:"env-config" bson:"env-config"`
}

type ProviderDB struct {
	OS                   string `json:"os" bson:"os"`
	Nickname             string `json:"nickname" bson:"nickname"`
	HostAddress          string `json:"host_address" bson:"host_address"`
	Port                 int    `json:"port" bson:"port"`
	UseSeleniumGrid      bool   `json:"use_selenium_grid" bson:"use_selenium_grid"`
	SeleniumGrid         string `json:"selenium_grid" bson:"selenium_grid"`
	ProvideAndroid       bool   `json:"provide_android" bson:"provide_android"`
	ProvideIOS           bool   `json:"provide_ios" bson:"provide_ios"`
	WdaBundleID          string `json:"wda_bundle_id" bson:"wda_bundle_id"`
	SupervisionPassword  string `json:"supervision_password" bson:"supervision_password"`
	WdaRepoPath          string `json:"wda_repo_path" bson:"wda_repo_path"`
	ProviderFolder       string `json:"-" bson:"-"`
	LastUpdatedTimestamp int64  `json:"last_updated" bson:"last_updated"`
	ProvidedDevices      int    `json:"provided_devices_count" bson:"provided_devices_count"`
	WebDriverBinary      string `json:"-" bson:"-"`
	SeleniumJarFile      string `json:"-" bson:"-"`
	UseGadsIosStream     bool   `json:"use_gads_ios_stream" bson:"use_gads_ios_stream"`
	UseCustomWDA         bool   `json:"use_custom_wda" bson:"use_custom_wda"`
}

type ProviderData struct {
	ProviderData ProviderDB `json:"provider"`
	DeviceData   []*Device  `json:"device_data"`
}
