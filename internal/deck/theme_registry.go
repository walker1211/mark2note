package deck

import "strings"

const (
	ThemeDefault       = "default"
	ThemeWarmPaper     = "warm-paper"
	ThemeEditorialCool = "editorial-cool"
	ThemeTechNoir      = "tech-noir"
	ThemePlumInk       = "plum-ink"
	ThemeSageMist      = "sage-mist"
	ThemeFreshGreen    = "fresh-green"
)

type CoverAuthor struct {
	Show bool
	Text string
}

func RegisteredThemes() map[string]Theme {
	return map[string]Theme{
		ThemeDefault: {
			Name:               ThemeDefault,
			BG:                 "#F4F1EB",
			Card:               "#FAF8F3",
			Text:               "#121212",
			Sub:                "#5F5A54",
			Accent:             "#181818",
			AccentSoft:         "#E8E2D8",
			Line:               "#D7D0C6",
			Panel:              "#EFE9DF",
			White:              "#FFFFFF",
			AccentForeground:   "#FAF8F3",
			InversePillColor:   "#121212",
			WatermarkColor:     "rgba(23, 23, 23, 0.28)",
			EmphasisColor:      "#2A2723",
			NumberColor:        "#181818",
			InlineCodeBG:       "#EFE9DF",
			InlineCodeBorder:   "#CFC5B7",
			InlineCodeColor:    "#121212",
			CodeBlockBG:        "#F7F3EC",
			CodeBlockBorder:    "#D7D0C6",
			CodeBlockColor:     "#121212",
			AuthorColor:        "#5F5A54",
			AuthorWeight:       "600",
			AuthorSize:         "24px",
			AuthorBottomOffset: "236px",
		},
		ThemeWarmPaper: {
			Name:               ThemeWarmPaper,
			BG:                 "#F7F1E8",
			Card:               "#FFFDF9",
			Text:               "#2F241D",
			Sub:                "#75665B",
			Accent:             "#B86A3F",
			AccentSoft:         "#F4E4D6",
			Line:               "#E5D7C9",
			Panel:              "#F2E8DD",
			White:              "#FFFFFF",
			AccentForeground:   "#FFFFFF",
			InversePillColor:   "#2F241D",
			WatermarkColor:     "rgba(23, 23, 23, 0.28)",
			EmphasisColor:      "#A65B34",
			NumberColor:        "#B86A3F",
			InlineCodeBG:       "#F2E8DD",
			InlineCodeBorder:   "#D8C5B4",
			InlineCodeColor:    "#2F241D",
			CodeBlockBG:        "#FBF5EE",
			CodeBlockBorder:    "#E5D7C9",
			CodeBlockColor:     "#2F241D",
			AuthorColor:        "#75665B",
			AuthorWeight:       "600",
			AuthorSize:         "24px",
			AuthorBottomOffset: "236px",
		},
		ThemeEditorialCool: {
			Name:               ThemeEditorialCool,
			BG:                 "#EEF3F7",
			Card:               "#FBFDFF",
			Text:               "#18222D",
			Sub:                "#5B6A78",
			Accent:             "#3C7FB0",
			AccentSoft:         "#DCEAF5",
			Line:               "#D4E0EA",
			Panel:              "#E7EEF4",
			White:              "#FFFFFF",
			AccentForeground:   "#FFFFFF",
			InversePillColor:   "#18222D",
			WatermarkColor:     "rgba(23, 23, 23, 0.28)",
			EmphasisColor:      "#2E6F9D",
			NumberColor:        "#3C7FB0",
			InlineCodeBG:       "#E7EEF4",
			InlineCodeBorder:   "#C8D7E3",
			InlineCodeColor:    "#18222D",
			CodeBlockBG:        "#F6FAFD",
			CodeBlockBorder:    "#D4E0EA",
			CodeBlockColor:     "#18222D",
			AuthorColor:        "#5B6A78",
			AuthorWeight:       "600",
			AuthorSize:         "24px",
			AuthorBottomOffset: "236px",
		},
		ThemeTechNoir: {
			Name:               ThemeTechNoir,
			BG:                 "#111315",
			Card:               "#181C20",
			Text:               "#F5F1E8",
			Sub:                "#A6ADB5",
			Accent:             "#C9A86A",
			AccentSoft:         "#2A241A",
			Line:               "#2A3138",
			Panel:              "#14181C",
			White:              "#FFFFFF",
			AccentForeground:   "#111315",
			InversePillColor:   "#111315",
			WatermarkColor:     "rgba(245, 241, 232, 0.32)",
			EmphasisColor:      "#C9A86A",
			NumberColor:        "#C9A86A",
			InlineCodeBG:       "#171B1F",
			InlineCodeBorder:   "#C9A86A",
			InlineCodeColor:    "#F5F1E8",
			CodeBlockBG:        "#15191D",
			CodeBlockBorder:    "#2A3138",
			CodeBlockColor:     "#F5F1E8",
			AuthorColor:        "#A6ADB5",
			AuthorWeight:       "600",
			AuthorSize:         "24px",
			AuthorBottomOffset: "236px",
		},
		ThemePlumInk: {
			Name:               ThemePlumInk,
			BG:                 "#F5F1F6",
			Card:               "#FFFDFE",
			Text:               "#241B2B",
			Sub:                "#6F6378",
			Accent:             "#8B3A62",
			AccentSoft:         "#F0DDE8",
			Line:               "#DDD2E0",
			Panel:              "#E9E1EC",
			White:              "#FFFFFF",
			AccentForeground:   "#FFF8FC",
			InversePillColor:   "#241B2B",
			WatermarkColor:     "rgba(36, 27, 43, 0.28)",
			EmphasisColor:      "#8B3A62",
			NumberColor:        "#8B3A62",
			InlineCodeBG:       "#EEE6F0",
			InlineCodeBorder:   "#D7C7DC",
			InlineCodeColor:    "#241B2B",
			CodeBlockBG:        "#FBF7FC",
			CodeBlockBorder:    "#DDD2E0",
			CodeBlockColor:     "#241B2B",
			AuthorColor:        "#6F6378",
			AuthorWeight:       "600",
			AuthorSize:         "24px",
			AuthorBottomOffset: "236px",
		},
		ThemeSageMist: {
			Name:               ThemeSageMist,
			BG:                 "#F2F5EF",
			Card:               "#FFFDF8",
			Text:               "#1F2A24",
			Sub:                "#667464",
			Accent:             "#6F8564",
			AccentSoft:         "#E4ECDE",
			Line:               "#D7E0D2",
			Panel:              "#E9EFE4",
			White:              "#FFFFFF",
			AccentForeground:   "#FFFDF8",
			InversePillColor:   "#1F2A24",
			WatermarkColor:     "rgba(31, 42, 36, 0.28)",
			EmphasisColor:      "#5F7554",
			NumberColor:        "#6F8564",
			InlineCodeBG:       "#E9EFE4",
			InlineCodeBorder:   "#CBD9C5",
			InlineCodeColor:    "#1F2A24",
			CodeBlockBG:        "#FAFCF8",
			CodeBlockBorder:    "#D7E0D2",
			CodeBlockColor:     "#1F2A24",
			AuthorColor:        "#667464",
			AuthorWeight:       "600",
			AuthorSize:         "24px",
			AuthorBottomOffset: "236px",
		},
		ThemeFreshGreen: {
			Name:               ThemeFreshGreen,
			BG:                 "#F2F7F2",
			Card:               "#FCFFFC",
			Text:               "#1A1F1B",
			Sub:                "#4F6155",
			Accent:             "#2F8F61",
			AccentSoft:         "#DFF5E8",
			Line:               "#D4E8DA",
			Panel:              "#ECF4EE",
			White:              "#FFFFFF",
			AccentForeground:   "#FFFFFF",
			InversePillColor:   "#1A1F1B",
			WatermarkColor:     "rgba(23, 23, 23, 0.28)",
			EmphasisColor:      "#267950",
			NumberColor:        "#2F8F61",
			InlineCodeBG:       "#ECF4EE",
			InlineCodeBorder:   "#C9DED0",
			InlineCodeColor:    "#1A1F1B",
			CodeBlockBG:        "#F7FBF8",
			CodeBlockBorder:    "#D4E8DA",
			CodeBlockColor:     "#1A1F1B",
			AuthorColor:        "#4F6155",
			AuthorWeight:       "600",
			AuthorSize:         "24px",
			AuthorBottomOffset: "236px",
		},
	}
}

func ResolveDeckTheme(input string) string {
	candidate := strings.TrimSpace(input)
	if candidate == "" {
		return ThemeDefault
	}
	if _, ok := RegisteredThemes()[candidate]; ok {
		return candidate
	}
	return ThemeDefault
}

func ResolvePageTheme(deckTheme string, legacyPageTheme string) string {
	return ResolveDeckTheme(deckTheme)
}

func ResolveConcretePageTheme(deckTheme string, legacyPageTheme string, assignedPageTheme string) string {
	return ResolveDeckTheme(deckTheme)
}

func ResolveCoverAuthor(explicit string, fallback string) CoverAuthor {
	text := normalizeAuthor(explicit)
	if text == "" {
		text = normalizeAuthor(fallback)
	}
	if text == "" {
		return CoverAuthor{}
	}
	return CoverAuthor{Show: true, Text: "@" + text}
}

func normalizeAuthor(input string) string {
	text := strings.TrimSpace(input)
	text = strings.TrimLeft(text, "@")
	return strings.TrimSpace(text)
}
