package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/walker1211/mark2note/internal/ai"
	"github.com/walker1211/mark2note/internal/app"
	"github.com/walker1211/mark2note/internal/config"
	"github.com/walker1211/mark2note/internal/deck"
	"github.com/walker1211/mark2note/internal/render"
	"github.com/walker1211/mark2note/internal/xhs"
)

type Options = app.Options

func defaultOptions() Options {
	return Options{
		OutDir:        "output",
		ChromePath:    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		Jobs:          2,
		ConfigPath:    "configs/config.yaml",
		ImportTimeout: 120 * time.Second,
		Animated: app.AnimatedOptions{
			Format:     "webp",
			DurationMS: 2400,
			FPS:        8,
		},
		Live: app.LiveOptions{
			PhotoFormat:   "jpeg",
			CoverFrame:    "middle",
			OutputDir:     "",
			ImportTimeout: 120 * time.Second,
		},
	}
}

func usageText() string {
	return `mark2note

把 Markdown 文件通过 configs/config.yaml 中配置的 AI CLI 解析成 deck JSON，并渲染为 HTML 和 PNG。

Usage:
  mark2note --input <file.md> [flags]
  mark2note --from-deck <deck.json> [flags]
  mark2note capture-html --input <path> [flags]
  mark2note publish-xhs --account <name> [flags]
  mark2note --help

Commands:
  capture-html   capture existing html file(s) to sibling png files
  publish-xhs    publish assets to Xiaohongshu only-self-visible or schedule queue

Flags:
  --input <file.md>          markdown input path
  --from-deck <deck.json>    saved deck layout json input path; skips Markdown/AI generation
  --out <dir>                output directory (default: <output.dir>/<markdown-file-name>-<timestamp>, e.g. article-20260328-153000)
  --chrome <path>            chrome binary path
  --jobs <n>                 parallel screenshot jobs (default: 2)
  --config <file>            config file path (default: configs/config.yaml)
  --theme <name>             one-off deck theme override (default from deck.theme)
  --author <name>            one-off cover author input (blank falls back to deck.author) (default from deck.author)
  --prompt-extra <text>      extra natural-language guidance for deck generation
  --publish-xhs              publish generated PNG files to Xiaohongshu after render
  --xhs-tags <csv>           override auto-generated Xiaohongshu topics for --publish-xhs
  --import-photos            import generated PNG files into Apple Photos after export
  --import-album <name>      Apple Photos album name for imported PNG files
  --import-timeout <d>       Apple Photos PNG import timeout (default: 2m0s)
  --animated                 enable animated enhancement output
  --animated-format <name>   animated format (default: webp; supported: webp, mp4)
  --animated-duration <ms>   page animation timeline duration; also affects Live motion timing when --live is enabled (default: 2400)
  --animated-fps <n>         animation capture fps / sampling density; affects Animated WebP/MP4 output and Live frame sampling (default: 8)
  --live                     enable experimental live package export
  --live-photo-format <name> live cover photo format (default: jpeg)
  --live-cover-frame <name>  live cover frame strategy (default: middle; supported: first, middle, last)
  --live-assemble            assemble final Apple Live Photo artifacts with makelive
  --live-output-dir <dir>    final Apple Live Photo output directory (default: <out>/apple-live)
  --live-import-photos       import assembled Live Photos into Apple Photos after export
  --live-import-album <name> Apple Photos album name for imported Live Photos
  --live-import-timeout <d>  Apple Photos import timeout (default: 2m0s)

Examples:
  mark2note --help
  mark2note --input ./example.md
  mark2note --input ./example.md --out ./output/preview
  mark2note --input ./example.md --config ./configs/config.yaml
  mark2note --input ./example.md --config ./config.yaml
  mark2note --input ./example.md --prompt-extra "封面更抓眼，少一点教程感"
  mark2note --input ./example.md --theme fresh-green --publish-xhs
  mark2note --input ./example.md --import-photos --import-album "mark2note"
  mark2note --from-deck ./output/preview/deck.json --import-photos --import-album "mark2note"
  mark2note capture-html --input ./output/preview/p02-quote.html
  mark2note capture-html --input ./output/preview
  mark2note publish-xhs --account main --title "标题" --content "正文" --images ./cover.jpg
  mark2note publish-xhs --account main --title-file ./title.txt --content-file ./body.md --live-report ./output/report.json --live-pages p01-cover,p02-bullets

Config defaults:
  deck.theme_mode fixed / weekly theme selection mode
  deck.weekly_themes weekly mode chooses a theme by local weekday
  deck.theme      fixed mode theme and weekly fallback when the day is not configured
  deck.author     default cover author used when --author is not set

Supported themes:
  ` + deck.RegisteredThemeList()
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

func publishXHSUsageText() string {
	return `mark2note publish-xhs

Publish generated assets to Xiaohongshu only-self-visible or schedule queue.

Usage:
  mark2note publish-xhs --account <name> [flags]
  mark2note publish-xhs --help

Flags:
  --config <file>          config file path (default: configs/config.yaml)
  --account <name>         publish account/profile target (default from xhs.publish.account)
  --title <text>           inline title text (exactly one of --title / --title-file)
  --title-file <file>      title file path
  --content <text>         inline content text (exactly one of --content / --content-file)
  --content-file <file>    content file path
  --tags <csv>             comma-separated tags
  --mode <name>            publish mode: only-self or schedule (default from xhs.publish.mode or only-self)
  --schedule-at <time>     schedule time in YYYY-MM-DD HH:MM:SS (Asia/Shanghai)
  --images <csv>           comma-separated image paths
  --live-report <file>     live delivery report path
  --live-pages <csv>       ordered live page subset; requires --live-report
  --chrome <path>          chrome binary path (default from xhs.publish.browser_path)
  --headless               run browser automation headless (default: true; xhs.publish.headless can override)
  --profile-dir <dir>      browser profile directory (default from xhs.publish.profile_dir)
  --declare-original       declare original content before submit (default from xhs.publish.declare_original)
  --allow-content-copy     keep allow content copy enabled (default from xhs.publish.allow_content_copy)

Rules:
  - exactly one of --title / --title-file is required
  - exactly one of --content / --content-file is required
  - exactly one media source is required: --images or --live-report
  - --schedule-at is required when --mode schedule
  - --live-pages is accepted only with --live-report`
}

func isHelpRequest(args []string) bool {
	return len(args) == 1 && args[0] == "help"
}

func parseOptions(args []string) (Options, error) {
	opts := defaultOptions()
	var xhsTags string
	fs := flag.NewFlagSet("mark2note", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.OutDir, "out", opts.OutDir, "output directory")
	fs.StringVar(&opts.ChromePath, "chrome", opts.ChromePath, "chrome binary path")
	fs.IntVar(&opts.Jobs, "jobs", opts.Jobs, "parallel screenshot jobs")
	fs.StringVar(&opts.InputPath, "input", opts.InputPath, "markdown input path")
	fs.StringVar(&opts.FromDeckPath, "from-deck", opts.FromDeckPath, "saved deck layout json input path")
	fs.StringVar(&opts.ConfigPath, "config", opts.ConfigPath, "config file path")
	fs.StringVar(&opts.Theme, "theme", opts.Theme, "one-off deck theme override")
	fs.StringVar(&opts.Author, "author", opts.Author, "one-off cover author input (blank falls back to deck.author)")
	fs.StringVar(&opts.PromptExtra, "prompt-extra", opts.PromptExtra, "extra natural-language guidance for deck generation")
	fs.BoolVar(&opts.PublishXHS, "publish-xhs", opts.PublishXHS, "publish generated PNG files to Xiaohongshu after render")
	fs.StringVar(&xhsTags, "xhs-tags", xhsTags, "comma-separated Xiaohongshu topics for auto publish")
	fs.BoolVar(&opts.ImportPhotos, "import-photos", opts.ImportPhotos, "import generated PNG files into Apple Photos after export")
	fs.StringVar(&opts.ImportAlbum, "import-album", opts.ImportAlbum, "Apple Photos album name for imported PNG files")
	fs.DurationVar(&opts.ImportTimeout, "import-timeout", opts.ImportTimeout, "Apple Photos PNG import timeout")
	fs.BoolVar(&opts.Animated.Enabled, "animated", opts.Animated.Enabled, "enable animated enhancement output")
	fs.StringVar(&opts.Animated.Format, "animated-format", opts.Animated.Format, "animated format")
	fs.IntVar(&opts.Animated.DurationMS, "animated-duration", opts.Animated.DurationMS, "animated duration per page in milliseconds")
	fs.IntVar(&opts.Animated.FPS, "animated-fps", opts.Animated.FPS, "animated frames per second")
	fs.BoolVar(&opts.Live.Enabled, "live", opts.Live.Enabled, "enable experimental live package export")
	fs.StringVar(&opts.Live.PhotoFormat, "live-photo-format", opts.Live.PhotoFormat, "live cover photo format")
	fs.StringVar(&opts.Live.CoverFrame, "live-cover-frame", opts.Live.CoverFrame, "live cover frame strategy")
	fs.BoolVar(&opts.Live.Assemble, "live-assemble", opts.Live.Assemble, "assemble final Apple Live Photo artifacts with makelive")
	fs.StringVar(&opts.Live.OutputDir, "live-output-dir", opts.Live.OutputDir, "final Apple Live Photo output directory")
	fs.BoolVar(&opts.Live.ImportPhotos, "live-import-photos", opts.Live.ImportPhotos, "import assembled Live Photos into Apple Photos after export")
	fs.StringVar(&opts.Live.ImportAlbum, "live-import-album", opts.Live.ImportAlbum, "Apple Photos album name for imported Live Photos")
	fs.DurationVar(&opts.Live.ImportTimeout, "live-import-timeout", opts.Live.ImportTimeout, "Apple Photos import timeout")

	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}
	if fs.NArg() > 0 {
		return Options{}, fmt.Errorf("unexpected positional args: %s", strings.Join(fs.Args(), " "))
	}
	if opts.Jobs <= 0 {
		return Options{}, fmt.Errorf("jobs must be >= 1")
	}
	opts.XHSTags = splitCSV(xhsTags)
	hasInput := strings.TrimSpace(opts.InputPath) != ""
	hasFromDeck := strings.TrimSpace(opts.FromDeckPath) != ""
	if hasInput == hasFromDeck {
		return Options{}, fmt.Errorf("exactly one of --input or --from-deck is required\n\n%s", usageText())
	}
	if hasFromDeck && strings.TrimSpace(opts.PromptExtra) != "" {
		return Options{}, fmt.Errorf("--prompt-extra can only be used with --input\n\n%s", usageText())
	}
	if hasFromDeck && opts.PublishXHS {
		return Options{}, fmt.Errorf("--publish-xhs can only be used with --input\n\n%s", usageText())
	}
	outChanged := false
	animatedEnabledChanged := false
	animatedFormatChanged := false
	animatedDurationChanged := false
	animatedFPSChanged := false
	importPhotosChanged := false
	importAlbumChanged := false
	importTimeoutChanged := false
	liveEnabledChanged := false
	livePhotoFormatChanged := false
	liveCoverFrameChanged := false
	liveAssembleChanged := false
	liveOutputDirChanged := false
	liveImportPhotosChanged := false
	liveImportAlbumChanged := false
	liveImportTimeoutChanged := false
	chromePathChanged := false
	xhsTagsChanged := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "out":
			outChanged = true
		case "chrome":
			chromePathChanged = true
		case "animated":
			animatedEnabledChanged = true
		case "animated-format":
			animatedFormatChanged = true
		case "animated-duration":
			animatedDurationChanged = true
		case "animated-fps":
			animatedFPSChanged = true
		case "import-photos":
			importPhotosChanged = true
		case "import-album":
			importAlbumChanged = true
		case "import-timeout":
			importTimeoutChanged = true
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
		case "live-import-photos":
			liveImportPhotosChanged = true
		case "live-import-album":
			liveImportAlbumChanged = true
		case "live-import-timeout":
			liveImportTimeoutChanged = true
		case "xhs-tags":
			xhsTagsChanged = true
		}
	})
	opts.OutDirChanged = outChanged
	opts.ChromePathChanged = chromePathChanged
	opts.AnimatedEnabledChanged = animatedEnabledChanged
	opts.AnimatedFormatChanged = animatedFormatChanged
	opts.AnimatedDurationChanged = animatedDurationChanged
	opts.AnimatedFPSChanged = animatedFPSChanged
	opts.ImportPhotosChanged = importPhotosChanged
	opts.ImportAlbumChanged = importAlbumChanged
	opts.ImportTimeoutChanged = importTimeoutChanged
	opts.LiveEnabledChanged = liveEnabledChanged
	opts.LivePhotoFormatChanged = livePhotoFormatChanged
	opts.LiveCoverFrameChanged = liveCoverFrameChanged
	opts.LiveAssembleChanged = liveAssembleChanged
	opts.LiveOutputDirChanged = liveOutputDirChanged
	opts.LiveImportPhotosChanged = liveImportPhotosChanged
	opts.LiveImportAlbumChanged = liveImportAlbumChanged
	opts.LiveImportTimeoutChanged = liveImportTimeoutChanged
	opts.XHSTagsChanged = xhsTagsChanged
	if opts.XHSTagsChanged && !opts.PublishXHS {
		return Options{}, fmt.Errorf("--xhs-tags requires --publish-xhs\n\n%s", usageText())
	}
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

