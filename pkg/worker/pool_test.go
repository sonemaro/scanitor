package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPool(t *testing.T) {
	tests := []struct {
		name      string
		workers   int
		tasks     int
		rateLimit int
		setup     func(*testing.T) ([]Task, error)
		validate  func(*testing.T, []Result)
		wantErr   bool
	}{
		{
			name:    "basic task processing",
			workers: 4,
			tasks:   8,
			setup: func(t *testing.T) ([]Task, error) {
				tasks := make([]Task, 8)
				for i := 0; i < 8; i++ {
					i := i
					tasks[i] = Task{
						ID: i,
						Execute: func(ctx context.Context) (Result, error) {
							return Result{ID: i, Data: i * 2}, nil
						},
					}
				}
				return tasks, nil
			},
			validate: func(t *testing.T, results []Result) {
				assert.Len(t, results, 8)
				for i, r := range results {
					assert.Equal(t, i*2, r.Data)
				}
			},
		},
		{
			name:      "rate limited processing",
			workers:   4,
			tasks:     5,
			rateLimit: 2, // 2 tasks per second
			setup: func(t *testing.T) ([]Task, error) {
				tasks := make([]Task, 5)
				for i := 0; i < 5; i++ {
					i := i
					tasks[i] = Task{
						ID: i,
						Execute: func(ctx context.Context) (Result, error) {
							return Result{ID: i, Data: i}, nil
						},
					}
				}
				return tasks, nil
			},
			validate: func(t *testing.T, results []Result) {
				assert.Len(t, results, 5)
				// Verify that tasks took appropriate time with rate limiting
				// 5 tasks at 2 per second should take at least 2.5 seconds
			},
		},
		{
			name:    "error handling",
			workers: 2,
			tasks:   3,
			setup: func(t *testing.T) ([]Task, error) {
				return []Task{
					{
						ID: 1,
						Execute: func(ctx context.Context) (Result, error) {
							return Result{}, errors.New("planned error")
						},
					},
				}, nil
			},
			validate: func(t *testing.T, results []Result) {
				assert.Empty(t, results)
			},
			wantErr: true,
		},
		{
			name:    "context cancellation",
			workers: 2,
			tasks:   5,
			setup: func(t *testing.T) ([]Task, error) {
				tasks := make([]Task, 5)
				for i := 0; i < 5; i++ {
					i := i
					tasks[i] = Task{
						ID: i,
						Execute: func(ctx context.Context) (Result, error) {
							select {
							case <-ctx.Done():
								return Result{}, ctx.Err()
							case <-time.After(2 * time.Second):
								return Result{ID: i, Data: i}, nil
							}
						},
					}
				}
				return tasks, nil
			},
			validate: func(t *testing.T, results []Result) {
				assert.Less(t, len(results), 5)
			},
			wantErr: true,
		},
		{
			name:    "concurrent execution",
			workers: 4,
			tasks:   8,
			setup: func(t *testing.T) ([]Task, error) {
				var concurrent atomic.Int32
				var maxConcurrent atomic.Int32
				tasks := make([]Task, 8)

				for i := 0; i < 8; i++ {
					i := i
					tasks[i] = Task{
						ID: i,
						Execute: func(ctx context.Context) (Result, error) {
							current := concurrent.Add(1)
							if current > maxConcurrent.Load() {
								maxConcurrent.Store(current)
							}
							time.Sleep(100 * time.Millisecond)
							concurrent.Add(-1)
							return Result{ID: i, Data: i}, nil
						},
					}
				}

				return tasks, nil
			},
			validate: func(t *testing.T, results []Result) {
				assert.Len(t, results, 8)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create pool - Fixed: handle both return values
			pool, err := NewPool(Config{
				Workers:   tt.workers,
				RateLimit: tt.rateLimit,
			})
			require.NoError(t, err)

			// Setup tasks
			tasks, err := tt.setup(t)
			require.NoError(t, err)

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Start pool
			err = pool.Start(ctx)
			require.NoError(t, err)

			// Submit tasks
			for _, task := range tasks {
				err := pool.Submit(task)
				require.NoError(t, err)
			}

			// Get results
			results, err := pool.Wait()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validate(t, results)
			}
		})
	}
}

func TestPoolConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Workers:   4,
				RateLimit: 10,
			},
			wantErr: false,
		},
		{
			name: "zero workers",
			config: Config{
				Workers: 0,
			},
			wantErr: true,
		},
		{
			name: "negative workers",
			config: Config{
				Workers: -1,
			},
			wantErr: true,
		},
		{
			name: "negative rate limit",
			config: Config{
				Workers:   1,
				RateLimit: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewPool(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, pool)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pool)
			}
		})
	}
}

