package main

import (
	"GADS/util"
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

//=======================//
//=====API FUNCTIONS=====//

// Load the general logs page
func GetLogsPage(w http.ResponseWriter, r *http.Request) {
	var logs_page = template.Must(template.ParseFiles("static/project_logs.html"))
	if err := logs_page.Execute(w, util.ConfigData); err != nil {
		log.WithFields(log.Fields{
			"event": "project_logs_page",
		}).Error("Couldn't load project_logs.html: " + err.Error())
		return
	}
}

// @Summary      Get project logs
// @Description  Provides project logs as plain text response
// @Tags         project-logs
// @Produces	 text
// @Success      200
// @Failure      200
// @Router       /project-logs [get]
func GetLogs(w http.ResponseWriter, r *http.Request) {
	// Create the command string to read the last 1000 lines of project.log
	commandString := "tail -n 1000 ./gads-project.log"

	// Create the command
	cmd := exec.Command("bash", "-c", commandString)

	// Create a buffer for the output
	var out bytes.Buffer

	// Pipe the Stdout of the command to the buffer pointer
	cmd.Stdout = &out

	// Execute the command
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_project_logs",
		}).Error("Attempted to get project logs but no logs available.")

		// Reply with generic message on error
		fmt.Fprintf(w, "No logs available")
		return
	}

	if out.String() == "" {
		fmt.Fprintf(w, "No logs available")
	} else {
		// Reply with the read logs lines
		fmt.Fprintf(w, out.String())
	}
}
