package proxy

import (
	"GADS/device"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

func ProxyHandler(c *gin.Context) {
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

	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
