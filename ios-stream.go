package main

import (
	"bytes"
	"fmt"
	"image"
	jpeg2 "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/screenshotr"
	"github.com/pixiv/go-libjpeg/jpeg"
)

var iOSImageChan = make(chan image.Image, 1)
var initialImageChan = make(chan streamImageData, 1)
var lastImageBytes []byte
var lastImage image.Image

type iOSStreamHandler struct {
	Next    func() (image.Image, error)
	Options *jpeg.EncoderOptions
}

type streamImageData struct {
	Image     image.Image
	StartTime int64
	EndTime   int64
}

var lastStartTime int64
var lastEndTime int64

var ch = make(chan int, 5)

func StreamIOS(w http.ResponseWriter, r *http.Request) {
	device, err := ios.GetDevice("00008030000418C136FB802E")
	if err != nil {
		panic(err.Error())
	}

	go streamScreenshot(device)
	go sendScreenshotCheck()
	w.Header().Add("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	boundary := "\r\n--frame\r\nContent-Type: image/jpeg\r\n\r\n"
	stream := iOSStreamHandler{
		Next: func() (image.Image, error) {
			return <-iOSImageChan, nil
		},
		Options: &jpeg.EncoderOptions{Quality: 100, OptimizeCoding: false},
	}
	for {
		// get handler new image from imageChan
		img, err := stream.Next()
		if err != nil {
			return
		}

		n, err := io.WriteString(w, boundary)
		if err != nil || n != len(boundary) {
			return
		}

		err = jpeg.Encode(w, img, stream.Options)
		if err != nil {
			return
		}

		n, err = io.WriteString(w, "\r\n")
		if err != nil || n != 2 {
			return
		}
	}
}

func streamScreenshot(device ios.DeviceEntry) {
	for {
		ch <- 1
		go completeFunction(device)
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("Number of routines %v\n", runtime.NumGoroutine())
		fmt.Println(len(ch))
	}
}

func completeFunction(device ios.DeviceEntry) {
	startTime := time.Now().UnixMilli()
	screenshotrConn, err := screenshotr.New(device)
	if err != nil {
		panic(err.Error())
	}

	imageBytes, err := screenshotrConn.TakeScreenshot()
	if err != nil {
		screenshotrConn.Close()
		return
	}
	screenshotrConn.Close()

	var im image.Image

	im, err = png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		fmt.Println("failed on png decode")
		return
	}

	buf := new(bytes.Buffer)

	err = jpeg2.Encode(buf, im, nil)
	if err != nil {
		fmt.Println(err.Error())
	}

	finalImage, err := jpeg.Decode(buf, &jpeg.DecoderOptions{})
	if err != nil {
		fmt.Println("Couldn't decode jpeg")
	}
	endTime := time.Now().UnixMilli()
	imageStuff := streamImageData{Image: finalImage, StartTime: startTime, EndTime: endTime}
	initialImageChan <- imageStuff
	<-ch
}

func scheduleNextScreenshot(timeInterval float64, timeStarted time.Time, device ios.DeviceEntry) {
	time.Sleep(100 * time.Millisecond)
	go streamScreenshot(device)
}

func takeScreenshotToBytes(device ios.DeviceEntry) []byte {
	test, err := screenshotr.New(device)
	if err != nil {
		fmt.Println("could not connect to screenshtor service")
	}
	imageBytes, err := test.TakeScreenshot()
	if err != nil {
		test.Close()
		fmt.Println("Error on take screenshot")
		return []byte{}
	} else {
		test.Close()
		return imageBytes
	}

}

func sendScreenshotCheck() {
	for {
		koleo := <-initialImageChan
		if koleo.StartTime > lastStartTime && koleo.EndTime > lastEndTime {
			lastStartTime = koleo.StartTime
			lastEndTime = koleo.StartTime
			iOSImageChan <- koleo.Image
		}
	}

}

func createJPEG(device ios.DeviceEntry) {
	imageBytes := takeScreenshotToBytes(device)
	jpgDecode(pngDecode(imageBytes))
}

func pngDecode(imageBytes []byte) image.Image {
	res := bytes.Compare(lastImageBytes, imageBytes)
	if res == 0 {
		return lastImage
	}
	im, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil
	}
	return im
}

func jpgDecode(im image.Image) {
	buf := new(bytes.Buffer)

	err := jpeg2.Encode(buf, im, nil)
	if err != nil {
		fmt.Println(err.Error())
	}

	finalImage, err := jpeg.Decode(buf, &jpeg.DecoderOptions{})
	if err != nil {
		fmt.Println("Couldn't decode jpeg")
	}
	lastImage = finalImage
	iOSImageChan <- finalImage
}
