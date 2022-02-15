package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

//=================//
//=====STRUCTS=====//

type ErrorJSON struct {
	EventName    string `json:"event"`
	ErrorMessage string `json:"error_message"`
}

type SimpleResponseJSON struct {
	Message string `json:"message"`
}

//=======================//
//=====API FUNCTIONS=====//

// Write to a ResponseWriter an event and message with a response code
func JSONError(w http.ResponseWriter, event string, error_string string, code int) {
	var errorMessage = ErrorJSON{
		EventName:    event,
		ErrorMessage: error_string}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorMessage)
}

// Write to a ResponseWriter an event and message with a response code
func SimpleJSONResponse(w http.ResponseWriter, response_message string, code int) {
	var message = SimpleResponseJSON{
		Message: response_message,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(message)
}

// @Summary      Upload WDA
// @Description  Uploads the provided *.ipa into the ./apps folder with the expected "WebDriverAgent.ipa" name
// @Tags         configuration
// @Produce      json
// @Success      200 {object} SimpleResponseJSON
// @Failure      500 {object} ErrorJSON
// @Router       /configuration/upload-wda [post]
func UploadWDA(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer file.Close()

	// Create the ipa folder if it doesn't
	// already exist
	err = os.MkdirAll("./apps", os.ModePerm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a new file in the uploads directory
	dst, err := os.Create("./apps/WebDriverAgent.ipa")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer dst.Close()

	// Copy the uploaded file to the filesystem
	// at the specified destination
	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Uploaded and saved as WebDriverAgent.ipa in the './apps' folder.")
}

func UploadApp(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer file.Close()

	// Create the ipa folder if it doesn't
	// already exist
	err = os.MkdirAll("./apps", os.ModePerm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a new file in the uploads directory
	dst, err := os.Create("./apps/" + header.Filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer dst.Close()

	// Copy the uploaded file to the filesystem
	// at the specified destination
	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Uploaded '"+header.Filename+"' to the ./apps folder.")
}

//=======================//
//=====FUNCTIONS=====//

// Create a tar archive from an array of files while preserving directory structure
func CreateArchive(files []string, buf io.Writer) error {
	// Create new Writers for gzip and tar
	// These writers are chained. Writing to the tar writer will
	// write to the gzip writer which in turn will write to
	// the "buf" writer
	gw := gzip.NewWriter(buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Iterate over files and add them to the tar archive
	for _, file := range files {
		err := addToArchive(tw, file)
		if err != nil {
			return err
		}
	}

	return nil
}

// Add files to the tar writer
func addToArchive(tw *tar.Writer, filename string) error {
	// Open the file which will be written into the archive
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	// Use full path as name (FileInfoHeader only takes the basename)
	// If we don't do this the directory strucuture would
	// not be preserved
	// https://golang.org/src/archive/tar/common.go?#L626
	header.Name = filename

	// Write file header to the tar archive
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	// Copy file content to tar archive
	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}

	return nil
}

// Delete file by path
func DeleteFile(filePath string) {
	_, err := os.Stat(filePath)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "delete_file",
		}).Error("Could not find file:" + filePath + ". Error:" + err.Error())
		fmt.Printf("Could not find file:'" + filePath)
		return
	} else {
		err = os.Remove(string(filePath))
		if err != nil {
			log.WithFields(log.Fields{
				"event": "delete_file",
			}).Error("Could not delete file:" + filePath + ". Error:" + err.Error())
			fmt.Printf("Could not delete file:'" + filePath)
		}
	}
}

// Copy file using shell, needed when copying to a protected folder. Needs `sudo_password` set in env.json
func CopyFileShell(currentFilePath string, newFilePath string, sudoPassword string) error {
	commandString := "echo '" + sudoPassword + "' | sudo -S cp " + currentFilePath + " " + newFilePath
	cmd := exec.Command("bash", "-c", commandString)
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "delete_file_shell",
		}).Error("Could not copy file:" + currentFilePath + " to:" + newFilePath + ". Error:" + err.Error())
		return errors.New("Could not copy file:" + currentFilePath + " with shell.")
	}
	return nil
}

// Delete file using shell, needed when deleting from a protected folder. Needs `sudo_password` set in env.json
func DeleteFileShell(filePath string, sudoPassword string) error {
	commandString := "echo '" + sudoPassword + "' | sudo -S rm " + filePath
	cmd := exec.Command("bash", "-c", commandString)
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "delete_file_shell",
		}).Error("Could not delete file:" + filePath + " with shell. Error:" + err.Error())
		return errors.New("Could not delete file: " + filePath + "with shell")
	}
	return nil
}

// Set file permissions using shell. Needs `sudo_password` set in env.json
func SetFilePermissionsShell(filePath string, permissionsCode string, sudoPassword string) error {
	commandString := "echo '" + sudoPassword + "' | sudo -S chmod " + permissionsCode + " " + filePath
	cmd := exec.Command("bash", "-c", commandString)
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "file_permissions_shell",
		}).Error("Could not set permissions on file:" + filePath + " with shell. Error:" + err.Error())
		return errors.New("Could not set permissions on file:" + filePath + " with shell.")
	}
	return nil
}

// Enable the usbmuxd.service after updating it in /lib/systemd/system
func EnableUsbmuxdService() error {
	commandString := "sudo systemctl enable usbmuxd.service"
	cmd := exec.Command("bash", "-c", commandString)
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "enabled_usbmuxd_service",
		}).Error("Could not enable usbmuxd service. Error: " + err.Error())
		return errors.New("Could not enable usbmuxd service.")
	}
	return nil
}

// Read a json file from a provided path into a byte slice
func ReadJSONFile(jsonFilePath string) ([]byte, error) {
	jsonFile, err := os.Open(jsonFilePath)

	if err != nil {
		log.WithFields(log.Fields{
			"event": "read_json_file",
		}).Error("Could not open json file at path: " + jsonFilePath + ", error: " + err.Error())
		fmt.Println(err)
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "read_json_file",
		}).Error("Could not read json file at path: " + jsonFilePath + ", error: " + err.Error())
		return nil, err
	} else {
		return byteValue, nil
	}
}

// Check if an iOS device is registered in config.json by provided UDID
func CheckIOSDeviceInDevicesList(device_udid string) bool {
	deviceList, _ := ios.ListDevices()
	for start := time.Now(); time.Since(start) < 5*time.Second; {
		for _, device := range deviceList.DeviceList {
			if device.Properties.SerialNumber == device_udid {
				return true
			}
		}
	}
	return false
}

// Convert interface into JSON string
func ConvertToJSONString(data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return string(b)
}

// Prettify JSON with indentation and stuff
func PrettifyJSON(data string) string {
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, []byte(data), "", "  ")
	return prettyJSON.String()
}

func GetStringInBetween(str string, start string, end string) (result string) {
	s := strings.Index(str, start)
	if s == -1 {
		return
	}
	s += len(start)
	e := strings.Index(str[s:], end)
	if e == -1 {
		return
	}
	return str[s : s+e]
}
