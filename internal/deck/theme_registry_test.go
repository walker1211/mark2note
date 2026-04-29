package deck

import "testing"

func TestResolveDeckThemeFallsBackToDefault(t *testing.T) {
	for _, input := range []string{"missing-theme", "editorial-mono", "lifestyle-light", "shuffle-light"} {
		if got := ResolveDeckTheme(input); got != ThemeDefault {
			t.Fatalf("ResolveDeckTheme(%q) = %q, want %q", input, got, ThemeDefault)
		}
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

func TestRegisteredThemesIncludesNamedThemes(t *testing.T) {
	themes := RegisteredThemes()
	for _, key := range stableThemeKeys() {
		if _, ok := themes[key]; !ok {
			t.Fatalf("RegisteredThemes missing %s", key)
		}
	}
}

func TestResolveDeckThemeReturnsNamedThemesDirectly(t *testing.T) {
	for _, key := range stableThemeKeys() {
		if got := ResolveDeckTheme(key); got != key {
			t.Fatalf("ResolveDeckTheme(%q) = %q, want %q", key, got, key)
		}
	}
}

func TestRegisteredThemesExposeSemanticTokens(t *testing.T) {
	themes := RegisteredThemes()

	for _, key := range stableThemeKeys() {
		theme, ok := themes[key]
		if !ok {
			t.Fatalf("RegisteredThemes missing %s", key)
		}
		if err := theme.Validate(); err != nil {
			t.Fatalf("theme %s validation failed: %v", key, err)
		}
	}
}

func TestResolvePageThemeIgnoresLegacyPageMeta(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{name: "default ignores green legacy meta", got: ResolvePageTheme(ThemeDefault, "green"), want: ThemeDefault},
		{name: "fresh green ignores orange legacy meta", got: ResolvePageTheme(ThemeFreshGreen, "orange"), want: ThemeFreshGreen},
		{name: "concrete page ignores legacy and assigned themes", got: ResolveConcretePageTheme("shuffle-light", "green", "fresh-green"), want: ThemeDefault},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("got = %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestRegisteredThemesPreserveRenamedTokenValues(t *testing.T) {
	themes := RegisteredThemes()

	defaultTheme := themes[ThemeDefault]
	if defaultTheme.BG != "#F4F1EB" || defaultTheme.Accent != "#181818" {
		t.Fatalf("default tokens = BG %q, Accent %q", defaultTheme.BG, defaultTheme.Accent)
	}

	freshGreen := themes[ThemeFreshGreen]
	if freshGreen.BG != "#F2F7F2" || freshGreen.Accent != "#2F8F61" {
		t.Fatalf("fresh-green tokens = BG %q, Accent %q", freshGreen.BG, freshGreen.Accent)
	}
}

func stableThemeKeys() []string {
	return []string{
		ThemeDefault,
		ThemeWarmPaper,
		ThemeEditorialCool,
		ThemeTechNoir,
		ThemePlumInk,
		ThemeSageMist,
		ThemeFreshGreen,
	}
}
