/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package extractor

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// mdBlock is a single piece of page content: either a paragraph of text or a
// table reconstructed from ruling lines.
type mdBlock struct {
	text  string
	table *mdLineTable
}

// blocks returns the page content as an ordered list of text and table blocks.
func (pt PageText) blocks() []mdBlock {
	marks := pt.Marks().Elements()
	strokes := pt.strokes
	t := pt.lineTable()
	if t == nil {
		if txt := mdReconstructText(marks, strokes); txt != "" {
			return []mdBlock{{text: txt}}
		}
		return nil
	}

	var above, below []TextMark
	for _, m := range marks {
		cy := (m.BBox.Lly + m.BBox.Ury) / 2
		if cy > t.top()+2 {
			above = append(above, m)
		} else if cy < t.bottom()-2 {
			below = append(below, m)
		}
	}

	var blocks []mdBlock
	if txt := mdReconstructText(above, strokes); txt != "" {
		blocks = append(blocks, mdBlock{text: txt})
	}
	blocks = append(blocks, mdBlock{table: t})
	if txt := mdReconstructText(below, strokes); txt != "" {
		blocks = append(blocks, mdBlock{text: txt})
	}
	return blocks
}

// Markdown returns the page content as Markdown text. Paragraphs are joined in
// reading order, underlined runs are wrapped in <u></u> and tables that are
// drawn with ruling lines are reconstructed as Markdown tables (cell line
// breaks are encoded as <br>).
func (pt PageText) Markdown() string {
	return mdRenderBlocks(pt.blocks())
}

// DocumentMarkdown returns the Markdown for a whole document given its pages in
// order. Adjacent tables with no text between them and the same number of
// columns are merged into a single table. This reunites tables that span
// several pages.
//
// When joinSentences is true, lines that were broken mid-sentence are joined
// heuristically: if a line does not end with a period and the next line (at
// most one newline away) starts with a lower case letter or a digit, the two
// lines are joined.
func DocumentMarkdown(pages []*PageText, joinSentences bool) string {
	var blocks []mdBlock
	for _, pt := range pages {
		if pt == nil {
			continue
		}
		for _, blk := range pt.blocks() {
			if blk.table != nil && len(blocks) > 0 {
				if prev := &blocks[len(blocks)-1]; prev.table != nil && prev.table.cols() == blk.table.cols() {
					prev.table.cells = append(prev.table.cells, blk.table.cells...)
					continue
				}
			}
			blocks = append(blocks, blk)
		}
	}
	out := mdRenderBlocks(blocks)
	if joinSentences {
		out = mdJoinSentences(out)
	}
	return out
}

