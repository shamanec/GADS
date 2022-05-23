package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

func updateStreamSize(screen_size string) []string {
	// Set max height of 850 pixels for the stream element
	maxHeight := 850
	dimensions := strings.Split(screen_size, "x")
	x, err := strconv.Atoi(dimensions[0])
	y, err := strconv.Atoi(dimensions[1])

	if err != nil {
		return nil
	}

	stream_width := ""
	stream_height := ""

	if y < maxHeight {
		stream_width = dimensions[1] + "px"
		stream_height = dimensions[0] + "px"
	} else {
		device_ratio := x / y

		stream_height = strconv.Itoa(maxHeight) + "px"
		stream_width = strconv.Itoa((maxHeight * device_ratio)) + "px"
	}

	return []string{stream_height, stream_width}
}

func changeStream() {

}

func GetDeviceControlInfo2(w http.ResponseWriter, r *http.Request) {
	var rdata DeviceControlInfoDataRequest
	err := UnmarshalRequestBody(r.Body, &rdata)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_container_create",
		}).Error("Could not unmarshal request body")
		fmt.Fprintf(w, "error")
	}

	var ios_info IOSDeviceInfo
	var android_info AndroidDeviceInfo

	if rdata.DeviceType == "ios" {
		ios_info = getIOSDeviceInfo(rdata.DeviceUdid)
	} else if rdata.DeviceType == "android" {
		android_info = getAndroidDeviceInfo(rdata.DeviceUdid)
	} else {
		fmt.Fprintf(w, "error")
	}

	funcMap := template.FuncMap{
		"contains":   strings.Contains,
		"streamSize": updateStreamSize,
	}

	// Parse the template and return response with the container table rows
	var tmpl = template.Must(template.New("device_control").Funcs(funcMap).ParseFiles("static/device_control2.html"))
	if rdata.DeviceType == "ios" {
		if err := tmpl.ExecuteTemplate(w, "device_control", ios_info); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		if err := tmpl.ExecuteTemplate(w, "device_control", android_info); err != nil {
			fmt.Fprintf(w, "error")
		}
	}

}
