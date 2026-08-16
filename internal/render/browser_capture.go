package render

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
)

const captureReadyTimeout = 15 * time.Second

const capturePaintProbeAttempts = 4

const captureReadyScript = `() => Promise.all([
  document.fonts && document.fonts.ready ? document.fonts.ready : Promise.resolve(),
  ...Array.from(document.images).map((image) => {
    if (image.complete) {
      return typeof image.decode === 'function' ? image.decode().catch(() => undefined) : Promise.resolve();
    }
    return new Promise((resolve) => {
      image.addEventListener('load', resolve, { once: true });
      image.addEventListener('error', resolve, { once: true });
    });
  })
]).then(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))))`

const capturePaintProbeWaitScript = `() => new Promise((resolve) => {
  requestAnimationFrame(() => setTimeout(() => requestAnimationFrame(resolve), 100));
})`

type rodCaptureBrowser struct{}

func (rodCaptureBrowser) Capture(tasks []captureTask, jobs, width, height int, chromePath string) error {
	if len(tasks) == 0 {
		return nil
	}
	if jobs < 1 {
		jobs = 1
	}
	if jobs > len(tasks) {
		jobs = len(tasks)
	}

	instance := launcher.New().Headless(true).
		Set(flags.Flag("disable-gpu")).
		Set(flags.Flag("hide-scrollbars")).
		Set(flags.Flag("allow-file-access-from-files")).
		Set(flags.Flag("disable-background-networking"))
	if chromePath = strings.TrimSpace(chromePath); chromePath != "" {
		instance = instance.Bin(chromePath)
	}
	controlURL, err := instance.Launch()
	if err != nil {
		return fmt.Errorf("launch shared screenshot browser: %w", err)
	}
	defer instance.Cleanup()

	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("connect shared screenshot browser: %w", err)
	}
	defer func() { _ = browser.Close() }()

	work := make(chan captureTask)
	errCh := make(chan error, len(tasks))
	var wg sync.WaitGroup
	for range jobs {
		wg.Go(func() {
			for task := range work {
				if err := capturePage(browser, task, width, height); err != nil {
					errCh <- fmt.Errorf("screenshot %s: %w", task.name, err)
				}
			}
		})
	}
	for _, task := range tasks {
		work <- task
	}
	close(work)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func capturePage(browser *rod.Browser, task captureTask, width, height int) error {
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return err
	}
	defer func() { _ = page.Close() }()

	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             width,
		Height:            height,
		DeviceScaleFactor: 1,
		Mobile:            false,
	}); err != nil {
		return err
	}
	if err := page.Navigate(fileURI(task.htmlPath)); err != nil {
		return err
	}
	timedPage := page.Timeout(captureReadyTimeout)
	if err := timedPage.WaitLoad(); err != nil {
		return err
	}
	if _, err := timedPage.Eval(captureReadyScript); err != nil {
		return err
	}
	if !task.skipPaintStability {
		if err := waitForStablePaint(timedPage); err != nil {
			return err
		}
	}
	png, err := timedPage.Screenshot(false, &proto.PageCaptureScreenshot{
		Format:      proto.PageCaptureScreenshotFormatPng,
		FromSurface: true,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(task.pngPath, png, 0o644); err != nil {
		return err
	}
	return nil
}

func waitForStablePaint(page *rod.Page) error {
	quality := 10
	return waitForMatchingPaintProbes(
		func() ([]byte, error) {
			return page.Screenshot(false, &proto.PageCaptureScreenshot{
				Format:           proto.PageCaptureScreenshotFormatJpeg,
				Quality:          &quality,
				FromSurface:      true,
				OptimizeForSpeed: true,
			})
		},
		func() error {
			_, err := page.Eval(capturePaintProbeWaitScript)
			return err
		},
		capturePaintProbeAttempts,
	)
}

func waitForMatchingPaintProbes(capture func() ([]byte, error), wait func() error, attempts int) error {
	if attempts < 2 {
		return fmt.Errorf("paint stability requires at least 2 probes")
	}
	var previous []byte
	for attempt := 1; attempt <= attempts; attempt++ {
		current, err := capture()
		if err != nil {
			return fmt.Errorf("capture paint probe %d: %w", attempt, err)
		}
		if attempt > 1 && bytes.Equal(previous, current) {
			return nil
		}
		previous = current
		if attempt < attempts {
			if err := wait(); err != nil {
				return fmt.Errorf("wait after paint probe %d: %w", attempt, err)
			}
		}
	}
	return fmt.Errorf("page paint did not stabilize after %d probes", attempts)
}
