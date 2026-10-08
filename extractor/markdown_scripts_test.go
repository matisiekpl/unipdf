package extractor

import (
	"strings"
	"testing"

	"github.com/matisiekpl/unipdf/v3/model"
)

func scriptMark(text string, llx, lly, ury, size float64) TextMark {
	return TextMark{
		Text:     text,
		FontSize: size,
		BBox:     model.PdfRectangle{Llx: llx, Lly: lly, Urx: llx + 0.5*size*float64(len([]rune(text))), Ury: ury},
	}
}

func scriptTexts(marks []TextMark) []string {
	texts := make([]string, len(marks))
	for index, mark := range marks {
		texts[index] = mark.Text
	}
	return texts
}

func TestSmallerRaisedGlyphBecomesSuperscriptOnBaseLine(t *testing.T) {
	marks := []TextMark{scriptMark("m", 301.05, 681.92, 692.90, 11), scriptMark("2", 306.55, 686.42, 693.44, 7)}

	mdInlineScripts(marks)

	if marks[1].Text != "²" || marks[0].Text != "m" {
		t.Errorf("got %q, want m followed by ²", scriptTexts(marks))
	}
	if marks[1].BBox.Lly != 681.92 || marks[1].BBox.Ury != 692.90 {
		t.Errorf("superscript kept its own line %.2f..%.2f", marks[1].BBox.Lly, marks[1].BBox.Ury)
	}
}

func TestSameSizeRaisedGlyphAfterBaseBecomesSuperscript(t *testing.T) {
	marks := []TextMark{
		scriptMark("/", 315.32, 251.36, 261.38, 10), scriptMark("m", 318.04, 251.36, 261.38, 10),
		scriptMark("2", 323.04, 255.86, 265.88, 10), scriptMark("r", 333.24, 251.36, 261.38, 10),
	}

	mdInlineScripts(marks)

	if got := scriptTexts(marks); got[1] != "m" || got[2] != "²" || got[0] != "/" || got[3] != "r" {
		t.Errorf("got %q, want only the raised 2 converted", got)
	}
}

func TestSameSizeLoweredGlyphStaysPlain(t *testing.T) {
	marks := []TextMark{scriptMark("C", 100, 500, 510, 10), scriptMark("2", 105, 496, 506, 10)}

	mdInlineScripts(marks)

	if got := scriptTexts(marks); got[1] != "2" {
		t.Errorf("got %q, want a same-size lowered glyph left alone", got)
	}
}

func TestSmallerLoweredRunBecomesSubscript(t *testing.T) {
	marks := []TextMark{
		scriptMark("C", 100, 500, 511, 11),
		scriptMark("m", 105.5, 497, 504, 7), scriptMark("a", 109, 497, 504, 7), scriptMark("x", 112.5, 497, 504, 7),
	}

	mdInlineScripts(marks)

	if got := scriptTexts(marks); got[0] != "C" || got[1] != "ₘ" || got[2] != "ₐ" || got[3] != "ₓ" {
		t.Errorf("got %q, want Cₘₐₓ", got)
	}
}

func TestFootnoteLetterAfterWordBecomesSuperscript(t *testing.T) {
	marks := []TextMark{scriptMark("ść", 100, 500, 511, 11), scriptMark("a", 111, 505, 512, 7)}

	mdInlineScripts(marks)

	if marks[1].Text != "ᵃ" {
		t.Errorf("got %q, want ᵃ", marks[1].Text)
	}
}

func TestSmallTextOnSameBaselineStaysPlain(t *testing.T) {
	marks := []TextMark{scriptMark("Dawka", 100, 500, 511, 11), scriptMark("mg", 127.5, 500, 507, 7)}

	mdInlineScripts(marks)

	if marks[1].Text != "mg" {
		t.Errorf("got %q, want small text on the same baseline left alone", marks[1].Text)
	}
}

