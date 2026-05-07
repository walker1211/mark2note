package poster

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	bilibiliFingerprintURL = "https://api.bilibili.com/x/frontend/finger/spi"
	bilibiliNavURL         = "https://api.bilibili.com/x/web-interface/nav"
	bilibiliSearchURL      = "https://api.bilibili.com/x/web-interface/wbi/search/type"
)

type BilibiliProvider struct {
	Client HTTPClient

	mu      sync.Mutex
	cookies []string
	imgKey  string
	subKey  string
}

type bilibiliSearchItem struct {
	Title      string   `json:"title"`
	OrgTitle   string   `json:"org_title"`
	Cover      string   `json:"cover"`
	HitColumns []string `json:"hit_columns"`
}

func (p *BilibiliProvider) Name() string { return "bilibili" }

func (p *BilibiliProvider) Search(ctx context.Context, title string) ([]Candidate, error) {
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	cookies, err := p.bilibiliCookies(ctx, client)
	if err != nil {
		return nil, err
	}
	imgKey, subKey, err := p.bilibiliWBIKeys(ctx, client, cookies)
	if err != nil {
		return nil, err
	}
	searchURL, err := bilibiliSignedSearchURL(title, imgKey, subKey, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		content, err := p.fetchSearchText(ctx, client, searchURL, cookies)
		if err != nil {
			var statusErr bilibiliStatusError
			if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusPreconditionFailed {
				return nil, err
			}
			cookies, err = p.refreshBilibiliCookies(ctx, client)
			if err != nil {
				return nil, err
			}
			imgKey, subKey, err = p.refreshBilibiliWBIKeys(ctx, client, cookies)
			if err != nil {
				return nil, err
			}
			searchURL, err = bilibiliSignedSearchURL(title, imgKey, subKey, time.Now().Unix())
			if err != nil {
				return nil, err
			}
			content, err = p.fetchSearchText(ctx, client, searchURL, cookies)
			if err != nil {
				return nil, err
			}
		}
		candidates, err := bilibiliCandidatesFromSearch(title, content, p.Name())
		if err != nil {
			return nil, err
		}
		if len(candidates) > 0 {
			return candidates, nil
		}
	}
	return nil, nil
}

func bilibiliCandidatesFromSearch(title string, content string, source string) ([]Candidate, error) {
	var decoded struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Result []bilibiliSearchItem `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(content), &decoded); err != nil {
		return nil, err
	}
	if decoded.Code != 0 {
		return nil, fmt.Errorf("bilibili code %d: %s", decoded.Code, strings.TrimSpace(decoded.Message))
	}
	var best Candidate
	bestScore := 0
	for _, item := range decoded.Data.Result {
		image := normalizeBilibiliImageURL(item.Cover)
		if image == "" {
			continue
		}
		confidence, score, ok := bilibiliMatchConfidence(title, item)
		if !ok || score <= bestScore {
			continue
		}
		bestScore = score
		best = Candidate{Title: cleanBilibiliTitle(firstNonEmpty(item.Title, item.OrgTitle)), ImageURL: image, Source: source, Confidence: confidence, Reason: "Bilibili bangumi cover"}
	}
	if bestScore == 0 {
		return nil, nil
	}
	return []Candidate{best}, nil
}

func (p *BilibiliProvider) fetchSearchText(ctx context.Context, client HTTPClient, rawURL string, cookies []string) (string, error) {
	req, err := newBilibiliRequest(ctx, rawURL, cookies)
	if err != nil {
		return "", err
	}
	return fetchBilibiliText(client, req)
}

func (p *BilibiliProvider) bilibiliWBIKeys(ctx context.Context, client HTTPClient, cookies []string) (string, string, error) {
	p.mu.Lock()
	if p.imgKey != "" && p.subKey != "" {
		imgKey, subKey := p.imgKey, p.subKey
		p.mu.Unlock()
		return imgKey, subKey, nil
	}
	p.mu.Unlock()
	return p.refreshBilibiliWBIKeys(ctx, client, cookies)
}

func (p *BilibiliProvider) refreshBilibiliWBIKeys(ctx context.Context, client HTTPClient, cookies []string) (string, string, error) {
	req, err := newBilibiliRequest(ctx, bilibiliNavURL, cookies)
	if err != nil {
		return "", "", err
	}
	content, err := fetchBilibiliText(client, req)
	if err != nil {
		return "", "", err
	}
	var decoded struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			WBIImg struct {
				ImgURL string `json:"img_url"`
				SubURL string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(content), &decoded); err != nil {
		return "", "", err
	}
	imgKey := bilibiliImageKey(decoded.Data.WBIImg.ImgURL)
	subKey := bilibiliImageKey(decoded.Data.WBIImg.SubURL)
	if imgKey == "" || subKey == "" {
		if decoded.Code != 0 {
			return "", "", fmt.Errorf("bilibili nav code %d: %s", decoded.Code, strings.TrimSpace(decoded.Message))
		}
		return "", "", fmt.Errorf("bilibili nav missing wbi keys")
	}
	p.mu.Lock()
	p.imgKey, p.subKey = imgKey, subKey
	p.mu.Unlock()
	return imgKey, subKey, nil
}

func (p *BilibiliProvider) bilibiliCookies(ctx context.Context, client HTTPClient) ([]string, error) {
	p.mu.Lock()
	if len(p.cookies) > 0 {
		cookies := append([]string(nil), p.cookies...)
		p.mu.Unlock()
		return cookies, nil
	}
	p.mu.Unlock()
	return p.refreshBilibiliCookies(ctx, client)
}

func (p *BilibiliProvider) refreshBilibiliCookies(ctx context.Context, client HTTPClient) ([]string, error) {
	req, err := newBilibiliRequest(ctx, bilibiliFingerprintURL, nil)
	if err != nil {
		return nil, err
	}
	content, err := fetchBilibiliText(client, req)
	if err != nil {
		return nil, err
	}
	var decoded struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			B3 string `json:"b_3"`
			B4 string `json:"b_4"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(content), &decoded); err != nil {
		return nil, err
	}
	if decoded.Code != 0 {
		return nil, fmt.Errorf("bilibili fingerprint code %d: %s", decoded.Code, strings.TrimSpace(decoded.Message))
	}
	var cookies []string
	if decoded.Data.B3 != "" {
		cookies = append(cookies, (&http.Cookie{Name: "buvid3", Value: decoded.Data.B3}).String())
	}
	if decoded.Data.B4 != "" {
		cookies = append(cookies, (&http.Cookie{Name: "buvid4", Value: decoded.Data.B4}).String())
	}

	p.mu.Lock()
	p.cookies = append([]string(nil), cookies...)
	p.mu.Unlock()
	return cookies, nil
}

