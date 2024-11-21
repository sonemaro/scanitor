package output

import (
	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/sonemaro/scanitor/pkg/scanner"
)

// stats holds statistics about the scanned directory
type stats struct {
	Files     int   `json:"totalFiles"`
	Dirs      int   `json:"totalDirectories"`
	Symlinks  int   `json:"totalSymlinks"`
	TotalSize int64 `json:"totalSize"`
}

func (f *formatter) calculateStats(node *scanner.Node) *stats {
	f.log.Debug("Calculating directory statistics")

	stats := &stats{}
	f.walkStats(node, stats)

	f.log.WithFields(logger.Fields{
		"files":    stats.Files,
		"dirs":     stats.Dirs,
		"symlinks": stats.Symlinks,
		"size":     stats.TotalSize,
	}).Debug("Statistics calculated")

	return stats
}

func (f *formatter) walkStats(node *scanner.Node, stats *stats) {
	if node == nil {
		return
	}

	switch node.Type {
	case scanner.FileNode:
		stats.Files++
		stats.TotalSize += node.Size
	case scanner.DirNode:
		stats.Dirs++
	case scanner.SymlinkNode:
		stats.Symlinks++
	}

	for _, child := range node.Children {
		f.walkStats(child, stats)
	}
}
