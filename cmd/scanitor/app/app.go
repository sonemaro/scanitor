/*
Package app provides the main application container and orchestration for the Scanitor
application. It manages component lifecycle, coordinates operations, and handles
graceful shutdown.

The application container initializes and manages all core components:
- Logger for structured logging
- Scanner for directory traversal
- Worker pool for concurrent operations
- Progress visualization
- Output formatting

Usage:

	app := app.New(config)
	if err := app.Run(path, options); err != nil {
	    log.Fatal(err)
	}
*/
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/sonemaro/scanitor/internal/config"
	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/sonemaro/scanitor/pkg/output"
	"github.com/sonemaro/scanitor/pkg/progress"
	"github.com/sonemaro/scanitor/pkg/scanner"
	"github.com/sonemaro/scanitor/pkg/util"
	"github.com/sonemaro/scanitor/pkg/worker"
	"github.com/spf13/afero"
)

// ScanOptions defines the options for a scan operation
type ScanOptions struct {
	// Output format (tree, json, yaml)
	Format output.Format

	// Output file path (empty for stdout)
	OutputPath string

	// Patterns to ignore
	IgnorePatterns []string

	// Include file contents in output
	IncludeContent bool

	// Follow symbolic links
	FollowSymlinks bool
}

// App represents the main application container
type App struct {
	config *config.Config
	log    logger.Logger

	scanner   scanner.Scanner
	pool      worker.Pool
	formatter output.Formatter
	progress  progress.Progress

	ctx     context.Context
	cancel  context.CancelFunc
	signals chan os.Signal
	done    chan struct{}
	mu      sync.RWMutex
}

// New creates a new application instance
func New(cfg *config.Config) *App {
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		config:  cfg,
		signals: make(chan os.Signal, 1),
		done:    make(chan struct{}),
		ctx:     ctx,
		cancel:  cancel,
	}

	app.initLogger()
	app.initComponents()
	app.setupSignalHandling()

	app.log.WithFields(logger.Fields{
		"workers": cfg.Workers,
		"verbose": cfg.Verbose,
	}).Info("Application initialized")

	return app
}

// Run executes a scan operation with the given options
func (a *App) Run(path string, opts *ScanOptions) error {
	defer func() {
		if r := recover(); r != nil {
			a.log.WithFields(logger.Fields{
				"panic": r,
				"stack": string(debug.Stack()),
			}).Error("Recovered from panic")
		}
	}()

	a.log.WithFields(logger.Fields{
		"path":    path,
		"format":  opts.Format,
		"content": opts.IncludeContent,
	}).Info("Starting scan operation")

	a.progress.Start("Initializing scan...")

	ctx, cancel := context.WithTimeout(a.ctx, 1*time.Hour)
	defer cancel()

	result, err := a.scanner.Scan(ctx, path, opts.IgnorePatterns, opts.IncludeContent, opts.FollowSymlinks)
	if err != nil {
		a.progress.Error(fmt.Sprintf("Scan failed: %v", err))
		return fmt.Errorf("scan operation failed: %w", err)
	}

	// Format output
	formattedOutput, err := a.formatter.Format(result.Root)
	if err != nil {
		a.progress.Error(fmt.Sprintf("Formatting failed: %v", err))
		return fmt.Errorf("output formatting failed: %w", err)
	}

	// Write output
	if err := a.writeOutput(formattedOutput, opts.OutputPath); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	a.progress.Complete("Scan completed successfully")

	a.log.WithFields(logger.Fields{
		"files":    result.Stats.TotalFiles,
		"dirs":     result.Stats.TotalDirs,
		"size":     result.Stats.TotalSize,
		"duration": result.Stats.Duration,
		"errors":   len(result.Errors),
		"outputTo": opts.OutputPath,
	}).Info("Scan operation completed")

	return nil
}

