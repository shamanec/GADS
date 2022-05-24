package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	_ "GADS/docs"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	httpSwagger "github.com/swaggo/http-swagger"
)

var project_log_file *os.File

type ProjectConfigPageData struct {
	WebDriverAgentProvided bool
	SudoPasswordSet        bool
	UdevIOSListenerStatus  string
	ImageStatus            string
	ProjectConfigValues    AppiumConfig
}

type AppiumConfig struct {
	DevicesHost             string `json:"devices_host"`
	SeleniumHubHost         string `json:"selenium_hub_host"`
	SeleniumHubPort         string `json:"selenium_hub_port"`
	SeleniumHubProtocolType string `json:"selenium_hub_protocol_type"`
	WDABundleID             string `json:"wda_bundle_id"`
}

type DeviceConfig struct {
	OS              string `json:"os"`
	AppiumPort      int    `json:"appium_port"`
	DeviceName      string `json:"device_name"`
	DeviceOSVersion string `json:"device_os_version"`
	DeviceUDID      string `json:"device_udid"`
	WDAMjpegPort    int    `json:"wda_mjpeg_port,omitempty"`
	WDAPort         int    `json:"wda_port,omitempty"`
	ViewportSize    string `json:"viewport_size,omitempty"`
	StreamPort      int    `json:"stream_port,omitempty"`
}

type EnvConfig struct {
	SudoPassword        string `json:"sudo_password"`
	ConnectSeleniumGrid bool   `json:"connect_selenium_grid"`
	SupervisionPassword string `json:"supervision_password"`
}

type ConfigJsonData struct {
	AppiumConfig AppiumConfig   `json:"appium-config"`
	EnvConfig    EnvConfig      `json:"env-config"`
	DeviceConfig []DeviceConfig `json:"devices-config"`
}

// Load the initial page
func GetInitialPage(w http.ResponseWriter, r *http.Request) {
	var index = template.Must(template.ParseFiles("static/index.html"))
	if err := index.Execute(w, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "index_page_load",
		}).Error("Couldn't load index.html")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Load the initial page with the project configuration info
func GetProjectConfigurationPage(w http.ResponseWriter, r *http.Request) {
	jsonFile, err := os.Open("./configs/config.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var projectConfig ConfigJsonData
	json.Unmarshal(byteValue, &projectConfig)
	var configRow = AppiumConfig{
		DevicesHost:             projectConfig.AppiumConfig.DevicesHost,
		SeleniumHubHost:         projectConfig.AppiumConfig.SeleniumHubHost,
		SeleniumHubPort:         projectConfig.AppiumConfig.SeleniumHubPort,
		SeleniumHubProtocolType: projectConfig.AppiumConfig.SeleniumHubProtocolType,
		WDABundleID:             projectConfig.AppiumConfig.WDABundleID}

	var index = template.Must(template.ParseFiles("static/project_config.html"))
	pageData := ProjectConfigPageData{WebDriverAgentProvided: CheckWDAProvided(), SudoPasswordSet: CheckSudoPasswordSet(), UdevIOSListenerStatus: UdevIOSListenerState(), ImageStatus: ImageExists(), ProjectConfigValues: configRow}
	if err := index.Execute(w, pageData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// @Summary      Update project configuration
// @Description  Updates one  or multiple configuration values
// @Tags         configuration
// @Param        config body ProjectConfig true "Update config"
// @Accept		 json
// @Produce      json
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /configuration/update-config [put]
func UpdateProjectConfigHandler(w http.ResponseWriter, r *http.Request) {
	var requestData AppiumConfig
	err := UnmarshalRequestBody(r.Body, &requestData)

	// Get the config data
	var configData ConfigJsonData
	err = UnmarshalJSONFile("./configs/config.json", &configData)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not unmarshal config.json file when trying to change configuration values.")
		return
	}

	if requestData.DevicesHost != "" {
		configData.AppiumConfig.DevicesHost = requestData.DevicesHost
	}
	if requestData.SeleniumHubHost != "" {
		configData.AppiumConfig.SeleniumHubHost = requestData.SeleniumHubHost
	}
	if requestData.SeleniumHubPort != "" {
		configData.AppiumConfig.SeleniumHubPort = requestData.SeleniumHubPort
	}
	if requestData.SeleniumHubProtocolType != "" {
		configData.AppiumConfig.SeleniumHubProtocolType = requestData.SeleniumHubProtocolType
	}
	if requestData.WDABundleID != "" {
		configData.AppiumConfig.WDABundleID = requestData.WDABundleID
	}

	bs, err := json.MarshalIndent(configData, "", "  ")

	err = ioutil.WriteFile("./configs/config.json", bs, 0644)
	if err != nil {
		JSONError(w, "config_file_interaction", "Could not write to the config.json file.", 500)
		return
	}
	SimpleJSONResponse(w, "Successfully updated project config in ./configs/config.json", 200)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func setLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	project_log_file, err := os.OpenFile("./logs/project.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic(err)
	}
	log.SetOutput(project_log_file)
}

func GetLogsPage(w http.ResponseWriter, r *http.Request) {
	var logs_page = template.Must(template.ParseFiles("static/project_logs.html"))
	if err := logs_page.Execute(w, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "project_logs_page",
		}).Error("Couldn't load project_logs.html")
		return
	}
}

