package extractor

import (
	"math"
	"testing"
)

func TestClippedTextOverprintingVisibleTextIsDropped(t *testing.T) {
	marks := stateStackMarks(t, `
		BT /Courier 10 Tf 1 0 0 1 50 650 Tm (Zaburzenia) Tj ET
		q 40 690 200 25 re W n
		BT /Courier 10 Tf 1 0 0 1 50 650 Tm (nerwowego) Tj ET
		Q
	`)

	if got := stateStackText(marks); got != "Zaburzenia" {
		t.Errorf("got %q, want the hidden layer over visible text dropped", got)
	}
}

func TestClippedTextWithNothingVisibleUnderIsKept(t *testing.T) {
	marks := stateStackMarks(t, `
		q 40 690 200 25 re W n
		BT /Courier 10 Tf 1 0 0 1 50 700 Tm (Inside) Tj ET
		BT /Courier 10 Tf 1 0 0 1 50 650 Tm (Overflow) Tj ET
		Q
	`)

	if got := stateStackText(marks); got != "InsideOverflow" {
		t.Errorf("got %q, want text cut off by a clip kept when it covers nothing", got)
	}
}

func TestClipEndsWithItsGraphicsState(t *testing.T) {
	marks := stateStackMarks(t, `
		q 40 690 200 25 re W n
		BT /Courier 10 Tf 1 0 0 1 50 650 Tm (Hidden) Tj ET
		Q
		BT /Courier 10 Tf 1 0 0 1 50 650 Tm (Visible) Tj ET
	`)

	if got := stateStackText(marks); got != "Visible" {
		t.Errorf("got %q, want text after Q visible and the clipped layer under it dropped", got)
	}
}

func TestNestedClipsIntersect(t *testing.T) {
	marks := stateStackMarks(t, `
		q 40 640 200 80 re W n
		q 40 690 200 25 re W n
		BT /Courier 10 Tf 1 0 0 1 50 650 Tm (Outer) Tj ET
		Q
		BT /Courier 10 Tf 1 0 0 1 50 650 Tm (Shown) Tj ET
		Q
	`)

	if got := stateStackText(marks); got != "Shown" {
		t.Errorf("got %q, want the inner clip to hide Outer and Q to restore the outer clip", got)
	}
}

func TestGlyphsCutByClipKeepTheirPositions(t *testing.T) {
	marks := stateStackMarks(t, `
		BT /Courier 10 Tf 1 0 0 1 50 700 Tm (abcd) Tj ET
		q 74 690 60 25 re W n
		BT /Courier 10 Tf 1 0 0 1 50 700 Tm (nerwowego) Tj ET
		Q
	`)

	if got := stateStackText(marks); got != "abcdowego" {
		t.Fatalf("got %q, want the clipped glyphs over abcd dropped", got)
	}
	if math.Abs(marks[4].BBox.Llx-(50+4*courierGlyphWidth)) > 0.01 {
		t.Errorf("first visible clipped-run glyph starts at %.2f, want %.2f", marks[4].BBox.Llx, 50+4*courierGlyphWidth)
	}
}

func TestClipAppliedTogetherWithFillHidesText(t *testing.T) {
	marks := stateStackMarks(t, `
		BT /Courier 10 Tf 1 0 0 1 50 650 Tm (Shown) Tj ET
		q 0.9 g 40 690 200 25 re W f
		BT /Courier 10 Tf 1 0 0 1 50 650 Tm (Gone!) Tj ET
		Q
	`)

	if got := stateStackText(marks); got != "Shown" {
		t.Errorf("got %q, want a clip set by W f to hide the overprinted text", got)
	}
}

func TestPathWithoutClipLeavesTextVisible(t *testing.T) {
	marks := stateStackMarks(t, `
		BT /Courier 10 Tf 1 0 0 1 50 650 Tm (Shown) Tj ET
		40 690 200 25 re f
		BT /Courier 10 Tf 1 0 0 1 50 650 Tm (Again) Tj ET
	`)

	if got := stateStackText(marks); got != "ShownAgain" {
		t.Errorf("got %q, want a filled rectangle not to clip text", got)
	}
}