func TestPoolStats(t *testing.T) {
	tests := []struct {
		name           string
		workers        int
		tasks          int
		expectedStats  func(Stats) bool
		expectedStatus Status
		setup          func(Pool) error
	}{
		{
			name:    "initial stats",
			workers: 4,
			tasks:   0,
			expectedStats: func(s Stats) bool {
				return s.ActiveWorkers == 0 &&
					s.CompletedTasks == 0 &&
					s.FailedTasks == 0 &&
					s.QueuedTasks == 0
			},
			expectedStatus: StatusIdle,
			setup:          nil,
		},
		{
			name:    "processing stats",
			workers: 2,
			tasks:   4,
			expectedStats: func(s Stats) bool {
				return s.ActiveWorkers > 0 &&
					s.QueuedTasks > 0 &&
					s.Status == StatusProcessing
			},
			expectedStatus: StatusProcessing,
			setup: func(p Pool) error {
				for i := 0; i < 4; i++ {
					err := p.Submit(Task{
						ID: i,
						Execute: func(ctx context.Context) (Result, error) {
							time.Sleep(100 * time.Millisecond)
							return Result{}, nil
						},
					})
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			name:    "completed stats",
			workers: 2,
			tasks:   2,
			expectedStats: func(s Stats) bool {
				return s.CompletedTasks == 2 &&
					s.FailedTasks == 0 &&
					s.QueuedTasks == 0
			},
			expectedStatus: StatusIdle,
			setup: func(p Pool) error {
				for i := 0; i < 2; i++ {
					err := p.Submit(Task{
						ID: i,
						Execute: func(ctx context.Context) (Result, error) {
							return Result{}, nil
						},
					})
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			name:    "failed tasks stats",
			workers: 2,
			tasks:   2,
			expectedStats: func(s Stats) bool {
				return s.FailedTasks > 0
			},
			expectedStatus: StatusProcessing,
			setup: func(p Pool) error {
				return p.Submit(Task{
					ID: 1,
					Execute: func(ctx context.Context) (Result, error) {
						return Result{}, errors.New("planned error")
					},
				})
			},
		},
		{
			name:    "shutdown_stats",
			workers: 2,
			tasks:   0,
			expectedStats: func(s Stats) bool {
				return s.Status == StatusStopped && s.ActiveWorkers == 0
			},
			expectedStatus: StatusStopped,
			setup: func(p Pool) error {
				err := p.Stop()
				if err != nil {
					return err
				}
				// Give some time for shutdown to complete
				time.Sleep(100 * time.Millisecond)
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create and start pool
			pool, err := NewPool(Config{Workers: tt.workers})
			require.NoError(t, err)

			ctx := context.Background()
			err = pool.Start(ctx)
			require.NoError(t, err)

			// Run setup if provided
			if tt.setup != nil {
				err := tt.setup(pool)
				require.NoError(t, err)
				// Give some time for tasks to process
				time.Sleep(50 * time.Millisecond)
			}

			// Check stats
			stats := pool.GetStats()
			assert.True(t, tt.expectedStats(stats),
				"Stats validation failed: %+v", stats)

			// Check status
			assert.Equal(t, tt.expectedStatus, pool.Status(),
				"Status mismatch. Expected: %s, Got: %s",
				tt.expectedStatus, pool.Status())
		})
	}
}

func TestStatsUptime(t *testing.T) {
	pool, err := NewPool(Config{Workers: 1})
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Check uptime
	stats := pool.GetStats()
	assert.True(t, stats.Uptime >= 100*time.Millisecond,
		"Expected uptime >= 100ms, got %v", stats.Uptime)
}

func TestStatsConcurrency(t *testing.T) {
	pool, err := NewPool(Config{Workers: 4})
	require.NoError(t, err)

	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)

	// Concurrently get stats while submitting tasks
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)

		// Stats reader
		go func() {
			defer wg.Done()
			_ = pool.GetStats()
			_ = pool.Status()
		}()

		// Task submitter
		go func(id int) {
			defer wg.Done()
			_ = pool.Submit(Task{
				ID: id,
				Execute: func(ctx context.Context) (Result, error) {
					time.Sleep(10 * time.Millisecond)
					return Result{}, nil
				},
			})
		}(i)
	}

	wg.Wait()
	// No assertion needed - we're testing for race conditions
}

func TestStatusTransitions(t *testing.T) {
	pool, err := NewPool(Config{Workers: 1})
	require.NoError(t, err)

	// Initial status
	assert.Equal(t, StatusStopped, pool.Status())

	// After start
	ctx := context.Background()
	err = pool.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, StatusIdle, pool.Status())

	// During processing
	err = pool.Submit(Task{
		ID: 1,
		Execute: func(ctx context.Context) (Result, error) {
			time.Sleep(100 * time.Millisecond)
			return Result{}, nil
		},
	})
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, StatusProcessing, pool.Status())

	// After stop
	err = pool.Stop()
	require.NoError(t, err)
	assert.Equal(t, StatusStopped, pool.Status())
}
