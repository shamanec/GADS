package router

import (
	"GADS/provider/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"net/http/pprof"
)

func HandleRequests() *gin.Engine {
	// Start sending live provider data
	// to connected clients
	go sendProviderLiveData()

	r := gin.Default()
	rConfig := cors.DefaultConfig()
	rConfig.AllowAllOrigins = true
	rConfig.AllowHeaders = []string{"X-Auth-Token", "Content-Type"}
	r.Use(cors.New(rConfig))

	r.GET("/info", GetProviderData)
	r.GET("/info-ws", GetProviderDataWS)
	r.GET("/devices", DevicesInfo)
	r.POST("/uploadFile", UploadFile)

	pprofGroup := r.Group("/debug/pprof")
	{
		pprofGroup.GET("/", gin.WrapF(pprof.Index))
		pprofGroup.GET("/cmdline", gin.WrapF(pprof.Cmdline))
		pprofGroup.GET("/profile", gin.WrapF(pprof.Profile))
		pprofGroup.POST("/symbol", gin.WrapF(pprof.Symbol))
		pprofGroup.GET("/symbol", gin.WrapF(pprof.Symbol))
		pprofGroup.GET("/trace", gin.WrapF(pprof.Trace))
		pprofGroup.GET("/allocs", gin.WrapF(pprof.Handler("allocs").ServeHTTP))
		pprofGroup.GET("/block", gin.WrapF(pprof.Handler("block").ServeHTTP))
		pprofGroup.GET("/goroutine", gin.WrapF(pprof.Handler("goroutine").ServeHTTP))
		pprofGroup.GET("/heap", gin.WrapF(pprof.Handler("heap").ServeHTTP))
		pprofGroup.GET("/mutex", gin.WrapF(pprof.Handler("mutex").ServeHTTP))
		pprofGroup.GET("/threadcreate", gin.WrapF(pprof.Handler("threadcreate").ServeHTTP))
	}

	deviceGroup := r.Group("/device")
	deviceGroup.GET("/:udid/info", DeviceInfo)
	deviceGroup.GET("/:udid/health", DeviceHealth)
	deviceGroup.POST("/:udid/tap", DeviceTap)
	deviceGroup.POST("/:udid/touchAndHold", DeviceTouchAndHold)
	deviceGroup.POST("/:udid/home", DeviceHome)
	deviceGroup.POST("/:udid/lock", DeviceLock)
	deviceGroup.POST("/:udid/unlock", DeviceUnlock)
	deviceGroup.POST("/:udid/screenshot", DeviceScreenshot)
	deviceGroup.POST("/:udid/swipe", DeviceSwipe)
	deviceGroup.GET("/:udid/appiumSource", DeviceAppiumSource)
	deviceGroup.POST("/:udid/typeText", DeviceTypeText)
	deviceGroup.POST("/:udid/clearText", DeviceClearText)
	deviceGroup.Any("/:udid/appium/*proxyPath", AppiumReverseProxy)
	deviceGroup.GET("/:udid/android-stream", AndroidStreamProxy)
	deviceGroup.GET("/:udid/android-stream-mjpeg", AndroidStreamMJPEG)
	if config.Config.EnvConfig.UseGadsIosStream {
		deviceGroup.GET("/:udid/ios-stream", IosStreamProxyGADS)
		deviceGroup.GET("/:udid/ios-stream-mjpeg", IOSStreamMJPEG)
	} else {
		deviceGroup.GET("/:udid/ios-stream", IosStreamProxyWDA)
		deviceGroup.GET("/:udid/ios-stream-mjpeg", IOSStreamMJPEGWda)
	}
	deviceGroup.POST("/:udid/uninstallApp", UninstallApp)
	deviceGroup.POST("/:udid/installApp", InstallApp)
	deviceGroup.POST("/:udid/reset", ResetDevice)
	deviceGroup.POST("/:udid/uploadFile", UploadFile)

	return r
}