type publishXHSCLIOptions struct {
	app.PublishOptions

	ConfigPath              string
	ConfigPathChanged       bool
	AccountChanged          bool
	HeadlessChanged         bool
	ChromePathChanged       bool
	ProfileDirChanged       bool
	ModeChanged             bool
	DeclareOriginalChanged  bool
	AllowContentCopyChanged bool
}

func parsePublishXHSOptions(args []string) (publishXHSCLIOptions, error) {
	opts := publishXHSCLIOptions{
		PublishOptions: app.PublishOptions{
			ChromePath: defaultOptions().ChromePath,
			Headless:   true,
			Mode:       string(xhs.PublishModeOnlySelf),
		},
		ConfigPath: defaultOptions().ConfigPath,
	}
	var tags string
	var images string
	var livePages string
	fs := flag.NewFlagSet("mark2note publish-xhs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.ConfigPath, "config", opts.ConfigPath, "config file path")
	fs.StringVar(&opts.Account, "account", opts.Account, "publish account/profile target")
	fs.StringVar(&opts.Title, "title", opts.Title, "inline title text")
	fs.StringVar(&opts.TitleFile, "title-file", opts.TitleFile, "title file path")
	fs.StringVar(&opts.Content, "content", opts.Content, "inline content text")
	fs.StringVar(&opts.ContentFile, "content-file", opts.ContentFile, "content file path")
	fs.StringVar(&tags, "tags", tags, "comma-separated tags")
	fs.StringVar(&opts.Mode, "mode", opts.Mode, "publish mode")
	fs.StringVar(&opts.ScheduleAt, "schedule-at", opts.ScheduleAt, "schedule time")
	fs.StringVar(&images, "images", images, "comma-separated image paths")
	fs.StringVar(&opts.LiveReportPath, "live-report", opts.LiveReportPath, "live delivery report path")
	fs.StringVar(&livePages, "live-pages", livePages, "ordered live page subset")
	fs.StringVar(&opts.ChromePath, "chrome", opts.ChromePath, "chrome binary path")
	fs.BoolVar(&opts.Headless, "headless", opts.Headless, "run browser automation headless")
	fs.StringVar(&opts.ProfileDir, "profile-dir", opts.ProfileDir, "browser profile directory")
	fs.BoolVar(&opts.DeclareOriginal, "declare-original", opts.DeclareOriginal, "declare original content before submit")
	fs.BoolVar(&opts.AllowContentCopy, "allow-content-copy", opts.AllowContentCopy, "leave allow content copy enabled")

	if err := fs.Parse(args); err != nil {
		return publishXHSCLIOptions{}, err
	}
	if fs.NArg() > 0 {
		return publishXHSCLIOptions{}, fmt.Errorf("unexpected positional args: %s", strings.Join(fs.Args(), " "))
	}
	opts.Tags = splitCSV(tags)
	opts.ImagePaths = splitCSV(images)
	opts.LivePages = splitCSV(livePages)
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "config":
			opts.ConfigPathChanged = true
		case "account":
			opts.AccountChanged = true
		case "headless":
			opts.HeadlessChanged = true
		case "chrome":
			opts.ChromePathChanged = true
		case "profile-dir":
			opts.ProfileDirChanged = true
		case "mode":
			opts.ModeChanged = true
		case "declare-original":
			opts.DeclareOriginalChanged = true
		case "allow-content-copy":
			opts.AllowContentCopyChanged = true
		}
	})
	return opts, nil
}

