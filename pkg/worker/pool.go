/*
Package worker provides a flexible and efficient worker pool implementation
for concurrent task processing with rate limiting and context cancellation support.

Basic usage:

	pool := worker.NewPool(worker.Config{
		Workers: 4,
		RateLimit: 10, // 10 ops/sec
	})

	// Start the pool with context
	ctx := context.Background()
	pool.Start(ctx)

	// Submit tasks
	pool.Submit(worker.Task{
		ID: 1,
		Execute: func(ctx context.Context) (worker.Result, error) {
			// Task implementation
			return worker.Result{ID: 1, Data: "processed"}, nil
		},
	})

	// Wait for results
	results, err := pool.Wait()
*/
package worker

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// Task represents a unit of work to be processed by the worker pool
type Task struct {
	// ID uniquely identifies the task
	ID int

	// Execute is the function that performs the actual work
	// It receives a context for cancellation support
	Execute func(context.Context) (Result, error)
}

// Result represents the output of a processed task
type Result struct {
	// ID matches the task ID that produced this result
	ID int

	// Data holds the actual result data
	Data interface{}

	// order is used internally to maintain submission order
	order int
}

// Config holds the configuration for the worker pool
type Config struct {
	// Workers is the number of concurrent workers
	Workers int

	// RateLimit is the maximum number of operations per second (0 for unlimited)
	RateLimit int
}

// Pool defines the interface for a worker pool
type Pool interface {
	// Start initializes and starts the worker pool
	Start(context.Context) error

	// Submit adds a task to the pool for processing
	Submit(Task) error

	// Wait blocks until all submitted tasks are processed
	// and returns the results or an error
	Wait() ([]Result, error)

	// GetStats returns current statistics about the pool
	GetStats() Stats

	// Status returns the current status of the pool
	Status() Status

	// Stop gracefully shuts down the pool
	Stop() error
}

// pool implements the Pool interface
type pool struct {
	config        Config
	tasks         chan taskWithOrder
	results       chan Result
	errors        chan error
	limiter       *rate.Limiter
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	started       bool
	stopped       bool
	stats         Stats
	startTime     time.Time
	statsMu       sync.RWMutex // Separate mutex for stats to avoid blocking pool operations
	activeWorkers atomic.Int32 // Track active workers atomically
	taskOrder     int
	orderMu       sync.Mutex
}

type taskWithOrder struct {
	Task
	order int
}

// NewPool creates a new worker pool with the given configuration
func NewPool(config Config) (Pool, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	var limiter *rate.Limiter
	if config.RateLimit > 0 {
		limiter = rate.NewLimiter(rate.Limit(config.RateLimit), 1)
	}

	p := &pool{
		config:  config,
		tasks:   make(chan taskWithOrder, config.Workers*2),
		results: make(chan Result, config.Workers*2),
		errors:  make(chan error, config.Workers),
		limiter: limiter,
		stats: Stats{
			Status: StatusStopped,
		},
	}

	// Initialize atomic counter
	p.activeWorkers.Store(0)

	return p, nil
}

// validateConfig checks if the pool configuration is valid
func validateConfig(config Config) error {
	if config.Workers <= 0 {
		return fmt.Errorf("number of workers must be positive")
	}
	if config.RateLimit < 0 {
		return fmt.Errorf("rate limit must be non-negative")
	}
	return nil
}

// Start initializes and starts the worker pool
func (p *pool) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return fmt.Errorf("pool already started")
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.started = true
	p.startTime = time.Now()

	// Initialize stats
	p.statsMu.Lock()
	p.stats = Stats{
		ActiveWorkers:  0,
		QueuedTasks:    0,
		CompletedTasks: 0,
		FailedTasks:    0,
		Status:         StatusIdle,
		Uptime:         0,
	}
	p.statsMu.Unlock()

	// Start workers
	for i := 0; i < p.config.Workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	return nil
}

// Submit adds a task to the pool for processing
func (p *pool) Submit(task Task) error {
	if !p.started {
		return fmt.Errorf("pool not started")
	}

	p.orderMu.Lock()
	order := p.taskOrder
	p.taskOrder++
	p.orderMu.Unlock()

	select {
	case <-p.ctx.Done():
		return fmt.Errorf("pool is shutting down: %w", p.ctx.Err())
	case p.tasks <- struct {
		Task
		order int
	}{task, order}:
		return nil
	}
}

