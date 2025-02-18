package cli

import (
	"GADS/provider/logger"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"github.com/shirou/gopsutil/process"
)

func ExecuteCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)

	var combinedOutput bytes.Buffer
	cmd.Stdout = &combinedOutput
	cmd.Stderr = &combinedOutput

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %s", cmd.Path)
	}

	return combinedOutput.String(), nil
}

func ExecuteCommandWithContext(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %s, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// Kill process found by gopsutil
func KillProcess(p *process.Process) error {
	switch runtime.GOOS {
	case "windows":
		return p.Kill()
	default:
		return p.SendSignal(syscall.SIGKILL)
	}
}

// Kills any hanging appium processes for the current device
func KillAppiumProcess(udid string) error {
	processes, err := process.Processes()
	if err != nil {
		return fmt.Errorf("KillAppiumProcess: failed to list processes: %v", err)
	}

	for _, p := range processes {
		cmdline, err := p.Cmdline()
		if err != nil {
			continue // Ignore processes that cant be read
		}

		if strings.Contains(cmdline, "node") && strings.Contains(cmdline, "appium") && strings.Contains(cmdline, udid) {
			pid := p.Pid
			logger.ProviderLogger.LogDebug("kill_appium_process", fmt.Sprintf("Killing Appium process with pid `%d` for device `%s`", pid, udid))

			if err := KillProcess(p); err != nil {
				return fmt.Errorf("KillAppiumProcess: failed to kill process %d: %v", pid, err)
			}
		}
	}

	return nil
}
