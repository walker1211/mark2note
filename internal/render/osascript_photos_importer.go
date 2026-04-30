package render

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const photosImportSubmittedMessage = "import command submitted to Photos; manual verification required"

type scriptExecutor interface {
	CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execScriptExecutor struct{}

func (execScriptExecutor) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

type osascriptPhotosImporter struct {
	Executor scriptExecutor
	GOOS     func() string
}

func (i osascriptPhotosImporter) CheckAvailable(ctx context.Context) error {
	if i.goos() != "darwin" {
		return ErrUnsupportedPlatform
	}
	output, err := i.executor().CombinedOutput(ctx, "osascript", "-e", photosAvailabilityScript())
	if err != nil {
		return mapPhotosScriptError(err, string(output))
	}
	if stringsTrim(string(output)) != "OK" {
		return fmt.Errorf("%w: unexpected availability output: %s", ErrScriptExecution, stringsTrim(string(output)))
	}
	return nil
}

func (i osascriptPhotosImporter) EnsureAlbum(ctx context.Context, name string) (EnsureAlbumResult, error) {
	if i.goos() != "darwin" {
		return EnsureAlbumResult{}, ErrUnsupportedPlatform
	}
	albumName := stringsTrim(name)
	if albumName == "" {
		return EnsureAlbumResult{}, fmt.Errorf("ensure album: album name is required")
	}
	output, err := i.executor().CombinedOutput(ctx, "osascript", "-e", photosEnsureAlbumScript(albumName))
	if err != nil {
		return EnsureAlbumResult{}, mapPhotosScriptError(err, string(output))
	}
	result := stringsTrim(string(output))
	switch {
	case strings.HasPrefix(result, "FOUND:"):
		resolvedName := stringsTrim(strings.TrimPrefix(result, "FOUND:"))
		if resolvedName == "" {
			return EnsureAlbumResult{}, fmt.Errorf("%w: empty found album name", ErrScriptExecution)
		}
		return EnsureAlbumResult{AlbumName: resolvedName, Created: false}, nil
	case strings.HasPrefix(result, "CREATED:"):
		resolvedName := stringsTrim(strings.TrimPrefix(result, "CREATED:"))
		if resolvedName == "" {
			return EnsureAlbumResult{}, fmt.Errorf("%w: empty created album name", ErrScriptExecution)
		}
		return EnsureAlbumResult{AlbumName: resolvedName, Created: true}, nil
	case result == "AMBIGUOUS":
		return EnsureAlbumResult{}, ErrAlbumAmbiguous
	default:
		return EnsureAlbumResult{}, fmt.Errorf("%w: unexpected ensure album output: %s", ErrScriptExecution, result)
	}
}

func (i osascriptPhotosImporter) ImportDirectory(ctx context.Context, req ImportPhotosRequest) (RawImportResult, error) {
	if i.goos() != "darwin" {
		return RawImportResult{}, ErrUnsupportedPlatform
	}
	sourceDir := stringsTrim(req.SourceDir)
	if sourceDir == "" || !filepath.IsAbs(sourceDir) {
		return RawImportResult{}, fmt.Errorf("import photos: source dir must be absolute")
	}
	albumName := stringsTrim(req.AlbumName)
	if albumName == "" {
		return RawImportResult{}, fmt.Errorf("import photos: album name is required")
	}
	output, err := i.executor().CombinedOutput(ctx, "osascript", "-e", photosImportDirectoryScript(sourceDir, albumName))
	if err != nil {
		return RawImportResult{}, mapPhotosScriptError(err, string(output))
	}
	result := stringsTrim(string(output))
	if !strings.HasPrefix(result, "IMPORTED:") {
		return RawImportResult{}, fmt.Errorf("%w: unexpected import output: %s", ErrScriptExecution, result)
	}
	return RawImportResult{Executed: true, Message: photosImportSubmittedMessage}, nil
}

func (i osascriptPhotosImporter) executor() scriptExecutor {
	if i.Executor != nil {
		return i.Executor
	}
	return execScriptExecutor{}
}

func (i osascriptPhotosImporter) goos() string {
	if i.GOOS != nil {
		return i.GOOS()
	}
	return runtime.GOOS
}

func mapPhotosScriptError(err error, output string) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", ErrTimeout, err)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("photos automation canceled: %w", err)
	}
	text := strings.ToLower(output)
	if strings.Contains(text, "not authorized to send apple events") || strings.Contains(text, "not authorised to send apple events") || strings.Contains(text, "(-1743)") {
		return fmt.Errorf("%w: %s", ErrAutomationPermissionDenied, stringsTrim(output))
	}
	return fmt.Errorf("%w: %s", ErrScriptExecution, stringsTrim(output))
}

func photosAvailabilityScript() string {
	return `tell application "Photos"
	activate
	return "OK"
end tell`
}

func photosEnsureAlbumScript(name string) string {
	escaped := escapeAppleScriptString(name)
	return fmt.Sprintf(`set targetName to "%s"
tell application "Photos"
	set exactMatches to albums whose name is targetName
	if (count of exactMatches) = 1 then
		return "FOUND:" & targetName
	else if (count of exactMatches) > 1 then
		return "AMBIGUOUS"
	else
		make new album named targetName
		return "CREATED:" & targetName
	end if
end tell`, escaped)
}

func photosImportDirectoryScript(sourceDir string, albumName string) string {
	escapedSourceDir := escapeAppleScriptString(sourceDir)
	escapedAlbumName := escapeAppleScriptString(albumName)
	return fmt.Sprintf(`set sourceDir to POSIX file "%s"
set targetName to "%s"
tell application "Photos"
	set targetAlbum to first album whose name is targetName
	import sourceDir into targetAlbum
	return "IMPORTED:" & targetName
end tell`, escapedSourceDir, escapedAlbumName)
}

func escapeAppleScriptString(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
	)
	return replacer.Replace(value)
}
