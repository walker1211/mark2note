package xhs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

const (
	xhsPublishURL      = "https://creator.xiaohongshu.com/publish/publish?source=official"
	xhsImagePublishURL = "https://creator.xiaohongshu.com/publish/publish?from=menu&target=image"
	xhsLoginURL        = "https://creator.xiaohongshu.com/login"
	profileStateFile   = "profile-state.json"
	accountProbeScript = `() => {
		const selectors = [
			'[data-testid="user-name"]',
			'[data-testid="account-name"]',
			'.creator-name',
			'.creatorName',
			'.user-name',
			'.userName',
			'.user-nickname',
			'.userNickname',
			'.nickname'
		];
		const blocked = new Set(['创作服务平台', '小红书', '发布笔记', '草稿箱', '创作中心']);
		const normalize = (value) => {
			if (typeof value !== 'string') {
				return '';
			}
			const text = value.trim();
			if (!text || blocked.has(text)) {
				return '';
			}
			return text;
		};
		for (const selector of selectors) {
			const el = document.querySelector(selector);
			if (!el || !el.textContent) {
				continue;
			}
			const text = normalize(el.textContent);
			if (text) {
				return text;
			}
		}
		const readUserName = (storage) => {
			if (!storage) {
				return '';
			}
			const keys = ['USER_INFO_FOR_BIZ', 'USER_INFO', 'userInfo', 'user_info'];
			for (const key of keys) {
				const raw = storage.getItem(key);
				if (!raw) {
					continue;
				}
				try {
					const parsed = JSON.parse(raw);
					const candidates = [
						parsed && parsed.userName,
						parsed && parsed.nickname,
						parsed && parsed.user && parsed.user.userName,
						parsed && parsed.user && parsed.user.nickname,
						parsed && parsed.user && parsed.user.value && parsed.user.value.userName,
						parsed && parsed.user && parsed.user.value && parsed.user.value.nickname,
					];
					for (const candidate of candidates) {
						const text = normalize(candidate);
						if (text) {
							return text;
						}
					}
				} catch (e) {
					continue;
				}
			}
			return '';
		};
		return readUserName(window.localStorage) || readUserName(window.sessionStorage) || '';
	}`
	loginPromptScript = `() => {
		const text = document.body ? document.body.innerText : '';
		return /扫码登录|登录后|手机号登录|立即登录/.test(text);
	}`
)

var (
	ErrBrowserLaunch   = errors.New("browser launch failed")
	ErrNotLoggedIn     = errors.New("not logged in to Xiaohongshu creator center")
	ErrAccountMismatch = errors.New("xiaohongshu account mismatch")
)

type SessionOptions struct {
	Account    string
	ChromePath string
	Headless   bool
	ProfileDir string
}

type BrowserSession interface {
	Open(ctx context.Context) error
	EnsureLoggedIn(ctx context.Context) error
	Close() error
	PublisherPage(ctx context.Context) (PublishPage, error)
}

type rodBrowserSession struct {
	opts               SessionOptions
	userConfigDir      func() (string, error)
	mkdirAll           func(string, os.FileMode) error
	readFile           func(string) ([]byte, error)
	writeFile          func(string, []byte, os.FileMode) error
	newLauncher        func(SessionOptions, string) sessionLauncher
	newBrowser         func(string) (sessionBrowser, error)
	logf               func(string, ...any)
	loginPollInterval  time.Duration
	loginGracePeriod   time.Duration
	interactiveTimeout time.Duration
	profileDir         string
	browser            sessionBrowser
	page               sessionPage
	ownsBrowser        bool
}

type sessionLauncher interface {
	Launch() (string, error)
}

type sessionBrowser interface {
	Page(string) (sessionPage, error)
	Close() error
}

type sessionPage interface {
	PublishPage
	Navigate(string) error
	URL() (string, error)
	AccountName() (string, error)
	HasLoginPrompt() (bool, error)
}

type profileState struct {
	Account string `json:"account"`
}

