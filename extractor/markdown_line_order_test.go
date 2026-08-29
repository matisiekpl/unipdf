package extractor

import "testing"

// Three marks each within the tolerance of their neighbour but spanning more than it end to
// end are what makes a tolerance comparator intransitive, so this is the shape that used to
// come back interleaved.
func TestLineOrderGroupsDriftingBaselinesIntoOneLine(t *testing.T) {
	y := []float64{100, 97, 94}
	x := []float64{30, 20, 10}

	order := mdLineOrder(len(y), func(i int) float64 { return y[i] }, func(i int) float64 { return x[i] }, 4)

	want := []int{2, 1, 0}
	for index, position := range order {
		if position != want[index] {
			t.Fatalf("order = %v, want %v (drifting baselines must read left to right)", order, want)
		}
	}
}

func TestLineOrderKeepsSeparateLinesApart(t *testing.T) {
	y := []float64{100, 100, 80, 80}
	x := []float64{50, 10, 60, 20}

	order := mdLineOrder(len(y), func(i int) float64 { return y[i] }, func(i int) float64 { return x[i] }, 4)

	want := []int{1, 0, 3, 2}
	for index, position := range order {
		if position != want[index] {
			t.Fatalf("order = %v, want %v (top line first, each read left to right)", order, want)
		}
	}
}

func TestJoinCellDoesNotInterleaveAdjacentLines(t *testing.T) {
	marks := []mdCellMark{
		{x0: 40, x1: 50, y: 97, s: "s"},
		{x0: 10, x1: 20, y: 100, s: "p"},
		{x0: 50, x1: 60, y: 94, s: "i"},
		{x0: 20, x1: 30, y: 99, s: "i"},
		{x0: 30, x1: 40, y: 98, s: "e"},
	}

	if got := mdJoinCell(marks); got != "pies i" && got != "piesi" {
		t.Errorf("mdJoinCell = %q, want the marks in left-to-right order", got)
	}
}
