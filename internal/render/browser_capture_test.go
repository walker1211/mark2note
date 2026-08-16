package render

import (
	"errors"
	"strings"
	"testing"
)

func TestWaitForMatchingPaintProbesWaitsForTwoEqualFrames(t *testing.T) {
	frames := [][]byte{[]byte("partial"), []byte("complete"), []byte("complete")}
	captures := 0
	waits := 0

	err := waitForMatchingPaintProbes(func() ([]byte, error) {
		frame := frames[captures]
		captures++
		return frame, nil
	}, func() error {
		waits++
		return nil
	}, 4)
	if err != nil {
		t.Fatalf("waitForMatchingPaintProbes() error = %v", err)
	}
	if captures != 3 || waits != 2 {
		t.Fatalf("captures=%d waits=%d, want captures=3 waits=2", captures, waits)
	}
}

func TestWaitForMatchingPaintProbesRejectsUnstableFrames(t *testing.T) {
	captures := 0
	err := waitForMatchingPaintProbes(func() ([]byte, error) {
		captures++
		return []byte{byte(captures)}, nil
	}, func() error { return nil }, 3)
	if err == nil || !strings.Contains(err.Error(), "did not stabilize after 3 probes") {
		t.Fatalf("waitForMatchingPaintProbes() error = %v, want stability failure", err)
	}
}

func TestWaitForMatchingPaintProbesPropagatesCaptureError(t *testing.T) {
	want := errors.New("capture failed")
	err := waitForMatchingPaintProbes(func() ([]byte, error) {
		return nil, want
	}, func() error { return nil }, 2)
	if !errors.Is(err, want) {
		t.Fatalf("waitForMatchingPaintProbes() error = %v, want %v", err, want)
	}
}

func TestWaitForMatchingPaintProbesPropagatesWaitError(t *testing.T) {
	want := errors.New("wait failed")
	err := waitForMatchingPaintProbes(func() ([]byte, error) {
		return []byte("frame"), nil
	}, func() error { return want }, 2)
	if !errors.Is(err, want) {
		t.Fatalf("waitForMatchingPaintProbes() error = %v, want %v", err, want)
	}
}
