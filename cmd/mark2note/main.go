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
  --input <file.md>   markdown input path (required)
  --out <dir>         output directory (default: <output.dir>/<markdown-file-name>-<timestamp>, e.g. article-20260328-153000)
  --chrome <path>     chrome binary path
  --jobs <n>          parallel screenshot jobs (default: 2)
  --config <file>     config file path (default: configs/config.yaml)
  --theme <name>      one-off deck theme override (default from deck.theme)
  --author <name>     one-off cover author input (blank falls back to deck.author) (default from deck.author)

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

Capture existing HTML file(s) to sibling PNG files using the same Chrome screenshot settings as the main render flow.

Usage:
  mark2note capture-html --input <path> [flags]
  mark2note capture-html --help

Flags:
  --input <path>      html file or directory path (required)
  --chrome <path>     chrome binary path
  --jobs <n>          parallel screenshot jobs (default: 2)

Behavior:
  - supports a single html file or a directory
  - directory mode scans current directory only
  - directory mode only captures lowercase .html files
  - output png files are written beside each html file
  - directory mode is non-recursive

Examples:
  mark2note capture-html --input ./output/preview/p02-quote.html
  mark2note capture-html --input ./output/preview`
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
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "out" {
			outChanged = true
		}
	})
	opts.OutDirChanged = outChanged
	return opts, nil
}

func parseCaptureHTMLOptions(args []string) (Options, error) {
	opts := defaultOptions()
	fs := flag.NewFlagSet("mark2note capture-html", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.ChromePath, "chrome", opts.ChromePath, "chrome binary path")
	fs.IntVar(&opts.Jobs, "jobs", opts.Jobs, "parallel screenshot jobs")
	fs.StringVar(&opts.InputPath, "input", opts.InputPath, "html input path")

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
	return buildRenderer(opts).CaptureHTMLPath(absolutePath(opts.InputPath))
}

func buildRenderer(opts Options) render.Renderer {
	return render.Renderer{
		OutDir:     absolutePath(opts.OutDir),
		ChromePath: opts.ChromePath,
		Jobs:       opts.Jobs,
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
