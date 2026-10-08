package extractor

import (
	"strings"
	"testing"
)

func markerLine(markerX float64, marker string, textX float64, text string) []mdWord {
	return append([]mdWord{{s: marker, x0: markerX, x1: markerX + 5}}, proseLine(textX, text)...)
}

func TestSubBulletsIndentedPastParentTextAreNested(t *testing.T) {
	got := renderProse(500,
		markerLine(70.9, "•", 88.9, "Zakażenia wywoływane przez paciorkowce, np.:"),
		markerLine(88.9, "−", 106.9, "zakażenia górnych dróg oddechowych;"),
		markerLine(88.9, "−", 106.9, "zapalenie osierdzia;"),
		markerLine(70.9, "•", 88.9, "Zakażenia skóry i tkanek miękkich."),
	)

	want := "- Zakażenia wywoływane przez paciorkowce, np.:\n  - zakażenia górnych dróg oddechowych;\n  - zapalenie osierdzia;\n- Zakażenia skóry i tkanek miękkich."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestThirdLevelBulletsAreIndentedTwice(t *testing.T) {
	got := renderProse(500,
		markerLine(70, "•", 88, "Stany prowadzące do zatrzymania sodu, u pacjentów z:"),
		markerLine(88, "-", 106, "hiperaldosteronizmem wtórnym, związanym na przykład z:"),
		markerLine(106, "o", 124, "nadciśnieniem,"),
		markerLine(106, "o", 124, "marskością wątroby,"),
		markerLine(88, "-", 106, "zespołem Cushinga,"),
	)

	for _, line := range []string{"- Stany", "\n  - hiperaldosteronizmem", "\n    - nadciśnieniem,", "\n    - marskością", "\n  - zespołem Cushinga,"} {
		if !strings.Contains(got, line) {
			t.Errorf("got %q, missing %q", got, line)
		}
	}
}

func TestShiftedMarkerWithAlignedTextStaysOnSameLevel(t *testing.T) {
	got := renderProse(500,
		markerLine(57.6, "•", 93, "Ciąża i okres karmienia piersią (patrz punkt 4.6)."),
		markerLine(75.6, "•", 93, "Jednoczesne podawanie gemfibrozylu (patrz punkt 4.5)."),
	)

	if strings.Contains(got, "  - ") || strings.Count(got, "\n- ") != 1 {
		t.Errorf("got %q, want two items on one level", got)
	}
}

func TestDeeperMarkerWithShallowerTextStaysOnSameLevel(t *testing.T) {
	got := renderProse(500,
		markerLine(57.6, "•", 93, "Pierwszy punkt listy kończy się kropką."),
		markerLine(66, "-", 80, "Drugi punkt listy kończy się kropką."),
	)

	if strings.Contains(got, "  - ") {
		t.Errorf("got %q, want no nesting when the text does not move right", got)
	}
}

func TestParagraphBreakEndsNesting(t *testing.T) {
	lines := [][]mdWord{
		markerLine(70, "•", 88, "Pierwszy punkt listy kończy się dwukropkiem:"),
		markerLine(88, "-", 106, "podpunkt;"),
		proseLine(70, "Akapit."),
		markerLine(88, "-", 106, "nowa lista po akapicie."),
	}

	got := mdRenderProse(lines, []float64{700, 687, 640, 620}, nil, 500)

	if !strings.Contains(got, "\n  - podpunkt;") || !strings.Contains(got, "\n- nowa lista") {
		t.Errorf("got %q, want the list after a paragraph to start at the top level", got)
	}
}

func TestLetterOWithTabGapIsBullet(t *testing.T) {
	got := renderProse(500,
		proseLine(57, "Przeciwwskazania obejmują:"),
		markerLine(92.7, "o", 110.7, "Wszystkie powiązane z leczeniem zdarzenia niepożądane."),
		markerLine(92.7, "o", 110.7, "Wszystkie ciężkie zdarzenia niepożądane."),
	)

	if strings.Count(got, "\n- Wszystkie") != 2 || strings.Contains(got, "o Wszystkie") {
		t.Errorf("got %q, want both o lines as bullets", got)
	}
}

func TestPrepositionOAtLineStartIsNotBullet(t *testing.T) {
	got := renderProse(500,
		proseLine(56.7, "Każda kapsułka zawiera substancję pomocniczą o znanym działaniu, mieszaninę"),
		proseLine(56.7, "o składzie: laktoza jednowodna i sacharoza."),
	)

	if strings.Contains(got, "- składzie") || !strings.Contains(got, "mieszaninę o składzie:") {
		t.Errorf("got %q, want the preposition kept in the sentence", got)
	}
}

func TestJustifiedLineStartingWithOIsNotBullet(t *testing.T) {
	line := []mdWord{{s: "o", x0: 56, x1: 61}, {s: "etiologii", x0: 69, x1: 110}, {s: "innej", x0: 118, x1: 143}, {s: "niż", x0: 151, x1: 166}, {s: "cukrzycowa", x0: 174, x1: 224}}
	got := renderProse(500, proseLine(56, "Jawna nefropatia kłębuszkowa ze szczególnym uwzględnieniem postaci"), line)

	if strings.Contains(got, "- etiologii") {
		t.Errorf("got %q, want stretched word spacing not taken for a bullet gap", got)
	}
}

func TestLetteredLegendKeepsEachEntryAndLetterO(t *testing.T) {
	got := renderProse(500,
		markerLine(88.9, "n", 106.9, "Ból jamy ustnej i gardła oraz ból gardła i krtani"),
		markerLine(88.9, "o", 106.9, "Zapalenie jamy ustnej i afty jamy ustnej"),
		markerLine(88.9, "p", 106.9, "Ból brzucha, ból w podbrzuszu i ból w nadbrzuszu"),
		markerLine(88.9, "u", 106.9, "Łuszczycopodobne zapalenie skóry, wysypka złuszczająca,"),
		proseLine(106.9, "wysypka grudkowa i swędząca wysypka"),
	)

	want := "n Ból jamy ustnej i gardła oraz ból gardła i krtani\n\no Zapalenie jamy ustnej i afty jamy ustnej\n\np Ból brzucha, ból w podbrzuszu i ból w nadbrzuszu\n\nu Łuszczycopodobne zapalenie skóry, wysypka złuszczająca, wysypka grudkowa i swędząca wysypka"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSingleLetterLineAloneIsNotLegend(t *testing.T) {
	got := renderProse(500,
		proseLine(70, "Lek należy przyjmować codziennie o tej samej porze dnia, niezależnie od posiłków"),
		markerLine(70, "a", 88, "Także w okresie leczenia podtrzymującego."),
	)

	if strings.Contains(got, "\n\na Także") {
		t.Errorf("got %q, want a lone wide-gap letter left in its paragraph", got)
	}
}

func TestLegendEntriesAreNotJoinedAcrossParagraphs(t *testing.T) {
	previous := "b Zapalenie oskrzeli, zapalenie dolnych dróg oddechowych, zapalenie płuc i zapalenie dróg oddechowych"
	if mdShouldJoin(previous, "c Ropień, ropień w obrębie kończyny") {
		t.Errorf("legend entries joined")
	}
	if !mdShouldJoin("Produkt leczniczy należy stosować ostrożnie u pacjentów z zaburzeniami czynności nerek oraz", "u pacjentów w podeszłym wieku.") {
		t.Errorf("ordinary continuation not joined")
	}
}

func TestNestedItemIsNotJoinedWithFollowingParagraph(t *testing.T) {
	previous := "  - zapobieganie nawrotom epizodów maniakalnych lub depresyjnych u pacjentów z zaburzeniem afektywnym"
	if mdShouldJoin(previous, "dwubiegunowym, u których uzyskano reakcję na leczenie") {
		t.Errorf("nested list item joined with the next paragraph")
	}
}

func TestWingdingsLetterAtLineStartIsBullet(t *testing.T) {
	widths := "/FirstChar 32 /LastChar 126 /Widths [" + strings.Repeat("500 ", 95) + "]"
	pdf := syntheticPDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R /F2 6 0 R /F3 7 0 R >> >> /Contents 4 0 R >>",
		syntheticStream("", "BT /F1 11 Tf 70 700 Td (s) Tj ET BT /F2 11 Tf 88 700 Td (w leczeniu schizofrenii;) Tj ET "+
			"BT /F1 11 Tf 70 686 Td (s) Tj ET BT /F2 11 Tf 88 686 Td (w leczeniu manii.) Tj ET "+
			"BT /F3 11 Tf 70 650 Td (a) Tj ET BT /F2 11 Tf 77 650 Td (= 0,05) Tj ET"),
		"<< /Type /Font /Subtype /TrueType /BaseFont /Wingdings-Regular " + widths + " /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Symbol >>",
	})

	markdown := DocumentMarkdown([]*PageText{syntheticPageText(t, pdf)}, false)

	if !strings.Contains(markdown, "- w leczeniu schizofrenii;\n- w leczeniu manii.") {
		t.Errorf("got %q, want the Wingdings glyphs as bullets", markdown)
	}
	if !strings.Contains(markdown, "α") || strings.Contains(markdown, "•") {
		t.Errorf("got %q, want Symbol letters kept", markdown)
	}
}

