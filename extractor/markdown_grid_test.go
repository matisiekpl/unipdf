package extractor

import (
	"strings"
	"testing"

	"github.com/matisiekpl/unipdf/v3/model"
)

func gridPage(strokes []Stroke, cellRects []Rect, whiteCellRects []Rect, marks ...TextMark) *PageText {
	return &PageText{
		viewMarks:      marks,
		strokes:        strokes,
		cellRects:      cellRects,
		whiteCellRects: whiteCellRects,
		pageSize:       model.PdfRectangle{Llx: 0, Lly: 0, Urx: testPageWidth, Ury: testPageHeight},
	}
}

func horizontalRule(y, x0, x1 float64) Stroke {
	return Stroke{X1: x0, Y1: y, X2: x1, Y2: y}
}

func verticalRule(x, y0, y1 float64) Stroke {
	return Stroke{X1: x, Y1: y0, X2: x, Y2: y1}
}

func rowRules(columns []float64, rows []float64) []Stroke {
	var strokes []Stroke
	for _, y := range rows {
		strokes = append(strokes, horizontalRule(y, columns[0], columns[len(columns)-1]))
	}
	return strokes
}

func segmentedColumnRules(columns []float64, rows []float64) []Stroke {
	var strokes []Stroke
	for _, x := range columns {
		for index := 1; index < len(rows); index++ {
			strokes = append(strokes, verticalRule(x, rows[index], rows[index-1]))
		}
	}
	return strokes
}

func cellMarks(columns []float64, rows []float64, texts [][]string) []TextMark {
	var marks []TextMark
	for rowIndex, row := range texts {
		center := (rows[rowIndex] + rows[rowIndex+1]) / 2
		for columnIndex, text := range row {
			if text != "" {
				marks = append(marks, testMark(text, columns[columnIndex]+5, center))
			}
		}
	}
	return marks
}

func assertContainsRows(t *testing.T, markdown string, rows ...string) {
	t.Helper()
	for _, row := range rows {
		if !strings.Contains(markdown, row) {
			t.Errorf("missing row %q in:\n%s", row, markdown)
		}
	}
}

var (
	gridColumns = []float64{100, 200, 300}
	gridRows    = []float64{700, 680, 660, 640}
	gridTexts   = [][]string{{"a1", "b1"}, {"a2", "b2"}, {"a3", "b3"}}
)

func TestColumnRulesDrawnOnePiecePerRowFormTable(t *testing.T) {
	strokes := append(rowRules(gridColumns, gridRows), segmentedColumnRules(gridColumns, gridRows)...)
	page := gridPage(strokes, nil, nil, cellMarks(gridColumns, gridRows, gridTexts)...)

	assertContainsRows(t, page.Markdown(), "| a1 | b1 |", "| a2 | b2 |", "| a3 | b3 |")
}

func TestTableRuleWiderThanEightyPercentOfPageIsKept(t *testing.T) {
	columns := []float64{40, 300, 555}
	strokes := rowRules(columns, gridRows)
	for _, x := range columns {
		strokes = append(strokes, verticalRule(x, gridRows[len(gridRows)-1], gridRows[0]))
	}
	page := gridPage(strokes, nil, nil, cellMarks(columns, gridRows, gridTexts)...)

	assertContainsRows(t, page.Markdown(), "| a1 | b1 |", "| a3 | b3 |")
}

func TestPageFrameIsDropped(t *testing.T) {
	frame := []Stroke{
		horizontalRule(5, 2, testPageWidth-2),
		horizontalRule(testPageHeight-5, 2, testPageWidth-2),
		verticalRule(2, 2, testPageHeight-2),
		verticalRule(testPageWidth-2, 2, testPageHeight-2),
	}
	page := gridPage(frame, nil, nil)

	if strokes := page.cleanStrokes(); len(strokes) != 0 {
		t.Errorf("page frame survived as %d strokes", len(strokes))
	}
}

func clipGrid(columns []float64, rows []float64, gap float64) []Rect {
	var rects []Rect
	for rowIndex := 1; rowIndex < len(rows); rowIndex++ {
		for columnIndex := 1; columnIndex < len(columns); columnIndex++ {
			rects = append(rects, Rect{Llx: columns[columnIndex-1], Lly: rows[rowIndex], Urx: columns[columnIndex] - gap, Ury: rows[rowIndex-1]})
		}
	}
	return rects
}

