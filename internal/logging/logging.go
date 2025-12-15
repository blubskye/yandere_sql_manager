// YSM - Yandere SQL Manager
// Copyright (C) 2025 blubskye
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// Source code: https://github.com/blubskye/yandere_sql_manager

package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents the logging level
type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
	LevelTrace
)

func (l Level) String() string {
	switch l {
	case LevelError:
		return "ERROR"
	case LevelWarn:
		return "WARN"
	case LevelInfo:
		return "INFO"
	case LevelDebug:
		return "DEBUG"
	case LevelTrace:
		return "TRACE"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging with levels and optional stack traces
type Logger struct {
	mu            sync.Mutex
	level         Level
	output        io.Writer
	logFile       *os.File
	showTimestamp bool
	showCaller    bool
	stackOnError  bool
	prefix        string
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Default returns the default logger instance
func Default() *Logger {
	once.Do(func() {
		defaultLogger = &Logger{
			level:         LevelInfo,
			output:        os.Stderr,
			showTimestamp: true,
			showCaller:    false,
			stackOnError:  false,
		}
	})
	return defaultLogger
}

// New creates a new logger
func New() *Logger {
	return &Logger{
		level:         LevelInfo,
		output:        os.Stderr,
		showTimestamp: true,
		showCaller:    false,
		stackOnError:  false,
	}
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetLevelFromString sets the logging level from a string
func (l *Logger) SetLevelFromString(level string) error {
	switch strings.ToLower(level) {
	case "error":
		l.SetLevel(LevelError)
	case "warn", "warning":
		l.SetLevel(LevelWarn)
	case "info":
		l.SetLevel(LevelInfo)
	case "debug":
		l.SetLevel(LevelDebug)
	case "trace":
		l.SetLevel(LevelTrace)
	default:
		return fmt.Errorf("unknown log level: %s", level)
	}
	return nil
}

// SetOutput sets the output writer
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// SetLogFile sets a file for logging output
func (l *Logger) SetLogFile(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Close existing file if any
	if l.logFile != nil {
		l.logFile.Close()
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.logFile = f
	l.output = io.MultiWriter(os.Stderr, f)
	return nil
}

// EnableTimestamp enables/disables timestamps in log output
func (l *Logger) EnableTimestamp(enable bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.showTimestamp = enable
}

// EnableCaller enables/disables caller information in log output
func (l *Logger) EnableCaller(enable bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.showCaller = enable
}

// EnableStackOnError enables/disables stack traces on errors
func (l *Logger) EnableStackOnError(enable bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stackOnError = enable
}

// SetPrefix sets a prefix for all log messages
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

// Close closes the logger and any open files
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logFile != nil {
		err := l.logFile.Close()
		l.logFile = nil
		return err
	}
	return nil
}

// log is the internal logging function
func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level > l.level {
		return
	}

	var b strings.Builder

	// Timestamp
	if l.showTimestamp {
		b.WriteString(time.Now().Format("2006-01-02 15:04:05.000"))
		b.WriteString(" ")
	}

	// Level
	b.WriteString("[")
	b.WriteString(level.String())
	b.WriteString("] ")

	// Prefix
	if l.prefix != "" {
		b.WriteString("[")
		b.WriteString(l.prefix)
		b.WriteString("] ")
	}

	// Caller
	if l.showCaller {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			b.WriteString(filepath.Base(file))
			b.WriteString(":")
			b.WriteString(fmt.Sprintf("%d", line))
			b.WriteString(" ")
		}
	}

	// Message
	if len(args) > 0 {
		b.WriteString(fmt.Sprintf(format, args...))
	} else {
		b.WriteString(format)
	}
	b.WriteString("\n")

	// Stack trace for errors
	if level == LevelError && l.stackOnError {
		b.WriteString(GetStackTrace(3))
		b.WriteString("\n")
	}

	fmt.Fprint(l.output, b.String())
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

// Trace logs a trace message
func (l *Logger) Trace(format string, args ...interface{}) {
	l.log(LevelTrace, format, args...)
}

// ErrorWithStack logs an error with a stack trace regardless of settings
func (l *Logger) ErrorWithStack(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if LevelError > l.level {
		return
	}

	var b strings.Builder

	if l.showTimestamp {
		b.WriteString(time.Now().Format("2006-01-02 15:04:05.000"))
		b.WriteString(" ")
	}

	b.WriteString("[ERROR] ")

	if l.prefix != "" {
		b.WriteString("[")
		b.WriteString(l.prefix)
		b.WriteString("] ")
	}

	if len(args) > 0 {
		b.WriteString(fmt.Sprintf(format, args...))
	} else {
		b.WriteString(format)
	}
	b.WriteString("\n")
	b.WriteString(GetStackTrace(2))
	b.WriteString("\n")

	fmt.Fprint(l.output, b.String())
}

// GetStackTrace returns a formatted stack trace
func GetStackTrace(skip int) string {
	var b strings.Builder
	b.WriteString("Stack trace:\n")

	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip+1, pcs)
	frames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := frames.Next()

		// Skip runtime internals
		if strings.Contains(frame.File, "runtime/") {
			if !more {
				break
			}
			continue
		}

		b.WriteString(fmt.Sprintf("  %s\n", frame.Function))
		b.WriteString(fmt.Sprintf("    %s:%d\n", frame.File, frame.Line))

		if !more {
			break
		}
	}

	return b.String()
}

// Package-level convenience functions using the default logger

// SetLevel sets the default logger level
func SetLevel(level Level) {
	Default().SetLevel(level)
}

// SetLevelFromString sets the default logger level from a string
func SetLevelFromString(level string) error {
	return Default().SetLevelFromString(level)
}

// EnableDebug enables debug logging on the default logger
func EnableDebug() {
	Default().SetLevel(LevelDebug)
	Default().EnableCaller(true)
}

// EnableTrace enables trace logging on the default logger
func EnableTrace() {
	Default().SetLevel(LevelTrace)
	Default().EnableCaller(true)
}

// EnableStackOnError enables stack traces on errors for the default logger
func EnableStackOnError() {
	Default().EnableStackOnError(true)
}

// SetLogFile sets a log file for the default logger
func SetLogFile(path string) error {
	return Default().SetLogFile(path)
}

// Error logs an error using the default logger
func Error(format string, args ...interface{}) {
	Default().Error(format, args...)
}

// Warn logs a warning using the default logger
func Warn(format string, args ...interface{}) {
	Default().Warn(format, args...)
}

// Info logs info using the default logger
func Info(format string, args ...interface{}) {
	Default().Info(format, args...)
}

// Debug logs debug using the default logger
func Debug(format string, args ...interface{}) {
	Default().Debug(format, args...)
}

// Trace logs trace using the default logger
func Trace(format string, args ...interface{}) {
	Default().Trace(format, args...)
}

// ErrorWithStack logs an error with stack trace using the default logger
func ErrorWithStack(format string, args ...interface{}) {
	Default().ErrorWithStack(format, args...)
}

// IsDebugEnabled returns true if debug logging is enabled
func IsDebugEnabled() bool {
	return Default().level >= LevelDebug
}

// IsTraceEnabled returns true if trace logging is enabled
func IsTraceEnabled() bool {
	return Default().level >= LevelTrace
}
