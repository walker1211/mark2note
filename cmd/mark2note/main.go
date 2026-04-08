package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/walker1211/mark2note/internal/ai"
	"github.com/walker1211/mark2note/internal/app"
	"github.com/walker1211/mark2note/internal/config"
	"github.com/walker1211/mark2note/internal/render"
)

type Options = app.Options

func defaultOptions() Options {
	return Options{
		OutDir:     "output",
		ChromePath: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		Jobs:       2,
		ConfigPath: "configs/config.yaml",
		Animated: app.AnimatedOptions{
			Format:     "webp",
			DurationMS: 2400,
			FPS:        8,
		},
		Live: app.LiveOptions{
			PhotoFormat: "jpeg",
			CoverFrame:  "middle",
			OutputDir:   "",
		},
	}
}

func usageText() string {
	return `mark2note

把 Markdown 文件通过 configs/config.yaml 中配置的 AI CLI 解析成 deck JSON，并渲染为 HTML 和 PNG。

Usage:
  mark2note --input <file.md> [flags]
  mark2note capture-html --input <path> [flags]
  mark2note --help

Commands:
  capture-html   capture existing html file(s) to sibling png files

Flags:
  --input <file.md>          markdown input path (required)
  --out <dir>                output directory (default: <output.dir>/<markdown-file-name>-<timestamp>, e.g. article-20260328-153000)
  --chrome <path>            chrome binary path
  --jobs <n>                 parallel screenshot jobs (default: 2)
  --config <file>            config file path (default: configs/config.yaml)
  --theme <name>             one-off deck theme override (default from deck.theme)
  --author <name>            one-off cover author input (blank falls back to deck.author) (default from deck.author)
  --animated                 enable animated enhancement output
  --animated-format <name>   animated format (default: webp; supported: webp, mp4)
  --animated-duration <ms>   page animation timeline duration; also affects Live motion timing when --live is enabled (default: 2400)
  --animated-fps <n>         animation capture fps / sampling density; affects Animated WebP/MP4 output and Live frame sampling (default: 8)
  --live                     enable experimental live package export
  --live-photo-format <name> live cover photo format (default: jpeg)
  --live-cover-frame <name>  live cover frame strategy (default: middle; supported: first, middle, last)
  --live-assemble            assemble final Apple Live Photo artifacts with makelive
  --live-output-dir <dir>    final Apple Live Photo output directory (default: <out>/apple-live)

Examples:
  mark2note --help
  mark2note --input ./example.md
  mark2note --input ./example.md --out ./output/preview
  mark2note --input ./example.md --config ./configs/config.yaml
  mark2note --input ./example.md --config ./config.yaml
  mark2note capture-html --input ./output/preview/p02-quote.html
  mark2note capture-html --input ./output/preview

Config defaults:
  deck.theme   default theme name used when --theme is not set
  deck.author  default cover author used when --author is not set

Supported themes:
  default / warm-paper / editorial-cool / lifestyle-light / tech-noir / editorial-mono`
}

func captureHTMLUsageText() string {
	return `mark2note capture-html

Capture existing HTML file(s) to sibling PNG files.

Usage:
  mark2note capture-html --input <path> [flags]
  mark2note capture-html --help

Flags:
  --input <path>      html file or directory path (required)
  --chrome <path>     chrome binary path
  --jobs <n>          parallel screenshot jobs (default: 2)
  --config <file>     optional config file path for render.viewport

Behavior:
  - supports a single html file or a directory
  - directory mode scans current directory only
  - directory mode only captures lowercase .html files
  - output png files are written beside each html file
  - directory mode is non-recursive
  - without --config, screenshot viewport falls back to 1242 x 1656
  - with --config, render.viewport.width/height will be used when set

Examples:
  mark2note capture-html --input ./output/preview/p02-quote.html
  mark2note capture-html --input ./output/preview
  mark2note capture-html --input ./output/preview --config ./configs/config.yaml`
}

func isHelpRequest(args []string) bool {
	return len(args) == 1 && args[0] == "help"
}

