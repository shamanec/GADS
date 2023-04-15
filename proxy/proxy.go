package proxy

import (
	"GADS/device"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

func DeviceProxyHandler(c *gin.Context) {
	udid := c.Param("udid")
	path := c.Param("path")
	device := device.GetDeviceByUDID(udid)

	// Replace this URL with your provider server's base URL
	providerBaseURL := "http://" + device.Host + ":10001"
	providerURL, err := url.Parse(providerBaseURL + "/device/" + udid + path)
	if err != nil {
		fmt.Println("Error 1")
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Forward the request to the provider server
	req, err := http.NewRequest(c.Request.Method, providerURL.String(), c.Request.Body)
	if err != nil {
		fmt.Println("Error 2")
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

	c.Status(resp.StatusCode)
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