func loadPublishXHSConfig(path string, explicit bool) (*config.Config, error) {
	cfg, err := loadConfig(path)
	if err == nil {
		return cfg, nil
	}
	if !explicit && errors.Is(err, os.ErrNotExist) {
		return &config.Config{}, nil
	}
	return nil, fmt.Errorf("%w: %v", app.ErrLoadConfig, err)
}

func mergePublishXHSDefaults(cli publishXHSCLIOptions, cfg *config.Config) app.PublishOptions {
	opts := cli.PublishOptions
	defaults := cfg.XHS.Publish
	if !cli.AccountChanged && strings.TrimSpace(opts.Account) == "" {
		opts.Account = strings.TrimSpace(defaults.Account)
	}
	if !cli.HeadlessChanged && defaults.Headless != nil {
		opts.Headless = *defaults.Headless
	}
	if !cli.ChromePathChanged && strings.TrimSpace(defaults.BrowserPath) != "" {
		opts.ChromePath = strings.TrimSpace(defaults.BrowserPath)
	}
	if !cli.ProfileDirChanged && strings.TrimSpace(opts.ProfileDir) == "" {
		opts.ProfileDir = strings.TrimSpace(defaults.ProfileDir)
	}
	if !cli.ModeChanged && strings.TrimSpace(defaults.Mode) != "" {
		opts.Mode = strings.TrimSpace(defaults.Mode)
	}
	if !cli.DeclareOriginalChanged && defaults.DeclareOriginal != nil {
		opts.DeclareOriginal = *defaults.DeclareOriginal
	}
	if !cli.AllowContentCopyChanged && defaults.AllowContentCopy != nil {
		opts.AllowContentCopy = *defaults.AllowContentCopy
	}
	if defaults.ChromeArgs != nil {
		opts.ChromeArgs = append([]string(nil), defaults.ChromeArgs...)
	}
	return opts
}

