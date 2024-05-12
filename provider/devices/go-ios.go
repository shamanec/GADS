package devices

import (
	"fmt"
	"github.com/danielpaulus/go-ios/ios/zipconduit"

	"GADS/common/models"
	"GADS/provider/logger"
)

func InstallAppWithDevice(device *models.Device, filePath string) error {
	logger.ProviderLogger.LogInfo("ios_device", fmt.Sprintf("Installing app `%s` on iOS device `%s`", filePath, device.UDID))
	conn, err := zipconduit.New(device.GoIOSDeviceEntry)
	if err != nil {
		return fmt.Errorf("InstallAppWithDevice: Failed creating zip conduit with go-ios - %s", err)
	}

	err = conn.SendFile(filePath)
	if err != nil {
		return fmt.Errorf("InstallAppWithDevice: Failed installing application with go-ios - %s", err)
	}
	return nil
}