func TestClippedCellsGiveColumnsToTableWithOnlyOuterRules(t *testing.T) {
	strokes := []Stroke{horizontalRule(gridRows[0], 100, 300), horizontalRule(gridRows[len(gridRows)-1], 100, 300)}
	page := gridPage(strokes, clipGrid(gridColumns, gridRows, 0.48), nil, cellMarks(gridColumns, gridRows, gridTexts)...)

	assertContainsRows(t, page.Markdown(), "| a1 | b1 |", "| a2 | b2 |", "| a3 | b3 |")
}

func TestCellsSeparatedByNarrowGapsFormGrid(t *testing.T) {
	page := gridPage(nil, clipGrid(gridColumns, gridRows, 2.16), nil)

	if strokes := page.tiledCellStrokes(); len(strokes) == 0 {
		t.Errorf("cells with 2.16 point gaps produced no grid")
	}
}

func TestWordClipsOfJustifiedLinesDoNotFormGrid(t *testing.T) {
	var rects []Rect
	lines := [][]float64{{100, 140, 143, 190, 193, 230}, {100, 125, 128, 200, 203, 230}, {100, 160, 163, 180, 183, 230}}
	for lineIndex, edges := range lines {
		top := 700 - float64(lineIndex)*12
		for index := 0; index < len(edges); index += 2 {
			rects = append(rects, Rect{Llx: edges[index], Lly: top - 10, Urx: edges[index+1], Ury: top})
		}
	}
	page := gridPage(nil, rects, nil)

	if strokes := page.tiledCellStrokes(); len(strokes) != 0 {
		t.Errorf("clips around words of justified lines produced %d grid strokes", len(strokes))
	}
}

func TestSingleRowOfTouchingClipsDoesNotFormGrid(t *testing.T) {
	rects := []Rect{{Llx: 100, Lly: 690, Urx: 150, Ury: 700}, {Llx: 150, Lly: 690, Urx: 200, Ury: 700}, {Llx: 200, Lly: 690, Urx: 250, Ury: 700}}
	page := gridPage(nil, rects, nil)

	if strokes := page.tiledCellStrokes(); len(strokes) != 0 {
		t.Errorf("one row of touching clips produced %d grid strokes", len(strokes))
	}
}

func TestWhiteCellsGiveColumnsOnlyWithoutColumnRules(t *testing.T) {
	cells := clipGrid(gridColumns, gridRows, 0)
	unruled := gridPage(nil, nil, cells)
	ruled := gridPage([]Stroke{verticalRule(150, 600, 720)}, nil, cells)

	if got := len(unruled.unruledWhiteCellRects()); got != len(cells) {
		t.Errorf("unruled page kept %d of %d white cells", got, len(cells))
	}
	if got := len(ruled.unruledWhiteCellRects()); got != 0 {
		t.Errorf("page with a column rule across the cells kept %d white cells", got)
	}
}

func TestOuterBorderComesFromRulesReachingBeyondColumns(t *testing.T) {
	columns := []float64{100, 200, 300, 400}
	shaded := clipGrid(columns[1:], gridRows, 0)
	strokes := rowRules(columns, gridRows)
	texts := [][]string{{"a1", "b1", "c1"}, {"a2", "b2", "c2"}, {"a3", "b3", "c3"}}
	page := gridPage(strokes, shaded, nil, cellMarks(columns, gridRows, texts)...)

	assertContainsRows(t, page.Markdown(), "| a1 | b1 | c1 |", "| a3 | b3 | c3 |")
}

func TestOuterBorderNeedsTwoRules(t *testing.T) {
	hsegs := []mdHSeg{{y: 700, x0: 100, x1: 400}, {y: 640, x0: 200, x1: 400}}

	if borders := mdImplicitOuterBorders(hsegs, []float64{200, 300, 400}, 640, 700); len(borders) != 0 {
		t.Errorf("one rule reaching left gave borders %v", borders)
	}
}

func TestBorderFromCellsIsKeptInRowWithoutCellBackground(t *testing.T) {
	columns := []float64{100, 200, 300}
	rows := []float64{720, 690, 660, 630}
	var shaded []Rect
	for _, rowIndex := range []int{1, 3} {
		for columnIndex := 1; columnIndex < len(columns); columnIndex++ {
			shaded = append(shaded, Rect{Llx: columns[columnIndex-1], Lly: rows[rowIndex], Urx: columns[columnIndex], Ury: rows[rowIndex-1]})
		}
	}
	page := gridPage(rowRules(columns, rows), shaded, nil, cellMarks(columns, rows, gridTexts)...)

	assertContainsRows(t, page.Markdown(), "| a2 | b2 |")
}

