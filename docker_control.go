package main

import (
	"bytes"
	"context"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

var project_dir = GetEnvValue("project_dir")

func buildDockerImage() {
	//error_message := "Could not build 'ios-appium' image"
	log.WithFields(log.Fields{
		"event": "docker_image_build",
	}).Info("Attempting to build the 'ios-appium' docker image.")

	// Delete build-context.tar if it exists
	DeleteFile("./build-context.tar")

	// Create a tar to be used as build-context for the image build
	// The tar should include all files needed by the Dockerfile to successfully create the image
	files := []string{"Dockerfile", "configs/nodeconfiggen.sh", "configs/wdaSync.sh"}
	out, err := os.Create("build-context.tar")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_image_build",
		}).Error("Could not create build-context.tar archive file. Error: " + err.Error())
		return
	}
	defer out.Close()
	err = CreateArchive(files, out)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_image_build",
		}).Error("Could not create build-context.tar archive. Error: " + err.Error())
		return
	}

	// Create the context and Docker client
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

	// Build the Docker image using the tar reader
	buf := new(bytes.Buffer)
	imageBuildResponse, err := cli.ImageBuild(ctx, buildContextTarReader, types.ImageBuildOptions{Remove: true, Tags: []string{"ios-appium"}})
	if err != nil {
		// Get the image build logs on failure
		buf.ReadFrom(imageBuildResponse.Body)
		log.WithFields(log.Fields{
			"event": "docker_image_build",
		}).Error("Could not create build docker image. Error: " + err.Error() + "\n" + buf.String())
		return
	}
	defer imageBuildResponse.Body.Close()
	buf.ReadFrom(imageBuildResponse.Body)
	log.WithFields(log.Fields{
		"event": "docker_image_build",
	}).Info("Built 'ios-appium' docker image:\n")
}

func BuildDockerImage(w http.ResponseWriter, r *http.Request) {
	go buildDockerImage()
	w.WriteHeader(http.StatusAccepted)
	//buf.ReadFrom(imageBuildResponse.Body)
}

func RemoveDockerImage(w http.ResponseWriter, r *http.Request) {
	error_message := "Could not remove ios-appium image."
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

	imageRemoveResponse, err := cli.ImageRemove(ctx, "ios-appium", types.ImageRemoveOptions{PruneChildren: true})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_image_remove",
		}).Error("Could not remove ios-appium image. Error: " + err.Error())
		JSONError(w, "docker_image_remove", error_message, 500)
		return
	}
	SimpleJSONResponse(w, "docker_image_remove", "Successfully removed image tagged: '"+imageRemoveResponse[0].Untagged+"'", 200)
}

// Function that returns all current iOS device containers and their info
func GetIOSContainers(w http.ResponseWriter, r *http.Request) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	// Get the current containers list
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	// Define the rows that will be built for the struct used by the template for the table
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
		re := regexp.MustCompile("[^-]*$")
		match := re.FindStringSubmatch(containerName)

		// Create a struct object for the respective container using the parameters by the above split
		var containerRow = ContainerRow{ContainerID: container.ID, ImageName: container.Image, ContainerStatus: container.Status, ContainerPorts: containerPorts, ContainerName: containerName, DeviceUDID: match[0]}
		// Append each struct object to the rows that will be displayed in the table
		rows = append(rows, containerRow)
	}
	// Parse the template and return response with the container table rows
	var index = template.Must(template.ParseFiles("static/ios_containers.html"))
	if err := index.Execute(w, rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Restart docker container
func RestartContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["container_id"]

	log.WithFields(log.Fields{
		"event": "docker_container_restart",
	}).Info("Attempting to restart container with ID: " + key)

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_restart",
		}).Error("Could not create docker client while attempting to restart container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "docker_container_restart", "Could not restart container with ID: "+key, 500)
		return
	}

	if err := cli.ContainerRestart(ctx, key, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_restart",
		}).Error("Could not restart container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "docker_container_restart", "Could not restart container with ID: "+key, 500)
		return
	}
	log.WithFields(log.Fields{
		"event": "docker_container_restart",
	}).Info("Successfully attempted to restart container with ID: " + key)
	SimpleJSONResponse(w, "docker_container_restart", "Successfully attempted to restart container with ID: "+key, 200)
}

