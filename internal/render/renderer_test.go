package render

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/walker1211/mark2note/internal/deck"
)

type fakeRunner struct {
	mu    sync.Mutex
	calls [][]string
}

func (r *fakeRunner) Run(name string, args ...string) error {
	call := append([]string{name}, args...)
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	return nil
}

func (r *fakeRunner) snapshotCalls() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([][]string, len(r.calls))
	for i := range r.calls {
		out[i] = append([]string(nil), r.calls[i]...)
	}
	return out
}

type concurrentRunner struct {
	delay time.Duration

	mu        sync.Mutex
	active    int
	maxActive int
}

func (r *concurrentRunner) Run(name string, args ...string) error {
	r.mu.Lock()
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.mu.Unlock()

	time.Sleep(r.delay)

	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	return nil
}

func (r *concurrentRunner) max() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxActive
}

type failRunner struct {
	failName string
	failErr  error

	mu    sync.Mutex
	calls [][]string
}

func (r *failRunner) Run(name string, args ...string) error {
	call := append([]string{name}, args...)
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	if name == r.failName {
		return r.failErr
	}
	return nil
}

func (r *failRunner) snapshotCalls() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([][]string, len(r.calls))
	for i := range r.calls {
		out[i] = append([]string(nil), r.calls[i]...)
	}
	return out
}

func sampleDeck(outDir string) deck.Deck {
	return deck.Deck{
		OutDir:    outDir,
		ThemeName: deck.ThemeDefault,
		Themes:    deck.RegisteredThemes(),
		Pages: []deck.Page{
			{Name: "p01-cover", Variant: "cover", Meta: deck.PageMeta{Badge: "第 1 页", Counter: "1/3", Theme: "orange", CTA: "cta1"}, Content: deck.PageContent{Title: "封面", Subtitle: "副标题1"}},
			{Name: "p02-bullets", Variant: "bullets", Meta: deck.PageMeta{Badge: "第 2 页", Counter: "2/3", Theme: "orange", CTA: "cta2"}, Content: deck.PageContent{Title: "中间", Items: []string{"要点1"}}},
			{Name: "p03-ending", Variant: "ending", Meta: deck.PageMeta{Badge: "第 3 页", Counter: "3/3", Theme: "green", CTA: "cta3"}, Content: deck.PageContent{Title: "结尾", Body: "正文3"}},
		},
	}
}

func TestRenderHTMLPagesWritesHTMLWithoutCommands(t *testing.T) {
	runner := &fakeRunner{}
	outDir := t.TempDir()
	r := Renderer{OutDir: outDir, Runner: runner}
	d := sampleDeck(outDir)

	if err := r.RenderHTMLPages(d); err != nil {
		t.Fatalf("RenderHTMLPages() error = %v", err)
	}

	for _, page := range d.Pages {
		htmlPath := filepath.Join(outDir, page.Name+".html")
		if _, err := os.Stat(htmlPath); err != nil {
			t.Fatalf("expected html file %q, stat error = %v", htmlPath, err)
		}
	}
	if got := len(runner.snapshotCalls()); got != 0 {
		t.Fatalf("runner calls = %d, want 0", got)
	}
}

func TestCapturePNGsBuildsChromeCommandsOnly(t *testing.T) {
	runner := &fakeRunner{}
	outDir := t.TempDir()
	r := Renderer{OutDir: outDir, ChromePath: "chrome", Jobs: 2, Runner: runner}
	d := sampleDeck(outDir)

	if err := r.CapturePNGs(d.Pages, outDir); err != nil {
		t.Fatalf("CapturePNGs() error = %v", err)
	}

	calls := runner.snapshotCalls()
	if len(calls) != len(d.Pages) {
		t.Fatalf("len(calls) = %d, want %d", len(calls), len(d.Pages))
	}
	for _, call := range calls {
		if len(call) == 0 {
			t.Fatalf("empty command call")
		}
		if call[0] != "chrome" {
			t.Fatalf("command = %q, want chrome", call[0])
		}
		for _, arg := range call[1:] {
			if arg == "montage" {
				t.Fatalf("unexpected montage arg in chrome call: %v", call)
			}
		}
	}
}

