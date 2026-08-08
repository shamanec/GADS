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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	if claims, err := auth.GetClaimsFromRequest(c); err == nil {
		username = claims.Username
		tenant = claims.Tenant
	}

	// Tenant-scope stored app installs before any device-state checks: only files
	// from the caller's tenant can be installed (admins bypass).
	if c.Request.Method == http.MethodPost && strings.HasSuffix(path, "/installStoredApp") {
		if err := authorizeStoredAppInstall(c); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
	}

	device, ok := devices.HubDeviceStore.Get(udid)
	if !ok || device == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with UDID `%s` not found or is nil", udid)})
		return
	}

	device.Mu.RLock()
	isAvailable := device.Available
	isLockedByOther := device.IsLockedByOther(username, tenant)
	device.Mu.RUnlock()

	if isLockedByOther {
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
			req.URL.Host = device.Host
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

	// Keep the lock alive and record the action timestamp
	device.Mu.Lock()
	device.LastActionTS = time.Now().UnixMilli()
	device.RefreshLock()
	device.Mu.Unlock()

	// Forward the request which in this case accepts the Gin ResponseWriter and Request objects
	proxy.ServeHTTP(c.Writer, c.Request)
}

// authorizeStoredAppInstall inspects the installStoredApp body (restoring it for the proxy)
// and verifies the referenced file belongs to the caller's tenant.
func authorizeStoredAppInstall(c *gin.Context) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body")
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var payload struct {
		FileID string `json:"file_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.FileID == "" {
		return fmt.Errorf("a file_id referencing an uploaded app is required")
	}

	file, err := db.GlobalMongoStore.GetFileByID(payload.FileID)
	if err != nil {
		return fmt.Errorf("no app found with id `%s`", payload.FileID)
	}

	claims, err := auth.GetClaimsFromRequest(c)
	if err != nil {
		return fmt.Errorf("unauthorized")
	}
	// Files without a tenant association (legacy uploads) are admin-only, matching GetApps.
	if claims.Role != "admin" && (file.Metadata.Tenant == "" || file.Metadata.Tenant != claims.Tenant) {
		return fmt.Errorf("you can only install apps uploaded by your tenant")
	}
	return nil
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