// Function that returns all current iOS device containers and their info
func GetContainerLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["container_id"]

	log.WithFields(log.Fields{
		"event": "get_container_logs",
	}).Info("Attempting to get logs for container with ID: " + key)

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_logs",
		}).Error("Could not create docker client while attempting to get logs for container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "get_container_logs", "Could not get logs for container with ID: "+key, 500)
		return
	}

	options := types.ContainerLogsOptions{ShowStdout: true}
	// Replace this ID with a container that really exists
	out, err := cli.ContainerLogs(ctx, key, options)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_logs",
		}).Error("Could not get logs for container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "get_container_logs", "Could not get logs for container with ID: "+key, 500)
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(out)
	newStr := buf.String()

	if newStr != "" {
		SimpleJSONResponse(w, "get_container_logs", newStr, 200)
	} else {
		SimpleJSONResponse(w, "get_container_logs", "There are no actual logs for this container.", 200)
	}
}

// Load the initial page with the project configuration info
func getAndroidContainers(w http.ResponseWriter, r *http.Request) {
	var index = template.Must(template.ParseFiles("static/android_containers.html"))
	if err := index.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Check if the ios-appium image exists and return info string
func ImageExists() (imageStatus string) {
	// Create the context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_image_status",
		}).Error("Could not create docker client while attempting to get image status. Error:" + err.Error())
		imageStatus = "Image status undefined"
		return
	}

	// Get the images list
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