func TestCapturePNGsUses1656WindowSize(t *testing.T) {
	runner := &fakeRunner{}
	outDir := t.TempDir()
	r := Renderer{OutDir: outDir, ChromePath: "chrome", Jobs: 1, Runner: runner}
	d := sampleDeck(outDir)

	if err := r.CapturePNGs(d.Pages[:1], outDir); err != nil {
		t.Fatalf("CapturePNGs() error = %v", err)
	}

	calls := runner.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	found := false
	for _, arg := range calls[0][1:] {
		if arg == "--window-size=1242,1656" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("chrome args = %v, want --window-size=1242,1656", calls[0])
	}
}

func TestRenderHTMLPagesRequiresOutDir(t *testing.T) {
	r := Renderer{Runner: &fakeRunner{}}
	d := sampleDeck("")
	d.OutDir = ""

	err := r.RenderHTMLPages(d)
	if err == nil {
		t.Fatalf("RenderHTMLPages() error = nil, want non-nil")
	}
	if got := err.Error(); got != "out dir is required" {
		t.Fatalf("RenderHTMLPages() error = %q", got)
	}
}

func TestCapturePNGsRejectsEmptyPages(t *testing.T) {
	r := Renderer{Runner: &fakeRunner{}}

	err := r.CapturePNGs(nil, t.TempDir())
	if err == nil {
		t.Fatalf("CapturePNGs() error = nil, want non-nil")
	}
	if got := err.Error(); got != "deck must contain at least 1 page for capture" {
		t.Fatalf("CapturePNGs() error = %q", got)
	}
}

func TestCapturePNGsRejectsEmptyOutDir(t *testing.T) {
	r := Renderer{Runner: &fakeRunner{}}
	d := sampleDeck(t.TempDir())

	err := r.CapturePNGs(d.Pages, "")
	if err == nil {
		t.Fatalf("CapturePNGs() error = nil, want non-nil")
	}
	if got := err.Error(); got != "out dir is required" {
		t.Fatalf("CapturePNGs() error = %q", got)
	}
}

func TestCapturePNGsPropagatesRunnerError(t *testing.T) {
	outDir := t.TempDir()
	runner := &failRunner{failName: "chrome", failErr: errors.New("boom")}
	r := Renderer{OutDir: outDir, ChromePath: "chrome", Jobs: 1, Runner: runner}
	d := sampleDeck(outDir)

	err := r.CapturePNGs(d.Pages, outDir)
	if err == nil {
		t.Fatalf("CapturePNGs() error = nil, want non-nil")
	}
	if got := err.Error(); got != "screenshot p01-cover: boom" {
		t.Fatalf("CapturePNGs() error = %q", got)
	}
}

func TestRenderStopsWhenCaptureFails(t *testing.T) {
	outDir := t.TempDir()
	runner := &failRunner{failName: "chrome", failErr: errors.New("boom")}
	r := Renderer{OutDir: outDir, ChromePath: "chrome", Jobs: 1, Runner: runner}
	d := sampleDeck(outDir)

	err := r.Render(d)
	if err == nil {
		t.Fatalf("Render() error = nil, want non-nil")
	}
	if got := err.Error(); got != "screenshot p01-cover: boom" {
		t.Fatalf("Render() error = %q", got)
	}

	calls := runner.snapshotCalls()
	if len(calls) == 0 {
		t.Fatalf("expected runner calls")
	}
	last := calls[len(calls)-1]
	if last[0] == "magick" {
		t.Fatalf("unexpected montage call after capture failure: %v", calls)
	}
}

