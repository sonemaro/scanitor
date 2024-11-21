/*
Package scanner provides functionality for concurrent directory scanning with
support for filtering, content extraction, and progress reporting.

The scanner uses a worker pool for concurrent processing and provides detailed
logging of all operations. It supports various configuration options including
depth limiting, pattern-based ignoring, and rate limiting.

Basic usage:

	config := scanner.Config{
		Workers: 4,
		MaxDepth: -1,
		IgnorePatterns: []string{".git", "node_modules"},
		IncludeContent: true,
	}

	scanner := scanner.NewScanner(config, fs, log)
	result, err := scanner.Scan(ctx, "/path/to/scan")
*/
package scanner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/sonemaro/scanitor/pkg/worker"
	"github.com/spf13/afero"
)

type symlinkFs interface {
	afero.Fs
	ReadlinkIfPossible(string) (string, error)
}

// Scanner defines the interface for directory scanning operations
type Scanner interface {
	// Scan performs a directory scan starting from the given root path
	Scan(ctx context.Context, root string, ignorePatterns []string, includeContent bool, followSymlinks bool) (Result, error)

	// Progress returns the current scanning progress
	Progress() Progress
}

// scanner implements the Scanner interface
type scanner struct {
	config    Config
	fs        SymlinkFs
	log       logger.Logger
	pool      worker.Pool
	stats     *ScannerStats
	startTime time.Time
}

func NewScanner(config Config, fs afero.Fs, log logger.Logger) Scanner {
	var symlinkFs SymlinkFs
	if sf, ok := fs.(SymlinkFs); ok {
		symlinkFs = sf
	} else {
		symlinkFs = &BasicSymlinkFs{Fs: fs}
	}

	return &scanner{
		config: config,
		fs:     symlinkFs,
		log:    log,
		stats:  NewScannerStats(),
	}
}

// Process worker results after Wait()
func (s *scanner) processWorkerResults(results []worker.Result, result *Result) {
	for _, workerResult := range results {
		if scanResult, ok := workerResult.Data.(scanResult); ok {
			if scanResult.err != nil {
				result.Errors[scanResult.path] = scanResult.err
			} else if scanResult.content != nil {
				result.Contents[scanResult.path] = scanResult.content
			}
		}
	}
}