func CreateIOSContainer(w http.ResponseWriter, r *http.Request) {
	// Get the parameters
	vars := mux.Vars(r)
	device_udid := vars["device_udid"]

	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Attempting to create a container for iOS device with udid: " + device_udid)

	error_message := "Could not create container for device with udid: " + device_udid

	if !CheckIOSDeviceInDevicesList(device_udid) {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Warn("Device with udid: " + device_udid + " is not available in the attached devices list from go-ios.")
		JSONError(w, "ios_container_create", "Device is not available in the attached devices list from go-ios.", 500)
		return
	}

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + device_udid)
		JSONError(w, "ios_container_create", error_message, 500)
		return
	}

	jsonFile, err := os.Open("./configs/config.json")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not open ./configs/config.json when attempting to create a container for device with udid: " + device_udid)
		JSONError(w, "ios_container_create", error_message, 500)
		return
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not read ./configs/config.json when attempting to create a container for device with udid: " + device_udid)
		JSONError(w, "ios_container_create", error_message, 500)
		return
	}
	appium_port := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").appium_port`)
	device_name := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").device_name`)
	device_os_version := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").device_os_version`)
	wda_mjpeg_port := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").wda_mjpeg_port`)
	wda_port := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").wda_port`)
	wda_bundle_id := gjson.Get(string(byteValue), "wda_bundle_id")
	selenium_hub_port := gjson.Get(string(byteValue), "selenium_hub_port")
	selenium_hub_host := gjson.Get(string(byteValue), "selenium_hub_host")
	devices_host := gjson.Get(string(byteValue), "devices_host")
	hub_protocol := gjson.Get(string(byteValue), "hub_protocol")

	config := &container.Config{
		Image: "ios-appium",
		ExposedPorts: nat.PortSet{
			nat.Port(appium_port.Raw):    struct{}{},
			nat.Port(wda_port.Raw):       struct{}{},
			nat.Port(wda_mjpeg_port.Raw): struct{}{},
		},
		Env: []string{"ON_GRID=false",
			"DEVICE_UDID=" + device_udid,
			"WDA_PORT=" + wda_port.Raw,
			"MJPEG_PORT=" + wda_mjpeg_port.Raw,
			"APPIUM_PORT=" + appium_port.Raw,
			"DEVICE_OS_VERSION=" + device_os_version.Str,
			"DEVICE_NAME=" + device_name.Str,
			"WDA_BUNDLEID=" + wda_bundle_id.Str,
			"SELENIUM_HUB_PORT=" + selenium_hub_port.Str,
			"SELENIUM_HUB_HOST=" + selenium_hub_host.Str,
			"DEVICES_HOST=" + devices_host.Str,
			"HUB_PROTOCOL=" + hub_protocol.Str},
	}

	host_config := &container.HostConfig{
		PortBindings: nat.PortMap{
			nat.Port(appium_port.Raw): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: appium_port.Raw,
				},
			},
			nat.Port(wda_port.Raw): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: wda_port.Raw,
				},
			},
			nat.Port(wda_mjpeg_port.Raw): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: wda_mjpeg_port.Raw,
				},
			},
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: "/var/run/usbmuxd",
				Target: "/var/run/usbmuxd",
			},
			{
				Type:   mount.TypeBind,
				Source: "/var/lib/lockdown",
				Target: "/var/lib/lockdown",
			},
			{
				Type:   mount.TypeBind,
				Source: project_dir + "/logs/container_" + device_name.Str + "-" + device_udid,
				Target: "/opt/logs",
			},
			{
				Type:   mount.TypeBind,
				Source: project_dir + "/ipa",
				Target: "/opt/ipa",
			},
			{
				Type:   mount.TypeBind,
				Source: project_dir + "/WebDriverAgent",
				Target: "/opt/WebDriverAgent",
			},
		},
	}

	err = os.MkdirAll("./logs/container_"+device_name.Str+"-"+device_udid, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create logs folder for container. Error: " + err.Error())
		JSONError(w, "ios_container_create", error_message, 500)
		return
	}

	resp, err := cli.ContainerCreate(ctx, config, host_config, nil, nil, "ios_device_"+device_name.Str+"-"+device_udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create container. Error: " + err.Error())
		JSONError(w, "ios_container_create", error_message, 500)
		return
	}

	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not start container. Error: " + err.Error())
		JSONError(w, "ios_container_create", error_message, 500)
		return
	}
}

func CreateIOSContainerLocal(device_udid string) {
	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Attempting to create a container for iOS device with udid: " + device_udid)

	if !CheckIOSDeviceInDevicesList(device_udid) {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Warn("Device with udid: " + device_udid + " is not available in the attached devices list from go-ios.")
		return
	}

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + device_udid)
		return
	}

	jsonFile, err := os.Open("./configs/config.json")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not open ./configs/config.json when attempting to create a container for device with udid: " + device_udid)
		return
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not read ./configs/config.json when attempting to create a container for device with udid: " + device_udid)
		return
	}
	appium_port := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").appium_port`)
	device_name := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").device_name`)
	device_os_version := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").device_os_version`)
	wda_mjpeg_port := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").wda_mjpeg_port`)
	wda_port := gjson.Get(string(byteValue), `devicesList.#(device_udid="`+device_udid+`").wda_port`)
	wda_bundle_id := gjson.Get(string(byteValue), "wda_bundle_id")

	config := &container.Config{
		Image: "ios-appium",
		ExposedPorts: nat.PortSet{
			nat.Port(appium_port.Raw):    struct{}{},
			nat.Port(wda_port.Raw):       struct{}{},
			nat.Port(wda_mjpeg_port.Raw): struct{}{},
		},
		Env: []string{"ON_GRID=false",
			"DEVICE_UDID=" + device_udid,
			"WDA_PORT=" + wda_port.Raw,
			"MJPEG_PORT=" + wda_mjpeg_port.Raw,
			"APPIUM_PORT=" + appium_port.Raw,
			"DEVICE_OS_VERSION=" + device_os_version.Str,
			"DEVICE_NAME=" + device_name.Str,
			"WDA_BUNDLEID=" + wda_bundle_id.Str},
	}

	host_config := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "always", MaximumRetryCount: 0},
		PortBindings: nat.PortMap{
			nat.Port(appium_port.Raw): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: appium_port.Raw,
				},
			},
			nat.Port(wda_port.Raw): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: wda_port.Raw,
				},
			},
			nat.Port(wda_mjpeg_port.Raw): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: wda_mjpeg_port.Raw,
				},
			},
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: "/var/run/usbmuxd",
				Target: "/var/run/usbmuxd",
			},
			{
				Type:   mount.TypeBind,
				Source: "/var/lib/lockdown",
				Target: "/var/lib/lockdown",
			},
			{
				Type:   mount.TypeBind,
				Source: project_dir + "/logs/container_" + device_name.Str + "-" + device_udid,
				Target: "/opt/logs",
			},
			{
				Type:   mount.TypeBind,
				Source: project_dir + "/ipa",
				Target: "/opt/ipa",
			},
			{
				Type:   mount.TypeBind,
				Source: project_dir + "/WebDriverAgent",
				Target: "/opt/WebDriverAgent",
			},
		},
	}

	err = os.MkdirAll("./logs/container_"+device_name.Str+"-"+device_udid, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create logs folder when attempting to create a container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	resp, err := cli.ContainerCreate(ctx, config, host_config, nil, nil, "ios_device_"+device_name.Str+"-"+device_udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create a container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not start container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}
}

