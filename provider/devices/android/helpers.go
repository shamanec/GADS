/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package android

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"GADS/common/constants"
	"GADS/common/models"
	"GADS/common/utils"
)

// getHardwareModel queries ADB for the device brand and model, then combines
// them into a human-readable string stored in d.info.HardwareModel.
// Failures are non-fatal — HardwareModel is set to "Unknown".
func (d *AndroidDevice) getHardwareModel(ctx context.Context) {
	brandOut, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID, "shell", "getprop", "ro.product.brand")
	if err != nil {
		d.info.HardwareModel = "Unknown"
		return
	}
	brand := strings.TrimSpace(string(brandOut))

	modelOut, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID, "shell", "getprop", "ro.product.model")
	if err != nil {
		d.info.HardwareModel = "Unknown"
		return
	}
	model := strings.TrimSpace(string(modelOut))

	d.info.HardwareModel = fmt.Sprintf("%s %s", brand, model)
}

// updateScreenSize queries `adb shell wm size` and populates info.ScreenWidth
// and info.ScreenHeight. Some devices (e.g. Samsung S20) return both a
// "Physical size" and an "Override size" line — the override takes precedence.
func (d *AndroidDevice) updateScreenSize(ctx context.Context) error {
	out, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID, "shell", "wm", "size")
	if err != nil {
		return fmt.Errorf("updateScreenSize %s: %w", d.info.UDID, err)
	}

	lines := strings.Split(string(out), "\n")
	// Two lines → one size + blank; three lines → physical + override + blank.
	switch len(lines) {
	case 2:
		parts := strings.Split(lines[0], ": ")
		if len(parts) < 2 {
			return fmt.Errorf("updateScreenSize %s: unexpected wm size output: %q", d.info.UDID, string(out))
		}
		dims := strings.Split(parts[1], "x")
		if len(dims) < 2 {
			return fmt.Errorf("updateScreenSize %s: unexpected dimensions: %q", d.info.UDID, parts[1])
		}
		d.info.ScreenWidth = strings.TrimSpace(dims[0])
		d.info.ScreenHeight = strings.TrimSpace(dims[1])
	case 3:
		// Use the second line (override size).
		parts := strings.Split(lines[1], ": ")
		if len(parts) < 2 {
			return fmt.Errorf("updateScreenSize %s: unexpected wm size output: %q", d.info.UDID, string(out))
		}
		dims := strings.Split(parts[1], "x")
		if len(dims) < 2 {
			return fmt.Errorf("updateScreenSize %s: unexpected dimensions: %q", d.info.UDID, parts[1])
		}
		d.info.ScreenWidth = strings.TrimSpace(dims[0])
		d.info.ScreenHeight = strings.TrimSpace(dims[1])
	default:
		return fmt.Errorf("updateScreenSize %s: unexpected line count %d in wm size output", d.info.UDID, len(lines))
	}

	if err := d.store.AddOrUpdateDevice(d.info); err != nil {
		return fmt.Errorf("updateScreenSize %s: persist dimensions: %w", d.info.UDID, err)
	}
	return nil
}

// disableAutoRotation sets accelerometer_rotation to 0 so the provider can
// control screen orientation via ADB without the device fighting back.
func (d *AndroidDevice) disableAutoRotation(ctx context.Context) error {
	// 0 = disabled, 1 = enabled
	_, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID, "shell",
		"settings", "put", "system", "accelerometer_rotation", "0")
	if err != nil {
		return fmt.Errorf("disableAutoRotation %s: %w", d.info.UDID, err)
	}
	return nil
}

// forwardPort runs `adb forward tcp:<hostPort> tcp:<devicePort>` so that a
// service running on the device is reachable on the host.
func (d *AndroidDevice) forwardPort(ctx context.Context, hostPort, devicePort string) error {
	_, err := d.cmd.Run(ctx, "adb", "-s", d.info.UDID,
		"forward", "tcp:"+hostPort, "tcp:"+devicePort)
	if err != nil {
		return fmt.Errorf("forwardPort %s: %s→%s: %w", d.info.UDID, devicePort, hostPort, err)
	}
	return nil
}

// GetCurrentRotation returns "portrait" or "landscape" based on the current
// user_rotation system setting read via ADB.
func (d *AndroidDevice) GetCurrentRotation() (string, error) {
	out, err := d.cmd.Run(context.Background(), "adb", "-s", d.info.UDID,
		"shell", "settings", "get", "system", "user_rotation")
	if err != nil {
		return "portrait", fmt.Errorf("GetCurrentRotation %s: %w", d.info.UDID, err)
	}
	if strings.TrimSpace(string(out)) == "1" {
		return "landscape", nil
	}
	return "portrait", nil
}

