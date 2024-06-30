package router

import (
	"GADS/provider/logger"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"GADS/provider/devices"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

func AndroidStreamProxy(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		logger.ProviderLogger.LogError("AndroidStreamProxy", fmt.Sprintf("Failed upgrading http to ws for device `%s` - %s", device.UDID, err))
		return
	}
	defer conn.Close()

	u := url.URL{Scheme: "ws", Host: "localhost:" + device.StreamPort, Path: ""}
	destConn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		logger.ProviderLogger.LogError("AndroidStreamProxy", fmt.Sprintf("Failed connecting to device `%s` stream port - %s", device.UDID, err))
		return
	}
	defer destConn.Close()

	// Read messages(jpegs) from the device streaming websocket server
	// And send them to the provider websocket client
	for {
		data, code, err := wsutil.ReadServerData(destConn)
		if err != nil {
			logger.ProviderLogger.LogError("AndroidStreamProxy", fmt.Sprintf("Failed reading data from device `%s` ws conn - %s", device.UDID, err))
			return
		}

		err = wsutil.WriteServerMessage(conn, code, data)
		if err != nil {
			logger.ProviderLogger.LogError("AndroidStreamProxy", fmt.Sprintf("Failed writing data to provider ws connection for device `%s` - %s", device.UDID, err))
			return
		}
	}
}

func AndroidStreamMJPEG(c *gin.Context) {
	c.Header("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	c.Writer.WriteHeader(http.StatusOK)
	c.Deadline()

	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	u := url.URL{Scheme: "ws", Host: "localhost:" + device.StreamPort, Path: ""}
	conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		logger.ProviderLogger.LogError("AndroidStreamProxy", fmt.Sprintf("Failed connecting to device `%s` stream port - %s", device.UDID, err))
		return
	}
	defer conn.Close()

	// Read messages(jpegs) from the device streaming websocket server
	// And send them to the provider websocket client
	for {
		data, _, err := wsutil.ReadServerData(conn)
		if err != nil {
			logger.ProviderLogger.LogError("AndroidStreamProxy", fmt.Sprintf("Failed reading data from device `%s` ws conn - %s", device.UDID, err))
			return
		}

		// Write the boundary and content type for each frame
		_, err = c.Writer.Write([]byte("\r\n--frame\r\nContent-Type: image/jpeg\r\n\r\n"))
		if err != nil {
			break
		}

		// Write the image to the response
		_, err = c.Writer.Write(data)
		if err != nil {
			break
		}

		// Flush the response writer to ensure the client receives the frame immediately
		c.Writer.Flush()
	}
}

func findJPEGMarkers(data []byte) (int, int) {
	start := bytes.Index(data, []byte{0xFF, 0xD8})
	end := bytes.Index(data, []byte{0xFF, 0xD9})
	return start, end
}

