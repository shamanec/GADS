package proxy

import (
	"GADS/device"
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// This is a proxy handler for device interaction endpoints
func DeviceProxyHandler(c *gin.Context) {
	// Get the UDID of the device from the path
	udid := c.Param("udid")
	// Get the remaining path which can be any
	path := c.Param("path")

	// Get a Device pointer for the respective device
	device := device.GetDeviceByUDID(udid)

	// Generate the provider base url for the respective device
	providerBaseURL := "http://" + device.Host + ":10001"
	// Generate the actual endpoint that will accept the proxied request
	providerURL, err := url.Parse(providerBaseURL + "/device/" + udid + path)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error generating provider url for proxied endpoint: "+err.Error())
		return
	}

	// Forward the request to the provider server
	req, err := http.NewRequest(c.Request.Method, providerURL.String(), c.Request.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	req.Header = c.Request.Header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	// Copy the headers from the provider response to the handler response
	// Write the body of the provider response to the handler response
	copyHeaders(c.Writer.Header(), resp.Header)
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		return
	}
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
