package worker

import "time"

// Status represents the current state of the worker pool
type Status string

const (
	// StatusIdle indicates the pool is ready but not processing
	StatusIdle Status = "idle"

	// StatusProcessing indicates the pool is actively processing tasks
	StatusProcessing Status = "processing"

	// StatusShuttingDown indicates the pool is gracefully shutting down
	StatusShuttingDown Status = "shutting_down"

	// StatusStopped indicates the pool has been stopped
	StatusStopped Status = "stopped"
)

// Stats provides runtime statistics about the worker pool
type Stats struct {
	// ActiveWorkers is the number of workers currently processing tasks
	ActiveWorkers int

	// QueuedTasks is the number of tasks waiting to be processed
	QueuedTasks int

	// CompletedTasks is the number of tasks that have been processed
	CompletedTasks int

	// FailedTasks is the number of tasks that failed processing
	FailedTasks int

	// Status is the current state of the pool
	Status Status

	// Uptime is how long the pool has been running
	Uptime time.Duration
}
