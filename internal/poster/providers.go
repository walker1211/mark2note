package poster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

const maxProviderResponseBytes = 4 * 1024 * 1024

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Candidate struct {
	Title      string
	ImageURL   string
	Source     string
	Confidence string
	Reason     string
}

type Provider interface {
	Name() string
	Search(ctx context.Context, title string) ([]Candidate, error)
}

type AniListProvider struct {
	Client HTTPClient
}

func (p AniListProvider) Name() string { return "anilist" }

func (p AniListProvider) Search(ctx context.Context, title string) ([]Candidate, error) {
	for _, mediaType := range []string{"ANIME", "MANGA"} {
		candidates, err := p.searchMedia(ctx, title, mediaType)
		if err != nil {
			return nil, err
		}
		if mediaType == "MANGA" || !hasEastAsianScript(title) {
			candidates = highConfidenceCandidates(candidates)
		}
		if len(candidates) > 0 {
			return candidates, nil
		}
	}
	return nil, nil
}

func (p AniListProvider) searchMedia(ctx context.Context, title string, mediaType string) ([]Candidate, error) {
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	payload := map[string]any{
		"query":     fmt.Sprintf(`query ($search:String){ Media(search:$search, type:%s){ title{romaji english native} coverImage{extraLarge large} } }`, mediaType),
		"variables": map[string]string{"search": title},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://graphql.anilist.co", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "mark2note/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("anilist status %d", resp.StatusCode)
	}
	var decoded struct {
		Data struct {
			Media struct {
				Title struct {
					Romaji  string `json:"romaji"`
					English string `json:"english"`
					Native  string `json:"native"`
				} `json:"title"`
				CoverImage struct {
					ExtraLarge string `json:"extraLarge"`
					Large      string `json:"large"`
				} `json:"coverImage"`
			} `json:"Media"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	image := firstNonEmpty(decoded.Data.Media.CoverImage.ExtraLarge, decoded.Data.Media.CoverImage.Large)
	if image == "" {
		return nil, nil
	}
	matched := firstNonEmpty(decoded.Data.Media.Title.English, decoded.Data.Media.Title.Romaji, decoded.Data.Media.Title.Native)
	confidence := "medium"
	if titleMatches(title, decoded.Data.Media.Title.Romaji, decoded.Data.Media.Title.English, decoded.Data.Media.Title.Native) {
		confidence = "high"
	}
	return []Candidate{{Title: matched, ImageURL: image, Source: p.Name(), Confidence: confidence, Reason: "AniList media cover"}}, nil
}

type MyDramaListProvider struct {
	Client HTTPClient
}

func (p MyDramaListProvider) Name() string { return "mydramalist" }

func (p MyDramaListProvider) Search(ctx context.Context, title string) ([]Candidate, error) {
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	searchURL := "https://mydramalist.com/search?q=" + url.QueryEscape(title)
	searchHTML, err := fetchText(ctx, client, searchURL)
	if err != nil {
		return nil, err
	}
	href, matched := firstMyDramaListResult(searchHTML)
	if href == "" || !titleMatches(title, matched) {
		return nil, nil
	}
	pageURL := "https://mydramalist.com" + href
	pageHTML, err := fetchText(ctx, client, pageURL)
	if err != nil {
		return nil, err
	}
	image := ogImage(pageHTML)
	if image == "" {
		return nil, nil
	}
	return []Candidate{{Title: matched, ImageURL: image, Source: p.Name(), Confidence: "high", Reason: "MyDramaList search result"}}, nil
}

func DefaultProviders(sources []string, client HTTPClient) []Provider {
	providers, _ := ProvidersForSources(sources, client)
	return providers
}

func ProvidersForSources(sources []string, client HTTPClient) ([]Provider, error) {
	if len(sources) == 0 {
		sources = []string{"bilibili", "bangumi", "anilist", "mydramalist"}
	}
	providers := make([]Provider, 0, len(sources))
	for _, source := range sources {
		name := strings.ToLower(strings.TrimSpace(source))
		switch name {
		case "", "none":
			continue
		case "bilibili":
			providers = append(providers, &BilibiliProvider{Client: client})
		case "bangumi":
			providers = append(providers, BangumiProvider{Client: client})
		case "anilist":
			providers = append(providers, AniListProvider{Client: client})
		case "mydramalist":
			providers = append(providers, MyDramaListProvider{Client: client})
		default:
			return nil, fmt.Errorf("unknown poster provider %q", source)
		}
	}
	return providers, nil
}

func fetchText(ctx context.Context, client HTTPClient, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 mark2note local")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s status %d", rawURL, resp.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderResponseBytes+1))
	if err != nil {
		return "", err
	}
	if len(content) > maxProviderResponseBytes {
		return "", fmt.Errorf("GET %s response exceeds %d bytes", rawURL, maxProviderResponseBytes)
	}
	return string(content), nil
}

var (
	mdlResultRe = regexp.MustCompile(`href="(/\d+[^"#?]+)"[^>]*>([^<]+)`)
	ogImageRe   = regexp.MustCompile(`<meta\s+(?:property="og:image"\s+content="([^"]+)"|content="([^"]+)"\s+property="og:image")`)
)

func firstMyDramaListResult(html string) (string, string) {
	match := mdlResultRe.FindStringSubmatch(html)
	if len(match) == 0 {
		return "", ""
	}
	return match[1], strings.TrimSpace(match[2])
}

func ogImage(html string) string {
	match := ogImageRe.FindStringSubmatch(html)
	if len(match) == 0 {
		return ""
	}
	return firstNonEmpty(match[1], match[2])
}

func titleMatches(input string, values ...string) bool {
	needle := normalizeTitle(input)
	for _, value := range values {
		if normalizeTitle(value) == needle {
			return true
		}
	}
	return false
}

func highConfidenceCandidates(candidates []Candidate) []Candidate {
	filtered := candidates[:0]
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Confidence) == "high" {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func hasEastAsianScript(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
	}
	return false
}
