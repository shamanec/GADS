//go:build windows

package devices

import (
	"os/exec"
	"syscall"
)

func SetupProcessAttributes(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true, // Hide the window to detach the process from the terminal
	}
}
