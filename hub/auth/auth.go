package auth

import (
	"GADS/common/db"
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

// var (
// sessionsMap = make(map[string]*Session)
// mapMutex = &sync.Mutex{}
// )

// type Session struct {
// 	User      models.User
// 	SessionID string
// 	ExpireAt  time.Time
// }

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

	user, err := db.GetUserFromDB(creds.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	if user.Password != creds.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	sessionID := uuid.New()
	// session := &Session{
	// 	User:      user,
	// 	SessionID: sessionID.String(),
	// 	ExpireAt:  time.Now().Add(time.Hour),
	// }

	// mapMutex.Lock()
	// sessionsMap[sessionID.String()] = session
	// mapMutex.Unlock()

	CreateSession(user, sessionID)

	c.JSON(http.StatusOK, gin.H{"sessionID": sessionID, "username": user.Username, "role": user.Role})
}

func LogoutHandler(c *gin.Context) {
	sessionID := c.GetHeader("X-Auth-Token")
	// mapMutex.Lock()
	// defer mapMutex.Unlock()
	if _, exists := GetSession(sessionID); exists {
		// delete(sessionsMap, sessionID)
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
			// mapMutex.Lock()
			if session, exists := GetSession(sessionID); exists {
				if session.ExpireAt.Before(time.Now()) {
					// delete(sessionsMap, sessionID)
					// mapMutex.Unlock()
					DeleteSession(sessionID)
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
					return
				}
				// Refresh the session expiry time
				session.ExpireAt = time.Now().Add(time.Hour)
				// mapMutex.Unlock()

				if strings.Contains(path, "admin") && session.User.Role != "admin" {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "you need admin privileges to access this endpoint"})
					return
				}
			} else {
				// mapMutex.Unlock()
				// If the session doesn't exist
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
		}

		c.Next()
	}
}
