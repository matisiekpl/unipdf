package extractor

import (
	"reflect"
	"strings"
	"testing"

	"github.com/matisiekpl/unipdf/v3/model"
)

func TestContinuationWithDroppedEmptyColumnsIsPlacedByPosition(t *testing.T) {
	table := &mdLineTable{xs: []float64{35, 148, 291, 333, 396, 468, 560}, cells: [][]string{
		{"Badania", "Zwiększenie AspAT", "Niezbyt często", "", "", ""},
	}}
	next := &mdLineTable{xs: []float64{148, 291, 333, 396, 560}, cells: [][]string{
		{"Zwiększenie enzymów", "", "Często", "Bardzo rzadko"},
	}}

	if !table.absorb(next) {
		t.Fatalf("continuation with a subset of the columns was not merged")
	}
	want := []string{"", "Zwiększenie enzymów", "", "Często", "Bardzo rzadko", ""}
	if !reflect.DeepEqual(table.cells[1], want) {
		t.Errorf("continuation row placed as %q, want %q", table.cells[1], want)
	}
}

func TestContinuationWithMoreColumnsWidensTable(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300}, cells: [][]string{{"a1", "c1"}}}
	next := &mdLineTable{xs: []float64{100, 150, 200, 300}, cells: [][]string{{"a2", "b2", "c2"}}}

	if !table.absorb(next) {
		t.Fatalf("continuation with more columns was not merged")
	}
	if !reflect.DeepEqual(table.cells, [][]string{{"a1", "", "c1"}, {"a2", "b2", "c2"}}) {
		t.Errorf("widened table is %q", table.cells)
	}
}

func TestTablesWithDifferentColumnsStaySeparate(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300}, cells: [][]string{{"a1", "b1"}}}
	next := &mdLineTable{xs: []float64{100, 150, 250, 300}, cells: [][]string{{"x", "y", "z"}}}

	if table.absorb(next) {
		t.Errorf("tables with unrelated columns were merged into %q", table.cells)
	}
}

func TestTablesWithSameColumnCountMergeWhenShifted(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300}, cells: [][]string{{"a1", "b1"}}}
	next := &mdLineTable{xs: []float64{120, 220, 320}, cells: [][]string{{"a2", "b2"}}}

	if !table.absorb(next) || len(table.cells) != 2 {
		t.Errorf("shifted continuation with the same columns was not merged: %q", table.cells)
	}
}

func TestRepeatedHeaderRowsAreDropped(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300}, cells: [][]string{
		{"Klasyfikacja", "Częstość"}, {"", "Często"}, {"Zaburzenia krwi", "Rzadko"},
	}}
	next := &mdLineTable{xs: []float64{100, 200, 300}, cells: [][]string{
		{"Klasyfikacja", "Częstość"}, {"", "Często"}, {"Zaburzenia serca", "Rzadko"},
	}}

	table.absorb(next)

	want := [][]string{{"Klasyfikacja", "Częstość"}, {"", "Często"}, {"Zaburzenia krwi", "Rzadko"}, {"Zaburzenia serca", "Rzadko"}}
	if !reflect.DeepEqual(table.cells, want) {
		t.Errorf("merged table is %q, want %q", table.cells, want)
	}
}

func TestHeaderWithDifferentLineBreaksIsRecognised(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300}, cells: [][]string{{"Częstość<br>występowania", "Działanie"}, {"Rzadko", "a"}}}
	next := &mdLineTable{xs: []float64{100, 200, 300}, cells: [][]string{{"Częstość występowania", "Działanie"}, {"Często", "b"}}}

	table.absorb(next)

	if len(table.cells) != 3 {
		t.Errorf("repeated header was kept: %q", table.cells)
	}
}

func TestRowCutByPageContinuesCellAbove(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"Zaburzenia<br>układu<br>oddechowego,", "Duszność", "Niezbyt często"},
		{"", "Eozynofilowe zapalenie płuc", "Rzadko"},
	}}
	next := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"klatki piersiowej i<br>śródpiersia", "", ""},
		{"Zaburzenia wątroby", "Żółtaczka", "Rzadko"},
	}}

	table.absorb(next)

	if table.cells[0][0] != "Zaburzenia<br>układu<br>oddechowego,<br>klatki piersiowej i<br>śródpiersia" {
		t.Errorf("cut label continued as %q", table.cells[0][0])
	}
	if len(table.cells) != 3 || table.cells[2][0] != "Zaburzenia wątroby" {
		t.Errorf("merged table is %q", table.cells)
	}
}

func TestNewRowStartingLowercaseIsKept(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{{"Zaburzenia", "kaszel", "Rzadko"}}}
	next := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{{"", "ból głowy", "Często"}}}

	table.absorb(next)

	if len(table.cells) != 2 || table.cells[0][1] != "kaszel" {
		t.Errorf("a new row with several cells was merged into the row above: %q", table.cells)
	}
}

