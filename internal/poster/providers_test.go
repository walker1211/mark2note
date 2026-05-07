package poster

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

const fakeBilibiliNavResponse = `{"code":-101,"message":"账号未登录","data":{"wbi_img":{"img_url":"https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png","sub_url":"https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png"}}}`

func requireBilibiliSearchRequest(t *testing.T, req *http.Request, keyword string) {
	t.Helper()
	if req.URL.Scheme != "https" || req.URL.Host != "api.bilibili.com" || req.URL.Path != "/x/web-interface/wbi/search/type" {
		t.Fatalf("search url = %s", req.URL.String())
	}
	query := req.URL.Query()
	if query.Get("keyword") != keyword || query.Get("search_type") != "media_bangumi" || query.Get("page") != "1" {
		t.Fatalf("search query = %s", req.URL.RawQuery)
	}
	if query.Get("wts") == "" || query.Get("w_rid") == "" {
		t.Fatalf("search query missing WBI fields: %s", req.URL.RawQuery)
	}
}

func requireBangumiSearchRequest(t *testing.T, req *http.Request, keyword string) {
	t.Helper()
	if req.URL.Scheme != "https" || req.URL.Host != "api.bgm.tv" || req.URL.EscapedPath() != "/search/subject/"+url.PathEscape(keyword) {
		t.Fatalf("search url = %s", req.URL.String())
	}
	query := req.URL.Query()
	if query.Get("type") != "2" || query.Get("responseGroup") != "small" {
		t.Fatalf("search query = %s", req.URL.RawQuery)
	}
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

func TestAniListProviderSearchFallsBackFromAnimeToManga(t *testing.T) {
	var queries []string
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		query := string(body)
		queries = append(queries, query)
		if strings.Contains(query, "type:ANIME") {
			return textResponse(404, `not found`), nil
		}
		if strings.Contains(query, "type:MANGA") {
			return textResponse(200, `{"data":{"Media":{"title":{"romaji":"Usogui","english":"","native":"噬谎者"},"coverImage":{"extraLarge":"https://img.example/usogui-large.jpg","large":"https://img.example/usogui.jpg"}}}}`), nil
		}
		t.Fatalf("query does not include expected media type: %s", query)
		return nil, nil
	}}

	candidates, err := AniListProvider{Client: client}.Search(context.Background(), "噬谎者")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].ImageURL != "https://img.example/usogui-large.jpg" {
		t.Fatalf("candidates = %#v", candidates)
	}
	if len(queries) != 2 || !strings.Contains(queries[0], "type:ANIME") || !strings.Contains(queries[1], "type:MANGA") {
		t.Fatalf("queries = %#v, want ANIME then MANGA", queries)
	}
}

func TestAniListProviderSearchSkipsMangaFallbackWhenTitleDoesNotMatch(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		query := string(body)
		if strings.Contains(query, "type:ANIME") {
			return textResponse(404, `not found`), nil
		}
		if strings.Contains(query, "type:MANGA") {
			return textResponse(200, `{"data":{"Media":{"title":{"romaji":"Wrong Manga","english":"","native":"别的作品"},"coverImage":{"extraLarge":"https://img.example/wrong.jpg","large":""}}}}`), nil
		}
		t.Fatalf("query does not include expected media type: %s", query)
		return nil, nil
	}}

	candidates, err := AniListProvider{Client: client}.Search(context.Background(), "尸鬼")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %#v, want none", candidates)
	}
}

func TestAniListProviderSearchSkipsAsciiAnimeWhenTitleDoesNotMatch(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		query := string(body)
		if strings.Contains(query, "type:ANIME") {
			return textResponse(200, `{"data":{"Media":{"title":{"romaji":"Golden Kamuy 2nd Season OVA","english":"","native":"ゴールデンカムイ"},"coverImage":{"extraLarge":"https://img.example/wrong.jpg","large":""}}}}`), nil
		}
		if strings.Contains(query, "type:MANGA") {
			return textResponse(404, `not found`), nil
		}
		t.Fatalf("query does not include expected media type: %s", query)
		return nil, nil
	}}

	candidates, err := AniListProvider{Client: client}.Search(context.Background(), "MONSTER")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %#v, want none", candidates)
	}
}

