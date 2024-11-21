package output

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/sonemaro/scanitor/pkg/scanner"
	"github.com/sonemaro/scanitor/pkg/util"
)

// formatTree formats the node tree in a tree-like structure
func (f *formatter) formatTree(node *scanner.Node) (string, error) {
	f.log.Debug("Formatting tree output")

	var builder strings.Builder
	f.formatTreeNode(&builder, node, "", true, true)

	if f.config.WithStats {
		f.log.Debug("Adding statistics to output")
		stats := f.calculateStats(node)
		builder.WriteString("\n\nStatistics:\n")
		builder.WriteString(fmt.Sprintf("  Total Files: %d\n", stats.Files))
		builder.WriteString(fmt.Sprintf("  Total Directories: %d\n", stats.Dirs))
		builder.WriteString(fmt.Sprintf("  Total Size: %s\n", util.FormatSize(stats.TotalSize)))
		builder.WriteString(fmt.Sprintf("  Symlinks: %d\n", stats.Symlinks))
	}

	return builder.String(), nil
}

func (f *formatter) formatTreeNode(builder *strings.Builder, node *scanner.Node, prefix string, isLast, isRoot bool) {
	if node == nil {
		return
	}

	f.log.WithFields(logger.Fields{
		"node":   node.Name,
		"prefix": prefix,
		"isLast": isLast,
		"isRoot": isRoot,
	}).Trace("Formatting tree node")

	// Don't add prefix for root when it's the first line
	if !isRoot || prefix != "" {
		if isLast {
			builder.WriteString(prefix + "└── ")
		} else {
			builder.WriteString(prefix + "├── ")
		}
	}

	// Format node name with colors if enabled
	nodeName := node.Name
	if f.config.WithColors {
		f.log.Debug("Applying color formatting")
		switch node.Type {
		case scanner.DirNode:
			nodeName = color.New(color.FgBlue, color.Bold).Sprint(nodeName)
		case scanner.SymlinkNode:
			nodeName = color.New(color.FgCyan).Sprint(nodeName)
		}
	}

	builder.WriteString(nodeName)
	if node.Type == scanner.DirNode {
		builder.WriteString("/")
	}
	builder.WriteString("\n")

	newPrefix := prefix
	if !isRoot || prefix != "" {
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
	}

	for i, child := range node.Children {
		isLastChild := i == len(node.Children)-1
		f.formatTreeNode(builder, child, newPrefix, isLastChild, false)
	}
}
