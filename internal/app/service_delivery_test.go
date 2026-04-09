package app

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/walker1211/mark2note/internal/config"
	"github.com/walker1211/mark2note/internal/render"
)

func TestServiceGeneratePreviewReturnsDeliveryReport(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: "configured-output"}}
	reportPath := filepath.Join(t.TempDir(), "apple-live", "import-result.json")
	report := &render.DeliveryReport{
		SourceDir: filepath.Dir(reportPath),
		AlbumName: "mark2note-live-20260409-123456",
		Status:    "partial",
		Message:   "import command submitted to Photos; manual verification required",
	}
	r := &fakeRenderer{result: render.RenderResult{DeliveryReport: report, DeliveryReportPath: reportPath}}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:   func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			return `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`,
				nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	result, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if result.DeliveryReport == nil || result.DeliveryReport.Status != "partial" {
		t.Fatalf("DeliveryReport = %#v", result.DeliveryReport)
	}
	if result.DeliveryReportPath != reportPath {
		t.Fatalf("DeliveryReportPath = %q, want %q", result.DeliveryReportPath, reportPath)
	}
}

func TestServiceGeneratePreviewReturnsFailedDeliveryReportWithError(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: "configured-output"}}
	reportPath := filepath.Join(t.TempDir(), "apple-live", "import-result.json")
	report := &render.DeliveryReport{
		SourceDir: filepath.Dir(reportPath),
		AlbumName: "mark2note-live-20260409-123456",
		Status:    "failed",
		Message:   "photos automation permission denied",
	}
	r := &fakeRenderer{
		result: render.RenderResult{DeliveryReport: report, DeliveryReportPath: reportPath},
		err:    errors.New("photos automation permission denied"),
	}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:   func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			return `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`,
				nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	result, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err == nil {
		t.Fatal("GeneratePreview() error = nil, want error")
	}
	if !errors.Is(err, ErrRenderPreview) {
		t.Fatalf("GeneratePreview() error = %v, want ErrRenderPreview", err)
	}
	if result.DeliveryReport == nil || result.DeliveryReport.Status != "failed" {
		t.Fatalf("DeliveryReport = %#v", result.DeliveryReport)
	}
	if result.DeliveryReportPath != reportPath {
		t.Fatalf("DeliveryReportPath = %q, want %q", result.DeliveryReportPath, reportPath)
	}
}