func parseOptions(args []string) (Options, error) {
	opts := defaultOptions()
	fs := flag.NewFlagSet("mark2note", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.OutDir, "out", opts.OutDir, "output directory")
	fs.StringVar(&opts.ChromePath, "chrome", opts.ChromePath, "chrome binary path")
	fs.IntVar(&opts.Jobs, "jobs", opts.Jobs, "parallel screenshot jobs")
	fs.StringVar(&opts.InputPath, "input", opts.InputPath, "markdown input path")
	fs.StringVar(&opts.ConfigPath, "config", opts.ConfigPath, "config file path")
	fs.StringVar(&opts.Theme, "theme", opts.Theme, "one-off deck theme override")
	fs.StringVar(&opts.Author, "author", opts.Author, "one-off cover author input (blank falls back to deck.author)")
	fs.BoolVar(&opts.Animated.Enabled, "animated", opts.Animated.Enabled, "enable animated enhancement output")
	fs.StringVar(&opts.Animated.Format, "animated-format", opts.Animated.Format, "animated format")
	fs.IntVar(&opts.Animated.DurationMS, "animated-duration", opts.Animated.DurationMS, "animated duration per page in milliseconds")
	fs.IntVar(&opts.Animated.FPS, "animated-fps", opts.Animated.FPS, "animated frames per second")
	fs.BoolVar(&opts.Live.Enabled, "live", opts.Live.Enabled, "enable experimental live package export")
	fs.StringVar(&opts.Live.PhotoFormat, "live-photo-format", opts.Live.PhotoFormat, "live cover photo format")
	fs.StringVar(&opts.Live.CoverFrame, "live-cover-frame", opts.Live.CoverFrame, "live cover frame strategy")
	fs.BoolVar(&opts.Live.Assemble, "live-assemble", opts.Live.Assemble, "assemble final Apple Live Photo artifacts with makelive")
	fs.StringVar(&opts.Live.OutputDir, "live-output-dir", opts.Live.OutputDir, "final Apple Live Photo output directory")

	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}
	if fs.NArg() > 0 {
		return Options{}, fmt.Errorf("unexpected positional args: %s", strings.Join(fs.Args(), " "))
	}
	if opts.Jobs <= 0 {
		return Options{}, fmt.Errorf("jobs must be >= 1")
	}
	if strings.TrimSpace(opts.InputPath) == "" {
		return Options{}, fmt.Errorf("--input is required\n\n%s", usageText())
	}
	outChanged := false
	animatedEnabledChanged := false
	animatedFormatChanged := false
	animatedDurationChanged := false
	animatedFPSChanged := false
	liveEnabledChanged := false
	livePhotoFormatChanged := false
	liveCoverFrameChanged := false
	liveAssembleChanged := false
	liveOutputDirChanged := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "out":
			outChanged = true
		case "animated":
			animatedEnabledChanged = true
		case "animated-format":
			animatedFormatChanged = true
		case "animated-duration":
			animatedDurationChanged = true
		case "animated-fps":
			animatedFPSChanged = true
		case "live":
			liveEnabledChanged = true
		case "live-photo-format":
			livePhotoFormatChanged = true
		case "live-cover-frame":
			liveCoverFrameChanged = true
		case "live-assemble":
			liveAssembleChanged = true
		case "live-output-dir":
			liveOutputDirChanged = true
		}
	})
	opts.OutDirChanged = outChanged
	opts.AnimatedEnabledChanged = animatedEnabledChanged
	opts.AnimatedFormatChanged = animatedFormatChanged
	opts.AnimatedDurationChanged = animatedDurationChanged
	opts.AnimatedFPSChanged = animatedFPSChanged
	opts.LiveEnabledChanged = liveEnabledChanged
	opts.LivePhotoFormatChanged = livePhotoFormatChanged
	opts.LiveCoverFrameChanged = liveCoverFrameChanged
	opts.LiveAssembleChanged = liveAssembleChanged
	opts.LiveOutputDirChanged = liveOutputDirChanged
	return opts, nil
}

func parseCaptureHTMLOptions(args []string) (Options, error) {
	opts := defaultOptions()
	opts.ConfigPath = ""
	fs := flag.NewFlagSet("mark2note capture-html", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.ChromePath, "chrome", opts.ChromePath, "chrome binary path")
	fs.IntVar(&opts.Jobs, "jobs", opts.Jobs, "parallel screenshot jobs")
	fs.StringVar(&opts.InputPath, "input", opts.InputPath, "html input path")
	fs.StringVar(&opts.ConfigPath, "config", opts.ConfigPath, "optional config file path for render.viewport")

	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}
	if fs.NArg() > 0 {
		return Options{}, fmt.Errorf("unexpected positional args: %s", strings.Join(fs.Args(), " "))
	}
	if opts.Jobs <= 0 {
		return Options{}, fmt.Errorf("jobs must be >= 1")
	}
	if strings.TrimSpace(opts.InputPath) == "" {
		return Options{}, fmt.Errorf("--input is required\n\n%s", captureHTMLUsageText())
	}
	return opts, nil
}

