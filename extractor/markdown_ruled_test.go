package extractor

import (
	"strings"
	"testing"
)

func cellsLine(cells ...interface{}) []mdWord {
	var line []mdWord
	for index := 0; index+1 < len(cells); index += 2 {
		x := 0.0
		switch value := cells[index].(type) {
		case int:
			x = float64(value)
		case float64:
			x = value
		}
		line = append(line, proseLine(x, cells[index+1].(string))...)
	}
	return line
}

func ruleSegments(y float64, xs ...float64) []Stroke {
	var segments []Stroke
	for index := 0; index+1 < len(xs); index++ {
		segments = append(segments, Stroke{X1: xs[index+1], Y1: y, X2: xs[index], Y2: y})
	}
	return segments
}

func dosingTable() ([][]mdWord, []float64, []Stroke) {
	lines := [][]mdWord{
		proseLine(90, "U pacjentów z niewydolnością nerek zaleca się następujące dawkowanie:"),
		cellsLine(112.7, "Klirens", 260.0, "Dawkowanie", 390.0, "Objętość wlewu i"),
		cellsLine(103.3, "kreatyniny", 420.0, "czas"),
		cellsLine(105.6, "(mL/min)"),
		cellsLine(93.4, "50 ≤ CLcr< 80", 210.0, "6 mg (6 mL koncentratu do", 395.0, "100 mL / 15 minut"),
		cellsLine(244.0, "sporządzania"),
		cellsLine(227.0, "roztworu do infuzji)"),
		cellsLine(91.9, "30 ≤ CLcr < 50", 210.0, "4 mg (4 mL koncentratu do", 395.0, "500 mL / 1 godzinę"),
		cellsLine(244.0, "sporządzania"),
		cellsLine(227.0, "roztworu do infuzji)"),
		cellsLine(118.0, "< 30", 210.0, "2 mg (2 mL koncentratu do", 395.0, "500 mL / 1 godzinę"),
		cellsLine(244.0, "sporządzania"),
		cellsLine(227.0, "roztworu do infuzji)"),
		proseLine(90, "Podawanie co 3 do 4 tygodni."),
	}
	lineYs := []float64{600, 556.3, 543.4, 529.2, 513.9, 500.6, 486.8, 472.5, 459.2, 445.4, 431.1, 417.8, 404.0, 365.4}
	var strokes []Stroke
	for _, y := range []float64{565.6, 520.6, 519.6, 395.4, 394.4} {
		strokes = append(strokes, ruleSegments(y, 84.6, 176, 374.4, 476.2)...)
	}
	return lines, lineYs, strokes
}

