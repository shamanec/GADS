package auth

import (
	"GADS/util"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func LoginHandler(c *gin.Context) {
	username := c.Query("username")
	password := c.Query("password")

	user, err := util.GetUserFromDB(username)
	if err != nil {
		c.String(http.StatusUnauthorized, "Invalid username or password!")
		return
	}
	if user.Password != password {
		c.String(http.StatusUnauthorized, "Invalid username or password!")
		return
	}

	session := sessions.Default(c)

	session.Set(user.ID, "admin")
	if err := session.Save(); err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create session - %s", err))
		return
	}
	c.String(http.StatusOK, "Successful authentication!")
}

func LogoutHandler(c *gin.Context) {
	dbUser, err := util.GetUserFromDB("admin")
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Login unsuccessful - %s", err))
	}

	session := sessions.Default(c)
	user := session.Get(dbUser.ID)
	if user == nil {
		c.String(http.StatusBadRequest, "Invalid session token")
		return
	}
	session.Delete(dbUser.ID)
	if err := session.Save(); err != nil {
		c.String(http.StatusInternalServerError, "Failed to save session")
		return
	}
	c.String(http.StatusOK, "Successfully logged out!")
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if !strings.Contains(path, "appium") {
			dbUser, err := util.GetUserFromDB("admin")
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Unauthorized - %s", err))
			}

			session := sessions.Default(c)
			user := session.Get(dbUser.ID)
			if user == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
		}

		c.Next()
	}
}
