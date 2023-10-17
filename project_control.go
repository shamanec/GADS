package main

import (
	"GADS/util"
	"bytes"
	"html/template"
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

//=======================//
//=====API FUNCTIONS=====//

func GetLogsPage(c *gin.Context) {
	var logs_page = template.Must(template.ParseFiles("static/project_logs.html"))
	util.ConfigData.Providers = util.GetProvidersFromDB()

	err := logs_page.Execute(c.Writer, util.ConfigData)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "project_logs_page",
		}).Error("Couldn't load project_logs.html: " + err.Error())
		c.String(http.StatusInternalServerError, err.Error())
	}
}

func GetLogs(c *gin.Context) {
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
		c.String(http.StatusOK, "No logs available")
		return
	}

	if out.String() == "" {
		c.String(http.StatusOK, "No logs available")
	} else {
		c.String(http.StatusOK, out.String())
	}
}
