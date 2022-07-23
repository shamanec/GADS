package main

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var project_dir, _ = os.Getwd()
var on_grid = GetEnvValue("connect_selenium_grid")

type ContainerRow struct {
	ContainerID     string
	ImageName       string
	ContainerStatus string
	ContainerPorts  string
	ContainerName   string
	DeviceUDID      string
}

type CreateDeviceContainerRequest struct {
	DeviceType string `json:"device_type"`
	Udid       string `json:"udid"`
}

type RemoveDeviceContainerData struct {
	Udid string `json:"udid"`
}

//=======================//
//=====API FUNCTIONS=====//

// IOS Containers html page
func LoadDeviceContainers(w http.ResponseWriter, r *http.Request) {

	// Generate all available device container rows
	rows, err := deviceContainerRows()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Make functions available in the html template
	funcMap := template.FuncMap{
		"contains": strings.Contains,
	}

	// Parse the template and return response with the container table rows
	var tmpl = template.Must(template.New("device_containers.html").Funcs(funcMap).ParseFiles("static/device_containers.html", "static/device_containers_table.html"))
	if err := tmpl.ExecuteTemplate(w, "device_containers.html", rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// @Summary      Build docker images
// @Description  Starts building a docker image in a goroutine and just returns Accepted.
// @Tags         configuration
// @Param        image_type path string true "Image type: ios-appium, android-appium"
// @Success      202
// @Router       /configuration/build-image/{image_type} [post]
func BuildDockerImage(w http.ResponseWriter, r *http.Request) {
	// Get the request path vars
	vars := mux.Vars(r)
	image_type := vars["image_type"]

	// Start building the image in a goroutine and immediately return Accepted
	go buildDockerImage(image_type)
	w.WriteHeader(http.StatusAccepted)
}

// @Summary      Remove 'ios-appium' or 'android-appium' image
// @Description  Removes the 'ios-appium' or 'android-appium' Docker image
// @Tags         configuration
// @Produce      json
// @Param        image_type path string true "Image type: ios-appium, android-appium"
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /configuration/remove-image/{image_type} [post]
func RemoveDockerImage(w http.ResponseWriter, r *http.Request) {
	// Get the request path vars
	vars := mux.Vars(r)
	image_type := vars["image_type"]

	// Set a generic error message to reuse
	error_message := "Could not remove " + image_type + " image."

	// Create the context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_image_remove",
		}).Error("Could not create docker client while attempting to remove ios-appium image. Error: " + err.Error())
		JSONError(w, "docker_image_remove", error_message, 500)
		return
	}

	// Try to remove the image by its name
	imageRemoveResponse, err := cli.ImageRemove(ctx, image_type, types.ImageRemoveOptions{PruneChildren: true})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_image_remove",
		}).Error(error_message + " Error: " + err.Error())
		JSONError(w, "docker_image_remove", error_message, 500)
		return
	}
	SimpleJSONResponse(w, "Successfully removed image tagged: '"+imageRemoveResponse[0].Untagged+"'", 200)
}

// @Summary      Restart container
// @Description  Restarts container by provided container ID
// @Tags         containers
// @Produce      json
// @Param        container_id path string true "Container ID"
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /containers/{container_id}/restart [post]
func RestartContainer(w http.ResponseWriter, r *http.Request) {
	// Get the request path vars
	vars := mux.Vars(r)
	container_id := vars["container_id"]

	log.WithFields(log.Fields{
		"event": "docker_container_restart",
	}).Info("Attempting to restart container with ID: " + container_id)

	// Call the internal function to restart the container
	err := RestartContainerInternal(container_id)
	if err != nil {
		JSONError(w, "docker_container_restart", "Could not restart container with ID: "+container_id, 500)
	}

	SimpleJSONResponse(w, "Successfully attempted to restart container with ID: "+container_id, 200)
}