func validatePublishXHSOptions(opts app.PublishOptions) error {
	if strings.TrimSpace(opts.Account) == "" {
		return fmt.Errorf("--account is required\n\n%s", publishXHSUsageText())
	}
	if strings.TrimSpace(opts.Mode) == string(xhs.PublishModeSchedule) && strings.TrimSpace(opts.ScheduleAt) == "" {
		return fmt.Errorf("--schedule-at is required when --mode schedule\n\n%s", publishXHSUsageText())
	}
	if len(opts.LivePages) > 0 && strings.TrimSpace(opts.LiveReportPath) == "" {
		return fmt.Errorf("--live-pages requires --live-report\n\n%s", publishXHSUsageText())
	}
	if !exactlyOne(strings.TrimSpace(opts.Title) != "", strings.TrimSpace(opts.TitleFile) != "") {
		return fmt.Errorf("exactly one of --title / --title-file is required\n\n%s", publishXHSUsageText())
	}
	hasContent := strings.TrimSpace(opts.Content) != "" || strings.TrimSpace(opts.ContentFile) != ""
	if len(opts.Tags) == 0 && !hasContent {
		return fmt.Errorf("exactly one of --content / --content-file is required\n\n%s", publishXHSUsageText())
	}
	if strings.TrimSpace(opts.Content) != "" && strings.TrimSpace(opts.ContentFile) != "" {
		return fmt.Errorf("exactly one of --content / --content-file is required\n\n%s", publishXHSUsageText())
	}
	if !exactlyOne(len(opts.ImagePaths) > 0, strings.TrimSpace(opts.LiveReportPath) != "") {
		return fmt.Errorf("exactly one media source is required: --images or --live-report\n\n%s", publishXHSUsageText())
	}
	if _, err := xhs.ValidateMode(opts.Mode); err != nil {
		return fmt.Errorf("%v\n\n%s", err, publishXHSUsageText())
	}
	return nil
}