func TestSmallRaisedGlyphAwayFromWordStaysPlain(t *testing.T) {
	marks := []TextMark{scriptMark("m", 100, 500, 511, 11), scriptMark("2", 110, 505, 512, 7)}

	mdInlineScripts(marks)

	if marks[1].Text != "2" {
		t.Errorf("got %q, want a raised glyph separated by a space left alone", marks[1].Text)
	}
}

func TestSuperscriptStaysOnItsLineInTableCell(t *testing.T) {
	columns := []float64{100, 200, 300}
	strokes := rowRules(columns, []float64{700, 680, 660})
	for _, x := range columns {
		strokes = append(strokes, verticalRule(x, 660, 700))
	}
	page := gridPage(strokes, nil, nil,
		scriptMark("Dawka", 105, 685, 695, 10), scriptMark("1,3 mg/m", 205, 685, 695, 10), scriptMark("2", 245, 689.5, 699.5, 10),
		scriptMark("a", 105, 665, 675, 10), scriptMark("b", 205, 665, 675, 10))

	assertContainsRows(t, page.Markdown(), "| Dawka | 1,3 mg/m² |")
}

func TestLongLoweredWordInFormulaStaysPlainOnBaseLine(t *testing.T) {
	marks := []TextMark{scriptMark("=", 100, 500, 511, 11)}
	for index, character := range []string{"s", "t", "ę", "ż", "e", "n", "i", "e"} {
		marks = append(marks, scriptMark(character, 105.5+3.5*float64(index), 495, 502, 7))
	}

	mdInlineScripts(marks)

	if got := stateStackText(marks); got != "=stężenie" {
		t.Errorf("got %q, want the formula word left in plain letters", got)
	}
	if marks[1].BBox.Lly != 500 {
		t.Errorf("formula word kept its own line at %.1f", marks[1].BBox.Lly)
	}
}

func TestDigitsOnSeparateLinesInCellAreNotStackedText(t *testing.T) {
	var cell []mdCellMark
	for index, digit := range []string{"1", "2", "3", "4", "5", "6"} {
		cell = append(cell, mdCellMark{x0: 100, x1: 105, y: 700 - 12*float64(index), s: digit})
	}

	if got := mdJoinCell(cell); got != "1<br>2<br>3<br>4<br>5<br>6" {
		t.Errorf("got %q, want digits kept on their lines", got)
	}
}

func TestRotatedLettersInCellAreStacked(t *testing.T) {
	var cell []mdCellMark
	for index, letter := range []string{"D", "a", "w", "k", "a", "s"} {
		cell = append(cell, mdCellMark{x0: 100, x1: 105, y: 700 - 12*float64(index), s: letter})
	}

	if got := mdJoinCell(cell); got != "Dawkas" {
		t.Errorf("got %q, want stacked letters joined", got)
	}
}

func TestMarkdownLeavesPageMarksUntouched(t *testing.T) {
	page := testPage(scriptMark("m", 301.05, 681.92, 692.90, 11), scriptMark("2", 306.55, 686.42, 693.44, 7))

	first := page.Markdown()
	second := page.Markdown()

	if page.Marks().Elements()[1].Text != "2" || first != second {
		t.Errorf("rendering changed the page marks: %q, then %q and %q", page.Marks().Elements()[1].Text, first, second)
	}
}

func TestFootnoteRunWithCommaAndAsteriskKeepsSuperscriptLetters(t *testing.T) {
	marks := []TextMark{
		scriptMark("o", 100, 500, 511, 11), scriptMark("b", 105.5, 504, 511, 7), scriptMark(",", 109, 504, 511, 7), scriptMark("*", 111, 504, 511, 7),
	}

	mdInlineScripts(marks)

	if got := scriptTexts(marks); got[1] != "ᵇ" || got[2] != "," || got[3] != "*" {
		t.Errorf("got %q, want ᵇ,*", got)
	}
}