func TestProseBetweenShadedRowsStaysProse(t *testing.T) {
	columns := []float64{100, 200, 300}
	rows := []float64{720, 690, 660, 630}
	var shaded []Rect
	for _, rowIndex := range []int{1, 3} {
		for columnIndex := 1; columnIndex < len(columns); columnIndex++ {
			shaded = append(shaded, Rect{Llx: columns[columnIndex-1], Lly: rows[rowIndex], Urx: columns[columnIndex], Ury: rows[rowIndex-1]})
		}
	}
	marks := []TextMark{
		testMark("a1", 105, 705), testMark("b1", 205, 705),
		testMark("sentence-across-columns", 105, 675),
		testMark("a3", 105, 645), testMark("b3", 205, 645),
	}
	page := gridPage(rowRules(columns, rows), shaded, nil, marks...)

	markdown := page.Markdown()
	if strings.Contains(markdown, "| sentence") {
		t.Errorf("a line crossing the column border was split into cells:\n%s", markdown)
	}
	if !strings.Contains(markdown, "sentence-across-columns") {
		t.Errorf("the line between the rows was lost:\n%s", markdown)
	}
}

func TestMissingColumnRuleInRowIsColspan(t *testing.T) {
	columns := []float64{100, 200, 300, 400}
	strokes := rowRules(columns, gridRows)
	strokes = append(strokes, verticalRule(100, 640, 700), verticalRule(400, 640, 700), verticalRule(200, 640, 700), verticalRule(300, 640, 680))
	marks := []TextMark{
		testMark("head", 105, 690), testMark("spanning", 205, 690),
		testMark("a2", 105, 670), testMark("b2", 205, 670), testMark("c2", 305, 670),
		testMark("a3", 105, 650), testMark("b3", 205, 650), testMark("c3", 305, 650),
	}
	page := gridPage(strokes, nil, nil, marks...)

	assertContainsRows(t, page.Markdown(), "| head | spanning |  |", "| a2 | b2 | c2 |")
}

func TestBorderFromCellsIsNotForcedThroughWord(t *testing.T) {
	straddles := func(marks []TextMark, x float64) bool {
		words, _ := mdWords(marks, nil)
		return mdWordStraddles(words, 660, 640, x)
	}
	if !straddles([]TextMark{testMark("Zaburzenia", 100, 650)}, 130) {
		t.Errorf("a border through the middle of a word was not detected")
	}
	if !straddles([]TextMark{testMark("Zab", 100, 650), testMark("urz", 118.5, 650)}, 118.2) {
		t.Errorf("a border between two glyphs of one word was not detected")
	}
	if straddles([]TextMark{testMark("a1", 100, 650), testMark("b1", 140, 650)}, 125) {
		t.Errorf("a border in the space between words was taken as straddling")
	}
}

func TestWordsGroupGlyphsOfOneLineWhateverTheirOrder(t *testing.T) {
	marks := []TextMark{
		testMark("rz", 112, 650.5), testMark("x", 100, 630), testMark("Zabu", 88, 650), testMark(" ", 124, 650),
		testMark("enia", 130, 649.5), testMark("y", 106, 630.2), testMark("serca", 160, 650),
	}
	words, markWord := mdWords(marks, nil)

	if len(words) != 4 {
		t.Fatalf("got %d words, want 4: %+v", len(words), words)
	}
	if markWord[0] != markWord[2] || markWord[1] != markWord[5] || markWord[2] == markWord[4] || markWord[3] != -1 {
		t.Errorf("glyphs grouped as %v, want rz with Zabu, x with y, enia apart after a space, space in no word", markWord)
	}
	first := words[markWord[2]]
	if first.x0 != 88 || first.x1 != 124 {
		t.Errorf("first word spans %.1f..%.1f, want 88..124", first.x0, first.x1)
	}
}

func TestWordOverhangingColumnRuleStaysWhole(t *testing.T) {
	columns := []float64{100, 194.6, 300}
	strokes := rowRules(columns, gridRows)
	for _, x := range columns {
		strokes = append(strokes, verticalRule(x, 640, 700))
	}
	marks := []TextMark{
		testMark("ab", 105, 690), testMark("4", 190, 690), testMark(".4).", 196, 690),
		testMark("cd", 105, 670), testMark("(", 190, 670), testMark("nie", 196, 670),
		testMark("ef", 105, 650), testMark("gh", 205, 650),
	}
	page := gridPage(strokes, nil, nil, marks...)

	assertContainsRows(t, page.Markdown(), "| ab | 4.4). |", "| cd | (nie |", "| ef | gh |")
}

