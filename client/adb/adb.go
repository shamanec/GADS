/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package adb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gobwas/ws"
	"github.com/spf13/pflag"
)

func Start(flags *pflag.FlagSet) {
	log.SetFlags(log.Ldate | log.Ltime)

	hub, _ := flags.GetString("hub")
	udid, _ := flags.GetString("udid")
	username, _ := flags.GetString("username")
	password, _ := flags.GetString("password")
	if password == "" {
		password = os.Getenv("GADS_PASSWORD")
	}
	if password == "" {
		log.Fatal("password is required: use --password flag or GADS_PASSWORD env var")
	}
	localPort, _ := flags.GetInt("port")

	hub = strings.TrimRight(hub, "/")

	// Authenticate
	token, err := authenticate(hub, username, password)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}
	log.Println("Authenticated successfully")

	// Open local TCP listener
	listenAddr := fmt.Sprintf("localhost:%d", localPort)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", listenAddr, err)
	}

	actualPort := ln.Addr().(*net.TCPAddr).Port
	adbAddr := fmt.Sprintf("localhost:%d", actualPort)

	// Handle shutdown via signal or tunnel loss
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Build WebSocket URL
	wsScheme := "ws"
	if strings.HasPrefix(hub, "https") {
		wsScheme = "wss"
	}
	parsedHub, err := url.Parse(hub)
	if err != nil {
		log.Fatalf("Invalid hub URL: %v", err)
	}

	wsURL := url.URL{
		Scheme: wsScheme,
		Host:   parsedHub.Host,
		Path:   fmt.Sprintf("/devices/control/%s/adb-tunnel", udid),
	}

	log.Printf("ADB tunnel listening on %s", adbAddr)

	// Verify the tunnel works before connecting ADB
	log.Println("Verifying ADB tunnel connectivity...")
	if err := verifyTunnel(ctx, wsURL.String(), token); err != nil {
		log.Fatalf("ADB tunnel not available: %v", err)
	}
	log.Println("ADB tunnel verified, connecting adb...")

	// Connect ADB after a short delay to let the accept loop start
	go func() {
		time.Sleep(500 * time.Millisecond)
		out, err := exec.Command("adb", "connect", adbAddr).CombinedOutput()
		result := strings.TrimSpace(string(out))
		if err != nil || !strings.Contains(result, "connected") {
			log.Printf("Warning: adb connect failed: %s", result)
			return
		}
		log.Printf("adb connect: %s", result)
	}()

	// Cleanup on context cancellation
	go func() {
		<-ctx.Done()
		log.Println("Shutting down...")
		ln.Close()

		if err := exec.Command("adb", "disconnect", adbAddr).Run(); err != nil {
			log.Printf("Warning: failed to adb disconnect %s: %v", adbAddr, err)
		} else {
			log.Printf("Disconnected adb from %s", adbAddr)
		}
	}()

	// Accept connections
	for {
		tcpConn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}
		go handleConnection(ctx, cancel, tcpConn, wsURL.String(), token)
	}
}

func verifyTunnel(ctx context.Context, wsURL string, token string) error {
	dialer := ws.Dialer{
		Header: ws.HandshakeHeaderHTTP(http.Header{
			"Authorization": []string{"Bearer " + token},
		}),
	}

	conn, _, _, err := dialer.Dial(ctx, wsURL)
	if err != nil {
		return wsErrorToMessage(err)
	}
	conn.Close()
	return nil
}

func handleConnection(ctx context.Context, shutdown context.CancelFunc, tcpConn net.Conn, wsURL string, token string) {
	defer tcpConn.Close()

	dialer := ws.Dialer{
		Header: ws.HandshakeHeaderHTTP(http.Header{
			"Authorization": []string{"Bearer " + token},
		}),
	}

	wsConn, _, _, err := dialer.Dial(ctx, wsURL)
	if err != nil {
		// If the tunnel is rejected (session ended, auth failed, etc.), shut down the client
		friendlyErr := wsErrorToMessage(err)
		log.Printf("Remote control session lost: %v", friendlyErr)
		shutdown()
		return
	}
	defer wsConn.Close()

	log.Println("ADB connection established")

	done := make(chan struct{})

	go func() {
		io.Copy(wsConn, tcpConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(tcpConn, wsConn)
		done <- struct{}{}
	}()

	<-done
	log.Println("ADB connection closed")
}

func wsErrorToMessage(err error) error {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "409"):
		return fmt.Errorf("device requires an active remote control session - open the device in the web UI first")
	case strings.Contains(errMsg, "401"):
		return fmt.Errorf("authentication failed - check your credentials")
	case strings.Contains(errMsg, "400"):
		return fmt.Errorf("bad request - device not found or not an Android device")
	case strings.Contains(errMsg, "422"):
		return fmt.Errorf("device is not available")
	case strings.Contains(errMsg, "502"):
		return fmt.Errorf("hub could not connect to provider - check if the provider is running")
	default:
		return err
	}
}

func authenticate(hub, username, password string) (string, error) {
	payload, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		return "", fmt.Errorf("failed to encode credentials: %w", err)
	}
	resp, err := http.Post(hub+"/authenticate", "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("Failed to post to authenticate endpoint - %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("auth returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool `json:"success"`
		Result  struct {
			AccessToken string `json:"access_token"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode auth response: %v", err)
	}
	if result.Result.AccessToken == "" {
		return "", fmt.Errorf("no access_token in auth response")
	}
	return result.Result.AccessToken, nil
}
