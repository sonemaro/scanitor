package output

import (
	"encoding/json"
	"time"

	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/sonemaro/scanitor/pkg/scanner"
)

// jsonNode represents a node in JSON output
type jsonNode struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Size     int64       `json:"size"`
	ModTime  time.Time   `json:"modTime"`
	Children []*jsonNode `json:"children,omitempty"`
}

// jsonOutput represents the complete JSON output
type jsonOutput struct {
	Root       *jsonNode `json:"root"`
	Statistics *stats    `json:"statistics,omitempty"`
	Generated  time.Time `json:"generated"`
}

func (f *formatter) formatJSON(node *scanner.Node) (string, error) {
	f.log.Debug("Formatting JSON output")

	output := &jsonOutput{
		Root:      f.convertToJSONNode(node),
		Generated: time.Now(),
	}

	if f.config.WithStats {
		f.log.Debug("Adding statistics to JSON output")
		output.Statistics = f.calculateStats(node)
	}

	bytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		f.log.WithFields(logger.Fields{
			"error": err,
		}).Error("Failed to marshal JSON")
		return "", err
	}

	return string(bytes), nil
}

func (f *formatter) convertToJSONNode(node *scanner.Node) *jsonNode {
	if node == nil {
		return nil
	}

	f.log.WithFields(logger.Fields{
		"node": node.Name,
		"type": node.Type,
	}).Trace("Converting node to JSON format")

	jNode := &jsonNode{
		Name:    node.Name,
		Size:    node.Size,
		ModTime: node.ModTime,
	}

	switch node.Type {
	case scanner.DirNode:
		jNode.Type = "directory"
	case scanner.FileNode:
		jNode.Type = "file"
	case scanner.SymlinkNode:
		jNode.Type = "symlink"
		f.log.Debug("Processing symlink")
	}

	if len(node.Children) > 0 {
		jNode.Children = make([]*jsonNode, len(node.Children))
		for i, child := range node.Children {
			jNode.Children[i] = f.convertToJSONNode(child)
		}
	}

	return jNode
}
