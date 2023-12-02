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

	session.Set("userID", user.ID)
	session.Set("role", user.Role)
	if err := session.Save(); err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create session - %s", err))
		return
	}
	c.String(http.StatusOK, "Successful authentication!")
}

func LogoutHandler(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("userID")
	if userID == nil {
		c.String(http.StatusBadRequest, "Invalid session token")
		return
	}
	session.Delete("userID")
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
			session := sessions.Default(c)
			user := session.Get("userID")
			if user == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}

			if strings.Contains(path, "admin") {
				role := session.Get("role").(string)
				if role != "admin" {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "you need admin privileges"})
					return
				}
				return
			}
		}

		c.Next()
	}
}