// mdJoinSentences joins lines that were split mid-sentence. A line is joined
// with the following one when it does not end with a period and that following
// line (directly below, with no blank line in between) starts with a lower case
// letter or a digit. Table rows and bullet list items are left untouched.
func mdJoinSentences(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	pendingBlanks := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			pendingBlanks++
			continue
		}
		if len(out) > 0 && pendingBlanks <= 1 && mdShouldJoin(out[len(out)-1], line) {
			out[len(out)-1] = strings.TrimRight(out[len(out)-1], " ") + " " + strings.TrimSpace(line)
		} else {
			for k := 0; k < pendingBlanks; k++ {
				out = append(out, "")
			}
			out = append(out, line)
		}
		pendingBlanks = 0
	}
	for k := 0; k < pendingBlanks; k++ {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// mdHeadingRegexp matches numbered section headings such as "1. NAZWA",
// "4.1 Wskazania" or "10. DATA" (a number, optional dotted subnumbers, then an
// upper case word). Such lines must never be joined to surrounding text.
var mdHeadingRegexp = regexp.MustCompile(`^\d+(\.\d+)*\.?\s+\p{Lu}`)

func mdShouldJoin(prev, cur string) bool {
	prev = strings.TrimRight(prev, " ")
	cur = strings.TrimSpace(cur)
	if prev == "" || cur == "" {
		return false
	}
	if strings.HasPrefix(prev, "|") || strings.HasPrefix(prev, "- ") {
		return false
	}
	if strings.HasSuffix(prev, ".") {
		return false
	}
	if mdHeadingRegexp.MatchString(prev) || mdHeadingRegexp.MatchString(cur) {
		return false
	}
	first := []rune(cur)[0]
	return unicode.IsDigit(first) || unicode.IsLower(first)
}

func mdRenderBlocks(blocks []mdBlock) string {
	var parts []string
	for _, blk := range blocks {
		if blk.table != nil {
			parts = append(parts, strings.TrimRight(blk.table.markdown(), "\n"))
		} else {
			parts = append(parts, blk.text)
		}
	}
	return strings.Join(parts, "\n\n") + "\n"
}

// mdLineTable is a table reconstructed from the ruling lines drawn on a page.
type mdLineTable struct {
	xs, ys []float64
	cells  [][]string
}

func (t *mdLineTable) cols() int       { return len(t.xs) - 1 }
func (t *mdLineTable) top() float64    { return t.ys[0] }
func (t *mdLineTable) bottom() float64 { return t.ys[len(t.ys)-1] }

func (t *mdLineTable) markdown() string {
	var b strings.Builder
	for r, row := range t.cells {
		b.WriteString("| " + strings.Join(row, " | ") + " |\n")
		if r == 0 {
			b.WriteString("|" + strings.Repeat(" --- |", len(row)) + "\n")
		}
	}
	return b.String()
}

type mdVSeg struct{ x, ylo, yhi float64 }
type mdHSeg struct{ y, x0, x1 float64 }

// lineTable reconstructs the table drawn with ruling lines on the page, or
// returns nil if no such table is found.
func (pt PageText) lineTable() *mdLineTable {
	var vxs []float64
	var vsegs []mdVSeg
	var hsegs []mdHSeg
	for _, s := range pt.strokes {
		if s.IsVertical() && mdAbs(s.Y2-s.Y1) > 20 {
			x := (s.X1 + s.X2) / 2
			lo, hi := mdMinMax(s.Y1, s.Y2)
			vxs = append(vxs, x)
			vsegs = append(vsegs, mdVSeg{x, lo, hi})
		}
		if s.IsHorizontal() {
			lo, hi := mdMinMax(s.X1, s.X2)
			hsegs = append(hsegs, mdHSeg{(s.Y1 + s.Y2) / 2, lo, hi})
		}
	}
	if len(vxs) == 0 {
		return nil
	}
	xs := mdCluster(vxs, 8)
	if len(xs) < 3 {
		return nil
	}
	tableWidth := xs[len(xs)-1] - xs[0]

	var hys []float64
	for _, h := range hsegs {
		hys = append(hys, h.y)
	}
	vmin, vmax := vsegs[0].ylo, vsegs[0].yhi
	for _, v := range vsegs {
		vmin = mdMin(vmin, v.ylo)
		vmax = mdMax(vmax, v.yhi)
	}
	var ys []float64
	for _, y := range mdPickRowBorders(mdCluster(hys, 3), hsegs, tableWidth) {
		if y >= vmin-3 && y <= vmax+3 {
			ys = append(ys, y)
		}
	}
	if len(ys) < 2 {
		return nil
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(ys)))

	cols := len(xs) - 1
	rows := len(ys) - 1
	t := &mdLineTable{xs: xs, ys: ys, cells: make([][]string, rows)}
	for r := range t.cells {
		t.cells[r] = make([]string, cols)
	}

	// For each row band determine which column borders actually exist (a vertical
	// segment covers most of the band). Consecutive present borders define a cell
	// that may span several columns (colspan).
	rowBorders := make([][]float64, rows)
	for r := 0; r < rows; r++ {
		top, bot := ys[r], ys[r+1]
		height := top - bot
		var present []float64
		for _, x := range xs {
			for _, v := range vsegs {
				if mdAbs(v.x-x) > 4 {
					continue
				}
				if mdMin(v.yhi, top)-mdMax(v.ylo, bot) > height*0.6 {
					present = append(present, x)
					break
				}
			}
		}
		if len(present) < 2 {
			present = []float64{xs[0], xs[len(xs)-1]}
		}
		rowBorders[r] = present
	}

	cellMarks := make(map[[2]int][]mdCellMark)
	for _, m := range pt.Marks().Elements() {
		if strings.TrimSpace(m.Text) == "" {
			continue
		}
		cx := (m.BBox.Llx + m.BBox.Urx) / 2
		cy := (m.BBox.Lly + m.BBox.Ury) / 2
		r := mdFindCell(ys, cy, true)
		if r < 0 {
			continue
		}
		c := mdFindCell(rowBorders[r], cx, false)
		if c < 0 {
			continue
		}
		gc := mdNearestIndex(xs, rowBorders[r][c])
		// Merge upward across rows when no horizontal border separates this cell
		// from the one above for this column span (rowspan).
		for r > 0 && !mdHasHBorder(hsegs, ys[r], xs[gc], rowBorders[r][c+1]) {
			r--
		}
		cellMarks[[2]int{r, gc}] = append(cellMarks[[2]int{r, gc}],
			mdCellMark{m.BBox.Llx, m.BBox.Urx, cy, m.Text})
	}
	for key, cms := range cellMarks {
		t.cells[key[0]][key[1]] = mdJoinCell(cms)
	}
	return t
}

