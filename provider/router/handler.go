/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"GADS/provider/config"
	"net/http/pprof"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func HandleRequests() *gin.Engine {
	r := gin.Default()
	rConfig := cors.DefaultConfig()
	rConfig.AllowAllOrigins = true
	rConfig.AllowHeaders = []string{"Authorization", "Content-Type"}
	r.Use(cors.New(rConfig))

	r.GET("/info", GetProviderData)
	r.GET("/devices", DevicesInfo)
	r.POST("/uploadFile", UploadAndInstallApp)
	r.GET("/rtc", WebRTCSocket) // This endpoint is for easier testing and debugging of WebRTC instead of building on a device each time

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

	deviceGroup := r.Group("/device/:udid")
	deviceGroup.GET("/info", DeviceInfo)
	deviceGroup.GET("/files", DeviceFiles)
	deviceGroup.POST("/files/push", PushFileToSharedStorage)
	deviceGroup.POST("/files/delete", DeleteFileFromSharedStorage)
	deviceGroup.POST("/files/pull", PullFileFromSharedStorage)
	deviceGroup.GET("/apps", DeviceInstalledApps)
	deviceGroup.GET("/health", DeviceHealth)
	deviceGroup.POST("/tap", DeviceTap)
	deviceGroup.POST("/touchAndHold", DeviceTouchAndHold)
	deviceGroup.POST("/home", DeviceHome)
	deviceGroup.POST("/lock", DeviceLock)
	deviceGroup.POST("/unlock", DeviceUnlock)
	deviceGroup.POST("/screenshot", DeviceScreenshot)
	deviceGroup.POST("/swipe", DeviceSwipe)
	deviceGroup.GET("/appiumSource", DeviceAppiumSource)
	deviceGroup.POST("/typeText", DeviceTypeText)
	deviceGroup.GET("/getClipboard", DeviceGetClipboard)
	deviceGroup.Any("/appium/*proxyPath", AppiumReverseProxy)
	deviceGroup.GET("/android-stream", AndroidStreamProxy)
	deviceGroup.GET("/android-stream-mjpeg", AndroidStreamMJPEG)
	deviceGroup.POST("/update-stream-settings", UpdateDeviceStreamSettings)
	if config.ProviderConfig.UseGadsIosStream {
		deviceGroup.GET("/ios-stream", IosStreamProxyGADS)
		deviceGroup.GET("/ios-stream-mjpeg", IOSStreamMJPEG)
	} else {
		deviceGroup.GET("/ios-stream", IosStreamProxyWDA)
		deviceGroup.GET("/ios-stream-mjpeg", IOSStreamMJPEGWda)
	}
	deviceGroup.POST("/uninstallApp", UninstallApp)
	deviceGroup.POST("/reset", ResetDevice)
	deviceGroup.POST("/uploadAndInstallApp", UploadAndInstallApp)
	deviceGroup.GET("/webrtc", DevicesWebRTCSocket)
	deviceAppiumPluginGroup := deviceGroup.Group("/appium-plugin")
	deviceAppiumPluginGroup.POST("/log", AppiumPluginLog)
	deviceAppiumPluginGroup.POST("/register", AppiumPluginRegister)
	deviceAppiumPluginGroup.POST("/session/add/:session_id", AppiumPluginAddSession)
	deviceAppiumPluginGroup.POST("/session/remove", AppiumPluginRemoveSession)
	deviceAppiumPluginGroup.POST("/ping", AppiumPluginPing)
	deviceAppiumPluginGroup.POST("/log-session", AppiumPluginSessionLog)

	return r
}
