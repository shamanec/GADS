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
	"io"
	"log/slog"
	"os"
	"sync"

	"GADS/common/db"
	"GADS/provider/config"

	"go.mongodb.org/mongo-driver/mongo"
)

// Levels above slog.LevelError preserving logrus's fatal/panic level strings
// in the log files and the MongoDB `logs` database.
const (
	LevelFatal = slog.LevelError + 4
	LevelPanic = slog.LevelError + 8
)

// mongoRingSize is the number of log entries buffered in memory while the
// MongoDB drainer catches up (e.g. during a lost Mongo connection). When the
// buffer is full the oldest entry is evicted.
const mongoRingSize = 2000

type CustomLogger struct {
	logger *slog.Logger
	mongo  *mongoHandler
}

var logLevelMapping = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

var ProviderLogger *CustomLogger
var logLevel string

func SetupLogging(level string) {
	logLevel = level

	var err error
	fmt.Printf("Provider will be logging to `%s/provider.log`\n", config.ProviderConfig.ProviderFolder)
	ProviderLogger, err = createCustomLogger(fmt.Sprintf("%s/provider.log", config.ProviderConfig.ProviderFolder), config.ProviderConfig.Nickname, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create custom logger for the provider instance - %s\n", err)
		os.Exit(1)
	}
}

func (l CustomLogger) log(level slog.Level, eventName, message string) {
	l.logger.Log(context.Background(), level, message, "event", eventName)
}

func (l CustomLogger) LogDebug(eventName string, message string) {
	l.log(slog.LevelDebug, eventName, message)
}

func (l CustomLogger) LogInfo(eventName string, message string) {
	l.log(slog.LevelInfo, eventName, message)
}

func (l CustomLogger) LogError(eventName string, message string) {
	l.log(slog.LevelError, eventName, message)
}

func (l CustomLogger) LogWarn(eventName string, message string) {
	l.log(slog.LevelWarn, eventName, message)
}

func (l CustomLogger) LogFatal(eventName string, message string) {
	l.log(LevelFatal, eventName, message)
	if l.mongo != nil {
		l.mongo.flush()
	}
	os.Exit(1)
}

func (l CustomLogger) LogPanic(eventName string, message string) {
	l.log(LevelPanic, eventName, message)
	if l.mongo != nil {
		l.mongo.flush()
	}
	panic(message)
}

func (l CustomLogger) LogDebugf(eventName string, format string, args ...any) {
	l.log(slog.LevelDebug, eventName, fmt.Sprintf(format, args...))
}

func (l CustomLogger) LogInfof(eventName string, format string, args ...any) {
	l.log(slog.LevelInfo, eventName, fmt.Sprintf(format, args...))
}

func (l CustomLogger) LogErrorf(eventName string, format string, args ...any) {
	l.log(slog.LevelError, eventName, fmt.Sprintf(format, args...))
}

func (l CustomLogger) LogWarnf(eventName string, format string, args ...any) {
	l.log(slog.LevelWarn, eventName, fmt.Sprintf(format, args...))
}

func (l CustomLogger) LogFatalf(eventName string, format string, args ...any) {
	l.LogFatal(eventName, fmt.Sprintf(format, args...))
}

func (l CustomLogger) LogPanicf(eventName string, format string, args ...any) {
	l.LogPanic(eventName, fmt.Sprintf(format, args...))
}

// CreateCustomLogger creates a device-scoped logger writing JSON to the given
// log file and asynchronously to the MongoDB `logs` database. Device loggers
// do not mirror to stdout so a provider with many devices keeps a readable
// terminal - device logs are available in the log file and through the UI.
func CreateCustomLogger(logFilePath, collection string) (*CustomLogger, error) {
	return createCustomLogger(logFilePath, collection, false)
}

func createCustomLogger(logFilePath, collection string, mirrorStdout bool) (*CustomLogger, error) {
	level, ok := logLevelMapping[logLevel]
	if !ok {
		level = slog.LevelInfo
	}

	// Open the log file
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return &CustomLogger{}, fmt.Errorf("Could not set log output - %v", err)
	}

	var out io.Writer = logFile
	if mirrorStdout {
		out = io.MultiWriter(logFile, os.Stdout)
	}

	jsonHandler := slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				if lvl, ok := a.Value.Any().(slog.Level); ok {
					a.Value = slog.StringValue(levelString(lvl))
				}
			}
			return a
		},
	})

	mongoH := newMongoHandler(db.GlobalMongoStore.Client, db.GlobalMongoStore.Ctx, "logs", collection, level)

	return &CustomLogger{
		logger: slog.New(multiHandler{jsonHandler, mongoH}),
		mongo:  mongoH,
	}, nil
}

