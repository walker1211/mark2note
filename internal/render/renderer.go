package render

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/walker1211/mark2note/internal/deck"
)

const defaultChromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

type CommandRunner interface {
	Run(name string, args ...string) error
}

type Renderer struct {
	OutDir             string
	ChromePath         string
	Jobs               int
	ViewportWidth      int
	ViewportHeight     int
	Animated           animatedOptions
	Live               liveOptions
	Runner             CommandRunner
	WebPEncoder        WebPEncoder
	MP4Encoder         MP4Encoder
	LivePackageBuilder LivePackageBuilder
	LivePhotoAssembler LivePhotoAssembler
	PhotosImporter     PhotosImporter
	ImportResultWriter ImportResultWriter
	Now                func() time.Time
}

type RenderResult struct {
	Warnings           []string
	DeliveryReport     *DeliveryReport
	DeliveryReportPath string
}

type livePackageTask struct {
	PageName     string
	OutputDir    string
	FramePaths   []string
	FramePattern string
	DurationMS   int
	FPS          int
	PhotoFormat  string
	CoverFrame   string
}

type appleLiveTask struct {
	PageName   string
	PackageDir string
	PhotoPath  string
	VideoPath  string
	OutputDir  string
}

type LivePackageBuilder interface {
	CheckAvailable() error
	Build(task livePackageTask) error
}

type LivePhotoAssembler interface {
	CheckAvailable() error
	Assemble(task appleLiveTask) error
}

type captureTask struct {
	name     string
	htmlPath string
	pngPath  string
}

type execRunner struct{}

