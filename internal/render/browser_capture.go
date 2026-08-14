package render

import (
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
