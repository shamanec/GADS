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
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var ws_conn *websocket.Conn
var project_log_file *os.File

// Devices struct which contains
// an array of devices from the config.json
type Devices struct {
	Devices []Device `json:"devicesList"`
}

// Device struct which contains device info
type Device struct {
	AppiumPort      int    `json:"appium_port"`
	DeviceName      string `json:"device_name"`
	DeviceOSVersion string `json:"device_os_version"`
	DeviceUDID      string `json:"device_udid"`
	WdaMjpegPort    int    `json:"wda_mjpeg_port"`
	WdaPort         int    `json:"wda_port"`
}

// ProjectConfig struct which contains the project configuration values
type ProjectConfig struct {
	DevicesHost             string `json:"devices_host"`
	SeleniumHubHost         string `json:"selenium_hub_host"`
	SeleniumHubPort         string `json:"selenium_hub_port"`
	SeleniumHubProtocolType string `json:"selenium_hub_protocol_type"`
	WdaBundleID             string `json:"wda_bundle_id"`
}

type ProjectConfigPageData struct {
	WebDriverAgentProvided bool
	SudoPasswordSet        bool
	UdevIOSListenerStatus  string
	ImageStatus            string
	ProjectConfigValues    ProjectConfig
}

type ContainerRow struct {
	ContainerID     string
	ImageName       string
	ContainerStatus string
	ContainerPorts  string
	ContainerName   string
	DeviceUDID      string
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
	var projectConfig ProjectConfig
	json.Unmarshal(byteValue, &projectConfig)
	var configRow = ProjectConfig{
		DevicesHost:             projectConfig.DevicesHost,
		SeleniumHubHost:         projectConfig.SeleniumHubHost,
		SeleniumHubPort:         projectConfig.SeleniumHubPort,
		SeleniumHubProtocolType: projectConfig.SeleniumHubProtocolType,
		WdaBundleID:             projectConfig.WdaBundleID}

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
	requestBody, _ := ioutil.ReadAll(r.Body)
	devices_host := gjson.Get(string(requestBody), "devices_host").Str
	selenium_hub_host := gjson.Get(string(requestBody), "selenium_hub_host").Str
	selenium_hub_port := gjson.Get(string(requestBody), "selenium_hub_port").Str
	selenium_hub_protocol_type := gjson.Get(string(requestBody), "selenium_hub_protocol_type").Str
	wda_bundle_id := gjson.Get(string(requestBody), "wda_bundle_id").Str
	// Open the configuration json file
	jsonFile, err := os.Open("./configs/config.json")
	if err != nil {
		JSONError(w, "config_file_interaction", "Could not open the config.json file.", 500)
		return
	}
	defer jsonFile.Close()

	// Read the configuration json file into byte array
	configJson, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		JSONError(w, "config_file_interaction", "Could not read the config.json file.", 500)
		return
	}

	var updatedJSON string
	updatedJSON, _ = sjson.Set(string(configJson), "devicesList.-1", devices_host)

	if devices_host != "" {
		updatedJSON, _ = sjson.Set(string(configJson), "devices_host", devices_host)
	}
	if selenium_hub_host != "" {
		updatedJSON, _ = sjson.Set(string(configJson), "selenium_hub_host", selenium_hub_host)
	}
	if selenium_hub_port != "" {
		updatedJSON, _ = sjson.Set(string(configJson), "selenium_hub_port", selenium_hub_port)
	}
	if selenium_hub_protocol_type != "" {
		updatedJSON, _ = sjson.Set(string(configJson), "selenium_hub_protocol_type", selenium_hub_protocol_type)
	}
	if wda_bundle_id != "" {
		updatedJSON, _ = sjson.Set(string(configJson), "wda_bundle_id", wda_bundle_id)
	}

	// Prettify the json so it looks good inside the file
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, []byte(updatedJSON), "", "  ")

	err = ioutil.WriteFile("./configs/config.json", []byte(prettyJSON.String()), 0644)
	if err != nil {
		JSONError(w, "config_file_interaction", "Could not write to the config.json file.", 500)
		return
	}
	SimpleJSONResponse(w, "config_file_interaction", "Successfully updated project config in ./configs/config.json", 200)
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
	myRouter.HandleFunc("/ios-containers/update", UpdateIOSContainers).Methods("POST")

	// Android containers endpoints

	// General containers endpoints
	myRouter.HandleFunc("/containers/{container_id}/restart", RestartContainer).Methods("POST")
	myRouter.HandleFunc("/containers/{container_id}/remove", RemoveContainer).Methods("POST")
	myRouter.HandleFunc("/containers/{container_id}/logs", GetContainerLogs).Methods("GET")
	myRouter.HandleFunc("/containers/running-containers", GetRunningContainerNames).Methods("GET")

	// Configuration endpoints
	myRouter.HandleFunc("/configuration/build-image", BuildDockerImage).Methods("POST")
	myRouter.HandleFunc("/configuration/remove-image", RemoveDockerImage).Methods("POST")
	myRouter.HandleFunc("/configuration/setup-ios-listener", SetupUdevListener).Methods("POST")
	myRouter.HandleFunc("/configuration/remove-ios-listener", RemoveUdevListener).Methods("POST")
	myRouter.HandleFunc("/configuration/update-config", UpdateProjectConfigHandler).Methods("PUT")
	myRouter.HandleFunc("/configuration/set-sudo-password", SetSudoPassword).Methods("PUT")
	myRouter.HandleFunc("/configuration/upload-wda", UploadWDA).Methods("POST")
	myRouter.HandleFunc("/configuration/upload-app", UploadApp).Methods("POST")

	// Devices endpoints
	myRouter.HandleFunc("/device/{device_udid}", ReturnDeviceInfo).Methods("GET")
	myRouter.HandleFunc("/device-logs/{log_type}/{device_udid}", GetDeviceLogs).Methods("GET")
	myRouter.HandleFunc("/ios-devices", GetConnectedIOSDevices).Methods("GET")
	myRouter.HandleFunc("/ios-devices/register", RegisterIOSDevice).Methods("POST")
	myRouter.HandleFunc("/ios-devices/{device_udid}/device-state", IOSDeviceState).Methods("POST", "GET")
	myRouter.HandleFunc("/ios-devices/{device_udid}/info", GetIOSDeviceInfo).Methods("GET")
	myRouter.HandleFunc("/ios-devices/{device_udid}/install-app", InstallIOSApp).Methods("POST")
	myRouter.HandleFunc("/ios-devices/{device_udid}/uninstall-app", UninstallIOSApp).Methods("POST")
	myRouter.HandleFunc("/ios-devices/{device_udid}/wda-stream-url", GetIOSDeviceMjpegStreamURL).Methods("GET")
	myRouter.HandleFunc("/devices/device-control", GetDeviceControlInfo).Methods("GET")

	// Logs
	myRouter.HandleFunc("/project-logs", GetLogs).Methods("GET")

	// Asset endpoints
	myRouter.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	myRouter.PathPrefix("/main/").Handler(http.StripPrefix("/main/", http.FileServer(http.Dir("./"))))

	// Page loads
	myRouter.HandleFunc("/configuration.html", GetProjectConfigurationPage)
	myRouter.HandleFunc("/android-containers.html", getAndroidContainers)
	myRouter.HandleFunc("/ios-containers.html", GetIOSContainers)
	myRouter.HandleFunc("/project-logs.html", GetLogsPage)
	myRouter.HandleFunc("/device-control.html", GetDeviceControlPage)
	myRouter.HandleFunc("/", GetInitialPage)

	//log.Fatal(http.ListenAndServeTLS(":10000", "ca-cert.pem", "ca-key.pem", myRouter))
	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

func main() {
	setLogging()
	handleRequests()
}
