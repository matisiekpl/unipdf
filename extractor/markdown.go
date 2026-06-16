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

// cleanStrokes returns the page strokes with exact duplicates removed and with
// page-spanning strokes (clipping rectangles and page borders/backgrounds that
// some PDFs draw repeatedly) dropped, so they don't pollute table detection.
func (pt PageText) cleanStrokes() []Stroke {
	pageWidth := pt.pageSize.Urx - pt.pageSize.Llx
	pageHeight := pt.pageSize.Ury - pt.pageSize.Lly
	seen := make(map[[4]int]bool)
	var out []Stroke
	for _, s := range pt.strokes {
		if s.IsVertical() && mdAbs(s.Y2-s.Y1) > pageHeight*0.8 {
			continue
		}
		if s.IsHorizontal() && mdAbs(s.X2-s.X1) > pageWidth*0.8 {
			continue
		}
		key := [4]int{int(s.X1), int(s.Y1), int(s.X2), int(s.Y2)}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

// marginBand reports whether device-y `cy` is inside the top or bottom margin
// band of the page, where running headers and page numbers live.
func (pt PageText) marginBand(cy float64) bool {
	pageHeight := pt.pageSize.Ury - pt.pageSize.Lly
	return cy < pt.pageSize.Lly+pageHeight*0.08 || cy > pt.pageSize.Ury-pageHeight*0.08
}

// mdMarginLine is a text line located entirely in a top or bottom margin band,
// where running headers/footers and page numbers live.
type mdMarginLine struct {
	text    string
	indices []int
}

// marginLines groups the marks in the page's margin bands into lines (marks at
// the same vertical position), returning each line's text and the indices of
// its marks in pt.Marks().
func (pt PageText) marginLines() []mdMarginLine {
	marks := pt.Marks().Elements()
	used := make(map[int]bool)
	var lines []mdMarginLine
	for i := range marks {
		if used[i] || strings.TrimSpace(marks[i].Text) == "" {
			continue
		}
		cy := (marks[i].BBox.Lly + marks[i].BBox.Ury) / 2
		if !pt.marginBand(cy) {
			continue
		}
		var idx []int
		for j := range marks {
			if strings.TrimSpace(marks[j].Text) == "" {
				continue
			}
			cyj := (marks[j].BBox.Lly + marks[j].BBox.Ury) / 2
			if mdAbs(cyj-cy) <= 6 {
				idx = append(idx, j)
				used[j] = true
			}
		}
		sort.Slice(idx, func(a, b int) bool { return marks[idx[a]].BBox.Llx < marks[idx[b]].BBox.Llx })
		var b strings.Builder
		for _, j := range idx {
			b.WriteString(strings.TrimSpace(marks[j].Text))
		}
		lines = append(lines, mdMarginLine{text: b.String(), indices: idx})
	}
	return lines
}

// mdPageNumberRegexp matches a page number, optionally in "N/total" form.
var mdPageNumberRegexp = regexp.MustCompile(`^(\d{1,4})(/\d{1,4})?$`)

// mdPageNumberValue returns the (leading) page-number value of a margin line if
// it is a page number such as "7" or "2/21", and false otherwise. "15.04.2023"
// or "PT/H/026" do not match.
func mdPageNumberValue(text string) (int, bool) {
	m := mdPageNumberRegexp.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	value := 0
	for _, r := range m[1] {
		value = value*10 + int(r-'0')
	}
	return value, true
}

// mdLabelRegexp masks digit runs so that a running label with a varying page
// number (e.g. "PT/H/0653/001/IA/026 19") normalizes to a stable form.
var mdLabelRegexp = regexp.MustCompile(`\d+`)

func mdNormalizeLabel(text string) string {
	return mdLabelRegexp.ReplaceAllString(text, "#")
}

// contentMarks returns the page text marks with page-number footers/headers and
// recurring running labels removed. stripPageNumbers is set when the document
// was found to number its pages; repeatedLabels holds normalized margin lines
// that recur on most pages.
func (pt PageText) contentMarks(stripPageNumbers bool, repeatedLabels map[string]bool) []TextMark {
	marks := pt.Marks().Elements()
	strip := make(map[int]bool)
	for _, line := range pt.marginLines() {
		_, isPageNumber := mdPageNumberValue(line.text)
		if (stripPageNumbers && isPageNumber) || repeatedLabels[mdNormalizeLabel(line.text)] {
			for _, j := range line.indices {
				strip[j] = true
			}
		}
	}
	if len(strip) == 0 {
		return marks
	}
	out := make([]TextMark, 0, len(marks))
	for i, m := range marks {
		if !strip[i] {
			out = append(out, m)
		}
	}
	return out
}

// blocks returns the page content as an ordered list of text and table blocks.
// Tables drawn with ruling lines are reconstructed; text that sits above, below
// or between them (including prose that ended up inside a grid) is emitted as
// text blocks in reading order.
func (pt PageText) blocks(stripPageNumbers bool, repeatedLabels map[string]bool) []mdBlock {
	marks := pt.contentMarks(stripPageNumbers, repeatedLabels)
	strokes := pt.cleanStrokes()
	tables, consumed := pt.lineTables(marks)
	if len(tables) == 0 {
		if txt := mdReconstructText(marks, strokes); txt != "" {
			return []mdBlock{{text: txt}}
		}
		return nil
	}

	var leftover []TextMark
	for i, m := range marks {
		if !consumed[i] {
			leftover = append(leftover, m)
		}
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].top() > tables[j].top() })

	var blocks []mdBlock
	used := make([]bool, len(leftover))
	collect := func(limit float64, hasLimit bool) {
		var ms []TextMark
		for i, m := range leftover {
			if used[i] {
				continue
			}
			if !hasLimit || (m.BBox.Lly+m.BBox.Ury)/2 > limit-2 {
				ms = append(ms, m)
				used[i] = true
			}
		}
		if txt := mdReconstructText(ms, strokes); txt != "" {
			blocks = append(blocks, mdBlock{text: txt})
		}
	}
	for _, table := range tables {
		collect(table.top(), true)
		blocks = append(blocks, mdBlock{table: table})
	}
	collect(0, false)
	return blocks
}

