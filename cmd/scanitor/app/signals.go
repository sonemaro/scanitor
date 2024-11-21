/*
Package app signal handling implementation provides graceful shutdown and cleanup
functionality for the Scanitor application. It handles system signals like SIGINT
and SIGTERM, ensuring proper resource cleanup and operation termination.
*/
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sonemaro/scanitor/internal/config"
	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/sonemaro/scanitor/pkg/output"
	"github.com/sonemaro/scanitor/pkg/progress"
	"github.com/sonemaro/scanitor/pkg/worker"
)

// signalState tracks the state of signal handling
type signalState struct {
	shutdownInitiated atomic.Bool
	forceShutdown     atomic.Bool
}

// setupSignalHandling initializes signal handling for graceful shutdown
func (a *App) setupSignalHandling() {
	state := &signalState{}

	a.log.Debug("Initializing signal handlers")

	// Create signal channel for specified signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
	)

	go a.handleSignals(sigChan, state)
}

// handleSignals processes incoming system signals
func (a *App) handleSignals(sigChan chan os.Signal, state *signalState) {
	for sig := range sigChan {
		a.log.WithFields(logger.Fields{
			"signal": sig.String(),
		}).Debug("Received system signal")

		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			if state.forceShutdown.Load() {
				a.handleForcedShutdown()
				return
			}

			if state.shutdownInitiated.Load() {
				a.log.Warn("Received second interrupt, initiating forced shutdown")
				state.forceShutdown.Store(true)
				continue
			}

			if !state.shutdownInitiated.CompareAndSwap(false, true) {
				continue
			}

			a.handleGracefulShutdown()

		case syscall.SIGHUP:
			a.handleHangup()
		}
	}
}

// handleGracefulShutdown performs a graceful shutdown of the application
func (a *App) handleGracefulShutdown() {
	a.log.Info("Initiating graceful shutdown")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Notify progress of shutdown
	if a.progress != nil {
		a.progress.Update(progress.Status{
			CurrentItem: "Shutting down...",
		})
	}

	// Create error channel for shutdown operations
	errCh := make(chan error, 1)

	go func() {
		errCh <- a.performShutdown(ctx)
	}()

	// Wait for shutdown completion or timeout
	select {
	case err := <-errCh:
		if err != nil {
			a.log.WithFields(logger.Fields{
				"error": err,
			}).Error("Shutdown encountered errors")
		} else {
			a.log.Info("Graceful shutdown completed")
		}

	case <-ctx.Done():
		a.log.Error("Shutdown timed out")
	}
}

// handleForcedShutdown performs an immediate shutdown
func (a *App) handleForcedShutdown() {
	a.log.Warn("Forced shutdown initiated")

	// Cancel all operations immediately
	a.cancel()

	// Perform minimal cleanup
	if a.progress != nil {
		a.progress.Stop()
	}

	if a.pool != nil {
		err := a.pool.Stop()
		if err != nil {
			a.log.WithFields(logger.Fields{
				"error": err,
			}).Error("Failed to stop worker pool during forced shutdown")
		}
	}

	a.log.Info("Forced shutdown completed")
	os.Exit(1)
}

// handleHangup handles SIGHUP signal
func (a *App) handleHangup() {
	a.log.Info("Received SIGHUP signal")

	// Reload configuration if supported
	if err := a.reloadConfiguration(); err != nil {
		a.log.WithFields(logger.Fields{
			"error": err,
		}).Error("Failed to reload configuration")
	}
}

// performShutdown executes the shutdown sequence
func (a *App) performShutdown(ctx context.Context) error {
	a.log.Debug("Executing shutdown sequence")

	// Create error channel for collecting shutdown errors
	errCh := make(chan error, 3)

	// Stop accepting new tasks
	if a.pool != nil {
		go func() {
			if err := a.pool.Stop(); err != nil {
				errCh <- fmt.Errorf("worker pool shutdown failed: %w", err)
				return
			}
			errCh <- nil
		}()
	}

	// Stop progress display
	if a.progress != nil {
		go func() {
			a.progress.Stop()
			errCh <- nil
		}()
	}

	// Cleanup temporary resources
	go func() {
		if err := a.cleanup(); err != nil {
			errCh <- fmt.Errorf("cleanup failed: %w", err)
			return
		}
		errCh <- nil
	}()

	// Collect errors from shutdown operations
	var shutdownErrors []error
	for i := 0; i < cap(errCh); i++ {
		select {
		case err := <-errCh:
			if err != nil {
				shutdownErrors = append(shutdownErrors, err)
			}
		case <-ctx.Done():
			return fmt.Errorf("shutdown timed out")
		}
	}

	// Log shutdown completion status
	if len(shutdownErrors) > 0 {
		for _, err := range shutdownErrors {
			a.log.WithFields(logger.Fields{
				"error": err,
			}).Error("Shutdown error occurred")
		}
		return fmt.Errorf("shutdown completed with %d errors", len(shutdownErrors))
	}

	a.log.Debug("Shutdown sequence completed successfully")
	return nil
}

// reloadConfiguration reloads application configuration
func (a *App) reloadConfiguration() error {
	a.log.Debug("Reloading configuration")

	a.mu.Lock()
	defer a.mu.Unlock()

	// Reload configuration from environment
	newConfig, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to reload configuration: %w", err)
	}

	// Update configuration
	a.config = &newConfig

	// Update components with new configuration
	if err := a.updateComponents(); err != nil {
		return fmt.Errorf("failed to update components: %w", err)
	}

	a.log.Info("Configuration reloaded successfully")
	return nil
}

// updateComponents updates component configurations
func (a *App) updateComponents() error {
	a.log.Debug("Recreating components with new configuration")

	// Recreate worker pool if it exists
	if a.pool != nil {
		// Stop existing pool first
		if err := a.pool.Stop(); err != nil {
			a.log.WithFields(logger.Fields{
				"error": err,
			}).Error("Failed to stop existing worker pool")
			return fmt.Errorf("failed to stop worker pool: %w", err)
		}

		// Create new pool with updated configuration
		newPool, err := worker.NewPool(worker.Config{
			Workers:   a.config.Workers,
			RateLimit: a.config.RateLimit,
		})
		if err != nil {
			a.log.WithFields(logger.Fields{
				"error": err,
			}).Error("Failed to create new worker pool")
			return fmt.Errorf("failed to create new worker pool: %w", err)
		}

		a.pool = newPool
		a.log.Debug("Worker pool recreated with new configuration")
	}

	// Update progress display settings
	if a.progress != nil {
		a.progress.EnableStats(!a.config.NoProgress)
		a.log.Debug("Progress display settings updated")
	}

	// Recreate formatter with new settings
	if a.formatter != nil {
		a.formatter = output.NewFormatter(output.Config{
			WithColors: !a.config.NoColor,
			WithStats:  true,
		}, a.log)
		a.log.Debug("Output formatter recreated with new configuration")
	}

	return nil
}
