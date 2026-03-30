package render

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const authorMaxWidth = 439.0

type coverAuthorLayout struct {
	Show                      bool
	ResolvedAuthorDisplayText string
	ResolvedAuthorFontSize    int
	HideAuthorBecauseOverflow bool
}

func resolveCoverAuthorLayout(text string) coverAuthorLayout {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return coverAuthorLayout{}
	}

	for _, size := range []int{24, 22, 20} {
		if estimateAuthorWidth(trimmed, size) <= authorMaxWidth {
			return coverAuthorLayout{
				Show:                      true,
				ResolvedAuthorDisplayText: trimmed,
				ResolvedAuthorFontSize:    size,
			}
		}
	}

	truncated := truncateAuthorToWidth(trimmed, 20, authorMaxWidth)
	if truncated != "" && estimateAuthorWidth(truncated, 20) <= authorMaxWidth {
		visibleRunes := utf8.RuneCountInString(strings.TrimSuffix(truncated, "…"))
		originalRunes := utf8.RuneCountInString(trimmed)
		if visibleRunes*2 >= originalRunes {
			return coverAuthorLayout{
				Show:                      true,
				ResolvedAuthorDisplayText: truncated,
				ResolvedAuthorFontSize:    20,
			}
		}
	}

	return coverAuthorLayout{HideAuthorBecauseOverflow: true}
}

func truncateAuthorToWidth(text string, fontSize int, maxWidth float64) string {
	if estimateAuthorWidth(text, fontSize) <= maxWidth {
		return text
	}
	const ellipsis = "…"
	if estimateAuthorWidth(ellipsis, fontSize) > maxWidth {
		return ""
	}

	var b strings.Builder
	for _, r := range text {
		candidate := b.String() + string(r) + ellipsis
		if estimateAuthorWidth(candidate, fontSize) > maxWidth {
			break
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String() + ellipsis
}

func estimateAuthorWidth(text string, fontSize int) float64 {
	width := 0.0
	previousWasSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if previousWasSpace {
				continue
			}
			previousWasSpace = true
			width += float64(fontSize) * 0.28
			continue
		}
		previousWasSpace = false
		width += float64(fontSize) * authorWidthFactor(r)
	}
	return width
}

func authorWidthFactor(r rune) float64 {
	if r == '@' || r == '_' || r == '-' || unicode.IsPunct(r) {
		return 0.38
	}
	if r <= unicode.MaxASCII {
		switch {
		case unicode.IsUpper(r) || unicode.IsDigit(r):
			return 0.62
		case unicode.IsLower(r):
			return 0.56
		default:
			return 0.38
		}
	}
	if utf8.RuneLen(r) > 1 {
		return 1.00
	}
	return 1.00
}
