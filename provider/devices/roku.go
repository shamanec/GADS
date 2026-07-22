package devices

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/icholy/digest"

	"GADS/common/models"
	"GADS/provider/logger"
)

const (
	rokuECPPort = 8060
	rokuWebPort = 80
)

// RokuDevice holds Roku TV-specific runtime state alongside the shared RuntimeState.
type RokuDevice struct {
	RuntimeState
}

var rokuHTTPClient = &http.Client{Timeout: 5 * time.Second}

type rokuDeviceInfo struct {
	ModelName       string `xml:"model-name"`
	SoftwareVersion string `xml:"software-version"`
}

type rokuApps struct {
	Apps []rokuApp `xml:"app"`
}

type rokuApp struct {
	ID   string `xml:"id,attr"`
	Name string `xml:",chardata"`
}

// rokuHost returns the TV IP from the UDID, stripping the optional ECP port.
func rokuHost(udid string) string {
	if host, _, found := strings.Cut(udid, ":"); found {
		return host
	}
	return udid
}

func rokuECPURL(udid, path string) string {
	return fmt.Sprintf("http://%s:%d%s", rokuHost(udid), rokuECPPort, path)
}

// Setup runs the full Roku device provisioning sequence.
func (d *RokuDevice) Setup() error {
	d.SetupMutex.Lock()
	defer d.SetupMutex.Unlock()

	d.SetProviderState("preparing")
	logger.ProviderLogger.LogInfo("roku_device_setup", fmt.Sprintf("Running setup for Roku device `%v`", d.GetUDID()))

	d.DBDevice.IPAddress = rokuHost(d.GetUDID())
	d.getTVInfo()

	if err := setupAppiumForDevice(d); err != nil {
		return err
	}

	d.SetProviderState("live")
	return nil
}

// AppiumCapabilities returns the Roku-specific Appium server capabilities.
func (d *RokuDevice) AppiumCapabilities() models.AppiumServerCapabilities {
	return models.AppiumServerCapabilities{
		AutomationName: "roku",
		PlatformName:   "Roku",
		UDID:           d.GetUDID(),
		DeviceName:     d.DBDevice.Name,
		RokuHost:       rokuHost(d.GetUDID()),
		RokuEcpPort:    rokuECPPort,
		RokuWebPort:    rokuWebPort,
		RokuUser:       "rokudev",
		RokuHeaderHost: rokuHost(d.GetUDID()),
	}
}

func (d *RokuDevice) getTVInfo() {
	resp, err := rokuHTTPClient.Get(rokuECPURL(d.GetUDID(), "/query/device-info"))
	if err != nil {
		logger.ProviderLogger.LogError("roku_device_setup", fmt.Sprintf("Failed to get device info for Roku device %s: %v", d.GetUDID(), err))
		return
	}
	defer resp.Body.Close()

	var info rokuDeviceInfo
	if err := xml.NewDecoder(resp.Body).Decode(&info); err != nil {
		logger.ProviderLogger.LogError("roku_device_setup", fmt.Sprintf("Failed to decode device info for Roku device %s: %v", d.GetUDID(), err))
		return
	}

	d.HardwareModel = info.ModelName
	if info.SoftwareVersion != "" {
		d.DBDevice.OSVersion = info.SoftwareVersion
	}
}

// getConnectedDevicesRoku returns the configured Roku devices whose ECP endpoint responds.
func getConnectedDevicesRoku() []string {
	var connectedDevices []string
	for _, dev := range DevManager.All() {
		if dev.GetOS() != "roku" {
			continue
		}

		udid := dev.GetUDID()
		resp, err := rokuHTTPClient.Get(rokuECPURL(udid, "/query/device-info"))
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			connectedDevices = append(connectedDevices, udid)
		}
	}
	return connectedDevices
}

func (d *RokuDevice) queryApps() []rokuApp {
	resp, err := rokuHTTPClient.Get(rokuECPURL(d.GetUDID(), "/query/apps"))
	if err != nil {
		logger.ProviderLogger.LogError("roku_list_apps", fmt.Sprintf("Failed to list apps for Roku device %s: %v", d.GetUDID(), err))
		return nil
	}
	defer resp.Body.Close()

	var apps rokuApps
	if err := xml.NewDecoder(resp.Body).Decode(&apps); err != nil {
		logger.ProviderLogger.LogError("roku_list_apps", fmt.Sprintf("Failed to decode apps for Roku device %s: %v", d.GetUDID(), err))
		return nil
	}
	return apps.Apps
}

