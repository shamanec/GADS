package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
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
	EventName    string `json:"event"`
	ErrorMessage string `json:"error_message"`
}

type SimpleResponseJSON struct {
	EventName string `json:"event"`
	Message   string `json:"message"`
}

func JSONError(w http.ResponseWriter, event string, error_string string, code int) {
	var errorMessage = ErrorJSON{
		EventName:    event,
		ErrorMessage: error_string}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorMessage)
}

func SimpleJSONResponse(w http.ResponseWriter, event string, response_message string, code int) {
	var message = SimpleResponseJSON{
		EventName: event,
		Message:   response_message,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(message)
}

func ReadJSONFile(jsonFilePath string) ([]byte, error) {
	// Open the env.json
	jsonFile, err := os.Open(jsonFilePath)

	// if os.Open returns an error then handle it
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

func UploadWDA(w http.ResponseWriter, r *http.Request) {
	// truncated for brevity

	// The argument to FormFile must match the name attribute
	// of the file input on the frontend
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer file.Close()

	if _, err := os.Stat("./WebDriverAgent"); !os.IsNotExist(err) {
		err = os.RemoveAll("./WebDriverAgent")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Create the WebDriverAgent folder if it doesn't
	// already exist
	err = os.MkdirAll("./WebDriverAgent", os.ModePerm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a new file in the uploads directory
	dst, err := os.Create("WDA.zip")
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

	_, err = Unzip("./WDA.zip", "WebDriverAgent")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	DeleteFile("./WDA.zip")
	fmt.Fprintf(w, "Uploaded and unzipped into 'WebDriverAgent' folder.")
}

func Unzip(src string, dest string) ([]string, error) {

	var filenames []string

	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}
	defer r.Close()

	for _, f := range r.File {

		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip. More Info: http://bit.ly/2MsjAWE
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return filenames, fmt.Errorf("%s: illegal file path", fpath)
		}

		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return filenames, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return filenames, err
		}

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}

		_, err = io.Copy(outFile, rc)

		// Close the file without defer to close before next iteration of loop
		outFile.Close()
		rc.Close()

		if err != nil {
			return filenames, err
		}
	}
	return filenames, nil
}

func UploadIPA(w http.ResponseWriter, r *http.Request) {
	// truncated for brevity

	// The argument to FormFile must match the name attribute
	// of the file input on the frontend
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer file.Close()

	if _, err := os.Stat("./WebDriverAgent"); !os.IsNotExist(err) {
		err = os.RemoveAll("./WebDriverAgent")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Create the WebDriverAgent folder if it doesn't
	// already exist
	err = os.MkdirAll("./WebDriverAgent", os.ModePerm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a new file in the uploads directory
	dst, err := os.Create("WDA.zip")
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

	_, err = Unzip("./WDA.zip", "WebDriverAgent")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Uploaded and unzipped into 'WebDriverAgent' folder.")
}

func ConvertToJSONString(data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return string(b)
}