// @Summary      Get container logs
// @Description  Get logs of container by provided container ID
// @Tags         containers
// @Produce      json
// @Param        container_id path string true "Container ID"
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /containers/{container_id}/logs [get]
func GetContainerLogs(w http.ResponseWriter, r *http.Request) {
	// Get the request path vars
	vars := mux.Vars(r)
	key := vars["container_id"]

	// Create the context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_logs",
		}).Error("Could not create docker client while attempting to get logs for container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "get_container_logs", "Could not get logs for container with ID: "+key, 500)
		return
	}

	// Create the options for the container logs function
	options := types.ContainerLogsOptions{ShowStdout: true}

	// Get the container logs
	out, err := cli.ContainerLogs(ctx, key, options)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_logs",
		}).Error("Could not get logs for container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "get_container_logs", "Could not get logs for container with ID: "+key, 500)
		return
	}

	// Get the ReadCloser of the logs into a buffer
	// And convert it to string
	buf := new(bytes.Buffer)
	buf.ReadFrom(out)
	newStr := buf.String()

	// If there are any logs - reply with them
	// Or reply with a generic string
	if newStr != "" {
		SimpleJSONResponse(w, newStr, 200)
	} else {
		SimpleJSONResponse(w, "There are no actual logs for this container.", 200)
	}
}

// @Summary      Create container for device
// @Description  Creates a container for a connected registered device
// @Tags         device-containers
// @Param        config body CreateDeviceContainerRequest true "Create container for device"
// @Success      202
// @Router       /device-containers/create [post]
func CreateDeviceContainer(w http.ResponseWriter, r *http.Request) {
	var data CreateDeviceContainerRequest

	// Read the request data
	err := UnmarshalReader(r.Body, &data)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_container_create",
		}).Error("Could not unmarshal request body when creating container: " + err.Error())
		return
	}

	os_type := data.DeviceType
	device_udid := data.Udid

	// Start creating a device container in a goroutine and immediately reply with Accepted
	go func() {
		// Check if container exists and get the container ID and current status
		container_exists, container_id, _ := checkContainerExistsByName(device_udid)

		// Create a container if no container exists for this device
		// or restart a non-running container that already exists for this device
		// this is useful after restart and reconnecting devices
		if !container_exists {
			if os_type == "android" {
				go CreateAndroidContainer(device_udid)
			} else if os_type == "ios" {
				go CreateIOSContainer(device_udid)
			}
			return
		} else {
			go RestartContainerInternal(container_id)
			return
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}

// @Summary      Remove container for device
// @Description  Removes a running container for a disconnected registered device by device UDID
// @Tags         device-containers
// @Param        config body RemoveDeviceContainerData true "Remove container for device"
// @Success      202
// @Router       /device-containers/remove [post]
func RemoveDeviceContainer(w http.ResponseWriter, r *http.Request) {
	var data RemoveDeviceContainerData

	// Read the request data
	err := UnmarshalReader(r.Body, &data)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_container_remove",
		}).Error("Could not unmarshal request body when removing container: " + err.Error())
		return
	}

	// Check if container exists and get the container ID
	container_exists, container_id, _ := checkContainerExistsByName(data.Udid)

	if container_exists {
		// Start removing the container in a goroutine and immediately reply with Accepted
		go removeContainerByID(container_id)
	}
	w.WriteHeader(http.StatusAccepted)
}

// @Summary      Remove container
// @Description  Removes container by provided container ID
// @Tags         containers
// @Produce      json
// @Param        container_id path string true "Container ID"
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /containers/{container_id}/remove [post]
func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	// Get the request path vars
	vars := mux.Vars(r)
	key := vars["container_id"]

	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Attempting to remove container with ID: " + key)

	// Create a new context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not create docker client while attempting to remove container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "docker_container_remove", "Could not remove container with ID: "+key, 500)
		return
	}

	// Try to stop the container
	if err := cli.ContainerStop(ctx, key, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "docker_container_remove", "Could not remove container with ID: "+key, 500)
		return
	}

	// Try to remove the stopped container
	if err := cli.ContainerRemove(ctx, key, types.ContainerRemoveOptions{}); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "docker_container_remove", "Could not remove container with ID: "+key, 500)
		return
	}

	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Successfully removed container with ID: " + key)
	SimpleJSONResponse(w, "Successfully removed container with ID: "+key, 200)
}

