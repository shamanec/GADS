package proxy

import (
	"GADS/device"
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

	// Create a new ReverseProxy instance that will forward the requests
	// Update its scheme, host and path in the Director
	// Limit the number of open connections for the host
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			udid := c.Param("udid")
			req.URL.Scheme = "http"
			req.URL.Host = device.GetDeviceByUDID(udid).Host + ":10001"
			req.URL.Path = "/device/" + udid + c.Param("path")
		},
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			DisableCompression:  true,
		},
	}

	// Forward the request which in this case accepts the Gin ResponseWriter and Request objects
	proxy.ServeHTTP(c.Writer, c.Request)
}