func TestSingleCellRowStartingUppercaseIsKept(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300}, cells: [][]string{{"Zaburzenia", "Kaszel"}}}
	next := &mdLineTable{xs: []float64{100, 200, 300}, cells: [][]string{{"", "Duszność"}}}

	table.absorb(next)

	if len(table.cells) != 2 {
		t.Errorf("a new row starting with a capital was merged into the row above: %q", table.cells)
	}
}

func TestTableSplitAcrossPagesIsReunited(t *testing.T) {
	columns := []float64{100, 200, 300}
	rules := func(rows []float64) []Stroke {
		strokes := rowRules(columns, rows)
		for _, x := range columns {
			strokes = append(strokes, verticalRule(x, rows[len(rows)-1], rows[0]))
		}
		return strokes
	}
	first := gridPage(rules([]float64{700, 680, 660}), nil, nil,
		testMark("Klasyfikacja", 105, 690), testMark("Działanie", 205, 690),
		testMark("Zaburzenia", 105, 670), testMark("Kaszel", 205, 670))
	second := gridPage(rules([]float64{700, 680, 660}), nil, nil,
		testMark("Klasyfikacja", 105, 690), testMark("Działanie", 205, 690),
		testMark("oddechowe", 105, 670))

	markdown := DocumentMarkdown([]*PageText{first, second}, false)

	if strings.Count(markdown, "Klasyfikacja") != 1 || !strings.Contains(markdown, "| Zaburzenia<br>oddechowe | Kaszel |") {
		t.Errorf("table split across pages rendered as:\n%s", markdown)
	}
}

func TestRowCutByPageWithLabelAndTextContinuesBoth(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"Ciąża, połóg i okres okołoporodo", "", "Zespół odstawienia (patrz"},
	}}
	next := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"wy", "", "punkt 4.6)"},
		{"Zaburzenia piersi", "", "Ginekomastia"},
	}}

	table.absorb(next)

	if table.cells[0][0] != "Ciąża, połóg i okres okołoporodo<br>wy" || table.cells[0][2] != "Zespół odstawienia (patrz<br>punkt 4.6)" || len(table.cells) != 2 {
		t.Errorf("cut row continued as %q", table.cells)
	}
}

func TestCutLabelContinuesWhileNextSubRowStays(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"Zaburzenia mięśniowo-szkieletowe i tkanki", "Kurcze mięśni", "Często"},
	}}
	next := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"podskórnej", "Osłabienie mięśni", "Niezbyt często"},
	}}

	table.absorb(next)

	want := [][]string{{"Zaburzenia mięśniowo-szkieletowe i tkanki<br>podskórnej", "Kurcze mięśni", "Często"}, {"", "Osłabienie mięśni", "Niezbyt często"}}
	if !reflect.DeepEqual(table.cells, want) {
		t.Errorf("merged table is %q", table.cells)
	}
}

func TestRowCutByPageWithSeveralLowercaseCellsIsJoined(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"Zaburzenia serca", "Wydłużenie", "nagła"},
	}}
	next := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"", "odstępu QTc", "śmierć"},
		{"Zaburzenia oka", "Zaćma", "Ślepota"},
	}}

	table.absorb(next)

	want := [][]string{{"Zaburzenia serca", "Wydłużenie<br>odstępu QTc", "nagła<br>śmierć"}, {"Zaburzenia oka", "Zaćma", "Ślepota"}}
	if !reflect.DeepEqual(table.cells, want) {
		t.Errorf("merged table is %q", table.cells)
	}
}

func TestLowercaseFrequencyRowAfterPageBreakStaysSeparate(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"Zaburzenia skóry", "często", "świąd"},
	}}
	next := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"", "rzadko", "wysypka"},
	}}

	table.absorb(next)

	if len(table.cells) != 2 {
		t.Errorf("frequency row was merged into %q", table.cells)
	}
}

func TestCutRowWithCapitalisedNameContinuationIsJoined(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300, 400, 500}, cells: [][]string{
		{"Zaburzenia skóry", "wysypka skórna /<br>wyprysk /", "pokrzywka,<br>obrzęk", "zespół Stevensa–<br>Johnsona, zespół"},
	}}
	next := &mdLineTable{xs: []float64{100, 200, 300, 400, 500}, cells: [][]string{
		{"", "wykwity skórne,<br>świąd", "naczynioruchowy", "Lyella (TEN)"},
	}}

	table.absorb(next)

	want := [][]string{{"Zaburzenia skóry", "wysypka skórna /<br>wyprysk /<br>wykwity skórne,<br>świąd", "pokrzywka,<br>obrzęk<br>naczynioruchowy", "zespół Stevensa–<br>Johnsona, zespół<br>Lyella (TEN)"}}
	if !reflect.DeepEqual(table.cells, want) {
		t.Errorf("merged table is %q", table.cells)
	}
}

func TestNewRowWithCapitalisedEntriesAfterPageBreakStaysSeparate(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"Zaburzenia skóry", "Świąd", "Wysypka"},
	}}
	next := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"", "Pokrzywka", "Rumień"},
	}}

	table.absorb(next)

	if len(table.cells) != 2 {
		t.Errorf("capitalised new row was merged into %q", table.cells)
	}
}