var loadConfig = config.Load
var readFile = os.ReadFile
var nowFunc = time.Now
var buildDeckJSON = func(cfg *config.Config, markdown string) (string, error) {
	b := ai.Builder{}
	b.SetCommand(cfg.AI.Command, cfg.AI.Args)
	return b.BuildDeckJSON(markdown)
}

var buildPublishTopics = func(cfg *config.Config, markdown string, title string) ([]string, error) {
	b := ai.TopicBuilder{}
	b.SetCommand(cfg.AI.Command, cfg.AI.Args)
	return b.BuildPublishTopics(markdown, title)
}

var buildPublishTitle = func(cfg *config.Config, markdown string, title string, maxRunes int) (string, error) {
	b := ai.TitleBuilder{}
	b.SetCommand(cfg.AI.Command, cfg.AI.Args)
	return b.BuildPublishTitle(markdown, title, maxRunes)
}

func newPreviewService(opts Options) app.Service {
	return app.Service{
		LoadConfig:  loadConfig,
		ReadFile:    readFile,
		PromptExtra: opts.PromptExtra,
		NewRenderer: func(o app.Options) app.DeckRenderer {
			return buildRenderer(o)
		},
	}
}

var generatePreview = func(opts Options) (app.Result, error) {
	return newPreviewService(opts).GeneratePreview(opts)
}
var generateFromDeck = func(opts Options) (app.Result, error) {
	return newPreviewService(opts).GenerateFromDeck(opts)
}
var publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
	svc := app.PublishService{
		ReadFile: readFile,
		Now:      nowFunc,
	}
	return svc.Publish(opts)
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
		ImportPhotos:   opts.ImportPhotos,
		ImportAlbum:    opts.ImportAlbum,
		ImportTimeout:  opts.ImportTimeout,
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
		Enabled:       opts.Enabled,
		PhotoFormat:   opts.PhotoFormat,
		CoverFrame:    opts.CoverFrame,
		Assemble:      opts.Assemble,
		OutputDir:     outputDir,
		ImportPhotos:  opts.ImportPhotos,
		ImportAlbum:   opts.ImportAlbum,
		ImportTimeout: opts.ImportTimeout,
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

