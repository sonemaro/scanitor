package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/sonemaro/scanitor/pkg/logger"
	"golang.org/x/term"
)

type progress struct {
	config Config
	log    logger.Logger
	writer io.Writer

	// State
	status     Status
	startTime  time.Time
	lastUpdate time.Time
	message    string
	isActive   bool
	hasError   bool

	// Rendering
	renderer    renderer
	refreshRate time.Duration
	width       int

	// Synchronization
	mu       sync.RWMutex
	stopChan chan struct{}
	doneChan chan struct{}
}

// New creates a new progress visualization instance
func New(config Config, log logger.Logger) Progress {
	if config.RefreshRate == 0 {
		config.RefreshRate = 100 * time.Millisecond
	}

	p := &progress{
		config:      config,
		log:         log,
		writer:      os.Stdout,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
		refreshRate: config.RefreshRate,
	}

	// Initialize renderer based on style
	p.renderer = p.createRenderer()

	// Auto-detect terminal width if not specified
	if p.config.Width == 0 {
		p.width = p.getTerminalWidth()
	} else {
		p.width = p.config.Width
	}

	p.log.WithFields(logger.Fields{
		"style":     p.config.Style,
		"width":     p.width,
		"showStats": p.config.ShowStats,
		"noColor":   p.config.NoColor,
		"refresh":   p.config.RefreshRate,
	}).Debug("Created new progress instance")

	return p
}

func (p *progress) Start(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.WithFields(logger.Fields{
		"message": message,
	}).Debug("Starting progress")

	p.message = message
	p.startTime = time.Now()
	p.isActive = true
	p.hasError = false

	// Start render loop
	go p.renderLoop()
}

func (p *progress) Update(status Status) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.WithFields(logger.Fields{
		"current": status.Current,
		"total":   status.Total,
		"item":    status.CurrentItem,
	}).Trace("Updating progress")

	p.status = status
	p.lastUpdate = time.Now()

	// Immediate render for edge cases
	if p.isActive {
		p.render()
	}
}

func (p *progress) Complete(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.WithFields(logger.Fields{
		"message": message,
	}).Debug("Completing progress")

	p.message = message
	p.status.Current = p.status.Total
	p.render()

	if p.config.HideAfterComplete {
		p.clearLine()
	}
	p.isActive = false
}

func (p *progress) Error(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.WithFields(logger.Fields{
		"message": message,
	}).Debug("Error in progress")

	p.message = message
	p.hasError = true
	p.isActive = false
	p.render() // Final render
}

func (p *progress) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.Debug("Stopping progress")

	if p.isActive {
		close(p.stopChan)
		<-p.doneChan
		p.isActive = false
	}

	p.clearLine()
}

func (p *progress) SetStyle(style Style) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.WithFields(logger.Fields{
		"style": style,
	}).Debug("Setting progress style")

	p.config.Style = style
	p.renderer = p.createRenderer()
}

func (p *progress) EnableStats(enable bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.WithFields(logger.Fields{
		"enabled": enable,
	}).Debug("Toggling statistics display")

	p.config.ShowStats = enable
}

func (p *progress) IsSupportedTerminal() bool {
	if f, ok := p.writer.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// Internal methods

func (p *progress) renderLoop() {
	ticker := time.NewTicker(p.refreshRate)
	defer ticker.Stop()
	defer close(p.doneChan)

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.mu.Lock()
			p.render()
			p.mu.Unlock()
		}
	}
}

func (p *progress) render() {
	output := p.renderer.render(p.status, p.message, p.calculateStats())
	p.clearLine()
	fmt.Fprint(p.writer, output)
}

func (p *progress) clearLine() {
	if p.IsSupportedTerminal() {
		fmt.Fprint(p.writer, "\r\033[K") // Clear line
	} else {
		fmt.Fprint(p.writer, "\r") // Just return to start
	}
}

func (p *progress) getTerminalWidth() int {
	if p.IsSupportedTerminal() {
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			return w
		}
	}

	return 80 // Default width
}

func (p *progress) calculateStats() Statistics {
	now := time.Now()
	elapsed := now.Sub(p.startTime)

	var stats Statistics
	stats.StartTime = p.startTime
	stats.ElapsedTime = elapsed

	if p.status.Total > 0 {
		stats.ProgressPercentage = float64(p.status.Current) / float64(p.status.Total) * 100

		if p.status.Current > 0 {
			itemsPerSecond := float64(p.status.ItemsProcessed) / elapsed.Seconds()
			stats.ProcessingSpeed = itemsPerSecond

			remainingItems := p.status.Total - p.status.Current
			if itemsPerSecond > 0 {
				remainingTime := time.Duration(float64(remainingItems)/itemsPerSecond) * time.Second
				stats.RemainingTime = remainingTime
				stats.EstimatedEndTime = now.Add(remainingTime)
			}
		}
	}

	stats.BytesProcessed = p.status.BytesRead
	stats.ItemsProcessed = p.status.ItemsProcessed
	stats.CurrentRate = float64(p.status.ItemsProcessed) / elapsed.Seconds()

	return stats
}

func (p *progress) createRenderer() renderer {
	switch p.config.Style {
	case StyleBar:
		return &barRenderer{
			width:     p.width,
			noColor:   p.config.NoColor,
			showStats: p.config.ShowStats,
		}
	case StyleSpinner:
		return &spinnerRenderer{
			noColor:   p.config.NoColor,
			showStats: p.config.ShowStats,
		}
	default:
		return &simpleRenderer{
			noColor:   p.config.NoColor,
			showStats: p.config.ShowStats,
		}
	}
}
