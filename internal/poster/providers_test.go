package poster

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeHTTPClient struct {
	do func(*http.Request) (*http.Response, error)
}

func (c fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.do(req)
}

func textResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func TestAniListProviderSearchReturnsCoverCandidate(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", req.Method)
		}
		if req.URL.String() != "https://graphql.anilist.co" {
			t.Fatalf("url = %s, want AniList GraphQL endpoint", req.URL.String())
		}
		return textResponse(200, `{"data":{"Media":{"title":{"romaji":"Death Note","english":"Death Note","native":"死亡笔记"},"coverImage":{"extraLarge":"https://img.example/death-note-large.jpg","large":"https://img.example/death-note.jpg"}}}}`), nil
	}}

	candidates, err := AniListProvider{Client: client}.Search(context.Background(), "死亡笔记")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %#v, want one", candidates)
	}
	got := candidates[0]
	if got.ImageURL != "https://img.example/death-note-large.jpg" || got.Source != "anilist" || got.Confidence != "high" {
		t.Fatalf("candidate = %#v", got)
	}
}

func TestAniListProviderSearchFallsBackToLargeCover(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		return textResponse(200, `{"data":{"Media":{"title":{"romaji":"Usogui","english":"","native":"噬谎者"},"coverImage":{"extraLarge":"","large":"https://img.example/usogui.jpg"}}}}`), nil
	}}

	candidates, err := AniListProvider{Client: client}.Search(context.Background(), "噬谎者")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].ImageURL != "https://img.example/usogui.jpg" {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestAniListProviderSearchReturnsStatusError(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		return textResponse(429, `rate limited`), nil
	}}

	_, err := AniListProvider{Client: client}.Search(context.Background(), "死亡笔记")
	if err == nil || !strings.Contains(err.Error(), "anilist status 429") {
		t.Fatalf("Search() error = %v, want status error", err)
	}
}

func TestMyDramaListProviderSearchReturnsOGImageCandidate(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://mydramalist.com/search?q=Society+Game":
			return textResponse(200, `<a class="block" href="/123-society-game">Society Game</a>`), nil
		case "https://mydramalist.com/123-society-game":
			return textResponse(200, `<html><head><meta content="https://img.example/society-game.jpg" property="og:image"></head></html>`), nil
		default:
			t.Fatalf("unexpected url: %s", req.URL.String())
			return nil, nil
		}
	}}

	candidates, err := MyDramaListProvider{Client: client}.Search(context.Background(), "Society Game")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %#v, want one", candidates)
	}
	got := candidates[0]
	if got.Title != "Society Game" || got.ImageURL != "https://img.example/society-game.jpg" || got.Source != "mydramalist" || got.Confidence != "high" {
		t.Fatalf("candidate = %#v", got)
	}
}

func TestProvidersForSourcesRejectsUnknownSource(t *testing.T) {
	_, err := ProvidersForSources([]string{"anlist", "mydramalist"}, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown poster provider") {
		t.Fatalf("ProvidersForSources() error = %v, want unknown provider", err)
	}
}