func TestSignBilibiliParamsAddsWBIFields(t *testing.T) {
	params := url.Values{}
	params.Set("keyword", "冰果")
	params.Set("search_type", "media_bangumi")

	signed := signBilibiliParams(params, "7cd084941338484aae1ad9425b84077c", "4932caff0ff746eab6f01bf08b70ac45", 1700000000)

	if got := signed.Encode(); got != "keyword=%E5%86%B0%E6%9E%9C&search_type=media_bangumi&w_rid=5516272908a045cccd79f45c8e2764c1&wts=1700000000" {
		t.Fatalf("signed query = %q", got)
	}
	if params.Get("w_rid") != "" || params.Get("wts") != "" {
		t.Fatalf("original params mutated: %s", params.Encode())
	}
}

func TestBilibiliProviderSearchReturnsBangumiCoverCandidate(t *testing.T) {
	var sawSearch bool
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.String() == "https://api.bilibili.com/x/frontend/finger/spi":
			return textResponse(200, `{"code":0,"message":"0","data":{"b_3":"BUVID3","b_4":"BUVID4"}}`), nil
		case req.URL.String() == "https://api.bilibili.com/x/web-interface/nav":
			return textResponse(200, fakeBilibiliNavResponse), nil
		case req.URL.Path == "/x/web-interface/wbi/search/type":
			sawSearch = true
			requireBilibiliSearchRequest(t, req, "侦探学园Q")
			cookie := req.Header.Get("Cookie")
			if !strings.Contains(cookie, "buvid3=BUVID3") || !strings.Contains(cookie, "buvid4=BUVID4") {
				t.Fatalf("Cookie = %q, want buvid3 and buvid4", cookie)
			}
			return textResponse(200, `{"code":0,"message":"0","data":{"result":[{"title":"<em class=\"keyword\">侦探学园Q</em>","org_title":"探偵学園Q","hit_columns":["title"],"media_id":5158,"season_id":5158,"cover":"http://i0.hdslb.com/bfs/bangumi/image/a09e168ca80fca0084ce5a5e3b999879ed9fe56a.png"}]}}`), nil
		default:
			t.Fatalf("unexpected url: %s", req.URL.String())
			return nil, nil
		}
	}}

	provider := BilibiliProvider{Client: client}
	candidates, err := provider.Search(context.Background(), "侦探学园Q")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if !sawSearch {
		t.Fatalf("search request was not sent")
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %#v, want one", candidates)
	}
	got := candidates[0]
	if got.Title != "侦探学园Q" || got.ImageURL != "https://i0.hdslb.com/bfs/bangumi/image/a09e168ca80fca0084ce5a5e3b999879ed9fe56a.png" || got.Source != "bilibili" || got.Confidence != "high" {
		t.Fatalf("candidate = %#v", got)
	}
}

func TestBilibiliProviderSearchPrefersExactTitle(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.String() == "https://api.bilibili.com/x/frontend/finger/spi":
			return textResponse(200, `{"code":0,"message":"0","data":{"b_3":"BUVID3","b_4":"BUVID4"}}`), nil
		case req.URL.String() == "https://api.bilibili.com/x/web-interface/nav":
			return textResponse(200, fakeBilibiliNavResponse), nil
		case req.URL.Path == "/x/web-interface/wbi/search/type":
			requireBilibiliSearchRequest(t, req, "约定的梦幻岛")
			return textResponse(200, `{"code":0,"message":"0","data":{"result":[{"title":"<em class=\"keyword\">约定的梦幻岛</em> 粤配版","org_title":"約束のネバーランド","hit_columns":[],"cover":"http://img.example/dubbed.png"},{"title":"<em class=\"keyword\">约定的梦幻岛</em>","org_title":"約束のネバーランド","hit_columns":[],"cover":"http://img.example/original.png"}]}}`), nil
		default:
			t.Fatalf("unexpected url: %s", req.URL.String())
			return nil, nil
		}
	}}

	provider := BilibiliProvider{Client: client}
	candidates, err := provider.Search(context.Background(), "约定的梦幻岛")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].ImageURL != "https://img.example/original.png" || candidates[0].Confidence != "high" {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestBilibiliProviderSearchSkipsUnrelatedResults(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.String() == "https://api.bilibili.com/x/frontend/finger/spi":
			return textResponse(200, `{"code":0,"message":"0","data":{"b_3":"BUVID3","b_4":"BUVID4"}}`), nil
		case req.URL.String() == "https://api.bilibili.com/x/web-interface/nav":
			return textResponse(200, fakeBilibiliNavResponse), nil
		case req.URL.Path == "/x/web-interface/wbi/search/type":
			requireBilibiliSearchRequest(t, req, "Another")
			return textResponse(200, `{"code":0,"message":"0","data":{"result":[{"title":"奇幻世界舅舅","org_title":"異世界おじさん","hit_columns":[],"cover":"http://img.example/wrong.png"}]}}`), nil
		default:
			t.Fatalf("unexpected url: %s", req.URL.String())
			return nil, nil
		}
	}}

	provider := BilibiliProvider{Client: client}
	candidates, err := provider.Search(context.Background(), "Another")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %#v, want none", candidates)
	}
}

