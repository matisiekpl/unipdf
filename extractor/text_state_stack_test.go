package extractor

import (
	"math"
	"strings"
	"testing"

	"github.com/matisiekpl/unipdf/v3/model"
)

const courierGlyphWidth = 6.0

func stateStackMarks(t *testing.T, contents string) []TextMark {
	t.Helper()
	resources := model.NewPdfPageResources()
	courier := model.NewStandard14FontMustCompile(model.CourierName)
	helvetica := model.NewStandard14FontMustCompile(model.HelveticaName)
	resources.SetFontByName("Courier", courier.ToPdfObject())
	resources.SetFontByName("Helvetica", helvetica.ToPdfObject())
	extractor := Extractor{resources: resources, contents: contents, mediaBox: r(0, 0, 600, 800)}
	pageText, _, _, err := extractor.ExtractPageText()
	if err != nil {
		t.Fatalf("extracting text: %v", err)
	}
	var glyphs []TextMark
	for _, mark := range pageText.Marks().Elements() {
		if !mark.Meta && strings.TrimSpace(mark.Text) != "" {
			glyphs = append(glyphs, mark)
		}
	}
	return glyphs
}

func stateStackLine(marks []TextMark, baseline float64) []TextMark {
	var line []TextMark
	for _, mark := range marks {
		if math.Abs(mark.BBox.Lly-baseline) < 4 {
			line = append(line, mark)
		}
	}
	return line
}

func stateStackText(marks []TextMark) string {
	var builder strings.Builder
	for _, mark := range marks {
		builder.WriteString(mark.Text)
	}
	return builder.String()
}

func TestTextAfterNestedQKeepsFontSizeAndOrder(t *testing.T) {
	marks := stateStackMarks(t, `
		q q
		BT /Courier 10 Tf 1 0 0 1 50 700 Tm (Header) Tj ET
		q BT /Helvetica 10 Tf 1 0 0 1 50 650 Tm (Bold) Tj ET Q
		BT 1 0 0 1 50 600 Tm (Bardzo) Tj ET
		Q Q
	`)

	line := stateStackLine(marks, 600)
	if text := stateStackText(line); text != "Bardzo" {
		t.Fatalf("text after the block reads %q, want %q", text, "Bardzo")
	}
	for _, glyph := range line {
		if width := glyph.BBox.Urx - glyph.BBox.Llx; math.Abs(width-courierGlyphWidth) > 0.01 {
			t.Errorf("glyph %q is %.2f wide, want %.2f", glyph.Text, width, courierGlyphWidth)
		}
	}
}

func TestTextLeadingIsRestoredAfterQ(t *testing.T) {
	marks := stateStackMarks(t, `
		BT /Courier 10 Tf 14 TL ET
		q BT /Helvetica 10 Tf 30 TL ET Q
		BT 1 0 0 1 50 700 Tm (First) Tj T* (Second) Tj ET
	`)

	if text := stateStackText(stateStackLine(marks, 686)); text != "Second" {
		t.Errorf("line 14 points below reads %q, want %q", text, "Second")
	}
	if text := stateStackText(stateStackLine(marks, 700)); text != "First" {
		t.Errorf("first line reads %q, want %q", text, "First")
	}
}

func TestFontSetInsideBlockDoesNotOutliveIt(t *testing.T) {
	marks := stateStackMarks(t, `
		BT /Courier 10 Tf ET
		q BT /Helvetica 20 Tf 1 0 0 1 50 700 Tm (Inner) Tj ET Q
		BT 1 0 0 1 50 650 Tm (Outer) Tj ET
	`)

	line := stateStackLine(marks, 650)
	if text := stateStackText(line); text != "Outer" {
		t.Fatalf("text after the block reads %q, want %q", text, "Outer")
	}
	for _, glyph := range line {
		if width := glyph.BBox.Urx - glyph.BBox.Llx; math.Abs(width-courierGlyphWidth) > 0.01 {
			t.Errorf("glyph %q after the block is %.2f wide, want the outer Courier 10 width %.2f", glyph.Text, width, courierGlyphWidth)
		}
	}
}

func TestUnbalancedQKeepsCurrentState(t *testing.T) {
	marks := stateStackMarks(t, `
		BT /Courier 10 Tf ET
		Q Q
		BT 1 0 0 1 50 700 Tm (Text) Tj ET
	`)

	line := stateStackLine(marks, 700)
	if text := stateStackText(line); text != "Text" {
		t.Fatalf("text reads %q, want %q", text, "Text")
	}
	for _, glyph := range line {
		if width := glyph.BBox.Urx - glyph.BBox.Llx; math.Abs(width-courierGlyphWidth) > 0.01 {
			t.Errorf("glyph %q is %.2f wide after an unbalanced Q, want %.2f", glyph.Text, width, courierGlyphWidth)
		}
	}
}

func TestRestoringStateWithoutFontKeepsCurrentFont(t *testing.T) {
	marks := stateStackMarks(t, `
		q BT /Courier 10 Tf 1 0 0 1 50 700 Tm (Inside) Tj ET Q
		BT 1 0 0 1 50 650 Tm (After) Tj ET
	`)

	line := stateStackLine(marks, 650)
	if text := stateStackText(line); text != "After" {
		t.Fatalf("text reads %q, want %q", text, "After")
	}
	for _, glyph := range line {
		if width := glyph.BBox.Urx - glyph.BBox.Llx; math.Abs(width-courierGlyphWidth) > 0.01 {
			t.Errorf("glyph %q is %.2f wide, want %.2f", glyph.Text, width, courierGlyphWidth)
		}
	}
}

func TestCharacterSpacingIsRestoredAfterQ(t *testing.T) {
	marks := stateStackMarks(t, `
		BT /Courier 10 Tf 2 Tc ET
		q BT 5 Tc ET Q
		BT 1 0 0 1 50 700 Tm (AB) Tj ET
	`)

	line := stateStackLine(marks, 700)
	if len(line) != 2 {
		t.Fatalf("got %d glyphs, want 2", len(line))
	}
	if advance := line[1].BBox.Llx - line[0].BBox.Llx; math.Abs(advance-(courierGlyphWidth+2)) > 0.01 {
		t.Errorf("glyph advance is %.2f, want %.2f", advance, courierGlyphWidth+2)
	}
}

func TestThreeNestedBlocksRestoreTheirOwnStates(t *testing.T) {
	marks := stateStackMarks(t, `
		BT /Courier 10 Tf ET
		q BT /Helvetica 12 Tf ET
		q BT /Courier 20 Tf ET
		q Q Q
		BT 1 0 0 1 50 700 Tm (X) Tj ET
		Q
		BT 1 0 0 1 50 650 Tm (Y) Tj ET
	`)

	middle := stateStackLine(marks, 700)
	helveticaX := 0.667 * 12
	if len(middle) != 1 || math.Abs(middle[0].BBox.Urx-middle[0].BBox.Llx-helveticaX) > 0.01 {
		t.Errorf("text after two Q is not set in the Helvetica 12 of the middle block: %+v", middle)
	}
	outer := stateStackLine(marks, 650)
	if len(outer) != 1 || math.Abs(outer[0].BBox.Urx-outer[0].BBox.Llx-courierGlyphWidth) > 0.01 {
		t.Errorf("text after the last Q is not set in the outer Courier 10: %+v", outer)
	}
}