func TestRendererBuildsChromeCommandsOnly(t *testing.T) {
	runner := &fakeRunner{}
	r := Renderer{
		OutDir:     t.TempDir(),
		ChromePath: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		Jobs:       2,
		Runner:     runner,
	}
	d := deck.Deck{
		OutDir:    r.OutDir,
		ThemeName: deck.ThemeDefault,
		Themes:    deck.RegisteredThemes(),
		Pages: []deck.Page{
			{Name: "p01-cover", Variant: "cover", Meta: deck.PageMeta{Badge: "第 1 页", Counter: "1/3", Theme: "orange", CTA: "cta1"}, Content: deck.PageContent{Title: "封面", Subtitle: "副标题1"}},
			{Name: "p02-bullets", Variant: "bullets", Meta: deck.PageMeta{Badge: "第 2 页", Counter: "2/3", Theme: "orange", CTA: "cta2"}, Content: deck.PageContent{Title: "中间", Items: []string{"要点1"}}},
			{Name: "p03-ending", Variant: "ending", Meta: deck.PageMeta{Badge: "第 3 页", Counter: "3/3", Theme: "green", CTA: "cta3"}, Content: deck.PageContent{Title: "结尾", Body: "正文3"}},
		},
	}
	if err := r.Render(d); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	calls := runner.snapshotCalls()
	if len(calls) != 3 {
		t.Fatalf("len(calls) = %d, want 3", len(calls))
	}

	for _, call := range calls {
		if len(call) == 0 {
			t.Fatalf("empty command call")
		}
		if call[0] != r.ChromePath {
			t.Fatalf("command = %q, want %q", call[0], r.ChromePath)
		}
	}
}

func TestRendererDefaultsJobsToTwo(t *testing.T) {
	r := Renderer{Jobs: 0}
	if got := r.effectiveJobs(); got != 2 {
		t.Fatalf("effectiveJobs() = %d, want 2", got)
	}
}

func TestRendererRejectsEmptyDeck(t *testing.T) {
	r := Renderer{OutDir: t.TempDir(), Runner: &fakeRunner{}}
	d := deck.DefaultDeck(r.OutDir)
	d.Pages = nil

	err := r.Render(d)
	if err == nil {
		t.Fatalf("Render() error = nil, want non-nil")
	}
	if got := err.Error(); got != "deck must contain at least 1 page for render" {
		t.Fatalf("Render() error = %q", got)
	}
}

func TestRendererUsesBoundedConcurrency(t *testing.T) {
	runner := &concurrentRunner{delay: 25 * time.Millisecond}
	r := Renderer{
		OutDir:     t.TempDir(),
		ChromePath: "chrome",
		Jobs:       2,
		Runner:     runner,
	}
	d := deck.DefaultDeck(r.OutDir)

	if err := r.Render(d); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got := runner.max(); got != 2 {
		t.Fatalf("max concurrent chrome jobs = %d, want 2", got)
	}
}

func TestCaptureHTMLDirectoryUsesSortedTopLevelHTMLFilesOnly(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "b.html"), "<html></html>")
	mustWriteFile(t, filepath.Join(root, "a.html"), "<html></html>")
	mustWriteFile(t, filepath.Join(root, "ignore.HTML"), "<html></html>")
	mustWriteFile(t, filepath.Join(root, "notes.txt"), "ignored")
	mustMkdirAll(t, filepath.Join(root, "nested"))
	mustWriteFile(t, filepath.Join(root, "nested", "c.html"), "<html></html>")

	runner := &fakeRunner{}
	r := Renderer{ChromePath: "chrome", Jobs: 1, Runner: runner}

	if err := r.CaptureHTMLPath(root); err != nil {
		t.Fatalf("CaptureHTMLPath() error = %v", err)
	}

	gotTargets := extractScreenshotTargetsInOrder(runner.snapshotCalls())
	want := []string{
		filepath.Join(root, "a.png"),
		filepath.Join(root, "b.png"),
	}
	if !reflect.DeepEqual(gotTargets, want) {
		t.Fatalf("targets = %#v, want %#v", gotTargets, want)
	}
}

func TestCaptureHTMLPathSingleFileCreatesSiblingPNG(t *testing.T) {
	root := t.TempDir()
	htmlPath := filepath.Join(root, "page.html")
	mustWriteFile(t, htmlPath, "<html></html>")

	runner := &fakeRunner{}
	r := Renderer{ChromePath: "chrome", Jobs: 1, Runner: runner}

	if err := r.CaptureHTMLPath(htmlPath); err != nil {
		t.Fatalf("CaptureHTMLPath() error = %v", err)
	}

	gotTargets := extractScreenshotTargets(runner.snapshotCalls())
	want := []string{filepath.Join(root, "page.png")}
	if !reflect.DeepEqual(gotTargets, want) {
		t.Fatalf("targets = %#v, want %#v", gotTargets, want)
	}
}

