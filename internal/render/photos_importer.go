package render

import (
	"context"
	"errors"
)

type PhotosImporter interface {
	CheckAvailable(ctx context.Context) error
	EnsureAlbum(ctx context.Context, name string) (EnsureAlbumResult, error)
	ImportDirectory(ctx context.Context, req ImportPhotosRequest) (RawImportResult, error)
}

type EnsureAlbumResult struct {
	AlbumName string
	Created   bool
}

type ImportPhotosRequest struct {
	SourceDir string
	AlbumName string
}

type RawImportResult struct {
	Executed bool
	Message  string
}

var (
	ErrUnsupportedPlatform        = errors.New("unsupported platform")
	ErrAutomationPermissionDenied = errors.New("photos automation permission denied")
	ErrAlbumAmbiguous             = errors.New("photos album ambiguous")
	ErrTimeout                    = errors.New("photos automation timeout")
	ErrScriptExecution            = errors.New("photos script execution failed")
)
