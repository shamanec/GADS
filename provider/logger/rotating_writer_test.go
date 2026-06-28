/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRotatingFileWriterRotatesAndCleansOldBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.log")
	writer, err := newRotatingFileWriter(path, LogFileRetention{
		MaxSizeBytes: 20,
		MaxBackups:   2,
	})
	require.NoError(t, err)
	defer writer.Close()

	lines := []string{
		"first log line\n",
		"second log line\n",
		"third log line\n",
		"fourth log line\n",
	}

	for _, line := range lines {
		_, err := writer.Write([]byte(line))
		require.NoError(t, err)
	}

	assertFileContains(t, path, "fourth log line")
	assertFileContains(t, path+".1", "third log line")
	assertFileContains(t, path+".2", "second log line")
	assert.NoFileExists(t, path+".3")
	assertFileNotContains(t, path+".2", "first log line")
}

func TestRotatingFileWriterCanDisableCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.log")
	writer, err := newRotatingFileWriter(path, LogFileRetention{
		MaxSizeBytes: 0,
		MaxBackups:   0,
	})
	require.NoError(t, err)
	defer writer.Close()

	_, err = writer.Write([]byte(strings.Repeat("a", 128)))
	require.NoError(t, err)
	_, err = writer.Write([]byte(strings.Repeat("b", 128)))
	require.NoError(t, err)

	assert.NoFileExists(t, path+".1")
	assertFileContains(t, path, strings.Repeat("a", 128))
	assertFileContains(t, path, strings.Repeat("b", 128))
}

func TestNewLogFileRetentionRejectsInvalidValues(t *testing.T) {
	_, err := NewLogFileRetention(-1, 1)
	assert.ErrorContains(t, err, "log max size")

	_, err = NewLogFileRetention(1, -1)
	assert.ErrorContains(t, err, "log max backups")
}

func TestRotatingFileWriterReturnsOpenError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "provider.log")
	_, err := newRotatingFileWriter(path, LogFileRetention{
		MaxSizeBytes: 20,
		MaxBackups:   1,
	})
	assert.Error(t, err)
}

func TestRotatingFileWriterReturnsRotationCleanupError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.log")
	writer, err := newRotatingFileWriter(path, LogFileRetention{
		MaxSizeBytes: 10,
		MaxBackups:   1,
	})
	require.NoError(t, err)
	defer writer.Close()

	require.NoError(t, os.Mkdir(path+".1", 0755))
	require.NoError(t, os.WriteFile(filepath.Join(path+".1", "blocked"), []byte("backup"), 0644))

	_, err = writer.Write([]byte("first line\n"))
	require.NoError(t, err)

	_, err = writer.Write([]byte("second line\n"))
	assert.Error(t, err)
}

func assertFileContains(t *testing.T, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), expected)
}

func assertFileNotContains(t *testing.T, path, unexpected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(content), unexpected)
}
