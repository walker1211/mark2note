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

const (
	liveImportResultFilename   = "import-result.json"
	photosImportResultFilename = "photos-import-result.json"
)

type importResultWriter struct {
	Filename string
}

func (w importResultWriter) Write(report DeliveryReport) (string, error) {
	if report.SourceDir == "" || !filepath.IsAbs(report.SourceDir) {
		return "", fmt.Errorf("import result writer requires absolute source_dir")
	}
	filename := w.Filename
	if filename == "" {
		filename = liveImportResultFilename
	}

	path := filepath.Join(report.SourceDir, filename)
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal import result: %w", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", fmt.Errorf("write import result: %w", err)
	}
	return path, nil
}
