package device

import (
	"encoding/json"
	"io"
	"time"

	"github.com/gin-gonic/gin"
)

func AvailableDevicesSSE(c *gin.Context) {
	c.Stream(func(w io.Writer) bool {
		for _, device := range latestDevices {

			if device.Connected && device.LastUpdatedTimestamp >= (time.Now().UnixMilli()-5000) {
				device.Available = true
				if device.InUseLastTS <= (time.Now().UnixMilli() - 5000) {
					device.InUse = false
				} else {
					device.InUse = true
				}
				continue
			}
			device.InUse = false
			device.Available = false
		}

		jsonData, _ := json.Marshal(&latestDevices)
		c.SSEvent("", string(jsonData))
		c.Writer.Flush()
		time.Sleep(1 * time.Second)
		return true
	})
}
