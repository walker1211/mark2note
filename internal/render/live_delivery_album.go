package render

import (
	"context"
	"fmt"
	"time"
)

func defaultImportAlbumName(now time.Time) string {
	return "mark2note-live-" + now.Format("20060102-150405")
}

func finalizeAlbumName(ctx context.Context, importer PhotosImporter, requested string, now time.Time) (string, error) {
	trimmed := stringsTrim(requested)
	if trimmed != "" {
		result, err := importer.EnsureAlbum(ctx, trimmed)
		if err != nil {
			return "", err
		}
		return result.AlbumName, nil
	}

	base := defaultImportAlbumName(now)
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate = fmt.Sprintf("%s-%d", base, suffix)
		}
		result, err := importer.EnsureAlbum(ctx, candidate)
		if err != nil {
			return "", err
		}
		if result.Created {
			return result.AlbumName, nil
		}
	}
}