var loadConfig = config.Load
var readFile = os.ReadFile
var buildDeckJSON = func(cfg *config.Config, markdown string) (string, error) {
	b := ai.Builder{}
	b.SetCommand(cfg.AI.Command, cfg.AI.Args)
	return b.BuildDeckJSON(markdown)
}
var generatePreview = func(opts Options) (app.Result, error) {
	svc := app.Service{
		LoadConfig:    loadConfig,
		ReadFile:      readFile,
		BuildDeckJSON: buildDeckJSON,
		NewRenderer: func(o app.Options) app.DeckRenderer {
			return buildRenderer(o)
		},
	}
	return svc.GeneratePreview(opts)
}

var captureHTML = func(opts Options) error {
	if strings.TrimSpace(opts.ConfigPath) != "" {
		cfg, err := loadConfig(opts.ConfigPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if opts.ViewportWidth == 0 {
			opts.ViewportWidth = cfg.Render.Viewport.Width
		}
		if opts.ViewportHeight == 0 {
			opts.ViewportHeight = cfg.Render.Viewport.Height
		}
	}
	return buildRenderer(opts).CaptureHTMLPath(absolutePath(opts.InputPath))
}

func buildRenderer(opts Options) render.Renderer {
	return render.Renderer{
		OutDir:         absolutePath(opts.OutDir),
		ChromePath:     opts.ChromePath,
		Jobs:           opts.Jobs,
		ViewportWidth:  opts.ViewportWidth,
		ViewportHeight: opts.ViewportHeight,
		Animated:       renderAnimatedOptions(opts.Animated),
		Live:           renderLiveOptions(opts.Live),
	}
}

func renderAnimatedOptions(opts app.AnimatedOptions) render.AnimatedOptions {
	return render.AnimatedOptions{
		Enabled:    opts.Enabled,
		Format:     opts.Format,
		DurationMS: opts.DurationMS,
		FPS:        opts.FPS,
	}
}

func renderLiveOptions(opts app.LiveOptions) render.LiveOptions {
	outputDir := opts.OutputDir
	if strings.TrimSpace(outputDir) != "" && !filepath.IsAbs(outputDir) {
		outputDir = absolutePath(outputDir)
	}
	return render.LiveOptions{
		Enabled:     opts.Enabled,
		PhotoFormat: opts.PhotoFormat,
		CoverFrame:  opts.CoverFrame,
		Assemble:    opts.Assemble,
		OutputDir:   outputDir,
	}
}

func absolutePath(path string) string {
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			return abs
		}
	}
	return path
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "capture-html" {
		return runCaptureHTML(args[1:], stdout, stderr)
	}
	if isHelpRequest(args) {
		fmt.Fprintln(stdout, usageText())
		return 0
	}

	opts, err := parseOptions(args)
	if err != nil {
		if err == flag.ErrHelp {
			fmt.Fprintln(stdout, usageText())
			return 0
		}
		fmt.Fprintf(stderr, "error parsing flags: %v\n", err)
		return 1
	}

	result, err := generatePreview(opts)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrLoadConfig):
			fmt.Fprintf(stderr, "error loading config: %s\n", stripErrorPrefixes(err, app.ErrLoadConfig))
		case errors.Is(err, app.ErrReadMarkdown):
			fmt.Fprintf(stderr, "error reading markdown: %s\n", stripErrorPrefixes(err, app.ErrReadMarkdown))
		case errors.Is(err, app.ErrBuildDeckJSON):
			fmt.Fprintf(stderr, "error building deck json: %s\n", stripErrorPrefixes(err, app.ErrBuildDeckJSON))
		default:
			fmt.Fprintf(stderr, "render preview failed: %s\n", stripErrorPrefixes(err, app.ErrParseDeck, app.ErrRenderPreview))
		}
		return 1
	}

	fmt.Fprintf(stdout, "generated %d preview pages\n", result.PageCount)
	for _, warning := range result.Warnings {
		fmt.Fprintln(stderr, warning)
	}
	return 0
}

func runCaptureHTML(args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseCaptureHTMLOptions(args)
	if err != nil {
		if err == flag.ErrHelp {
			fmt.Fprintln(stdout, captureHTMLUsageText())
			return 0
		}
		fmt.Fprintf(stderr, "error parsing flags: %v\n", err)
		return 1
	}
	if err := captureHTML(opts); err != nil {
		fmt.Fprintf(stderr, "capture html failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "captured html to png")
	return 0
}

func stripErrorPrefixes(err error, sentinels ...error) string {
	message := err.Error()
	for _, sentinel := range sentinels {
		prefix := sentinel.Error() + ": "
		message = strings.TrimPrefix(message, prefix)
	}
	message = strings.TrimPrefix(message, "parse deck: ")
	return message
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
