package timing

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const prefix = "MARK2NOTE_TIMING"

var (
	outputMu sync.Mutex
	output   io.Writer = os.Stderr
	keyRe              = regexp.MustCompile(`[^A-Za-z0-9_\-]+`)
)

// Field returns an optional key=value detail for a timing line.
func Field(key string, value any) string {
	key = keyRe.ReplaceAllString(strings.TrimSpace(key), "_")
	key = strings.Trim(key, "_")
	if key == "" {
		return ""
	}
	valueText := strings.Join(strings.Fields(fmt.Sprint(value)), "_")
	valueText = strings.ReplaceAll(valueText, "/", "_")
	valueText = strings.ReplaceAll(valueText, `\\`, "_")
	if valueText == "" {
		return ""
	}
	return key + "=" + valueText
}

// SetOutput changes the timing output writer and returns the previous writer.
// It is intended for tests; production code writes to stderr by default.
func SetOutput(w io.Writer) io.Writer {
	outputMu.Lock()
	defer outputMu.Unlock()
	old := output
	if w == nil {
		output = io.Discard
	} else {
		output = w
	}
	return old
}

// Stage starts a timer and returns a function that emits one stable timing line.
func Stage(name string, details ...string) func(error) {
	start := time.Now()
	stage := sanitizeValue(name)
	return func(err error) {
		elapsed := time.Since(start).Milliseconds()
		if elapsed < 0 {
			elapsed = 0
		}
		status := "ok"
		if err != nil {
			status = "error"
		}
		fields := []string{prefix, Field("stage", stage), Field("elapsed_ms", elapsed), Field("status", status)}
		for _, detail := range details {
			if strings.TrimSpace(detail) != "" {
				fields = append(fields, detail)
			}
		}
		outputMu.Lock()
		defer outputMu.Unlock()
		fmt.Fprintln(output, strings.Join(fields, " "))
	}
}

func sanitizeValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Join(strings.Fields(value), "_")
	value = strings.ReplaceAll(value, string(os.PathSeparator), "_")
	if value == "" {
		return "unknown"
	}
	return value
}
