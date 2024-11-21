package scanner

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFS implements a custom filesystem for testing with symlink support
type testFS struct {
	afero.Fs
	symlinks map[string]string
}

func (fs *testFS) ReadlinkIfPossible(name string) (string, error) {
	if target, ok := fs.symlinks[name]; ok {
		return target, nil
	}
	return "", fmt.Errorf("not a symlink")
}

func (fs *testFS) CreateSymlink(source, target string) error {
	if fs.symlinks == nil {
		fs.symlinks = make(map[string]string)
	}
	fs.symlinks[target] = source
	return nil
}

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

func setupTestFS(t *testing.T) (SymlinkFs, *mockLogger) {
	memFs := afero.NewMemMapFs()
	fs := &TestSymlinkFs{
		Fs:       memFs,
		symlinks: make(map[string]string),
	}
	log := &mockLogger{}

	// Create test directory structure
	files := map[string]string{
		"/root/file1.txt":              "content1",
		"/root/file2.log":              "content2",
		"/root/dir1/file3.txt":         "content3",
		"/root/dir1/file4.json":        `{"key": "value"}`,
		"/root/dir2/file5.txt":         "content5",
		"/root/dir2/subdir/file6.yaml": "key: value",
		"/root/.git/config":            "git config",
		"/root/node_modules/pkg.json":  "package",
	}

	// Create directories first
	for path := range files {
		dir := filepath.Dir(path)
		err := fs.MkdirAll(dir, 0755)
		require.NoError(t, err)
	}

	// Create files with content
	for path, content := range files {
		err := afero.WriteFile(fs, path, []byte(content), 0644)
		require.NoError(t, err)
	}

	return fs, log
}

func TestScanner(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		includeContent bool
		ignorePatterns []string
		followSymlinks bool
		setup          func(afero.Fs) error
		verify         func(*testing.T, Result)
		wantErr        bool
	}{
		{
			name: "basic scan without content",
			config: Config{
				Workers:  4,
				MaxDepth: -1,
			},
			includeContent: false,
			ignorePatterns: nil,
			verify: func(t *testing.T, result Result) {
				assert.Equal(t, "root", result.Root.Name)
				assert.Equal(t, 8, countFiles(result.Root)) // Total regular files
				assert.Equal(t, 6, countDirs(result.Root))  // Total directories
				assert.Nil(t, result.Contents)
			},
		},
		{
			name: "scan with content",
			config: Config{
				Workers:  4,
				MaxDepth: -1,
			},
			includeContent: true,
			ignorePatterns: nil,
			verify: func(t *testing.T, result Result) {
				assert.NotNil(t, result.Contents)
				content, exists := result.Contents["/root/file1.txt"]
				assert.True(t, exists)
				assert.Equal(t, "content1", string(content))
			},
		},
		{
			name: "scan with depth limit",
			config: Config{
				Workers:  4,
				MaxDepth: 1,
			},
			includeContent: false,
			ignorePatterns: nil,
			verify: func(t *testing.T, result Result) {
				assert.Equal(t, 1, getMaxDepth(result.Root))
			},
		},
		{
			name: "scan with ignore patterns",
			config: Config{
				Workers:  4,
				MaxDepth: -1,
			},
			includeContent: false,
			ignorePatterns: []string{".git", "node_modules", "*.log"},
			verify: func(t *testing.T, result Result) {
				assert.Equal(t, 5, countFiles(result.Root)) // Excluding .git and node_modules files
				for _, file := range getAllFiles(result.Root) {
					assert.NotContains(t, file, ".git/")
					assert.NotContains(t, file, "node_modules/")
					assert.NotContains(t, file, ".log")
				}
			},
		},
		{
			name: "scan with permission errors",
			config: Config{
				Workers:  4,
				MaxDepth: -1,
			},
			includeContent: true,
			ignorePatterns: nil,
			setup: func(fs afero.Fs) error {
				// Create a file with no read permissions
				err := afero.WriteFile(fs, "/root/noperm.txt", []byte("secret"), 0000)
				if err != nil {
					return err
				}
				// Verify the file exists but is not readable
				info, err := fs.Stat("/root/noperm.txt")
				if err != nil {
					return err
				}
				if info.Mode().Perm()&0444 != 0 {
					return fmt.Errorf("file permissions not set correctly")
				}
				return nil
			},
			verify: func(t *testing.T, result Result) {
				assert.NotNil(t, result.Root, "Root node should exist")
				assert.Contains(t, result.Errors, "/root/noperm.txt",
					"Should contain error for unreadable file")

				// verify the error message
				if err, ok := result.Errors["/root/noperm.txt"]; ok {
					assert.Contains(t, err.Error(), "permission denied",
						"Error should indicate permission issue")
				}
			},
		},
		{
			name: "scan with symlinks",
			config: Config{
				Workers:  4,
				MaxDepth: -1,
			},
			includeContent: false,
			ignorePatterns: nil,
			followSymlinks: true,
			setup: func(fs afero.Fs) error {
				// Correct type assertion to TestSymlinkFs
				if testFs, ok := fs.(*TestSymlinkFs); ok {
					return testFs.CreateSymlink("/root/file1.txt", "/root/link1.txt")
				}
				return fmt.Errorf("filesystem does not support symlinks")
			},
			verify: func(t *testing.T, result Result) {
				// Verify symlink exists and has correct content
				files := getAllFiles(result.Root)
				assert.Contains(t, files, "link1.txt", "Symlink should be present")

				if result.Contents != nil {
					originalContent := result.Contents["/root/file1.txt"]
					linkContent := result.Contents["/root/link1.txt"]
					assert.NotNil(t, originalContent, "Original file content should exist")
					assert.NotNil(t, linkContent, "Symlink content should exist")
					assert.Equal(t, originalContent, linkContent, "Symlink should have same content as original")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test filesystem
			fs, log := setupTestFS(t)
			if tt.setup != nil {
				err := tt.setup(fs)
				require.NoError(t, err)
			}

			// Create scanner
			scanner := NewScanner(tt.config, fs, log)

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Perform scan
			result, err := scanner.Scan(ctx, "/root", tt.ignorePatterns, tt.includeContent, tt.followSymlinks)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			tt.verify(t, result)

			// Verify logging
			assert.NotEmpty(t, log.logs)
			assert.Contains(t, log.logs[0], "Starting scan")
		})
	}
}

// Helper functions for verification
func countFiles(node *Node) int {
	if node == nil {
		return 0
	}

	count := 0
	if node.Type == FileNode {
		count++
	}

	for _, child := range node.Children {
		count += countFiles(child)
	}
	return count
}

func countDirs(node *Node) int {
	if node == nil {
		return 0
	}

	count := 0
	if node.Type == DirNode {
		count++
	}

	for _, child := range node.Children {
		count += countDirs(child)
	}
	return count
}

func getMaxDepth(node *Node) int {
	if node == nil || len(node.Children) == 0 {
		return 0
	}

	maxChildDepth := 0
	for _, child := range node.Children {
		childDepth := getMaxDepth(child)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}
	return maxChildDepth + 1
}

func getAllFiles(node *Node) []string {
	var files []string
	if node == nil {
		return files
	}

	if node.Type == FileNode || node.Type == SymlinkNode {
		files = append(files, node.Name)
	}

	for _, child := range node.Children {
		files = append(files, getAllFiles(child)...)
	}
	return files
}
