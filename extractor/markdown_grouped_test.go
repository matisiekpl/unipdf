package extractor

import (
	"strings"
	"testing"
)

func cellWords(column int, y float64, x float64, text string) [][2]interface{} {
	var out [][2]interface{}
	for index, word := range strings.Fields(text) {
		if index > 0 {
			word = " " + word
		}
		out = append(out, [2]interface{}{column, mdCellMark{x0: x, x1: x + 5*float64(len([]rune(word))), y: y, s: word}})
		x += 5*float64(len([]rune(word))) + 3
	}
	return out
}

func groupedRowMarks() map[[2]int][]mdCellMark {
	cellMarks := map[[2]int][]mdCellMark{}
	var items [][2]interface{}
	items = append(items, cellWords(0, 690, 99, "Zaburzenia oka")...)
	items = append(items, cellWords(0, 670, 99, "Rzadko")...)
	items = append(items, cellWords(1, 670, 331, "Zaburzenia wzroku")...)
	items = append(items, cellWords(0, 650, 99, "Nieznana")...)
	items = append(items, cellWords(1, 650, 331, "Ostra jaskra z zamkniętym kątem")...)
	items = append(items, cellWords(1, 636, 331, "przesączania")...)
	items = append(items, cellWords(0, 616, 99, "Zaburzenia serca")...)
	for _, item := range items {
		key := [2]int{0, item[0].(int)}
		cellMarks[key] = append(cellMarks[key], item[1].(mdCellMark))
	}
	return cellMarks
}

func TestBorderlessCellRowHoldingSeveralRowsIsSplit(t *testing.T) {
	rows := mdSplitCellRow(groupedRowMarks(), 0, 2, true, false)

	want := [][]string{{"Zaburzenia oka", ""}, {"Rzadko", "Zaburzenia wzroku"}, {"Nieznana", "Ostra jaskra z zamkniętym kątem<br>przesączania"}, {"Zaburzenia serca", ""}}
	if len(rows) != len(want) {
		t.Fatalf("got %q, want %q", rows, want)
	}
	for index := range want {
		if rows[index][0] != want[index][0] || rows[index][1] != want[index][1] {
			t.Errorf("row %d: got %q, want %q", index, rows[index], want[index])
		}
	}
}

func TestBorderedCellRowStaysOneRow(t *testing.T) {
	rows := mdSplitCellRow(groupedRowMarks(), 0, 2, false, false)

	if len(rows) != 1 {
		t.Errorf("got %d rows, want a bordered cell kept whole", len(rows))
	}
}

func TestWrappedLabelWithOneAlignedLineStaysOneRow(t *testing.T) {
	cellMarks := map[[2]int][]mdCellMark{}
	for _, item := range append(append(cellWords(0, 690, 99, "Zaburzenia krwi i"), cellWords(1, 690, 331, "Niedokrwistość")...), cellWords(0, 676, 99, "Układu Chłonnego")...) {
		key := [2]int{0, item[0].(int)}
		cellMarks[key] = append(cellMarks[key], item[1].(mdCellMark))
	}

	if rows := mdSplitCellRow(cellMarks, 0, 2, true, false); len(rows) != 1 {
		t.Errorf("got %q, want a single row", rows)
	}
}

func TestLineStraddlesBorderInsideWordGap(t *testing.T) {
	heading := []mdWord{{s: "Zaburzenia", x0: 99, x1: 160, y: 500}, {s: "mięśniowo-szkieletowe", x0: 163, x1: 276, y: 500}, {s: "i", x0: 279, x1: 282, y: 500}}
	row := []mdWord{{s: "Nieznana", x0: 99, x1: 144, y: 480}, {s: "Gorączka", x0: 331, x1: 380, y: 480}}

	if !mdLineStraddles(heading, 500, []float64{278.9}) {
		t.Errorf("heading split by a border in a word gap not detected")
	}
	if mdLineStraddles(row, 480, []float64{324}) {
		t.Errorf("two-column row taken for a spanning line")
	}
}

func groupedTable() *mdLineTable {
	return &mdLineTable{xs: []float64{93.5, 324, 549}, ys: []float64{336, 184.2}, cells: [][]string{{"Zaburzenia skóry", ""}, {"Często", "Pokrzywka"}}}
}

func trailingMarks(words ...[]interface{}) []TextMark {
	var marks []TextMark
	for _, word := range words {
		marks = append(marks, wordMark(word[0].(string), word[1].(float64), word[2].(float64), word[2].(float64)+11, 11))
	}
	return marks
}

