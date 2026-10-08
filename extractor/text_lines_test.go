package extractor

import (
	"testing"

	"github.com/matisiekpl/unipdf/v3/model"
)

func pathPage(t *testing.T, contents string) *PageText {
	t.Helper()
	extractor := Extractor{resources: model.NewPdfPageResources(), contents: contents, mediaBox: r(0, 0, 600, 800)}
	pageText, _, _, err := extractor.ExtractPageText()
	if err != nil {
		t.Fatalf("extracting page: %v", err)
	}
	return pageText
}

func TestStrokedPathBecomesRule(t *testing.T) {
	page := pathPage(t, `0 g 100 500 m 300 500 l S`)

	if len(page.strokes) != 1 {
		t.Fatalf("got %d strokes, want 1", len(page.strokes))
	}
}

func TestThinFillBecomesRuleWhateverItsColour(t *testing.T) {
	black := pathPage(t, `0 g 100 500 200 0.5 re f`)
	white := pathPage(t, `1 g 100 500 200 0.5 re f`)

	if len(black.strokes) != 4 {
		t.Errorf("thin black fill gave %d strokes, want 4", len(black.strokes))
	}
	if len(white.strokes) != 4 {
		t.Errorf("thin white fill drawn as a separator gave %d strokes, want 4", len(white.strokes))
	}
}

func TestWideShadedFillIsCellCandidateNotRule(t *testing.T) {
	page := pathPage(t, `0.9 g 100 500 200 20 re f`)

	if len(page.strokes) != 0 {
		t.Errorf("shaded area gave %d strokes, want none", len(page.strokes))
	}
	if len(page.cellRects) != 1 || len(page.whiteCellRects) != 0 {
		t.Errorf("shaded area recorded as %d cell and %d white cell rectangles, want 1 and 0", len(page.cellRects), len(page.whiteCellRects))
	}
}

func TestWideWhiteFillIsWhiteCellCandidate(t *testing.T) {
	for name, contents := range map[string]string{
		"gray": `1 g 100 500 200 20 re f`,
		"rgb":  `1 1 1 rg 100 500 200 20 re f`,
		"cmyk": `0 0 0 0 k 100 500 200 20 re f`,
	} {
		page := pathPage(t, contents)
		if len(page.strokes) != 0 || len(page.cellRects) != 0 || len(page.whiteCellRects) != 1 {
			t.Errorf("%s white fill: %d strokes, %d cell, %d white cell rectangles, want 0, 0, 1", name, len(page.strokes), len(page.cellRects), len(page.whiteCellRects))
		}
	}
}

func TestClipRectangleIsCellCandidateNotRule(t *testing.T) {
	page := pathPage(t, `q 100 500 200 20 re W n Q`)

	if len(page.strokes) != 0 {
		t.Errorf("clip gave %d strokes, want none", len(page.strokes))
	}
	if len(page.cellRects) != 1 {
		t.Errorf("clip recorded %d cell rectangles, want 1", len(page.cellRects))
	}
}

func TestUnpaintedPathIsDiscarded(t *testing.T) {
	page := pathPage(t, `100 500 m 300 500 l n 100 400 200 20 re n`)

	if len(page.strokes) != 0 || len(page.cellRects) != 0 {
		t.Errorf("unpainted path left %d strokes and %d cell rectangles", len(page.strokes), len(page.cellRects))
	}
}

func TestEachThinSubpathOfOneFillBecomesRule(t *testing.T) {
	page := pathPage(t, `0 g
		100 500 m 100.5 500 l 100.5 600 l 100 600 l h
		300 500 m 300.5 500 l 300.5 600 l 300 600 l h
		f*`)

	if len(page.strokes) != 8 {
		t.Errorf("two thin bars in one fill gave %d strokes, want 8", len(page.strokes))
	}
}

func TestWideNonRectangularFillIsNeitherRuleNorCell(t *testing.T) {
	page := pathPage(t, `0.9 g 100 500 m 300 500 l 300 520 l 100 520 l h f`)

	if len(page.strokes) != 0 || len(page.cellRects) != 0 {
		t.Errorf("wide polygon left %d strokes and %d cell rectangles", len(page.strokes), len(page.cellRects))
	}
}

func TestIsWhite(t *testing.T) {
	cases := []struct {
		color model.PdfColor
		white bool
	}{
		{model.NewPdfColorDeviceGray(1), true},
		{model.NewPdfColorDeviceGray(0.93), false},
		{model.NewPdfColorDeviceRGB(1, 1, 1), true},
		{model.NewPdfColorDeviceRGB(1, 1, 0.5), false},
		{model.NewPdfColorDeviceCMYK(0, 0, 0, 0), true},
		{model.NewPdfColorDeviceCMYK(0, 0, 0, 1), false},
		{nil, false},
	}
	for _, testCase := range cases {
		if got := isWhite(testCase.color); got != testCase.white {
			t.Errorf("isWhite(%v) = %v, want %v", testCase.color, got, testCase.white)
		}
	}
}
