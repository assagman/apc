package logger

import (
	"fmt"
	"io"
	"os"
	"time"
)

// LogLevel represents the severity of the log message
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarning
	LevelError
	LevelCritical
)

// Color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorBlue   = "\033[34m"
	colorYellow = "\033[33m"
	colorOrange = "\033[38;5;208m" // ANSI code for orange
	colorRed    = "\033[31m"
)

// Logger is a simple custom logger
type Logger struct {
	writer io.Writer
}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	return &Logger{
		writer: os.Stdout,
	}
}

// log writes a log message with timestamp, severity, and color
func (l *Logger) log(level LogLevel, msg string, args ...any) {
	// Format the message if there are arguments
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}

	// Get current timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Set color and severity text based on level
	var color, severity string
	switch level {
	case LevelDebug:
		color = colorGreen
		severity = "DEBUG"
	case LevelInfo:
		color = colorBlue
		severity = "INFO"
	case LevelWarning:
		color = colorYellow
		severity = "WARNING"
	case LevelError:
		color = colorOrange
		severity = "ERROR"
	case LevelCritical:
		color = colorRed
		severity = "CRITICAL"
	default:
		color = colorReset
		severity = "UNKNOWN"
	}

	// Format and write the log message
	logLine := fmt.Sprintf("%s[%s] [%s] %s%s\n", color, timestamp, severity, msg, colorReset)
	fmt.Fprint(l.writer, logLine)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.log(LevelDebug, msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.log(LevelInfo, msg, args...)
}

func (l *Logger) Warning(msg string, args ...any) {
	l.log(LevelWarning, msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.log(LevelError, msg, args...)
}

func (l *Logger) Critical(msg string, args ...any) {
	l.log(LevelCritical, msg, args...)
}

func (l *Logger) Writer() io.Writer {
	return l.writer
}

func (l *Logger) SetWriter(w io.Writer) {
	l.writer = w
}

var L = NewLogger()

func Debug(msg string, args ...any)    { L.Debug(msg, args...) }
func Info(msg string, args ...any)     { L.Info(msg, args...) }
func Warning(msg string, args ...any)  { L.Warning(msg, args...) }
func Error(msg string, args ...any)    { L.Error(msg, args...) }
func Critical(msg string, args ...any) { L.Critical(msg, args...) }
