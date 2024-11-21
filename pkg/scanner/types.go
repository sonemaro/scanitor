package scanner

import (
	"sync/atomic"
	"time"

	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/sonemaro/scanitor/pkg/worker"
)

// NodeType represents the type of filesystem node
type NodeType int

const (
	// FileNode represents a regular file
	FileNode NodeType = iota
	// DirNode represents a directory
	DirNode
	// SymlinkNode represents a symbolic link
	SymlinkNode
)

// Node represents a filesystem node in the scanned tree
type Node struct {
	Name     string
	Path     string
	Type     NodeType
	Size     int64
	Mode     uint32
	ModTime  time.Time
	Children []*Node
}

// Result contains the complete scan results
type Result struct {
	Root     *Node
	Contents map[string][]byte
	Errors   map[string]error
	Stats    ScanStats
}

// ScanStats contains statistics about the scanning operation
type ScanStats struct {
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	TotalFiles   int64
	TotalDirs    int64
	TotalSize    int64
	ErrorCount   int
	SkippedFiles int64
}

// Config contains scanner configuration options
type Config struct {
	Workers    int
	MaxDepth   int
	BufferSize int
	RateLimit  int
}

// Progress represents the current progress of the scanning operation
type Progress struct {
	CurrentPath    string
	TotalFiles     int64
	ProcessedFiles int64
	CurrentDepth   int
	StartTime      time.Time
	BytesRead      int64
}

// scanResult represents the result of processing a single file
type scanResult struct {
	path    string
	content []byte
	err     error
}

// ScannerStats holds the atomic counters for scanner statistics
type ScannerStats struct {
	filesFound         atomic.Int64
	filesScanned       atomic.Int64
	directoriesScanned atomic.Int64
	bytesRead          atomic.Int64
	currentDepth       atomic.Int32
}

// BatchConfig holds configuration for batch processor
type BatchConfig struct {
	BatchSize      int
	NodesChan      chan<- *Node
	ErrorsChan     chan<- error
	Logger         logger.Logger
	IncludeContent bool
	Pool           worker.Pool
}
