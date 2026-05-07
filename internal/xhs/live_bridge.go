package xhs

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrLiveSourceMissing          = errors.New("live source missing")
	ErrPhotosLookupFailed         = errors.New("live photos lookup failed")
	ErrLiveBridgeFailed           = errors.New("live bridge failed")
	ErrLiveBridgePermissionDenied = errors.New("live bridge permission denied")
	ErrLivePublishUnsupported     = errors.New("live publish unsupported")
)

type LiveAttachRequest struct {
	AlbumName string
	Items     []ResolvedLiveItem
}

type LiveAttachResult struct {
	AttachedCount int
	ItemNames     []string
	UIPreserved   bool
}

type LiveBridge interface {
	Attach(ctx context.Context, req LiveAttachRequest) (LiveAttachResult, error)
}

func newDefaultLiveBridge() LiveBridge {
	return osascriptLiveBridge{}
}

func buildLiveAttachRequest(request PublishRequest) (LiveAttachRequest, error) {
	if request.Live.Resolved == nil {
		return LiveAttachRequest{}, fmt.Errorf("%w: resolved live source is required", ErrLiveSourceMissing)
	}
	if len(request.Live.Resolved.Items) == 0 {
		return LiveAttachRequest{}, fmt.Errorf("%w: at least one live item is required", ErrLiveSourceMissing)
	}
	albumName := strings.TrimSpace(request.Live.Resolved.Report.AlbumName)
	if albumName == "" {
		return LiveAttachRequest{}, fmt.Errorf("%w: album name is required", ErrLiveSourceMissing)
	}
	items := append([]ResolvedLiveItem(nil), request.Live.Resolved.Items...)
	return LiveAttachRequest{AlbumName: albumName, Items: items}, nil
}

func liveAttachItemNames(items []ResolvedLiveItem) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, strings.TrimSpace(item.PageName))
	}
	return result
}