// ChangeRotation sets the device screen rotation via ADB. rotation must be
// "portrait" or "landscape"; any other value is treated as portrait.
func (d *AndroidDevice) ChangeRotation(rotation string) error {
	val := "0"
	if rotation == "landscape" {
		val = "1"
	}
	_, err := d.cmd.Run(context.Background(), "adb", "-s", d.info.UDID,
		"shell", "settings", "put", "system", "user_rotation", val)
	if err != nil {
		return fmt.Errorf("ChangeRotation %s: %w", d.info.UDID, err)
	}
	return nil
}

// GetSharedStorageFileTree returns a tree of files and directories rooted at
// the Android shared storage root, filtered to allowed top-level folders only
// (DCIM, Documents, Download, Movies, Music, Pictures).
func (d *AndroidDevice) GetSharedStorageFileTree() (*models.AndroidFileNode, error) {
	fileOut, err := d.cmd.Run(context.Background(), "adb", "-s", d.info.UDID,
		"shell", "find", constants.AndroidSharedStorageRoot, "-type", "f")
	if err != nil {
		return nil, fmt.Errorf("GetSharedStorageFileTree %s: list files: %w", d.info.UDID, err)
	}

	dirOut, err := d.cmd.Run(context.Background(), "adb", "-s", d.info.UDID,
		"shell", "find", constants.AndroidSharedStorageRoot, "-type", "d")
	if err != nil {
		return nil, fmt.Errorf("GetSharedStorageFileTree %s: list dirs: %w", d.info.UDID, err)
	}

	fileSet := make(map[string]bool)
	dirSet := make(map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(string(fileOut)))
	for scanner.Scan() {
		p := strings.TrimSpace(scanner.Text())
		if isPathAllowed(p) {
			fileSet[p] = true
		}
	}
	scanner = bufio.NewScanner(strings.NewReader(string(dirOut)))
	for scanner.Scan() {
		p := strings.TrimSpace(scanner.Text())
		if isPathAllowed(p) {
			dirSet[p] = true
		}
	}

	allPaths := make([]string, 0, len(fileSet)+len(dirSet))
	for p := range dirSet {
		allPaths = append(allPaths, p)
	}
	for p := range fileSet {
		allPaths = append(allPaths, p)
	}

	root := &models.AndroidFileNode{
		Name:     constants.AndroidSharedStorageRoot,
		FullPath: constants.AndroidSharedStorageRoot,
		IsFile:   false,
	}
	for _, p := range allPaths {
		addFileNode(root, p, fileSet)
	}
	return root, nil
}

// PullFile pulls filePath from the device shared storage to a temporary file
// on the host and returns the local path.
func (d *AndroidDevice) PullFile(filePath, fileName string) (string, error) {
	tempPath := filepath.Join(os.TempDir(), fileName)
	_, err := d.cmd.Run(context.Background(), "adb", "-s", d.info.UDID, "pull", filePath, tempPath)
	if err != nil {
		return tempPath, fmt.Errorf("PullFile %s: %w", d.info.UDID, err)
	}
	return tempPath, nil
}

// DeleteFile removes filePath from the device shared storage.
func (d *AndroidDevice) DeleteFile(filePath string) error {
	_, err := d.cmd.Run(context.Background(), "adb", "-s", d.info.UDID,
		"shell", "rm", fmt.Sprintf("%q", filePath))
	if err != nil {
		return fmt.Errorf("DeleteFile %s: %w", d.info.UDID, err)
	}
	return nil
}

// isPathAllowed returns true if path is under an allowed shared-storage folder
// and is not a hidden directory (starts with a dot component).
func isPathAllowed(p string) bool {
	if strings.Contains(p, "/.") {
		return false
	}
	return utils.StringStartsWithAny(p, constants.AndroidAllowedSharedStorageFolders...)
}

// addFileNode inserts fullPath into the tree rooted at root, marking leaf
// entries as files when they appear in fileSet.
func addFileNode(root *models.AndroidFileNode, fullPath string, fileSet map[string]bool) {
	rel := strings.TrimPrefix(fullPath, constants.AndroidSharedStorageRoot)
	parts := strings.Split(strings.TrimPrefix(rel, "/"), "/")

	cur := root
	curPath := constants.AndroidSharedStorageRoot

	for i, part := range parts {
		if part == "" {
			continue
		}
		if cur.Children == nil {
			cur.Children = make(map[string]*models.AndroidFileNode)
		}
		curPath = path.Join(curPath, part)
		child, ok := cur.Children[part]
		if !ok {
			child = &models.AndroidFileNode{
				Name:     part,
				FullPath: curPath,
				IsFile:   false,
			}
			cur.Children[part] = child
		}
		if i == len(parts)-1 && fileSet[fullPath] {
			child.IsFile = true
			child.Children = nil
		}
		cur = child
	}
}
