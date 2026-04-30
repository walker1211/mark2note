package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ImportResultWriter interface {
	Write(report DeliveryReport) (string, error)
}

type importResultWriter struct{}

func (importResultWriter) Write(report DeliveryReport) (string, error) {
	if report.SourceDir == "" || !filepath.IsAbs(report.SourceDir) {
		return "", fmt.Errorf("import result writer requires absolute source_dir")
	}

	path := filepath.Join(report.SourceDir, "import-result.json")
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal import result: %w", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", fmt.Errorf("write import result: %w", err)
	}
	return path, nil
}
