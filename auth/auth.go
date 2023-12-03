package auth

import (
	"GADS/models"
	"GADS/util"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthCreds struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var sessionsMap = make(map[string]*Session)

type Session struct {
	User      models.User
	SessionID string
	ExpireAt  time.Time
}

func LoginHandler(c *gin.Context) {
	var creds AuthCreds
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	err = json.Unmarshal(body, &creds)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "internal server error"})
		return
	}

	user, err := util.GetUserFromDB(creds.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "internal server error"})
		return
	}
	if user.Password != creds.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	sessionID := uuid.New()
	session := &Session{
		User:      user,
		SessionID: sessionID.String(),
		ExpireAt:  time.Now().Add(time.Hour),
	}
	sessionsMap[sessionID.String()] = session

	c.JSON(http.StatusOK, gin.H{"sessionID": sessionID})
}

func LogoutHandler(c *gin.Context) {
	sessionID := c.GetHeader("X-Auth-Token")
	if _, exists := sessionsMap[sessionID]; exists {
		delete(sessionsMap, sessionID)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "session does not exist"})
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		sessionID := c.GetHeader("X-Auth-Token")

		if !strings.Contains(path, "appium") && !strings.Contains(path, "stream") {
			if session, exists := sessionsMap[sessionID]; exists {
				if session.ExpireAt.Before(time.Now()) {
					delete(sessionsMap, sessionID)
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
					return
				}
				// Refresh the session expiry time
				session.ExpireAt = time.Now().Add(time.Hour)

				if strings.Contains(path, "admin") && session.User.Role != "admin" {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "you need admin privileges to access this endpoint"})
					return
				}
			} else {
				// If the session doesn't exist
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
		}

		c.Next()
	}
}
