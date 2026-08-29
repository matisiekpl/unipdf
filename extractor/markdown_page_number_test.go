package extractor

import (
	"strings"
	"testing"

	"github.com/matisiekpl/unipdf/v3/model"
)

const (
	testPageWidth  = 595.32
	testPageHeight = 841.92
)

func testMark(text string, llx, cy float64) TextMark {
	return TextMark{
		Text: text,
		BBox: model.PdfRectangle{Llx: llx, Lly: cy - 5, Urx: llx + 6*float64(len(text)), Ury: cy + 5},
	}
}

func testPage(marks ...TextMark) *PageText {
	return &PageText{
		viewMarks: marks,
		pageSize:  model.PdfRectangle{Llx: 0, Lly: 0, Urx: testPageWidth, Ury: testPageHeight},
	}
}

func TestMarginBandCoversFooterBelowElevenPercent(t *testing.T) {
	page := testPage()
	for _, share := range []float64{0.05, 0.0819, 0.0889, 0.0950} {
		if !page.marginBand(testPageHeight * share) {
			t.Errorf("footer at %.2f%% of page height must count as margin", share*100)
		}
	}
	if page.marginBand(testPageHeight * 0.20) {
		t.Error("body text at 20% of page height must not count as margin")
	}
}

func TestMarginSegmentsSplitPageNumberFromRunningLabel(t *testing.T) {
	footer := testPageHeight * 0.05
	page := testPage(
		testMark("NL/H/1575/002/IB/046", 108, footer),
		testMark("7", 494, footer),
	)

	var numbers []int
	for _, line := range page.marginLines() {
		if value, ok := mdPageNumberValue(line.text); ok {
			numbers = append(numbers, value)
		}
	}

	if len(numbers) != 1 || numbers[0] != 7 {
		t.Fatalf("page number next to a running label must be recognised, got %v", numbers)
	}
}

func TestMarginSegmentsKeepWordsOfOneLabelTogether(t *testing.T) {
	footer := testPageHeight * 0.05
	page := testPage(
		testMark("Charakterystyka", 100, footer),
		testMark("Produktu", 200, footer),
		testMark("Leczniczego", 260, footer),
	)

	for _, line := range page.marginLines() {
		if _, ok := mdPageNumberValue(line.text); ok {
			t.Fatalf("a running label must not split into a page number, got %q", line.text)
		}
	}
}

func TestDocumentMarkdownStripsConstantFooterNumber(t *testing.T) {
	var pages []*PageText
	for pageNumber := 1; pageNumber <= 4; pageNumber++ {
		pages = append(pages, testPage(
			testMark("Produkt stosuje sie na skore.", 70, testPageHeight*0.5),
			testMark("4", 295, testPageHeight*0.0586),
		))
	}

	markdown := DocumentMarkdown(pages, true)

	if strings.Contains(markdown, "4") {
		t.Errorf("a footer stamping the same number on every page must be dropped:\n%s", markdown)
	}
	if !strings.Contains(markdown, "Produkt stosuje sie na skore.") {
		t.Errorf("body text was dropped:\n%s", markdown)
	}
}

func TestDocumentMarkdownKeepsNumberThatIsNotFooterFurniture(t *testing.T) {
	var pages []*PageText
	for pageNumber := 1; pageNumber <= 6; pageNumber++ {
		marks := []TextMark{testMark("Sredni wynik punktacji", 70, testPageHeight*0.5)}
		if pageNumber == 2 {
			marks = append(marks, testMark("65", 295, testPageHeight*0.5-20))
		}
		pages = append(pages, testPage(marks...))
	}

	if markdown := DocumentMarkdown(pages, true); !strings.Contains(markdown, "65") {
		t.Errorf("a number in the body must survive:\n%s", markdown)
	}
}

func TestDocumentMarkdownStripsFooterOutsideTheOldBand(t *testing.T) {
	var pages []*PageText
	for pageNumber := 1; pageNumber <= 4; pageNumber++ {
		pages = append(pages, testPage(
			testMark("Dawkowanie ustala lekarz.", 70, testPageHeight*0.5),
			testMark(string(rune('0'+pageNumber)), 295, testPageHeight*0.0889),
		))
	}

	markdown := DocumentMarkdown(pages, true)

	for _, line := range strings.Split(markdown, "\n") {
		if strings.TrimSpace(line) != "" && strings.Trim(strings.TrimSpace(line), "0123456789") == "" {
			t.Errorf("page number survived as %q in:\n%s", line, markdown)
		}
	}
	if !strings.Contains(markdown, "Dawkowanie ustala lekarz.") {
		t.Errorf("body text was dropped:\n%s", markdown)
	}
}