type loginStatus struct {
	url            string
	accountName    string
	hasLoginPrompt bool
	onLoginPage    bool
	loggedIn       bool
}

func NewBrowserSession(opts SessionOptions) BrowserSession {
	return &rodBrowserSession{
		opts:          opts,
		userConfigDir: os.UserConfigDir,
		mkdirAll:      os.MkdirAll,
		readFile:      os.ReadFile,
		writeFile: func(path string, data []byte, perm os.FileMode) error {
			return os.WriteFile(path, data, perm)
		},
		newLauncher:        defaultSessionLauncher,
		newBrowser:         defaultSessionBrowser,
		logf:               defaultXHSLogger,
		loginPollInterval:  2 * time.Second,
		loginGracePeriod:   5 * time.Second,
		interactiveTimeout: 2 * time.Minute,
	}
}

func (s *rodBrowserSession) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.browser != nil {
		s.debugf("reuse browser session")
		return nil
	}
	profileDir, err := resolveSessionProfileDir(s.userConfigDir, s.opts.Account, s.opts.ProfileDir)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBrowserLaunch, err)
	}
	s.debugf("open browser session account=%s profile=%s headless=%t", strings.TrimSpace(s.opts.Account), profileDir, s.opts.Headless)
	if err := s.mkdirAll(profileDir, 0o755); err != nil {
		return fmt.Errorf("%w: create profile dir: %v", ErrBrowserLaunch, err)
	}
	if controlURL, ok, err := readRunningBrowserControlURL(s.readFile, profileDir); err != nil {
		s.debugf("read running browser control url failed: %v", err)
	} else if ok {
		s.debugf("reuse running browser control url=%s", controlURL)
		browser, connectErr := s.newBrowser(controlURL)
		if connectErr == nil {
			s.debugf("open publish page url=%s", xhsPublishURL)
			page, pageErr := browser.Page(xhsPublishURL)
			if pageErr == nil {
				s.profileDir = profileDir
				s.browser = browser
				s.page = page
				s.ownsBrowser = false
				s.debugf("browser session ready reused=true")
				return nil
			}
			s.debugf("running browser page open failed: %v", pageErr)
		} else {
			s.debugf("running browser connect failed: %v", connectErr)
		}
	}
	launched, err := s.newLauncher(s.opts, profileDir).Launch()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBrowserLaunch, err)
	}
	s.debugf("launcher connected url=%s", launched)
	browser, err := s.newBrowser(launched)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBrowserLaunch, err)
	}
	s.debugf("open publish page url=%s", xhsPublishURL)
	page, err := browser.Page(xhsPublishURL)
	if err != nil {
		_ = browser.Close()
		return fmt.Errorf("%w: open publish page: %v", ErrBrowserLaunch, err)
	}
	s.profileDir = profileDir
	s.browser = browser
	s.page = page
	s.ownsBrowser = true
	s.debugf("browser session ready reused=false")
	return nil
}

func (s *rodBrowserSession) EnsureLoggedIn(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.debugf("ensure login start")
	if err := s.Open(ctx); err != nil {
		return err
	}
	status, err := s.inspectLoginStatus(ctx)
	if err != nil {
		return err
	}
	if status.loggedIn {
		return s.confirmLoggedIn(status.accountName)
	}
	return s.waitForInteractiveLogin(ctx, status)
}

