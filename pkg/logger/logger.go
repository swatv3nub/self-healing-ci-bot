package logger

import (
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
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] [%s] %s", timestamp, level, msg)
	log.Println(logLine)
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
	if l.debug {
		l.logf("DEBUG", msg)
	}
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.debug {
		l.logf("DEBUG", format, args...)
	}
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
