package deck

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestResolveDeckThemeFallsBackToDefault(t *testing.T) {
	if got := ResolveDeckTheme("missing-theme"); got != ThemeDefault {
		t.Fatalf("ResolveDeckTheme() = %q, want %q", got, ThemeDefault)
	}
}

func TestResolveCoverAuthorPrefersExplicitValue(t *testing.T) {
	got := ResolveCoverAuthor("@单次作者", "全局作者")
	if !got.Show || got.Text != "@单次作者" {
		t.Fatalf("got = %#v", got)
	}
}

func TestResolveCoverAuthorTreatsWhitespaceAsMissing(t *testing.T) {
	got := ResolveCoverAuthor("   ", "  全局作者  ")
	if !got.Show || got.Text != "@全局作者" {
		t.Fatalf("got = %#v", got)
	}
}

func TestResolveCoverAuthorNormalizesRepeatedAtPrefix(t *testing.T) {
	got := ResolveCoverAuthor("@@作者", "")
	if !got.Show || got.Text != "@作者" {
		t.Fatalf("got = %#v", got)
	}
}

func TestRegisteredThemesIncludesNewThemes(t *testing.T) {
	themes := RegisteredThemes()
	if _, ok := themes[ThemeTechNoir]; !ok {
		t.Fatalf("RegisteredThemes missing %s", ThemeTechNoir)
	}
	if _, ok := themes[ThemeEditorialMono]; !ok {
		t.Fatalf("RegisteredThemes missing %s", ThemeEditorialMono)
	}
}

func TestResolveDeckThemeReturnsNewThemesDirectly(t *testing.T) {
	if got := ResolveDeckTheme(ThemeTechNoir); got != ThemeTechNoir {
		t.Fatalf("ResolveDeckTheme() = %q, want %q", got, ThemeTechNoir)
	}
	if got := ResolveDeckTheme(ThemeEditorialMono); got != ThemeEditorialMono {
		t.Fatalf("ResolveDeckTheme() = %q, want %q", got, ThemeEditorialMono)
	}
	if got := ResolveDeckTheme(ThemeShuffleLight); got != ThemeShuffleLight {
		t.Fatalf("ResolveDeckTheme() = %q, want %q", got, ThemeShuffleLight)
	}
}

func TestShuffleLightPalettePoolExcludesTechNoir(t *testing.T) {
	pool := ShuffleLightPaletteKeys()
	want := []string{
		paletteDefaultOrange,
		paletteDefaultGreen,
		ThemeWarmPaper,
		ThemeEditorialCool,
		ThemeLifestyle,
		ThemeEditorialMono,
	}
	if !reflect.DeepEqual(pool, want) {
		t.Fatalf("ShuffleLightPaletteKeys() = %#v, want %#v", pool, want)
	}
}

func TestAssignShuffleLightPageThemesNeverRepeatsAdjacentPages(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	got, err := AssignShuffleLightPageThemes(8, r)
	if err != nil {
		t.Fatalf("AssignShuffleLightPageThemes() error = %v", err)
	}
	if len(got) != 8 {
		t.Fatalf("len(got) = %d, want 8", len(got))
	}
	allowed := make(map[string]struct{}, len(ShuffleLightPaletteKeys()))
	for _, key := range ShuffleLightPaletteKeys() {
		allowed[key] = struct{}{}
	}
	for i, key := range got {
		if _, ok := allowed[key]; !ok {
			t.Fatalf("got[%d] = %q, not in allowed pool", i, key)
		}
		if key == ThemeTechNoir {
			t.Fatalf("got[%d] unexpectedly used tech-noir", i)
		}
		if i > 0 && got[i-1] == key {
			t.Fatalf("adjacent pages repeated palette %q at index %d", key, i)
		}
	}
}

func TestRegisteredThemesExposeSemanticTokens(t *testing.T) {
	themes := RegisteredThemes()
	required := []string{
		paletteDefaultOrange,
		paletteDefaultGreen,
		ThemeWarmPaper,
		ThemeEditorialCool,
		ThemeLifestyle,
		ThemeTechNoir,
		ThemeEditorialMono,
	}

	for _, key := range required {
		theme, ok := themes[key]
		if !ok {
			t.Fatalf("RegisteredThemes missing %s", key)
		}
		if err := theme.Validate(); err != nil {
			t.Fatalf("theme %s validation failed: %v", key, err)
		}
	}
}

func TestResolvePageThemePreservesDefaultOrangeGreenResolutionBehavior(t *testing.T) {
	cases := []struct {
		name            string
		deckTheme       string
		legacyPageTheme string
		want            string
	}{
		{name: "default orange", deckTheme: ThemeDefault, legacyPageTheme: "orange", want: paletteDefaultOrange},
		{name: "default green", deckTheme: ThemeDefault, legacyPageTheme: "green", want: paletteDefaultGreen},
		{name: "custom deck theme bypasses legacy orange", deckTheme: ThemeTechNoir, legacyPageTheme: "orange", want: ThemeTechNoir},
		{name: "custom deck theme bypasses legacy green", deckTheme: ThemeEditorialMono, legacyPageTheme: "green", want: ThemeEditorialMono},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolvePageTheme(tc.deckTheme, tc.legacyPageTheme); got != tc.want {
				t.Fatalf("ResolvePageTheme(%q, %q) = %q, want %q", tc.deckTheme, tc.legacyPageTheme, got, tc.want)
			}
		})
	}
}