// Markdown returns the page content as Markdown text. Paragraphs are joined in
// reading order, underlined runs are wrapped in <u></u> and tables that are
// drawn with ruling lines are reconstructed as Markdown tables (cell line
// breaks are encoded as <br>).
func (pt PageText) Markdown() string {
	return mdRenderBlocks(pt.blocks(false, nil))
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
	stripPageNumbers := mdDetectPageNumbers(pages)
	repeatedLabels := mdDetectRepeatedLabels(pages)
	var blocks []mdBlock
	for _, pt := range pages {
		if pt == nil {
			continue
		}
		for _, blk := range pt.blocks(stripPageNumbers, repeatedLabels) {
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

// mdDetectPageNumbers reports whether the document numbers its pages in the
// margins. It looks for an offset k such that, on most pages, a digit-only
// margin line equal to (pageIndex+1+k) appears — i.e. a number that increments
// by one from page to page. Random integers that happen to sit in a margin do
// not form such a run and are left untouched.
func mdDetectPageNumbers(pages []*PageText) bool {
	pageValues := make([][]int, len(pages))
	for i, pt := range pages {
		if pt == nil {
			continue
		}
		for _, line := range pt.marginLines() {
			if v, ok := mdPageNumberValue(line.text); ok {
				pageValues[i] = append(pageValues[i], v)
			}
		}
	}
	bestCount := 0
	for k := -4; k <= 4; k++ {
		count := 0
		for i := range pages {
			expected := i + 1 + k
			for _, v := range pageValues[i] {
				if v == expected {
					count++
					break
				}
			}
		}
		if count > bestCount {
			bestCount = count
		}
	}
	// A run of margin numbers that increments page-to-page on at least a third
	// of the pages (and at least 3) confirms the document numbers its pages.
	return bestCount >= 3 && bestCount*3 >= len(pages)
}

// mdDetectRepeatedLabels returns the set of normalized margin lines (digit runs
// masked) that recur in the margins of most pages — i.e. running headers and
// footers such as "PT/H/0653/001/IA/026". Requiring a strong majority avoids
// pruning ordinary content. A pure page number is excluded here because it is
// handled separately and would otherwise always qualify.
func mdDetectRepeatedLabels(pages []*PageText) map[string]bool {
	nonNil := 0
	counts := make(map[string]int)
	for _, pt := range pages {
		if pt == nil {
			continue
		}
		nonNil++
		seen := make(map[string]bool)
		for _, line := range pt.marginLines() {
			if _, isPageNumber := mdPageNumberValue(line.text); isPageNumber {
				continue
			}
			norm := mdNormalizeLabel(line.text)
			if len(norm) < 4 || seen[norm] {
				continue
			}
			seen[norm] = true
			counts[norm]++
		}
	}
	repeated := make(map[string]bool)
	for norm, count := range counts {
		if count >= 3 && count*2 >= nonNil {
			repeated[norm] = true
		}
	}
	return repeated
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

func (t *mdLineTable) cols() int {
	if len(t.cells) > 0 {
		return len(t.cells[0])
	}
	return len(t.xs) - 1
}
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
func (pt PageText) lineTables(marks []TextMark) (tables []*mdLineTable, consumed map[int]bool) {
	var vxs []float64
	var vsegs []mdVSeg
	var hsegs []mdHSeg
	for _, s := range pt.cleanStrokes() {
		if s.IsVertical() {
			x := (s.X1 + s.X2) / 2
			lo, hi := mdMinMax(s.Y1, s.Y2)
			length := hi - lo
			// Long verticals are column-border candidates. Short ones (e.g. a
			// single-line header row separator) are kept only as vsegs so they can
			// still establish the table extent and per-row column structure.
			if length > 20 {
				vxs = append(vxs, x)
			}
			if length > 5 {
				vsegs = append(vsegs, mdVSeg{x, lo, hi})
			}
		}
		if s.IsHorizontal() {
			lo, hi := mdMinMax(s.X1, s.X2)
			hsegs = append(hsegs, mdHSeg{(s.Y1 + s.Y2) / 2, lo, hi})
		}
	}
	if len(vxs) == 0 {
		return nil, nil
	}
	xs := mdCluster(vxs, 8)
	if len(xs) < 3 {
		return nil, nil
	}
	tableWidth := xs[len(xs)-1] - xs[0]

	var hys []float64
	for _, h := range hsegs {
		hys = append(hys, h.y)
	}
	// The table vertical extent is determined only by verticals that sit on a
	// detected column border. This lets short header-row separators extend the
	// extent (so the header row is kept) while ignoring stray rules and mid-cell
	// ticks that are not aligned with any column.
	vmin, vmax := 0.0, 0.0
	haveExtent := false
	for _, v := range vsegs {
		if !mdNearAny(xs, v.x, 6) {
			continue
		}
		if !haveExtent {
			vmin, vmax, haveExtent = v.ylo, v.yhi, true
			continue
		}
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
		return nil, nil
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(ys)))

	cols := len(xs) - 1
	rows := len(ys) - 1

	// For each row band determine which column borders actually exist (a vertical
	// segment covers most of the band). Consecutive present borders define a cell
	// that may span several columns (colspan). verticalCount records how many
	// column verticals cross the band before any fallback.
	rowBorders := make([][]float64, rows)
	verticalCount := make([]int, rows)
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
		verticalCount[r] = len(present)
		if len(present) < 2 {
			present = []float64{xs[0], xs[len(xs)-1]}
		}
		rowBorders[r] = present
	}

	cellMarks := make(map[[2]int][]mdCellMark)
	markRow := make([]int, len(marks))
	for i := range markRow {
		markRow[i] = -1
	}
	for i, m := range marks {
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
		markRow[i] = r
		cellMarks[[2]int{r, gc}] = append(cellMarks[[2]int{r, gc}],
			mdCellMark{m.BBox.Llx, m.BBox.Urx, cy, m.Text})
	}
	fullCells := make([][]string, rows)
	for r := range fullCells {
		fullCells[r] = make([]string, cols)
	}
	for key, cms := range cellMarks {
		fullCells[key[0]][key[1]] = mdJoinCell(cms)
	}

	// A row band with fewer than two crossing column verticals has no tabular
	// structure: it is absorbed prose (a footnote, caption or paragraph that sits
	// between table grids). Split the grid into contiguous tabular segments at
	// such rows and leave their marks for text reconstruction.
	consumed = make(map[int]bool)
	r := 0
	for r < rows {
		if verticalCount[r] < 2 {
			r++
			continue
		}
		start := r
		for r < rows && verticalCount[r] >= 2 {
			r++
		}
		segCells := make([][]string, r-start)
		for k := range segCells {
			segCells[k] = fullCells[start+k]
		}
		seg := &mdLineTable{xs: xs, ys: ys[start : r+1], cells: segCells}
		seg.dropEmptyRows()
		seg.dropEmptyColumns()
		if len(seg.cells) == 0 {
			continue
		}
		tables = append(tables, seg)
		for i, mr := range markRow {
			if mr >= start && mr < r {
				consumed[i] = true
			}
		}
	}
	return tables, consumed
}

// dropEmptyRows removes rows whose cells are all empty. Such rows come from
// spacer bands in the ruling grid and carry no content.
func (t *mdLineTable) dropEmptyRows() {
	kept := t.cells[:0]
	for _, row := range t.cells {
		empty := true
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				empty = false
				break
			}
		}
		if !empty {
			kept = append(kept, row)
		}
	}
	t.cells = kept
}

