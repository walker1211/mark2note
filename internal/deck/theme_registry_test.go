package deck

import "testing"

func TestResolveDeckThemeFallsBackToDefault(t *testing.T) {
	if got := ResolveDeckTheme("missing-theme"); got != ThemeDefault {
		t.Fatalf("ResolveDeckTheme() = %q, want %q", got, ThemeDefault)
	}
}

func TestResolvePageThemeKeepsDefaultOrangeGreenVariants(t *testing.T) {
	if got := ResolvePageTheme(ThemeDefault, "orange"); got != paletteDefaultOrange {
		t.Fatalf("ResolvePageTheme() = %q, want %q", got, paletteDefaultOrange)
	}
	if got := ResolvePageTheme(ThemeDefault, "green"); got != paletteDefaultGreen {
		t.Fatalf("ResolvePageTheme() = %q, want %q", got, paletteDefaultGreen)
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
