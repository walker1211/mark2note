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

	"github.com/walker1211/mark2note/internal/deck"
)

const defaultChromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

type CommandRunner interface {
	Run(name string, args ...string) error
}

type Renderer struct {
	OutDir     string
	ChromePath string
	Jobs       int
	Runner     CommandRunner
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

func (r Renderer) Render(d deck.Deck) error {
	if len(d.Pages) == 0 {
		return fmt.Errorf("deck must contain at least 1 page for render")
	}
	outDir := r.effectiveOutDir(d)
	if outDir == "" {
		return fmt.Errorf("out dir is required")
	}
	if err := r.RenderHTMLPages(d); err != nil {
		return err
	}
	if err := r.CapturePNGs(d.Pages, outDir); err != nil {
		return err
	}
	return nil
}

func (r Renderer) RenderHTMLPages(d deck.Deck) error {
	outDir := r.effectiveOutDir(d)
	if outDir == "" {
		return fmt.Errorf("out dir is required")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}

	for _, page := range d.Pages {
		html, err := RenderPageHTML(d, page)
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

func (r Renderer) effectiveRunner() CommandRunner {
	if r.Runner != nil {
		return r.Runner
	}
	return execRunner{}
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
	return r.runCaptureTasks(tasks)
}

func (r Renderer) CaptureHTMLPath(inputPath string) error {
	tasks, err := collectCaptureTasks(inputPath)
	if err != nil {
		return fmt.Errorf("capture html path %s: %w", inputPath, err)
	}
	return r.runCaptureTasks(tasks)
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

func (r Renderer) runCaptureTasks(captureTasks []captureTask) error {
	runner := r.effectiveRunner()
	chrome := r.effectiveChromePath()
	tasks := make(chan captureTask)
	errCh := make(chan error, len(captureTasks))

	var wg sync.WaitGroup
	for i := 0; i < r.effectiveJobs(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				if err := runner.Run(chrome, r.screenshotArgs(task)...); err != nil {
					errCh <- fmt.Errorf("screenshot %s: %w", task.name, err)
				}
			}
		}()
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
		"--window-size=1242,1656",
		"--screenshot=" + task.pngPath,
		fileURI(task.htmlPath),
	}
}

func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}
