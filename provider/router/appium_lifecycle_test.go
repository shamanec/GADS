package router

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"GADS/provider/devices"
	"github.com/stretchr/testify/require"
)

func TestAppiumSessionLifecycle(t *testing.T) {
	for _, mode := range []string{"normal", "failed-test", "partial-creation", "disconnected-client"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			active := ""
			count := 0
			started, finish := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete {
					mu.Lock()
					active = ""
					mu.Unlock()
					w.Write([]byte(`{"value":null}`))
					return
				}
				mu.Lock()
				count++
				n := count
				previous := active
				active = fmt.Sprint(n)
				mu.Unlock()
				if previous != "" {
					t.Errorf("new session overlapped old session %s", previous)
				}
				if n == 1 && mode == "disconnected-client" {
					close(started)
					<-finish
				}
				if n == 1 && mode == "partial-creation" {
					// Driver creation fails after allocation; Appium's driver teardown
					// completes before returning the session-not-created response.
					mu.Lock()
					active = ""
					mu.Unlock()
					w.WriteHeader(500)
					w.Write([]byte(`{"value":{"error":"session not created"}}`))
					return
				}
				w.Write([]byte(fmt.Sprintf(`{"value":{"sessionId":%q}}`, fmt.Sprint(n))))
			}))
			defer server.Close()
			d := &devices.IOSDevice{}
			parsed, _ := url.Parse(server.URL)
			d.SetAppiumPort(parsed.Port())
			send := func(ctx context.Context, method, path string) {
				req := httptest.NewRequest(method, path, nil).WithContext(ctx)
				serveAppiumLifecycle(d, newAppiumProxy(server.URL, path), path, httptest.NewRecorder(), req)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "disconnected-client" {
				done := make(chan struct{})
				go func() { defer close(done); send(ctx, "POST", "/session") }()
				<-started
				cancel()
				close(finish)
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Fatal("creation/cleanup did not finish")
				}
			} else {
				send(ctx, "POST", "/session")
				if mode != "partial-creation" {
					send(ctx, "DELETE", "/session/1")
				}
			}
			mu.Lock()
			require.Empty(t, active)
			mu.Unlock()
			send(context.Background(), "POST", "/session")
			mu.Lock()
			require.Equal(t, "2", active)
			mu.Unlock()
		})
	}
}

func TestDeleteSurvivesClientCancellation(t *testing.T) {
	started, finish := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-finish
		w.Write([]byte(`{"value":null}`))
	}))
	defer server.Close()
	d := &devices.IOSDevice{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		req := httptest.NewRequest("DELETE", "/session/a", nil).WithContext(ctx)
		serveAppiumLifecycle(d, newAppiumProxy(server.URL, "/session/a"), "/session/a", httptest.NewRecorder(), req)
	}()
	<-started
	cancel()
	select {
	case <-done:
		t.Error("released transition before upstream cleanup finished")
	default:
	}
	// A different device's lifecycle remains independent.
	other := &devices.IOSDevice{}
	unlock := other.LockAppiumLifecycle()
	unlock()
	close(finish)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("DELETE did not finish")
	}
}

func TestSessionRemovalMatchesID(t *testing.T) {
	d := &devices.IOSDevice{}
	d.UpdateAppiumSession("b", map[string]interface{}{"platformName": "iOS"})
	require.False(t, d.ClearAppiumSession("a"))
	require.Equal(t, "b", d.ToSyncUpdate().AppiumSessionID)
	require.True(t, d.ClearAppiumSession("b"))
	require.False(t, d.ClearAppiumSession("b"))
	require.False(t, d.ToSyncUpdate().HasAppiumSession)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				d.UpdateAppiumSession("a", nil)
				d.ClearAppiumSession("a")
				d.ToSyncUpdate()
				d.GetAppiumSessionID()
			}
		}()
	}
	wg.Wait()
}
