package extractor

import (
	"strings"
	"testing"
)

// The tolerance that lets a mark sit slightly outside the ruled grid must not be applied at the
// inner borders: neighbouring cells would overlap by twice its width and the first match would
// win, so a word starting just past a border loses its leading characters to the cell before it.
func TestFindCellDoesNotStealFromTheCellPastABorder(t *testing.T) {
	borders := []float64{77.24, 175.47, 286.49, 359.28, 430.56, 529.96}

	if got := mdFindCell(borders, 361.24, false); got != 3 {
		t.Errorf("a mark 1.96 past the border went to cell %d, want 3", got)
	}
	if got := mdFindCell(borders, 365.57, false); got != 3 {
		t.Errorf("the next mark of the same word went to cell %d, want 3", got)
	}
}

func TestFindCellKeepsTheToleranceAtTheOuterBorders(t *testing.T) {
	borders := []float64{100, 200, 300}

	if got := mdFindCell(borders, 98.5, false); got != 0 {
		t.Errorf("a mark just left of the grid went to cell %d, want 0", got)
	}
	if got := mdFindCell(borders, 301.5, false); got != 1 {
		t.Errorf("a mark just right of the grid went to cell %d, want 1", got)
	}
	if got := mdFindCell(borders, 90, false); got != -1 {
		t.Errorf("a mark well outside the grid went to cell %d, want -1", got)
	}
}

func TestFindCellOrdersDescendingBorders(t *testing.T) {
	borders := []float64{300, 200, 100}

	for value, want := range map[float64]int{250: 0, 150: 1, 201: 0, 199: 1} {
		if got := mdFindCell(borders, value, true); got != want {
			t.Errorf("value %.0f went to row %d, want %d", value, got, want)
		}
	}
}

// Two runs drawn over each other read as one interleaved run once the marks are sorted by x.
func TestJoinCellSeparatesRunsDrawnOverEachOther(t *testing.T) {
	marks := []mdCellMark{
		{179.16, 184.06, 663.60, "c"},
		{179.16, 187.13, 663.60, "w"},
		{184.08, 188.99, 663.60, "z"},
		{187.08, 191.98, 663.60, "ą"},
		{189.01, 194.53, 663.60, "y"},
		{192.00, 195.07, 663.60, "t"},
	}

	if got := mdJoinCell(marks); strings.Contains(got, "cwzą") {
		t.Errorf("the two runs came back interleaved: %q", got)
	}
}

// Ordinary kerning overlaps neighbouring glyph boxes and must not split a word into runs.
func TestJoinCellKeepsAKernedWordTogether(t *testing.T) {
	marks := []mdCellMark{
		{100.0, 108.0, 500.0, "m"},
		{107.4, 112.9, 500.0, "u"},
		{112.9, 116.0, 500.0, "i"},
	}

	if got := mdJoinCell(marks); got != "mui" {
		t.Errorf("a kerned word was reordered: %q, want %q", got, "mui")
	}
}
