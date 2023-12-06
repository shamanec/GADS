package router

import (
	"GADS/util"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func GetInitialPage(c *gin.Context) {
	var index = template.Must(template.ParseFiles("static/index.html"))
	err := index.Execute(c.Writer, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Could not create the initial page html - %s", err.Error()))
	}
}

func GetSeleniumGridPage(c *gin.Context) {
	var index = template.Must(template.ParseFiles("static/selenium_grid.html"))
	err := index.Execute(c.Writer, util.ConfigData)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Could not create the selenium grid page html - %s", err.Error()))
	}
}

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
