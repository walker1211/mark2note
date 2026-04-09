package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/walker1211/mark2note/internal/app"
	"github.com/walker1211/mark2note/internal/render"
)

func TestRunPrintsLiveDeliverySummaryWhenReportExists(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	reportPath := filepath.Join(t.TempDir(), "apple-live", "import-result.json")
	generatePreview = func(Options) (app.Result, error) {
		return app.Result{
			PageCount: 3,
			OutDir:    t.TempDir(),
			DeliveryReport: &render.DeliveryReport{
				Status:  "partial",
				Message: "import command submitted to Photos; manual verification required",
			},
			DeliveryReportPath: reportPath,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "live delivery: partial") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), reportPath) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunReturnsZeroWhenDeliveryStatusIsPartial(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	generatePreview = func(Options) (app.Result, error) {
		return app.Result{PageCount: 3, OutDir: t.TempDir(), DeliveryReport: &render.DeliveryReport{Status: "partial", Message: "manual verification required"}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0, stderr = %s", code, stderr.String())
	}
}

func TestRunReturnsOneWhenDeliveryFails(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	generatePreview = func(Options) (app.Result, error) {
		return app.Result{}, fmt.Errorf("%w: photos automation permission denied", app.ErrRenderPreview)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "render preview failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPrintsFailedDeliveryReportPathWhenAvailable(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	reportPath := filepath.Join(t.TempDir(), "apple-live", "import-result.json")
	generatePreview = func(Options) (app.Result, error) {
		return app.Result{
			DeliveryReport:     &render.DeliveryReport{Status: "failed", Message: "photos automation permission denied"},
			DeliveryReportPath: reportPath,
		}, fmt.Errorf("%w: photos automation permission denied", app.ErrRenderPreview)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), reportPath) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
