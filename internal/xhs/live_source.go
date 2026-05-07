package xhs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/walker1211/mark2note/internal/render"
)

type ResolvedLiveItem struct {
	PageName  string
	PhotoPath string
	VideoPath string
}

type ResolvedLiveSource struct {
	Report render.DeliveryReport
	Items  []ResolvedLiveItem
}

func ResolveLiveSource(reportPath string, requested []string) (ResolvedLiveSource, error) {
	trimmedReportPath := expandUserHome(strings.TrimSpace(reportPath))
	if trimmedReportPath == "" {
		return ResolvedLiveSource{}, fmt.Errorf("live report path is required")
	}
	content, err := os.ReadFile(trimmedReportPath)
	if err != nil {
		return ResolvedLiveSource{}, fmt.Errorf("read live delivery report: %w", err)
	}
	var report render.DeliveryReport
	if err := json.Unmarshal(content, &report); err != nil {
		return ResolvedLiveSource{}, fmt.Errorf("decode live delivery report: %w", err)
	}
	if strings.TrimSpace(report.Status) != "partial" {
		return ResolvedLiveSource{}, fmt.Errorf("live delivery report status must be partial")
	}
	sourceDir := strings.TrimSpace(report.SourceDir)
	if sourceDir == "" {
		return ResolvedLiveSource{}, fmt.Errorf("live delivery report source_dir is required")
	}
	if !filepath.IsAbs(sourceDir) {
		return ResolvedLiveSource{}, fmt.Errorf("live delivery report source_dir must be absolute")
	}
	items, err := scanResolvedLiveItems(sourceDir)
	if err != nil {
		return ResolvedLiveSource{}, err
	}
	if len(items) == 0 {
		return ResolvedLiveSource{}, fmt.Errorf("no resolved live items found in %s", sourceDir)
	}
	selected, err := selectResolvedLiveItems(items, trimmedNonEmpty(requested))
	if err != nil {
		return ResolvedLiveSource{}, err
	}
	return ResolvedLiveSource{Report: report, Items: selected}, nil
}

type resolvedLiveEntry struct {
	photoPath string
	videoPath string
}

func scanResolvedLiveItems(sourceDir string) ([]ResolvedLiveItem, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("scan live source dir: %w", err)
	}
	itemsByBaseName := make(map[string]resolvedLiveEntry)
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		fullPath := filepath.Join(sourceDir, name)
		item := itemsByBaseName[base]
		switch ext {
		case ".jpg":
			item.photoPath = fullPath
		case ".mov":
			item.videoPath = fullPath
		default:
			continue
		}
		itemsByBaseName[base] = item
	}
	baseNames := make([]string, 0, len(itemsByBaseName))
	for baseName := range itemsByBaseName {
		baseNames = append(baseNames, baseName)
	}
	sort.Strings(baseNames)
	items := make([]ResolvedLiveItem, 0, len(baseNames))
	for _, baseName := range baseNames {
		item := itemsByBaseName[baseName]
		if item.photoPath == "" || item.videoPath == "" {
			continue
		}
		items = append(items, ResolvedLiveItem{PageName: baseName, PhotoPath: item.photoPath, VideoPath: item.videoPath})
	}
	return items, nil
}

func selectResolvedLiveItems(items []ResolvedLiveItem, requested []string) ([]ResolvedLiveItem, error) {
	if len(requested) == 0 {
		return append([]ResolvedLiveItem(nil), items...), nil
	}
	itemsByPageName := make(map[string]ResolvedLiveItem, len(items))
	for _, item := range items {
		itemsByPageName[item.PageName] = item
	}
	seen := make(map[string]struct{}, len(requested))
	selected := make([]ResolvedLiveItem, 0, len(requested))
	for _, pageName := range requested {
		if _, duplicated := seen[pageName]; duplicated {
			return nil, fmt.Errorf("requested live page must be unique: %s", pageName)
		}
		seen[pageName] = struct{}{}
		item, ok := itemsByPageName[pageName]
		if !ok {
			return nil, fmt.Errorf("requested live page must map to exactly one imported item: %s", pageName)
		}
		selected = append(selected, item)
	}
	return selected, nil
}
