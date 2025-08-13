package logger_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/assagman/apc/internal/logger"
)

// switch writer temporarily from stdout to buffer to capture output
func captureOutput(f func()) string {
	oldWriter := logger.L.Writer()
	defer func() {
		logger.L.SetWriter(oldWriter)
	}()

	var buf bytes.Buffer
	logger.L.SetWriter(&buf)

	f()

	return buf.String()
}

func TestLogger_Debug(t *testing.T) {
	output := captureOutput(func() {
		logger.Debug("This is a %s debug", "test")
	})

	if !strings.Contains(output, "DEBUG") {
		t.Error("Expected 'WARNING' in output")
	}
	if !strings.Contains(output, "test debug") {
		t.Error("Expected formatted message to be included")
	}
	if !strings.Contains(output, "\033[32m") { // green
		t.Error("Expected green color for Debug")
	}
}

func TestLogger_Info(t *testing.T) {
	output := captureOutput(func() {
		logger.Info("This is an info message")
	})

	// Check basic structure
	if !strings.Contains(output, "INFO") {
		t.Error("Expected output to contain 'INFO'")
	}
	if !strings.Contains(output, "This is an info message") {
		t.Error("Expected output to contain the message")
	}

	// Check timestamp format (roughly)
	now := time.Now().Format("2006-01-02")
	if !strings.Contains(output, now) {
		t.Error("Expected output to contain today's date")
	}

	// Check color codes
	if !strings.Contains(output, "\033[34m") { // blue
		t.Error("Expected blue color code for Info")
	}
	if !strings.Contains(output, "\033[0m") { // reset
		t.Error("Expected color reset code")
	}
}

func TestLogger_Warning(t *testing.T) {
	output := captureOutput(func() {
		logger.Warning("This is a %s warning", "test")
	})

	if !strings.Contains(output, "WARNING") {
		t.Error("Expected 'WARNING' in output")
	}
	if !strings.Contains(output, "test warning") {
		t.Error("Expected formatted message to be included")
	}
	if !strings.Contains(output, "\033[33m") { // yellow
		t.Error("Expected yellow color for Warning")
	}
}

func TestLogger_Error(t *testing.T) {
	output := captureOutput(func() {
		logger.Error("Something went wrong")
	})

	if !strings.Contains(output, "ERROR") {
		t.Error("Expected 'ERROR' in output")
	}
	if !strings.Contains(output, "\033[38;5;208m") { // orange
		t.Error("Expected orange color for Error")
	}
}

func TestLogger_Critical(t *testing.T) {
	output := captureOutput(func() {
		logger.Critical("Something went wrong")
	})

	if !strings.Contains(output, "CRITICAL") {
		t.Error("Expected 'CRITICAL' in output")
	}
	if !strings.Contains(output, "\033[31m") { // orange
		t.Error("Expected red color for Critical")
	}
}

func TestLogger_Formatting(t *testing.T) {
	output := captureOutput(func() {
		logger.Info("User %s logged in from %s", "alice", "192.168.1.1")
	})

	if !strings.Contains(output, "User alice logged in from 192.168.1.1") {
		t.Error("Expected formatted string to be correctly interpolated")
	}
}
