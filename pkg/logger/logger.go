package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// Logger provides structured logging capabilities
type Logger struct {
	debug bool
}

// New creates a new logger instance
func New(debug bool) *Logger {
	return &Logger{debug: debug}
}

func (l *Logger) logf(level string, format string, args ...interface{}) {
	if level == "DEBUG" && !l.debug {
		return
	}
	timestamp := time.Now().Format(time.RFC3339)
	msg := fmt.Sprintf(format, args...)
	entry := map[string]interface{}{
		"timestamp": timestamp,
		"level":     level,
		"message":   msg,
	}
	jsonBytes, _ := json.Marshal(entry)
	log.Println(string(jsonBytes))
}

// Info logs an info-level message
func (l *Logger) Info(msg string) {
	l.logf("INFO", msg)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.logf("INFO", format, args...)
}

// Error logs an error-level message
func (l *Logger) Error(msg string) {
	l.logf("ERROR", msg)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logf("ERROR", format, args...)
}

// Debug logs a debug-level message (only if debug is enabled)
func (l *Logger) Debug(msg string) {
	l.logf("DEBUG", msg)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.logf("DEBUG", format, args...)
}

// Fatal logs and exits
func (l *Logger) Fatal(msg string) {
	l.logf("FATAL", msg)
	os.Exit(1)
}

// Fatalf logs formatted message and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.logf("FATAL", format, args...)
	os.Exit(1)
}
