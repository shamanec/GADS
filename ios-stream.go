package main

import (
	"bytes"
	"fmt"
	"image"
	jpeg2 "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/screenshotr"
	"github.com/pixiv/go-libjpeg/jpeg"
)

var maxFPS = 60
var iOSImageChan = make(chan image.Image, 1)
var screenshotsChan = make(chan []byte)
var lastImageBytes []byte
var lastImage image.Image

type iOSStreamHandler struct {
	Next    func() (image.Image, error)
	Options *jpeg.EncoderOptions
}

func StreamIOS(w http.ResponseWriter, r *http.Request) {
	device, err := ios.GetDevice("00008030000418C136FB802E")
	if err != nil {
		panic(err.Error())
	}

	go streamScreenshot(device)
	w.Header().Add("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	boundary := "\r\n--frame\r\nContent-Type: image/jpeg\r\n\r\n"
	stream := iOSStreamHandler{
		Next: func() (image.Image, error) {
			return <-iOSImageChan, nil
		},
		Options: &jpeg.EncoderOptions{Quality: 50, OptimizeCoding: false},
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
	frameRate := 30
	timeInterval := (1.0 / float64(frameRate)) * 1000000000

	timeStarted := time.Now()
	//scheduleNextScreenshot(device, timeInterval, timeStarted)
	//for {
	// go createJPEG(takeScreenshotToBytes(device))
	go createJPEG(device)
	//fmt.Printf("Current time after goroutine execution be %v\n", time.Now().UnixNano())
	//s := fmt.Sprintf("%.2f", timeInterval)
	// fmt.Printf("Currently in stream screenshot sending %v as time interval and %v as time started\n", s, timeStarted)
	//fmt.Printf("Currently in stream screenshot sending %v as time interval and %v as time started\n", s, timeStarted)
	//fmt.Printf("Current time just before schedule %v\n", time.Now().UnixNano())
	scheduleNextScreenshot(timeInterval, timeStarted, device)
	//}

}

func scheduleNextScreenshot(timeInterval float64, timeStarted time.Time, device ios.DeviceEntry) {
	//fmt.Printf("Current time for time elapsed calculation will be %v\n", time.Now().UnixNano())
	//timeElapsed := time.Now().Sub(timeStarted)
	//koleo := timeElapsed.Nanoseconds()
	//fmt.Printf("This is time elapsed after calculation %v\n", koleo)
	//nextTickDelta := timeInterval - float64(koleo)
	//fmt.Printf("This is the next tick delta %v\n", nextTickDelta)
	//s := fmt.Sprintf("%.2f", nextTickDelta)
	//fmt.Println(s)

	time.Sleep(100 * time.Millisecond)
	go streamScreenshot(device)

	// if nextTickDelta > 0 {
	// 	time.Sleep(100 * time.Nanosecond)
	// 	fmt.Println("inside next tick delta")
	// 	//time.AfterFunc(time.Duration(nextTickDelta)*time.Nanosecond, f)
	// 	go streamScreenshot(screenshotrService)
	// } else {
	// 	fmt.Println("outside next tick delta")
	// 	go streamScreenshot(screenshotrService)
	// }
}

func takeScreenshotToBytes(device ios.DeviceEntry) []byte {
	test, err := screenshotr.New(device)
	if err != nil {
		panic(err.Error())
	}
	imageBytes, err := test.TakeScreenshot()
	if err != nil {
		test.Close()
		fmt.Println("Error on take screenshot")
		return []byte{}
	} else {
		test.Close()
		//fmt.Println("actually setting bytes")
		return imageBytes
	}

}

func createJPEG(device ios.DeviceEntry) {
	//fmt.Println("Goroutine start time is: " + time.Now().String())
	imageBytes := takeScreenshotToBytes(device)
	jpgDecode(pngDecode(imageBytes))
}

func pngDecode(imageBytes []byte) image.Image {
	res := bytes.Compare(lastImageBytes, imageBytes)
	if res == 0 {
		//fmt.Println("returning last image")
		return lastImage
	}
	im, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		//fmt.Println("NOT A PNG FILE")
		return nil
	}
	return im
}

func jpgDecode(im image.Image) {
	buf := new(bytes.Buffer)

	err := jpeg2.Encode(buf, im, nil)
	if err != nil {
		//fmt.Println("Couldn't encode jpeg")
		fmt.Println(err.Error())
	}

	finalImage, err := jpeg.Decode(buf, &jpeg.DecoderOptions{})
	if err != nil {
		fmt.Println("Couldn't decode jpeg")
	}
	//fmt.Println("New image")
	lastImage = finalImage
	//fmt.Println("Returned last frame time: " + strconv.Itoa(int(time.Now().UnixMilli())))
	//fmt.Println("Goroutine last frame time is: " + time.Now().String())
	fmt.Println("Sending screenshot at: " + time.Now().String())
	iOSImageChan <- finalImage
}
