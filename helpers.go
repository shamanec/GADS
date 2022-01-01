package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/danielpaulus/go-ios/ios"
)

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
		err := AddToArchive(tw, file)
		if err != nil {
			return err
		}
	}

	return nil
}

// Add files to the tar writer
func AddToArchive(tw *tar.Writer, filename string) error {
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
		fmt.Printf("File at path: '" + filePath + "' doesn't exist\n")
		return
	} else {
		err = os.Remove(string(filePath))
		if err != nil {
			panic("Could not delete file at: " + string(filePath) + ". " + err.Error())
		}
	}
}

func CopyFileShell(currentFilePath string, newFilePath string, sudoPassword string) error {
	commandString := "echo '" + sudoPassword + "' | sudo -S cp " + currentFilePath + " " + newFilePath
	cmd := exec.Command("bash", "-c", commandString)
	err := cmd.Run()
	if err != nil {
		return errors.New("Could not copy file: " + err.Error() + "\n")
	}
	return nil
}

func DeleteFileShell(filePath string, sudoPassword string) error {
	commandString := "echo '" + sudoPassword + "' | sudo -S rm " + filePath
	cmd := exec.Command("bash", "-c", commandString)
	err := cmd.Run()
	if err != nil {
		return errors.New("Could not delete file: " + err.Error() + "\n")
	}
	return nil
}

func SetFilePermissionsShell(filePath string, permissionsCode string, sudoPassword string) error {
	commandString := "echo '" + sudoPassword + "' | sudo -S chmod " + permissionsCode + " " + filePath
	cmd := exec.Command("bash", "-c", commandString)
	err := cmd.Run()
	if err != nil {
		return errors.New("Could not set " + permissionsCode + " permissions to file at path: " + filePath + "\n" + err.Error())
	}
	return nil
}

// Device struct which contains device info
type ErrorJSON struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

func JSONError(w http.ResponseWriter, error_code string, error_string string, code int) {
	var errorMessage = ErrorJSON{
		ErrorCode:    error_code,
		ErrorMessage: error_string}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorMessage)
}

func ReadJSONFile(jsonFilePath string) ([]byte, error) {
	// Open the env.json
	jsonFile, err := os.Open(jsonFilePath)

	// if os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	} else {
		return byteValue, nil
	}
}

func CheckIOSDevicesListNotEmpty() bool {
	deviceList, _ := ios.ListDevices()
	if len(deviceList.DeviceList) > 0 {
		return true
	}
	return false
}

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
