package timing

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStageWritesStableOKLine(t *testing.T) {
	var buf bytes.Buffer
	old := SetOutput(&buf)
	defer SetOutput(old)

	done := Stage("render", Field("pages", 3))
	time.Sleep(time.Millisecond)
	done(nil)

	line := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(line, "MARK2NOTE_TIMING ") {
		t.Fatalf("line prefix = %q", line)
	}
	for _, want := range []string{"stage=render", "elapsed_ms=", "status=ok", "pages=3"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q does not contain %q", line, want)
		}
	}
	if strings.Contains(line, "/Users/") {
		t.Fatalf("line leaks local absolute path: %q", line)
	}
}

func TestStageWritesErrorStatus(t *testing.T) {
	var buf bytes.Buffer
	old := SetOutput(&buf)
	defer SetOutput(old)

	done := Stage("publish")
	done(errors.New("boom"))

	line := strings.TrimSpace(buf.String())
	if !strings.Contains(line, "stage=publish") || !strings.Contains(line, "status=error") {
		t.Fatalf("unexpected timing line: %q", line)
	}
}

func TestFieldDoesNotPreserveAbsolutePathShape(t *testing.T) {
	field := Field("path", "/Users/me/project/file.md")
	if strings.Contains(field, "/Users/") || strings.Contains(field, "/") {
		t.Fatalf("field preserves path shape: %q", field)
	}
}
