package output

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/sonemaro/scanitor/pkg/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements logger.Logger interface for testing
type mockLogger struct {
	logs []string
}

func (m *mockLogger) Info(msg string)                               { m.logs = append(m.logs, "INFO: "+msg) }
func (m *mockLogger) Debug(msg string)                              { m.logs = append(m.logs, "DEBUG: "+msg) }
func (m *mockLogger) Error(msg string)                              { m.logs = append(m.logs, "ERROR: "+msg) }
func (m *mockLogger) Warn(msg string)                               { m.logs = append(m.logs, "WARN: "+msg) }
func (m *mockLogger) Trace(msg string)                              { m.logs = append(m.logs, "TRACE: "+msg) }
func (m *mockLogger) WithFields(fields logger.Fields) logger.Logger { return m }

func createTestTree() *scanner.Node {
	modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	return &scanner.Node{
		Name:    "root",
		Path:    "/root",
		Type:    scanner.DirNode,
		Size:    4096,
		ModTime: modTime,
		Children: []*scanner.Node{
			{
				Name:    "dir1",
				Path:    "/root/dir1",
				Type:    scanner.DirNode,
				Size:    4096,
				ModTime: modTime,
				Children: []*scanner.Node{
					{
						Name:    "file1.txt",
						Path:    "/root/dir1/file1.txt",
						Type:    scanner.FileNode,
						Size:    100,
						ModTime: modTime,
					},
					{
						Name:    "file2.json",
						Path:    "/root/dir1/file2.json",
						Type:    scanner.FileNode,
						Size:    200,
						ModTime: modTime,
					},
				},
			},
			{
				Name:    "dir2",
				Path:    "/root/dir2",
				Type:    scanner.DirNode,
				Size:    4096,
				ModTime: modTime,
				Children: []*scanner.Node{
					{
						Name:    "link1",
						Path:    "/root/dir2/link1",
						Type:    scanner.SymlinkNode,
						Size:    100,
						ModTime: modTime,
					},
				},
			},
			{
				Name:    "file3.txt",
				Path:    "/root/file3.txt",
				Type:    scanner.FileNode,
				Size:    300,
				ModTime: modTime,
			},
		},
	}
}

func TestFormatter(t *testing.T) {
	tests := []struct {
		name       string
		format     Format
		withStats  bool
		withColors bool
		verify     func(*testing.T, string, *mockLogger)
	}{
		{
			name:       "tree format basic",
			format:     FormatTree,
			withStats:  false,
			withColors: false,
			verify: func(t *testing.T, output string, log *mockLogger) {
				// For root node, we don't expect "└── " prefix at the start
				assert.Contains(t, output, "root/")
				assert.Contains(t, output, "├── dir1")
				assert.Contains(t, output, "│   ├── file1.txt")
				assert.Contains(t, output, "│   └── file2.json")
				assert.Contains(t, output, "├── dir2")
				assert.Contains(t, output, "│   └── link1")
				assert.Contains(t, output, "└── file3.txt")
			},
		},
		{
			name:       "tree format with colors",
			format:     FormatTree,
			withStats:  false,
			withColors: true,
			verify: func(t *testing.T, output string, log *mockLogger) {
				assert.Contains(t, output, "\x1b[34;1m") // Bold blue for directories
				assert.Contains(t, output, "\x1b[36m")   // Cyan for symlinks
				assert.Contains(t, output, "\x1b[0m")    // Reset
				assert.Contains(t, log.logs, "DEBUG: Applying color formatting")
			},
		},
		{
			name:      "tree format with stats",
			format:    FormatTree,
			withStats: true,
			verify: func(t *testing.T, output string, log *mockLogger) {
				assert.Contains(t, output, "Total Files:")
				assert.Contains(t, output, "Total Size:")
				assert.Contains(t, output, "Directories:")
				assert.Contains(t, log.logs, "DEBUG: Adding statistics to output")
			},
		},
		{
			name:   "json format",
			format: FormatJSON,
			verify: func(t *testing.T, output string, log *mockLogger) {
				assert.Contains(t, output, `"name": "root"`)
				assert.Contains(t, output, `"type": "directory"`)
				assert.Contains(t, output, `"children"`)
				assert.Contains(t, log.logs, "DEBUG: Formatting JSON output")
			},
		},
		{
			name:   "yaml format",
			format: FormatYAML,
			verify: func(t *testing.T, output string, log *mockLogger) {
				assert.Contains(t, output, "name: root")
				assert.Contains(t, output, "type: directory")
				assert.Contains(t, output, "children:")
				assert.Contains(t, log.logs, "DEBUG: Formatting YAML output")
			},
		},
		{
			name:   "json format with symlinks",
			format: FormatJSON,
			verify: func(t *testing.T, output string, log *mockLogger) {
				assert.Contains(t, output, `"type": "symlink"`)
				assert.Contains(t, log.logs, "DEBUG: Processing symlink")
			},
		},
		{
			name:      "json format with stats",
			format:    FormatJSON,
			withStats: true,
			verify: func(t *testing.T, output string, log *mockLogger) {
				assert.Contains(t, output, `"statistics"`)
				assert.Contains(t, output, `"totalFiles"`)
				assert.Contains(t, log.logs, "DEBUG: Adding statistics to JSON output")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &mockLogger{}

			formatter := NewFormatter(Config{
				Format:     tt.format,
				WithStats:  tt.withStats,
				WithColors: tt.withColors,
			}, log)

			tree := createTestTree()
			output, err := formatter.Format(tree)

			require.NoError(t, err)
			require.NotEmpty(t, output)

			tt.verify(t, output, log)
		})
	}
}

func TestFormatterEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		tree      *scanner.Node
		format    Format
		wantErr   bool
		errString string
	}{
		{
			name:      "nil tree",
			tree:      nil,
			format:    FormatTree,
			wantErr:   true,
			errString: "nil tree",
		},
		{
			name: "empty directory",
			tree: &scanner.Node{
				Name: "empty",
				Type: scanner.DirNode,
			},
			format:  FormatTree,
			wantErr: false,
		},
		{
			name:      "invalid format",
			tree:      createTestTree(),
			format:    "invalid",
			wantErr:   true,
			errString: "unsupported format",
		},
		{
			name: "deep nesting",
			tree: func() *scanner.Node {
				node := &scanner.Node{Name: "root", Type: scanner.DirNode}
				current := node
				for i := 0; i < 100; i++ {
					child := &scanner.Node{
						Name: fmt.Sprintf("level%d", i),
						Type: scanner.DirNode,
					}
					current.Children = []*scanner.Node{child}
					current = child
				}
				return node
			}(),
			format:  FormatTree,
			wantErr: false,
		},
		{
			name: "large number of siblings",
			tree: func() *scanner.Node {
				node := &scanner.Node{Name: "root", Type: scanner.DirNode}
				children := make([]*scanner.Node, 1000)
				for i := 0; i < 1000; i++ {
					children[i] = &scanner.Node{
						Name: fmt.Sprintf("file%d", i),
						Type: scanner.FileNode,
					}
				}
				node.Children = children
				return node
			}(),
			format:  FormatTree,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &mockLogger{}
			formatter := NewFormatter(Config{Format: tt.format}, log)

			output, err := formatter.Format(tt.tree)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errString)

				// Update error log verification
				hasError := false
				for _, logMsg := range log.logs {
					if strings.HasPrefix(logMsg, "ERROR: ") {
						hasError = true
						break
					}
				}
				assert.True(t, hasError, "Expected error log message not found")
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, output)
			}
		})
	}
}