func TestFootnoteMarkerOpeningLineBecomesSuperscript(t *testing.T) {
	marks := []TextMark{scriptMark("1", 70.94, 712.23, 718.49, 7), scriptMark(" Po", 78.98, 707.4, 719.6, 11)}

	mdInlineScripts(marks)

	if got := scriptTexts(marks); got[0] != "¹" || got[1] != " Po" {
		t.Errorf("got %q, want ¹ before Po", got)
	}
}

func TestSteadyStateSubscriptOfFiveLettersIsConverted(t *testing.T) {
	marks := []TextMark{scriptMark("C", 100, 500, 511, 11), scriptMark("maxss", 105.9, 499, 506, 7)}

	mdInlineScripts(marks)

	if got := scriptTexts(marks); got[1] != "ₘₐₓₛₛ" {
		t.Errorf("got %q, want ₘₐₓₛₛ", got)
	}
}

func TestSpacePrefixedSuperscriptAttachedToNumber(t *testing.T) {
	marks := []TextMark{scriptMark(" 1", 502.86, 122.52, 132.48, 10), scriptMark("0", 507.89, 122.52, 132.48, 10), scriptMark(" 9", 513.24, 126, 132.48, 6.5), scriptMark("/", 516.48, 122.52, 132.48, 10)}

	mdInlineScripts(marks)

	if got := scriptTexts(marks); got[2] != "⁹" {
		t.Errorf("got %q, want ⁹ attached to 10", got)
	}
}

func TestSteadyStateSubscriptWithHyphenIsConverted(t *testing.T) {
	marks := []TextMark{scriptMark("C", 100, 500, 511, 11), scriptMark("max-ss", 105.9, 499, 506, 7)}

	mdInlineScripts(marks)

	if got := scriptTexts(marks); got[1] != "ₘₐₓ₋ₛₛ" {
		t.Errorf("got %q, want ₘₐₓ₋ₛₛ", got)
	}
}

func TestSevenLetterSubscriptStaysPlain(t *testing.T) {
	marks := []TextMark{scriptMark("C", 100, 500, 511, 11), scriptMark("stężenie", 105.9, 499, 506, 7)}

	mdInlineScripts(marks)

	if got := scriptTexts(marks); got[1] != "stężenie" {
		t.Errorf("got %q, want a long lowered word left alone", got)
	}
}

func TestWordSubscriptDroppedByOnePointIsConverted(t *testing.T) {
	marks := []TextMark{
		scriptMark("(", 142.43, 666.70, 677.74, 11), scriptMark("C", 146.16, 666.70, 677.74, 11),
		scriptMark("m", 153.50, 665.74, 672.70, 7), scriptMark("a", 158.91, 665.74, 672.70, 7), scriptMark("x", 162.01, 665.74, 672.70, 7),
		scriptMark(")", 165.50, 666.70, 677.74, 11),
	}
	marks[1].BBox.Urx = 153.53

	mdInlineScripts(marks)

	if got := strings.Join(scriptTexts(marks), ""); got != "(Cₘₐₓ)" {
		t.Errorf("got %q, want (Cₘₐₓ)", got)
	}
}

func TestSmallTextOnSameBaselineIsNotSubscript(t *testing.T) {
	marks := []TextMark{scriptMark("C", 146.16, 666.70, 677.74, 11), scriptMark("max", 153.50, 666.6, 673.6, 7)}
	marks[0].BBox.Urx = 153.53

	mdInlineScripts(marks)

	if got := scriptTexts(marks); got[1] != "max" {
		t.Errorf("got %q, want small text on the base line left alone", got)
	}
}

func TestIsotopeMassNumberBeforeElementIsFullySuperscript(t *testing.T) {
	marks := []TextMark{scriptMark("1", 100, 505, 512, 7), scriptMark("4", 103.5, 505, 512, 7), scriptMark("C", 107, 500, 511, 11)}

	mdInlineScripts(marks)

	if got := strings.Join(scriptTexts(marks), ""); got != "¹⁴C" {
		t.Errorf("got %q, want ¹⁴C", got)
	}
}

