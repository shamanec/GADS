package cli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
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
