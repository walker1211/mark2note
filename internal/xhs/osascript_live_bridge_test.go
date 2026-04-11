package xhs

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeLiveScriptExecutor struct {
	output []byte
	err    error
	calls  [][]string
}

func (f *fakeLiveScriptExecutor) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)
	return append([]byte(nil), f.output...), f.err
}

func TestOsaScriptLiveBridgeBuildsAttachSequence(t *testing.T) {
	executor := &fakeLiveScriptExecutor{output: []byte(`{"AttachedCount":2,"ItemNames":["p02-bullets","p01-cover"],"UIPreserved":true}`)}
	bridge := osascriptLiveBridge{Executor: executor, GOOS: func() string { return "darwin" }}

	got, err := bridge.Attach(context.Background(), LiveAttachRequest{
		AlbumName: "mark2note-live",
		Items: []ResolvedLiveItem{
			{PageName: "p02-bullets"},
			{PageName: "p01-cover"},
		},
	})
	if err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	want := LiveAttachResult{AttachedCount: 2, ItemNames: []string{"p02-bullets", "p01-cover"}, UIPreserved: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Attach() = %#v, want %#v", got, want)
	}
	assertSingleLiveOsaScriptCall(t, executor.calls)
	assertLiveCallContains(t, executor.calls[0], `set targetAlbum to "mark2note-live"`)
	assertLiveCallContains(t, executor.calls[0], `set targetItems to {"p02-bullets", "p01-cover"}`)
	assertLiveCallContains(t, executor.calls[0], `tell application "Photos"`)
	assertLiveCallContains(t, executor.calls[0], `tell process "Google Chrome"`)
	assertLiveCallContains(t, executor.calls[0], `return "{\"AttachedCount\":"`)
}

func TestOsaScriptLiveBridgeMapsPermissionDenied(t *testing.T) {
	executor := &fakeLiveScriptExecutor{output: []byte("Photos got an error: Not authorized to send Apple events to Photos. (-1743)"), err: errors.New("exit status 1")}
	bridge := osascriptLiveBridge{Executor: executor, GOOS: func() string { return "darwin" }}

	_, err := bridge.Attach(context.Background(), LiveAttachRequest{AlbumName: "mark2note-live", Items: []ResolvedLiveItem{{PageName: "p01-cover"}}})
	if !errors.Is(err, ErrLiveBridgePermissionDenied) {
		t.Fatalf("Attach() error = %v, want ErrLiveBridgePermissionDenied", err)
	}
}

func TestOsaScriptLiveBridgeRejectsNonDarwin(t *testing.T) {
	bridge := osascriptLiveBridge{GOOS: func() string { return "linux" }}

	_, err := bridge.Attach(context.Background(), LiveAttachRequest{AlbumName: "mark2note-live", Items: []ResolvedLiveItem{{PageName: "p01-cover"}}})
	if !errors.Is(err, ErrLivePublishUnsupported) {
		t.Fatalf("Attach() error = %v, want ErrLivePublishUnsupported", err)
	}
}

func TestOsaScriptLiveBridgePreservesTimeoutAndCancel(t *testing.T) {
	bridgeTimeout := osascriptLiveBridge{Executor: &fakeLiveScriptExecutor{err: context.DeadlineExceeded}, GOOS: func() string { return "darwin" }}
	_, timeoutErr := bridgeTimeout.Attach(context.Background(), LiveAttachRequest{AlbumName: "mark2note-live", Items: []ResolvedLiveItem{{PageName: "p01-cover"}}})
	if !errors.Is(timeoutErr, ErrLiveBridgeFailed) || !errors.Is(timeoutErr, context.DeadlineExceeded) {
		t.Fatalf("Attach() timeout error = %v", timeoutErr)
	}

	bridgeCanceled := osascriptLiveBridge{Executor: &fakeLiveScriptExecutor{err: context.Canceled}, GOOS: func() string { return "darwin" }}
	_, canceledErr := bridgeCanceled.Attach(context.Background(), LiveAttachRequest{AlbumName: "mark2note-live", Items: []ResolvedLiveItem{{PageName: "p01-cover"}}})
	if !errors.Is(canceledErr, ErrLiveBridgeFailed) || !errors.Is(canceledErr, context.Canceled) {
		t.Fatalf("Attach() canceled error = %v", canceledErr)
	}
}

func assertSingleLiveOsaScriptCall(t *testing.T, calls [][]string) {
	t.Helper()
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if len(calls[0]) < 3 {
		t.Fatalf("call = %#v, want osascript -e <script>", calls[0])
	}
	if calls[0][0] != "osascript" || calls[0][1] != "-e" {
		t.Fatalf("call = %#v, want osascript -e <script>", calls[0])
	}
}

func assertLiveCallContains(t *testing.T, call []string, want string) {
	t.Helper()
	got := strings.Join(call, "\n")
	if !strings.Contains(got, want) {
		t.Fatalf("call missing %q: %s", want, got)
	}
}