func newBilibiliRequest(ctx context.Context, rawURL string, cookies []string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	if len(cookies) > 0 {
		req.Header.Set("Cookie", strings.Join(cookies, "; "))
	}
	return req, nil
}

type bilibiliStatusError struct {
	URL        string
	StatusCode int
}

func (e bilibiliStatusError) Error() string {
	return fmt.Sprintf("GET %s status %d", e.URL, e.StatusCode)
}

func fetchBilibiliText(client HTTPClient, req *http.Request) (string, error) {
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", bilibiliStatusError{URL: req.URL.String(), StatusCode: resp.StatusCode}
	}
	content, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderResponseBytes+1))
	if err != nil {
		return "", err
	}
	if len(content) > maxProviderResponseBytes {
		return "", fmt.Errorf("GET %s response exceeds %d bytes", req.URL.String(), maxProviderResponseBytes)
	}
	return string(content), nil
}

var bilibiliHTMLTagRe = regexp.MustCompile(`<[^>]+>`)

var bilibiliMixinKeyEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}

func bilibiliSignedSearchURL(title string, imgKey string, subKey string, timestamp int64) (string, error) {
	searchURL, err := url.Parse(bilibiliSearchURL)
	if err != nil {
		return "", err
	}
	params := url.Values{}
	params.Set("keyword", title)
	params.Set("page", "1")
	params.Set("search_type", "media_bangumi")
	searchURL.RawQuery = signBilibiliParams(params, imgKey, subKey, timestamp).Encode()
	return searchURL.String(), nil
}

func signBilibiliParams(params url.Values, imgKey string, subKey string, timestamp int64) url.Values {
	signed := url.Values{}
	for key, values := range params {
		for _, value := range values {
			signed.Add(key, filterBilibiliWBIValue(value))
		}
	}
	signed.Set("wts", strconv.FormatInt(timestamp, 10))
	query := sortedBilibiliQuery(signed)
	sum := md5.Sum([]byte(query + bilibiliMixinKey(imgKey, subKey)))
	signed.Set("w_rid", hex.EncodeToString(sum[:]))
	return signed
}

func sortedBilibiliQuery(params url.Values) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		values := append([]string(nil), params[key]...)
		sort.Strings(values)
		for _, value := range values {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
	}
	return strings.Join(parts, "&")
}

func bilibiliMixinKey(imgKey string, subKey string) string {
	orig := imgKey + subKey
	var b strings.Builder
	for _, idx := range bilibiliMixinKeyEncTab {
		if idx < len(orig) {
			b.WriteByte(orig[idx])
		}
	}
	key := b.String()
	if len(key) > 32 {
		return key[:32]
	}
	return key
}

func filterBilibiliWBIValue(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '!', '\'', '(', ')', '*':
			return -1
		default:
			return r
		}
	}, value)
}

func bilibiliImageKey(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	base := path.Base(parsed.Path)
	return strings.TrimSuffix(base, path.Ext(base))
}

func cleanBilibiliTitle(title string) string {
	clean := bilibiliHTMLTagRe.ReplaceAllString(title, "")
	return strings.TrimSpace(html.UnescapeString(clean))
}

func bilibiliMatchConfidence(input string, item bilibiliSearchItem) (string, int, bool) {
	matched := cleanBilibiliTitle(firstNonEmpty(item.Title, item.OrgTitle))
	orgTitle := cleanBilibiliTitle(item.OrgTitle)
	if titleMatches(input, matched, orgTitle) {
		return "high", 100, true
	}
	needle := normalizeTitle(input)
	matchedKey := normalizeTitle(matched)
	if needle != "" && matchedKey != "" && strings.Contains(matchedKey, needle) {
		return "medium", 80, true
	}
	if titleHasBilibiliHighlight(item.Title) {
		return "medium", 70, true
	}
	if hasBilibiliTitleHit(item.HitColumns) {
		return "medium", 60, true
	}
	return "", 0, false
}

func titleHasBilibiliHighlight(title string) bool {
	return strings.Contains(strings.ToLower(title), "<em")
}

func hasBilibiliTitleHit(columns []string) bool {
	for _, column := range columns {
		switch strings.TrimSpace(column) {
		case "title", "org_title":
			return true
		}
	}
	return false
}

func normalizeBilibiliImageURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if strings.HasPrefix(trimmed, "//") {
		return "https:" + trimmed
	}
	if rest, ok := strings.CutPrefix(trimmed, "http://"); ok {
		return "https://" + rest
	}
	return trimmed
}
