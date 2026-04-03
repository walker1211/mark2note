package deck

import "strings"

const (
	ThemeDefault       = "default"
	ThemeWarmPaper     = "warm-paper"
	ThemeEditorialCool = "editorial-cool"
	ThemeLifestyle     = "lifestyle-light"
	ThemeTechNoir      = "tech-noir"
	ThemeEditorialMono = "editorial-mono"

	paletteDefaultOrange = "default-orange"
	paletteDefaultGreen  = "default-green"
)

type CoverAuthor struct {
	Show bool
	Text string
}

func RegisteredThemes() map[string]Theme {
	return map[string]Theme{
		paletteDefaultOrange: {
			Name:               paletteDefaultOrange,
			BG:                 "#F7F3EC",
			Card:               "#FFFDF9",
			Text:               "#171717",
			Sub:                "#5E5A56",
			Accent:             "#E85B3A",
			AccentSoft:         "#FDE6DE",
			Line:               "#E8DED2",
			Panel:              "#F5EFE6",
			White:              "#FFFFFF",
			AuthorColor:        "#5E5A56",
			AuthorWeight:       "600",
			AuthorSize:         "24px",
			AuthorBottomOffset: "236px",
		},
		paletteDefaultGreen: {
			Name:               paletteDefaultGreen,
			BG:                 "#F2F7F2",
			Card:               "#FCFFFC",
			Text:               "#1A1F1B",
			Sub:                "#4F6155",
			Accent:             "#2F8F61",
			AccentSoft:         "#DFF5E8",
			Line:               "#D4E8DA",
			Panel:              "#ECF4EE",
			White:              "#FFFFFF",
			AuthorColor:        "#4F6155",
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
			AuthorColor:        "#5B6A78",
			AuthorWeight:       "600",
			AuthorSize:         "24px",
			AuthorBottomOffset: "236px",
		},
		ThemeLifestyle: {
			Name:               ThemeLifestyle,
			BG:                 "#F6F4EF",
			Card:               "#FFFDFC",
			Text:               "#2A241F",
			Sub:                "#6C6258",
			Accent:             "#C77C52",
			AccentSoft:         "#F7E7DE",
			Line:               "#E7DDD2",
			Panel:              "#F1EAE2",
			White:              "#FFFFFF",
			AuthorColor:        "#6C6258",
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
			AuthorColor:        "#A6ADB5",
			AuthorWeight:       "600",
			AuthorSize:         "24px",
			AuthorBottomOffset: "236px",
		},
		ThemeEditorialMono: {
			Name:               ThemeEditorialMono,
			BG:                 "#F4F1EB",
			Card:               "#FAF8F3",
			Text:               "#121212",
			Sub:                "#5F5A54",
			Accent:             "#181818",
			AccentSoft:         "#E8E2D8",
			Line:               "#D7D0C6",
			Panel:              "#EFE9DF",
			White:              "#FFFFFF",
			AuthorColor:        "#5F5A54",
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
	if candidate == ThemeDefault {
		return ThemeDefault
	}
	if _, ok := RegisteredThemes()[candidate]; ok {
		return candidate
	}
	return ThemeDefault
}

func ResolvePageTheme(deckTheme string, legacyPageTheme string) string {
	resolvedDeckTheme := ResolveDeckTheme(deckTheme)
	if resolvedDeckTheme != ThemeDefault {
		return resolvedDeckTheme
	}
	switch strings.TrimSpace(legacyPageTheme) {
	case "orange":
		return paletteDefaultOrange
	case "green":
		return paletteDefaultGreen
	default:
		return ""
	}
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