func leadingMark(text string, x, y float64) TextMark {
	return TextMark{Text: text, FontSize: 10, BBox: model.PdfRectangle{Llx: x, Lly: y, Urx: x + 6*float64(len([]rune(text))), Ury: y + 10}}
}

func leadingTable() *mdLineTable {
	return &mdLineTable{xs: []float64{72, 155, 523}, ys: []float64{731, 693}, cells: [][]string{{"Niezbyt często:", "Ataksja stopnia 3"}}}
}

func TestContinuationLinesAboveTableBecomeFirstRow(t *testing.T) {
	table := leadingTable()
	marks := []TextMark{leadingMark("Ból", 160, 748), leadingMark("głowy", 184, 748), leadingMark("Zawroty", 160, 735), leadingMark("głowy", 208, 735)}
	used := make([]bool, len(marks))

	table.attachLeadingLines(marks, used)

	if len(table.cells) != 2 || table.cells[0][0] != "" || table.cells[0][1] != "Ból głowy<br>Zawroty głowy" || !used[0] || !used[3] {
		t.Errorf("table is %q, used %v", table.cells, used)
	}
}

func TestCaptionInLabelColumnStaysOutsideTable(t *testing.T) {
	table := leadingTable()
	marks := []TextMark{leadingMark("Tabela", 72, 745), leadingMark("2.", 112, 745)}
	used := make([]bool, len(marks))

	table.attachLeadingLines(marks, used)

	if len(table.cells) != 1 || used[0] {
		t.Errorf("caption was attached: %q", table.cells)
	}
}

func TestCaptionCrossingColumnsStaysOutsideTable(t *testing.T) {
	table := &mdLineTable{xs: []float64{72, 155, 200, 245, 523}, ys: []float64{731, 693}, cells: [][]string{{"B", "D1.", "D4.", "D8."}}}
	marks := []TextMark{leadingMark("Bortezomib", 170, 745), leadingMark("podawany", 236, 745)}
	used := make([]bool, len(marks))

	table.attachLeadingLines(marks, used)

	if len(table.cells) != 1 {
		t.Errorf("centred caption was split into cells: %q", table.cells)
	}
}

func TestDistantLinesAboveTableStayOutside(t *testing.T) {
	table := leadingTable()
	marks := []TextMark{leadingMark("Ból", 160, 790)}
	used := make([]bool, len(marks))

	table.attachLeadingLines(marks, used)

	if len(table.cells) != 1 {
		t.Errorf("distant line was attached: %q", table.cells)
	}
}

func TestTouchingTablesMergeByColumnCenters(t *testing.T) {
	header := &mdLineTable{xs: []float64{139.14, 188.28, 250.62, 333.54, 385.38}, ys: []float64{719.54, 666.2}, cells: [][]string{{"Brak", "Małe", "Umiarkowane", "Ciężkie"}}}
	body := &mdLineTable{xs: []float64{70.38, 140.61, 192.96, 255.78, 325.77, 384.28, 455.34, 516.78}, ys: []float64{665.84, 500.3}, cells: [][]string{{"Cmax", "8,1", "10,4", "10,5", "15,3", "15,4", "16,6"}}}

	if !header.absorb(body) || header.cols() != 7 || header.cells[0][1] != "Brak" || header.cells[0][4] != "Ciężkie" || header.cells[1][0] != "Cmax" {
		t.Errorf("got %q, want the header placed over the body columns", header.cells)
	}
}

func TestDistantTablesDoNotMergeByColumnCenters(t *testing.T) {
	header := &mdLineTable{xs: []float64{139.14, 188.28, 250.62, 333.54, 385.38}, ys: []float64{719.54, 690}, cells: [][]string{{"Brak", "Małe", "Umiarkowane", "Ciężkie"}}}
	body := &mdLineTable{xs: []float64{70.38, 140.61, 192.96, 255.78, 325.77, 384.28, 455.34, 516.78}, ys: []float64{665.84, 500.3}, cells: [][]string{{"Cmax", "8,1", "10,4", "10,5", "15,3", "15,4", "16,6"}}}

	if header.absorb(body) {
		t.Errorf("tables %v apart were merged", 690-665.84)
	}
}

func TestLabelContinuationWithNewFrequencyStartsRow(t *testing.T) {
	table := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"Zaburzenia psychiczne", "niezbyt często", "niepokój"},
		{"Zaburzenia układu", "często", "senność, zawroty głowy"},
	}}
	next := &mdLineTable{xs: []float64{100, 200, 300, 400}, cells: [][]string{
		{"nerwowego", "niezbyt często", "udar naczyniowy mózgu"},
		{"", "bardzo rzadko", "przeczulica"},
	}}

	table.absorb(next)

	if len(table.cells) != 4 || table.cells[1][0] != "Zaburzenia układu<br>nerwowego" || table.cells[1][1] != "często" || table.cells[2][1] != "niezbyt często" {
		t.Errorf("got %q", table.cells)
	}
}
