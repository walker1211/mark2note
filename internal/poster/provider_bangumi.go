package poster

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

type BangumiProvider struct {
	Client HTTPClient
}

func (p BangumiProvider) Name() string { return "bangumi" }

func (p BangumiProvider) Search(ctx context.Context, title string) ([]Candidate, error) {
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	searchURL := "https://api.bgm.tv/search/subject/" + url.PathEscape(title) + "?" + url.Values{"type": {"2"}, "responseGroup": {"small"}}.Encode()
	content, err := fetchText(ctx, client, searchURL)
	if err != nil {
		return nil, err
	}
	var decoded struct {
		List []struct {
			Name   string `json:"name"`
			NameCN string `json:"name_cn"`
			Images struct {
				Large  string `json:"large"`
				Common string `json:"common"`
				Medium string `json:"medium"`
				Grid   string `json:"grid"`
				Small  string `json:"small"`
			} `json:"images"`
		} `json:"list"`
	}
	if err := json.Unmarshal([]byte(content), &decoded); err != nil {
		return nil, err
	}
	for _, item := range decoded.List {
		if !titleMatches(title, item.NameCN, item.Name) {
			continue
		}
		image := normalizeBangumiImageURL(firstNonEmpty(item.Images.Large, item.Images.Common, item.Images.Medium, item.Images.Grid, item.Images.Small))
		if image == "" {
			continue
		}
		matched := item.NameCN
		if titleMatches(title, item.Name) {
			matched = item.Name
		}
		matched = firstNonEmpty(matched, item.NameCN, item.Name)
		return []Candidate{{Title: matched, ImageURL: image, Source: p.Name(), Confidence: "high", Reason: "Bangumi subject cover"}}, nil
	}
	return nil, nil
}

func normalizeBangumiImageURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if strings.HasPrefix(trimmed, "//") {
		return "https:" + trimmed
	}
	if rest, ok := strings.CutPrefix(trimmed, "http://"); ok {
		return "https://" + rest
	}
	return trimmed
}
