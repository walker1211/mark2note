package render

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type PNGDeliveryOrchestrator struct {
	Importer PhotosImporter
	Writer   ImportResultWriter
	Now      func() time.Time
}

type pngDeliveryRequest struct {
	SourceDir     string
	Paths         []string
	AlbumName     string
	ImportTimeout time.Duration
}

func (o PNGDeliveryOrchestrator) Deliver(req pngDeliveryRequest) (liveDeliveryResult, error) {
	paths := append([]string(nil), req.Paths...)
	report := DeliveryReport{SourceDir: req.SourceDir, CandidatePairs: len(paths)}
	if len(paths) == 0 {
		report.Status = deliveryStatusFailed
		report.Message = "no generated PNG files found"
		return o.writeReport(report, errors.New(report.Message))
	}

	ctx, cancel := context.WithTimeout(context.Background(), req.ImportTimeout)
	defer cancel()
	if err := o.importer().CheckAvailable(ctx); err != nil {
		report.Status = deliveryStatusFailed
		report.Message = err.Error()
		return o.writeReport(reportWithErrorAlbum(report, req.AlbumName), err)
	}
	albumName, err := finalizeAlbumNameWithPrefix(ctx, o.importer(), req.AlbumName, o.now()(), "mark2note-photos")
	if err != nil {
		report.Status = deliveryStatusFailed
		report.Message = err.Error()
		return o.writeReport(reportWithErrorAlbum(report, req.AlbumName), err)
	}
	stageDir, err := stagePNGFiles(req.SourceDir, paths)
	if err != nil {
		report.Status = deliveryStatusFailed
		report.Message = err.Error()
		return o.writeReport(report, err)
	}
	importResult, err := o.importer().ImportDirectory(ctx, ImportPhotosRequest{SourceDir: stageDir, AlbumName: albumName})
	report.AlbumName = albumName
	if err != nil {
		report.Status = deliveryStatusFailed
		report.Message = err.Error()
		return o.writeReport(report, err)
	}
	if !importResult.Executed {
		report.Status = deliveryStatusFailed
		report.Message = "photos import did not execute"
		return o.writeReport(report, errors.New(report.Message))
	}
	if importResult.Message == "" {
		importResult.Message = photosImportSubmittedMessage
	}
	report.Status = deliveryStatusPartial
	report.Message = importResult.Message
	return o.writeReport(report)
}

func stagePNGFiles(sourceDir string, paths []string) (string, error) {
	stageDir := filepath.Join(sourceDir, ".photos-import", "png")
	if err := os.RemoveAll(stageDir); err != nil {
		return "", fmt.Errorf("prepare PNG import staging dir: %w", err)
	}
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return "", fmt.Errorf("create PNG import staging dir: %w", err)
	}
	for _, src := range paths {
		dst := filepath.Join(stageDir, filepath.Base(src))
		content, err := os.ReadFile(src)
		if err != nil {
			return "", fmt.Errorf("read PNG for import: %w", err)
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			return "", fmt.Errorf("stage PNG for import: %w", err)
		}
	}
	return stageDir, nil
}

func (o PNGDeliveryOrchestrator) writeReport(report DeliveryReport, cause ...error) (liveDeliveryResult, error) {
	path, err := o.writer().Write(report)
	if err != nil {
		return liveDeliveryResult{}, fmt.Errorf("write import report: %w", err)
	}
	result := liveDeliveryResult{Report: report, ReportPath: path}
	if len(cause) > 0 && cause[0] != nil {
		return result, cause[0]
	}
	return result, nil
}

func (o PNGDeliveryOrchestrator) importer() PhotosImporter {
	if o.Importer != nil {
		return o.Importer
	}
	return osascriptPhotosImporter{}
}

func (o PNGDeliveryOrchestrator) writer() ImportResultWriter {
	if o.Writer != nil {
		return o.Writer
	}
	return importResultWriter{Filename: photosImportResultFilename}
}

func (o PNGDeliveryOrchestrator) now() func() time.Time {
	if o.Now != nil {
		return o.Now
	}
	return time.Now
}