// Shutdown performs a graceful shutdown of the application
func (a *App) Shutdown() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Info("Initiating graceful shutdown")

	// Cancel context to stop ongoing operations
	a.cancel()

	// Stop progress visualization
	a.progress.Stop()

	// Stop worker pool
	if a.pool != nil {
		if err := a.pool.Stop(); err != nil {
			a.log.WithFields(logger.Fields{
				"error": err,
			}).Error("Failed to stop worker pool")
		}
	}

	close(a.done)
	a.log.Info("Shutdown complete")
	return nil
}

// cmd/scanitor/app/app.go (continued...)

// initLogger initializes the application logger
func (a *App) initLogger() {
	a.log = logger.NewLogger(logger.Config{
		Verbosity: a.config.Verbose,
	})

	a.log.WithFields(logger.Fields{
		"verbosity": a.config.Verbose,
	}).Debug("Logger initialized")
}

// initComponents initializes all application components
func (a *App) initComponents() {
	a.log.Debug("Initializing application components")

	// Initialize worker pool
	pool, err := worker.NewPool(worker.Config{
		Workers:   a.config.Workers,
		RateLimit: a.config.RateLimit,
	})
	if err != nil {
		a.log.WithFields(logger.Fields{
			"error": err,
		}).Error("Failed to initialize worker pool")
		return
	}
	a.pool = pool

	fs := afero.NewOsFs()

	// Initialize scanner
	a.scanner = scanner.NewScanner(scanner.Config{
		Workers:  a.config.Workers,
		MaxDepth: a.config.MaxDepth,
	}, fs, a.log)

	// Initialize formatter
	a.formatter = output.NewFormatter(output.Config{
		Format:     output.FormatTree,
		WithStats:  true,
		WithColors: !a.config.NoColor,
	}, a.log)

	// Initialize progress visualization
	a.progress = progress.New(progress.Config{
		Style:             progress.StyleBar,
		ShowStats:         true,
		NoColor:           a.config.NoColor,
		RefreshRate:       100 * time.Millisecond,
		HideAfterComplete: false,
	}, a.log)

	a.log.Debug("Components initialized successfully")
}

// writeOutput writes the formatted output to the specified destination
func (a *App) writeOutput(content string, outputPath string) error {
	a.log.WithFields(logger.Fields{
		"path": outputPath,
	}).Debug("Writing output")

	if outputPath == "" {
		_, err := fmt.Fprintln(os.Stdout, content)
		if err != nil {
			a.log.WithFields(logger.Fields{
				"error": err,
			}).Error("Failed to write to stdout")
		}
		return err
	}

	err := os.WriteFile(outputPath, []byte(content), 0644)
	if err != nil {
		a.log.WithFields(logger.Fields{
			"error": err,
			"path":  outputPath,
		}).Error("Failed to write output file")
		return fmt.Errorf("failed to write output file: %w", err)
	}

	a.log.WithFields(logger.Fields{
		"path": outputPath,
	}).Info("Output written successfully")
	return nil
}

// handleScannerError processes scanner-specific errors
func (a *App) handleScannerError(err error) error {
	a.log.WithFields(logger.Fields{
		"error": err,
	}).Debug("Processing scanner error")

	switch e := err.(type) {
	case *scanner.PermissionError:
		a.log.WithFields(logger.Fields{
			"path": e.Path,
		}).Warn("Permission denied")
		return fmt.Errorf("permission denied: %s", e.Path)

	case *scanner.MaxDepthError:
		a.log.WithFields(logger.Fields{
			"path":     e.Path,
			"maxDepth": e.MaxDepth,
		}).Info("Max depth reached")
		return nil // Not a fatal error

	default:
		return fmt.Errorf("scanner error: %w", err)
	}
}

// updateProgress updates the progress visualization
func (a *App) updateProgress(stats scanner.ScanStats) {
	a.progress.Update(progress.Status{
		Current:        stats.TotalFiles, // TODO wrong value
		Total:          stats.TotalFiles,
		CurrentItem:    "",
		BytesRead:      stats.TotalSize,
		ItemsProcessed: stats.TotalFiles, // TODO wrong value
		StartTime:      stats.StartTime,
	})
}

