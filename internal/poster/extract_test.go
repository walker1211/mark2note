package poster

import (
	"reflect"
	"testing"

	"github.com/walker1211/mark2note/internal/deck"
)

func TestExtractTitlesFromMarkdownKeepsFirstSeenOrder(t *testing.T) {
	markdown := `# 片单

- **噬谎者**：规则、胆量都算进局里。
- **死亡笔记**：夜神月和 L 的对抗。
- 也可以看《朋友游戏》和《真实账号》。
- **死亡笔记**：重复项不应该再出现。
`
	want := []string{"朋友游戏", "真实账号", "噬谎者", "死亡笔记"}
	if got := ExtractTitlesFromMarkdown(markdown); !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractTitlesFromMarkdown() = %#v, want %#v", got, want)
	}
}

func TestExtractTitlesFromDeckReadsCompareLabelsRowsAndGallery(t *testing.T) {
	d := deck.Deck{Pages: []deck.Page{
		{
			Variant: "compare",
			Content: deck.PageContent{Compare: &deck.CompareBlock{
				LeftLabel:  "《噬谎者》",
				RightLabel: "《死亡笔记》",
				Rows:       []deck.CompareRow{{Left: "《朋友游戏》：人性局", Right: "《真实账号》：社交网络"}},
			}},
		},
		{
			Variant: "bullets",
			Content: deck.PageContent{Items: []string{"《ACMA:GAME》：规则系", "《约定的梦幻岛》：逃生"}},
		},
		{
			Variant: "gallery-steps",
			Content: deck.PageContent{
				Steps:  []string{"《游戏人生》：高概念"},
				Images: []deck.ImageBlock{{Caption: "《忧国的莫里亚蒂》", Alt: "不会覆盖重复"}},
			},
		},
	}}
	want := []string{"噬谎者", "死亡笔记", "朋友游戏", "真实账号", "ACMA:GAME", "约定的梦幻岛", "游戏人生", "忧国的莫里亚蒂", "不会覆盖重复"}
	if got := ExtractTitlesFromDeck(d); !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractTitlesFromDeck() = %#v, want %#v", got, want)
	}
}