// @Summary      Refresh the device-containers data
// @Description  Refreshes the device-containers data by returning an updated HTML table
// @Produce      html
// @Success      200
// @Failure      500
// @Router       /refresh-device-containers [post]
func RefreshDeviceContainers(w http.ResponseWriter, r *http.Request) {
	// Generate the data for each device container row in a slice of ContainerRow
	rows, err := deviceContainerRows()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Make functions available in html template
	funcMap := template.FuncMap{
		// The name "title" is what the function will be called in the template text.
		"contains": strings.Contains,
	}

	// Parse the template and return response with the container table rows
	// This will generate only the device table, not the whole page
	var tmpl = template.Must(template.New("device_containers_table").Funcs(funcMap).ParseFiles("static/device_containers_table.html"))

	// Reply with the new table
	if err := tmpl.ExecuteTemplate(w, "device_containers_table", rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

//===================//
//=====FUNCTIONS=====//

// Build a Docker image by device type
func buildDockerImage(image_type string) {
	log.WithFields(log.Fields{
		"event": "docker_image_build",
	}).Info("Started building the '" + image_type + "' docker image.")

	// Delete the build-context.tar file if it exists already
	DeleteFileShell("./build-context.tar", sudo_password)

	var files []string
	var image_name string
	var docker_file_name string

	// Set up the needed files and image name based on the image type requested
	if image_type == "ios-appium" {
		docker_file_name = "Dockerfile-iOS"
		files = []string{docker_file_name, "configs/nodeconfiggen.sh", "configs/ios-sync.sh", "apps/WebDriverAgent.ipa", "configs/supervision.p12", "configs/container-server"}
		image_name = "ios-appium"
	} else if image_type == "android-appium" {
		docker_file_name = "Dockerfile-Android"
		files = []string{docker_file_name, "configs/nodeconfiggen-android.sh", "configs/android-sync.sh", "configs/container-server"}
		image_name = "android-appium"
	} else {
		return
	}

	// Create a tar to be used as build-context for the image build
	// The tar should include all files needed by the Dockerfile to successfully create the image
	tarFile, err := os.Create("build-context.tar")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_image_build",
		}).Error("Could not create build-context.tar archive file. Error: " + err.Error())
		return
	}
	defer tarFile.Close()

	// Add all the data to the build-context.tar
	err = CreateArchive(files, tarFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_image_build",
		}).Error("Could not create build-context.tar archive. Error: " + err.Error())
		return
	}

	// Create a new context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_image_build",
		}).Error("Could not create docker client while attempting to build image. Error: " + err.Error())
		return
	}

	// Read the build-context tar into bytes.Reader
	buildContextFileReader, err := os.Open("build-context.tar")
	readBuildContextFile, err := ioutil.ReadAll(buildContextFileReader)
	buildContextTarReader := bytes.NewReader(readBuildContextFile)

	// Build the Docker image using the tar reader and the provided options
	_, err = cli.ImageBuild(ctx, buildContextTarReader, types.ImageBuildOptions{Dockerfile: docker_file_name, Remove: true, Tags: []string{image_name}})
	if err != nil {
		// Get the image build logs on failure
		log.WithFields(log.Fields{
			"event": "docker_image_build",
		}).Error("Could not build docker image. Please build it from terminal using: docker build -t " + image_type + " -f Dockerifle-iOS; docker build -t " + image_type + " -f Dockerifle-Android to observe the output.")
		return
	}

	log.WithFields(log.Fields{
		"event": "docker_image_build",
	}).Info("Built '" + image_name + "' docker image:\n")
}

// TODO - Revise this
// Check if the 'ios-appium' image exists and return info string
func ImageExists() (imageStatus string) {
	// Create a new context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_image_status",
		}).Error("Could not create docker client while attempting to get image status. Error:" + err.Error())
		imageStatus = "Image status undefined"
		return
	}

	imageListResponse, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_image_status",
		}).Error("Could not get image list while attempting to get image status. Error:" + err.Error())
		imageStatus = "Image status undefined"
		return
	}

	// Loop through the images list and search for the 'ios-appium' image
	for i := 0; i < len(imageListResponse); i++ {
		if strings.Contains(imageListResponse[i].RepoTags[0], "ios-appium") {
			imageStatus = "Image available"
			return
		}
	}
	imageStatus = "Image not available"
	return
}

