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
	"fmt"
	"os"
	"sync"
)

const (
	bytesPerMegabyte     = 1024 * 1024
	DefaultLogMaxSizeMB  = 10
	DefaultLogMaxBackups = 3
)

type LogFileRetention struct {
	MaxSizeBytes int64
	MaxBackups   int
}

func NewLogFileRetention(maxSizeMB, maxBackups int) (LogFileRetention, error) {
	if maxSizeMB < 0 {
		return LogFileRetention{}, fmt.Errorf("log max size must be greater than or equal to 0")
	}
	if maxBackups < 0 {
		return LogFileRetention{}, fmt.Errorf("log max backups must be greater than or equal to 0")
	}

	return LogFileRetention{
		MaxSizeBytes: int64(maxSizeMB) * bytesPerMegabyte,
		MaxBackups:   maxBackups,
	}, nil
}

type rotatingFileWriter struct {
	mu        sync.Mutex
	path      string
	retention LogFileRetention
	file      *os.File
	size      int64
}

func newRotatingFileWriter(path string, retention LogFileRetention) (*rotatingFileWriter, error) {
	writer := &rotatingFileWriter{
		path:      path,
		retention: retention,
	}
	if err := writer.open(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *rotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.shouldRotate(len(p)) {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *rotatingFileWriter) open() error {
	file, err := os.OpenFile(w.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}

	w.file = file
	w.size = info.Size()
	return nil
}

func (w *rotatingFileWriter) shouldRotate(nextWriteBytes int) bool {
	return w.retention.MaxSizeBytes > 0 &&
		w.size > 0 &&
		w.size+int64(nextWriteBytes) > w.retention.MaxSizeBytes
}

func (w *rotatingFileWriter) rotate() error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
		w.file = nil
	}

	if w.retention.MaxBackups == 0 {
		if err := removeIfExists(w.path); err != nil {
			return err
		}
		return w.open()
	}

	if err := removeIfExists(w.backupPath(w.retention.MaxBackups)); err != nil {
		return err
	}

	for index := w.retention.MaxBackups - 1; index >= 1; index-- {
		source := w.backupPath(index)
		destination := w.backupPath(index + 1)
		if err := renameIfExists(source, destination); err != nil {
			return err
		}
	}

	if err := renameIfExists(w.path, w.backupPath(1)); err != nil {
		return err
	}

	return w.open()
}

func (w *rotatingFileWriter) backupPath(index int) string {
	return fmt.Sprintf("%s.%d", w.path, index)
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func renameIfExists(source, destination string) error {
	err := os.Rename(source, destination)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}