func (execRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

func (r Renderer) Render(d deck.Deck) (RenderResult, error) {
	if len(d.Pages) == 0 {
		return RenderResult{}, fmt.Errorf("deck must contain at least 1 page for render")
	}
	r = r.withDeckViewportFallback(d)
	outDir := r.effectiveOutDir(d)
	if outDir == "" {
		return RenderResult{}, fmt.Errorf("out dir is required")
	}

	mode, animatedResult := r.normalizedAnimated()
	liveMode, liveWarnings := normalizeLiveOptions(r.Live)
	captureMode, captureWarnings := r.normalizedCaptureTiming(mode, liveMode.Enabled)
	if err := r.renderHTMLPages(d, captureMode); err != nil {
		return RenderResult{}, err
	}
	if err := r.CapturePNGs(d.Pages, outDir); err != nil {
		return RenderResult{}, err
	}
	warnings := append([]string(nil), animatedResult.Warnings...)
	warnings = append(warnings, liveWarnings...)
	warnings = append(warnings, captureWarnings...)
	if captureMode.Enabled && (mode.Enabled || liveMode.Enabled) {
		warnings = append(warnings, r.runAnimatedExports(d.Pages, outDir, captureMode, mode, liveMode)...)
	}
	result := RenderResult{Warnings: warnings}
	if liveMode.Enabled && liveMode.Assemble && liveMode.ImportPhotos {
		sourceDir := r.liveDeliverySourceDir(outDir, liveMode)
		if !filepath.IsAbs(sourceDir) {
			absSourceDir, err := filepath.Abs(sourceDir)
			if err != nil {
				return result, fmt.Errorf("resolve live delivery source dir: %w", err)
			}
			sourceDir = absSourceDir
		}
		delivery, err := r.liveDeliveryOrchestrator().Deliver(liveDeliveryRequest{
			SourceDir:     sourceDir,
			AlbumName:     liveMode.ImportAlbum,
			ImportTimeout: liveMode.ImportTimeout,
		})
		result.DeliveryReport = &delivery.Report
		result.DeliveryReportPath = delivery.ReportPath
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (r Renderer) RenderHTMLPages(d deck.Deck) error {
	r = r.withDeckViewportFallback(d)
	mode, _ := r.normalizedAnimated()
	liveMode, _ := normalizeLiveOptions(r.Live)
	captureMode, _ := r.normalizedCaptureTiming(mode, liveMode.Enabled)
	return r.renderHTMLPages(d, captureMode)
}

func (r Renderer) renderHTMLPages(d deck.Deck, mode normalizedAnimatedOptions) error {
	outDir := r.effectiveOutDir(d)
	if outDir == "" {
		return fmt.Errorf("out dir is required")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}

	renderDeck := d
	renderDeck.ViewportWidth = r.effectiveViewportWidth(d.ViewportWidth)
	renderDeck.ViewportHeight = r.effectiveViewportHeight(d.ViewportHeight)

	for _, page := range renderDeck.Pages {
		var (
			html string
			err  error
		)
		if mode.Enabled {
			html, err = RenderAnimatedPageHTML(renderDeck, page, mode.DurationMS)
		} else {
			html, err = RenderPageHTML(renderDeck, page)
		}
		if err != nil {
			return fmt.Errorf("render html %s: %w", page.Name, err)
		}
		htmlPath := filepath.Join(outDir, page.Name+".html")
		if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
			return fmt.Errorf("write html %s: %w", page.Name, err)
		}
	}
	return nil
}

func (r Renderer) normalizedAnimated() (normalizedAnimatedOptions, RenderResult) {
	mode, warning := normalizeAnimatedOptions(r.Animated)
	result := RenderResult{}
	if warning != "" {
		result.Warnings = append(result.Warnings, warning)
	}
	return mode, result
}

func (r Renderer) normalizedCaptureTiming(animated normalizedAnimatedOptions, liveEnabled bool) (normalizedAnimatedOptions, []string) {
	if animated.Enabled || !liveEnabled {
		return animated, nil
	}
	mode, warning := normalizeAnimatedOptions(animatedOptions{Enabled: true, Format: animatedFormatWebP, DurationMS: r.Animated.DurationMS, FPS: r.Animated.FPS})
	if warning != "" {
		return normalizedAnimatedOptions{}, []string{"live export skipped: " + strings.TrimPrefix(warning, "animated export skipped: ")}
	}
	return mode, nil
}

func (r Renderer) effectiveJobs() int {
	if r.Jobs <= 0 {
		return 2
	}
	return r.Jobs
}

func (r Renderer) effectiveOutDir(d deck.Deck) string {
	if strings.TrimSpace(r.OutDir) != "" {
		return r.OutDir
	}
	return d.OutDir
}

func (r Renderer) effectiveChromePath() string {
	if strings.TrimSpace(r.ChromePath) == "" {
		return defaultChromePath
	}
	return r.ChromePath
}

func (r Renderer) withDeckViewportFallback(d deck.Deck) Renderer {
	if r.ViewportWidth <= 0 {
		r.ViewportWidth = d.ViewportWidth
	}
	if r.ViewportHeight <= 0 {
		r.ViewportHeight = d.ViewportHeight
	}
	return r
}

func (r Renderer) effectiveViewportWidth(deckWidth int) int {
	if r.ViewportWidth > 0 {
		return r.ViewportWidth
	}
	if deckWidth > 0 {
		return deckWidth
	}
	return 1242
}

func (r Renderer) effectiveViewportHeight(deckHeight int) int {
	if r.ViewportHeight > 0 {
		return r.ViewportHeight
	}
	if deckHeight > 0 {
		return deckHeight
	}
	return 1656
}

func (r Renderer) effectiveRunner() CommandRunner {
	if r.Runner != nil {
		return r.Runner
	}
	return execRunner{}
}

func (r Renderer) effectiveWebPEncoder() WebPEncoder {
	if r.WebPEncoder != nil {
		return r.WebPEncoder
	}
	return img2webpEncoder{Runner: r.effectiveRunner()}
}

func (r Renderer) effectiveMP4Encoder() MP4Encoder {
	if r.MP4Encoder != nil {
		return r.MP4Encoder
	}
	return ffmpegEncoder{Runner: r.effectiveRunner()}
}

func (r Renderer) effectiveLivePackageBuilder() LivePackageBuilder {
	if r.LivePackageBuilder != nil {
		return r.LivePackageBuilder
	}
	return livePackageBuilder{Runner: r.effectiveRunner()}
}

func (r Renderer) effectiveLivePhotoAssembler() LivePhotoAssembler {
	if r.LivePhotoAssembler != nil {
		return r.LivePhotoAssembler
	}
	return makeliveAssembler{Runner: r.effectiveRunner()}
}

func (r Renderer) effectivePhotosImporter() PhotosImporter {
	if r.PhotosImporter != nil {
		return r.PhotosImporter
	}
	return osascriptPhotosImporter{}
}

func (r Renderer) effectiveImportResultWriter() ImportResultWriter {
	if r.ImportResultWriter != nil {
		return r.ImportResultWriter
	}
	return importResultWriter{}
}

func (r Renderer) effectiveNow() func() time.Time {
	if r.Now != nil {
		return r.Now
	}
	return time.Now
}

func (r Renderer) CapturePNGs(pages []deck.Page, outDir string) error {
	if len(pages) == 0 {
		return fmt.Errorf("deck must contain at least 1 page for capture")
	}
	if strings.TrimSpace(outDir) == "" {
		return fmt.Errorf("out dir is required")
	}

	tasks := make([]captureTask, 0, len(pages))
	for _, page := range pages {
		tasks = append(tasks, captureTask{
			name:     page.Name,
			htmlPath: filepath.Join(outDir, page.Name+".html"),
			pngPath:  filepath.Join(outDir, page.Name+".png"),
		})
	}
	return r.runCaptureTasksWithJobs(tasks, r.effectiveJobs())
}

func (r Renderer) CaptureHTMLPath(inputPath string) error {
	tasks, err := collectCaptureTasks(inputPath)
	if err != nil {
		return fmt.Errorf("capture html path %s: %w", inputPath, err)
	}
	return r.runCaptureTasksWithJobs(tasks, r.effectiveJobs())
}

func (r Renderer) runAnimatedExports(pages []deck.Page, outDir string, captureMode normalizedAnimatedOptions, animated normalizedAnimatedOptions, live normalizedLiveOptions) []string {
	tasks := buildAnimatedCaptureTasks(pages, outDir, captureMode)
	warningsList := make([]string, 0, 3)
	if len(tasks) == 0 {
		return warningsList
	}

	webpEnabled := animated.Enabled && animated.Format == animatedFormatWebP
	mp4Enabled := animated.Enabled && animated.Format == animatedFormatMP4

	var webpEncoder WebPEncoder
	if webpEnabled {
		webpEncoder = r.effectiveWebPEncoder()
		if err := webpEncoder.CheckAvailable(); err != nil {
			warningsList = append(warningsList, fmt.Sprintf("animated export skipped: %v", err))
			webpEnabled = false
		}
	}

	var mp4Encoder MP4Encoder
	if mp4Enabled {
		mp4Encoder = r.effectiveMP4Encoder()
		if err := mp4Encoder.CheckAvailable(); err != nil {
			warningsList = append(warningsList, fmt.Sprintf("animated export skipped: %v", err))
			mp4Enabled = false
		}
	}

	liveBuilder := r.effectiveLivePackageBuilder()
	if live.Enabled {
		if err := liveBuilder.CheckAvailable(); err != nil {
			warningsList = append(warningsList, fmt.Sprintf("live export skipped: %v", err))
			live = normalizedLiveOptions{}
		}
	}

	var liveAssembler LivePhotoAssembler
	if live.Enabled && live.Assemble {
		liveAssembler = r.effectiveLivePhotoAssembler()
		if err := liveAssembler.CheckAvailable(); err != nil {
			warningsList = append(warningsList, fmt.Sprintf("live assemble skipped: %v", err))
			live.Assemble = false
			liveAssembler = nil
		}
	}
	if !webpEnabled && !mp4Enabled && !live.Enabled {
		sort.Strings(warningsList)
		return warningsList
	}

	warnings := make(chan string, len(tasks))
	work := make(chan animatedCaptureTask)

	var wg sync.WaitGroup
	for i := 0; i < r.effectiveJobs(); i++ {
		wg.Go(func() {
			for task := range work {
				if warning := r.runAnimatedExportTask(task, captureMode, live, webpEnabled, webpEncoder, mp4Enabled, mp4Encoder, liveBuilder, liveAssembler); warning != "" {
					warnings <- warning
				}
			}
		})
	}

	for _, task := range tasks {
		work <- task
	}
	close(work)
	wg.Wait()
	close(warnings)

	collected := append([]string(nil), warningsList...)
	for warning := range warnings {
		collected = append(collected, warning)
	}
	sort.Strings(collected)
	return collected
}

func (r Renderer) runAnimatedExportTask(task animatedCaptureTask, mode normalizedAnimatedOptions, live normalizedLiveOptions, webpEnabled bool, webpEncoder WebPEncoder, mp4Enabled bool, mp4Encoder MP4Encoder, liveBuilder LivePackageBuilder, liveAssembler LivePhotoAssembler) string {
	captureTasks := make([]captureTask, 0, len(task.framePaths))
	for i := range task.framePaths {
		captureTasks = append(captureTasks, captureTask{name: task.pageName, htmlPath: task.frameURIs[i], pngPath: task.framePaths[i]})
	}
	for _, path := range task.framePaths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Sprintf("animated export skipped for %s: %v", task.pageName, err)
		}
	}
	if err := r.runCaptureTasksWithJobs(captureTasks, 1); err != nil {
		return fmt.Sprintf("animated export failed for %s: %v", task.pageName, err)
	}
	if webpEnabled {
		if err := webpEncoder.Encode(task.outputPath, frameSpecsForTask(task, mode.FrameMS)); err != nil {
			return fmt.Sprintf("animated export failed for %s: %v", task.pageName, err)
		}
	}
	if mp4Enabled {
		if err := mp4Encoder.Encode(task.outputPath, animatedSequenceSpec{FramePattern: task.framePattern, FPS: mode.FPS}); err != nil {
			return fmt.Sprintf("animated export failed for %s: %v", task.pageName, err)
		}
	}
	if live.Enabled && liveBuilder != nil {
		if err := liveBuilder.Build(livePackageTask{PageName: task.pageName, OutputDir: task.liveOutputDir, FramePaths: append([]string(nil), task.framePaths...), FramePattern: task.framePattern, DurationMS: mode.DurationMS, FPS: mode.FPS, PhotoFormat: live.PhotoFormat, CoverFrame: live.CoverFrame}); err != nil {
			return fmt.Sprintf("live export failed for %s: %v", task.pageName, err)
		}
		if live.Assemble && liveAssembler != nil {
			if err := liveAssembler.Assemble(appleLiveTask{PageName: task.pageName, PackageDir: task.liveOutputDir, PhotoPath: filepath.Join(task.liveOutputDir, liveCoverFilename), VideoPath: filepath.Join(task.liveOutputDir, liveMotionFilename), OutputDir: live.OutputDir}); err != nil {
				return fmt.Sprintf("live assemble failed for %s: %v", task.pageName, err)
			}
		}
	}
	return ""
}

