package xhs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeSessionLauncher struct {
	url string
	err error
}

func (f fakeSessionLauncher) Launch() (string, error) {
	return f.url, f.err
}

type fakeSessionBrowser struct {
	page       *fakeSessionPage
	pageErr    error
	pageURLs   []string
	closeCalls int
}

func (f *fakeSessionBrowser) Page(url string) (sessionPage, error) {
	f.pageURLs = append(f.pageURLs, url)
	if f.pageErr != nil {
		return nil, f.pageErr
	}
	return f.page, nil
}

func (f *fakeSessionBrowser) Close() error {
	f.closeCalls++
	return nil
}

type fakeSessionPage struct {
	url             string
	account         string
	hasLoginPrompt  bool
	navigations     []string
	navigateErr     error
	urlErr          error
	accountErr      error
	promptErr       error
	urlSequence     []string
	accountSequence []string
	promptSequence  []bool
	urlCalls        int
	accountCalls    int
	promptCalls     int
}

func (f *fakeSessionPage) Navigate(url string) error {
	f.navigations = append(f.navigations, url)
	return f.navigateErr
}

func (f *fakeSessionPage) URL() (string, error) {
	if f.urlErr != nil {
		return "", f.urlErr
	}
	if len(f.urlSequence) == 0 {
		return f.url, nil
	}
	index := f.urlCalls
	if index >= len(f.urlSequence) {
		index = len(f.urlSequence) - 1
	}
	f.urlCalls++
	return f.urlSequence[index], nil
}

func (f *fakeSessionPage) AccountName() (string, error) {
	if f.accountErr != nil {
		return "", f.accountErr
	}
	if len(f.accountSequence) == 0 {
		return f.account, nil
	}
	index := f.accountCalls
	if index >= len(f.accountSequence) {
		index = len(f.accountSequence) - 1
	}
	f.accountCalls++
	return f.accountSequence[index], nil
}

func (f *fakeSessionPage) HasLoginPrompt() (bool, error) {
	if f.promptErr != nil {
		return false, f.promptErr
	}
	if len(f.promptSequence) == 0 {
		return f.hasLoginPrompt, nil
	}
	index := f.promptCalls
	if index >= len(f.promptSequence) {
		index = len(f.promptSequence) - 1
	}
	f.promptCalls++
	return f.promptSequence[index], nil
}

func (f *fakeSessionPage) Open(context.Context) error                          { return nil }
func (f *fakeSessionPage) UploadImages(context.Context, []string) error        { return nil }
func (f *fakeSessionPage) FillTitle(context.Context, string) error             { return nil }
func (f *fakeSessionPage) FillContent(context.Context, string, []string) error { return nil }
func (f *fakeSessionPage) SaveDraft(context.Context) error                     { return nil }
func (f *fakeSessionPage) ConfirmDraftSaved(context.Context) error             { return nil }
func (f *fakeSessionPage) SetSchedule(context.Context, time.Time) error        { return nil }
func (f *fakeSessionPage) SubmitScheduled(context.Context) error               { return nil }
func (f *fakeSessionPage) ConfirmScheduledSubmitted(context.Context) error     { return nil }

func TestBrowserSessionResolvesDefaultProfileDir(t *testing.T) {
	got, err := resolveSessionProfileDir(func() (string, error) { return "/Users/test/Library/Application Support", nil }, "writer", "")
	if err != nil {
		t.Fatalf("resolveSessionProfileDir() error = %v", err)
	}
	want := filepath.Join("/Users/test/Library/Application Support", "mark2note", "xhs", "profiles", "writer")
	if got != want {
		t.Fatalf("resolveSessionProfileDir() = %q, want %q", got, want)
	}
}

