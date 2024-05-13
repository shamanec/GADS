package util

type ConfigJsonData struct {
	HostAddress          string `json:"host_address"`
	Port                 string `json:"port"`
	MongoDB              string `json:"mongo_db"`
	SeleniumGridInstance string `json:"selenium_grid_instance"`
	AdminUsername        string `json:"admin_username"`
	AdminPassword        string `json:"admin_password"`
	AdminEmail           string `json:"admin_email"`
	OSTempDir            string `json:"-"`
	UIFilesTempDir       string `json:"-"`
}

var ConfigData *ConfigJsonData