func TestBilibiliProviderSearchAcceptsHighlightedTitleVariant(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.String() == "https://api.bilibili.com/x/frontend/finger/spi":
			return textResponse(200, `{"code":0,"message":"0","data":{"b_3":"BUVID3","b_4":"BUVID4"}}`), nil
		case req.URL.String() == "https://api.bilibili.com/x/web-interface/nav":
			return textResponse(200, fakeBilibiliNavResponse), nil
		case req.URL.Path == "/x/web-interface/wbi/search/type":
			requireBilibiliSearchRequest(t, req, "冰果")
			return textResponse(200, `{"code":0,"message":"0","data":{"result":[{"title":"<em class=\"keyword\">冰菓</em>","org_title":"<em class=\"keyword\">冰菓</em>","hit_columns":[],"cover":"https://img.example/hyouka.png"}]}}`), nil
		default:
			t.Fatalf("unexpected url: %s", req.URL.String())
			return nil, nil
		}
	}}

	provider := BilibiliProvider{Client: client}
	candidates, err := provider.Search(context.Background(), "冰果")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].Title != "冰菓" || candidates[0].Confidence != "medium" {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestBilibiliProviderSearchRetriesEmptyResults(t *testing.T) {
	searches := 0
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.String() == "https://api.bilibili.com/x/frontend/finger/spi":
			return textResponse(200, `{"code":0,"message":"0","data":{"b_3":"BUVID3","b_4":"BUVID4"}}`), nil
		case req.URL.String() == "https://api.bilibili.com/x/web-interface/nav":
			return textResponse(200, fakeBilibiliNavResponse), nil
		case req.URL.Path == "/x/web-interface/wbi/search/type":
			requireBilibiliSearchRequest(t, req, "目隐都市的演绎者")
			searches++
			if searches == 1 {
				return textResponse(200, `{"code":0,"message":"OK","data":{"result":[]}}`), nil
			}
			return textResponse(200, `{"code":0,"message":"OK","data":{"result":[{"title":"目隐都市的演绎者","org_title":"メカクシティアクターズ","hit_columns":["title"],"cover":"https://img.example/mekaku.png"}]}}`), nil
		default:
			t.Fatalf("unexpected url: %s", req.URL.String())
			return nil, nil
		}
	}}

	provider := BilibiliProvider{Client: client}
	candidates, err := provider.Search(context.Background(), "目隐都市的演绎者")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if searches != 2 {
		t.Fatalf("searches = %d, want 2", searches)
	}
	if len(candidates) != 1 || candidates[0].ImageURL != "https://img.example/mekaku.png" {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestBilibiliProviderSearchRefreshesCookiesAfterPreconditionFailure(t *testing.T) {
	fingerprints := 0
	searches := 0
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.String() == "https://api.bilibili.com/x/frontend/finger/spi":
			fingerprints++
			return textResponse(200, fmt.Sprintf(`{"code":0,"message":"0","data":{"b_3":"BUVID%d","b_4":"BUVID4-%d"}}`, fingerprints, fingerprints)), nil
		case req.URL.String() == "https://api.bilibili.com/x/web-interface/nav":
			return textResponse(200, fakeBilibiliNavResponse), nil
		case req.URL.Path == "/x/web-interface/wbi/search/type":
			requireBilibiliSearchRequest(t, req, "GOSICK")
			searches++
			if searches == 1 {
				if !strings.Contains(req.Header.Get("Cookie"), "buvid3=BUVID1") {
					t.Fatalf("first Cookie = %q, want first buvid", req.Header.Get("Cookie"))
				}
				return textResponse(412, `blocked`), nil
			}
			if !strings.Contains(req.Header.Get("Cookie"), "buvid3=BUVID2") {
				t.Fatalf("retry Cookie = %q, want refreshed buvid", req.Header.Get("Cookie"))
			}
			return textResponse(200, `{"code":0,"message":"0","data":{"result":[{"title":"GOSICK","org_title":"GOSICK","hit_columns":["title"],"cover":"https://img.example/gosick.jpg"}]}}`), nil
		default:
			t.Fatalf("unexpected url: %s", req.URL.String())
			return nil, nil
		}
	}}

	provider := BilibiliProvider{Client: client}
	candidates, err := provider.Search(context.Background(), "GOSICK")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if fingerprints != 2 || searches != 2 {
		t.Fatalf("fingerprints/searches = %d/%d, want 2/2", fingerprints, searches)
	}
	if len(candidates) != 1 || candidates[0].Title != "GOSICK" {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestBangumiProviderSearchReturnsCoverCandidate(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		requireBangumiSearchRequest(t, req, "神的记事本")
		return textResponse(200, `{"results":8,"list":[{"name":"神様のメモ帳","name_cn":"神的记事本","images":{"large":"http://lain.bgm.tv/pic/cover/l/60/eb/15237_y1hzw.jpg","common":"http://lain.bgm.tv/pic/cover/c/60/eb/15237_y1hzw.jpg"}}]}`), nil
	}}

	candidates, err := BangumiProvider{Client: client}.Search(context.Background(), "神的记事本")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %#v, want one", candidates)
	}
	got := candidates[0]
	if got.Title != "神的记事本" || got.ImageURL != "https://lain.bgm.tv/pic/cover/l/60/eb/15237_y1hzw.jpg" || got.Source != "bangumi" || got.Confidence != "high" {
		t.Fatalf("candidate = %#v", got)
	}
}

