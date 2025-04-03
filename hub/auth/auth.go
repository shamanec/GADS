package auth

import (
	"GADS/common/db"
	"context"
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

func LoginHandler(c *gin.Context) {
	var creds AuthCreds
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	err = json.Unmarshal(body, &creds)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Internal server error"})
		return
	}

	user, err := db.GlobalMongoStore.GetUser(context.Background(), creds.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	if user.Password != creds.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	sessionID := uuid.New()

	CreateSession(user, sessionID)

	c.JSON(http.StatusOK, gin.H{"sessionID": sessionID, "username": user.Username, "role": user.Role})
}

func LogoutHandler(c *gin.Context) {
	sessionID := c.GetHeader("X-Auth-Token")
	if _, exists := GetSession(sessionID); exists {
		DeleteSession(sessionID)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "session does not exist"})
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		sessionID := c.GetHeader("X-Auth-Token")

		if !strings.Contains(path, "appium") && !strings.Contains(path, "stream") && !strings.Contains(path, "ws") {
			if session, exists := GetSession(sessionID); exists {
				if session.ExpireAt.Before(time.Now()) {
					DeleteSession(sessionID)
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