func TestGroupsBelowGridJoinTable(t *testing.T) {
	table := groupedTable()
	marks := trailingMarks(
		[]interface{}{"Zaburzenia", 99.0, 165.0}, []interface{}{"ogólne", 160.0, 165.0},
		[]interface{}{"Nieznana", 99.0, 151.0}, []interface{}{"Gorączka,", 331.0, 151.0}, []interface{}{"astenia", 382.0, 151.0},
		[]interface{}{"Zaburzenia", 99.0, 124.0}, []interface{}{"mięśni", 160.0, 124.0},
		[]interface{}{"Nieznana", 99.0, 110.0}, []interface{}{"Kurcze", 331.0, 110.0},
		[]interface{}{"4.9", 70.0, 80.0}, []interface{}{"Przedawkowanie", 90.0, 80.0},
	)
	used := make([]bool, len(marks))

	table.attachTrailingLines(marks, used, 0, nil)

	got := table.markdown()
	for _, row := range []string{"| Zaburzenia ogólne |  |", "| Nieznana | Gorączka, astenia |", "| Zaburzenia mięśni |  |", "| Nieznana | Kurcze |"} {
		if !strings.Contains(got, row) {
			t.Errorf("got\n%s\nmissing %q", got, row)
		}
	}
	if used[9] || strings.Contains(got, "Przedawkowanie") {
		t.Errorf("section heading after the table was absorbed")
	}
}

func TestJustifiedFootnoteBelowGridStaysOut(t *testing.T) {
	table := groupedTable()
	marks := trailingMarks(
		[]interface{}{"*Senność", 99.0, 165.0}, []interface{}{"obejmuje", 147.0, 165.0}, []interface{}{"działania", 322.0, 165.0}, []interface{}{"niepożądane", 373.0, 165.0},
		[]interface{}{"senność,", 99.0, 151.0}, []interface{}{"ospałość", 147.0, 151.0},
	)
	used := make([]bool, len(marks))

	table.attachTrailingLines(marks, used, 0, nil)

	if len(table.cells) != 2 {
		t.Errorf("got %q, want the footnote left as prose", table.cells)
	}
}

func TestHeadingWithoutFollowingRowStaysOut(t *testing.T) {
	table := groupedTable()
	marks := trailingMarks([]interface{}{"Zaburzenia", 99.0, 165.0}, []interface{}{"ogólne", 160.0, 165.0}, []interface{}{"Dalszy", 99.0, 151.0}, []interface{}{"tekst", 140.0, 151.0})
	used := make([]bool, len(marks))

	table.attachTrailingLines(marks, used, 0, nil)

	if len(table.cells) != 2 {
		t.Errorf("got %q, want no rows without a frequency line", table.cells)
	}
}

func TestPageTopRowsContinuePreviousTable(t *testing.T) {
	marks := trailingMarks(
		[]interface{}{"Zaburzenia", 99.0, 760.0}, []interface{}{"rozrodcze", 160.0, 760.0},
		[]interface{}{"Często", 99.0, 746.0}, []interface{}{"Impotencja", 331.0, 746.0},
		[]interface{}{"4.9", 70.0, 700.0}, []interface{}{"Przedawkowanie", 90.0, 700.0},
	)

	continuation, used := groupedTable().continuationRows(marks, nil)

	if continuation == nil || continuation.markdown() != "| Zaburzenia rozrodcze |  |\n| --- | --- |\n| Często | Impotencja |\n" || used[4] {
		t.Errorf("got %+v, want the two rows at the top of the page", continuation)
	}
}

func TestGroupHeadingAboveGroupedTable(t *testing.T) {
	table := &mdLineTable{xs: []float64{93.5, 324, 549}, ys: []float64{742.4, 559}, cells: [][]string{{"Rzadko", "Depresja"}, {"Zaburzenia oka", ""}, {"Rzadko", "Zaburzenia wzroku"}}}
	marks := trailingMarks([]interface{}{"Zaburzenia", 99.0, 746.0}, []interface{}{"psychiczne", 160.0, 746.0})
	used := make([]bool, len(marks))

	table.attachGroupHeading(marks, used)

	if table.cells[0][0] != "Zaburzenia psychiczne" || !used[0] {
		t.Errorf("got %q, want the heading as first row", table.cells)
	}
}