func TestHalfLifeSubscriptKeepsSlash(t *testing.T) {
	marks := []TextMark{scriptMark("t", 100, 500, 511, 11), scriptMark("1/2", 105.6, 499, 506, 7)}

	mdInlineScripts(marks)

	if got := strings.Join(scriptTexts(marks), ""); got != "t₁/₂" {
		t.Errorf("got %q, want t₁/₂", got)
	}
}

func TestMixedSubscriptConvertsWhatUnicodeHas(t *testing.T) {
	cases := map[string]string{"1c": "HbA₁c", "1A": "HbA₁A", "1/2β": "HbA₁/₂β"}
	for script, want := range cases {
		marks := []TextMark{scriptMark("HbA", 100, 500, 511, 11), scriptMark(script, 116.5, 499, 506, 7)}

		mdInlineScripts(marks)

		if got := strings.Join(scriptTexts(marks), ""); got != want {
			t.Errorf("%s: got %q, want %q", script, got, want)
		}
	}
}

func TestLoweredWordWithSeveralUnmappableLettersStaysPlain(t *testing.T) {
	for _, word := range []string{"ężć", "1ąę", "zgx2ąę", "do", "ze", "1abcd"} {
		marks := []TextMark{scriptMark("C", 100, 500, 511, 11), scriptMark(word, 105.9, 499, 506, 7)}

		mdInlineScripts(marks)

		if got := scriptTexts(marks); got[1] != word {
			t.Errorf("%s: got %q, want it left alone", word, got)
		}
	}
}

func TestFootnoteAfterNarrowGapAttachesToWord(t *testing.T) {
	marks := []TextMark{
		scriptMark("j", 497.11, 680.02, 691.06, 11), scriptMark("a", 500.14, 680.02, 691.06, 11),
		{Text: " "}, scriptMark(" 1", 506.88, 683.98, 690.94, 7),
	}
	marks[1].BBox.Urx = 505.04

	mdInlineScripts(marks)

	if marks[3].Text != "¹" {
		t.Fatalf("got %q, want the footnote marker as ¹ without the space", scriptTexts(marks))
	}
	if marks[3].BBox.Llx != marks[1].BBox.Urx || marks[3].BBox.Lly != marks[1].BBox.Lly {
		t.Errorf("marker box %+v, want it moved next to the word", marks[3].BBox)
	}
}

func TestFootnoteAfterClosingParenthesis(t *testing.T) {
	marks := []TextMark{
		scriptMark("4", 359.47, 490.75, 501.79, 11), scriptMark(")", 364.87, 490.75, 501.79, 11),
		{Text: " "}, scriptMark(" 1", 371.11, 494.71, 501.67, 7), scriptMark("2", 374.47, 494.71, 501.67, 7),
	}
	marks[1].BBox.Urx, marks[3].BBox.Urx = 368.54, 374.59

	mdInlineScripts(marks)

	if got := strings.Join(scriptTexts(marks), ""); got != "4) ¹²" {
		t.Errorf("got %q, want 4) ¹²", got)
	}
}

func TestSpacedRaisedTextStaysPlainUnlessItLooksLikeFootnote(t *testing.T) {
	cases := []struct {
		name string
		base string
		text string
		size float64
		gap  float64
	}{
		{"wider than a space", "a", " 1", 7, 3.6},
		{"after a digit", "2", " 1", 7, 1.8},
		{"after a comma", ",", " 1", 7, 1.8},
		{"unit text", "a", " mg", 7, 1.8},
		{"three digits", "a", " 123", 7, 1.8},
		{"same size", "a", " 1", 10, 1.8},
	}
	for _, testCase := range cases {
		base := scriptMark(testCase.base, 100, 500, 511, 11)
		marks := []TextMark{base, {Text: " "}, scriptMark(testCase.text, base.BBox.Urx+testCase.gap, 504, 511, testCase.size)}

		mdInlineScripts(marks)

		if marks[2].Text != testCase.text {
			t.Errorf("%s: got %q, want %q left alone", testCase.name, marks[2].Text, testCase.text)
		}
	}
}

