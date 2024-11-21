package output

import (
	"time"

	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/sonemaro/scanitor/pkg/scanner"
	"gopkg.in/yaml.v3"
)

func (f *formatter) formatYAML(node *scanner.Node) (string, error) {
	f.log.Debug("Formatting YAML output")

	// Reuse JSON structure for YAML output
	output := &jsonOutput{
		Root:      f.convertToJSONNode(node),
		Generated: time.Now(),
	}

	if f.config.WithStats {
		f.log.Debug("Adding statistics to YAML output")
		output.Statistics = f.calculateStats(node)
	}

	bytes, err := yaml.Marshal(output)
	if err != nil {
		f.log.WithFields(logger.Fields{
			"error": err,
		}).Error("Failed to marshal YAML")
		return "", err
	}

	return string(bytes), nil
}
