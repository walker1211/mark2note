package render

import (
	"context"
	"fmt"
	"time"
)

func defaultImportAlbumName(now time.Time) string {
	return defaultImportAlbumNameWithPrefix("mark2note-live", now)
}

func defaultImportAlbumNameWithPrefix(prefix string, now time.Time) string {
	return prefix + "-" + now.Format("20060102-150405")
}

func finalizeAlbumName(ctx context.Context, importer PhotosImporter, requested string, now time.Time) (string, error) {
	return finalizeAlbumNameWithPrefix(ctx, importer, requested, now, "mark2note-live")
}

func finalizeAlbumNameWithPrefix(ctx context.Context, importer PhotosImporter, requested string, now time.Time, prefix string) (string, error) {
	trimmed := stringsTrim(requested)
	if trimmed != "" {
		result, err := importer.EnsureAlbum(ctx, trimmed)
		if err != nil {
			return "", err
		}
		return result.AlbumName, nil
	}

	base := defaultImportAlbumNameWithPrefix(prefix, now)
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