func TestCommaSeparatedFootnotesStayOneRun(t *testing.T) {
	marks := []TextMark{
		scriptMark("i", 395.23, 465.43, 476.47, 11), scriptMark("a", 398.23, 465.43, 476.47, 11),
		scriptMark("7", 403.03, 469.39, 476.35, 7), scriptMark(",", 406.39, 469.39, 476.35, 7),
		{Text: " "}, scriptMark(" 1", 409.75, 469.39, 476.35, 7), scriptMark("2", 413.11, 469.39, 476.35, 7),
	}
	marks[3].BBox.Urx, marks[5].BBox.Urx = 408.13, 413.23

	mdInlineScripts(marks)

	if got := strings.Join(scriptTexts(marks), ""); got != "ia⁷,  ¹²" {
		t.Errorf("got %q, want ia⁷,  ¹²", got)
	}
	if marks[5].BBox.Llx != 409.75 {
		t.Errorf("second marker moved to %.2f, want it left in place", marks[5].BBox.Llx)
	}
}

func TestRaisedCommaAndLetterAfterAsterisk(t *testing.T) {
	marks := []TextMark{
		scriptMark("y", 306.84, 163.28, 174.32, 11), scriptMark("*", 312.24, 163.28, 174.32, 11),
		scriptMark(",", 317.76, 167.24, 174.20, 7), {Text: " "}, scriptMark(" a", 322.32, 167.24, 174.20, 7),
	}

	mdInlineScripts(marks)

	if got := strings.Join(scriptTexts(marks), ""); got != "y*,  ᵃ" {
		t.Errorf("got %q, want y*,  ᵃ", got)
	}
	if marks[4].BBox.Llx != 322.32 {
		t.Errorf("letter moved to %.2f, want it left after the comma", marks[4].BBox.Llx)
	}
}

func TestScriptOnItsOwnLineFindsBaseByPosition(t *testing.T) {
	marks := []TextMark{
		scriptMark("k", 328.47, 528.67, 539.71, 11), {Text: "\n"},
		scriptMark("6", 187.94, 519.91, 526.87, 7), {Text: "\n"},
		scriptMark("A", 148.10, 515.95, 526.99, 11), scriptMark("k", 156.02, 515.95, 526.99, 11), scriptMark("a", 161.54, 515.95, 526.99, 11),
		scriptMark("t", 166.44, 515.95, 526.99, 11), scriptMark("y", 169.56, 515.95, 526.99, 11), scriptMark("z", 175.08, 515.95, 526.99, 11),
		scriptMark("j", 179.88, 515.95, 526.99, 11), scriptMark("a", 183.00, 515.95, 526.99, 11),
	}
	marks[11].BBox.Urx = 187.90

	mdInlineScripts(marks)

	if marks[2].Text != "⁶" || marks[2].BBox.Lly != 515.95 {
		t.Errorf("got %q at %.2f, want ⁶ moved onto the line of its word", marks[2].Text, marks[2].BBox.Lly)
	}
}

func TestLineOpeningGlyphWithoutNearbyBaseStaysPlain(t *testing.T) {
	for _, size := range []float64{7, 11} {
		marks := []TextMark{
			scriptMark("k", 328.47, 528.67, 539.71, 11), {Text: "\n"},
			scriptMark("6", 220, 519.91, 526.87, size), {Text: "\n"},
			scriptMark("A", 148.10, 515.95, 526.99, 11), scriptMark("k", 156.02, 515.95, 526.99, 11), scriptMark("a", 161.54, 515.95, 526.99, 11),
			scriptMark("t", 166.44, 515.95, 526.99, 11), scriptMark("y", 169.56, 515.95, 526.99, 11), scriptMark("z", 175.08, 515.95, 526.99, 11),
			scriptMark("j", 179.88, 515.95, 526.99, 11), scriptMark("a", 183.00, 515.95, 526.99, 11),
		}

		mdInlineScripts(marks)

		if marks[2].Text != "6" || marks[2].BBox.Lly != 519.91 {
			t.Errorf("size %v: got %q at %.2f, want it left alone", size, marks[2].Text, marks[2].BBox.Lly)
		}
	}
}