type mdCellMark struct {
	x0, x1, y float64
	s         string
}

func mdJoinCell(marks []mdCellMark) string {
	sort.Slice(marks, func(i, j int) bool {
		if mdAbs(marks[i].y-marks[j].y) > 4 {
			return marks[i].y > marks[j].y
		}
		return marks[i].x0 < marks[j].x0
	})
	var lines []string
	var cur strings.Builder
	var lastY, lastX1 float64
	first := true
	for _, m := range marks {
		if !first && mdAbs(m.y-lastY) > 4 {
			lines = append(lines, strings.TrimSpace(cur.String()))
			cur.Reset()
		} else if !first && m.x0-lastX1 > 1.2 {
			cur.WriteString(" ")
		}
		cur.WriteString(m.s)
		lastY = m.y
		lastX1 = m.x1
		first = false
	}
	if cur.Len() > 0 {
		lines = append(lines, strings.TrimSpace(cur.String()))
	}
	return strings.Join(lines, "<br>")
}

type mdWord struct {
	x0, x1, baseline, y float64
	s                   string
}

func mdReconstructText(marks []TextMark, strokes []Stroke) string {
	type lineMark struct {
		x0, x1, baseline, y float64
		s                   string
	}
	var lms []lineMark
	for _, m := range marks {
		if strings.TrimSpace(m.Text) == "" {
			continue
		}
		lms = append(lms, lineMark{m.BBox.Llx, m.BBox.Urx, m.BBox.Lly,
			(m.BBox.Lly + m.BBox.Ury) / 2, m.Text})
	}
	if len(lms) == 0 {
		return ""
	}
	sort.Slice(lms, func(i, j int) bool {
		if mdAbs(lms[i].y-lms[j].y) > 3 {
			return lms[i].y > lms[j].y
		}
		return lms[i].x0 < lms[j].x0
	})

	var lines [][]mdWord
	var lineYs []float64
	var curWords []mdWord
	var curW mdWord
	haveW := false
	var lastY, lastX1 float64
	first := true
	flushWord := func() {
		if haveW {
			curWords = append(curWords, curW)
			haveW = false
		}
	}
	flushLine := func() {
		flushWord()
		if len(curWords) > 0 {
			lines = append(lines, curWords)
			lineYs = append(lineYs, lastY)
			curWords = nil
		}
	}
	for _, m := range lms {
		if !first && mdAbs(m.y-lastY) > 3 {
			flushLine()
		} else if !first && m.x0-lastX1 > 1.2 {
			flushWord()
		}
		if !haveW {
			curW = mdWord{x0: m.x0, x1: m.x1, baseline: m.baseline, y: m.y, s: m.s}
			haveW = true
		} else {
			curW.x1 = m.x1
			curW.s += m.s
		}
		lastY = m.y
		lastX1 = m.x1
		first = false
	}
	flushLine()

	rendered := make([]string, len(lines))
	for i, lineWords := range lines {
		rendered[i] = mdRenderLine(lineWords, strokes)
	}

	var b strings.Builder
	for i, line := range rendered {
		bullet := strings.HasPrefix(line, "•")
		if i > 0 {
			if bullet {
				b.WriteString("\n")
			} else if lineYs[i-1]-lineYs[i] > 18 {
				b.WriteString("\n\n")
			} else {
				b.WriteString(" ")
			}
		}
		if bullet {
			b.WriteString("- " + strings.TrimSpace(strings.TrimPrefix(line, "•")))
		} else {
			b.WriteString(line)
		}
	}
	return b.String()
}

