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
	"github.com/docker/docker/client"
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
