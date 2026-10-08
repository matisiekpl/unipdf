package extractor

import (
	"strconv"
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

func TestDocumentMarkdownKeepsNumbersOfTableHeaderInTopMargin(t *testing.T) {
	var pages []*PageText
	for pageNumber := 1; pageNumber <= 4; pageNumber++ {
		marks := []TextMark{
			testMark("Dawkowanie ustala lekarz.", 70, testPageHeight*0.5),
			testMark(string(rune('0'+pageNumber)), 295, testPageHeight*0.0586),
		}
		var strokes []Stroke
		if pageNumber == 2 {
			top := testPageHeight - 30
			strokes = rowRules([]float64{100, 200, 300, 400}, []float64{top, top - 20, top - 40, top - 60})
			for _, x := range []float64{100, 200, 300, 400} {
				strokes = append(strokes, verticalRule(x, top-60, top))
			}
			marks = append(marks,
				testMark("Tydzień", 105, top-10), testMark("1", 205, top-10), testMark("2", 305, top-10),
				testMark("D1", 105, top-30), testMark("D8", 205, top-30), testMark("D15", 305, top-30),
				testMark("D22", 105, top-50), testMark("D29", 205, top-50), testMark("D36", 305, top-50))
		}
		page := testPage(marks...)
		page.strokes = strokes
		pages = append(pages, page)
	}

	markdown := DocumentMarkdown(pages, true)

	if !strings.Contains(markdown, "| Tydzień | 1 | 2 |") {
		t.Errorf("week numbers in the table header were stripped as page numbers:\n%s", markdown)
	}
	if strings.Contains(markdown, "\n3\n") || strings.Contains(markdown, "\n4\n") {
		t.Errorf("page numbers survived:\n%s", markdown)
	}
}

func TestDocumentMarkdownStripsRunningFooterInsideItsBox(t *testing.T) {
	var pages []*PageText
	for pageNumber := 1; pageNumber <= 4; pageNumber++ {
		page := testPage(
			testMark("Dawkowanie ustala lekarz.", 70, testPageHeight*0.5),
			testMark("Confidential", 250, testPageHeight*0.05),
		)
		bottom, top := testPageHeight*0.05-10, testPageHeight*0.05+10
		page.strokes = []Stroke{
			horizontalRule(bottom, 60, 540), horizontalRule(top, 60, 540),
			verticalRule(60, bottom, top), verticalRule(540, bottom, top),
		}
		pages = append(pages, page)
	}

	if markdown := DocumentMarkdown(pages, true); strings.Contains(markdown, "Confidential") {
		t.Errorf("running footer in a box survived:\n%s", markdown)
	}
}

func TestDocumentMarkdownStripsPageNumbersOfExtractStartingLater(t *testing.T) {
	var pages []*PageText
	for index := 0; index < 4; index++ {
		pages = append(pages, testPage(
			testMark("Dawkowanie ustala lekarz.", 70, testPageHeight*0.5),
			testMark(strconv.Itoa(8+index), 295, testPageHeight*0.0586),
		))
	}

	markdown := DocumentMarkdown(pages, true)

	for _, number := range []string{"8", "9", "10", "11"} {
		if strings.Contains(markdown, number) {
			t.Errorf("page number %s of an extract survived:\n%s", number, markdown)
		}
	}
}

func TestShortVersionFooterIsRemovedFromEveryPage(t *testing.T) {
	var pages []*PageText
	for number := 1; number <= 4; number++ {
		pages = append(pages, testPage(
			testMark("Tekst", 70, 600), testMark("strony", 110, 600), testMark(strconv.Itoa(number), 160, 600),
			testMark("V002", 70, 40), testMark(strconv.Itoa(number), 300, 40),
		))
	}

	markdown := DocumentMarkdown(pages, true)

	if strings.Contains(markdown, "V002") {
		t.Errorf("version footer leaked into the text:\n%s", markdown)
	}
}

func TestShortBodyWordInMarginIsKept(t *testing.T) {
	var pages []*PageText
	for number := 1; number <= 4; number++ {
		marks := []TextMark{testMark("Tekst", 70, 600), testMark("strony", 110, 600)}
		if number == 2 {
			marks = append(marks, testMark("Nota", 70, 40))
		}
		pages = append(pages, testPage(marks...))
	}

	if markdown := DocumentMarkdown(pages, true); !strings.Contains(markdown, "Nota") {
		t.Errorf("one-off margin text was removed:\n%s", markdown)
	}
}

func TestDegreeLookalikesAreNormalised(t *testing.T) {
	page := testPage(testMark("Przechowywać", 70, 600), testMark("poniżej", 160, 600), testMark("25", 220, 600), testMark("˚C", 240, 600), testMark("lub", 270, 600), testMark("30", 300, 600), testMark("ºC.", 320, 600))

	markdown := DocumentMarkdown([]*PageText{page}, true)

	if !strings.Contains(markdown, "25 °C") || !strings.Contains(markdown, "30 °C.") || strings.ContainsAny(markdown, "˚º") {
		t.Errorf("got %q", markdown)
	}
}
