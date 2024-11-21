package scanner

import (
	"fmt"
	"sync"

	"github.com/spf13/afero"
)

// SymlinkFs extends afero.Fs with symlink support
type SymlinkFs interface {
	afero.Fs
	ReadlinkIfPossible(name string) (string, error)
}

// BasicSymlinkFs implements SymlinkFs with no symlink support
type BasicSymlinkFs struct {
	afero.Fs
}

// NewScannerStats creates and initializes a new ScannerStats instance
func NewScannerStats() *ScannerStats {
	stats := &ScannerStats{}
	// initialize atomic values to 0 (though they're automatically initialized xD)
	stats.filesFound.Store(0)
	stats.filesScanned.Store(0)
	stats.bytesRead.Store(0)
	stats.currentDepth.Store(0)
	stats.directoriesScanned.Store(0)

	return stats
}

func (s *ScannerStats) AddFilesFound(delta int64) int64 {
	return s.filesFound.Add(delta)
}

func (s *ScannerStats) AddFilesScanned(delta int64) int64 {
	return s.filesScanned.Add(delta)
}

func (s *ScannerStats) AddDirectoriesScanned(delta int64) int64 {
	return s.directoriesScanned.Add(delta)
}

func (s *ScannerStats) AddBytesRead(delta int64) int64 {
	return s.bytesRead.Add(delta)
}

func (s *ScannerStats) SetCurrentDepth(depth int32) int32 {
	return s.currentDepth.Swap(depth)
}

func (s *ScannerStats) GetFilesFound() int64 {
	return s.filesFound.Load()
}

func (s *ScannerStats) GetFilesScanned() int64 {
	return s.filesScanned.Load()
}

func (s *ScannerStats) GetBytesRead() int64 {
	return s.bytesRead.Load()
}

func (s *ScannerStats) GetCurrentDepth() int32 {
	return s.currentDepth.Load()
}

// ReadlinkIfPossible implements SymlinkFs for BasicSymlinkFs
func (fs *BasicSymlinkFs) ReadlinkIfPossible(name string) (string, error) {
	return "", fmt.Errorf("symlinks not supported")
}

// TestSymlinkFs implements SymlinkFs for testing
type TestSymlinkFs struct {
	afero.Fs
	symlinks map[string]string
	mu       sync.RWMutex
}

// NewTestSymlinkFs creates a new TestSymlinkFs
func NewTestSymlinkFs(fs afero.Fs) *TestSymlinkFs {
	return &TestSymlinkFs{
		Fs:       fs,
		symlinks: make(map[string]string),
	}
}

// ReadlinkIfPossible implements SymlinkFs for TestSymlinkFs
func (fs *TestSymlinkFs) ReadlinkIfPossible(name string) (string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	if target, ok := fs.symlinks[name]; ok {
		return target, nil
	}

	return "", fmt.Errorf("not a symlink")
}

// CreateSymlink creates a new symlink for testing
func (fs *TestSymlinkFs) CreateSymlink(source, target string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.symlinks[target] = source
	// Copy the content to make it accessible
	content, err := afero.ReadFile(fs.Fs, source)
	if err != nil {
		return err
	}

	return afero.WriteFile(fs.Fs, target, content, 0644)
}