// Scan performs the directory scan operation
func (s *scanner) Scan(ctx context.Context, root string, ignorePatterns []string, includeContent bool, followSymlinks bool) (Result, error) {
	if s.config.Workers <= 0 {
		return Result{}, fmt.Errorf("invalid configuration: workers count must be positive")
	}

	s.log.WithFields(logger.Fields{
		"path":     root,
		"workers":  s.config.Workers,
		"maxDepth": s.config.MaxDepth,
		"patterns": ignorePatterns,
	}).Info("Starting scan operation")

	s.startTime = time.Now()

	// Initialize result
	result := Result{
		Errors: make(map[string]error),
		Stats: ScanStats{
			StartTime: s.startTime,
		},
	}

	// Only initialize Contents map if we're including content
	if includeContent {
		result.Contents = make(map[string][]byte)
	}

	// Create worker pool
	pool, err := worker.NewPool(worker.Config{
		Workers:   s.config.Workers,
		RateLimit: s.config.RateLimit,
	})
	if err != nil {
		s.log.WithFields(logger.Fields{
			"error": err,
		}).Error("Failed to create worker pool")

		return result, fmt.Errorf("failed to create worker pool: %w", err)
	}

	// Start worker pool
	if err := pool.Start(ctx); err != nil {
		s.log.WithFields(logger.Fields{
			"error": err,
		}).Error("Failed to start worker pool")
		return result, fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Ensure pool is stopped after we're done
	defer func() {
		if err := pool.Stop(); err != nil {
			s.log.WithFields(logger.Fields{
				"error": err,
			}).Warn("Error stopping worker pool")
		}
	}()

	// Create root node
	rootInfo, err := s.fs.Stat(root)
	if err != nil {
		s.log.WithFields(logger.Fields{
			"error": err,
			"path":  root,
		}).Error("Failed to stat root directory")
		return result, fmt.Errorf("failed to stat root directory: %w", err)
	}

	result.Root = &Node{
		Name:    filepath.Base(root),
		Path:    root,
		Type:    DirNode,
		Size:    rootInfo.Size(),
		Mode:    uint32(rootInfo.Mode()),
		ModTime: rootInfo.ModTime(),
	}

	// Start recursive scan
	if err := s.scanDir(ctx, pool, result.Root, 0, &result, followSymlinks, includeContent, ignorePatterns); err != nil {
		s.log.WithFields(logger.Fields{
			"error": err,
		}).Error("Scan operation failed")
		return result, fmt.Errorf("scan operation failed: %w", err)
	}

	// Wait for all workers to complete
	workerResults, err := pool.Wait()
	if err != nil {
		s.log.WithFields(logger.Fields{
			"error": err,
		}).Error("Error waiting for workers to complete")

		return result, fmt.Errorf("error waiting for workers: %w", err)
	}

	// Process worker results
	s.processWorkerResults(workerResults, &result)

	// Update final statistics
	result.Stats.EndTime = time.Now()
	result.Stats.Duration = result.Stats.EndTime.Sub(result.Stats.StartTime)
	result.Stats.TotalFiles = s.stats.GetFilesScanned()
	result.Stats.TotalSize = s.stats.GetBytesRead()
	result.Stats.ErrorCount = len(result.Errors)

	s.log.WithFields(logger.Fields{
		"duration":     result.Stats.Duration,
		"totalFiles":   result.Stats.TotalFiles,
		"totalSize":    result.Stats.TotalSize,
		"errorCount":   result.Stats.ErrorCount,
		"skippedFiles": result.Stats.SkippedFiles,
	}).Info("Scan operation completed")

	return result, nil
}

// scanDir recursively scans a directory
func (s *scanner) scanDir(ctx context.Context, pool worker.Pool, node *Node, depth int, result *Result, followSymlinks bool, includeContent bool, ignorePatterns []string) error {
	s.stats.SetCurrentDepth(int32(depth))

	s.log.WithFields(logger.Fields{
		"path":  node.Path,
		"depth": depth,
	}).Debug("Scanning directory")

	if s.config.MaxDepth >= 0 && depth >= s.config.MaxDepth {
		s.log.WithFields(logger.Fields{
			"path":  node.Path,
			"depth": depth,
		}).Debug("Max depth reached")

		return nil
	}

	// Read directory entries
	entries, err := afero.ReadDir(s.fs, node.Path)
	if err != nil {
		s.log.WithFields(logger.Fields{
			"error": err,
			"path":  node.Path,
		}).Error("Failed to read directory")
		result.Errors[node.Path] = err
		return nil
	}

	// Process each entry
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			entryPath := filepath.Join(node.Path, entry.Name())

			// Check permissions before creating node
			info, err := s.fs.Stat(entryPath)
			if err != nil {
				s.log.WithFields(logger.Fields{
					"error": err,
					"path":  entryPath,
				}).Warn("Permission error")
				result.Errors[entryPath] = fmt.Errorf("permission denied: %s", entryPath)

				// Still create the node to show it exists
				childNode := &Node{
					Name:    entry.Name(),
					Path:    entryPath,
					Size:    0,
					Mode:    0,
					ModTime: time.Time{},
				}
				node.Children = append(node.Children, childNode)
				continue
			}

			childNode := &Node{
				Name:    entry.Name(),
				Path:    entryPath,
				Size:    info.Size(),
				Mode:    uint32(info.Mode()),
				ModTime: info.ModTime(),
			}

			if s.shouldIgnore(entryPath, ignorePatterns) {
				s.log.WithFields(logger.Fields{
					"path": entryPath,
				}).Debug("Ignoring path")
				atomic.AddInt64(&result.Stats.SkippedFiles, 1)
				continue
			}

			if info.IsDir() {
				childNode.Type = DirNode
				node.Children = append(node.Children, childNode)

				if s.config.MaxDepth < 0 || depth < s.config.MaxDepth {
					if err := s.scanDir(ctx, pool, childNode, depth+1, result, followSymlinks, includeContent, ignorePatterns); err != nil {
						return err
					}
				}
			} else {
				if info.Mode()&os.ModeSymlink != 0 {
					childNode.Type = SymlinkNode
					if followSymlinks {
					}
				} else {
					childNode.Type = FileNode
				}
				node.Children = append(node.Children, childNode)
				s.stats.AddFilesFound(1)

				// Try to read content if configured
				if includeContent {
					// Add specific permission check for reading
					if info.Mode().Perm()&0444 == 0 {
						s.log.WithFields(logger.Fields{
							"path": entryPath,
							"mode": info.Mode(),
						}).Warn("File not readable")
						result.Errors[entryPath] = fmt.Errorf("permission denied: %s", entryPath)
						continue
					}
					s.submitFileTask(ctx, pool, childNode, result)
				}
			}
		}
	}

	return nil
}

