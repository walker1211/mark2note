package render

import "github.com/walker1211/mark2note/internal/deck"

func themeVarsCSS(theme deck.Theme) string {
	accentForeground := "#FFFFFF"
	inversePillColor := theme.Text
	watermarkColor := "rgba(23, 23, 23, 0.28)"
	emphasisColor := theme.Accent
	numberColor := theme.Accent
	inlineCodeBG := theme.Panel
	inlineCodeBorder := theme.Line
	inlineCodeColor := theme.Text
	codeBlockBG := theme.Card
	codeBlockBorder := theme.Line
	codeBlockColor := theme.Text

	if theme.Name == deck.ThemeTechNoir {
		accentForeground = theme.BG
		inversePillColor = theme.BG
		watermarkColor = "rgba(245, 241, 232, 0.32)"
		emphasisColor = theme.Accent
		numberColor = theme.Accent
		inlineCodeBG = "#171B1F"
		inlineCodeBorder = theme.Accent
		inlineCodeColor = theme.Text
		codeBlockBG = "#15191D"
		codeBlockBorder = theme.Line
		codeBlockColor = theme.Text
	}

	return "" +
		":root {\n" +
		"  --bg: " + theme.BG + ";\n" +
		"  --card: " + theme.Card + ";\n" +
		"  --text: " + theme.Text + ";\n" +
		"  --sub: " + theme.Sub + ";\n" +
		"  --accent: " + theme.Accent + ";\n" +
		"  --accent-soft: " + theme.AccentSoft + ";\n" +
		"  --accent-foreground: " + accentForeground + ";\n" +
		"  --inverse-pill-color: " + inversePillColor + ";\n" +
		"  --watermark-color: " + watermarkColor + ";\n" +
		"  --line: " + theme.Line + ";\n" +
		"  --panel: " + theme.Panel + ";\n" +
		"  --white: " + theme.White + ";\n" +
		"  --cta-shadow: 0 18px 40px rgba(0, 0, 0, 0.12);\n" +
		"  --emphasis-color: " + emphasisColor + ";\n" +
		"  --number-color: " + numberColor + ";\n" +
		"  --inline-code-bg: " + inlineCodeBG + ";\n" +
		"  --inline-code-border: " + inlineCodeBorder + ";\n" +
		"  --inline-code-color: " + inlineCodeColor + ";\n" +
		"  --code-block-bg: " + codeBlockBG + ";\n" +
		"  --code-block-border: " + codeBlockBorder + ";\n" +
		"  --code-block-color: " + codeBlockColor + ";\n" +
		"  --author-color: " + theme.AuthorColor + ";\n" +
		"  --author-weight: " + theme.AuthorWeight + ";\n" +
		"  --author-size: " + theme.AuthorSize + ";\n" +
		"  --author-bottom-offset: " + theme.AuthorBottomOffset + ";\n" +
		"}"
}
