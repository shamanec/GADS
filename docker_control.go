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
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

var project_dir, _ = os.Getwd()
var on_grid = GetEnvValue("connect_selenium_grid")

//=======================//
//=====API FUNCTIONS=====//

// @Summary      Build docker images
// @Description  Starts building a docker image in a goroutine and just returns Accepted
// @Tags         configuration
// @Success      202
// @Router       /configuration/build-image/{image_type} [post]
func BuildDockerImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	image_type := vars["image_type"]
	go buildDockerImage(image_type)
	w.WriteHeader(http.StatusAccepted)
}

// @Summary      Remove 'ios-appium' image
// @Description  Removes the 'ios-appium' Docker image
// @Tags         configuration
// @Produce      json
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /configuration/remove-image [post]
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
	SimpleJSONResponse(w, "Successfully removed image tagged: '"+imageRemoveResponse[0].Untagged+"'", 200)
}

// @Summary      Restart container
// @Description  Restarts container by provided container ID
// @Tags         containers
// @Produce      json
// @Param        container_id path string true "Container ID"
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /containers/{container_id}/restart [post]
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
	SimpleJSONResponse(w, "Successfully attempted to restart container with ID: "+key, 200)
}

// @Summary      Get container logs
// @Description  Get logs of container by providing container ID
// @Tags         containers
// @Produce      json
// @Param        container_id path string true "Container ID"
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /containers/{container_id}/logs [get]
func GetContainerLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["container_id"]

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
		SimpleJSONResponse(w, newStr, 200)
	} else {
		SimpleJSONResponse(w, "There are no actual logs for this container.", 200)
	}
}

// Load the initial page with the project configuration info
func getAndroidContainers(w http.ResponseWriter, r *http.Request) {
	var index = template.Must(template.ParseFiles("static/android_containers.html"))
	if err := index.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// @Summary      Create container for device
// @Description  Creates a container for a connected registered device
// @Tags         device-containers
// @Param        device_udid path string true "Device UDID"
// @Success      202
// @Router       /device-containers/{device_udid}/create [post]
func CreateDeviceContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device_udid := vars["device_udid"]
	requestBody, _ := ioutil.ReadAll(r.Body)
	os_type := gjson.Get(string(requestBody), "os_type").Str
	if os_type == "Android" {
		go createAndroidContainer(device_udid)
	} else if os_type == "iOS" {
		go CreateIOSContainer(device_udid)
	}
	w.WriteHeader(http.StatusAccepted)
}

// @Summary      Remove container for device
// @Description  Removes a running container for a disconnected registered device by device UDID
// @Tags         device-containers
// @Param        device_udid path string true "Device UDID"
// @Success      202
// @Router       /device-containers/{device_udid}/remove [post]
func RemoveDeviceContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	container_exists, container_id := checkContainerExistsByName(vars["device_udid"])
	if container_exists {
		go removeContainerByID(container_id)
	}
	w.WriteHeader(http.StatusAccepted)
}

// @Summary      Remove container
// @Description  Removes container by provided container ID
// @Tags         containers
// @Produce      json
// @Param        container_id path string true "Container ID"
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /containers/{container_id}/remove [post]
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
	SimpleJSONResponse(w, "Successfully removed container with ID: "+key, 200)
}

