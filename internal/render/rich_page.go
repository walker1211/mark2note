package render

import "github.com/walker1211/mark2note/internal/deck"

type richCompareRow struct {
	Left  richText
	Right richText
}

type richCompare struct {
	LeftLabel  richText
	RightLabel richText
	Rows       []richCompareRow
}

type richPageContent struct {
	Title    richText
	Subtitle richText
	Body     richText
	Quote    richText
	Note     richText
	Tip      richText
	CTA      richText
	Items    []richText
	Steps    []richText
	Compare  *richCompare
}

func buildRichPageContent(page deck.Page) richPageContent {
	rich := richPageContent{
		Title:    parseRichText(page.Content.Title, richTextOptions{}),
		Subtitle: parseRichText(page.Content.Subtitle, richTextOptions{}),
		Body:     parseRichText(page.Content.Body, richTextOptions{AllowCodeBlocks: true}),
		Quote:    parseRichText(page.Content.Quote, richTextOptions{}),
		Note:     parseRichText(page.Content.Note, richTextOptions{AllowCodeBlocks: true}),
		Tip:      parseRichText(page.Content.Tip, richTextOptions{AllowCodeBlocks: true}),
		CTA:      parseRichText(page.Meta.CTA, richTextOptions{}),
		Items:    make([]richText, 0, len(page.Content.Items)),
		Steps:    make([]richText, 0, len(page.Content.Steps)),
	}

	for _, item := range page.Content.Items {
		rich.Items = append(rich.Items, parseRichText(item, richTextOptions{}))
	}
	for _, step := range page.Content.Steps {
		rich.Steps = append(rich.Steps, parseRichText(step, richTextOptions{}))
	}
	if page.Content.Compare != nil {
		compare := &richCompare{
			LeftLabel:  parseRichText(page.Content.Compare.LeftLabel, richTextOptions{}),
			RightLabel: parseRichText(page.Content.Compare.RightLabel, richTextOptions{}),
			Rows:       make([]richCompareRow, 0, len(page.Content.Compare.Rows)),
		}
		for _, row := range page.Content.Compare.Rows {
			compare.Rows = append(compare.Rows, richCompareRow{
				Left:  parseRichText(row.Left, richTextOptions{}),
				Right: parseRichText(row.Right, richTextOptions{}),
			})
		}
		rich.Compare = compare
	}

	return rich
}
