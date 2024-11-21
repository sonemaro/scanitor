package progress

import "time"

// Style represents the type of progress visualization
type Style string

const (
	// StyleBar shows a progress bar with percentage
	StyleBar Style = "bar"

	// StyleSpinner shows a spinning indicator
	StyleSpinner Style = "spinner"

	// StyleSimple shows basic text progress
	StyleSimple Style = "simple"
)

// Config holds the configuration for progress visualization
type Config struct {
	// Style defines how progress should be displayed
	Style Style

	// Width is the maximum width for the progress bar (0 = auto-detect)
	Width int

	// ShowStats enables/disables additional statistics
	ShowStats bool

	// NoColor disables colored output
	NoColor bool

	// RefreshRate defines how often the display updates
	RefreshRate time.Duration

	// HideAfterComplete removes progress bar after completion
	HideAfterComplete bool
}

// Status represents the current progress state
type Status struct {
	// Current progress value
	Current int64

	// Total expected value
	Total int64

	// Currently processing item
	CurrentItem string

	// Bytes processed
	BytesRead int64

	// Number of items processed
	ItemsProcessed int64

	// Start time of the operation
	StartTime time.Time

	// Additional statistics
	Stats map[string]interface{}
}

// Statistics provides detailed progress information
type Statistics struct {
	// Time-related stats
	StartTime        time.Time
	EstimatedEndTime time.Time
	ElapsedTime      time.Duration
	RemainingTime    time.Duration
	ProcessingSpeed  float64 // Items per second

	// Progress-related stats
	ProgressPercentage float64
	BytesProcessed     int64
	ItemsProcessed     int64
	CurrentRate        float64 // Current processing rate
}

// Progress defines the interface for progress visualization
type Progress interface {
	// Start begins progress visualization with initial message
	Start(message string)

	// Update updates the progress status
	Update(status Status)

	// Complete marks the operation as successfully completed
	Complete(message string)

	// Error marks the operation as failed
	Error(message string)

	// Stop stops progress visualization
	Stop()

	// SetStyle changes the progress style during operation
	SetStyle(style Style)

	// EnableStats enables/disables statistics display
	EnableStats(enable bool)

	// IsSupportedTerminal checks if terminal supports advanced features
	IsSupportedTerminal() bool
}