// levelString maps slog levels to the level strings logrus used, so the
// documents in the `logs` database and the log file format stay unchanged.
func levelString(l slog.Level) string {
	switch {
	case l >= LevelPanic:
		return "panic"
	case l >= LevelFatal:
		return "fatal"
	case l >= slog.LevelError:
		return "error"
	case l >= slog.LevelWarn:
		return "warning"
	case l >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}

type logEntry struct {
	Level     string `bson:"level"`
	Message   string `bson:"message"`
	Timestamp int64  `bson:"timestamp"`
	Host      string `bson:"host"`
	EventName string `bson:"eventname"`
}

// mongoHandler is a slog.Handler that stores each record in the MongoDB
// `logs` database. Records are buffered in a ring and inserted by a drainer
// goroutine so logging calls do not block on Mongo latency. Fatal/panic
// records are inserted synchronously because the process exits or unwinds
// immediately after the log call.
type mongoHandler struct {
	client     *mongo.Client
	ctx        context.Context
	db         string
	collection string
	level      slog.Level
	attrs      []slog.Attr

	ring   *ring
	wakeCh chan struct{}
}

func newMongoHandler(client *mongo.Client, ctx context.Context, db, collection string, level slog.Level) *mongoHandler {
	h := &mongoHandler{
		client:     client,
		ctx:        ctx,
		db:         db,
		collection: collection,
		level:      level,
		ring:       newRing(mongoRingSize),
		wakeCh:     make(chan struct{}, 1),
	}
	go h.drain()
	return h
}

func (h *mongoHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *mongoHandler) Handle(_ context.Context, r slog.Record) error {
	entry := logEntry{
		Level:     levelString(r.Level),
		Message:   r.Message,
		Timestamp: r.Time.UnixMilli(),
		Host:      config.ProviderConfig.Nickname,
		EventName: "unknown",
	}
	apply := func(a slog.Attr) {
		if a.Key == "event" {
			entry.EventName = a.Value.String()
		}
	}
	for _, a := range h.attrs {
		apply(a)
	}
	r.Attrs(func(a slog.Attr) bool { apply(a); return true })

	if r.Level >= LevelFatal {
		return h.insert(entry)
	}

	h.ring.push(entry)
	select {
	case h.wakeCh <- struct{}{}:
	default:
	}
	return nil
}

func (h *mongoHandler) WithAttrs(as []slog.Attr) slog.Handler {
	out := *h
	out.attrs = append(append([]slog.Attr(nil), h.attrs...), as...)
	return &out
}

func (h *mongoHandler) WithGroup(name string) slog.Handler {
	return h
}

func (h *mongoHandler) drain() {
	for range h.wakeCh {
		h.flush()
	}
}

// flush inserts all currently buffered entries into MongoDB.
func (h *mongoHandler) flush() {
	for {
		entry, ok := h.ring.pop()
		if !ok {
			return
		}
		h.insert(entry)
	}
}

func (h *mongoHandler) insert(entry logEntry) error {
	_, err := h.client.Database(h.db).Collection(h.collection).InsertOne(h.ctx, entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "MongoDB log handler failed to insert log - %s\n", err)
	}
	return err
}

// multiHandler fans a record out to all handlers. Errors from any handler are
// returned as the first non-nil error; subsequent handlers still run.
type multiHandler []slog.Handler

func (m multiHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range m {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (m multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (m multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make(multiHandler, len(m))
	for i, h := range m {
		out[i] = h.WithAttrs(attrs)
	}
	return out
}

func (m multiHandler) WithGroup(name string) slog.Handler {
	out := make(multiHandler, len(m))
	for i, h := range m {
		out[i] = h.WithGroup(name)
	}
	return out
}

// ring is a fixed-capacity circular buffer of log entries. When full, pushing
// evicts the oldest entry to make room.
type ring struct {
	mu   sync.Mutex
	data []logEntry
	head int
	tail int
	size int
	cap  int
}

func newRing(capacity int) *ring {
	return &ring{data: make([]logEntry, capacity), cap: capacity}
}

func (r *ring) push(entry logEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size == r.cap {
		// evict oldest
		r.head = (r.head + 1) % r.cap
		r.size--
	}
	r.data[r.tail] = entry
	r.tail = (r.tail + 1) % r.cap
	r.size++
}

func (r *ring) pop() (logEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size == 0 {
		return logEntry{}, false
	}
	entry := r.data[r.head]
	r.head = (r.head + 1) % r.cap
	r.size--
	return entry, true
}
