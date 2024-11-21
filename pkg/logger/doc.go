/*
Package logger provides a structured logging solution for the Scanitor application.
It wraps uber-go/zap logger to provide a simpler interface with support for
different verbosity levels and structured logging.

Basic Usage:

	logger := logger.NewLogger(logger.Config{
	    Verbosity: 0,  // Default level (INFO)
	})

	// Simple logging
	logger.Info("Application started")
	logger.Debug("Processing directory") // Only shown with verbosity >= 1
	logger.Trace("Detailed operation")   // Only shown with verbosity >= 2

Verbosity Levels:

	0: Info, Warn, Error (default)
	1: Debug + Level 0
	2: Trace + Level 1

Structured Logging:

	logger.WithFields(logger.Fields{
	    "component": "scanner",
	    "path":     "/some/path",
	    "count":    42,
	}).Info("Directory scan completed")

Output Example (JSON):

	{
	    "level": "info",
	    "ts": "2024-01-20T15:04:05.000Z",
	    "message": "Directory scan completed",
	    "component": "scanner",
	    "path": "/some/path",
	    "count": 42
	}

Configuration:

	type Config struct {
	    Verbosity int       // Logging verbosity level
	    Output    io.Writer // Output writer (defaults to os.Stderr)
	}

Environment Integration:

	verbosity := 0
	if verbose := os.Getenv("SCANITOR_VERBOSE"); verbose != "" {
	    verbosity = len(verbose)  // Each 'v' increases verbosity
	}

	logger := logger.NewLogger(logger.Config{
	    Verbosity: verbosity,
	})

Thread Safety:

The logger is safe for concurrent use by multiple goroutines.
All logging methods can be called concurrently.

Error Handling Example:

	if err != nil {
	    logger.WithFields(logger.Fields{
	        "error": err.Error(),
	        "file":  filename,
	    }).Error("Failed to process file")
	}

Performance Considerations:

The logger uses uber-go/zap internally, which provides high-performance
structured logging. Field allocation is only done when the log level
is enabled.
*/
package logger