func UpdateIOSContainersLocal() {
	time.Sleep(10 * time.Second)
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "update_containers",
		}).Error(". Error: " + err.Error())
		return
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "update_containers",
		}).Error(". Error: " + err.Error())
		return
	}

	devices, err := ios.ListDevices()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "update_containers",
		}).Error(". Error: " + err.Error())
		return
	}

	DestroyIOSContainers(devices, containers)
	CreateIOSContainers(devices, containers)

}

func CreateIOSContainers(devices ios.DeviceList, containers []types.Container) {
	device_has_container := false
	for _, device := range devices.DeviceList {
		for _, container := range containers {
			containerName := strings.Replace(container.Names[0], "/", "", -1)
			if strings.Contains(containerName, device.Properties.SerialNumber) {
				device_has_container = true
			}
		}
		if !device_has_container {
			CreateIOSContainerLocal(device.Properties.SerialNumber)
		}
	}
}

func DestroyIOSContainers(devices ios.DeviceList, containers []types.Container) {
	container_has_device := false
	for _, container := range containers {
		containerName := strings.Replace(container.Names[0], "/", "", -1)
		for _, device := range devices.DeviceList {
			if strings.Contains(containerName, device.Properties.SerialNumber) {
				container_has_device = true
			}
		}
		if !container_has_device {
			RemoveIOSContainerLocal(container.ID)
		}
	}
}

func UpdateIOSContainers(w http.ResponseWriter, r *http.Request) {
	go UpdateIOSContainersLocal()
	w.WriteHeader(http.StatusAccepted)
}

func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["container_id"]

	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Attempting to remove container with ID: " + key)

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not create docker client while attempting to remove container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "docker_container_remove", "Could not remove container with ID: "+key, 500)
		return
	}

	if err := cli.ContainerStop(ctx, key, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "docker_container_remove", "Could not remove container with ID: "+key, 500)
		return
	}

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
	SimpleJSONResponse(w, "docker_container_remove", "Successfully removed container with ID: "+key, 200)
}

func RemoveIOSContainerLocal(container_id string) {

	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Attempting to remove container with ID: " + container_id)

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not create docker client while attempting to remove container with ID: " + container_id + ". Error: " + err.Error())
		return
	}

	if err := cli.ContainerStop(ctx, container_id, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + container_id + ". Error: " + err.Error())
		return
	}

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
