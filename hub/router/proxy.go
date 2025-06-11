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
	"time"

	"github.com/gin-gonic/gin"
)

var proxyTransport = &http.Transport{
	MaxIdleConnsPerHost: 10,
	DisableCompression:  true,
	IdleConnTimeout:     60 * time.Second,
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

	var username string

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
			}
		}
	}

	devices.HubDevicesData.Mu.Lock()
	device, ok := devices.HubDevicesData.Devices[udid]

	// Verify if the device is already in use by another user
	if ok && device != nil && device.InUseBy != "" && device.InUseBy != "automation" &&
		(time.Now().UnixMilli()-device.InUseTS) < 3000 &&
		(device.InUseBy != username) {

		devices.HubDevicesData.Mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "This device is already linked to another user with an active session"})
		return
	}

	devices.HubDevicesData.Mu.Unlock()

	if !ok || device == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with UDID `%s` not found or is nil", udid)})
		return
	}

	// Check if device is available before proceeding with proxy operations
	devices.HubDevicesData.Mu.Lock()
	isAvailable := devices.HubDevicesData.Devices[udid].Available
	devices.HubDevicesData.Mu.Unlock()

	if !isAvailable {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": fmt.Sprintf("Device with UDID `%s` is not available", udid),
		})
		return
	}

	// Create a new ReverseProxy instance that will forward the requests
	// Update its scheme, host and path in the Director
	// Limit the number of open connections for the host
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			devices.HubDevicesData.Mu.Lock()
			req.URL.Host = devices.HubDevicesData.Devices[udid].Device.Host
			devices.HubDevicesData.Mu.Unlock()
			req.URL.Path = "/device/" + udid + path
		},
		Transport: proxyTransport,
		ModifyResponse: func(resp *http.Response) error {
			for headerName, _ := range resp.Header {
				if headerName == "Access-Control-Allow-Origin" {
					resp.Header.Del(headerName)
				}
			}

			return nil
		},
	}

	// Set the last action performed timestamp through the proxy
	devices.HubDevicesData.Mu.Lock()
	devices.HubDevicesData.Devices[udid].LastActionTS = time.Now().UnixMilli()
	devices.HubDevicesData.Mu.Unlock()

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
			for headerName, _ := range resp.Header {
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
