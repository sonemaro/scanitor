package progress

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLogger struct {
	logs []string
}

func (m *mockLogger) Info(msg string)                               { m.logs = append(m.logs, "INFO: "+msg) }
func (m *mockLogger) Debug(msg string)                              { m.logs = append(m.logs, "DEBUG: "+msg) }
func (m *mockLogger) Error(msg string)                              { m.logs = append(m.logs, "ERROR: "+msg) }
func (m *mockLogger) Warn(msg string)                               { m.logs = append(m.logs, "WARN: "+msg) }
func (m *mockLogger) Trace(msg string)                              { m.logs = append(m.logs, "TRACE: "+msg) }
func (m *mockLogger) WithFields(fields logger.Fields) logger.Logger { return m }

type testWriter struct {
	buffer      bytes.Buffer
	supportAnsi bool
	mu          sync.Mutex
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.Write(p)
}

func (w *testWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

func (w *testWriter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer.Reset()
}

func (w *testWriter) Fd() uintptr { return 0 }

func TestProgress(t *testing.T) {
	t.Log("Starting progress tests")

	tests := []struct {
		name       string
		config     Config
		operations func(*testing.T, Progress, chan struct{})
		verify     func(*testing.T, *testWriter, *mockLogger)
	}{
		{
			name: "basic progress bar",
			config: Config{
				Style:       StyleBar,
				Width:       50,
				ShowStats:   false,
				NoColor:     true,
				RefreshRate: 10 * time.Millisecond,
			},
			operations: func(t *testing.T, p Progress, done chan struct{}) {
				defer close(done)

				p.Start("Starting operation...")
				time.Sleep(20 * time.Millisecond)

				p.Update(Status{
					Current:        50,
					Total:          100,
					CurrentItem:    "file.txt",
					ItemsProcessed: 50,
				})
				time.Sleep(20 * time.Millisecond)

				p.Complete("Operation completed")
				time.Sleep(20 * time.Millisecond)
			},
			verify: func(t *testing.T, w *testWriter, log *mockLogger) {
				output := w.String()
				for _, msg := range log.logs {
					t.Logf("  %s", msg)
				}

				assert.Contains(t, output, "50%", "Should contain progress percentage")
				assert.Contains(t, output, "file.txt", "Should contain current file")
				assert.Contains(t, output, "Operation completed", "Should contain completion message")
			},
		},
		{
			name: "simple progress",
			config: Config{
				Style:       StyleSimple,
				ShowStats:   true,
				NoColor:     true,
				RefreshRate: 10 * time.Millisecond,
			},
			operations: func(t *testing.T, p Progress, done chan struct{}) {
				defer close(done)

				time.Sleep(20 * time.Millisecond)

				t.Log("Updating progress (75%)...")
				p.Update(Status{
					Current:        75,
					Total:          100,
					CurrentItem:    "file.txt",
					ItemsProcessed: 75,
				})
				time.Sleep(20 * time.Millisecond)

				p.Error("Operation failed")
				time.Sleep(20 * time.Millisecond)
			},
			verify: func(t *testing.T, w *testWriter, log *mockLogger) {
				output := w.String()
				for _, msg := range log.logs {
					t.Logf("  %s", msg)
				}

				assert.Contains(t, output, "75%", "Should contain progress percentage")
				assert.Contains(t, output, "Operation failed", "Should contain error message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("\nStarting test case: %s", tt.name)
			t.Logf("Configuration: %+v", tt.config)

			w := &testWriter{supportAnsi: true}
			log := &mockLogger{}

			t.Log("Creating new progress instance")
			p := New(tt.config, log)
			require.NotNil(t, p, "Progress instance should not be nil")

			p.(*progress).writer = w

			t.Log("Running operations...")
			done := make(chan struct{})

			go tt.operations(t, p, done)

			t.Log("Waiting for operations to complete...")
			select {
			case <-done:
				t.Log("Operations completed successfully")
				time.Sleep(50 * time.Millisecond)
			case <-time.After(1 * time.Second):
				t.Fatal("Test timeout")
			}

			t.Log("Stopping progress...")
			p.Stop()

			t.Log("Running verification...")
			tt.verify(t, w, log)

			t.Log("Test case completed")
		})
	}
}

func TestProgressEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		setup   func(*progress)
		verify  func(*testing.T, *progress, *testWriter)
		wantErr bool
	}{
		{
			name: "zero total",
			config: Config{
				Style:       StyleBar,
				RefreshRate: time.Millisecond * 10,
			},
			setup: func(p *progress) {
				p.Start("Starting...")
				time.Sleep(time.Millisecond * 20)
				p.Update(Status{Current: 50, Total: 0})
				time.Sleep(time.Millisecond * 20)
			},
			verify: func(t *testing.T, p *progress, w *testWriter) {
				output := w.String()
				assert.Contains(t, output, "0%", "Should show 0% for zero total")
			},
		},
		{
			name: "current exceeds total",
			config: Config{
				Style:       StyleBar,
				RefreshRate: time.Millisecond * 10,
			},
			setup: func(p *progress) {
				p.Start("Starting...")
				time.Sleep(time.Millisecond * 20)
				p.Update(Status{Current: 150, Total: 100})
				time.Sleep(time.Millisecond * 20)
			},
			verify: func(t *testing.T, p *progress, w *testWriter) {
				output := w.String()
				assert.Contains(t, output, "100%", "Should show 100% when current exceeds total")
			},
		},
		{
			name: "rapid updates",
			config: Config{
				Style:       StyleBar,
				RefreshRate: time.Millisecond * 50,
			},
			setup: func(p *progress) {
				p.Start("Starting...")
				for i := 0; i < 100; i++ {
					p.Update(Status{Current: int64(i), Total: 100})
				}
			},
			verify: func(t *testing.T, p *progress, w *testWriter) {
				assert.NotEmpty(t, w.String())
			},
		},
		{
			name: "terminal width adjustment",
			config: Config{
				Style: StyleBar,
				Width: 0,
			},
			setup: func(p *progress) {
				p.Start("Starting...")
				p.Update(Status{Current: 50, Total: 100})
			},
			verify: func(t *testing.T, p *progress, w *testWriter) {
				assert.NotEmpty(t, w.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &testWriter{supportAnsi: true}
			log := &mockLogger{}

			p := New(tt.config, log).(*progress)
			p.writer = w

			tt.setup(p)
			time.Sleep(time.Millisecond * 50)
			tt.verify(t, p, w)
			p.Stop()
		})
	}
}

func captureOutput(p Progress, duration time.Duration) string {
	w := &testWriter{supportAnsi: true}
	p.(*progress).writer = w

	time.Sleep(duration)
	return w.String()
}

func createTestStatus(current, total int64) Status {
	return Status{
		Current:        current,
		Total:          total,
		CurrentItem:    "test.file",
		ItemsProcessed: current,
		BytesRead:      current * 1024,
		StartTime:      time.Now().Add(-time.Second),
	}
}
