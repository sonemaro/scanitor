package progress

import (
	"fmt"
	"strings"
	"time"
)

type renderer interface {
	render(Status, string, Statistics) string
}

type barRenderer struct {
	width     int
	noColor   bool
	showStats bool
	lastSpin  int
}

func (r *barRenderer) render(status Status, message string, stats Statistics) string {
	var output strings.Builder

	// Add message first if it exists
	if message != "" {
		if !r.noColor && strings.Contains(message, "Error") {
			output.WriteString(fmt.Sprintf("\r\033[31m%s\033[0m\n", message)) // Red for errors
		} else if !r.noColor && strings.Contains(message, "Complete") {
			output.WriteString(fmt.Sprintf("\r\033[32m%s\033[0m\n", message)) // Green for completion
		} else {
			output.WriteString(fmt.Sprintf("\r%s\n", message))
		}
	}

	// Calculate bar width
	barWidth := r.width - 10 // Reserve space for percentage
	if barWidth < 10 {
		barWidth = 10
	}

	// Calculate progress
	var progress float64
	if status.Total > 0 {
		progress = float64(status.Current) / float64(status.Total)
	}
	if progress > 1 {
		progress = 1
	}

	filled := int(float64(barWidth) * progress)
	if filled > barWidth {
		filled = barWidth
	}

	// Build bar
	output.WriteString("[")

	if !r.noColor {
		output.WriteString("\033[32m") // Green color
	}

	output.WriteString(strings.Repeat("=", filled))
	if filled < barWidth {
		output.WriteString(">")
		output.WriteString(strings.Repeat(" ", barWidth-filled-1))
	}

	if !r.noColor {
		output.WriteString("\033[0m") // Reset color
	}

	output.WriteString("]")
	output.WriteString(fmt.Sprintf(" %3.0f%%", progress*100))

	// Add current item if exists
	if status.CurrentItem != "" {
		output.WriteString(fmt.Sprintf("\n%s", status.CurrentItem))
	}

	// Add stats if enabled
	if r.showStats {
		output.WriteString(fmt.Sprintf("\nProcessed: %d/%d | Speed: %.1f/s | ETA: %s",
			status.ItemsProcessed,
			status.Total,
			stats.ProcessingSpeed,
			formatDuration(stats.RemainingTime)))
	}

	return output.String()
}

type spinnerRenderer struct {
	noColor   bool
	showStats bool
	frame     int
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (r *spinnerRenderer) render(status Status, message string, stats Statistics) string {
	r.frame = (r.frame + 1) % len(spinnerFrames)
	spinner := spinnerFrames[r.frame]

	if !r.noColor {
		spinner = fmt.Sprintf("\033[36m%s\033[0m", spinner) // Cyan color
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("\r%s %s", spinner, message))

	if status.CurrentItem != "" {
		output.WriteString(fmt.Sprintf("\n%s", status.CurrentItem))
	}

	if r.showStats {
		output.WriteString(fmt.Sprintf("\nProgress: %.1f%% | Speed: %.1f/s",
			stats.ProgressPercentage,
			stats.ProcessingSpeed))
	}

	return output.String()
}

type simpleRenderer struct {
	noColor   bool
	showStats bool
}

func (r *simpleRenderer) render(status Status, message string, stats Statistics) string {
	var output strings.Builder

	if !r.noColor {
		switch {
		case strings.Contains(message, "Error"):
			message = fmt.Sprintf("\033[31m%s\033[0m", message) // Red for errors
		case strings.Contains(message, "Complete"):
			message = fmt.Sprintf("\033[32m%s\033[0m", message) // Green for completion
		}
	}

	output.WriteString(fmt.Sprintf("\r%s (%.0f%%)", message, stats.ProgressPercentage))

	if status.CurrentItem != "" {
		output.WriteString(fmt.Sprintf("\n%s", status.CurrentItem))
	}

	if r.showStats {
		output.WriteString(fmt.Sprintf("\nProcessed: %d items | %s",
			stats.ItemsProcessed,
			formatSize(stats.BytesProcessed)))
	}

	return output.String()
}

// Helper functions

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds()) // Changed %d to %.0f for float64
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds",
			int(d.Minutes()),    // Using int for minutes
			int(d.Seconds())%60) // Using int for seconds
	}
	return fmt.Sprintf("%dh%dm%ds",
		int(d.Hours()),      // Using int for hours
		int(d.Minutes())%60, // Using int for minutes
		int(d.Seconds())%60) // Using int for seconds
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
