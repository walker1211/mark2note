package render

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type LiveDeliveryOrchestrator struct {
	Scanner  livePairScanner
	Importer PhotosImporter
	Writer   ImportResultWriter
	Now      func() time.Time
}

type liveDeliveryRequest struct {
	SourceDir     string
	ImportDir     string
	AlbumName     string
	ImportTimeout time.Duration
}

type liveDeliveryResult struct {
	Report     DeliveryReport
	ReportPath string
}

func (o LiveDeliveryOrchestrator) Deliver(req liveDeliveryRequest) (liveDeliveryResult, error) {
	importDir := req.ImportDir
	if stringsTrim(importDir) == "" {
		importDir = req.SourceDir
	}
	scanResult, err := o.scanner().Scan(importDir)
	if err != nil {
		return liveDeliveryResult{}, err
	}
	report := DeliveryReport{
		SourceDir:      req.SourceDir,
		CandidatePairs: len(scanResult.Pairs),
		SkippedPairs:   len(scanResult.SkippedItems),
		SkippedItems:   append([]SkippedItem(nil), scanResult.SkippedItems...),
	}
	if report.CandidatePairs == 0 {
		report.Status = deliveryStatusFailed
		report.Message = "no candidate live photo pairs found"
		return o.writeReport(report, errors.New(report.Message))
	}

	ctx, cancel := context.WithTimeout(context.Background(), req.ImportTimeout)
	defer cancel()

	if err := o.importer().CheckAvailable(ctx); err != nil {
		report.Status = deliveryStatusFailed
		report.Message = err.Error()
		return o.writeReport(reportWithErrorAlbum(report, req.AlbumName), err)
	}
	albumName, err := finalizeAlbumName(ctx, o.importer(), req.AlbumName, o.now()())
	if err != nil {
		report.Status = deliveryStatusFailed
		report.Message = err.Error()
		return o.writeReport(reportWithErrorAlbum(report, req.AlbumName), err)
	}
	importResult, err := o.importer().ImportDirectory(ctx, ImportPhotosRequest{SourceDir: importDir, AlbumName: albumName})
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

func reportWithErrorAlbum(report DeliveryReport, requested string) DeliveryReport {
	report.AlbumName = stringsTrim(requested)
	return report
}

func (o LiveDeliveryOrchestrator) writeReport(report DeliveryReport, cause ...error) (liveDeliveryResult, error) {
	path, err := o.writer().Write(report)
	if err != nil {
		return liveDeliveryResult{}, fmt.Errorf("write delivery report: %w", err)
	}
	result := liveDeliveryResult{Report: report, ReportPath: path}
	if len(cause) > 0 && cause[0] != nil {
		return result, cause[0]
	}
	return result, nil
}

func (o LiveDeliveryOrchestrator) importer() PhotosImporter {
	if o.Importer != nil {
		return o.Importer
	}
	return osascriptPhotosImporter{}
}

func (o LiveDeliveryOrchestrator) writer() ImportResultWriter {
	if o.Writer != nil {
		return o.Writer
	}
	return importResultWriter{}
}

func (o LiveDeliveryOrchestrator) scanner() livePairScanner {
	return o.Scanner
}

func (o LiveDeliveryOrchestrator) now() func() time.Time {
	if o.Now != nil {
		return o.Now
	}
	return time.Now
}