// submitFileTask submits a file processing task to the worker pool
func (s *scanner) submitFileTask(ctx context.Context, pool worker.Pool, node *Node, result *Result) {
	currentID := s.stats.GetFilesFound()
	task := worker.Task{
		ID: int(currentID),
		Execute: func(ctx context.Context) (worker.Result, error) {
			s.log.WithFields(logger.Fields{
				"path": node.Path,
				"id":   currentID,
			}).Debug("Processing file content")

			content, err := s.readFileContent(ctx, node.Path)
			return worker.Result{
				ID: int(currentID),
				Data: scanResult{
					path:    node.Path,
					content: content,
					err:     err,
				},
			}, nil
		},
	}

	if err := pool.Submit(task); err != nil {
		s.log.WithFields(logger.Fields{
			"error": err,
			"path":  node.Path,
		}).Error("Failed to submit file processing task")
		result.Errors[node.Path] = err
	}
}

// readFileContent reads the content of a file with progress tracking
func (s *scanner) readFileContent(ctx context.Context, path string) ([]byte, error) {
	s.log.WithFields(logger.Fields{
		"path": path,
	}).Debug("Reading file content")

	// Check file permissions first
	info, err := s.fs.Stat(path)
	if err != nil {
		s.log.WithFields(logger.Fields{
			"error": err,
			"path":  path,
		}).Error("Failed to stat file")
		return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	if info.Mode().Perm()&0444 == 0 {
		s.log.WithFields(logger.Fields{
			"path": path,
			"mode": info.Mode(),
		}).Warn("File not readable")
		return nil, fmt.Errorf("permission denied: %s", path)
	}

	// Open file
	file, err := s.fs.Open(path)
	if err != nil {
		s.log.WithFields(logger.Fields{
			"error": err,
			"path":  path,
		}).Error("Failed to open file")
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()

	// Initialize buffer with configured size
	bufferSize := s.config.BufferSize
	if bufferSize <= 0 {
		bufferSize = 4096 // Default buffer size
	}

	buf := make([]byte, bufferSize)
	content := make([]byte, 0, info.Size()) // Pre-allocate if size is known

	s.log.WithFields(logger.Fields{
		"path":       path,
		"size":       info.Size(),
		"bufferSize": bufferSize,
	}).Trace("Starting file read")

	for {
		select {
		case <-ctx.Done():
			s.log.WithFields(logger.Fields{
				"path":   path,
				"reason": ctx.Err(),
			}).Debug("File read cancelled")
			return nil, ctx.Err()

		default:
			n, err := file.Read(buf)
			if n > 0 {
				content = append(content, buf[:n]...)
				bytesRead := s.stats.AddBytesRead(int64(n))

				s.log.WithFields(logger.Fields{
					"path":      path,
					"bytesRead": n,
					"totalRead": bytesRead,
				}).Trace("Read progress")
			}

			if err == io.EOF {
				s.stats.AddFilesScanned(1)
				s.log.WithFields(logger.Fields{
					"path":    path,
					"size":    len(content),
					"scanned": s.stats.GetFilesScanned(),
				}).Debug("File read completed")
				return content, nil
			}

			if err != nil {
				s.log.WithFields(logger.Fields{
					"error": err,
					"path":  path,
				}).Error("Error reading file")
				return nil, fmt.Errorf("error reading file %s: %w", path, err)
			}
		}
	}
}

// shouldIgnore checks if a path should be ignored based on ignore patterns
func (s *scanner) shouldIgnore(path string, patterns []string) bool {
	base := filepath.Base(path)
	relPath := filepath.ToSlash(path)

	s.log.WithFields(logger.Fields{
		"path":    path,
		"base":    base,
		"relPath": relPath,
	}).Debug("Checking ignore patterns")

	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)

		s.log.WithFields(logger.Fields{
			"pattern": pattern,
			"path":    path,
			"base":    base,
			"relPath": relPath,
		}).Trace("Checking pattern")

		// Direct base name match
		if matched, _ := filepath.Match(pattern, base); matched {
			s.log.WithFields(logger.Fields{
				"pattern": pattern,
				"path":    path,
				"reason":  "base name match",
			}).Debug("Path ignored")
			return true
		}

		// Handle directory-specific patterns (ending with /)
		if strings.HasSuffix(pattern, "/") {
			dirPattern := strings.TrimSuffix(pattern, "/")
			if strings.Contains(relPath, "/"+dirPattern+"/") {
				s.log.WithFields(logger.Fields{
					"pattern": pattern,
					"path":    path,
					"reason":  "directory pattern match",
				}).Debug("Path ignored")
				return true
			}
		}

		// Handle patterns with path separators
		if strings.Contains(pattern, "/") {
			if matched, _ := filepath.Match(pattern, relPath); matched {
				s.log.WithFields(logger.Fields{
					"pattern": pattern,
					"path":    path,
					"reason":  "path pattern match",
				}).Debug("Path ignored")

				return true
			}
			// Handle **/ prefix
			if strings.HasPrefix(pattern, "**/") {
				suffix := strings.TrimPrefix(pattern, "**/")
				if matched, _ := filepath.Match(suffix, base); matched {
					s.log.WithFields(logger.Fields{
						"pattern": pattern,
						"path":    path,
						"reason":  "recursive pattern match",
					}).Debug("Path ignored")

					return true
				}
			}
		}

		// Handle extension patterns
		if strings.HasPrefix(pattern, "*.") {
			if matched, _ := filepath.Match(pattern, base); matched {
				s.log.WithFields(logger.Fields{
					"pattern": pattern,
					"path":    path,
					"reason":  "extension pattern match",
				}).Debug("Path ignored")

				return true
			}
		}
	}

	s.log.WithFields(logger.Fields{
		"path": path,
	}).Debug("Path not ignored")

	return false
}