func buildAutoPublishXHSOptions(renderOpts Options, renderResult app.Result) (app.PublishOptions, error) {
	imagePaths := nonEmptyStrings(renderResult.ImagePaths)
	if len(imagePaths) == 0 {
		return app.PublishOptions{}, fmt.Errorf("no generated PNG files found")
	}
	for _, imagePath := range imagePaths {
		if _, err := os.Stat(imagePath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return app.PublishOptions{}, fmt.Errorf("generated PNG file not found: %s", imagePath)
			}
			return app.PublishOptions{}, fmt.Errorf("generated PNG file unavailable: %s: %v", imagePath, err)
		}
	}
	markdownBytes, err := readFile(renderOpts.InputPath)
	if err != nil {
		return app.PublishOptions{}, fmt.Errorf("%w: %v", app.ErrReadMarkdown, err)
	}
	markdown := string(markdownBytes)
	title := xhs.MarkdownPublishTitle(markdown, renderOpts.InputPath)
	cliOpts := publishXHSCLIOptions{
		PublishOptions: app.PublishOptions{
			Title:      title,
			Mode:       string(xhs.PublishModeOnlySelf),
			ImagePaths: imagePaths,
			ChromePath: renderOpts.ChromePath,
			Headless:   true,
		},
		ConfigPath:        renderOpts.ConfigPath,
		ChromePathChanged: renderOpts.ChromePathChanged,
	}
	cfg, err := loadPublishXHSConfig(cliOpts.ConfigPath, true)
	if err != nil {
		return app.PublishOptions{}, err
	}
	title, err = buildAutoPublishXHSTitle(*cfg, markdown, title)
	if err != nil {
		return app.PublishOptions{}, err
	}
	cliOpts.Title = title
	topics, err := buildAutoPublishXHSTopics(*cfg, markdown, title, renderOpts.XHSTags)
	if err != nil {
		return app.PublishOptions{}, err
	}
	cliOpts.Tags = topics
	publishOpts := mergePublishXHSDefaults(cliOpts, cfg)
	if err := validatePublishXHSOptions(publishOpts); err != nil {
		return app.PublishOptions{}, err
	}
	return publishOpts, nil
}

func buildAutoPublishXHSTitle(cfg config.Config, markdown string, title string) (string, error) {
	title = xhs.NormalizePublishTitle(title)
	maxRunes := cfg.XHS.Publish.TitleGeneration.MaxRunes
	if maxRunes == 0 {
		maxRunes = config.DefaultXHSPublishTitleMaxRunes
	}
	if maxRunes < 0 {
		return "", fmt.Errorf("xhs publish title max_runes must be > 0")
	}
	if utf8.RuneCountInString(title) <= maxRunes {
		return title, nil
	}
	if cfg.XHS.Publish.TitleGeneration.Enabled == nil || !*cfg.XHS.Publish.TitleGeneration.Enabled {
		return "", fmt.Errorf("xhs publish title exceeds %d characters; enable xhs.publish.title_generation.enabled or use a shorter Markdown title", maxRunes)
	}
	rewritten, err := buildPublishTitle(&cfg, markdown, title, maxRunes)
	if err != nil {
		return "", fmt.Errorf("generate xhs publish title: %w", err)
	}
	rewritten = xhs.NormalizePublishTitle(rewritten)
	if rewritten == "" {
		return "", fmt.Errorf("generate xhs publish title: empty title returned")
	}
	if utf8.RuneCountInString(rewritten) > maxRunes {
		return "", fmt.Errorf("generate xhs publish title: title still exceeds %d characters", maxRunes)
	}
	return rewritten, nil
}

