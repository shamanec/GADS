package main

import (
	"html/template"
	"net/http"
	"os"

	"GADS/db"
	"GADS/device"
	_ "GADS/docs"
	"GADS/proxy"
	"GADS/util"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	httpSwagger "github.com/swaggo/http-swagger"
)

var project_log_file *os.File

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

func setLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	project_log_file, err := os.OpenFile("./gads-project.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic(err)
	}
	log.SetOutput(project_log_file)
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

	myRouter.HandleFunc("/configuration/upload-app", util.UploadApp).Methods("POST")

	myRouter.HandleFunc("/devices", device.LoadDevices)
	myRouter.HandleFunc("/available-devices", device.AvailableDevicesWS)
	myRouter.HandleFunc("/devices/control/{device_udid}", device.GetDevicePage)

	// Logs
	myRouter.HandleFunc("/project-logs", GetLogs).Methods("GET")

	// Asset endpoints
	myRouter.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	myRouter.HandleFunc("/logs", GetLogsPage)
	myRouter.HandleFunc("/", GetInitialPage)

	myRouter.HandleFunc("/proxy/{udid}/{path:.*}", proxy.ProxyHandler)

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

func main() {
	util.GetConfigJsonData()

	db.NewConnection()
	go device.GetLatestDBDevices()
	go device.GetDevices()
	setLogging()
	handleRequests()
}
