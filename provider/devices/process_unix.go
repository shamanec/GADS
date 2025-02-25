//go:build linux || darwin

package devices

import (
	"os/exec"
	"syscall"
)

func SetupProcessAttributes(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // This detaches the child process from the parent and terminal
	}
}
