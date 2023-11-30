package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

//=================//
//=====STRUCTS=====//

type JsonErrorResponse struct {
	EventName    string `json:"event"`
	ErrorMessage string `json:"error_message"`
}

type JsonResponse struct {
	Message string `json:"message"`
}

type ConfigJsonData struct {
	GadsHostAddress      string         `json:"gads_host_address"`
	GadsPort             string         `json:"gads_port"`
	MongoDB              string         `json:"mongo_db"`
	Providers            []ProviderData `json:"providers"`
	SeleniumGridInstance string         `json:"selenium_grid_instance"`
	AdminUsername        string         `json:"admin_username"`
	AdminPassword        string         `json:"admin_password"`
}

var ConfigData *ConfigJsonData

//=======================//
//=====API FUNCTIONS=====//

// Write to a ResponseWriter an event and message with a response code
func JSONError(w http.ResponseWriter, event string, error_string string, code int) {
	var errorMessage = JsonErrorResponse{
		EventName:    event,
		ErrorMessage: error_string}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorMessage)
}

// Write to a ResponseWriter an event and message with a response code
func SimpleJSONResponse(w http.ResponseWriter, response_message string, code int) {
	var message = JsonResponse{
		Message: response_message,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(message)
}

// Upload application to the /apps folder to make available for Appium
func UploadApp(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer file.Close()

	// Create the apps folder if it doesn't
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

// Convert interface into JSON string
func ConvertToJSONString(data interface{}) string {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return string(b)
}

// Unmarshal provided JSON string into a struct
func UnmarshalJSONString(jsonString string, v interface{}) error {
	bs := []byte(jsonString)

	err := json.Unmarshal(bs, v)
	if err != nil {
		return err
	}

	return nil
}

// Get a ConfigJsonData pointer with the current configuration from config.json
func GetConfigJsonData() {
	jsonFile, err := os.Open("./config.json")
	if err != nil {
		panic(err)
	}
	defer jsonFile.Close()

	bs, err := io.ReadAll(jsonFile)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(bs, &ConfigData)
	if err != nil {
		panic(err)
	}
}