func GetDeviceControlPage(w http.ResponseWriter, r *http.Request) {
	var device_control_page = template.Must(template.ParseFiles("static/device_control.html"))
	if err := device_control_page.Execute(w, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "device_control_page",
		}).Error("Couldn't load device_control.html")
		return
	}
}

// @Summary      Get project logs
// @Description  Provides project logs as plain text response
// @Tags         project-logs
// @Success      200
// @Failure      200
// @Router       /project-logs [get]
func GetLogs(w http.ResponseWriter, r *http.Request) {
	// Execute the command to restart the container by container ID
	commandString := "tail -n 1000 ./logs/project.log"
	cmd := exec.Command("bash", "-c", commandString)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_project_logs",
		}).Error("Attempted to get project logs but no logs available.")
		fmt.Fprintf(w, "No logs available")
		return
	}
	//SimpleJSONResponse(w, "get_project_logs", out.String(), 200)
	fmt.Fprintf(w, out.String())
}

type Order struct {
	Username string `json:"username"`
	Fullname string `json:"fullname"`
}

func handleRequests() {
	// Create a new instance of the mux router
	myRouter := mux.NewRouter().StrictSlash(true)

	myRouter.PathPrefix("/swagger").Handler(httpSwagger.WrapHandler)

	myRouter.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("http://localhost:10000/swagger/doc.json"), //The url pointing to API definition
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("#swagger-ui"),
	))

	// iOS containers endpoints

	// Android containers endpoints

	// General containers endpoints
	myRouter.HandleFunc("/containers/{container_id}/restart", RestartContainer).Methods("POST")
	myRouter.HandleFunc("/containers/{container_id}/remove", RemoveContainer).Methods("POST")
	myRouter.HandleFunc("/containers/{container_id}/logs", GetContainerLogs).Methods("GET")
	myRouter.HandleFunc("/device-containers/remove", RemoveDeviceContainer).Methods("POST")
	myRouter.HandleFunc("/device-containers/create", CreateDeviceContainer).Methods("POST")

	// Configuration endpoints
	myRouter.HandleFunc("/configuration/build-image/{image_type}", BuildDockerImage).Methods("POST")
	myRouter.HandleFunc("/configuration/remove-image", RemoveDockerImage).Methods("POST")
	myRouter.HandleFunc("/configuration/setup-ios-listener", SetupUdevListener).Methods("POST")
	myRouter.HandleFunc("/configuration/remove-ios-listener", RemoveUdevListener).Methods("POST")
	myRouter.HandleFunc("/configuration/update-config", UpdateProjectConfigHandler).Methods("PUT")
	myRouter.HandleFunc("/configuration/set-sudo-password", SetSudoPassword).Methods("PUT")
	myRouter.HandleFunc("/configuration/upload-wda", UploadWDA).Methods("POST")
	myRouter.HandleFunc("/configuration/upload-app", UploadApp).Methods("POST")

	// Devices endpoints
	myRouter.HandleFunc("/device-logs/{log_type}/{device_udid}", GetDeviceLogs).Methods("GET")
	myRouter.HandleFunc("/ios-devices", GetConnectedIOSDevices).Methods("GET")
	myRouter.HandleFunc("/ios-devices/{device_udid}/install-app", InstallIOSApp).Methods("POST")
	myRouter.HandleFunc("/ios-devices/{device_udid}/uninstall-app", UninstallIOSApp).Methods("POST")
	myRouter.HandleFunc("/devices/device-control", GetDeviceControlInfo).Methods("GET")

	// Logs
	myRouter.HandleFunc("/project-logs", GetLogs).Methods("GET")

	// Asset endpoints
	myRouter.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	myRouter.PathPrefix("/main/").Handler(http.StripPrefix("/main/", http.FileServer(http.Dir("./"))))

	// Page loads
	myRouter.HandleFunc("/configuration", GetProjectConfigurationPage)
	myRouter.HandleFunc("/device-containers", LoadDeviceContainers)
	myRouter.HandleFunc("/refresh-device-containers", RefreshDeviceContainers)
	myRouter.HandleFunc("/logs", GetLogsPage)
	myRouter.HandleFunc("/device-control", GetDeviceControlPage)
	myRouter.HandleFunc("/", GetInitialPage)

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

func main() {
	setLogging()
	handleRequests()
}