func TestCaptureHTMLPathSingleUppercaseHTMLCreatesSiblingPNG(t *testing.T) {
	root := t.TempDir()
	htmlPath := filepath.Join(root, "page.HTML")
	mustWriteFile(t, htmlPath, "<html></html>")

	runner := &fakeRunner{}
	r := Renderer{ChromePath: "chrome", Jobs: 1, Runner: runner}

	if err := r.CaptureHTMLPath(htmlPath); err != nil {
		t.Fatalf("CaptureHTMLPath() error = %v", err)
	}

	gotTargets := extractScreenshotTargets(runner.snapshotCalls())
	want := []string{filepath.Join(root, "page.png")}
	if !reflect.DeepEqual(gotTargets, want) {
		t.Fatalf("targets = %#v, want %#v", gotTargets, want)
	}
}

func TestCaptureHTMLPathRejectsMissingPath(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing.html")

	r := Renderer{ChromePath: "chrome", Jobs: 1, Runner: &fakeRunner{}}
	err := r.CaptureHTMLPath(missing)
	if err == nil {
		t.Fatalf("CaptureHTMLPath() error = nil, want non-nil")
	}
	if got := err.Error(); got != "capture html path "+missing+": stat "+missing+": no such file or directory" {
		t.Fatalf("CaptureHTMLPath() error = %q", got)
	}
}

func TestCaptureHTMLPathRejectsNonHTMLFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "page.txt")
	mustWriteFile(t, path, "nope")

	r := Renderer{ChromePath: "chrome", Jobs: 1, Runner: &fakeRunner{}}
	err := r.CaptureHTMLPath(path)
	if err == nil {
		t.Fatalf("CaptureHTMLPath() error = nil, want non-nil")
	}
	if got := err.Error(); got != "capture html path "+path+": file must have .html extension" {
		t.Fatalf("CaptureHTMLPath() error = %q", got)
	}
}

func TestCaptureHTMLPathRejectsDirectoryWithoutHTML(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "page.HTML"), "<html></html>")
	mustWriteFile(t, filepath.Join(root, "notes.txt"), "ignored")

	r := Renderer{ChromePath: "chrome", Jobs: 1, Runner: &fakeRunner{}}
	err := r.CaptureHTMLPath(root)
	if err == nil {
		t.Fatalf("CaptureHTMLPath() error = nil, want non-nil")
	}
	if got := err.Error(); got != "capture html path "+root+": no .html files found" {
		t.Fatalf("CaptureHTMLPath() error = %q", got)
	}
}

func TestCaptureHTMLPathReturnsFailureAndKeepsSuccessfulOutputs(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "a.html"), "<html></html>")
	mustWriteFile(t, filepath.Join(root, "b.html"), "<html></html>")

	runner := &failRunner{failName: "chrome", failErr: errors.New("boom")}
	r := Renderer{ChromePath: "chrome", Jobs: 1, Runner: runner}

	err := r.CaptureHTMLPath(root)
	if err == nil {
		t.Fatalf("CaptureHTMLPath() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), filepath.Join(root, "a.html")) && !strings.Contains(err.Error(), filepath.Join(root, "b.html")) {
		t.Fatalf("CaptureHTMLPath() error = %q, want failing html path", err.Error())
	}
	if got := len(runner.snapshotCalls()); got != 2 {
		t.Fatalf("len(calls) = %d, want 2", got)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}

func extractScreenshotTargets(calls [][]string) []string {
	targets := extractScreenshotTargetsInOrder(calls)
	sort.Strings(targets)
	return targets
}

func extractScreenshotTargetsInOrder(calls [][]string) []string {
	targets := make([]string, 0, len(calls))
	for _, call := range calls {
		for _, arg := range call[1:] {
			if target, ok := strings.CutPrefix(arg, "--screenshot="); ok {
				targets = append(targets, target)
				break
			}
		}
	}
	return targets
}