func TestTextTouchingColumnRuleFromBothSidesStaysInItsCells(t *testing.T) {
	columns := []float64{100, 180.24, 300}
	strokes := rowRules(columns, gridRows)
	for _, x := range columns {
		strokes = append(strokes, verticalRule(x, 640, 700))
	}
	marks := []TextMark{
		testMark("klatki", 143.86, 690), testMark("Często", 180.24, 690),
		testMark("ab", 105, 670), testMark("cd", 205, 670),
		testMark("ef", 105, 650), testMark("gh", 205, 650),
	}
	page := gridPage(strokes, nil, nil, marks...)

	assertContainsRows(t, page.Markdown(), "| klatki | Często |")
}

func TestRowCutByPageBreakStaysInTable(t *testing.T) {
	strokes := rowRules(gridColumns, gridRows[:3])
	for _, x := range gridColumns {
		strokes = append(strokes, verticalRule(x, 640, 700))
	}
	page := gridPage(strokes, nil, nil, cellMarks(gridColumns, gridRows, gridTexts)...)

	assertContainsRows(t, page.Markdown(), "| a1 | b1 |", "| a3 | b3 |")
}

func TestMergeCollinearVerticals(t *testing.T) {
	merged := mdMergeCollinearVerticals([]Stroke{
		verticalRule(100, 600, 612), verticalRule(100, 612.5, 625), verticalRule(100, 630, 640), verticalRule(200, 600, 612),
	})
	var verticals []Stroke
	for _, stroke := range merged {
		if stroke.IsVertical() {
			verticals = append(verticals, stroke)
		}
	}
	if len(verticals) != 3 {
		t.Fatalf("got %d verticals, want 3: %+v", len(verticals), verticals)
	}
	if verticals[0].Y1 != 600 || verticals[0].Y2 != 625 {
		t.Errorf("touching segments merged into %.1f..%.1f, want 600..625", verticals[0].Y1, verticals[0].Y2)
	}
}

func TestMergeCollinearVerticalsKeepsRowSegmentsOfDoubleDrawnBorder(t *testing.T) {
	var strokes []Stroke
	for _, coordinates := range [][4]float64{
		{538.96, 750.95, 538.96, 766.45}, {538.96, 734.42, 538.96, 750.44}, {538.96, 717.92, 538.96, 733.92}, {538.96, 701.42, 538.96, 717.42},
		{538.96, 685.40, 538.96, 700.92}, {538.96, 668.90, 538.96, 684.90}, {538.96, 652.38, 538.96, 668.40}, {538.96, 636.38, 538.96, 651.88},
		{538.46, 798.97, 538.46, 799.47}, {538.45, 798.97, 538.95, 798.97}, {538.95, 799.47, 538.45, 799.47}, {538.45, 799.47, 538.45, 798.97},
		{538.45, 766.95, 538.95, 766.95}, {538.95, 766.95, 538.95, 798.98}, {538.45, 798.98, 538.45, 766.95}, {538.45, 750.95, 538.95, 750.95},
		{538.45, 766.45, 538.45, 750.95}, {538.45, 734.42, 538.95, 734.42}, {538.45, 750.44, 538.45, 734.42}, {538.46, 733.92, 538.46, 734.42},
		{538.45, 733.92, 538.95, 733.92}, {538.45, 734.42, 538.45, 733.92}, {538.45, 717.92, 538.95, 717.92}, {538.45, 733.92, 538.45, 717.92},
		{538.45, 701.42, 538.95, 701.42}, {538.45, 717.42, 538.45, 701.42}, {538.46, 700.92, 538.46, 701.42}, {538.45, 700.92, 538.95, 700.92},
		{538.45, 701.42, 538.45, 700.92}, {538.45, 685.40, 538.95, 685.40}, {538.45, 700.92, 538.45, 685.40}, {538.46, 684.90, 538.46, 685.40},
		{538.45, 684.90, 538.95, 684.90}, {538.45, 685.40, 538.45, 684.90}, {538.45, 668.90, 538.95, 668.90}, {538.45, 684.90, 538.45, 668.90},
		{538.45, 652.38, 538.95, 652.38}, {538.45, 668.40, 538.45, 652.38}, {538.46, 651.88, 538.46, 652.38}, {538.45, 651.88, 538.95, 651.88},
		{538.45, 652.38, 538.45, 651.88}, {538.46, 635.88, 538.46, 636.38}, {538.45, 636.38, 538.95, 636.38}, {538.45, 651.88, 538.45, 636.38},
		{538.45, 635.88, 538.95, 635.88}, {538.45, 636.38, 538.45, 635.88},
	} {
		strokes = append(strokes, Stroke{X1: coordinates[0], Y1: coordinates[1], X2: coordinates[2], Y2: coordinates[3]})
	}
	var verticals []Stroke
	for _, stroke := range mdMergeCollinearVerticals(strokes) {
		if stroke.IsVertical() && mdAbs(stroke.Y2-stroke.Y1) > 1 {
			verticals = append(verticals, stroke)
		}
	}

	if len(verticals) != 1 || verticals[0].Y1 != 635.88 || verticals[0].Y2 != 799.47 {
		t.Errorf("right border of a table drawn as two thin lines per row merged into %+v, want one rule 635.88..799.47", verticals)
	}
}

