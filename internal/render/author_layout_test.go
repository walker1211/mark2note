package render

import (
	"strings"
	"testing"
)

func TestResolveCoverAuthorLayoutFitsAt24px(t *testing.T) {
	got := resolveCoverAuthorLayout("@搁剑听风")
	if !got.Show {
		t.Fatalf("Show = false, want true")
	}
	if got.ResolvedAuthorFontSize != 24 {
		t.Fatalf("ResolvedAuthorFontSize = %d, want 24", got.ResolvedAuthorFontSize)
	}
	if got.ResolvedAuthorDisplayText != "@搁剑听风" {
		t.Fatalf("ResolvedAuthorDisplayText = %q", got.ResolvedAuthorDisplayText)
	}
	if got.HideAuthorBecauseOverflow {
		t.Fatalf("HideAuthorBecauseOverflow = true, want false")
	}
}

func TestResolveCoverAuthorLayoutCountsWhitespaceWidth(t *testing.T) {
	got := resolveCoverAuthorLayout("@" + strings.Repeat("A", 15) + " " + strings.Repeat("A", 15))
	if !got.Show {
		t.Fatalf("Show = false, want true")
	}
	if got.ResolvedAuthorFontSize != 22 {
		t.Fatalf("ResolvedAuthorFontSize = %d, want 22", got.ResolvedAuthorFontSize)
	}
}

func TestResolveCoverAuthorLayoutShrinksTo22Or20(t *testing.T) {
	got := resolveCoverAuthorLayout("@" + strings.Repeat("张", 19))
	if !got.Show {
		t.Fatalf("Show = false, want true")
	}
	if got.ResolvedAuthorFontSize != 22 {
		t.Fatalf("ResolvedAuthorFontSize = %d, want 22", got.ResolvedAuthorFontSize)
	}
	if got.HideAuthorBecauseOverflow {
		t.Fatalf("HideAuthorBecauseOverflow = true, want false")
	}
}

func TestResolveCoverAuthorLayoutUsesEllipsisAt20px(t *testing.T) {
	got := resolveCoverAuthorLayout("@" + strings.Repeat("张", 24))
	if !got.Show {
		t.Fatalf("Show = false, want true")
	}
	if got.ResolvedAuthorFontSize != 20 {
		t.Fatalf("ResolvedAuthorFontSize = %d, want 20", got.ResolvedAuthorFontSize)
	}
	if !strings.HasSuffix(got.ResolvedAuthorDisplayText, "…") {
		t.Fatalf("ResolvedAuthorDisplayText = %q, want suffix ellipsis", got.ResolvedAuthorDisplayText)
	}
	if got.HideAuthorBecauseOverflow {
		t.Fatalf("HideAuthorBecauseOverflow = true, want false")
	}
}

func TestResolveCoverAuthorLayoutHidesWhenEllipsisStillOverflows(t *testing.T) {
	got := resolveCoverAuthorLayout("@" + strings.Repeat("张", 80))
	if got.Show {
		t.Fatalf("Show = true, want false")
	}
	if !got.HideAuthorBecauseOverflow {
		t.Fatalf("HideAuthorBecauseOverflow = false, want true")
	}
}
