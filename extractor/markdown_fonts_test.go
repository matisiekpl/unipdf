package extractor

import (
	"strings"
	"testing"
)

func TestPageFontIsNotTakenFromFormWithSameName(t *testing.T) {
	markdown := documentMarkdownOf(t, "testdata/fonts/chpl_38136.pdf")

	if strings.ContainsAny(markdown, "⁐‰‱′″‴‵") {
		t.Errorf("footer decoded with the font of the page form: %q", markdown[:200])
	}
}

func TestTrueTypeWithoutGlyphNamesUsesCharacterCodes(t *testing.T) {
	markdown := documentMarkdownOf(t, "testdata/fonts/chpl_38822.pdf")

	for _, phrase := range []string{"CHARAKTERYSTYKA WETERYNARYJNEGO PRODUKTU LECZNICZEGO", "Substancja czynna", "Karprofen"} {
		if !strings.Contains(markdown, phrase) {
			t.Errorf("missing %q in %q", phrase, markdown[:300])
		}
	}
}