func TestBangumiProviderSearchSkipsMismatchedResult(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		requireBangumiSearchRequest(t, req, "彼方的阿斯特拉")
		return textResponse(200, `{"results":1,"list":[{"name":"Penguins of Madagascar","name_cn":"马达加斯加的企鹅","images":{"large":"http://lain.bgm.tv/pic/cover/l/9a/8a/116276_V957M.jpg"}}]}`), nil
	}}

	candidates, err := BangumiProvider{Client: client}.Search(context.Background(), "彼方的阿斯特拉")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %#v, want none", candidates)
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

func TestMyDramaListProviderSearchSkipsMismatchedResult(t *testing.T) {
	client := fakeHTTPClient{do: func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://mydramalist.com/search?q=%E7%A5%9E%E7%9A%84%E8%AE%B0%E4%BA%8B%E6%9C%AC":
			return textResponse(200, `<a class="block" href="/456-wrong">Wrong Drama</a>`), nil
		default:
			t.Fatalf("unexpected url: %s", req.URL.String())
			return nil, nil
		}
	}}

	candidates, err := MyDramaListProvider{Client: client}.Search(context.Background(), "神的记事本")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %#v, want none", candidates)
	}
}

func TestProvidersForSourcesDefaultUsesBilibiliFirst(t *testing.T) {
	providers, err := ProvidersForSources(nil, nil)
	if err != nil {
		t.Fatalf("ProvidersForSources() error = %v", err)
	}
	var names []string
	for _, provider := range providers {
		names = append(names, provider.Name())
	}
	want := []string{"bilibili", "bangumi", "anilist", "mydramalist"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("providers = %#v, want %#v", names, want)
	}
}

func TestProvidersForSourcesAcceptsBilibili(t *testing.T) {
	providers, err := ProvidersForSources([]string{"bilibili"}, nil)
	if err != nil {
		t.Fatalf("ProvidersForSources() error = %v", err)
	}
	if len(providers) != 1 || providers[0].Name() != "bilibili" {
		t.Fatalf("providers = %#v", providers)
	}
}

func TestProvidersForSourcesAcceptsBangumi(t *testing.T) {
	providers, err := ProvidersForSources([]string{"bangumi"}, nil)
	if err != nil {
		t.Fatalf("ProvidersForSources() error = %v", err)
	}
	if len(providers) != 1 || providers[0].Name() != "bangumi" {
		t.Fatalf("providers = %#v", providers)
	}
}

func TestProvidersForSourcesRejectsUnknownSource(t *testing.T) {
	_, err := ProvidersForSources([]string{"anlist", "mydramalist"}, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown poster provider") {
		t.Fatalf("ProvidersForSources() error = %v, want unknown provider", err)
	}
}