// validatePath checks if the given path is valid for scanning
func (a *App) validatePath(path string) error {
	a.log.WithFields(logger.Fields{
		"path": path,
	}).Debug("Validating path")

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			a.log.WithFields(logger.Fields{
				"path": path,
			}).Error("Path does not exist")
			return fmt.Errorf("path does not exist: %s", path)
		}
		return fmt.Errorf("failed to access path: %w", err)
	}

	if !info.IsDir() {
		a.log.WithFields(logger.Fields{
			"path": path,
		}).Error("Path is not a directory")
		return fmt.Errorf("path is not a directory: %s", path)
	}

	return nil
}

// createOutputDirectory ensures the output directory exists
func (a *App) createOutputDirectory(path string) error {
	if path == "" {
		return nil
	}

	dir := filepath.Dir(path)
	a.log.WithFields(logger.Fields{
		"directory": dir,
	}).Debug("Ensuring output directory exists")

	if err := os.MkdirAll(dir, 0755); err != nil {
		a.log.WithFields(logger.Fields{
			"error": err,
			"path":  dir,
		}).Error("Failed to create output directory")
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	return nil
}

// isTerminal checks if the output is going to a terminal
func (a *App) isTerminal() bool {
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		a.log.Debug("Output is going to a terminal")
		return true
	}
	a.log.Debug("Output is not going to a terminal")
	return false
}

// formatDuration formats a duration for display
func (a *App) formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds",
			int(d.Minutes()),
			int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm%ds",
		int(d.Hours()),
		int(d.Minutes())%60,
		int(d.Seconds())%60)
}

// monitorResources periodically logs system resource usage
func (a *App) monitorResources(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var m runtime.MemStats
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runtime.ReadMemStats(&m)
			a.log.WithFields(logger.Fields{
				"alloc":      util.FormatSize(int64(m.Alloc)),
				"totalAlloc": util.FormatSize(int64(m.TotalAlloc)),
				"sys":        util.FormatSize(int64(m.Sys)),
				"numGC":      m.NumGC,
				"goroutines": runtime.NumGoroutine(),
			}).Debug("Resource usage stats")
		}
	}
}

// validateOptions performs comprehensive validation of scan options
func (a *App) validateOptions(opts *ScanOptions) error {
	a.log.WithFields(logger.Fields{
		"format":         opts.Format,
		"outputPath":     opts.OutputPath,
		"ignorePatterns": opts.IgnorePatterns,
	}).Debug("Validating scan options")

	// Validate output format
	if !isValidFormat(opts.Format) {
		a.log.WithFields(logger.Fields{
			"format": opts.Format,
		}).Error("Invalid output format")
		return fmt.Errorf("invalid output format: %s", opts.Format)
	}

	// Validate ignore patterns
	for _, pattern := range opts.IgnorePatterns {
		if err := validatePattern(pattern); err != nil {
			a.log.WithFields(logger.Fields{
				"pattern": pattern,
				"error":   err,
			}).Error("Invalid ignore pattern")
			return fmt.Errorf("invalid ignore pattern '%s': %w", pattern, err)
		}
	}

	// Validate output path if specified
	if opts.OutputPath != "" {
		if err := a.validateOutputPath(opts.OutputPath); err != nil {
			return err
		}
	}

	return nil
}

// validateOutputPath checks if the output path is valid and writable
func (a *App) validateOutputPath(path string) error {
	a.log.WithFields(logger.Fields{
		"path": path,
	}).Debug("Validating output path")

	// Check if directory exists
	dir := filepath.Dir(path)
	if info, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output directory does not exist: %s", dir)
		}
		return fmt.Errorf("failed to access output directory: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("output path parent is not a directory: %s", dir)
	}

	// Check if file exists and if we can write to it
	if _, err := os.Stat(path); err == nil {
		// File exists, check if we can write to it
		file, err := os.OpenFile(path, os.O_WRONLY, 0666)
		if err != nil {
			a.log.WithFields(logger.Fields{
				"path":  path,
				"error": err,
			}).Error("Cannot write to existing output file")
			return fmt.Errorf("cannot write to output file: %w", err)
		}
		file.Close()
	}

	return nil
}