func IOSStreamMJPEG(c *gin.Context) {
	// Set the necessary headers for MJPEG streaming
	// Note: The "boundary" is arbitrary but must be unique and consistent.
	c.Header("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	c.Writer.WriteHeader(http.StatusOK)
	c.Deadline()

	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	// Read data from device
	server := "localhost:" + device.StreamPort
	// Connect to the server
	conn, err := net.Dial("tcp", server)
	if err != nil {
		os.Exit(1)
	}
	defer conn.Close()

	var buffer []byte
	for {

		// Read data from the connection
		tempBuf := make([]byte, 1024)
		n, err := conn.Read(tempBuf)
		if err != nil {
			if err != io.EOF {
				return
			}
			break
		}

		// Append the read bytes to the buffer
		buffer = append(buffer, tempBuf[:n]...)

		// Check if buffer has a complete JPEG image
		start, end := findJPEGMarkers(buffer)
		if start >= 0 && end > start {
			// Process the JPEG image
			jpegImage := buffer[start : end+2] // Include end marker
			// Keep any remaining data in the buffer for the next image
			buffer = buffer[end+2:]

			// Write the boundary and content type for each frame
			_, err = c.Writer.Write([]byte("\r\n--frame\r\nContent-Type: image/jpeg\r\n\r\n"))
			if err != nil {
				break
			}

			// Write the image to the response
			_, err = c.Writer.Write(jpegImage)
			if err != nil {
				break
			}

			// Flush the response writer to ensure the client receives the frame immediately
			c.Writer.Flush()
		}
	}
}

func IOSStreamMJPEGWda(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	// Set the necessary headers for MJPEG streaming
	// Note: The "boundary" is arbitrary but must be unique and consistent.
	c.Header("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	c.Writer.WriteHeader(http.StatusOK)
	c.Deadline()

	streamUrl := "http://localhost:" + device.WDAStreamPort

	req, err := http.NewRequest("GET", streamUrl, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer resp.Body.Close()

	// Get the media type and params after connecting to WebDriverAgent stream
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		fmt.Println("Error getting request mediaType and params:", err)
		return
	}

	// Get the boundary string
	// It has leading slashes -- that need to be removed for it to work properly
	boundary := strings.Replace(params["boundary"], "--", "", -1)

	if strings.HasPrefix(mediaType, "multipart/") {
		// Create a multipart reader from the response using the cleaned boundary
		mr := multipart.NewReader(resp.Body, boundary)

		// Loop and for each part in the multpart reader read the image and send it over the ws
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			jpegImage, err := io.ReadAll(part)
			if err != nil {
				break
			}

			// Write the boundary and content type for each frame
			_, err = c.Writer.Write([]byte("\r\n--frame\r\nContent-Type: image/jpeg\r\n\r\n"))
			if err != nil {
				break
			}

			// Write the image to the response
			_, err = c.Writer.Write(jpegImage)
			if err != nil {
				break
			}

			// Flush the response writer to ensure the client receives the frame immediately
			c.Writer.Flush()
		}
	}
}

func IosStreamProxyGADS(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]
	jpegChannel := make(chan []byte, 15)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create the new conn
	wsConn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		logger.ProviderLogger.LogError("ios_stream", fmt.Sprintf("Failed to upgrade http conn to ws when starting streaming for device `%s` - %s", udid, err))
		return
	}

	// Read data from device
	server := "localhost:" + device.StreamPort
	// Connect to the server
	conn, err := net.Dial("tcp", server)
	if err != nil {
		fmt.Println("Error connecting:", err.Error())
		os.Exit(1)
	}

	defer func() {
		err := wsConn.Close()
		if err != nil {
			logger.ProviderLogger.LogError("ios_stream", fmt.Sprintf("Failed to close websocket connection when finishing streaming for device `%s` - %s", udid, err))
		}
		err = conn.Close()
		if err != nil {
			logger.ProviderLogger.LogError("ios_stream", fmt.Sprintf("Failed to close broadcast TCP connection when finishing streaming for device `%s` - %s", udid, err))
		}
		close(jpegChannel)
	}()

	// Get data from the jpeg channel and send it over the ws
	// The channel will act as a buffer for slower consumer because this could crash the broadcast app
	// Or at least I assume this is the problem in long distance connections
	go func() {
		for {
			select {
			case jpegImage := <-jpegChannel:
				// Send the jpeg over the websocket
				err = wsutil.WriteServerBinary(wsConn, jpegImage)
				if err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	var buffer []byte
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Read data from the connection
			tempBuf := make([]byte, 1024)
			n, err := conn.Read(tempBuf)
			if err != nil {
				if err != io.EOF {
					return
				}
				break
			}

			// Append the read bytes to the buffer
			buffer = append(buffer, tempBuf[:n]...)

			// Check if buffer has a complete JPEG image
			start, end := findJPEGMarkers(buffer)
			if start >= 0 && end > start {
				// Process the JPEG image
				jpegImage := buffer[start : end+2] // Include end marker
				// Keep any remaining data in the buffer for the next image
				buffer = buffer[end+2:]
				// Send the jpeg to the channel
				jpegChannel <- jpegImage
			}
		}
	}
}

func IosStreamProxyWDA(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DBDeviceMap[udid]

	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		fmt.Println(err)
	}
	defer conn.Close()

	streamUrl := "http://localhost:" + device.WDAStreamPort

	req, err := http.NewRequest("GET", streamUrl, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer resp.Body.Close()

	// Get the media type and params after connecting to WebDriverAgent stream
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		fmt.Println("Error getting request mediaType and params:", err)
		return
	}

	// Get the boundary string
	// It has leading slashes -- that need to be removed for it to work properly
	boundary := strings.Replace(params["boundary"], "--", "", -1)

	// Should be multipart/x-mixed-replace
	// We know it's that one but check just in case
	if strings.HasPrefix(mediaType, "multipart/") {
		// Create a multipart reader from the response using the cleaned boundary
		mr := multipart.NewReader(resp.Body, boundary)

		// Loop and for each part in the multpart reader read the image and send it over the ws
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			jpg, err := io.ReadAll(part)
			if err != nil {
				break
			}
			wsutil.WriteServerBinary(conn, jpg)
		}
	}
}
