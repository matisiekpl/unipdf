package extractor

import "testing"

func TestUnderlinedGreaterThanBecomesGreaterOrEqual(t *testing.T) {
	marks := []TextMark{scriptMark("a", 206.16, 188.64, 199.68, 11), scriptMark(" >", 213.84, 188.64, 199.68, 11), scriptMark("2", 219.96, 188.64, 199.68, 11)}
	marks[1].BBox.Urx = 220.07
	strokes := []Stroke{{X1: 213.84, Y1: 186.96, X2: 220.07, Y2: 186.96}, {X1: 213.84, Y1: 187.44, X2: 220.07, Y2: 187.44}}

	mdUnderlinedComparisons(marks, strokes)

	if got := scriptTexts(marks); got[1] != " ≥" || got[0] != "a" || got[2] != "2" {
		t.Errorf("got %q, want the underlined > read as ≥", got)
	}
}

func TestUnderlinedLessThanBecomesLessOrEqual(t *testing.T) {
	marks := []TextMark{scriptMark("<", 297.46, 513.17, 524.21, 11)}
	marks[0].BBox.Urx = 303.66
	strokes := []Stroke{{X1: 297.46, Y1: 511.8, X2: 303.66, Y2: 511.8}}

	mdUnderlinedComparisons(marks, strokes)

	if marks[0].Text != "≤" {
		t.Errorf("got %q, want ≤", marks[0].Text)
	}
}

func TestComparisonInsideUnderlinedPhraseStaysStrict(t *testing.T) {
	marks := []TextMark{scriptMark(">", 120, 500, 511, 11), scriptMark("5", 126.2, 500, 511, 11)}
	marks[0].BBox.Urx = 126.2
	strokes := []Stroke{{X1: 100, Y1: 498.5, X2: 140, Y2: 498.5}, {X1: 120, Y1: 505, X2: 126.2, Y2: 505}}

	mdUnderlinedComparisons(marks, strokes)

	if marks[0].Text != ">" {
		t.Errorf("got %q, want > under a long underline or a strike-through left alone", marks[0].Text)
	}
}

func TestGreaterOrEqualTakesUnderlineOfPrecedingWord(t *testing.T) {
	strokes := []Stroke{{X1: 213.84, Y1: 187.2, X2: 220.07, Y2: 187.2}, {X1: 300, Y1: 187.2, X2: 347, Y2: 187.2}}
	alone := []mdWord{{s: "do", x0: 200, x1: 210, baseline: 188.64}, {s: "≥", x0: 213.84, x1: 220.07, baseline: 188.64}, {s: "50", x0: 223, x1: 234, baseline: 188.64}}
	inHeading := []mdWord{{s: "wieku", x0: 300, x1: 330, baseline: 188.64}, {s: "≥", x0: 333, x1: 339, baseline: 188.64}, {s: "6", x0: 342, x1: 347, baseline: 188.64}}

	if got := mdRenderLine(alone, strokes); got != "do ≥ 50" {
		t.Errorf("got %q, want ≥ without its own underline", got)
	}
	if got := mdRenderLine(inHeading, strokes); got != "<u>wieku ≥ 6</u>" {
		t.Errorf("got %q, want ≥ inside the underlined heading", got)
	}
}
