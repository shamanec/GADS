package router

import (
	"GADS/device"
	"GADS/util"
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/gin-gonic/gin"
)

// This is a proxy handler for device interaction endpoints
func DeviceProxyHandler(c *gin.Context) {
	// Not really sure its needed anymore now that the stream comes over ws, but I'll keep it just in case
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic: %v. \nThis happens when closing device screen stream and I need to handle it \n", r)
		}
	}()
	path := c.Param("path")

	// Create a new ReverseProxy instance that will forward the requests
	// Update its scheme, host and path in the Director
	// Limit the number of open connections for the host
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			udid := c.Param("udid")
			req.URL.Scheme = "http"
			req.URL.Host = device.GetDeviceByUDID(udid).HostAddress + ":10001"
			req.URL.Path = "/device/" + udid + path
		},
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			DisableCompression:  true,
		},
		ModifyResponse: func(resp *http.Response) error {
			for headerName, _ := range resp.Header {
				if headerName == "Access-Control-Allow-Origin" {
					resp.Header.Del(headerName)
				}
			}

			return nil
		},
	}

	// Forward the request which in this case accepts the Gin ResponseWriter and Request objects
	proxy.ServeHTTP(c.Writer, c.Request)
}

func ProviderProxyHandler(c *gin.Context) {
	path := c.Param("path")
	name := c.Param("name")
	providerAddress := ""

	providers := util.GetProvidersFromDB()
	for _, provider := range providers {
		if provider.Nickname == name {
			providerAddress = provider.HostAddress
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
			req.URL.Host = providerAddress + ":10001"
			req.URL.Path = path
		},
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			DisableCompression:  true,
		},
		ModifyResponse: func(resp *http.Response) error {
			for headerName, _ := range resp.Header {
				if headerName == "Access-Control-Allow-Origin" {
					resp.Header.Del(headerName)
				}
			}

			return nil
		},
	}

	// Forward the request which in this case accepts the Gin ResponseWriter and Request objects
	proxy.ServeHTTP(c.Writer, c.Request)
}
