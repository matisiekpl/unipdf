package extractor

import (
	"strings"
	"testing"
)

func cellSpansMarkdown(page *PageText) string {
	return DocumentMarkdownWithOptions([]*PageText{page}, MarkdownOptions{CellSpans: true})
}

func TestColspanIsMarkedWithEmptyCellWhenCellSpansAreOn(t *testing.T) {
	columns := []float64{100, 200, 300, 400}
	strokes := rowRules(columns, gridRows)
	strokes = append(strokes, verticalRule(100, 640, 700), verticalRule(400, 640, 700), verticalRule(200, 640, 700), verticalRule(300, 640, 680))
	marks := []TextMark{
		testMark("head", 105, 690), testMark("spanning", 205, 690),
		testMark("a2", 105, 670), testMark("b2", 205, 670), testMark("c2", 305, 670),
		testMark("a3", 105, 650), testMark("b3", 205, 650), testMark("c3", 305, 650),
	}
	page := gridPage(strokes, nil, nil, marks...)

	assertContainsRows(t, cellSpansMarkdown(page), "| head | spanning ||", "| a2 | b2 | c2 |")
	assertContainsRows(t, page.Markdown(), "| head | spanning |  |")
}

func TestRowspanBelowHeaderIsMarkedWhenCellSpansAreOn(t *testing.T) {
	strokes := []Stroke{
		horizontalRule(700, 100, 320), horizontalRule(680, 100, 320), horizontalRule(660, 220, 320), horizontalRule(640, 100, 320),
		verticalRule(100, 640, 700), verticalRule(220, 640, 700), verticalRule(320, 640, 700),
	}
	marks := []TextMark{
		testMark("Grupa", 105, 690), testMark("Wartość", 225, 690),
		testMark("Nerki", 105, 670), testMark("n1", 225, 670),
		testMark("n2", 225, 650),
	}
	page := gridPage(strokes, nil, nil, marks...)

	assertContainsRows(t, cellSpansMarkdown(page), "| Nerki | n1 |", "| ^^ | n2 |")
	assertContainsRows(t, page.Markdown(), "|  | n2 |")
}

func TestRowspanDirectlyUnderHeaderStaysEmpty(t *testing.T) {
	strokes := []Stroke{
		horizontalRule(700, 100, 320), horizontalRule(660, 100, 320), horizontalRule(680, 220, 320),
		horizontalRule(681, 105, 201),
		verticalRule(100, 660, 700), verticalRule(220, 660, 700), verticalRule(320, 660, 700),
	}
	marks := []TextMark{
		testMark("Zaburzenia", 105, 690), testMark("nerek", 171, 690), testMark("n1", 225, 690),
		testMark("i", 105, 670), testMark("dróg", 117, 670), testMark("n2", 225, 670),
	}
	page := gridPage(strokes, nil, nil, marks...)

	markdown := cellSpansMarkdown(page)
	assertContainsRows(t, markdown, "|  | n2 |")
	if strings.Contains(markdown, "^^") {
		t.Errorf("a row span reached across the header separator:\n%s", markdown)
	}
}