// Create an iOS container for a specific device(by UDID) using data from config.json so if device is not registered there it will not attempt to create a container for it
func CreateIOSContainer(device_udid string) {
	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Attempting to create a container for iOS device with udid: " + device_udid)

	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not unmarshal config.json file when trying to create a container for device with udid: " + device_udid)
		return
	}

	// Check if device is registered in config data
	var device_in_config bool
	var deviceConfig DeviceConfig
	for _, v := range configData.DeviceConfig {
		if v.DeviceUDID == device_udid {
			device_in_config = true
			deviceConfig = v
		}
	}

	// Stop execution if device not in config data
	if !device_in_config {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
		return
	}

	// Get the device specific config data
	appium_port := strconv.Itoa(deviceConfig.AppiumPort)
	device_name := deviceConfig.DeviceName
	device_os_version := deviceConfig.DeviceOSVersion
	wda_mjpeg_port := strconv.Itoa(deviceConfig.WDAMjpegPort)
	wda_port := strconv.Itoa(deviceConfig.WDAPort)
	wda_bundle_id := configData.AppiumConfig.WDABundleID
	selenium_hub_port := configData.AppiumConfig.SeleniumHubPort
	selenium_hub_host := configData.AppiumConfig.SeleniumHubHost
	devices_host := configData.AppiumConfig.DevicesHost
	hub_protocol := configData.AppiumConfig.SeleniumHubProtocolType
	containerized_usbmuxd := configData.EnvConfig.ContainerizedUsbmuxd
	screen_size := deviceConfig.ScreenSize
	container_server_port := strconv.Itoa(deviceConfig.ContainerServerPort)

	// Create docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + device_udid)
		return
	}

	// Create the container config
	config := &container.Config{
		Image: "ios-appium",
		ExposedPorts: nat.PortSet{
			nat.Port("4723"):                struct{}{},
			nat.Port(wda_port):              struct{}{},
			nat.Port(wda_mjpeg_port):        struct{}{},
			nat.Port(container_server_port): struct{}{},
		},
		Env: []string{"ON_GRID=" + on_grid,
			"APPIUM_PORT=" + appium_port,
			"DEVICE_UDID=" + device_udid,
			"WDA_PORT=" + wda_port,
			"MJPEG_PORT=" + wda_mjpeg_port,
			"DEVICE_OS_VERSION=" + device_os_version,
			"DEVICE_NAME=" + device_name,
			"WDA_BUNDLEID=" + wda_bundle_id,
			"SUPERVISION_PASSWORD=" + GetEnvValue("supervision_password"),
			"SELENIUM_HUB_PORT=" + selenium_hub_port,
			"SELENIUM_HUB_HOST=" + selenium_hub_host,
			"DEVICES_HOST=" + devices_host,
			"HUB_PROTOCOL=" + hub_protocol,
			"CONTAINERIZED_USBMUXD=" + containerized_usbmuxd,
			"SCREEN_SIZE=" + screen_size,
			"CONTAINER_SERVER_PORT=" + container_server_port,
			"DEVICE_OS=ios"},
	}

	bindOptions := &mount.BindOptions{Propagation: "shared"}
	var mounts []mount.Mount

	if containerized_usbmuxd == "false" {
		// Mount all iOS devices on the host to the container with /var/run/usbmuxd
		// Mount /var/lib/lockdown so you don't have to trust device on each new container
		mounts = []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: "/var/lib/lockdown",
				Target: "/var/lib/lockdown",
			},
			{
				Type:   mount.TypeBind,
				Source: "/var/run/usbmuxd",
				Target: "/var/run/usbmuxd",
			},
		}
	} else {
		// Mount the symlink for a specific device created by udev rule - usbmuxd will be started in the container itself
		mounts = []mount.Mount{
			{
				Type:        mount.TypeBind,
				Source:      "/dev/device_" + device_udid,
				Target:      "/dev/device_" + device_udid,
				BindOptions: bindOptions,
			},
		}
	}

	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: project_dir + "/logs/container_" + device_name + "-" + device_udid,
		Target: "/opt/logs",
	})

	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: project_dir + "/apps",
		Target: "/opt/ipa",
	})

	// Create the host config
	host_config := &container.HostConfig{
		Privileged:    true,
		RestartPolicy: container.RestartPolicy{Name: "on-failure", MaximumRetryCount: 3},
		PortBindings: nat.PortMap{
			nat.Port("4723"): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: appium_port,
				},
			},
			nat.Port(wda_port): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: wda_port,
				},
			},
			nat.Port(wda_mjpeg_port): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: wda_mjpeg_port,
				},
			},
			nat.Port(container_server_port): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: container_server_port,
				},
			},
		},
		Mounts: mounts,
	}

	// Create a folder for logging for the container
	err = os.MkdirAll("./logs/container_"+device_name+"-"+device_udid, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create logs folder when attempting to create a container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, config, host_config, nil, nil, "iosDevice_"+device_udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create a container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	// Start the container
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not start container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Successfully created a container for iOS device with udid: " + device_udid)
}