func collectCaptureTasks(inputPath string) ([]captureTask, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		entries, err := os.ReadDir(inputPath)
		if err != nil {
			return nil, err
		}
		tasks := make([]captureTask, 0)
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".html" {
				continue
			}
			htmlPath := filepath.Join(inputPath, entry.Name())
			tasks = append(tasks, captureTask{
				name:     htmlPath,
				htmlPath: htmlPath,
				pngPath:  strings.TrimSuffix(htmlPath, ".html") + ".png",
			})
		}
		if len(tasks) == 0 {
			return nil, fmt.Errorf("no .html files found")
		}
		sort.Slice(tasks, func(i, j int) bool {
			return filepath.Base(tasks[i].htmlPath) < filepath.Base(tasks[j].htmlPath)
		})
		return tasks, nil
	}

	if strings.ToLower(filepath.Ext(inputPath)) != ".html" {
		return nil, fmt.Errorf("file must have .html extension")
	}
	return []captureTask{{
		name:     inputPath,
		htmlPath: inputPath,
		pngPath:  strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + ".png",
	}}, nil
}

func (r Renderer) runCaptureTasksWithJobs(captureTasks []captureTask, jobs int) error {
	runner := r.effectiveRunner()
	chrome := r.effectiveChromePath()
	tasks := make(chan captureTask)
	errCh := make(chan error, len(captureTasks))

	if jobs <= 0 {
		jobs = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		wg.Go(func() {
			for task := range tasks {
				if err := runner.Run(chrome, r.screenshotArgs(task)...); err != nil {
					errCh <- fmt.Errorf("screenshot %s: %w", task.name, err)
				}
			}
		})
	}

	for _, task := range captureTasks {
		tasks <- task
	}
	close(tasks)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func (r Renderer) screenshotArgs(task captureTask) []string {
	return []string{
		"--headless=new",
		"--disable-gpu",
		"--hide-scrollbars",
		"--allow-file-access-from-files",
		"--run-all-compositor-stages-before-draw",
		"--virtual-time-budget=2000",
		fmt.Sprintf("--window-size=%d,%d", r.effectiveViewportWidth(0), r.effectiveViewportHeight(0)),
		"--screenshot=" + task.pngPath,
		r.captureTargetURI(task.htmlPath),
	}
}

func (r Renderer) captureTargetURI(path string) string {
	if strings.HasPrefix(path, "file://") {
		return path
	}
	return fileURI(path)
}

func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func (r Renderer) liveDeliverySourceDir(outDir string, live normalizedLiveOptions) string {
	if stringsTrim(live.OutputDir) != "" {
		return live.OutputDir
	}
	return filepath.Join(outDir, defaultAppleLiveDirName)
}

func (r Renderer) liveDeliveryOrchestrator() LiveDeliveryOrchestrator {
	return LiveDeliveryOrchestrator{
		Scanner:  livePairScanner{},
		Importer: r.effectivePhotosImporter(),
		Writer:   r.effectiveImportResultWriter(),
		Now:      r.effectiveNow(),
	}
}