func TestMergeCollinearHorizontalsWithInterleavedNearbyLines(t *testing.T) {
	merged := mdMergeCollinearHorizontals([]mdHSeg{
		{y: 500.4, x0: 300, x1: 400}, {y: 500, x0: 100, x1: 200}, {y: 500.8, x0: 199, x1: 301}, {y: 500.2, x0: 450, x1: 500},
	})

	if len(merged) != 2 || merged[0].x0 != 100 || merged[0].x1 != 400 || merged[1].x0 != 450 {
		t.Errorf("rule drawn in pieces at slightly different heights merged into %+v, want 100..400 and 450..500", merged)
	}
}

func TestMergeCollinearHorizontals(t *testing.T) {
	merged := mdMergeCollinearHorizontals([]mdHSeg{{y: 700, x0: 100, x1: 175.3}, {y: 700.2, x0: 175.6, x1: 303}, {y: 700, x0: 320, x1: 400}, {y: 650, x0: 100, x1: 200}})

	if len(merged) != 3 {
		t.Fatalf("got %d segments, want 3: %+v", len(merged), merged)
	}
}

func TestUnderlineOnRowBorderDoesNotSplitRowspanCell(t *testing.T) {
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

	assertContainsRows(t, page.Markdown(), "| Zaburzenia nerek<br>i dróg | n1 |", "|  | n2 |")
}

func TestRuleBelowTextReachingColumnBordersIsNotUnderline(t *testing.T) {
	words := []mdWord{{x0: 100.5, x1: 199.5, baseline: 685, y: 690}}

	if mdUnderline(mdHSeg{y: 682, x0: 100, x1: 200}, words, []float64{100, 200, 300}) {
		t.Errorf("cell border under text filling the cell was taken for an underline")
	}
	if !mdUnderline(mdHSeg{y: 682, x0: 105, x1: 150}, []mdWord{{x0: 105, x1: 150, baseline: 685, y: 690}}, []float64{100, 200, 300}) {
		t.Errorf("underline under a word inside the cell was not detected")
	}
	if mdUnderline(mdHSeg{y: 682, x0: 105, x1: 190}, []mdWord{{x0: 105, x1: 150, baseline: 685, y: 690}}, []float64{100, 200, 300}) {
		t.Errorf("rule reaching far beyond the text was taken for an underline")
	}
}

func TestProseBelowNarrowTableIsNeitherSwallowedNorDuplicated(t *testing.T) {
	strokes := []Stroke{
		horizontalRule(700, 100, 260), horizontalRule(680, 100, 260), horizontalRule(660, 100, 260),
		verticalRule(100, 660, 700), verticalRule(260, 660, 700),
		horizontalRule(600, 100, 400), horizontalRule(580, 100, 400), horizontalRule(560, 100, 400),
	}
	for _, x := range []float64{100, 200, 300, 400} {
		strokes = append(strokes, verticalRule(x, 560, 600))
	}
	marks := []TextMark{
		testMark("a1", 105, 690), testMark("a2", 105, 670),
		testMark("prose", 105, 630),
		testMark("b1", 105, 590), testMark("c1", 205, 590), testMark("d1", 305, 590),
		testMark("b2", 105, 570), testMark("c2", 205, 570), testMark("d2", 305, 570),
	}
	markdown := gridPage(strokes, nil, nil, marks...).Markdown()

	if strings.Count(markdown, "prose") != 1 || strings.Contains(markdown, "a2<br>prose") {
		t.Errorf("prose below the narrow table rendered as:\n%s", markdown)
	}
}

