package auth

import (
	"GADS/common/db"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type AuthCreds struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// GetOriginFromRequest extracts the origin from request headers
func GetOriginFromRequest(c *gin.Context) string {
	// Try to get from Origin header first (standard for CORS)
	origin := c.GetHeader("Origin")
	if origin != "" {
		return origin
	}

	// Try Referer header next
	referer := c.GetHeader("Referer")
	if referer != "" {
		return referer
	}

	// Try X-Origin custom header (might be set by proxies or clients)
	xorigin := c.GetHeader("X-Origin")
	if xorigin != "" {
		return xorigin
	}

	// Default to unknown origin
	return "unknown"
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

	user, err := db.GlobalMongoStore.GetUser(creds.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	if user.Password != creds.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Define scopes based on user permissions
	scopes := []string{"user"}
	if user.Role == "admin" {
		scopes = append(scopes, "admin")
	}

	// Get the user's tenant/workspace
	tenant := ""
	if len(user.WorkspaceIDs) > 0 {
		tenant = user.WorkspaceIDs[0]
	}

	// Get the request origin
	origin := GetOriginFromRequest(c)

	// Generate JWT token with 1 hour validity
	token, err := GenerateJWT(user.Username, user.Role, tenant, scopes, time.Hour, origin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Response in requested format
	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   3600, // 1 hour in seconds
		"username":     user.Username,
		"role":         user.Role,
	})
}

func LogoutHandler(c *gin.Context) {
	// Check if there's a bearer token
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		// For JWT tokens, we don't need to do anything on the server
		// The client should discard the token
		c.JSON(http.StatusOK, gin.H{"message": "success"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "session does not exist"})
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Bypass authentication for specific paths
		if strings.Contains(path, "appium") || strings.Contains(path, "stream") || strings.Contains(path, "ws") {
			c.Next()
			return
		}

		// Check JWT token in Authorization header
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString, err := ExtractTokenFromBearer(authHeader)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token format"})
				return
			}

			// Get the request origin
			origin := GetOriginFromRequest(c)

			// Validate JWT token with the origin
			claims, err := ValidateJWT(tokenString, origin)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
				return
			}

			// Check if token has expired
			if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
				return
			}

			// Check permissions (admin)
			if strings.Contains(path, "admin") && claims.Role != "admin" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "you need admin privileges to access this endpoint"})
				return
			}

			// Store user information in context for later use
			c.Set("username", claims.Username)
			c.Set("role", claims.Role)
			c.Set("tenant", claims.Tenant)
			c.Set("origin", claims.Origin) // Store origin in context

			// Continue execution
			c.Next()
			return
		}

		// If no valid bearer token is provided
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
}