func TestCaptionAboveGroupedTableStaysOut(t *testing.T) {
	for _, words := range [][][]interface{}{
		{{"Tabela", 99.0, 746.0}, {"3.", 135.0, 746.0}},
		{{"Zaburzenia", 99.0, 746.0}, {"psychiczne:", 160.0, 746.0}},
		{{"Badanie", 99.0, 746.0}, {"HOPE", 140.0, 746.0}, {"wyniki", 170.0, 746.0}},
	} {
		table := &mdLineTable{xs: []float64{93.5, 324, 549}, ys: []float64{742.4, 559}, cells: [][]string{{"Rzadko", "Depresja"}, {"Zaburzenia oka", ""}}}
		marks := trailingMarks(words...)

		table.attachGroupHeading(marks, make([]bool, len(marks)))

		if len(table.cells) != 2 {
			t.Errorf("got %q, want the line left out", table.cells)
		}
	}
}

func TestLowercaseLabelContinuationStaysInRow(t *testing.T) {
	cellMarks := map[[2]int][]mdCellMark{}
	items := append(cellWords(0, 690, 99, "Zaburzenia krwi"), cellWords(1, 690, 331, "Niedokrwistość")...)
	items = append(items, cellWords(0, 676, 99, "i układu chłonnego")...)
	items = append(items, cellWords(1, 676, 331, "Leukopenia")...)
	for _, item := range items {
		key := [2]int{0, item[0].(int)}
		cellMarks[key] = append(cellMarks[key], item[1].(mdCellMark))
	}

	if rows := mdSplitCellRow(cellMarks, 0, 2, true, false); len(rows) != 1 {
		t.Errorf("got %q, want the lowercase label line kept in the row", rows)
	}
}

func TestDistantLinesBelowGridStayOut(t *testing.T) {
	table := groupedTable()
	marks := trailingMarks([]interface{}{"Zaburzenia", 99.0, 120.0}, []interface{}{"ogólne", 160.0, 120.0}, []interface{}{"Nieznana", 99.0, 106.0}, []interface{}{"Gorączka", 331.0, 106.0})

	table.attachTrailingLines(marks, make([]bool, len(marks)), 0, nil)

	if len(table.cells) != 2 {
		t.Errorf("got %q, want lines far below the grid left out", table.cells)
	}
}

func TestFrequencyLabelGluedToTermIsSplit(t *testing.T) {
	table := &mdLineTable{cells: [][]string{
		{"często:", "wysypka, świąd"},
		{"niezbyt często: zapalenie skóry, nadwrażliwość na światło", ""},
		{"bardzo rzadko: zespół Stevensa-Johnsona", ""},
		{"Zaburzenia mięśniowo-szkieletowe i tkanki łącznej", ""},
		{"częstość nieznana (nie może być określona): ból", ""},
	}}

	table.splitFrequencyLabels()

	want := [][]string{{"często:", "wysypka, świąd"}, {"niezbyt często:", "zapalenie skóry, nadwrażliwość na światło"}, {"bardzo rzadko:", "zespół Stevensa-Johnsona"}, {"Zaburzenia mięśniowo-szkieletowe i tkanki łącznej", ""}, {"częstość nieznana (nie może być określona):", "ból"}}
	for index := range want {
		if table.cells[index][0] != want[index][0] || table.cells[index][1] != want[index][1] {
			t.Errorf("row %d: got %q, want %q", index, table.cells[index], want[index])
		}
	}
}

func TestFrequencySplitNeedsLabelColumn(t *testing.T) {
	table := &mdLineTable{cells: [][]string{{"Dawka", "Uwagi"}, {"rzadko: tylko po konsultacji", ""}}}

	table.splitFrequencyLabels()

	if table.cells[1][0] != "rzadko: tylko po konsultacji" {
		t.Errorf("got %q, want a table without frequency labels left alone", table.cells)
	}
}

func TestFrequencyLineBelowGridIsRow(t *testing.T) {
	table := &mdLineTable{xs: []float64{99, 220, 520}, ys: []float64{336, 184.2}, cells: [][]string{{"często:", "wysypka"}}}
	marks := trailingMarks([]interface{}{"bardzo", 104.0, 170.0}, []interface{}{"rzadko:", 140.0, 170.0}, []interface{}{"zespół", 182.0, 170.0}, []interface{}{"Stevensa", 220.0, 170.0})

	table.attachTrailingLines(marks, make([]bool, len(marks)), 0, nil)

	if len(table.cells) != 2 || table.cells[1][0] != "bardzo rzadko:" || table.cells[1][1] != "zespół Stevensa" {
		t.Errorf("got %q", table.cells)
	}
}
