package models

type User struct {
	Username string `json:"username" bson:"username"`
	Email    string `json:"email" bson:"email"`
	Password string `json:"password" bson:"password"`
	Role     string `json:"role,omitempty" bson:"role"`
	ID       string `json:"_id" bson:"_id,omitempty"`
}

type Device struct {
	Connected            bool   `json:"connected" bson:"connected"`
	UDID                 string `json:"udid" bson:"udid"`
	OS                   string `json:"os" bson:"os"`
	Name                 string `json:"name" bson:"name"`
	OSVersion            string `json:"os_version" bson:"os_version"`
	Model                string `json:"model" bson:"model"`
	Image                string `json:"image,omitempty" bson:"image,omitempty"`
	HostAddress          string `json:"host_address" bson:"host_address"`
	InUse                bool   `json:"in_use"`
	ScreenWidth          string `json:"screen_width" bson:"screen_width"`
	ScreenHeight         string `json:"screen_height" bson:"screen_height"`
	LastUpdatedTimestamp int64  `json:"last_updated_timestamp" bson:"last_updated_timestamp"`
	Available            bool   `json:"available" bson:"-"`
	ProviderState        string `json:"provider_state" bson:"provider_state"`
	Provider             string `json:"provider" bson:"provider"`
}

type ProviderDB struct {
	OS                   string            `json:"os" bson:"os"`
	Nickname             string            `json:"nickname" bson:"nickname"`
	HostAddress          string            `json:"host_address" bson:"host_address"`
	Port                 int               `json:"port" bson:"port"`
	UseSeleniumGrid      bool              `json:"use_selenium_grid" bson:"use_selenium_grid"`
	SeleniumGrid         string            `json:"selenium_grid" bson:"selenium_grid"`
	ProvideAndroid       bool              `json:"provide_android" bson:"provide_android"`
	ProvideIOS           bool              `json:"provide_ios" bson:"provide_ios"`
	WdaBundleID          string            `json:"wda_bundle_id" bson:"wda_bundle_id"`
	WdaRepoPath          string            `json:"wda_repo_path" bson:"wda_repo_path"`
	SupervisionPassword  string            `json:"supervision_password" bson:"supervision_password"`
	ConnectedDevices     []ConnectedDevice `json:"connected_devices" bson:"connected_devices"`
	LastUpdatedTimestamp int64             `json:"last_updated" bson:"last_updated"`
	ProvidedDevices      []Device          `json:"provided_devices" bson:"provided_devices"`
}

type ConnectedDevice struct {
	OS           string `json:"os" bson:"os"`
	UDID         string `json:"udid" bson:"udid"`
	IsConfigured bool   `json:"is_configured" bson:"-"`
}
