package fonts

import (
	"testing"

	"github.com/matisiekpl/unipdf/v3/internal/textencoding"
)

func TestEncoderWithoutGlyphNamesUsesCharacterCodes(t *testing.T) {
	ttf := TtfType{Chars: map[rune]GID{'s': 86, 'z': 93, 'c': 70}}

	encoder, err := ttf.MakeEncoder()
	if err != nil {
		t.Fatal(err)
	}

	for code, want := range map[textencoding.CharCode]rune{'s': 's', 'z': 'z', 'c': 'c'} {
		if got, ok := encoder.CharcodeToRune(code); !ok || got != want {
			t.Errorf("code %q decoded as %q, want %q", rune(code), got, want)
		}
	}
}

func TestEncoderWithGlyphNamesUsesThem(t *testing.T) {
	names := make([]GlyphName, 90)
	names[86] = "Aring"
	ttf := TtfType{Chars: map[rune]GID{'s': 86}, GlyphNames: names}

	encoder, err := ttf.MakeEncoder()
	if err != nil {
		t.Fatal(err)
	}

	if got, _ := encoder.CharcodeToRune('s'); got != 'Å' {
		t.Errorf("decoded as %q, want the glyph name to win", got)
	}
}