// dropEmptyColumns removes columns that are empty in every row. These come from
// spurious vertical rules to the side of the table.
func (t *mdLineTable) dropEmptyColumns() {
	if len(t.cells) == 0 {
		return
	}
	cols := len(t.cells[0])
	keep := make([]bool, cols)
	kept := 0
	for c := 0; c < cols; c++ {
		for _, row := range t.cells {
			if strings.TrimSpace(row[c]) != "" {
				keep[c] = true
				kept++
				break
			}
		}
	}
	if kept == cols {
		return
	}
	for r, row := range t.cells {
		trimmed := make([]string, 0, kept)
		for c, cell := range row {
			if keep[c] {
				trimmed = append(trimmed, cell)
			}
		}
		t.cells[r] = trimmed
	}
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
	if mdIsStacked(lines) {
		return strings.Join(lines, "")
	}
	return strings.Join(lines, "<br>")
}

// mdIsStacked reports whether a cell's lines are vertical/rotated text — many
// one-character lines, as produced when a rotated chart axis label is parsed as
// a cell. Such cells are joined into a single run instead of exploding into one
// <br> per character.
func mdIsStacked(lines []string) bool {
	if len(lines) < 6 {
		return false
	}
	single := 0
	for _, line := range lines {
		if len([]rune(line)) <= 1 {
			single++
		}
	}
	return single*2 >= len(lines)
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
		content, bullet := mdBulletContent(line)
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
			b.WriteString("- " + content)
		} else {
			b.WriteString(line)
		}
	}
	return b.String()
}

// mdBulletContent reports whether a physical line begins with a bullet marker
// (•, a dash, or an asterisk followed by a space) and returns the text after
// the marker. A trailing-position dash (hyphenated word wrap) is not a bullet
// because the marker must be at the very start of the line.
func mdBulletContent(line string) (string, bool) {
	if strings.HasPrefix(line, "•") {
		return strings.TrimSpace(line[len("•"):]), true
	}
	for _, marker := range []string{"- ", "– ", "− ", "* "} {
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(line[len(marker):]), true
		}
	}
	return line, false
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

func mdNearAny(xs []float64, x, tol float64) bool {
	for _, v := range xs {
		if mdAbs(v-x) <= tol {
			return true
		}
	}
	return false
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