func TestBrowserSessionUsesExplicitProfileDir(t *testing.T) {
	got, err := resolveSessionProfileDir(func() (string, error) { t.Fatal("userConfigDir should not be called"); return "", nil }, "writer", "/tmp/xhs-profile")
	if err != nil {
		t.Fatalf("resolveSessionProfileDir() error = %v", err)
	}
	if got != "/tmp/xhs-profile" {
		t.Fatalf("resolveSessionProfileDir() = %q, want %q", got, "/tmp/xhs-profile")
	}
}

func TestBrowserSessionCheckLoginReturnsNotLoggedIn(t *testing.T) {
	tempDir := t.TempDir()
	page := &fakeSessionPage{url: xhsLoginURL, hasLoginPrompt: true}
	browser := &fakeSessionBrowser{page: page}
	session := &rodBrowserSession{
		opts:               SessionOptions{Account: "writer", Headless: false},
		userConfigDir:      func() (string, error) { return tempDir, nil },
		mkdirAll:           os.MkdirAll,
		readFile:           os.ReadFile,
		writeFile:          func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher:        func(SessionOptions, string) sessionLauncher { return fakeSessionLauncher{url: "ws://browser"} },
		newBrowser:         func(string) (sessionBrowser, error) { return browser, nil },
		loginPollInterval:  20 * time.Millisecond,
		interactiveTimeout: 200 * time.Millisecond,
	}
	err := session.EnsureLoggedIn(context.Background())
	if err == nil || !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
	if !strings.Contains(err.Error(), "timed out waiting for QR login") {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
	if len(page.navigations) != 0 {
		t.Fatalf("login navigations = %#v", page.navigations)
	}
}

func TestBrowserSessionCheckLoginReturnsAccountMismatch(t *testing.T) {
	tempDir := t.TempDir()
	page := &fakeSessionPage{url: xhsPublishURL, account: "other-account"}
	browser := &fakeSessionBrowser{page: page}
	session := &rodBrowserSession{
		opts:          SessionOptions{Account: "writer", Headless: false},
		userConfigDir: func() (string, error) { return tempDir, nil },
		mkdirAll:      os.MkdirAll,
		readFile:      os.ReadFile,
		writeFile:     func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher: func(SessionOptions, string) sessionLauncher {
			return fakeSessionLauncher{url: "ws://browser"}
		},
		newBrowser: func(string) (sessionBrowser, error) { return browser, nil },
	}
	err := session.EnsureLoggedIn(context.Background())
	if err == nil || !errors.Is(err, ErrAccountMismatch) {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
	if !strings.Contains(err.Error(), "other-account") {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
}

func TestBrowserSessionRejectsHeadlessWhenLoginRequired(t *testing.T) {
	tempDir := t.TempDir()
	page := &fakeSessionPage{url: xhsLoginURL, hasLoginPrompt: true}
	browser := &fakeSessionBrowser{page: page}
	session := &rodBrowserSession{
		opts:          SessionOptions{Account: "writer", Headless: true},
		userConfigDir: func() (string, error) { return tempDir, nil },
		mkdirAll:      os.MkdirAll,
		readFile:      os.ReadFile,
		writeFile:     func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher:   func(SessionOptions, string) sessionLauncher { return fakeSessionLauncher{url: "ws://browser"} },
		newBrowser:    func(string) (sessionBrowser, error) { return browser, nil },
	}
	err := session.EnsureLoggedIn(context.Background())
	if err == nil || !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
	if !strings.Contains(err.Error(), "without --headless") {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
	if len(page.navigations) != 0 {
		t.Fatalf("headless navigations = %#v, want none", page.navigations)
	}
}

func TestBrowserSessionReturnsNavigateLoginError(t *testing.T) {
	tempDir := t.TempDir()
	page := &fakeSessionPage{url: xhsPublishURL, navigateErr: errors.New("boom")}
	browser := &fakeSessionBrowser{page: page}
	session := &rodBrowserSession{
		opts:             SessionOptions{Account: "writer", Headless: false},
		userConfigDir:    func() (string, error) { return tempDir, nil },
		mkdirAll:         os.MkdirAll,
		readFile:         os.ReadFile,
		writeFile:        func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher:      func(SessionOptions, string) sessionLauncher { return fakeSessionLauncher{url: "ws://browser"} },
		newBrowser:       func(string) (sessionBrowser, error) { return browser, nil },
		loginGracePeriod: 0,
	}
	err := session.EnsureLoggedIn(context.Background())
	if err == nil || !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
	if !strings.Contains(err.Error(), "navigate to login page") {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
}

func TestBrowserSessionWaitsForInteractiveLoginCompletion(t *testing.T) {
	tempDir := t.TempDir()
	page := &fakeSessionPage{
		urlSequence:     []string{xhsLoginURL, xhsPublishURL},
		accountSequence: []string{"", "writer"},
		promptSequence:  []bool{true},
	}
	browser := &fakeSessionBrowser{page: page}
	session := &rodBrowserSession{
		opts:               SessionOptions{Account: "writer", Headless: false},
		userConfigDir:      func() (string, error) { return tempDir, nil },
		mkdirAll:           os.MkdirAll,
		readFile:           os.ReadFile,
		writeFile:          func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher:        func(SessionOptions, string) sessionLauncher { return fakeSessionLauncher{url: "ws://browser"} },
		newBrowser:         func(string) (sessionBrowser, error) { return browser, nil },
		loginPollInterval:  20 * time.Millisecond,
		interactiveTimeout: 500 * time.Millisecond,
	}
	if err := session.EnsureLoggedIn(context.Background()); err != nil {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
}

func TestBrowserSessionTreatsCreatorHomeAsInteractiveLoginSuccess(t *testing.T) {
	tempDir := t.TempDir()
	page := &fakeSessionPage{
		urlSequence:     []string{xhsLoginURL, "https://creator.xiaohongshu.com/new/home", "https://creator.xiaohongshu.com/new/home"},
		accountSequence: []string{"", "", "walker"},
		promptSequence:  []bool{true, false, false},
	}
	browser := &fakeSessionBrowser{page: page}
	session := &rodBrowserSession{
		opts:               SessionOptions{Account: "walker", Headless: false},
		userConfigDir:      func() (string, error) { return tempDir, nil },
		mkdirAll:           os.MkdirAll,
		readFile:           os.ReadFile,
		writeFile:          func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher:        func(SessionOptions, string) sessionLauncher { return fakeSessionLauncher{url: "ws://browser"} },
		newBrowser:         func(string) (sessionBrowser, error) { return browser, nil },
		loginPollInterval:  20 * time.Millisecond,
		interactiveTimeout: 500 * time.Millisecond,
	}
	if err := session.EnsureLoggedIn(context.Background()); err != nil {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
}

func TestBrowserSessionRejectsCreatorHomeFallbackWithoutMatchingProfileMarker(t *testing.T) {
	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "mark2note", "xhs", "profiles", "walker")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, profileStateFile), []byte(`{"account":"other"}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	page := &fakeSessionPage{
		urlSequence:     []string{xhsLoginURL, "https://creator.xiaohongshu.com/new/home", "https://creator.xiaohongshu.com/new/home"},
		accountSequence: []string{"", "", ""},
		promptSequence:  []bool{true, false, false},
	}
	browser := &fakeSessionBrowser{page: page}
	session := &rodBrowserSession{
		opts:               SessionOptions{Account: "walker", Headless: false},
		userConfigDir:      func() (string, error) { return tempDir, nil },
		mkdirAll:           os.MkdirAll,
		readFile:           os.ReadFile,
		writeFile:          func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher:        func(SessionOptions, string) sessionLauncher { return fakeSessionLauncher{url: "ws://browser"} },
		newBrowser:         func(string) (sessionBrowser, error) { return browser, nil },
		loginPollInterval:  20 * time.Millisecond,
		interactiveTimeout: 500 * time.Millisecond,
	}
	err := session.EnsureLoggedIn(context.Background())
	if err == nil || !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
	if !strings.Contains(err.Error(), "account identity could not be confirmed") {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
}

func TestBrowserSessionAccountMismatchIgnoresCase(t *testing.T) {
	tempDir := t.TempDir()
	page := &fakeSessionPage{url: xhsPublishURL, account: "Walker"}
	browser := &fakeSessionBrowser{page: page}
	session := &rodBrowserSession{
		opts:          SessionOptions{Account: "walker", Headless: false},
		userConfigDir: func() (string, error) { return tempDir, nil },
		mkdirAll:      os.MkdirAll,
		readFile:      os.ReadFile,
		writeFile:     func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher:   func(SessionOptions, string) sessionLauncher { return fakeSessionLauncher{url: "ws://browser"} },
		newBrowser:    func(string) (sessionBrowser, error) { return browser, nil },
	}
	if err := session.EnsureLoggedIn(context.Background()); err != nil {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
}

func TestSanitizeAccountNameRejectsPlatformLabel(t *testing.T) {
	if got := sanitizeAccountName("创作服务平台"); got != "" {
		t.Fatalf("sanitizeAccountName() = %q, want empty", got)
	}
}

func TestSanitizeAccountNameKeepsRealNicknameContainingKeyword(t *testing.T) {
	if got := sanitizeAccountName("小红书运营阿明"); got != "小红书运营阿明" {
		t.Fatalf("sanitizeAccountName() = %q", got)
	}
}

func TestAccountProbeScriptContainsLocalStorageFallback(t *testing.T) {
	for _, want := range []string{"USER_INFO_FOR_BIZ", "window.localStorage", "userName"} {
		if !strings.Contains(accountProbeScript, want) {
			t.Fatalf("accountProbeScript missing %q", want)
		}
	}
}

func TestBrowserSessionAcceptsProfileMarkerCaseDifference(t *testing.T) {
	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "mark2note", "xhs", "profiles", "walker")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, profileStateFile), []byte(`{"account":"walker"}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	page := &fakeSessionPage{url: xhsPublishURL, account: "Walker"}
	browser := &fakeSessionBrowser{page: page}
	session := &rodBrowserSession{
		opts:          SessionOptions{Account: "walker", Headless: false},
		userConfigDir: func() (string, error) { return tempDir, nil },
		mkdirAll:      os.MkdirAll,
		readFile:      os.ReadFile,
		writeFile:     func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher:   func(SessionOptions, string) sessionLauncher { return fakeSessionLauncher{url: "ws://browser"} },
		newBrowser:    func(string) (sessionBrowser, error) { return browser, nil },
	}
	if err := session.EnsureLoggedIn(context.Background()); err != nil {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
}

func TestResolveSessionProfileDirRejectsInvalidAccount(t *testing.T) {
	_, err := resolveSessionProfileDir(func() (string, error) { return "/tmp", nil }, "../walker", "")
	if err == nil {
		t.Fatal("resolveSessionProfileDir() error = nil, want invalid account error")
	}
}

func TestReadRunningBrowserControlURL(t *testing.T) {
	profileDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(profileDir, "DevToolsActivePort"), []byte("9222\n/devtools/browser/test-browser\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, ok, err := readRunningBrowserControlURL(os.ReadFile, profileDir)
	if err != nil {
		t.Fatalf("readRunningBrowserControlURL() error = %v", err)
	}
	if !ok {
		t.Fatal("readRunningBrowserControlURL() ok = false, want true")
	}
	if got != "ws://127.0.0.1:9222/devtools/browser/test-browser" {
		t.Fatalf("readRunningBrowserControlURL() = %q", got)
	}
}

func TestBrowserSessionOpenReusesRunningBrowser(t *testing.T) {
	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "existing-profile")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "DevToolsActivePort"), []byte("9222\n/devtools/browser/test-browser\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	page := &fakeSessionPage{}
	browser := &fakeSessionBrowser{page: page}
	var gotControlURL string
	session := &rodBrowserSession{
		opts:          SessionOptions{Account: "walker", ProfileDir: profileDir},
		userConfigDir: func() (string, error) { return tempDir, nil },
		mkdirAll:      os.MkdirAll,
		readFile:      os.ReadFile,
		writeFile:     func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher: func(SessionOptions, string) sessionLauncher {
			t.Fatal("newLauncher should not be called when reusing running browser")
			return fakeSessionLauncher{}
		},
		newBrowser: func(controlURL string) (sessionBrowser, error) {
			gotControlURL = controlURL
			return browser, nil
		},
	}
	if err := session.Open(context.Background()); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if gotControlURL != "ws://127.0.0.1:9222/devtools/browser/test-browser" {
		t.Fatalf("controlURL = %q", gotControlURL)
	}
	if session.ownsBrowser {
		t.Fatal("session.ownsBrowser = true, want false")
	}
	if len(browser.pageURLs) != 1 || browser.pageURLs[0] != xhsPublishURL {
		t.Fatalf("pageURLs = %#v", browser.pageURLs)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if browser.closeCalls != 0 {
		t.Fatalf("closeCalls = %d, want 0", browser.closeCalls)
	}
}

func TestBrowserSessionDoesNotTrustProfileMarkerAsCurrentAccountProof(t *testing.T) {
	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "mark2note", "xhs", "profiles", "walker")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, profileStateFile), []byte(`{"account":"walker"}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	page := &fakeSessionPage{
		urlSequence:     []string{xhsLoginURL, "https://creator.xiaohongshu.com/new/home", "https://creator.xiaohongshu.com/new/home"},
		accountSequence: []string{"", "", ""},
		promptSequence:  []bool{true, false, false},
	}
	browser := &fakeSessionBrowser{page: page}
	session := &rodBrowserSession{
		opts:               SessionOptions{Account: "walker", Headless: false},
		userConfigDir:      func() (string, error) { return tempDir, nil },
		mkdirAll:           os.MkdirAll,
		readFile:           os.ReadFile,
		writeFile:          func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher:        func(SessionOptions, string) sessionLauncher { return fakeSessionLauncher{url: "ws://browser"} },
		newBrowser:         func(string) (sessionBrowser, error) { return browser, nil },
		loginPollInterval:  20 * time.Millisecond,
		interactiveTimeout: 500 * time.Millisecond,
	}
	err := session.EnsureLoggedIn(context.Background())
	if err == nil || !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
	if !strings.Contains(err.Error(), "current account identity could not be confirmed") {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
}

func TestBrowserSessionPersistsProfileMarkerOnSuccessfulLogin(t *testing.T) {
	tempDir := t.TempDir()
	page := &fakeSessionPage{url: xhsPublishURL, account: "writer"}
	browser := &fakeSessionBrowser{page: page}
	session := &rodBrowserSession{
		opts:          SessionOptions{Account: "writer", Headless: false},
		userConfigDir: func() (string, error) { return tempDir, nil },
		mkdirAll:      os.MkdirAll,
		readFile:      os.ReadFile,
		writeFile:     func(path string, data []byte, perm os.FileMode) error { return os.WriteFile(path, data, perm) },
		newLauncher:   func(SessionOptions, string) sessionLauncher { return fakeSessionLauncher{url: "ws://browser"} },
		newBrowser:    func(string) (sessionBrowser, error) { return browser, nil },
	}
	if err := session.EnsureLoggedIn(context.Background()); err != nil {
		t.Fatalf("EnsureLoggedIn() error = %v", err)
	}
	markerPath := filepath.Join(tempDir, "mark2note", "xhs", "profiles", "writer", profileStateFile)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", markerPath, err)
	}
	if !strings.Contains(string(data), "writer") {
		t.Fatalf("profile state = %s", string(data))
	}
}