// handleSymlink processes a symbolic link
func (s *scanner) handleSymlink(ctx context.Context, node *Node, result *Result, includeContent bool) {
	target, err := s.fs.ReadlinkIfPossible(node.Path)
	if err != nil {
		s.log.WithFields(logger.Fields{
			"error": err,
			"path":  node.Path,
		}).Warn("Failed to read symlink")
		result.Errors[node.Path] = fmt.Errorf("failed to read symlink %s: %w", node.Path, err)
		return
	}

	// Resolve relative symlinks
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(node.Path), target)
	}

	targetInfo, err := s.fs.Stat(target)
	if err != nil {
		s.log.WithFields(logger.Fields{
			"error":  err,
			"path":   node.Path,
			"target": target,
		}).Warn("Failed to stat symlink target")
		result.Errors[node.Path] = fmt.Errorf("failed to stat symlink target %s: %w", target, err)
		return
	}

	node.Size = targetInfo.Size()
	node.ModTime = targetInfo.ModTime()

	if includeContent {
		content, err := s.readFileContent(ctx, target)
		if err != nil {
			s.log.WithFields(logger.Fields{
				"error":  err,
				"path":   node.Path,
				"target": target,
			}).Warn("Failed to read symlink content")
			result.Errors[node.Path] = err
			return
		}
		result.Contents[node.Path] = content
	}
}

// Progress returns the current scanning progress
func (s *scanner) Progress() Progress {
	return Progress{
		TotalFiles:     s.stats.GetFilesFound(),
		ProcessedFiles: s.stats.GetFilesScanned(),
		CurrentDepth:   int(s.stats.GetCurrentDepth()),
		StartTime:      s.startTime,
		BytesRead:      s.stats.GetBytesRead(),
	}
}