func TestRuledTableWithCenteredColumnsAndWrappedCells(t *testing.T) {
	lines, lineYs, strokes := dosingTable()

	start, end, table := mdNextRuledTable(lines, lineYs, 0, strokes)

	if table == nil {
		t.Fatal("no table found between the rules")
	}
	if start != 1 || end != 13 {
		t.Errorf("table spans lines %d..%d, want 1..13", start, end)
	}
	want := "| Klirens<br>kreatyniny<br>(mL/min) | Dawkowanie | Objętość wlewu i<br>czas |\n" +
		"| --- | --- | --- |\n" +
		"| 50 ≤ CLcr< 80 | 6 mg (6 mL koncentratu do<br>sporządzania<br>roztworu do infuzji) | 100 mL / 15 minut |\n" +
		"| 30 ≤ CLcr < 50 | 4 mg (4 mL koncentratu do<br>sporządzania<br>roztworu do infuzji) | 500 mL / 1 godzinę |\n" +
		"| < 30 | 2 mg (2 mL koncentratu do<br>sporządzania<br>roztworu do infuzji) | 500 mL / 1 godzinę |\n"
	if got := table.markdown(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRuledTableKeepsSurroundingProse(t *testing.T) {
	lines, lineYs, strokes := dosingTable()

	blocks := mdLineBlocks(lines, lineYs, strokes, 500)

	if len(blocks) != 3 || !strings.HasPrefix(blocks[0].text, "U pacjentów") || blocks[1].table == nil || blocks[2].text != "Podawanie co 3 do 4 tygodni." {
		t.Errorf("got %+v, want prose, table, prose", blocks)
	}
}

func TestRuleSegmentsAreJoinedAndDoubleRulesMerged(t *testing.T) {
	strokes := append(ruleSegments(500, 84.6, 176, 374.4, 476.2), ruleSegments(499, 84.6, 476.2)...)
	strokes = append(strokes, ruleSegments(300, 84.6, 150)...)
	strokes = append(strokes, Stroke{X1: 100, Y1: 200, X2: 100, Y2: 400})

	rules := mdTableRules(strokes)

	if len(rules) != 1 || rules[0].X1 != 84.6 || rules[0].X2 != 476.2 {
		t.Errorf("got %+v, want one rule from 84.6 to 476.2", rules)
	}
}

func TestSegmentsWithGapAreNotJoined(t *testing.T) {
	strokes := []Stroke{{X1: 80, Y1: 500, X2: 150, Y2: 500}, {X1: 160, Y1: 500, X2: 230, Y2: 500}}

	if rules := mdTableRules(strokes); len(rules) != 0 {
		t.Errorf("got %+v, want short separated segments ignored", rules)
	}
}

func TestJustifiedGapDoesNotSplitCell(t *testing.T) {
	lines := [][]mdWord{
		{{s: "Nieznana", x0: 90, x1: 130}, {s: "Zaburzenia", x0: 200, x1: 250}, {s: "czynności", x0: 260, x1: 300}, {s: "nerek,", x0: 310, x1: 340}, {s: "ostra", x0: 360, x1: 385}},
		{{s: "niewydolność", x0: 200, x1: 260}, {s: "nerek", x0: 268, x1: 295}},
		{{s: "Rzadko", x0: 90, x1: 125}, {s: "Wysypka", x0: 200, x1: 240}},
	}
	strokes := append(ruleSegments(520, 84, 470), ruleSegments(460, 84, 470)...)

	_, _, table := mdNextRuledTable(lines, []float64{510, 497, 484}, 0, strokes)

	if table == nil || table.cols() != 2 || table.cells[0][1] != "Zaburzenia czynności nerek, ostra<br>niewydolność nerek" {
		t.Errorf("got %+v, want two columns with the wide justified space kept inside the cell", table)
	}
}

func TestOneLineChannelIsNotColumn(t *testing.T) {
	lines := [][]mdWord{
		{{s: "Nieznana", x0: 90, x1: 130}, {s: "Zaburzenia", x0: 200, x1: 250}, {s: "nerek,", x0: 310, x1: 340}, {s: "ostra", x0: 360, x1: 385}},
		proseLine(90, "niewydolność nerek i zaburzenia czynności wątroby"),
	}
	strokes := append(ruleSegments(520, 84, 470), ruleSegments(480, 84, 470)...)

	if _, _, table := mdNextRuledTable(lines, []float64{510, 497}, 0, strokes); table != nil {
		t.Errorf("got table %q, want channels seen in a single line ignored", table.markdown())
	}
}

func TestBulletLineBetweenRulesBreaksTable(t *testing.T) {
	lines := [][]mdWord{
		cellsLine(90, "•", 110, "Trombocytopenia", 330, "neutropenia"),
		cellsLine(90, "Jeśli liczba płytek krwi", 330, "Zalecane postępowanie"),
	}
	strokes := append(ruleSegments(520, 84, 470), ruleSegments(480, 84, 470)...)

	if _, _, table := mdNextRuledTable(lines, []float64{510, 497}, 0, strokes); table != nil {
		t.Errorf("got table %q, want no table across a bulleted heading", table.markdown())
	}
}

func TestFootnoteLegendBetweenRulesIsNotTable(t *testing.T) {
	lines := [][]mdWord{
		cellsLine(77, "§", 105, "Patrz także podpunkt opisu wybranych działań"),
		cellsLine(77, "†", 105, "W tym wszystkie zakażenia"),
		cellsLine(77, "‡", 105, "W tym inne krwawienia"),
	}
	strokes := append(ruleSegments(790, 70, 520), ruleSegments(700, 70, 520)...)

	if _, _, table := mdNextRuledTable(lines, []float64{778, 762, 750}, 0, strokes); table != nil {
		t.Errorf("got table %q, want footnote legend left as prose", table.markdown())
	}
}

func TestSingleColumnBetweenRulesIsNotTable(t *testing.T) {
	lines := [][]mdWord{
		proseLine(90, "Ostrzeżenie dotyczące stosowania u dzieci."),
		proseLine(90, "Lek należy przechowywać w miejscu niedostępnym."),
	}
	strokes := append(ruleSegments(520, 84, 470), ruleSegments(480, 84, 470)...)

	if _, _, table := mdNextRuledTable(lines, []float64{510, 497}, 0, strokes); table != nil {
		t.Errorf("got table %q, want boxed prose left alone", table.markdown())
	}
}

func TestTextOutsideRuleExtentIsNotTable(t *testing.T) {
	lines := [][]mdWord{
		cellsLine(40, "Dawka", 300, "Częstość"),
		cellsLine(40, "10 mg", 300, "raz na dobę"),
	}
	strokes := append(ruleSegments(520, 84, 470), ruleSegments(480, 84, 470)...)

	if _, _, table := mdNextRuledTable(lines, []float64{510, 497}, 0, strokes); table != nil {
		t.Errorf("got table %q, want text sticking out of the rules ignored", table.markdown())
	}
}

func TestRuledRowContinuation(t *testing.T) {
	cases := []struct {
		name     string
		previous []string
		row      []string
		want     bool
	}{
		{"empty first column", []string{"50", "6 mg"}, []string{"", "Sporządzania"}, true},
		{"lowercase label wrap", []string{"Klirens", "Dawkowanie"}, []string{"kreatyniny", "czas"}, true},
		{"parenthesis wrap", []string{"Klirens", "Dawkowanie"}, []string{"(mL/min)", ""}, true},
		{"lowercase label with number", []string{"30 do mniej niż 45 kg", "70 mg"}, []string{"co najmniej 45 kg", "100 mg"}, false},
		{"new label with continued value", []string{"Spadnie do 1, lub", "Przerwanie leczenia i"}, []string{"Spadnie do 0,5", "pełnej morfologii"}, true},
		{"label ending with preposition", []string{"(temperatura ciała ≥ 38,5°C), lub spadnie do", "Przerwanie"}, []string{"< 0,5 x 10⁹/l", ""}, true},
		{"new row after preposition label", []string{"Dawka do", "5 mg"}, []string{"10 mg", "Raz na dobę"}, false},
		{"lowercase value after sentence", []string{"Często", "Ból głowy."}, []string{"rzadko", "zawroty głowy"}, false},
		{"lowercase value after open cell", []string{"(temperatura ciała)", "pełnej morfologii krwi nie"}, []string{"lub", "rzadziej niż raz"}, true},
		{"capitalised row", []string{"Zmniejszy się", "Przerwanie"}, []string{"Powróci do", "Wznowienie"}, false},
		{"bracketed spread under values", []string{"Cmax (ng/ml)", "8,1", "10,4"}, []string{"(CV%)", "(30,7)", "(37,2)"}, true},
		{"bracketed value after finished cell", []string{"Cmax", "8,1.", "10,4"}, []string{"(CV%)", "(30,7)", "(37,2)"}, false},
		{"label ending with lub", []string{"(temperatura ciała ≥ 38,5°C) lub", "tygodniu"}, []string{"Spadnie do < 0,5 x 10⁹/l", ""}, true},
		{"label ending with comma", []string{"Spadnie do <1 x 10⁹/l na co najmniej 7 dni,", "Przerwanie"}, []string{"Spadnie do < 1 x 10⁹/l z gorączką", "pełnej morfologii"}, true},
		{"label ending with a word containing i", []string{"Zaburzenia krwi", "Niedokrwistość"}, []string{"Zaburzenia serca", ""}, false},
		{"label ending with lub but new value", []string{"Spadnie do 1, lub", "Przerwanie"}, []string{"Spadnie do 0,5", "Wznowienie"}, false},
	}
	for _, testCase := range cases {
		if got := mdContinuesRow(testCase.previous, testCase.row); got != testCase.want {
			t.Errorf("%s: got %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

func TestRuledTableSplitByRulesStartsRowAtEachRule(t *testing.T) {
	lines := [][]mdWord{
		cellsLine(90, "Jeśli ANC", 330, "Zalecane postępowanie"),
		cellsLine(90, "Spadnie do < 1 x 10⁹/l", 330, "Przerwanie leczenia i"),
		cellsLine(330, "wykonywanie morfologii"),
		cellsLine(90, "powróci do ≥ 1 x 10⁹/l", 330, "wznowienie leczenia"),
	}
	lineYs := []float64{510, 490, 477, 455}
	var strokes []Stroke
	for _, y := range []float64{520, 500, 465, 440} {
		strokes = append(strokes, ruleSegments(y, 84, 470)...)
	}

	_, _, table := mdNextRuledTable(lines, lineYs, 0, strokes)

	if table == nil || len(table.cells) != 3 || table.cells[1][1] != "Przerwanie leczenia i<br>wykonywanie morfologii" || table.cells[2][0] != "powróci do ≥ 1 x 10⁹/l" {
		t.Errorf("got %+v, want a new row at every rule", table)
	}
}

func TestBorderlessTableWinsUnlessRuledTableCoversIt(t *testing.T) {
	lines, lineYs, strokes := dosingTable()

	blocks := mdLineBlocks(lines, lineYs, strokes, 500)

	tables := 0
	for _, block := range blocks {
		if block.table != nil {
			tables++
			if block.table.cols() != 3 || len(block.table.cells) != 4 {
				t.Errorf("got %q, want the ruled dosing table", block.table.markdown())
			}
		}
	}
	if tables != 1 {
		t.Errorf("got %d tables, want 1", tables)
	}
}

func TestHeaderWithEmptyLabelCellIsCompletedByLaterLine(t *testing.T) {
	lines := [][]mdWord{
		cellsLine(330, "Wszystkie dawki (n=147)"),
		cellsLine(340, "400 mg (n=73)"),
		cellsLine(340, "600 mg (n=74)"),
		cellsLine(90, "Najlepsza odpowiedź", 350, "n (%)"),
		cellsLine(90, "Odpowiedź całkowita", 350, "1 (0,7)"),
		cellsLine(90, "Odpowiedź częściowa", 345, "98 (66,7)"),
	}
	lineYs := []float64{421.6, 407.8, 394.0, 380.2, 365.9, 352.1}
	var strokes []Stroke
	for _, y := range []float64{432, 373, 290} {
		strokes = append(strokes, ruleSegments(y, 84, 470)...)
	}

	_, _, table := mdNextRuledTable(lines, lineYs, 0, strokes)

	if table == nil || len(table.cells) != 3 || table.cells[0][0] != "Najlepsza odpowiedź" || table.cells[0][1] != "Wszystkie dawki (n=147)<br>400 mg (n=73)<br>600 mg (n=74)<br>n (%)" {
		t.Errorf("got %+v, want one header row", table)
	}
}

func TestHeaderWithoutRuleBelowKeepsFirstDataRow(t *testing.T) {
	lines := [][]mdWord{
		cellsLine(330, "Lek A", 410, "Placebo"),
		cellsLine(90, "Ból głowy", 335, "5%", 415, "3%"),
		cellsLine(90, "Nudności", 335, "4%", 415, "2%"),
	}
	strokes := append(ruleSegments(432, 84, 470), ruleSegments(370, 84, 470)...)

	_, _, table := mdNextRuledTable(lines, []float64{421, 407, 394}, 0, strokes)

	if table == nil || len(table.cells) != 3 || table.cells[1][0] != "Ból głowy" {
		t.Errorf("got %+v, want the first data row kept apart from the header", table)
	}
}

func TestSectionNumbersBetweenRulesAreNotTable(t *testing.T) {
	lines := [][]mdWord{
		cellsLine(70, "6.2", 110, "Niezgodności farmaceutyczne"),
		cellsLine(110, "Nie wolno mieszać z innymi produktami."),
		cellsLine(70, "6.3", 110, "Okres ważności"),
		cellsLine(110, "Okres ważności: 2 lata."),
	}
	strokes := append(ruleSegments(520, 64, 520), ruleSegments(440, 64, 520)...)

	if _, _, table := mdNextRuledTable(lines, []float64{510, 497, 484, 471}, 0, strokes); table != nil {
		t.Errorf("got table %q, want numbered sections left as prose", table.markdown())
	}
}

func TestCaptionAboveRuledTableStaysOutside(t *testing.T) {
	lines, lineYs, strokes := dosingTable()
	lines[0], lineYs[0] = proseLine(93, "Tabela 1"), 568

	start, _, table := mdNextRuledTable(lines, lineYs, 0, strokes)

	if table == nil || start != 1 {
		t.Errorf("table starts at line %d, want the caption left out", start)
	}
}

func TestHeaderLinesOverValueColumnsJoinRuledTable(t *testing.T) {
	lines, lineYs, strokes := dosingTable()
	lines[0], lineYs[0] = cellsLine(395.0, "na 100 mL"), 568

	start, _, table := mdNextRuledTable(lines, lineYs, 0, strokes)

	if table == nil || start != 0 || !strings.HasPrefix(table.cells[0][2], "na 100 mL") {
		t.Errorf("got start %d and %+v, want the line over the last column in the header", start, table)
	}
}

func TestProseLineCrossingTabStopsBlocksTable(t *testing.T) {
	lines := [][]mdWord{
		cellsLine(90, "Dawka", 300, "10 mg"),
		proseLine(90, "Produkt należy podawać ostrożnie u pacjentów z zaburzeniami czynności"),
		cellsLine(90, "Masa", 300, "70 kg"),
	}
	strokes := append(ruleSegments(520, 84, 470), ruleSegments(460, 84, 470)...)

	if _, _, table := mdNextRuledTable(lines, []float64{510, 497, 484}, 0, strokes); table != nil {
		t.Errorf("got table %q, want prose crossing the column gap to block it", table.markdown())
	}
}