// IOS Containers html page
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

	var rows []ContainerRow

	// Loop through the containers list
	for _, container := range containers {
		// Parse plain container name
		containerName := strings.Replace(container.Names[0], "/", "", -1)

		if strings.Contains(containerName, "ios_device") {
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

			var containerRow = ContainerRow{ContainerID: container.ID, ImageName: container.Image, ContainerStatus: container.Status, ContainerPorts: containerPorts, ContainerName: containerName, DeviceUDID: match[0]}
			rows = append(rows, containerRow)
		}
	}
	// Parse the template and return response with the container table rows
	var index = template.Must(template.ParseFiles("static/ios_containers.html"))
	if err := index.Execute(w, rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Android Containers html page
func GetAndroidContainers(w http.ResponseWriter, r *http.Request) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	// Get the current containers list
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	var rows []ContainerRow

	// Loop through the containers list
	for _, container := range containers {
		// Parse plain container name
		containerName := strings.Replace(container.Names[0], "/", "", -1)

		if strings.Contains(containerName, "android_device") {
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

			var containerRow = ContainerRow{ContainerID: container.ID, ImageName: container.Image, ContainerStatus: container.Status, ContainerPorts: containerPorts, ContainerName: containerName, DeviceUDID: match[0]}
			rows = append(rows, containerRow)
		}
	}
	// Parse the template and return response with the container table rows
	var index = template.Must(template.ParseFiles("static/android_containers.html"))
	if err := index.Execute(w, rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

//===================//
//=====FUNCTIONS=====//

func buildDockerImage(image_type string) {
	log.WithFields(log.Fields{
		"event": "docker_image_build",
	}).Info("Started building the '" + image_type + "' docker image.")

	DeleteFileShell("./build-context.tar", sudo_password)

	var files []string
	var image_name string
	var docker_file_name string

	// Create a tar to be used as build-context for the image build
	// The tar should include all files needed by the Dockerfile to successfully create the image
	if image_type == "ios-appium" {
		docker_file_name = "Dockerfile-iOS"
		files = []string{docker_file_name, "configs/nodeconfiggen.sh", "configs/ios-sync.sh", "apps/WebDriverAgent.ipa", "configs/supervision.p12"}
		image_name = "ios-appium"
	} else if image_type == "android-appium" {
		docker_file_name = "Dockerfile-Android"
		files = []string{docker_file_name, "configs/nodeconfiggen-android.sh", "configs/android-sync.sh"}
		image_name = "android-appium"
	} else {
		return
	}

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
	imageBuildResponse, err := cli.ImageBuild(ctx, buildContextTarReader, types.ImageBuildOptions{Dockerfile: docker_file_name, Remove: true, Tags: []string{image_name}})
	if err != nil {
		// Get the image build logs on failure
		buf.ReadFrom(imageBuildResponse.Body)
		// log.WithFields(log.Fields{
		// 	"event": "docker_image_build",
		// }).Error("Could not build docker image. Error: " + err.Error() + "\n" + buf.String())
		log.WithFields(log.Fields{
			"event": "docker_image_build",
		}).Error("Could not build docker image. Error: " + err.Error())
		return
	}
	defer imageBuildResponse.Body.Close()
	buf.ReadFrom(imageBuildResponse.Body)
	log.WithFields(log.Fields{
		"event": "docker_image_build",
	}).Info("Built '" + image_name + "' docker image:\n" + buf.String())
}

// Check if the 'ios-appium' image exists and return info string
func ImageExists() (imageStatus string) {
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
	time.Sleep(5 * time.Second)
	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Attempting to create a container for iOS device with udid: " + device_udid)

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

	appium_port := gjson.Get(string(byteValue), `ios-devices-list.#(device_udid="`+device_udid+`").appium_port`)
	if appium_port.Raw == "" {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
		return
	}
	device_name := gjson.Get(string(byteValue), `ios-devices-list.#(device_udid="`+device_udid+`").device_name`)
	device_os_version := gjson.Get(string(byteValue), `ios-devices-list.#(device_udid="`+device_udid+`").device_os_version`)
	wda_mjpeg_port := gjson.Get(string(byteValue), `ios-devices-list.#(device_udid="`+device_udid+`").wda_mjpeg_port`)
	wda_port := gjson.Get(string(byteValue), `ios-devices-list.#(device_udid="`+device_udid+`").wda_port`)
	wda_bundle_id := gjson.Get(string(byteValue), "wda_bundle_id")
	selenium_hub_port := gjson.Get(string(byteValue), "selenium_hub_port")
	selenium_hub_host := gjson.Get(string(byteValue), "selenium_hub_host")
	devices_host := gjson.Get(string(byteValue), "devices_host")
	hub_protocol := gjson.Get(string(byteValue), "hub_protocol")

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

	config := &container.Config{
		Image: "ios-appium",
		ExposedPorts: nat.PortSet{
			nat.Port(appium_port.Raw):    struct{}{},
			nat.Port(wda_port.Raw):       struct{}{},
			nat.Port(wda_mjpeg_port.Raw): struct{}{},
		},
		Env: []string{"ON_GRID=" + on_grid,
			"DEVICE_UDID=" + device_udid,
			"WDA_PORT=" + wda_port.Raw,
			"MJPEG_PORT=" + wda_mjpeg_port.Raw,
			"APPIUM_PORT=" + appium_port.Raw,
			"DEVICE_OS_VERSION=" + device_os_version.Str,
			"DEVICE_NAME=" + device_name.Str,
			"WDA_BUNDLEID=" + wda_bundle_id.Str,
			"SUPERVISION_PASSWORD=" + GetEnvValue("supervision_password"),
			"SELENIUM_HUB_PORT=" + selenium_hub_port.Str,
			"SELENIUM_HUB_HOST=" + selenium_hub_host.Str,
			"DEVICES_HOST=" + devices_host.Str,
			"HUB_PROTOCOL=" + hub_protocol.Str},
	}

	host_config := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "on-failure", MaximumRetryCount: 3},
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
				Source: project_dir + "/apps",
				Target: "/opt/ipa",
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

	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Successfully created a container for iOS device with udid: " + device_udid)
}

// Create an Android container for a specific device(by UDID) using data from config.json so if device is not registered there it will not attempt to create a container for it
// If container already exists for this device it will do nothing
func createAndroidContainer(device_udid string) {
	log.WithFields(log.Fields{
		"event": "android_container_create",
	}).Info("Attempting to create a container for Android device with udid: " + device_udid)

	container_exists, container_id := checkContainerExistsByName(device_udid)
	if container_exists {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Info("Container with ID:" + container_id + " already exists for device with udid:" + device_udid)
	} else {
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

		appium_port := gjson.Get(string(byteValue), `android-devices-list.#(device_udid="`+device_udid+`").appium_port`)
		if appium_port.Raw == "" {
			log.WithFields(log.Fields{
				"event": "ios_container_create",
			}).Error("Device with UDID:" + device_udid + " is not registered in the './configs/config.json' file. No container will be created.")
			return
		}
		device_name := gjson.Get(string(byteValue), `android-devices-list.#(device_udid="`+device_udid+`").device_name`)
		device_os_version := gjson.Get(string(byteValue), `android-devices-list.#(device_udid="`+device_udid+`").device_os_version`)
		stream_port := gjson.Get(string(byteValue), `android-devices-list.#(device_udid="`+device_udid+`").stream_port`)
		selenium_hub_port := gjson.Get(string(byteValue), "selenium_hub_port")
		selenium_hub_host := gjson.Get(string(byteValue), "selenium_hub_host")
		devices_host := gjson.Get(string(byteValue), "devices_host")
		hub_protocol := gjson.Get(string(byteValue), "hub_protocol")

		ctx := context.Background()
		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "ios_container_create",
			}).Error("Could not create docker client when attempting to create a container for device with udid: " + device_udid)
			return
		}

		config := &container.Config{
			Image: "android-appium",
			ExposedPorts: nat.PortSet{
				nat.Port(appium_port.Raw): struct{}{},
				nat.Port(stream_port.Raw): struct{}{},
			},
			Env: []string{"ON_GRID=" + on_grid,
				"DEVICE_UDID=" + device_udid,
				"APPIUM_PORT=" + appium_port.Raw,
				"DEVICE_OS_VERSION=" + device_os_version.Str,
				"DEVICE_NAME=" + device_name.Str,
				"SELENIUM_HUB_PORT=" + selenium_hub_port.Str,
				"SELENIUM_HUB_HOST=" + selenium_hub_host.Str,
				"DEVICES_HOST=" + devices_host.Str,
				"HUB_PROTOCOL=" + hub_protocol.Str,
				"STREAM_PORT=" + stream_port.Raw},
		}

		log.WithFields(log.Fields{
			"event": "test",
		}).Info("Will try to mount:/dev/device-" + device_name.Str + "-" + device_udid)

		host_config := &container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: "on-failure", MaximumRetryCount: 3},
			PortBindings: nat.PortMap{
				nat.Port(stream_port.Raw): []nat.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: stream_port.Raw,
					},
				},
				nat.Port(appium_port.Raw): []nat.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: appium_port.Raw,
					},
				},
			},
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: project_dir + "/logs/container_" + device_name.Str + "-" + device_udid,
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
						PathOnHost:        "/dev/device-" + device_name.Str + "-" + device_udid,
						PathInContainer:   "/dev/bus/usb/003/011",
						CgroupPermissions: "rwm",
					},
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

		resp, err := cli.ContainerCreate(ctx, config, host_config, nil, nil, "android_device_"+device_name.Str+"-"+device_udid)
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

		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Info("Successfully created a container for iOS device with udid: " + device_udid)
	}
}

// Check if container exists by name and also return container_id
func checkContainerExistsByName(container_name string) (bool, string) {
	containers, _ := getContainersList()
	container_exists := false
	container_id := ""
	for _, container := range containers {
		containerName := strings.Replace(container.Names[0], "/", "", -1)
		if strings.Contains(containerName, container_name) {
			container_exists = true
			container_id = container.ID
		}
	}
	return container_exists, container_id
}

// Get list of containers on host
func getContainersList() ([]types.Container, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_list",
		}).Error(". Error: " + err.Error())
		return nil, errors.New("Could not create docker client")
	}

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