// Wait blocks until all submitted tasks are processed
func (p *pool) Wait() ([]Result, error) {
	p.mu.Lock() // Lock to check and update state safely
	if !p.started {
		p.mu.Unlock()
		return nil, fmt.Errorf("pool not started")
	}

	if !p.stopped {
		// Only close if not already closed
		close(p.tasks)
		p.stopped = true
	}
	p.mu.Unlock()

	// Wait for all workers to finish
	p.wg.Wait()

	// Collect all results before closing the results channel
	var results []Result

	p.mu.Lock()
	// Close and drain the results channel
	close(p.results)
	p.mu.Unlock()

	// Collect remaining results
	for result := range p.results {
		results = append(results, result)
	}

	// Sort results by order
	sort.Slice(results, func(i, j int) bool {
		return results[i].order < results[j].order
	})

	// Check for errors
	select {
	case err := <-p.errors:
		return nil, err
	default:
		return results, nil
	}
}

// Stop gracefully shuts down the pool
func (p *pool) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return nil // Already stopped
	}

	if !p.started {
		p.stopped = true
		return nil
	}

	// Set stopped status first
	p.statsMu.Lock()
	p.stats.Status = StatusStopped
	p.stats.ActiveWorkers = 0
	p.statsMu.Unlock()

	// Mark as stopped
	p.stopped = true
	p.started = false

	// Cancel context and close channels
	p.cancel()

	// Close tasks channel only if it hasn't been closed
	select {
	case _, ok := <-p.tasks:
		if ok {
			close(p.tasks)
		}
	default:
		close(p.tasks)
	}

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(500 * time.Millisecond):
		return fmt.Errorf("shutdown timed out")
	}
}

func (p *pool) GetStats() Stats {
	p.statsMu.RLock()
	defer p.statsMu.RUnlock()

	activeWorkers := int(p.activeWorkers.Load())
	queuedTasks := len(p.tasks)

	// Use the consistent status determination
	status := p.getStatus()

	return Stats{
		ActiveWorkers:  activeWorkers,
		QueuedTasks:    queuedTasks,
		CompletedTasks: p.stats.CompletedTasks,
		FailedTasks:    p.stats.FailedTasks,
		Status:         status,
		Uptime:         time.Since(p.startTime),
	}
}

// Make Status consistent with GetStats
func (p *pool) Status() Status {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.getStatus()
}

// internal helper to get consistent status
func (p *pool) getStatus() Status {
	if !p.started {
		return StatusStopped
	}

	// If we're explicitly marked as stopped, return that
	if p.stats.Status == StatusStopped {
		return StatusStopped
	}

	activeWorkers := p.activeWorkers.Load()
	queuedTasks := len(p.tasks)

	// If there are failed tasks, active workers, or queued tasks, we're processing
	if p.stats.FailedTasks > 0 || activeWorkers > 0 || queuedTasks > 0 {
		return StatusProcessing
	}

	return StatusIdle
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// worker processes tasks from the pool
func (p *pool) worker(id int) {
	defer p.wg.Done()

	for taskWithOrder := range p.tasks {
		task := taskWithOrder.Task
		order := taskWithOrder.order

		// Increment active workers counter
		p.activeWorkers.Add(1)

		// Process task with rate limiting if configured
		if p.limiter != nil {
			err := p.limiter.Wait(p.ctx)
			if err != nil {
				p.activeWorkers.Add(-1)
				p.statsMu.Lock()
				p.stats.FailedTasks++
				p.stats.Status = StatusProcessing
				p.statsMu.Unlock()
				p.errors <- fmt.Errorf("rate limiter error: %w", err)
				return
			}
		}

		// Process task
		result, err := task.Execute(p.ctx)
		result.order = order // Preserve order in result

		// Decrement active workers counter after task completion
		p.activeWorkers.Add(-1)

		if err != nil {
			p.statsMu.Lock()
			p.stats.FailedTasks++
			p.stats.Status = StatusProcessing
			p.statsMu.Unlock()

			select {
			case p.errors <- fmt.Errorf("task %d failed: %w", task.ID, err):
			default:
				// Error channel is full, continue processing
			}
			continue
		}

		// Update completion statistics
		p.statsMu.Lock()
		p.stats.CompletedTasks++
		p.statsMu.Unlock()

		// Send result
		select {
		case <-p.ctx.Done():
			return
		case p.results <- result:
		}
	}
}