func (s *rodBrowserSession) inspectLoginStatus(ctx context.Context) (loginStatus, error) {
	if err := ctx.Err(); err != nil {
		return loginStatus{}, err
	}
	currentURL, err := s.page.URL()
	if err != nil {
		return loginStatus{}, fmt.Errorf("%w: inspect page url: %v", ErrNotLoggedIn, err)
	}
	status := loginStatus{url: strings.TrimSpace(currentURL)}
	s.debugf("current url=%s", status.url)
	if looksLikeLoginURL(status.url) {
		status.onLoginPage = true
		s.debugf("detected login url")
		return status, nil
	}
	accountName, err := s.page.AccountName()
	if err != nil {
		return loginStatus{}, fmt.Errorf("%w: inspect account identity: %v", ErrNotLoggedIn, err)
	}
	status.accountName = sanitizeAccountName(accountName)
	s.debugf("account probe result=%q", status.accountName)
	if status.accountName != "" {
		status.loggedIn = true
		return status, nil
	}
	hasPrompt, promptErr := s.page.HasLoginPrompt()
	if promptErr != nil {
		return loginStatus{}, fmt.Errorf("%w: inspect login prompt: %v", ErrNotLoggedIn, promptErr)
	}
	status.hasLoginPrompt = hasPrompt
	s.debugf("login prompt visible=%t", hasPrompt)
	return status, nil
}

func (s *rodBrowserSession) waitForInteractiveLogin(ctx context.Context, status loginStatus) error {
	s.debugf("handle unauthenticated state headless=%t loginPage=%t prompt=%t", s.opts.Headless, status.onLoginPage, status.hasLoginPrompt)
	if s.opts.Headless {
		return fmt.Errorf("%w: rerun without --headless to complete QR login", ErrNotLoggedIn)
	}
	if s.page != nil && !status.onLoginPage {
		s.debugf("navigate to login url=%s", xhsLoginURL)
		if err := s.page.Navigate(xhsLoginURL); err != nil {
			return fmt.Errorf("%w: navigate to login page: %v", ErrNotLoggedIn, err)
		}
	}
	gracePeriod := s.loginGracePeriod
	if gracePeriod <= 0 {
		gracePeriod = 5 * time.Second
	}
	deadline := time.Now().Add(gracePeriod)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		refreshed, err := s.inspectLoginStatus(ctx)
		if err != nil {
			return err
		}
		if refreshed.loggedIn {
			return s.confirmLoggedIn(refreshed.accountName)
		}
		if refreshed.onLoginPage || refreshed.hasLoginPrompt {
			status = refreshed
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !status.onLoginPage && !status.hasLoginPrompt {
		return fmt.Errorf("%w: login state could not be confirmed on %s; creator center may have changed its page structure", ErrNotLoggedIn, status.url)
	}
	return s.waitForLoginCompletion(ctx)
}

func (s *rodBrowserSession) waitForLoginCompletion(ctx context.Context) error {
	pollInterval := s.loginPollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	interactiveTimeout := s.interactiveTimeout
	if interactiveTimeout <= 0 {
		interactiveTimeout = 2 * time.Minute
	}
	s.debugf("waiting for QR login completion timeout=%s interval=%s", interactiveTimeout, pollInterval)
	fmt.Fprintf(os.Stderr, "请在打开的浏览器中完成小红书扫码登录，登录成功后命令会自动继续...\n")
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	timer := time.NewTimer(interactiveTimeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("%w: timed out waiting for QR login to complete", ErrNotLoggedIn)
		case <-ticker.C:
			status, err := s.inspectLoginStatus(ctx)
			if err != nil {
				return err
			}
			if status.loggedIn {
				return s.confirmLoggedIn(status.accountName)
			}
			if looksLikeInteractiveLoginSuccess(status) {
				resolved, ok, err := s.confirmInteractiveLoginAfterRedirect(ctx, status)
				if err != nil {
					return err
				}
				if ok {
					return s.confirmLoggedIn(resolved)
				}
			}
		}
	}
}

func (s *rodBrowserSession) confirmLoggedIn(accountName string) error {
	accountName = sanitizeAccountName(accountName)
	requestedAccount := sanitizeAccountName(s.opts.Account)
	if !accountNamesMatch(accountName, requestedAccount) {
		s.debugf("account mismatch active=%q requested=%q", accountName, requestedAccount)
		return fmt.Errorf("%w: active account %q does not match requested account %q", ErrAccountMismatch, accountName, requestedAccount)
	}
	stored, ok, err := s.readProfileState()
	if err != nil {
		return fmt.Errorf("%w: read profile state: %v", ErrAccountMismatch, err)
	}
	if ok && !accountNamesMatch(stored.Account, accountName) {
		s.debugf("profile marker mismatch marker=%q active=%q", stored.Account, accountName)
		return fmt.Errorf("%w: profile marker %q does not match active account %q", ErrAccountMismatch, stored.Account, accountName)
	}
	if err := s.writeProfileState(profileState{Account: accountName}); err != nil {
		return fmt.Errorf("%w: write profile state: %v", ErrBrowserLaunch, err)
	}
	s.debugf("ensure login success account=%q", accountName)
	return nil
}

