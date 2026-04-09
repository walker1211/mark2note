package render

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
	delay      time.Duration
	encodeName string

	mu        sync.Mutex
	active    int
	maxActive int
	calls     [][]string
}

func (r *concurrentRunner) Run(name string, args ...string) error {
	call := append([]string{name}, args...)
	r.mu.Lock()
	r.calls = append(r.calls, call)
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

func (r *concurrentRunner) countCalls(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, call := range r.calls {
		if len(call) > 0 && call[0] == name {
			count++
		}
	}
	return count
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

type fakeMP4Encoder struct {
	checkErr    error
	encodeErr   error
	writeOutput bool

	mu      sync.Mutex
	outputs []string
	inputs  []animatedSequenceSpec
}

func (e *fakeMP4Encoder) CheckAvailable() error {
	return e.checkErr
}

func (e *fakeMP4Encoder) Encode(outputPath string, sequence animatedSequenceSpec) error {
	e.mu.Lock()
	e.outputs = append(e.outputs, outputPath)
	e.inputs = append(e.inputs, sequence)
	e.mu.Unlock()
	if e.encodeErr != nil {
		return e.encodeErr
	}
	if e.writeOutput {
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, []byte("mp4"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (e *fakeMP4Encoder) snapshotOutputs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.outputs...)
}

type fakeLivePackageBuilder struct {
	checkErr    error
	buildErr    error
	writeOutput bool

	mu    sync.Mutex
	tasks []livePackageTask
}

type fakeLivePhotoAssembler struct {
	checkErr error
	callErr  error

	mu    sync.Mutex
	tasks []appleLiveTask
}

func (b *fakeLivePackageBuilder) CheckAvailable() error {
	return b.checkErr
}

func (b *fakeLivePackageBuilder) Build(task livePackageTask) error {
	b.mu.Lock()
	b.tasks = append(b.tasks, task)
	b.mu.Unlock()
	if b.buildErr != nil {
		return b.buildErr
	}
	if b.writeOutput {
		if err := os.MkdirAll(task.OutputDir, 0o755); err != nil {
			return err
		}
		manifestPath := filepath.Join(task.OutputDir, "manifest.json")
		if err := os.WriteFile(manifestPath, []byte("{}"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (b *fakeLivePackageBuilder) snapshotTasks() []livePackageTask {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]livePackageTask(nil), b.tasks...)
}

func (a *fakeLivePhotoAssembler) CheckAvailable() error {
	return a.checkErr
}

func (a *fakeLivePhotoAssembler) Assemble(task appleLiveTask) error {
	a.mu.Lock()
	a.tasks = append(a.tasks, task)
	a.mu.Unlock()
	if a.callErr != nil {
		return a.callErr
	}
	outputDir := task.OutputDir
	if stringsTrim(outputDir) == "" {
		outputDir = resolvedAppleLiveOutputDir(task)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDir, task.PageName+".jpg"), []byte("jpg"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDir, task.PageName+".mov"), []byte("mov"), 0o644); err != nil {
		return err
	}
	return nil
}

func (a *fakeLivePhotoAssembler) snapshotTasks() []appleLiveTask {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]appleLiveTask(nil), a.tasks...)
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

func TestRendererCarriesLiveOptions(t *testing.T) {
	r := Renderer{Live: liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first", Assemble: true, OutputDir: "/tmp/apple-live"}}
	if !r.Live.Enabled || r.Live.PhotoFormat != "jpeg" || r.Live.CoverFrame != "first" || !r.Live.Assemble || r.Live.OutputDir != "/tmp/apple-live" {
		t.Fatalf("Renderer.Live = %#v", r.Live)
	}
}

func TestRenderHTMLPagesWritesAnimatedCapableHTMLWhenAnimatedEnabled(t *testing.T) {
	runner := &fakeRunner{}
	outDir := t.TempDir()
	r := Renderer{OutDir: outDir, Runner: runner, Animated: animatedOptions{Enabled: true, Format: "webp", DurationMS: 2400, FPS: 8}}
	d := sampleDeck(outDir)

	if err := r.RenderHTMLPages(d); err != nil {
		t.Fatalf("RenderHTMLPages() error = %v", err)
	}

	htmlPath := filepath.Join(outDir, d.Pages[0].Name+".html")
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", htmlPath, err)
	}
	got := string(content)
	for _, want := range []string{`data-animated="true"`, `data-animated-ms="2400"`, `animated_ms`, `anim-fade-up`} {
		if !strings.Contains(got, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}

func TestRenderHTMLPagesWritesAnimatedCapableHTMLWhenOnlyLiveEnabled(t *testing.T) {
	runner := &fakeRunner{}
	outDir := t.TempDir()
	r := Renderer{OutDir: outDir, Runner: runner, Animated: animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8}, Live: liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle"}}
	d := sampleDeck(outDir)

	if err := r.RenderHTMLPages(d); err != nil {
		t.Fatalf("RenderHTMLPages() error = %v", err)
	}

	htmlPath := filepath.Join(outDir, d.Pages[0].Name+".html")
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", htmlPath, err)
	}
	got := string(content)
	for _, want := range []string{`data-animated="true"`, `data-animated-ms="2400"`, `animated_ms`, `anim-fade-up`} {
		if !strings.Contains(got, want) {
			t.Fatalf("live-only html missing %q", want)
		}
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

func TestRenderHTMLPagesUsesRendererViewportForScaledHTML(t *testing.T) {
	runner := &fakeRunner{}
	outDir := t.TempDir()
	r := Renderer{OutDir: outDir, Runner: runner, ViewportWidth: 720, ViewportHeight: 960}
	d := sampleDeck(outDir)

	if err := r.RenderHTMLPages(d); err != nil {
		t.Fatalf("RenderHTMLPages() error = %v", err)
	}

	htmlPath := filepath.Join(outDir, d.Pages[0].Name+".html")
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", htmlPath, err)
	}
	got := string(content)
	for _, want := range []string{`width=720,height=960,initial-scale=1`, `.page { transform-origin: top left; transform: translate(0px, 0px) scale(0.579710); }`} {
		if !strings.Contains(got, want) {
			t.Fatalf("html missing %q: %s", want, got)
		}
	}
}

func TestRenderHTMLPagesKeepsDeckViewportWhenRendererViewportUnset(t *testing.T) {
	runner := &fakeRunner{}
	outDir := t.TempDir()
	r := Renderer{OutDir: outDir, Runner: runner}
	d := sampleDeck(outDir)
	d.ViewportWidth = 720
	d.ViewportHeight = 960

	if err := r.RenderHTMLPages(d); err != nil {
		t.Fatalf("RenderHTMLPages() error = %v", err)
	}

	htmlPath := filepath.Join(outDir, d.Pages[0].Name+".html")
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", htmlPath, err)
	}
	got := string(content)
	for _, want := range []string{`width=720,height=960,initial-scale=1`, `.page { transform-origin: top left; transform: translate(0px, 0px) scale(0.579710); }`} {
		if !strings.Contains(got, want) {
			t.Fatalf("html missing %q: %s", want, got)
		}
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

func TestCapturePNGsUsesDefaultWindowSize(t *testing.T) {
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
	found := slices.Contains(calls[0][1:], "--window-size=1242,1656")
	if !found {
		t.Fatalf("chrome args = %v, want --window-size=1242,1656", calls[0])
	}
}

func TestCapturePNGsUsesConfiguredWindowSize(t *testing.T) {
	runner := &fakeRunner{}
	outDir := t.TempDir()
	r := Renderer{OutDir: outDir, ChromePath: "chrome", Jobs: 1, Runner: runner, ViewportWidth: 720, ViewportHeight: 960}
	d := sampleDeck(outDir)

	if err := r.CapturePNGs(d.Pages[:1], outDir); err != nil {
		t.Fatalf("CapturePNGs() error = %v", err)
	}

	calls := runner.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	found := slices.Contains(calls[0][1:], "--window-size=720,960")
	if !found {
		t.Fatalf("chrome args = %v, want --window-size=720,960", calls[0])
	}
}

func TestRenderUsesFinalAnimatedStateForPrimaryHTMLAndPNG(t *testing.T) {
	runner := &fakeRunner{}
	outDir := t.TempDir()
	r := Renderer{
		OutDir:     outDir,
		ChromePath: "chrome",
		Jobs:       1,
		Runner:     runner,
		Animated:   animatedOptions{Enabled: true, Format: "webp", DurationMS: 2400, FPS: 8},
		WebPEncoder: img2webpEncoder{LookPath: func(string) (string, error) {
			return "", errors.New("not found")
		}},
	}
	d := sampleDeck(outDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "img2webp") {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}

	htmlPath := filepath.Join(outDir, d.Pages[0].Name+".html")
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", htmlPath, err)
	}
	if !strings.Contains(string(content), `data-animated-ms="2400"`) {
		t.Fatalf("html should use final animated state: %s", string(content))
	}

	calls := runner.snapshotCalls()
	if len(calls) != len(d.Pages) {
		t.Fatalf("chrome calls = %d, want %d", len(calls), len(d.Pages))
	}
	for _, arg := range calls[0][1:] {
		if strings.Contains(arg, "animated_ms=") {
			t.Fatalf("primary png capture should use final rendered html, got %v", calls[0])
		}
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

	result, err := r.Render(d)
	if err == nil {
		t.Fatalf("Render() error = nil, want non-nil")
	}
	if got := err.Error(); got != "screenshot p01-cover: boom" {
		t.Fatalf("Render() error = %q", got)
	}
	if !reflect.DeepEqual(result, RenderResult{}) {
		t.Fatalf("Render() result = %#v, want empty", result)
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

func TestRenderReturnsWarningWhenAnimatedOptionsInvalid(t *testing.T) {
	runner := &fakeRunner{}
	outDir := t.TempDir()
	r := Renderer{OutDir: outDir, ChromePath: "chrome", Jobs: 1, Runner: runner, Animated: animatedOptions{Enabled: true, Format: "gif", DurationMS: 800, FPS: 20}}
	d := sampleDeck(outDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want 1 warning", result.Warnings)
	}
	if !strings.Contains(result.Warnings[0], "animated export skipped") || !strings.Contains(result.Warnings[0], "format") || !strings.Contains(result.Warnings[0], "duration_ms") || !strings.Contains(result.Warnings[0], "fps") {
		t.Fatalf("Warnings[0] = %q", result.Warnings[0])
	}

	htmlPath := filepath.Join(outDir, d.Pages[0].Name+".html")
	content, readErr := os.ReadFile(htmlPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", htmlPath, readErr)
	}
	if strings.Contains(string(content), `data-animated="true"`) {
		t.Fatalf("html should remain static when animated options are invalid: %s", string(content))
	}
}

func TestRendererReturnsWarningWhenAnimatedEncoderMissingButPNGSucceed(t *testing.T) {
	runner := &fakeRunner{}
	outDir := t.TempDir()
	r := Renderer{
		OutDir:      outDir,
		ChromePath:  "chrome",
		Jobs:        1,
		Runner:      runner,
		Animated:    animatedOptions{Enabled: true, Format: "webp", DurationMS: 2400, FPS: 8},
		WebPEncoder: img2webpEncoder{LookPath: func(string) (string, error) { return "", errors.New("not found") }},
	}
	d := sampleDeck(outDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "img2webp") {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
	if len(runner.snapshotCalls()) != len(d.Pages) {
		t.Fatalf("chrome calls = %d, want only PNG captures", len(runner.snapshotCalls()))
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
	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !reflect.DeepEqual(result, RenderResult{}) {
		t.Fatalf("Render() result = %#v, want empty", result)
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

	result, err := r.Render(d)
	if err == nil {
		t.Fatalf("Render() error = nil, want non-nil")
	}
	if got := err.Error(); got != "deck must contain at least 1 page for render" {
		t.Fatalf("Render() error = %q", got)
	}
	if !reflect.DeepEqual(result, RenderResult{}) {
		t.Fatalf("Render() result = %#v, want empty", result)
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

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !reflect.DeepEqual(result, RenderResult{}) {
		t.Fatalf("Render() result = %#v, want empty", result)
	}

	if got := runner.max(); got != 2 {
		t.Fatalf("max concurrent chrome jobs = %d, want 2", got)
	}
}

func TestCaptureAnimatedWebPProcessesPagesConcurrently(t *testing.T) {
	runner := &concurrentRunner{delay: 25 * time.Millisecond}
	encoder := img2webpEncoder{Runner: runner, Binary: "img2webp", LookPath: func(string) (string, error) { return "/opt/homebrew/bin/img2webp", nil }}
	r := Renderer{
		OutDir:      t.TempDir(),
		ChromePath:  "chrome",
		Jobs:        2,
		Runner:      runner,
		WebPEncoder: encoder,
		Animated:    animatedOptions{Enabled: true, Format: "webp", DurationMS: 2400, FPS: 8},
	}
	d := sampleDeck(r.OutDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !reflect.DeepEqual(result, RenderResult{}) {
		t.Fatalf("Render() result = %#v, want empty", result)
	}
	if got := runner.max(); got != 2 {
		t.Fatalf("max concurrent tasks = %d, want 2", got)
	}
	if got := runner.countCalls("img2webp"); got != len(d.Pages) {
		t.Fatalf("img2webp calls = %d, want %d", got, len(d.Pages))
	}
}

func TestRenderEmitsRootLevelMP4WhenAnimatedFormatIsMP4(t *testing.T) {
	outDir := t.TempDir()
	runner := &fakeRunner{}
	mp4Encoder := &fakeMP4Encoder{writeOutput: true}
	r := Renderer{
		OutDir:     outDir,
		ChromePath: "chrome",
		Jobs:       1,
		Runner:     runner,
		MP4Encoder: mp4Encoder,
		Animated:   animatedOptions{Enabled: true, Format: "mp4", DurationMS: 2400, FPS: 8},
	}
	d := sampleDeck(outDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !reflect.DeepEqual(result, RenderResult{}) {
		t.Fatalf("Render() result = %#v, want empty", result)
	}
	if got := mp4Encoder.snapshotOutputs(); len(got) != len(d.Pages) {
		t.Fatalf("mp4 outputs = %#v, want %d outputs", got, len(d.Pages))
	}
	for _, page := range d.Pages {
		if _, err := os.Stat(filepath.Join(outDir, page.Name+".mp4")); err != nil {
			t.Fatalf("expected mp4 for %s: %v", page.Name, err)
		}
	}
}

func TestRenderReusesOneFrameCaptureSetForAnimatedAndLive(t *testing.T) {
	outDir := t.TempDir()
	runner := &fakeRunner{}
	webpEncoder := img2webpEncoder{Runner: runner, Binary: "img2webp", LookPath: func(string) (string, error) { return "/opt/homebrew/bin/img2webp", nil }}
	liveBuilder := &fakeLivePackageBuilder{writeOutput: true}
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             runner,
		WebPEncoder:        webpEncoder,
		LivePackageBuilder: liveBuilder,
		Animated:           animatedOptions{Enabled: true, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle"},
	}
	d := sampleDeck(outDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !reflect.DeepEqual(result, RenderResult{}) {
		t.Fatalf("Render() result = %#v, want empty", result)
	}

	calls := runner.snapshotCalls()
	chromeCalls := 0
	for _, call := range calls {
		if len(call) > 0 && call[0] == "chrome" {
			chromeCalls++
		}
	}
	frameCount := len(frameTimesMS(2400, 8))
	wantChromeCalls := len(d.Pages) + len(d.Pages)*frameCount
	if chromeCalls != wantChromeCalls {
		t.Fatalf("chrome calls = %d, want %d", chromeCalls, wantChromeCalls)
	}
	if got := len(liveBuilder.snapshotTasks()); got != len(d.Pages) {
		t.Fatalf("live tasks = %d, want %d", got, len(d.Pages))
	}
}

func TestRenderRunsLiveExportWhenAnimatedOutputDisabled(t *testing.T) {
	outDir := t.TempDir()
	runner := &fakeRunner{}
	mp4Encoder := &fakeMP4Encoder{writeOutput: true}
	liveBuilder := &fakeLivePackageBuilder{writeOutput: true}
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             runner,
		MP4Encoder:         mp4Encoder,
		LivePackageBuilder: liveBuilder,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle"},
	}
	d := sampleDeck(outDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !reflect.DeepEqual(result, RenderResult{}) {
		t.Fatalf("Render() result = %#v, want empty", result)
	}
	if got := len(liveBuilder.snapshotTasks()); got != len(d.Pages) {
		t.Fatalf("live tasks = %d, want %d", got, len(d.Pages))
	}
	htmlPath := filepath.Join(outDir, d.Pages[0].Name+".html")
	content, readErr := os.ReadFile(htmlPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", htmlPath, readErr)
	}
	if !strings.Contains(string(content), `data-animated="true"`) || !strings.Contains(string(content), `data-animated-ms="2400"`) {
		t.Fatalf("live-only html should be animation-capable: %s", string(content))
	}
	if got := len(mp4Encoder.snapshotOutputs()); got != 0 {
		t.Fatalf("mp4 outputs = %#v, want none", mp4Encoder.snapshotOutputs())
	}
	for _, page := range d.Pages {
		if _, err := os.Stat(filepath.Join(outDir, page.Name+".mp4")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected mp4 for %s: %v", page.Name, err)
		}
	}
	calls := runner.snapshotCalls()
	chromeCalls := 0
	for _, call := range calls {
		if len(call) > 0 && call[0] == "chrome" {
			chromeCalls++
		}
	}
	if chromeCalls <= len(d.Pages) {
		t.Fatalf("chrome calls = %d, want frame captures beyond primary PNGs", chromeCalls)
	}
}

func TestRenderReturnsWarningWhenLiveBuilderCheckFails(t *testing.T) {
	outDir := t.TempDir()
	runner := &fakeRunner{}
	liveBuilder := &fakeLivePackageBuilder{checkErr: errors.New("exiftool not available: not found")}
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             runner,
		LivePackageBuilder: liveBuilder,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle"},
	}
	d := sampleDeck(outDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "exiftool") {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
	if len(runner.snapshotCalls()) != len(d.Pages) {
		t.Fatalf("chrome calls = %d, want only PNG captures", len(runner.snapshotCalls()))
	}
}

func TestRenderReturnsWarningWhenLiveAssemblerCheckFailsButStillBuildsPackage(t *testing.T) {
	outDir := t.TempDir()
	runner := &fakeRunner{}
	liveBuilder := &fakeLivePackageBuilder{writeOutput: true}
	liveAssembler := &fakeLivePhotoAssembler{checkErr: errors.New("makelive not available: not found")}
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             runner,
		LivePackageBuilder: liveBuilder,
		LivePhotoAssembler: liveAssembler,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true},
	}
	d := sampleDeck(outDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "makelive") {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
	if got := len(liveBuilder.snapshotTasks()); got != len(d.Pages) {
		t.Fatalf("live tasks = %d, want %d", got, len(d.Pages))
	}
	if got := len(liveAssembler.snapshotTasks()); got != 0 {
		t.Fatalf("assembler tasks = %d, want 0", got)
	}
}

func TestRenderAssemblesLiveArtifactsAfterPackageBuild(t *testing.T) {
	outDir := t.TempDir()
	runner := &fakeRunner{}
	liveBuilder := &fakeLivePackageBuilder{writeOutput: true}
	liveAssembler := &fakeLivePhotoAssembler{}
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             runner,
		LivePackageBuilder: liveBuilder,
		LivePhotoAssembler: liveAssembler,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true, OutputDir: filepath.Join(outDir, "apple-live")},
	}
	d := sampleDeck(outDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !reflect.DeepEqual(result, RenderResult{}) {
		t.Fatalf("Render() result = %#v, want empty", result)
	}
	if got := len(liveAssembler.snapshotTasks()); got != len(d.Pages) {
		t.Fatalf("assembler tasks = %d, want %d", got, len(d.Pages))
	}
	first := liveAssembler.snapshotTasks()[0]
	if first.PackageDir != filepath.Join(outDir, "p01-cover.live") {
		t.Fatalf("PackageDir = %q", first.PackageDir)
	}
	if first.OutputDir != filepath.Join(outDir, "apple-live") {
		t.Fatalf("OutputDir = %q", first.OutputDir)
	}
}

func TestRenderReturnsWarningWhenLiveAssemblerFails(t *testing.T) {
	outDir := t.TempDir()
	runner := &fakeRunner{}
	liveBuilder := &fakeLivePackageBuilder{writeOutput: true}
	liveAssembler := &fakeLivePhotoAssembler{callErr: errors.New("boom")}
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             runner,
		LivePackageBuilder: liveBuilder,
		LivePhotoAssembler: liveAssembler,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true},
	}
	d := sampleDeck(outDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "live assemble failed") {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
	if got := len(liveBuilder.snapshotTasks()); got != len(d.Pages) {
		t.Fatalf("live tasks = %d, want %d", got, len(d.Pages))
	}
}

func TestRenderReturnsWarningWhenMP4EncoderMissingButHTMLAndPNGSucceed(t *testing.T) {
	outDir := t.TempDir()
	runner := &fakeRunner{}
	mp4Encoder := &fakeMP4Encoder{checkErr: errors.New("ffmpeg not available: not found")}
	r := Renderer{
		OutDir:     outDir,
		ChromePath: "chrome",
		Jobs:       1,
		Runner:     runner,
		MP4Encoder: mp4Encoder,
		Animated:   animatedOptions{Enabled: true, Format: "mp4", DurationMS: 2400, FPS: 8},
	}
	d := sampleDeck(outDir)

	result, err := r.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "ffmpeg") {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
	if len(runner.snapshotCalls()) != len(d.Pages) {
		t.Fatalf("chrome calls = %d, want only PNG captures", len(runner.snapshotCalls()))
	}
	for _, page := range d.Pages {
		if _, err := os.Stat(filepath.Join(outDir, page.Name+".html")); err != nil {
			t.Fatalf("expected html for %s: %v", page.Name, err)
		}
	}
	gotTargets := extractScreenshotTargets(runner.snapshotCalls())
	wantTargets := []string{
		filepath.Join(outDir, "p01-cover.png"),
		filepath.Join(outDir, "p02-bullets.png"),
		filepath.Join(outDir, "p03-ending.png"),
	}
	if !reflect.DeepEqual(gotTargets, wantTargets) {
		t.Fatalf("png targets = %#v, want %#v", gotTargets, wantTargets)
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
