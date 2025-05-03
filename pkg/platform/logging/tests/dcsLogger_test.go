package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanehh/boiler/pkg/platform/logging"
)

func TestLoggerBasic(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Create logger with buffer as output
	config := logging.DefaultConfig()
	config.Output = &buf
	logger := logging.New(config)

	// Log a message
	logger.Info("test message", "key", "value")

	// Check if output contains expected string
	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected output to contain 'test message', got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("Expected output to contain 'key=value', got: %s", output)
	}
}

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer

	config := logging.DefaultConfig()
	config.Output = &buf
	config.Level = logging.InfoLevel
	logger := logging.New(config)

	// Debug shouldn't be logged with INFO level
	buf.Reset()
	logger.Debug("debug message")
	if buf.Len() > 0 {
		t.Errorf("Debug message should not be logged with INFO level: %s", buf.String())
	}

	// Info should be logged
	buf.Reset()
	logger.Info("info message")
	if !strings.Contains(buf.String(), "info message") {
		t.Errorf("Info message should be logged with INFO level")
	}

	// Change level to DEBUG
	config.Level = logging.DebugLevel
	logger.UpdateConfig(config)

	// Now debug should be logged
	buf.Reset()
	logger.Debug("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Errorf("Debug message should be logged with DEBUG level")
	}
}

func TestMultiWriterLogger(t *testing.T) {
	// Create buffers to capture outputs
	var consoleBuf, fileBuf bytes.Buffer

	// Configure multi-writer logger
	config := logging.DefaultConfig()
	config.Level = logging.DebugLevel

	// Configure main output as console (text format)
	config.Output = &consoleBuf
	config.JSONFormat = false

	// Add JSON file as additional output
	config.AdditionalOutputs = []logging.OutputConfig{
		{
			Writer:     &fileBuf,
			JSONFormat: true,
		},
	}

	// Create logger
	logger := logging.New(config)

	// Log messages
	logger.Info("multi-writer test", "attribute", "value")
	logger.Debug("debug message", "number", 42)

	// Verify console output (text format)
	consoleOutput := consoleBuf.String()
	if !strings.Contains(consoleOutput, "multi-writer test") {
		t.Errorf("Console output missing info message: %s", consoleOutput)
	}
	if !strings.Contains(consoleOutput, "attribute=value") {
		t.Errorf("Console output missing attribute: %s", consoleOutput)
	}
	if !strings.Contains(consoleOutput, "debug message") {
		t.Errorf("Console output missing debug message: %s", consoleOutput)
	}
	if !strings.Contains(consoleOutput, "number=42") {
		t.Errorf("Console output missing number attribute: %s", consoleOutput)
	}

	// Verify file output (JSON format)
	fileOutput := fileBuf.String()

	// Split JSON lines and parse each
	jsonLines := strings.Split(strings.TrimSpace(fileOutput), "\n")
	if len(jsonLines) != 2 {
		t.Errorf("Expected 2 JSON log entries, got %d", len(jsonLines))
	}

	// Parse first JSON line (info message)
	var infoEntry map[string]interface{}
	if err := json.Unmarshal([]byte(jsonLines[0]), &infoEntry); err != nil {
		t.Errorf("Failed to parse JSON log entry: %v", err)
	}

	// Check JSON fields
	if msg, ok := infoEntry["msg"].(string); !ok || msg != "multi-writer test" {
		t.Errorf("JSON log entry missing or incorrect 'msg' field: %v", infoEntry)
	}
	if attr, ok := infoEntry["attribute"].(string); !ok || attr != "value" {
		t.Errorf("JSON log entry missing or incorrect 'attribute' field: %v", infoEntry)
	}

	// Parse second JSON line (debug message)
	var debugEntry map[string]interface{}
	if err := json.Unmarshal([]byte(jsonLines[1]), &debugEntry); err != nil {
		t.Errorf("Failed to parse JSON log entry: %v", err)
	}

	// Check level field is "DEBUG"
	if level, ok := debugEntry["level"].(string); !ok || level != "DEBUG" {
		t.Errorf("JSON log entry has incorrect level: %v", debugEntry)
	}
}

func TestLoggerToFile(t *testing.T) {
	// Create temporary file
	tempDir := t.TempDir()
	logFilePath := filepath.Join(tempDir, "test.log")
	jsonFilePath := filepath.Join(tempDir, "test.json")

	// Create log files
	logFile, err := os.Create(logFilePath)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	defer logFile.Close()

	jsonFile, err := os.Create(jsonFilePath)
	if err != nil {
		t.Fatalf("Failed to create JSON file: %v", err)
	}
	defer jsonFile.Close()

	// Configure multi-writer logger
	config := logging.DefaultConfig()
	config.Level = logging.InfoLevel

	// Output to console and both files
	var consoleBuf bytes.Buffer
	config.Output = &consoleBuf
	config.JSONFormat = false

	config.AdditionalOutputs = []logging.OutputConfig{
		{
			Writer:     logFile,
			JSONFormat: false, // Text format
		},
		{
			Writer:     jsonFile,
			JSONFormat: true, // JSON format
		},
	}

	// Create logger
	logger := logging.New(config)

	// Log a message
	logger.Info("file test", "path", logFilePath)

	// Close files to flush buffers
	logFile.Close()
	jsonFile.Close()

	// Read back and verify log file content
	logContent, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(logContent), "file test") {
		t.Errorf("Log file doesn't contain expected message: %s", string(logContent))
	}

	// Read back and verify JSON file content
	jsonContent, err := os.ReadFile(jsonFilePath)
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}

	var jsonEntry map[string]any
	if err := json.Unmarshal(jsonContent, &jsonEntry); err != nil {
		t.Errorf("Failed to parse JSON log file: %v", err)
	}

	if msg, ok := jsonEntry["msg"].(string); !ok || msg != "file test" {
		t.Errorf("JSON log file missing or incorrect 'msg' field: %v", jsonEntry)
	}
}

func TestLoggerWith(t *testing.T) {
	var buf bytes.Buffer

	config := logging.DefaultConfig()
	config.Output = &buf
	logger := logging.New(config)

	// Create a derived logger with additional context
	derivedLogger := logger.With("context", "test", "request_id", "123")

	// Log with the derived logger
	derivedLogger.Info("contextualized message")

	output := buf.String()
	if !strings.Contains(output, "context=test") {
		t.Errorf("Expected output to contain context, got: %s", output)
	}
	if !strings.Contains(output, "request_id=123") {
		t.Errorf("Expected output to contain request_id, got: %s", output)
	}
}