func (s *rodBrowserSession) PublisherPage(ctx context.Context) (PublishPage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.Open(ctx); err != nil {
		return nil, err
	}
	return s.page, nil
}

func (s *rodBrowserSession) Close() error {
	if s.browser == nil {
		return nil
	}
	if !s.ownsBrowser {
		s.browser = nil
		s.page = nil
		s.ownsBrowser = false
		return nil
	}
	err := s.browser.Close()
	s.browser = nil
	s.page = nil
	s.ownsBrowser = false
	return err
}

func (s *rodBrowserSession) readProfileState() (profileState, bool, error) {
	if strings.TrimSpace(s.profileDir) == "" {
		return profileState{}, false, nil
	}
	data, err := s.readFile(filepath.Join(s.profileDir, profileStateFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return profileState{}, false, nil
		}
		return profileState{}, false, err
	}
	var state profileState
	if err := json.Unmarshal(data, &state); err != nil {
		return profileState{}, false, err
	}
	return state, true, nil
}

func (s *rodBrowserSession) writeProfileState(state profileState) error {
	if strings.TrimSpace(s.profileDir) == "" {
		return nil
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return s.writeFile(filepath.Join(s.profileDir, profileStateFile), data, 0o644)
}

func sanitizeAccountName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	switch trimmed {
	case "创作服务平台", "小红书", "发布笔记", "草稿箱", "创作中心":
		return ""
	default:
		return trimmed
	}
}

func accountNamesMatch(active string, requested string) bool {
	return strings.EqualFold(strings.TrimSpace(active), strings.TrimSpace(requested))
}

func looksLikeInteractiveLoginSuccess(status loginStatus) bool {
	if status.loggedIn || status.onLoginPage || status.hasLoginPrompt {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(status.url))
	if err != nil {
		return false
	}
	if !strings.EqualFold(parsed.Host, "creator.xiaohongshu.com") {
		return false
	}
	return parsed.Path == "/new/home" || strings.HasPrefix(parsed.Path, "/creator/home")
}

func (s *rodBrowserSession) confirmInteractiveLoginAfterRedirect(ctx context.Context, status loginStatus) (string, bool, error) {
	s.debugf("interactive login redirected to creator home url=%s; retrying account probe", status.url)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return "", false, err
		}
		refreshed, err := s.inspectLoginStatus(ctx)
		if err != nil {
			return "", false, err
		}
		if refreshed.loggedIn {
			return refreshed.accountName, true, nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	stored, ok, err := s.readProfileState()
	if err != nil {
		return "", false, fmt.Errorf("%w: read profile state: %v", ErrAccountMismatch, err)
	}
	if ok && accountNamesMatch(stored.Account, s.opts.Account) {
		s.debugf("profile marker matches requested account=%q but current session account is still unconfirmed", strings.TrimSpace(stored.Account))
	}
	return "", false, fmt.Errorf("%w: login redirect detected on %s but current account identity could not be confirmed", ErrNotLoggedIn, status.url)
}

