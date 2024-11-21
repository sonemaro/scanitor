package logger

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

type LogEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

func TestLogger(t *testing.T) {
	tests := []struct {
		name           string
		verbosityLevel int
		logFunc        func(Logger)
		expectedLevel  string
		expectedMsg    string
		shouldLog      bool
	}{
		{
			name:           "info level with default verbosity",
			verbosityLevel: 0,
			logFunc: func(l Logger) {
				l.Info("info message")
			},
			expectedLevel: "info",
			expectedMsg:   "info message",
			shouldLog:     true,
		},
		{
			name:           "debug level with insufficient verbosity",
			verbosityLevel: 0,
			logFunc: func(l Logger) {
				l.Debug("debug message")
			},
			shouldLog: false,
		},
		{
			name:           "debug level with sufficient verbosity",
			verbosityLevel: 1,
			logFunc: func(l Logger) {
				l.Debug("debug message")
			},
			expectedLevel: "debug",
			expectedMsg:   "debug message",
			shouldLog:     true,
		},
		{
			name:           "trace level with sufficient verbosity",
			verbosityLevel: 2,
			logFunc: func(l Logger) {
				l.Trace("trace message")
			},
			expectedLevel: "debug",
			expectedMsg:   "TRACE: trace message",
			shouldLog:     true,
		},
	}

	t.Log("Starting logger tests...")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("\nTest case: %s", tt.name)
			t.Logf("Verbosity level: %d", tt.verbosityLevel)
			t.Logf("Should log: %v", tt.shouldLog)
			if tt.shouldLog {
				t.Logf("Expected level: %s", tt.expectedLevel)
				t.Logf("Expected message: %s", tt.expectedMsg)
			}

			// Create a buffer to capture log output
			var buf bytes.Buffer

			// Create test logger
			t.Log("Creating test logger...")
			logger := NewLogger(Config{
				Verbosity: tt.verbosityLevel,
				Output:    &buf,
			})

			// Execute log function
			t.Log("Executing log function...")
			tt.logFunc(logger)

			// Check output
			if tt.shouldLog {
				t.Log("Checking logged output...")
				var entry LogEntry
				err := json.Unmarshal(buf.Bytes(), &entry)
				if err != nil {
					t.Logf("Error unmarshaling log entry: %v", err)
					t.Logf("Raw buffer content: %s", buf.String())
				}
				assert.NoError(t, err)

				t.Logf("Actual level: %s", entry.Level)
				t.Logf("Actual message: %s", entry.Message)

				assert.Equal(t, tt.expectedLevel, entry.Level)
				assert.Equal(t, tt.expectedMsg, entry.Message)
			} else {
				t.Log("Checking that nothing was logged...")
				assert.Empty(t, buf.String())
			}
		})
	}
}

func TestLoggerWithFields(t *testing.T) {
	t.Log("\nTesting logger with fields...")

	var buf bytes.Buffer
	logger := NewLogger(Config{
		Verbosity: 0,
		Output:    &buf,
	})

	testFields := Fields{
		"key1": "value1",
		"key2": 123,
	}

	t.Logf("Test fields: %+v", testFields)

	logger.WithFields(testFields).Info("test message")

	t.Log("Checking logged output with fields...")
	t.Logf("Raw buffer content: %s", buf.String())

	var entry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &entry)
	if err != nil {
		t.Logf("Error unmarshaling log entry: %v", err)
	}
	assert.NoError(t, err)

	t.Logf("Parsed entry: %+v", entry)

	assert.Equal(t, "value1", entry["key1"])
	assert.Equal(t, float64(123), entry["key2"])
	assert.Equal(t, "test message", entry["message"])
}

// Helper function to print the content of the buffer
func printBuffer(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	content := buf.String()
	if content == "" {
		t.Log("Buffer is empty")
	} else {
		t.Log("Buffer content:")
		t.Log(content)
	}
}