func TestIsotopeAfterNarrowGapStaysWithItsElement(t *testing.T) {
	marks := []TextMark{
		scriptMark("o", 303.00, 748.20, 759.24, 11), {Text: " "},
		scriptMark(" 1", 311.16, 752.16, 759.12, 7), scriptMark("4", 314.64, 752.16, 759.12, 7),
		scriptMark("C", 318.12, 748.20, 759.24, 11),
	}
	marks[0].BBox.Urx, marks[2].BBox.Urx = 308.52, 314.64

	mdInlineScripts(marks)

	if got := strings.Join(scriptTexts(marks), ""); got != "o  ¹⁴C" {
		t.Errorf("got %q, want the isotope number kept apart from the word before it", got)
	}
	if marks[2].BBox.Llx != 311.16 {
		t.Errorf("isotope number moved to %.2f, want it left before its element", marks[2].BBox.Llx)
	}
}

func TestFootnoteFollowedByCommaStaysOneWord(t *testing.T) {
	marks := []TextMark{
		scriptMark("j", 290.00, 500, 511, 11), scriptMark("e", 293.07, 500, 511, 11), {Text: " "},
		scriptMark(" 2", 299.80, 504, 511, 7), scriptMark(",", 303.28, 500, 511, 11),
	}
	marks[1].BBox.Urx, marks[3].BBox.Urx, marks[4].BBox.Urx = 297.97, 303.28, 306.04

	mdInlineScripts(marks)
	words, _ := mdWords(marks, nil)

	if marks[3].Text != "²" || len(words) != 1 {
		t.Errorf("got %q in %d words, want je², as one word", scriptTexts(marks), len(words))
	}
}

func TestFootnoteListAfterNarrowGap(t *testing.T) {
	marks := []TextMark{
		scriptMark("ś", 168.12, 128.46, 138.48, 10), scriptMark("ć", 172.02, 128.46, 138.48, 10), {Text: " "},
		scriptMark(" 2", 178.92, 131.94, 138.42, 6.5), scriptMark(",", 182.16, 131.94, 138.42, 6.5), {Text: " "},
		scriptMark(" 1", 185.40, 131.94, 138.42, 6.5), scriptMark("6", 188.64, 131.94, 138.42, 6.5), scriptMark(",", 191.88, 128.46, 138.48, 10),
	}
	marks[1].BBox.Urx, marks[3].BBox.Urx, marks[4].BBox.Urx, marks[6].BBox.Urx, marks[7].BBox.Urx, marks[8].BBox.Urx = 176.46, 182.16, 183.78, 188.64, 191.88, 194.38

	mdInlineScripts(marks)
	words, _ := mdWords(marks, nil)

	if got := strings.Join(scriptTexts(marks), ""); got != "ść ²,  ¹⁶," {
		t.Errorf("got %q, want ść ²,  ¹⁶,", got)
	}
	if len(words) != 2 {
		t.Errorf("got %d words, want the marker list split once after its comma", len(words))
	}
}

func TestFootnoteListOpeningWrappedLineUsesFollowingComma(t *testing.T) {
	marks := []TextMark{
		scriptMark("a", 247.92, 151.44, 161.46, 10), {Text: "\n"},
		scriptMark("4", 144.72, 143.40, 149.88, 6.5), scriptMark(",", 147.96, 143.40, 149.88, 6.5), {Text: " "},
		scriptMark(" 1", 151.20, 143.40, 149.88, 6.5), scriptMark("6", 154.44, 143.40, 149.88, 6.5), {Text: "\n"},
		scriptMark(",", 157.68, 139.92, 149.94, 10),
	}
	marks[0].BBox.Urx, marks[2].BBox.Urx, marks[3].BBox.Urx, marks[5].BBox.Urx, marks[6].BBox.Urx, marks[8].BBox.Urx = 252.37, 147.96, 149.58, 154.44, 157.68, 160.19

	mdInlineScripts(marks)

	if got := strings.Join(scriptTexts(marks), ""); got != "a\n⁴,  ¹⁶\n," {
		t.Errorf("got %q, want the wrapped marker list as superscripts", got)
	}
}