func readRunningBrowserControlURL(readFile func(string) ([]byte, error), profileDir string) (string, bool, error) {
	if strings.TrimSpace(profileDir) == "" {
		return "", false, nil
	}
	data, err := readFile(filepath.Join(profileDir, "DevToolsActivePort"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return "", false, nil
	}
	port := strings.TrimSpace(lines[0])
	path := strings.TrimSpace(lines[1])
	if port == "" || path == "" {
		return "", false, nil
	}
	if !strings.HasPrefix(path, "/devtools/browser/") {
		return "", false, nil
	}
	return fmt.Sprintf("ws://127.0.0.1:%s%s", port, path), true, nil
}

func resolveSessionProfileDir(userConfigDir func() (string, error), account string, explicit string) (string, error) {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return trimmed, nil
	}
	base, err := userConfigDir()
	if err != nil {
		return "", err
	}
	trimmedAccount := strings.TrimSpace(account)
	if trimmedAccount == "" {
		return "", fmt.Errorf("account is required")
	}
	if strings.Contains(trimmedAccount, "..") || strings.ContainsAny(trimmedAccount, `/\\`) {
		return "", fmt.Errorf("invalid account name: %q", trimmedAccount)
	}
	return filepath.Join(base, "mark2note", "xhs", "profiles", trimmedAccount), nil
}

func defaultXHSLogger(format string, args ...any) {
	if os.Getenv("MARK2NOTE_XHS_DEBUG") != "1" {
		return
	}
	fmt.Fprintf(os.Stderr, "[xhs-debug] "+format+"\n", args...)
}

func (s *rodBrowserSession) debugf(format string, args ...any) {
	if s.logf != nil {
		s.logf(format, args...)
	}
}

func looksLikeLoginURL(url string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(trimmed, "/login") || strings.Contains(trimmed, "login.xiaohongshu.com")
}

func defaultSessionLauncher(opts SessionOptions, profileDir string) sessionLauncher {
	instance := launcher.New().UserDataDir(profileDir).Headless(opts.Headless)
	if trimmed := strings.TrimSpace(opts.ChromePath); trimmed != "" {
		instance = instance.Bin(trimmed)
	}
	return &rodLauncher{launcher: instance}
}

func defaultSessionBrowser(controlURL string) (sessionBrowser, error) {
	var browser *rod.Browser
	if err := rodTry(func() {
		browser = rod.New().ControlURL(controlURL).MustConnect()
	}); err != nil {
		return nil, err
	}
	return &rodBrowser{browser: browser}, nil
}

type rodLauncher struct {
	launcher *launcher.Launcher
}

func (l *rodLauncher) Launch() (url string, err error) {
	err = rodTry(func() {
		url = l.launcher.MustLaunch()
	})
	return url, err
}

type rodBrowser struct {
	browser *rod.Browser
}

func (b *rodBrowser) Page(url string) (sessionPage, error) {
	var page *rod.Page
	if err := rodTry(func() {
		page = b.browser.MustPage(url)
	}); err != nil {
		return nil, err
	}
	if err := waitForRodPageReady(page); err != nil {
		return nil, err
	}
	return &rodPage{page: page}, nil
}

func (b *rodBrowser) Close() error {
	return rodTry(func() {
		b.browser.MustClose()
	})
}

type rodPage struct {
	page *rod.Page
}

func (p *rodPage) Navigate(url string) error {
	if err := rodTry(func() {
		p.page.MustNavigate(url)
	}); err != nil {
		return err
	}
	return waitForRodPageReady(p.page)
}

func (p *rodPage) URL() (result string, err error) {
	err = rodTry(func() {
		result = p.page.MustEval(`() => location.href`).String()
	})
	return result, err
}

func (p *rodPage) AccountName() (result string, err error) {
	err = rodTry(func() {
		result = strings.TrimSpace(p.page.MustEval(accountProbeScript).String())
	})
	return result, err
}

func (p *rodPage) HasLoginPrompt() (result bool, err error) {
	err = rodTry(func() {
		result = p.page.MustEval(loginPromptScript).Bool()
	})
	return result, err
}

func waitForRodPageReady(page *rod.Page) error {
	if err := rodTry(func() {
		page.Timeout(15 * time.Second).MustWaitLoad()
	}); err != nil {
		return err
	}
	return rodTry(func() {
		page.Timeout(5 * time.Second).MustElement("body")
	})
}

func rodTry(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	fn()
	return nil
}