func buildAutoPublishXHSTopics(cfg config.Config, markdown string, title string, overrideTags []string) ([]string, error) {
	if len(overrideTags) > 0 {
		return xhs.NormalizePublishTopics(overrideTags), nil
	}
	if cfg.XHS.Publish.TopicGeneration.Enabled == nil || !*cfg.XHS.Publish.TopicGeneration.Enabled {
		return nil, fmt.Errorf("xhs publish topic generation is disabled; pass --xhs-tags or enable xhs.publish.topic_generation.enabled")
	}
	topics, err := buildPublishTopics(&cfg, markdown, title)
	if err != nil {
		return nil, fmt.Errorf("generate xhs publish topics: %w", err)
	}
	topics = xhs.NormalizePublishTopics(topics)
	if len(topics) == 0 {
		return nil, fmt.Errorf("generate xhs publish topics: no valid topics returned")
	}
	return topics, nil
}

func runAutoPublishXHS(renderOpts Options, renderResult app.Result, stdout io.Writer, stderr io.Writer) int {
	publishOpts, err := buildAutoPublishXHSOptions(renderOpts, renderResult)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrLoadConfig):
			fmt.Fprintf(stderr, "error loading config: %s\n", stripErrorPrefixes(err, app.ErrLoadConfig))
		case errors.Is(err, app.ErrReadMarkdown):
			fmt.Fprintf(stderr, "error reading markdown for xhs publish: %s\n", stripErrorPrefixes(err, app.ErrReadMarkdown))
		default:
			fmt.Fprintf(stderr, "auto publish xhs failed: %v\n", err)
		}
		return 1
	}
	result, err := publishXHS(publishOpts)
	if err != nil {
		printPublishXHSError(stderr, err)
		return 1
	}
	return printPublishXHSResult(stdout, result)
}

func nonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "capture-html":
			return runCaptureHTML(args[1:], stdout, stderr)
		case "publish-xhs":
			return runPublishXHS(args[1:], stdout, stderr)
		}
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

	generate := generatePreview
	if strings.TrimSpace(opts.FromDeckPath) != "" {
		generate = generateFromDeck
	}
	result, err := generate(opts)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrLoadConfig):
			fmt.Fprintf(stderr, "error loading config: %s\n", stripErrorPrefixes(err, app.ErrLoadConfig))
		case errors.Is(err, app.ErrReadMarkdown):
			fmt.Fprintf(stderr, "error reading markdown: %s\n", stripErrorPrefixes(err, app.ErrReadMarkdown))
		case errors.Is(err, app.ErrReadDeck):
			fmt.Fprintf(stderr, "error reading deck: %s\n", stripErrorPrefixes(err, app.ErrReadDeck))
		case errors.Is(err, app.ErrReadRenderMeta):
			fmt.Fprintf(stderr, "error reading render meta: %s\n", stripErrorPrefixes(err, app.ErrReadRenderMeta))
		case errors.Is(err, app.ErrBuildDeckJSON):
			fmt.Fprintf(stderr, "error building deck json: %s\n", stripErrorPrefixes(err, app.ErrBuildDeckJSON))
		default:
			fmt.Fprintf(stderr, "render preview failed: %s\n", stripErrorPrefixes(err, app.ErrParseDeck, app.ErrRenderPreview))
		}
		if result.ImportReportPath != "" {
			fmt.Fprintf(stderr, "photos import report: %s\n", result.ImportReportPath)
		}
		if result.DeliveryReportPath != "" {
			fmt.Fprintf(stderr, "live delivery report: %s\n", result.DeliveryReportPath)
		}
		return 1
	}

	fmt.Fprintf(stdout, "generated %d preview pages\n", result.PageCount)
	if result.ImportReport != nil {
		fmt.Fprintf(stdout, "photos import: %s (%s)\n", result.ImportReport.Status, result.ImportReport.Message)
		if result.ImportReportPath != "" {
			fmt.Fprintf(stdout, "photos import report: %s\n", result.ImportReportPath)
		}
	}
	if result.DeliveryReport != nil {
		fmt.Fprintf(stdout, "live delivery: %s (%s)\n", result.DeliveryReport.Status, result.DeliveryReport.Message)
		if result.DeliveryReportPath != "" {
			fmt.Fprintf(stdout, "live delivery report: %s\n", result.DeliveryReportPath)
		}
	}
	for _, warning := range result.Warnings {
		fmt.Fprintln(stderr, warning)
	}
	if opts.PublishXHS {
		return runAutoPublishXHS(opts, result, stdout, stderr)
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

func printPublishXHSError(stderr io.Writer, err error) {
	switch {
	case errors.Is(err, xhs.ErrNotLoggedIn):
		fmt.Fprintln(stderr, "not logged in to Xiaohongshu creator center; open the Chrome profile and complete QR login")
	case errors.Is(err, xhs.ErrUploadInputMissing):
		fmt.Fprintf(stderr, "publish xhs failed: upload input not found; the Xiaohongshu session may be expired, blocked by login/verification, or not on the image publish page. Open the configured Chrome profile, complete login or verification, then retry. detail: %s\n", stripErrorPrefixes(err, app.ErrPublishExecute, xhs.ErrUploadFailed, xhs.ErrUploadInputMissing))
	case errors.Is(err, xhs.ErrLiveBridgeFailed), errors.Is(err, xhs.ErrLiveBridgePermissionDenied), errors.Is(err, xhs.ErrPhotosLookupFailed), errors.Is(err, xhs.ErrLivePublishUnsupported):
		fmt.Fprintf(stderr, "live attach failed: %s\n", stripErrorPrefixes(err, app.ErrPublishExecute))
	case errors.Is(err, app.ErrPublishRequestInvalid):
		fmt.Fprintf(stderr, "invalid publish request: %s\n", stripErrorPrefixes(err, app.ErrPublishRequestInvalid))
	case errors.Is(err, app.ErrPublishReadInput):
		fmt.Fprintf(stderr, "error reading publish input: %s\n", stripErrorPrefixes(err, app.ErrPublishReadInput))
	default:
		fmt.Fprintf(stderr, "publish xhs failed: %s\n", stripErrorPrefixes(err, app.ErrPublishExecute))
	}
}

func printPublishXHSResult(stdout io.Writer, result app.PublishResult) int {
	if result.Result.OnlySelfPublished {
		fmt.Fprintln(stdout, "xiaohongshu only-self-visible published")
		fmt.Fprintf(stdout, "account: %s\n", result.Request.Account)
		fmt.Fprintf(stdout, "media: %s\n", result.Result.MediaKind)
		if result.Result.AttachedCount > 0 {
			fmt.Fprintf(stdout, "attached: %d\n", result.Result.AttachedCount)
		}
		if len(result.Result.AttachedItems) > 0 {
			fmt.Fprintf(stdout, "items: %s\n", strings.Join(result.Result.AttachedItems, ","))
		}
		return 0
	}
	if result.Result.Mode == xhs.PublishModeSchedule && result.Result.ScheduleTime != nil {
		fmt.Fprintln(stdout, "xiaohongshu scheduled publish submitted")
		fmt.Fprintf(stdout, "account: %s\n", result.Request.Account)
		fmt.Fprintf(stdout, "at: %s\n", result.Result.ScheduleTime.In(xhsShanghaiLocation()).Format("2006-01-02 15:04:05"))
		fmt.Fprintf(stdout, "media: %s\n", result.Result.MediaKind)
		if result.Result.AttachedCount > 0 {
			fmt.Fprintf(stdout, "attached: %d\n", result.Result.AttachedCount)
		}
		if len(result.Result.AttachedItems) > 0 {
			fmt.Fprintf(stdout, "items: %s\n", strings.Join(result.Result.AttachedItems, ","))
		}
		return 0
	}
	fmt.Fprintf(stdout, "xhs publish queued for %s (%s)\n", result.Request.Account, result.Request.Mode)
	if result.Request.ScheduleTime != nil {
		fmt.Fprintf(stdout, "schedule at: %s\n", result.Request.ScheduleTime.In(xhsShanghaiLocation()).Format("2006-01-02 15:04:05"))
	}
	return 0
}

func runPublishXHS(args []string, stdout io.Writer, stderr io.Writer) int {
	cliOpts, err := parsePublishXHSOptions(args)
	if err != nil {
		if err == flag.ErrHelp {
			fmt.Fprintln(stdout, publishXHSUsageText())
			return 0
		}
		fmt.Fprintf(stderr, "error parsing flags: %v\n", err)
		return 1
	}
	cfg, err := loadPublishXHSConfig(cliOpts.ConfigPath, cliOpts.ConfigPathChanged)
	if err != nil {
		fmt.Fprintf(stderr, "error loading config: %s\n", stripErrorPrefixes(err, app.ErrLoadConfig))
		return 1
	}
	opts := mergePublishXHSDefaults(cliOpts, cfg)
	if err := validatePublishXHSOptions(opts); err != nil {
		fmt.Fprintf(stderr, "error parsing flags: %v\n", err)
		return 1
	}
	result, err := publishXHS(opts)
	if err != nil {
		printPublishXHSError(stderr, err)
		return 1
	}
	return printPublishXHSResult(stdout, result)
}

func splitCSV(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func exactlyOne(values ...bool) bool {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count == 1
}

func xhsShanghaiLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
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
