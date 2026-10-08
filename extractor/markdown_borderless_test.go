package extractor

import (
	"strings"
	"testing"
)

func borderlessLine(y float64, cells ...string) []TextMark {
	var marks []TextMark
	for index, cell := range cells {
		if cell == "" {
			continue
		}
		x := 70.0
		if index > 0 {
			x = 251 + 4.3*float64(index-1)
		}
		for _, word := range strings.Fields(cell) {
			marks = append(marks, testMark(word, x, y))
			x += 6*float64(len(word)) + 3
		}
	}
	return marks
}

func borderlessMarkdown(lines ...[]TextMark) string {
	var marks []TextMark
	for _, line := range lines {
		marks = append(marks, line...)
	}
	return testPage(marks...).Markdown()
}

func TestAlignedColumnsWithoutRulesBecomeTable(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "Zaburzenia metabolizmu i odżywiania"),
		borderlessLine(687, "Niezbyt często", "Odwodnienie"),
		borderlessLine(661, "Zaburzenia układu nerwowego"),
		borderlessLine(648, "Bardzo rzadko", "Zawroty głowy"),
		borderlessLine(635, "Niezbyt często", "Parestezje"),
		borderlessLine(622, "Nieznana", "Omdlenie"),
	)

	assertContainsRows(t, markdown,
		"| Zaburzenia metabolizmu i odżywiania |  |",
		"| Niezbyt często | Odwodnienie |",
		"| Zaburzenia układu nerwowego |  |",
		"| Bardzo rzadko | Zawroty głowy |",
		"| Nieznana | Omdlenie |")
}

func TestColumnStartsDriftingByFewPointsStayOneColumn(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "Niezbyt często", "Odwodnienie"),
		borderlessLine(687, "Bardzo rzadko", "Zawroty głowy"),
		borderlessLine(674, "Niezbyt często", "", "Kaszel"),
		borderlessLine(661, "Nieznana", "", "Obrzęk płuc"),
	)

	assertContainsRows(t, markdown, "| Niezbyt często | Odwodnienie |", "| Niezbyt często | Kaszel |", "| Nieznana | Obrzęk płuc |")
}

func TestWrappedCellLineJoinsItsRow(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "Zaburzenia oka"),
		borderlessLine(687, "Niezbyt często", "Nieostre widzenie"),
		borderlessLine(674, "Badania", "Zwiększenie stężenia kwasu,"),
		borderlessLine(661, "Nieznana", "zwiększenie bilirubiny,"),
		borderlessLine(648, "", "hipokaliemia"),
		borderlessLine(635, "", "Rzadko: wysypka"),
	)

	assertContainsRows(t, markdown, "| Badania | Zwiększenie stężenia kwasu, |", "| Nieznana | zwiększenie bilirubiny,<br>hipokaliemia |", "|  | Rzadko: wysypka |")
}

func TestCaptionSeparatedByBlankLineStaysProse(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(740, "Tabela 1. Działania niepożądane"),
		borderlessLine(700, "Zaburzenia oka"),
		borderlessLine(687, "Niezbyt często", "Nieostre widzenie"),
		borderlessLine(674, "Rzadko", "Zapalenie spojówek"),
		borderlessLine(661, "Nieznana", "Jaskra"),
	)

	if strings.Contains(markdown, "| Tabela 1.") || !strings.Contains(markdown, "| Zaburzenia oka |  |") {
		t.Errorf("caption or heading misplaced:\n%s", markdown)
	}
}

func TestTwoAlignedLinesAreNotTable(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "Niezbyt często", "Odwodnienie"),
		borderlessLine(687, "Bardzo rzadko", "Zawroty głowy"),
	)

	if strings.Contains(markdown, "|") {
		t.Errorf("two lines made a table:\n%s", markdown)
	}
}

func TestNumberedHeadingsAreNotTable(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "6.2", "Niezgodności farmaceutyczne"),
		borderlessLine(687, "Nie dotyczy."),
		borderlessLine(674, "6.3", "Okres ważności"),
		borderlessLine(661, "3 lata"),
		borderlessLine(648, "6.4", "Specjalne środki ostrożności"),
	)

	if strings.Contains(markdown, "|") {
		t.Errorf("numbered headings made a table:\n%s", markdown)
	}
}

