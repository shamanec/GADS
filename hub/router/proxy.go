/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"GADS/common/db"
	"GADS/hub/auth"
	"GADS/hub/devices"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var proxyTransport = &http.Transport{
	MaxIdleConnsPerHost: 10,
	DisableCompression:  true,
	IdleConnTimeout:     60 * time.Second,
}

// Get capability prefix from environment variable, default to "gads"
var capabilityPrefix = getEnvOrDefault("GADS_CAPABILITY_PREFIX", "gads")

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// This is a proxy handler for device interaction endpoints
func DeviceProxyHandler(c *gin.Context) {
	// Not really sure its needed anymore now that the stream comes over ws, but I'll keep it just in case
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic: %v. \nThis happens when closing device screen stream and I need to handle it \n", r)
		}
	}()
	udid := c.Param("udid")
	path := c.Param("path")

	// Block legacy Appium endpoint and instruct to use /grid
	if strings.Contains(path, "/appium") {
		c.JSON(http.StatusGone, gin.H{
			"value": gin.H{
				"error":      "unknown method",
				"message":    "The legacy endpoint /device/{udid}/appium is deprecated. Please use /grid endpoint instead.",
				"stacktrace": "",
			},
		})
		return
	}

	var username string
	var tenant string

	// If not a session creation or no credentials in capabilities, check for bearer token
	authToken := c.GetHeader("Authorization")
	if authToken == "" {
		authToken = c.Query("token")
	}

	if authToken != "" {
		// Extract token from Bearer format
		tokenString, err := auth.ExtractTokenFromBearer(authToken)
		if err == nil {
			// Get origin from request
			origin := auth.GetOriginFromRequest(c)

			// Get claims from token with origin
			claims, err := auth.GetClaimsFromToken(tokenString, origin)
			if err == nil {
				username = claims.Username
				tenant = claims.Tenant
			}
		}
	}

	device, ok := devices.HubDeviceStore.Get(udid)
	if !ok || device == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with UDID `%s` not found or is nil", udid)})
		return
	}

	// Verify if the device is already in use by another user
	device.Mu.RLock()
	inUseBy := device.InUseBy
	inUseByTenant := device.InUseByTenant
	inUseTS := device.InUseTS
	isAvailable := device.Available
	device.Mu.RUnlock()

	if inUseBy != "" &&
		(time.Now().UnixMilli()-inUseTS) < 3000 &&
		(inUseBy != username || inUseByTenant != tenant) {
		c.JSON(http.StatusConflict, gin.H{"error": "This device is already linked to another user with an active session"})
		return
	}

	if !isAvailable {
		if c.Request.Method == "POST" && strings.HasSuffix(path, "/session") {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"value": gin.H{
					"error":      "invalid argument",
					"message":    fmt.Sprintf("Device `%s` is not available", udid),
					"stacktrace": "",
				},
			})
			return
		} else {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": fmt.Sprintf("Device `%s` is not available", udid),
			})
			return
		}
	}

	// Create a new ReverseProxy instance that will forward the requests
	// Update its scheme, host and path in the Director
	// Limit the number of open connections for the host
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			device.Mu.RLock()
			req.URL.Host = device.Device.Host
			device.Mu.RUnlock()
			req.URL.Path = "/device/" + udid + path
		},
		Transport: proxyTransport,
		ModifyResponse: func(resp *http.Response) error {
			for headerName := range resp.Header {
				if headerName == "Access-Control-Allow-Origin" {
					resp.Header.Del(headerName)
				}
			}

			return nil
		},
	}

	// Set the last action performed timestamp through the proxy
	device.Mu.Lock()
	device.LastActionTS = time.Now().UnixMilli()
	device.Mu.Unlock()

	// Forward the request which in this case accepts the Gin ResponseWriter and Request objects
	proxy.ServeHTTP(c.Writer, c.Request)
}

func ProviderProxyHandler(c *gin.Context) {
	path := c.Param("path")
	name := c.Param("name")
	providerAddress := ""

	providers, _ := db.GlobalMongoStore.GetAllProviders()
	for _, provider := range providers {
		if provider.Nickname == name {
			providerAddress = fmt.Sprintf("%s:%v", provider.HostAddress, provider.Port)
		}
	}

	if providerAddress == "" {
		c.JSON(http.StatusNotFound, fmt.Sprintf("Provider with name `%s` does not exist", name))
		return
	}

	// Create a new ReverseProxy instance that will forward the requests
	// Update its scheme, host and path in the Director
	// Limit the number of open connections for the host
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = providerAddress
			req.URL.Path = path
		},
		Transport: proxyTransport,
		ModifyResponse: func(resp *http.Response) error {
			for headerName := range resp.Header {
				if headerName == "Access-Control-Allow-Origin" {
					resp.Header.Del(headerName)
				}
			}

			return nil
		},
	}

	c.Writer.Flush()

	// Forward the request which in this case accepts the Gin ResponseWriter and Request objects
	proxy.ServeHTTP(c.Writer, c.Request)
}