// Create an Android container for a specific device(by UDID) using data from config.json so if device is not registered there it will not attempt to create a container for it
// If container already exists for this device it will do nothing
func CreateAndroidContainer(device_udid string) {
	log.WithFields(log.Fields{
		"event": "android_container_create",
	}).Info("Attempting to create a container for Android device with udid: " + device_udid)

	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not unmarshal config.json file when trying to create a container for device with udid: " + device_udid)
		return
	}

	// Check if device is registered in config data
	var device_in_config bool
	var deviceConfig DeviceConfig
	for _, v := range configData.DeviceConfig {
		if v.DeviceUDID == device_udid {
			device_in_config = true
			deviceConfig = v
		}
	}

	// Stop execution if device not in config data
	if !device_in_config {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
		return
	}

	// Get the device config data
	appium_port := strconv.Itoa(deviceConfig.AppiumPort)
	device_name := deviceConfig.DeviceName
	device_os_version := deviceConfig.DeviceOSVersion
	stream_port := strconv.Itoa(deviceConfig.StreamPort)
	selenium_hub_port := configData.AppiumConfig.SeleniumHubPort
	selenium_hub_host := configData.AppiumConfig.SeleniumHubHost
	devices_host := configData.AppiumConfig.DevicesHost
	hub_protocol := configData.AppiumConfig.SeleniumHubProtocolType
	container_server_port := strconv.Itoa(deviceConfig.ContainerServerPort)

	// Create the docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + device_udid)
		return
	}

	// Create the container config
	config := &container.Config{
		Image: "android-appium",
		ExposedPorts: nat.PortSet{
			nat.Port("4723"):                struct{}{},
			nat.Port("4724"):                struct{}{},
			nat.Port(container_server_port): struct{}{},
		},
		Env: []string{"ON_GRID=" + on_grid,
			"APPIUM_PORT=" + appium_port,
			"DEVICE_UDID=" + device_udid,
			"DEVICE_OS_VERSION=" + device_os_version,
			"DEVICE_NAME=" + device_name,
			"SELENIUM_HUB_PORT=" + selenium_hub_port,
			"SELENIUM_HUB_HOST=" + selenium_hub_host,
			"DEVICES_HOST=" + devices_host,
			"HUB_PROTOCOL=" + hub_protocol,
			"CONTAINER_SERVER_PORT" + container_server_port,
			"DEVICE_OS=android"},
	}

	// Create the host config
	host_config := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "on-failure", MaximumRetryCount: 3},
		PortBindings: nat.PortMap{
			nat.Port("4724"): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: stream_port,
				},
			},
			nat.Port("4723"): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: appium_port,
				},
			},
			nat.Port(container_server_port): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: container_server_port,
				},
			},
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: project_dir + "/logs/container_" + device_name + "-" + device_udid,
				Target: "/opt/logs",
			},
			{
				Type:   mount.TypeBind,
				Source: project_dir + "/apps",
				Target: "/opt/ipa",
			},
			{
				Type:   mount.TypeBind,
				Source: "/home/shamanec/.android",
				Target: "/root/.android",
			},
			{
				Type:   mount.TypeBind,
				Source: project_dir + "/minicap",
				Target: "/root/minicap",
			},
		},
		Resources: container.Resources{
			Devices: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/device_" + device_udid,
					PathInContainer:   "/dev/bus/usb/003/011",
					CgroupPermissions: "rwm",
				},
			},
		},
	}

	// Create a folder for logging for the container
	err = os.MkdirAll("./logs/container_"+device_name+"-"+device_udid, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create logs folder when attempting to create a container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, config, host_config, nil, nil, "androidDevice_"+device_udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create a container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	// Start the container
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not start container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "android_container_create",
	}).Info("Successfully created a container for Android device with udid: " + device_udid)
}

