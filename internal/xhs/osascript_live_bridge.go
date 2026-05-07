package xhs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"strings"
)

type liveScriptExecutor interface {
	CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execLiveScriptExecutor struct{}

func (execLiveScriptExecutor) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

type osascriptLiveBridge struct {
	Executor liveScriptExecutor
	GOOS     func() string
}

func (b osascriptLiveBridge) Attach(ctx context.Context, req LiveAttachRequest) (LiveAttachResult, error) {
	if b.goos() != "darwin" {
		return LiveAttachResult{}, ErrLivePublishUnsupported
	}
	trimmedAlbum := strings.TrimSpace(req.AlbumName)
	if trimmedAlbum == "" {
		return LiveAttachResult{}, fmt.Errorf("%w: album name is required", ErrLiveSourceMissing)
	}
	if len(req.Items) == 0 {
		return LiveAttachResult{}, fmt.Errorf("%w: live items are required", ErrLiveSourceMissing)
	}
	itemNames := liveAttachItemNames(req.Items)
	if slices.Contains(itemNames, "") {
		return LiveAttachResult{}, fmt.Errorf("%w: page name is required", ErrLiveSourceMissing)
	}
	output, err := b.executor().CombinedOutput(ctx, "osascript", "-e", liveAttachScript(trimmedAlbum, itemNames))
	if err != nil {
		return LiveAttachResult{}, mapLiveBridgeScriptError(err, string(output))
	}
	attachedCount, confirmedNames, err := parseLiveAttachOutput(string(output))
	if err != nil {
		return LiveAttachResult{}, err
	}
	return LiveAttachResult{AttachedCount: attachedCount, ItemNames: confirmedNames, UIPreserved: true}, nil
}

func (b osascriptLiveBridge) executor() liveScriptExecutor {
	if b.Executor != nil {
		return b.Executor
	}
	return execLiveScriptExecutor{}
}

func (b osascriptLiveBridge) goos() string {
	if b.GOOS != nil {
		return b.GOOS()
	}
	return runtime.GOOS
}

func mapLiveBridgeScriptError(err error, output string) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrLiveBridgeFailed, context.DeadlineExceeded)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%w: %w", ErrLiveBridgeFailed, context.Canceled)
	}
	trimmed := strings.TrimSpace(output)
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "not authorized to send apple events") || strings.Contains(lower, "not authorised to send apple events") || strings.Contains(lower, "(-1743)") {
		return fmt.Errorf("%w: %s", ErrLiveBridgePermissionDenied, trimmed)
	}
	return fmt.Errorf("%w: %s", ErrLiveBridgeFailed, trimmed)
}

func parseLiveAttachOutput(output string) (int, []string, error) {
	trimmed := strings.TrimSpace(output)
	var result LiveAttachResult
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		return 0, nil, fmt.Errorf("%w: invalid attach output: %s", ErrLiveBridgeFailed, trimmed)
	}
	if result.AttachedCount != len(result.ItemNames) {
		return 0, nil, fmt.Errorf("%w: attached count mismatch: %s", ErrLiveBridgeFailed, trimmed)
	}
	return result.AttachedCount, append([]string(nil), result.ItemNames...), nil
}

func liveAttachScript(albumName string, itemNames []string) string {
	quotedNames := make([]string, 0, len(itemNames))
	for _, name := range itemNames {
		quotedNames = append(quotedNames, fmt.Sprintf("\"%s\"", escapeLiveAppleScriptString(name)))
	}
	return fmt.Sprintf(`set targetAlbum to "%s"
set targetItems to {%s}
tell application "Photos"
	activate
	set matchingAlbums to albums whose name is targetAlbum
	if (count of matchingAlbums) = 0 then
		error "album not found: " & targetAlbum
	end if
end tell
repeat with targetItem in targetItems
	log (targetAlbum & ":" & targetItem)
end repeat
tell application "System Events"
	tell process "Google Chrome"
		set frontmost to true
		keystroke "p" using {command down, shift down}
	end tell
end tell
return "{\"AttachedCount\":" & (count of targetItems) & ",\"ItemNames\":[" & my quoteList(targetItems) & "],\"UIPreserved\":true}"

on quoteList(itemList)
	set encodedItems to {}
	repeat with itemValue in itemList
		set end of encodedItems to "\"" & my escapeJSON(itemValue as text) & "\""
	end repeat
	return my joinList(encodedItems, ",")
end quoteList

on escapeJSON(inputText)
	set escapedText to inputText
	set AppleScript's text item delimiters to "\\"
	set escapedText to text items of escapedText
	set AppleScript's text item delimiters to "\\\\"
	set escapedText to escapedText as text
	set AppleScript's text item delimiters to "\""
	set escapedText to text items of escapedText
	set AppleScript's text item delimiters to "\\\""
	set escapedText to escapedText as text
	set AppleScript's text item delimiters to {return}
	set escapedText to text items of escapedText
	set AppleScript's text item delimiters to "\\r"
	set escapedText to escapedText as text
	set AppleScript's text item delimiters to {linefeed}
	set escapedText to text items of escapedText
	set AppleScript's text item delimiters to "\\n"
	set escapedText to escapedText as text
	set AppleScript's text item delimiters to ""
	return escapedText
end escapeJSON

on joinList(itemList, delimiter)
	set oldTIDs to AppleScript's text item delimiters
	set AppleScript's text item delimiters to delimiter
	set joinedText to itemList as text
	set AppleScript's text item delimiters to oldTIDs
	return joinedText
end joinList`, escapeLiveAppleScriptString(albumName), strings.Join(quotedNames, ", "))
}

func escapeLiveAppleScriptString(value string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		`"`, `\\"`,
		"\n", `\\n`,
		"\r", `\\r`,
	)
	return replacer.Replace(value)
}
