package deck

import (
	"fmt"
	"math/rand"
	"strings"
)

const (
	ThemeDefault       = "default"
	ThemeShuffleLight  = "shuffle-light"
	ThemeWarmPaper     = "warm-paper"
	ThemeEditorialCool = "editorial-cool"
	ThemeLifestyle     = "lifestyle-light"
	ThemeTechNoir      = "tech-noir"
	ThemeEditorialMono = "editorial-mono"

	paletteDefaultOrange = "default-orange"
	paletteDefaultGreen  = "default-green"
)

var shuffleLightPaletteKeys = []string{
	paletteDefaultOrange,
	paletteDefaultGreen,
	ThemeWarmPaper,
	ThemeEditorialCool,
	ThemeLifestyle,
	ThemeEditorialMono,
}

func ShuffleLightPaletteKeys() []string {
	out := make([]string, len(shuffleLightPaletteKeys))
	copy(out, shuffleLightPaletteKeys)
	return out
}

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
			AccentForeground:   "#FFFFFF",
			InversePillColor:   "#171717",
			WatermarkColor:     "rgba(23, 23, 23, 0.28)",
			EmphasisColor:      "#D24F2F",
			NumberColor:        "#E85B3A",
			InlineCodeBG:       "#F5EFE6",
			InlineCodeBorder:   "#E2CDBF",
			InlineCodeColor:    "#171717",
			CodeBlockBG:        "#FFF8F1",
			CodeBlockBorder:    "#E8DED2",
			CodeBlockColor:     "#171717",
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
			AccentForeground:   "#FFFFFF",
			InversePillColor:   "#2A241F",
			WatermarkColor:     "rgba(23, 23, 23, 0.28)",
			EmphasisColor:      "#B86F48",
			NumberColor:        "#C77C52",
			InlineCodeBG:       "#F1EAE2",
			InlineCodeBorder:   "#DCCEC2",
			InlineCodeColor:    "#2A241F",
			CodeBlockBG:        "#FBF7F3",
			CodeBlockBorder:    "#E7DDD2",
			CodeBlockColor:     "#2A241F",
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
	}
}

func ResolveDeckTheme(input string) string {
	candidate := strings.TrimSpace(input)
	if candidate == "" {
		return ThemeDefault
	}
	if candidate == ThemeDefault || candidate == ThemeShuffleLight {
		return candidate
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

func ResolveConcretePageTheme(deckTheme string, legacyPageTheme string, assignedPageTheme string) string {
	if ResolveDeckTheme(deckTheme) == ThemeShuffleLight {
		return strings.TrimSpace(assignedPageTheme)
	}
	return ResolvePageTheme(deckTheme, legacyPageTheme)
}

func AssignShuffleLightPageThemes(pageCount int, r *rand.Rand) ([]string, error) {
	if pageCount < 1 {
		return nil, fmt.Errorf("shuffle-light requires at least 1 page")
	}
	if r == nil {
		return nil, fmt.Errorf("shuffle-light requires random source")
	}
	result := make([]string, 0, pageCount)
	prev := ""
	for i := 0; i < pageCount; i++ {
		candidates := make([]string, 0, len(shuffleLightPaletteKeys))
		for _, key := range shuffleLightPaletteKeys {
			if key == prev {
				continue
			}
			candidates = append(candidates, key)
		}
		if len(candidates) == 0 {
			return nil, fmt.Errorf("shuffle-light has no candidate palette for page %d", i)
		}
		current := candidates[r.Intn(len(candidates))]
		result = append(result, current)
		prev = current
	}
	return result, nil
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
