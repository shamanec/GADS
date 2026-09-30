package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"GADS/provider/devices"
)

// Session transitions must finish before another transition on this device.
// Cancellation of a client connection does not cancel Appium's driver work;
// keep reading its response so a following create cannot overtake deletion.
func serveAppiumLifecycle(d devices.PlatformDevice, proxy *httputil.ReverseProxy, path string, w http.ResponseWriter, r *http.Request) {
	create := r.Method == http.MethodPost && strings.TrimSuffix(path, "/") == "/session"
	deleteSession := r.Method == http.MethodDelete && strings.HasPrefix(path, "/session/") && !strings.Contains(strings.TrimPrefix(path, "/session/"), "/")
	if !create && !deleteSession {
		proxy.ServeHTTP(w, r)
		return
	}
	unlock := d.LockAppiumLifecycle()
	defer unlock()
	if r.Context().Err() != nil {
		return
	}
	log := d.GetLogger()
	if log != nil {
		log.LogDebugf("appium_lifecycle", "Transition start udid=%s method=%s old_session=%s appium_port=%s appium_up=%t busy=true", d.GetUDID(), r.Method, d.GetAppiumSessionID(), d.GetAppiumPort(), d.GetIsAppiumUp())
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Minute)
	defer cancel()
	var newSessionID string
	var responseStatus int
	if create {
		proxy.ModifyResponse = func(resp *http.Response) error {
			responseStatus = resp.StatusCode
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return err
			}
			resp.Body = io.NopCloser(bytes.NewReader(body))
			var result struct {
				Value struct {
					SessionID string `json:"sessionId"`
				} `json:"value"`
			}
			if resp.StatusCode == http.StatusOK && json.Unmarshal(body, &result) == nil {
				newSessionID = result.Value.SessionID
			}
			return nil
		}
	}
	defer func() {
		// Appium may have created a session after its caller disappeared. Delete
		// precisely that session while still holding the device transition lock.
		if create && newSessionID != "" && r.Context().Err() != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cleanupCancel()
			req, _ := http.NewRequestWithContext(cleanupCtx, http.MethodDelete, "http://localhost:"+d.GetAppiumPort()+"/session/"+newSessionID, nil)
			resp, err := http.DefaultClient.Do(req)
			status := 0
			if resp != nil {
				status = resp.StatusCode
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
			if log != nil {
				log.LogDebugf("appium_lifecycle", "Disconnected client cleanup udid=%s session=%s status=%d transport_error=%t", d.GetUDID(), newSessionID, status, err != nil)
			}
		}
		if create && (responseStatus == 0 || responseStatus >= 400) {
			logAppiumFailure(d, responseStatus)
		}
		if log != nil {
			log.LogDebugf("appium_lifecycle", "Transition complete udid=%s new_session=%s client_canceled=%t upstream_canceled=%t", d.GetUDID(), newSessionID, r.Context().Err() != nil, ctx.Err() != nil)
		}
	}()
	proxy.ServeHTTP(w, r.WithContext(ctx))
}

// Probe only after creation failures; log status codes, never capabilities,
// response bodies, or credentials. These distinguish a live Appium server from
// an unavailable WDA listener without restarting either service.
func logAppiumFailure(d devices.PlatformDevice, responseStatus int) {
	log := d.GetLogger()
	if log == nil {
		return
	}
	probe := func(port string) int {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:"+port+"/status", nil)
		if err != nil {
			return 0
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	log.LogDebugf("appium_lifecycle", "Creation failed udid=%s session=%s response_status=%d appium_port=%s health_status=%d", d.GetUDID(), d.GetAppiumSessionID(), responseStatus, d.GetAppiumPort(), probe(d.GetAppiumPort()))
	if ios, ok := d.(*devices.IOSDevice); ok {
		log.LogDebugf("appium_lifecycle", "WDA diagnostic udid=%s port=%s mjpeg_port=%s health_status=%d", d.GetUDID(), ios.GetWDAPort(), ios.GetWDAStreamPort(), probe(ios.GetWDAPort()))
	}
}