func TestBulletListIsNotTable(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "•", "w nadwrażliwości na substancję czynną;"),
		borderlessLine(687, "•", "u noworodków i niemowląt;"),
		borderlessLine(674, "•", "w ospie wietrznej."),
	)

	if strings.Contains(markdown, "|") {
		t.Errorf("bullet list made a table:\n%s", markdown)
	}
}

func TestProseWithOrdinaryWordGapsIsNotTable(t *testing.T) {
	var lines [][]TextMark
	for index := 0; index < 5; index++ {
		lines = append(lines, []TextMark{
			testMark("Produkt", 70, 700-13*float64(index)), testMark("stosuje", 115, 700-13*float64(index)),
			testMark("się", 161, 700-13*float64(index)), testMark("na", 184, 700-13*float64(index)),
		})
	}

	if markdown := borderlessMarkdown(lines...); strings.Contains(markdown, "|") {
		t.Errorf("prose made a table:\n%s", markdown)
	}
}

func TestPipeInBorderlessCellDoesNotSplitIt(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "Tabletka 1:", "oznaczone G|1"),
		borderlessLine(687, "Tabletka 2:", "oznaczone G|2"),
		borderlessLine(674, "Tabletka 3:", "oznaczone G|3"),
	)

	assertContainsRows(t, markdown, "| Tabletka 1: | oznaczone G¦1 |")
}

func TestFootnotesWithSymbolMarkersAreNotTable(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "*", "Obserwowano u pacjentów z niewydolnością nerek"),
		borderlessLine(687, "", "otrzymujących duże dawki."),
		borderlessLine(674, "**", "Na podstawie danych po wprowadzeniu do obrotu."),
		borderlessLine(661, "***", "Częstość nieznana."),
	)

	if strings.Contains(markdown, "|") {
		t.Errorf("footnote list made a table:\n%s", markdown)
	}
}

func TestLettersAsListMarkersAreNotTable(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "a", "Dane z badania klinicznego."),
		borderlessLine(687, "b", "Dane po wprowadzeniu do obrotu."),
		borderlessLine(674, "c", "Częstość obliczona na podstawie badań."),
	)

	if strings.Contains(markdown, "|") {
		t.Errorf("lettered footnotes made a table:\n%s", markdown)
	}
}

func TestValueCentredBetweenColumnsEndsTable(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "Odpowiedź", "45%"),
		borderlessLine(687, "Mediana", "12 miesięcy"),
		borderlessLine(674, "Przeżycie", "80%"),
		[]TextMark{testMark("0,00201", 290, 661)},
	)

	if strings.Contains(markdown, "| 0,00201") || strings.Contains(markdown, "|  | 0,00201 |") {
		t.Errorf("centred value was put into a column:\n%s", markdown)
	}
	assertContainsRows(t, markdown, "| Odpowiedź | 45% |", "| Przeżycie | 80% |")
}

func TestSentenceTailAboveTableStaysProse(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(713, "i swędząca wysypka."),
		borderlessLine(700, "Niezbyt często", "Odwodnienie"),
		borderlessLine(687, "Bardzo rzadko", "Zawroty głowy"),
		borderlessLine(674, "Nieznana", "Omdlenie"),
	)

	if strings.Contains(markdown, "| i swędząca") {
		t.Errorf("sentence tail pulled into the table:\n%s", markdown)
	}
}

func TestNumberedSectionHeadingEndsBorderlessTable(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(726, "Tel.:", "+48 22 000 00 00"),
		borderlessLine(713, "Faks:", "+48 22 000 00 01"),
		borderlessLine(700, "E-mail:", "biuro@example.pl"),
		borderlessLine(687, "7.", "PODMIOT ODPOWIEDZIALNY"),
		borderlessLine(674, "Pharma", "ul. Długa 1"),
	)

	if strings.Contains(markdown, "| 7. |") || strings.Contains(markdown, "| Pharma |") {
		t.Errorf("section heading pulled into the table:\n%s", markdown)
	}
	assertContainsRows(t, markdown, "| Tel.: | +48 22 000 00 00 |")
}