func wordMark(text string, llx, lly, ury, size float64) TextMark {
	mark := scriptMark(text, llx, lly, ury, size)
	mark.Text = " " + text
	return mark
}

func TestRaisedLegendLabelJoinsTheLineBelow(t *testing.T) {
	marks := []TextMark{
		wordMark("§", 77.4, 773.28, 784.32, 11), wordMark("Patrz", 105.36, 773.28, 784.32, 11), wordMark("także", 135, 773.28, 784.32, 11),
		wordMark("a", 77.4, 762.36, 771.36, 9),
		wordMark("W", 105.36, 757.32, 768.36, 11), wordMark("tym", 118.54, 757.32, 768.36, 11), wordMark("zakażenia", 140, 757.32, 768.36, 11),
		wordMark("b", 77.4, 750.12, 759.12, 9),
		wordMark("W", 105.36, 745.08, 756.12, 11), wordMark("tym", 118.54, 745.08, 756.12, 11), wordMark("krwawienia", 140, 745.08, 756.12, 11),
	}

	blocks := mdReconstructBlocks(marks, nil, 200)

	if len(blocks) != 1 || blocks[0].text != "§ Patrz także\n\na W tym zakażenia\n\nb W tym krwawienia" {
		t.Errorf("got %+v, want each raised label in front of its entry", blocks)
	}
}

