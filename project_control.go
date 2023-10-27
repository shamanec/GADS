package main

import (
	"GADS/util"
	"html/template"
	"net/http"

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