// Check if container exists by name and also return container_id
func checkContainerExistsByName(container_name string) (bool, string, string) {
	// Get all the containers
	containers, _ := getContainersList()
	container_exists := false
	container_id := ""
	container_status := ""

	// Loop through the available containers
	// If a container with the provided name exists
	// return true and also return the container ID and status
	for _, container := range containers {
		containerName := strings.Replace(container.Names[0], "/", "", -1)
		if strings.Contains(containerName, container_name) {
			container_exists = true
			container_id = container.ID
			container_status = container.Status
		}
	}
	return container_exists, container_id, container_status
}

// Get list of containers on host
func getContainersList() ([]types.Container, error) {
	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_list",
		}).Error(". Error: " + err.Error())
		return nil, errors.New("Could not create docker client")
	}

	// Get the list of containers
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_list",
		}).Error(". Error: " + err.Error())
		return nil, errors.New("Could not get container list")
	}
	return containers, nil
}

// Remove any docker container by container ID
func removeContainerByID(container_id string) {
	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Attempting to remove container with ID: " + container_id)

	// Create a new context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not create docker client while attempting to remove container with ID: " + container_id + ". Error: " + err.Error())
		return
	}

	// Stop the container by the provided container ID
	if err := cli.ContainerStop(ctx, container_id, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + container_id + ". Error: " + err.Error())
		return
	}

	// Remove the stopped container
	if err := cli.ContainerRemove(ctx, container_id, types.ContainerRemoveOptions{}); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + container_id + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Successfully removed container with ID: " + container_id)
}

// Restart a docker container by provided container ID
func RestartContainerInternal(container_id string) error {
	// Create a new context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_restart",
		}).Error("Could not create docker client while attempting to restart container with ID: " + container_id + ". Error: " + err.Error())
		return err
	}

	// Try to restart the container
	if err := cli.ContainerRestart(ctx, container_id, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_restart",
		}).Error("Could not restart container with ID: " + container_id + ". Error: " + err.Error())
		return err
	}

	log.WithFields(log.Fields{
		"event": "docker_container_restart",
	}).Info("Successfully attempted to restart container with ID: " + container_id)

	return nil
}

// Generate the data for device containers table in the UI
func deviceContainerRows() ([]ContainerRow, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	// Get the current containers list
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var rows []ContainerRow

	// Loop through the containers list
	for _, container := range containers {
		// Parse plain container name
		containerName := strings.Replace(container.Names[0], "/", "", -1)

		// Get all the container ports from the returned array into string
		containerPorts := ""
		for i, s := range container.Ports {
			if i > 0 {
				containerPorts += "\n"
			}
			containerPorts += "{" + s.IP + ", " + strconv.Itoa(int(s.PrivatePort)) + ", " + strconv.Itoa(int(s.PublicPort)) + ", " + s.Type + "}"
		}

		// Extract the device UDID from the container name
		re := regexp.MustCompile("[^_]*$")
		match := re.FindStringSubmatch(containerName)

		// Create a table row data and append it to the slice
		var containerRow = ContainerRow{ContainerID: container.ID, ImageName: container.Image, ContainerStatus: container.Status, ContainerPorts: containerPorts, ContainerName: containerName, DeviceUDID: match[0]}
		rows = append(rows, containerRow)
	}
	return rows, nil
}