// GetInstalledApps returns installed apps info (returns as []models.DeviceApp for the interface).
func (d *RokuDevice) GetInstalledApps() ([]models.DeviceApp, error) {
	var result []models.DeviceApp
	for _, app := range d.queryApps() {
		result = append(result, models.DeviceApp{
			AppName:          strings.TrimSpace(app.Name),
			BundleIdentifier: app.ID,
			CanUninstall:     app.ID == "dev",
		})
	}
	return result, nil
}

// GetInstalledAppBundleIDs returns bundle identifiers (channel ids) of installed apps.
func (d *RokuDevice) GetInstalledAppBundleIDs() []string {
	var ids []string
	for _, app := range d.queryApps() {
		ids = append(ids, app.ID)
	}
	return ids
}

// LaunchApp launches a channel on the Roku device over ECP.
func (d *RokuDevice) LaunchApp(appID string) error {
	resp, err := rokuHTTPClient.Post(rokuECPURL(d.GetUDID(), "/launch/"+appID), "", nil)
	if err != nil {
		return fmt.Errorf("failed to launch app %s: %w", appID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to launch app %s: ECP returned status %d", appID, resp.StatusCode)
	}
	return nil
}

// KillApp exits the active channel by sending the Home key over ECP.
func (d *RokuDevice) KillApp(appID string) error {
	resp, err := rokuHTTPClient.Post(rokuECPURL(d.GetUDID(), "/keypress/Home"), "", nil)
	if err != nil {
		return fmt.Errorf("failed to kill app %s: %w", appID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to kill app %s: ECP returned status %d", appID, resp.StatusCode)
	}
	return nil
}

// InstallApp requires the developer password, supplied per request via InstallDevChannel.
func (d *RokuDevice) InstallApp(appName string) error {
	return fmt.Errorf("roku channel install requires the developer password")
}

// UninstallApp requires the developer password, supplied per request via UninstallDevChannel.
func (d *RokuDevice) UninstallApp(appID string) error {
	return fmt.Errorf("roku channel uninstall requires the developer password")
}

// InstallDevChannel sideloads a dev channel .zip through the Roku dev web installer.
func (d *RokuDevice) InstallDevChannel(zipPath, password string) error {
	if password == "" {
		return fmt.Errorf("roku dev password is required to sideload a channel")
	}
	return d.rokuWebInstaller("Install", zipPath, password)
}

// UninstallDevChannel removes the dev channel through the Roku dev web installer.
func (d *RokuDevice) UninstallDevChannel(password string) error {
	if password == "" {
		return fmt.Errorf("roku dev password is required to remove the dev channel")
	}
	return d.rokuWebInstaller("Delete", "", password)
}

func (d *RokuDevice) rokuWebInstaller(submit, zipPath, password string) error {
	body, contentType, err := buildRokuInstallerBody(submit, zipPath)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/plugin_install", rokuHost(d.GetUDID()))
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{
		Transport: &digest.Transport{Username: "rokudev", Password: password},
		Timeout:   30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach Roku web installer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("roku web installer rejected the developer password")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("roku web installer returned status %d", resp.StatusCode)
	}
	respBody, _ := io.ReadAll(resp.Body)
	if strings.Contains(strings.ToLower(string(respBody)), "failed") {
		return fmt.Errorf("roku web installer reported a failure (check the developer password and the channel package)")
	}
	return nil
}

func buildRokuInstallerBody(submit, zipPath string) ([]byte, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("mySubmit", submit); err != nil {
		return nil, "", err
	}

	if zipPath != "" {
		file, err := os.Open(zipPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to open channel package: %w", err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("archive", filepath.Base(zipPath))
		if err != nil {
			return nil, "", err
		}
		if _, err := io.Copy(part, file); err != nil {
			return nil, "", err
		}
	} else {
		if err := writer.WriteField("archive", ""); err != nil {
			return nil, "", err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}