// mdRenderLine renders a single line, coalescing consecutive words that share
// the same styling into a single run (e.g. "Sposób podawania" -> one <u></u>).
func mdRenderLine(lineWords []mdWord, strokes []Stroke) string {
	var b strings.Builder
	for j := 0; j < len(lineWords); {
		if j > 0 {
			b.WriteString(" ")
		}
		underlined := mdUnderlined(lineWords[j], strokes)
		k := j + 1
		for k < len(lineWords) && mdUnderlined(lineWords[k], strokes) == underlined {
			k++
		}
		var parts []string
		for _, w := range lineWords[j:k] {
			parts = append(parts, w.s)
		}
		run := strings.Join(parts, " ")
		if underlined {
			b.WriteString("<u>" + run + "</u>")
		} else {
			b.WriteString(run)
		}
		j = k
	}
	return b.String()
}

func mdUnderlined(w mdWord, strokes []Stroke) bool {
	width := w.x1 - w.x0
	if width <= 0 {
		return false
	}
	for _, s := range strokes {
		if !s.IsHorizontal() {
			continue
		}
		y := (s.Y1 + s.Y2) / 2
		if y > w.baseline+1 || y < w.baseline-5 {
			continue
		}
		lo := mdMax(mdMin(s.X1, s.X2), w.x0)
		hi := mdMin(mdMax(s.X1, s.X2), w.x1)
		if hi-lo > width*0.6 {
			return true
		}
	}
	return false
}

func mdPickRowBorders(yCandidates []float64, hsegs []mdHSeg, tableWidth float64) []float64 {
	var out []float64
	for _, y := range yCandidates {
		var intervals [][2]float64
		for _, h := range hsegs {
			if mdAbs(h.y-y) <= 3 {
				intervals = append(intervals, [2]float64{h.x0, h.x1})
			}
		}
		if mdUnionLen(intervals) > tableWidth*0.4 {
			out = append(out, y)
		}
	}
	return out
}

func mdHasHBorder(hsegs []mdHSeg, y, x0, x1 float64) bool {
	var intervals [][2]float64
	for _, h := range hsegs {
		if mdAbs(h.y-y) <= 3 {
			lo := mdMax(h.x0, x0)
			hi := mdMin(h.x1, x1)
			if hi > lo {
				intervals = append(intervals, [2]float64{lo, hi})
			}
		}
	}
	return mdUnionLen(intervals) > (x1-x0)*0.6
}

func mdUnionLen(intervals [][2]float64) float64 {
	if len(intervals) == 0 {
		return 0
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i][0] < intervals[j][0] })
	total := 0.0
	curLo, curHi := intervals[0][0], intervals[0][1]
	for _, iv := range intervals[1:] {
		if iv[0] <= curHi {
			if iv[1] > curHi {
				curHi = iv[1]
			}
		} else {
			total += curHi - curLo
			curLo, curHi = iv[0], iv[1]
		}
	}
	return total + curHi - curLo
}

func mdFindCell(borders []float64, v float64, descending bool) int {
	for i := 0; i+1 < len(borders); i++ {
		if descending {
			if v <= borders[i]+2 && v >= borders[i+1]-2 {
				return i
			}
		} else if v >= borders[i]-2 && v <= borders[i+1]+2 {
			return i
		}
	}
	return -1
}

func mdNearestIndex(xs []float64, x float64) int {
	best, bestD := 0, 1e9
	for i, v := range xs {
		if d := mdAbs(v - x); d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

func mdCluster(vals []float64, tol float64) []float64 {
	if len(vals) == 0 {
		return nil
	}
	sort.Float64s(vals)
	var out []float64
	group := []float64{vals[0]}
	for _, v := range vals[1:] {
		if v-group[len(group)-1] <= tol {
			group = append(group, v)
		} else {
			out = append(out, mdMean(group))
			group = []float64{v}
		}
	}
	return append(out, mdMean(group))
}

func mdMean(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func mdMinMax(a, b float64) (float64, float64) {
	if a > b {
		return b, a
	}
	return a, b
}

func mdAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func mdMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func mdMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