// handleInterrupt manages cleanup when operation is interrupted
func (a *App) handleInterrupt() error {
	a.log.Info("Operation interrupted, cleaning up...")

	// Stop ongoing operations
	a.cancel()

	// Wait for cleanup with timeout
	done := make(chan struct{})
	go func() {
		a.cleanup()
		close(done)
	}()

	select {
	case <-done:
		a.log.Info("Cleanup completed successfully")
		return nil
	case <-time.After(5 * time.Second):
		a.log.Error("Cleanup timed out")
		return fmt.Errorf("cleanup timed out after interrupt")
	}
}

// cleanup performs resource cleanup
func (a *App) cleanup() error {
	a.log.Debug("Starting cleanup process")

	// Stop progress display
	if a.progress != nil {
		a.progress.Stop()
	}

	// Stop worker pool
	if a.pool != nil {
		if err := a.pool.Stop(); err != nil {
			a.log.WithFields(logger.Fields{
				"error": err,
			}).Error("Failed to stop worker pool")

			return err
		}
	}

	// Remove temporary files if any
	if err := a.cleanupTempFiles(); err != nil {
		a.log.WithFields(logger.Fields{
			"error": err,
		}).Error("Failed to cleanup temporary files")

		return err
	}

	a.log.Debug("Cleanup process completed")

	return nil
}

// cleanupTempFiles removes any temporary files created during scanning
func (a *App) cleanupTempFiles() error {
	tempDir := os.TempDir()
	pattern := filepath.Join(tempDir, "scanitor-*")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find temporary files: %w", err)
	}

	for _, file := range matches {
		a.log.WithFields(logger.Fields{
			"file": file,
		}).Debug("Removing temporary file")

		if err := os.Remove(file); err != nil {
			a.log.WithFields(logger.Fields{
				"file":  file,
				"error": err,
			}).Warn("Failed to remove temporary file")
		}
	}

	return nil
}

// validatePattern checks if a glob pattern is valid
func validatePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("empty pattern")
	}

	// Check for invalid characters
	invalidChars := []string{"|", "&", ";", "(", ")", "<", ">"}
	for _, char := range invalidChars {
		if strings.Contains(pattern, char) {
			return fmt.Errorf("pattern contains invalid character: %s", char)
		}
	}

	// Validate pattern syntax
	if _, err := filepath.Match(pattern, "test"); err != nil {
		return fmt.Errorf("invalid pattern syntax: %w", err)
	}

	return nil
}

// isValidFormat checks if the output format is supported
func isValidFormat(format output.Format) bool {
	validFormats := map[output.Format]bool{
		output.FormatTree: true,
		output.FormatJSON: true,
		output.FormatYAML: true,
	}
	return validFormats[format]
}

// createBackup creates a backup of an existing output file
func (a *App) createBackup(path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil // File doesn't exist, no backup needed
	}

	backupPath := path + ".bak"
	a.log.WithFields(logger.Fields{
		"original": path,
		"backup":   backupPath,
	}).Debug("Creating backup of existing output file")

	if err := os.Rename(path, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

// restoreBackup restores a backup file if the operation failed
func (a *App) restoreBackup(path string) error {
	backupPath := path + ".bak"
	if _, err := os.Stat(backupPath); err != nil {
		return nil // No backup exists
	}

	a.log.WithFields(logger.Fields{
		"backup":   backupPath,
		"original": path,
	}).Debug("Restoring backup file")

	if err := os.Rename(backupPath, path); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	return nil
}

// acquireLock attempts to acquire a file lock to prevent concurrent runs
func (a *App) acquireLock() (release func(), err error) {
	lockFile := filepath.Join(os.TempDir(), "scanitor.lock")

	a.log.WithFields(logger.Fields{
		"path": lockFile,
	}).Debug("Attempting to acquire lock")

	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("another instance is already running")
		}
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}

	release = func() {
		file.Close()
		os.Remove(lockFile)
		a.log.Debug("Lock released")
	}

	return release, nil
}