func TestLongNumberListAfterGapStaysPlain(t *testing.T) {
	for _, text := range []string{" 123", " 1-2", " 1,234", " A"} {
		marks := []TextMark{scriptMark("a", 100, 500, 511, 11), {Text: " "}, scriptMark(text, 107.3, 504, 511, 7)}
		marks[0].BBox.Urx = 105.5

		mdInlineScripts(marks)

		if marks[2].Text != text {
			t.Errorf("got %q, want %q left alone", marks[2].Text, text)
		}
	}
}

func TestRaisedNumeratorAfterSpaceStaysApart(t *testing.T) {
	marks := []TextMark{
		scriptMark("o", 100, 500, 511, 11), {Text: " "},
		scriptMark(" 1", 108.2, 504, 511, 7), scriptMark("/", 111.7, 500, 511, 11), scriptMark("3", 114.8, 498, 505, 7),
	}
	marks[0].BBox.Urx, marks[2].BBox.Urx, marks[3].BBox.Urx = 105.5, 111.7, 114.8

	mdInlineScripts(marks)

	if marks[2].BBox.Llx != 108.2 || !strings.HasPrefix(marks[2].Text, " ") {
		t.Errorf("got %q at %.2f, want the numerator kept apart from the word before it", marks[2].Text, marks[2].BBox.Llx)
	}
}

func TestGlyphTouchingRelocatedMarkerLosesSpace(t *testing.T) {
	marks := []TextMark{
		scriptMark("◊", 300.40, 351.00, 360.00, 9), scriptMark("◊", 304.80, 351.00, 360.00, 9), {Text: "\n"},
		scriptMark("n", 291.88, 344.40, 355.40, 11), scriptMark("i", 297.48, 344.40, 355.40, 11),
		{Text: " "}, scriptMark(" ,", 309.20, 344.40, 355.40, 11), {Text: " "}, scriptMark(" k", 314.80, 344.40, 355.40, 11),
	}
	marks[0].BBox.Urx, marks[1].BBox.Urx, marks[4].BBox.Urx, marks[6].BBox.Urx = 304.85, 309.25, 300.53, 311.95

	mdInlineScripts(marks)

	if marks[6].Text != "," || marks[8].Text != " k" {
		t.Errorf("got %q, want the comma attached to the marker and the next word kept apart", scriptTexts(marks))
	}
}

func TestRealSpaceAfterSubscriptIsKept(t *testing.T) {
	marks := []TextMark{
		scriptMark("α", 222.65, 500, 511, 11), scriptMark("1", 228.05, 499, 506, 7), {Text: " "}, scriptMark(" a", 232.61, 500, 511, 11),
	}
	marks[0].BBox.Urx, marks[1].BBox.Urx = 228.43, 231.53

	mdInlineScripts(marks)

	if marks[1].Text != "₁" || marks[3].Text != " a" {
		t.Errorf("got %q, want the space before the next word kept", scriptTexts(marks))
	}
}

func TestPunctuationTouchingSubscriptLosesSpace(t *testing.T) {
	marks := []TextMark{
		scriptMark("B", 255.49, 657.95, 668.99, 11), {Text: " "}, scriptMark(" 6", 263.40, 657.00, 663.96, 7), {Text: " "}, scriptMark(" :", 267.48, 657.96, 669.00, 11),
	}
	marks[0].BBox.Urx, marks[2].BBox.Urx = 262.86, 266.88

	mdInlineScripts(marks)

	if marks[2].Text != "₆" || marks[4].Text != ":" {
		t.Errorf("got %q, want B₆:", scriptTexts(marks))
	}
}