func TestPipeInsideCellDoesNotSplitIt(t *testing.T) {
	columns := []float64{100, 200, 300}
	strokes := rowRules(columns, []float64{700, 680, 660})
	for _, x := range columns {
		strokes = append(strokes, verticalRule(x, 660, 700))
	}
	page := gridPage(strokes, nil, nil,
		testMark("Zmiana", 105, 690), testMark("|Δ|", 205, 690),
		testMark("a", 105, 670), testMark("b", 205, 670))

	assertContainsRows(t, page.Markdown(), "| Zmiana | ¦Δ¦ |")
}

func TestTableWithOnlyMiddleRuleUsesRulesReachingBothSides(t *testing.T) {
	strokes := []Stroke{
		horizontalRule(700, 70, 540), horizontalRule(660, 70, 540), horizontalRule(620, 70, 540),
		verticalRule(308, 620, 700),
	}
	marks := []TextMark{
		testMark("Toksyczność", 75, 690), testMark("Modyfikacja", 315, 690),
		testMark("Neutropenia", 75, 640), testMark("Wstrzymać", 315, 640),
	}

	assertContainsRows(t, gridPage(strokes, nil, nil, marks...).Markdown(), "| Toksyczność | Modyfikacja |", "| Neutropenia | Wstrzymać |")
}

func TestTwoLevelHeaderWithRuleUnderSpanningCell(t *testing.T) {
	strokes := []Stroke{
		horizontalRule(720, 70, 520), horizontalRule(690, 350, 520), horizontalRule(660, 70, 520), horizontalRule(640, 70, 520),
	}
	for _, x := range []float64{70, 170, 350, 520} {
		strokes = append(strokes, verticalRule(x, 640, 720))
	}
	strokes = append(strokes, verticalRule(430, 640, 690))
	marks := []TextMark{
		testMark("Klasyfikacja", 75, 705), testMark("Działania", 175, 705), testMark("Częstość", 380, 705),
		testMark("MedDRA", 75, 675), testMark("Amlodypina", 355, 675), testMark("Peryndopryl", 435, 675),
		testMark("Zakażenia", 75, 650), testMark("Zapalenie", 175, 650), testMark("Rzadko", 355, 650), testMark("Często", 435, 650),
	}

	assertContainsRows(t, gridPage(strokes, nil, nil, marks...).Markdown(),
		"| Klasyfikacja<br>MedDRA | Działania | Częstość |  |",
		"|  |  | Amlodypina | Peryndopryl |",
		"| Zakażenia | Zapalenie | Rzadko | Często |")
}

func TestLineClipsInsideRuledCellDoNotSplitRow(t *testing.T) {
	strokes := []Stroke{
		horizontalRule(700, 70, 400), horizontalRule(650, 70, 400), horizontalRule(600, 70, 400), horizontalRule(669, 250, 400),
		verticalRule(70, 600, 700), verticalRule(180, 600, 700), verticalRule(250, 600, 700), verticalRule(400, 600, 700),
	}
	clips := []Rect{
		{Llx: 70.5, Lly: 663, Urx: 179.8, Ury: 675.5}, {Llx: 180.2, Lly: 663, Urx: 249.8, Ury: 675.5},
		{Llx: 70.5, Lly: 650.5, Urx: 179.8, Ury: 663}, {Llx: 180.2, Lly: 650.5, Urx: 249.8, Ury: 663},
	}
	marks := []TextMark{
		testMark("Nagłówek", 75, 690), testMark("Kolumna", 185, 690), testMark("Pierwsza", 255, 690),
		testMark("Zaburzenia", 75, 669), testMark("Rzadko", 185, 669), testMark("Wysypka", 255, 687),
		testMark("skóry", 75, 657), testMark("Pokrzywka", 255, 662),
		testMark("Inne", 75, 625), testMark("Często", 185, 625), testMark("Ból", 255, 625),
	}
	page := gridPage(strokes, clips, nil, marks...)

	markdown := page.Markdown()
	if !strings.Contains(markdown, "Zaburzenia<br>skóry") || strings.Contains(markdown, "|  |  | Wysypka |") {
		t.Errorf("line clips split the ruled cell:\n%s", markdown)
	}
}
