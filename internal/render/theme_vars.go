package render

import "github.com/walker1211/mark2note/internal/deck"

func themeVarsCSS(theme deck.Theme) string {
	return "" +
		":root {\n" +
		"  --bg: " + theme.BG + ";\n" +
		"  --card: " + theme.Card + ";\n" +
		"  --text: " + theme.Text + ";\n" +
		"  --sub: " + theme.Sub + ";\n" +
		"  --accent: " + theme.Accent + ";\n" +
		"  --accent-soft: " + theme.AccentSoft + ";\n" +
		"  --accent-foreground: " + theme.AccentForeground + ";\n" +
		"  --inverse-pill-color: " + theme.InversePillColor + ";\n" +
		"  --watermark-color: " + theme.WatermarkColor + ";\n" +
		"  --line: " + theme.Line + ";\n" +
		"  --panel: " + theme.Panel + ";\n" +
		"  --white: " + theme.White + ";\n" +
		"  --cta-shadow: 0 18px 40px rgba(0, 0, 0, 0.12);\n" +
		"  --emphasis-color: " + theme.EmphasisColor + ";\n" +
		"  --number-color: " + theme.NumberColor + ";\n" +
		"  --inline-code-bg: " + theme.InlineCodeBG + ";\n" +
		"  --inline-code-border: " + theme.InlineCodeBorder + ";\n" +
		"  --inline-code-color: " + theme.InlineCodeColor + ";\n" +
		"  --code-block-bg: " + theme.CodeBlockBG + ";\n" +
		"  --code-block-border: " + theme.CodeBlockBorder + ";\n" +
		"  --code-block-color: " + theme.CodeBlockColor + ";\n" +
		"  --author-color: " + theme.AuthorColor + ";\n" +
		"  --author-weight: " + theme.AuthorWeight + ";\n" +
		"  --author-size: " + theme.AuthorSize + ";\n" +
		"  --author-bottom-offset: " + theme.AuthorBottomOffset + ";\n" +
		"}"
}
