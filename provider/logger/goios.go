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
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"GADS/provider/config"

	"github.com/danielpaulus/go-ios/ios"
)

// goIOSLinesPerDevice is the number of recent go-ios log lines retained in
// memory per device - enough to cover a full tunnel/forward/WDA setup sequence.
const goIOSLinesPerDevice = 500

// goIOSRingLevel is the minimum level kept in the in-memory ring buffers that
// are attached to device failure logs. Warn drops install-progress noise so
// the buffer stays useful for diagnosing tunnel/forward/WDA failures.
const goIOSRingLevel = slog.LevelWarn

// GoIOSLogs captures go-ios's slog output per device.
//
// go-ios logs through log/slog and tags most of its records with a "udid"
// attr. Each record is routed to a per-device `device_<udid>/goios.log` file
// and a per-device in-memory ring buffer, instead of one global stream
// interleaving every device on the provider terminal. Records without a udid
// (early discovery, process-global operations) land in `<provider-folder>/goios.log`.
//
// Use Tail(udid, n) to retrieve recent lines when a go-ios operation fails
// for a device.
var GoIOSLogs = &GoIOSLogStore{
	buffers:   make(map[string]*lineRing),
	files:     make(map[string]*os.File),
	fileLevel: slog.LevelInfo,
}

// SetupGoIOSLogging routes all of go-ios's internal logging into GoIOSLogs so
// go-ios chatter never reaches the provider terminal. The goios.log files
// follow the provider `--log-level` flag - note that go-ios debug output is
// very chatty. Must be called after SetupLogging (for the configured level)
// and after the provider config is set up because the log files live in the
// provider folder.
func SetupGoIOSLogging() {
	if level, ok := logLevelMapping[logLevel]; ok {
		GoIOSLogs.fileLevel = level
	}
	ios.SetLogger(slog.New(&goIOSLogHandler{store: GoIOSLogs}))
}

// GoIOSLogStore holds one log file and one ring buffer per device udid,
// created lazily on first write. Device counts are small and udids are stable
// per physical device, so neither is ever evicted or closed.
type GoIOSLogStore struct {
	mu        sync.Mutex
	buffers   map[string]*lineRing
	files     map[string]*os.File
	fileLevel slog.Level // minimum level written to the goios.log files, from the provider log level
}

// write routes a formatted go-ios log line to the file and/or ring buffer of
// the device it belongs to.
func (s *GoIOSLogStore) write(udid, line string, level slog.Level) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if level >= s.fileLevel {
		file, ok := s.files[udid]
		if !ok {
			file = s.openLogFile(udid)
			s.files[udid] = file
		}
		if file != nil {
			fmt.Fprintln(file, line)
		}
	}

	if level >= goIOSRingLevel {
		buf := s.buffers[udid]
		if buf == nil {
			buf = newLineRing(goIOSLinesPerDevice)
			s.buffers[udid] = buf
		}
		buf.push(line)
	}
}

// openLogFile opens the goios.log file for a device, creating the device
// folder if needed. Returns nil on failure - the store keeps working with
// just the ring buffer and does not retry, so a bad path fails only once.
func (s *GoIOSLogStore) openLogFile(udid string) *os.File {
	path := filepath.Join(config.ProviderConfig.ProviderFolder, "goios.log")
	if udid != "" {
		deviceFolder := filepath.Join(config.ProviderConfig.ProviderFolder, fmt.Sprintf("device_%s", udid))
		if err := os.MkdirAll(deviceFolder, os.ModePerm); err != nil {
			fmt.Fprintf(os.Stderr, "Could not create device folder for go-ios logs of device `%s` - %s\n", udid, err)
			return nil
		}
		path = filepath.Join(deviceFolder, "goios.log")
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open go-ios log file `%s` - %s\n", path, err)
		return nil
	}
	return file
}

// Tail returns the last n captured go-ios lines for the given device udid, or
// "" if nothing has been captured for it yet.
func (s *GoIOSLogStore) Tail(udid string, n int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf := s.buffers[udid]
	if buf == nil {
		return ""
	}
	return buf.tail(n)
}

// goIOSLogHandler is the slog.Handler handed to ios.SetLogger. It formats
// each record into a single line and routes it into the store keyed by the
// record's "udid" attr. It deliberately does not chain to any other handler,
// so go-ios logs stay out of the provider terminal and the `logs` database.
type goIOSLogHandler struct {
	store *GoIOSLogStore
	attrs []slog.Attr // accumulated via WithAttrs (go-ios pre-binds udid this way)
	group string      // accumulated via WithGroup, prefixed onto attr keys
}

func (h *goIOSLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.store.fileLevel || level >= goIOSRingLevel
}

func (h *goIOSLogHandler) Handle(_ context.Context, r slog.Record) error {
	udid := ""
	var line strings.Builder
	fmt.Fprintf(&line, "%s %s %s", r.Time.Format("15:04:05.000"), r.Level.String(), r.Message)

	writeAttr := func(a slog.Attr) {
		if a.Key == "" {
			return
		}
		if a.Key == "udid" {
			udid = a.Value.String()
		}
		fmt.Fprintf(&line, " %s=%v", h.qualify(a.Key), a.Value.Any())
	}
	// Pre-bound attrs (WithAttrs) first, then the record's own attrs.
	for _, a := range h.attrs {
		writeAttr(a)
	}
	r.Attrs(func(a slog.Attr) bool { writeAttr(a); return true })

	h.store.write(udid, line.String(), r.Level)
	return nil
}

func (h *goIOSLogHandler) WithAttrs(as []slog.Attr) slog.Handler {
	out := *h
	out.attrs = append(append([]slog.Attr(nil), h.attrs...), as...)
	return &out
}

func (h *goIOSLogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	out := *h
	if h.group != "" {
		out.group = h.group + "." + name
	} else {
		out.group = name
	}
	return &out
}

func (h *goIOSLogHandler) qualify(key string) string {
	if h.group == "" {
		return key
	}
	return h.group + "." + key
}

// lineRing is a circular buffer storing the last N log lines.
type lineRing struct {
	lines   []string
	cap     int
	pos     int // next write position (wraps around)
	written int // total lines ever written
}

func newLineRing(capacity int) *lineRing {
	return &lineRing{lines: make([]string, capacity), cap: capacity}
}

// push and tail are unsynchronized - GoIOSLogStore serializes access.
func (r *lineRing) push(line string) {
	r.lines[r.pos%r.cap] = line
	r.pos++
	r.written++
}

func (r *lineRing) tail(n int) string {
	if n > r.cap {
		n = r.cap
	}
	if r.written < n {
		n = r.written
	}
	if n == 0 {
		return ""
	}

	result := make([]string, n)
	start := r.pos - n
	for i := 0; i < n; i++ {
		result[i] = r.lines[(start+i)%r.cap]
	}
	return strings.Join(result, "\n")
}
