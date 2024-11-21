package scanner

import "fmt"

// PermissionError represents a permission-related error during scanning
type PermissionError struct {
	Path string
	Err  error
}

func (e *PermissionError) Error() string {
	return fmt.Sprintf("permission denied: %s: %v", e.Path, e.Err)
}

// MaxDepthError represents an error when maximum depth is reached
type MaxDepthError struct {
	Path     string
	MaxDepth int
}

func (e *MaxDepthError) Error() string {
	return fmt.Sprintf("max depth %d reached at: %s", e.MaxDepth, e.Path)
}