func TestRaisedLabelFarAboveOrRightOfTextIsNotAttached(t *testing.T) {
	for _, label := range []TextMark{wordMark("a", 77.4, 790, 799, 9), wordMark("a", 120, 762.36, 771.36, 9), wordMark("a", 101, 762.36, 771.36, 9), wordMark("a", 77.4, 752, 761, 9)} {
		marks := []TextMark{
			label,
			wordMark("W", 105.36, 757.32, 768.36, 11), wordMark("tym", 118.54, 757.32, 768.36, 11), wordMark("zakażenia", 140, 757.32, 768.36, 11),
			wordMark("b", 77.4, 745.08, 756.12, 11),
			wordMark("W", 105.36, 745.08, 756.12, 11), wordMark("tym", 118.54, 745.08, 756.12, 11), wordMark("krwawienia", 140, 745.08, 756.12, 11),
		}

		blocks := mdReconstructBlocks(marks, nil, 200)

		if len(blocks) != 1 || strings.Contains(blocks[0].text, "\n\nb W tym") {
			t.Errorf("label at %.1f,%.1f: got %+v, want no legend built from a detached label", label.BBox.Llx, label.BBox.Lly, blocks)
		}
	}
}

func TestSymbolLegendEntriesAreSeparateParagraphs(t *testing.T) {
	got := renderProse(500,
		markerLine(77, "†", 105, "Działanie niepożądane notowane tylko po wstrzyknięciu"),
		markerLine(77, "§", 105, "Rzadko zgłaszano po zbyt szybkim podaniu dożylnym"),
	)

	if got != "† Działanie niepożądane notowane tylko po wstrzyknięciu\n\n§ Rzadko zgłaszano po zbyt szybkim podaniu dożylnym" {
		t.Errorf("got %q, want one paragraph per footnote", got)
	}
}

func TestHangingLabelMergeIntoLineBelow(t *testing.T) {
	text := []TextMark{wordMark("W", 105.36, 757.32, 768.36, 11), wordMark("tym", 118.54, 757.32, 768.36, 11), wordMark("zakażenia", 140, 757.32, 768.36, 11)}
	cases := []struct {
		name   string
		label  TextMark
		merged bool
	}{
		{"raised label with gap", wordMark("a", 77.4, 762.36, 771.36, 9), true},
		{"label below the text", wordMark("a", 77.4, 753.5, 762.5, 9), false},
		{"label touching the text", wordMark("a", 101, 762.36, 771.36, 9), false},
		{"label far above", wordMark("a", 77.4, 775, 784, 9), false},
		{"word instead of label", wordMark("ab", 77.4, 762.36, 771.36, 9), false},
	}
	for _, testCase := range cases {
		lines, _ := mdLines(append([]TextMark{testCase.label}, text...))

		merged := len(lines) == 1 && lines[0][0].s == strings.TrimSpace(testCase.label.Text)
		if merged != testCase.merged {
			t.Errorf("%s: got %d lines, merged %v, want merged %v", testCase.name, len(lines), merged, testCase.merged)
		}
	}
}
