package render

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type liveAssetPair struct {
	BaseName  string
	PhotoPath string
	VideoPath string
}

type liveScanResult struct {
	Pairs        []liveAssetPair
	SkippedItems []SkippedItem
}

type livePairScanner struct{}

type livePairAccumulation struct {
	items map[string]livePairEntry
}

type livePairEntry struct {
	photoPath string
	videoPath string
}

func (livePairScanner) Scan(sourceDir string) (liveScanResult, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return liveScanResult{}, err
	}

	var acc livePairAccumulation
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		fullPath := filepath.Join(sourceDir, name)

		switch ext {
		case ".jpg":
			if err := acc.addPhoto(base, fullPath); err != nil {
				return liveScanResult{}, err
			}
		case ".mov":
			if err := acc.addVideo(base, fullPath); err != nil {
				return liveScanResult{}, err
			}
		}
	}

	return acc.result(), nil
}

func (a *livePairAccumulation) addPhoto(baseName string, path string) error {
	return a.add(baseName, path, true)
}

func (a *livePairAccumulation) addVideo(baseName string, path string) error {
	return a.add(baseName, path, false)
}

func (a *livePairAccumulation) add(baseName string, path string, isPhoto bool) error {
	if a.items == nil {
		a.items = make(map[string]livePairEntry)
	}
	item := a.items[baseName]
	if isPhoto {
		if item.photoPath != "" {
			return fmt.Errorf("duplicate live asset basename: %s", baseName)
		}
		item.photoPath = path
	} else {
		if item.videoPath != "" {
			return fmt.Errorf("duplicate live asset basename: %s", baseName)
		}
		item.videoPath = path
	}
	a.items[baseName] = item
	return nil
}

func (a livePairAccumulation) result() liveScanResult {
	result := liveScanResult{}
	if len(a.items) == 0 {
		return result
	}

	baseNames := make([]string, 0, len(a.items))
	for baseName := range a.items {
		baseNames = append(baseNames, baseName)
	}
	sort.Strings(baseNames)

	for _, baseName := range baseNames {
		item := a.items[baseName]
		switch {
		case item.photoPath != "" && item.videoPath != "":
			result.Pairs = append(result.Pairs, liveAssetPair{
				BaseName:  baseName,
				PhotoPath: item.photoPath,
				VideoPath: item.videoPath,
			})
		case item.photoPath != "":
			result.SkippedItems = append(result.SkippedItems, SkippedItem{BaseName: baseName, PhotoPath: item.photoPath, VideoPath: "", Message: "missing .mov pair"})
		case item.videoPath != "":
			result.SkippedItems = append(result.SkippedItems, SkippedItem{BaseName: baseName, PhotoPath: "", VideoPath: item.videoPath, Message: "missing .jpg pair"})
		}
	}

	return result
}