func TestWrappedLabelLineJoinsItsRow(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "Zmniejszy się do 0,5", "Przerwanie leczenia"),
		borderlessLine(687, "Powróci do 1 i neutropenia", "Wznowienie w dawce"),
		borderlessLine(674, "jedyną", "początkowej"),
		borderlessLine(661, "powróci do 0,5", "Wznowienie leczenia"),
	)

	assertContainsRows(t, markdown,
		"| Powróci do 1 i neutropenia<br>jedyną | Wznowienie w dawce<br>początkowej |",
		"| powróci do 0,5 | Wznowienie leczenia |")
}

func TestFrequencyLabelStartsItsOwnRow(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "Zaburzenia mięśniowo-kostne", "Ból"),
		borderlessLine(687, "często:", "bóle stawów"),
		borderlessLine(674, "rzadko:", "zmniejszenie ruchliwości"),
	)

	assertContainsRows(t, markdown, "| często: | bóle stawów |", "| rzadko: | zmniejszenie ruchliwości |")
}

func TestValueLinesAtPageTopJoinBorderlessTableBelow(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(760, "", "Ból głowy G1-4: 4,1%"),
		borderlessLine(747, "", "Zawroty głowy G1-4: 6%"),
		borderlessLine(734, "", "Zaburzenia smaku G1-2: 3,8%"),
		borderlessLine(721, "Niezbyt często:", "Ataksja stopnia 3: 0,3%"),
		borderlessLine(708, "Nieznana:", "Zespół encefalopatii"),
		borderlessLine(695, "Często:", "Zaburzenia widzenia"),
	)

	assertContainsRows(t, markdown, "|  | Ból głowy G1-4: 4,1%<br>Zawroty głowy G1-4: 6%<br>Zaburzenia smaku G1-2: 3,8% |", "| Niezbyt często: | Ataksja stopnia 3: 0,3% |")
}

func TestIndentedProseAboveBorderlessTableStaysProse(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(760, "Tabela 2. Działania niepożądane"),
		borderlessLine(721, "Niezbyt często:", "Ataksja stopnia 3: 0,3%"),
		borderlessLine(708, "Nieznana:", "Zespół encefalopatii"),
		borderlessLine(695, "Często:", "Zaburzenia widzenia"),
	)

	if strings.Contains(markdown, "| Tabela 2") {
		t.Errorf("caption at the margin was pulled into the table:\n%s", markdown)
	}
}

func TestEmptyLabelLineContinuesTwoColumnCell(t *testing.T) {
	table := &mdLineTable{xs: []float64{70, 177, 520}, cells: [][]string{{"Niezbyt często:", "niedociśnienie tętnicze."}}}

	if !table.wrapsInto([]float64{150, 400}, []string{"", "Niedociśnienie jest rzadko ciężkie."}, []float64{0, 60}) {
		t.Errorf("sentence under the same label not merged")
	}
	if table.wrapsInto([]float64{150, 400}, []string{"", "Rzadko: senność"}, []float64{0, 60}) {
		t.Errorf("frequency line merged into the previous cell")
	}
	wide := &mdLineTable{xs: []float64{70, 177, 300, 520}, cells: [][]string{{"Dawka", "10 mg", "raz"}}}
	if wide.wrapsInto([]float64{150, 280, 400}, []string{"", "Dzieci", "Dwa razy"}, []float64{0, 30, 40}) {
		t.Errorf("row of a wider table merged")
	}
}

func TestIntroLineEndingWithColonIsNotTableHeader(t *testing.T) {
	markdown := borderlessMarkdown(
		borderlessLine(700, "Początkowa dawka produktu leczniczego zależy od stopnia nasilenia choroby:"),
		borderlessLine(687, "- astma łagodna:", "100 µg do 250 µg dwa razy na dobę;"),
		borderlessLine(674, "- astma umiarkowana:", "250 µg do 500 µg dwa razy na dobę;"),
		borderlessLine(661, "- astma ciężka:", "500 µg do 1000 µg dwa razy na dobę."),
	)

	if strings.Contains(markdown, "| Początkowa") || !strings.Contains(markdown, "| - astma łagodna: | 100 µg do 250 µg dwa razy na dobę; |") {
		t.Errorf("got %q, want the intro line above the table", markdown)
	}
}
