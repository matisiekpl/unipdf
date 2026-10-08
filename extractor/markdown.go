/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package extractor

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/matisiekpl/unipdf/v3/model"
)

// mdBlock is a single piece of page content: either a paragraph of text or a
// table reconstructed from ruling lines.
type mdBlock struct {
	text       string
	table      *mdLineTable
	tail, lead float64
}

func (blk mdBlock) continuedBy(next mdBlock) bool {
	if blk.table != nil || next.table != nil || blk.text == "" || next.text == "" {
		return false
	}
	text := strings.TrimSuffix(blk.text, "</u>")
	last := text[strings.LastIndex(text, "\n")+1:]
	first := strings.SplitN(next.text, "\n", 2)[0]
	_, bullet := mdBulletContent(first)
	heading := strings.ToUpper(last) == last || (strings.HasPrefix(first, "<u>") && strings.HasSuffix(first, "</u>"))
	return blk.tail <= next.lead+lineEndSlack && !strings.ContainsAny(text[len(text)-1:], ".:;!?") && !bullet && !heading &&
		!mdHeadingRegexp.MatchString(first) && !mdHeadingRegexp.MatchString(last) && !mdFootnoteLeadRegexp.MatchString(first)
}

var mdFootnoteLeadRegexp = regexp.MustCompile(`^(\*{1,3}|[†‡§¶]|\p{No}{1,2}|[ᵃ-ᶻ])\s`)

// cleanStrokes returns the page strokes with exact duplicates removed and with
// page-spanning strokes (clipping rectangles and page borders/backgrounds that
// some PDFs draw repeatedly) dropped, so they don't pollute table detection.
func (pt PageText) cleanStrokes() []Stroke {
	pageWidth := pt.pageSize.Urx - pt.pageSize.Llx
	pageHeight := pt.pageSize.Ury - pt.pageSize.Lly
	seen := make(map[[4]int]bool)
	var out []Stroke
	for _, s := range append(pt.tiledCellStrokes(), pt.strokes...) {
		if s.IsVertical() && mdAbs(s.Y2-s.Y1) > pageHeight*pageFrameShare {
			continue
		}
		if s.IsHorizontal() && mdAbs(s.X2-s.X1) > pageWidth*pageFrameShare {
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

const pageFrameShare = 0.97

const cellTileTolerance = 1.0

const cellGapTolerance = 3.0

func (pt PageText) tiledCellStrokes() []Stroke {
	pageWidth := pt.pageSize.Urx - pt.pageSize.Llx
	seen := make(map[[4]int]bool)
	var candidates []Rect
	for _, rect := range append(pt.unruledWhiteCellRects(), pt.cellRects...) {
		if rect.Urx-rect.Llx > pageWidth*0.8 || rect.isThin() {
			continue
		}
		key := [4]int{int(rect.Llx * 2), int(rect.Lly * 2), int(rect.Urx * 2), int(rect.Ury * 2)}
		if seen[key] {
			continue
		}
		seen[key] = true
		candidates = append(candidates, rect)
	}
	var strokes []Stroke
	for index, rect := range candidates {
		if !rect.hasRowNeighbour(candidates, index) || !rect.hasColumnNeighbour(candidates, index) {
			continue
		}
		strokes = append(strokes,
			Stroke{X1: rect.Llx, Y1: rect.Lly, X2: rect.Urx, Y2: rect.Lly},
			Stroke{X1: rect.Llx, Y1: rect.Ury, X2: rect.Urx, Y2: rect.Ury},
			Stroke{X1: rect.Llx, Y1: rect.Lly, X2: rect.Llx, Y2: rect.Ury},
			Stroke{X1: rect.Urx, Y1: rect.Lly, X2: rect.Urx, Y2: rect.Ury})
	}
	return strokes
}

const minimumColumnRuleLength = 20.0

func (pt PageText) unruledWhiteCellRects() []Rect {
	if len(pt.whiteCellRects) == 0 {
		return nil
	}
	pageHeight := pt.pageSize.Ury - pt.pageSize.Lly
	var rules []Stroke
	for _, stroke := range mdMergeCollinearVerticals(pt.strokes) {
		length := mdAbs(stroke.Y2 - stroke.Y1)
		if stroke.IsVertical() && length > minimumColumnRuleLength && length <= pageHeight*pageFrameShare {
			rules = append(rules, stroke)
		}
	}
	var rects []Rect
	for _, rect := range pt.whiteCellRects {
		ruled := false
		for _, rule := range rules {
			low, high := mdMinMax(rule.Y1, rule.Y2)
			if mdMin(high, rect.Ury)-mdMax(low, rect.Lly) > 0 {
				ruled = true
				break
			}
		}
		if !ruled {
			rects = append(rects, rect)
		}
	}
	return rects
}

func (r Rect) hasRowNeighbour(rects []Rect, index int) bool {
	for other, candidate := range rects {
		if other == index {
			continue
		}
		overlap := mdMin(r.Ury, candidate.Ury) - mdMax(r.Lly, candidate.Lly)
		sameRow := overlap > 0.5*mdMin(r.Ury-r.Lly, candidate.Ury-candidate.Lly)
		if sameRow && (r.isCellGap(candidate.Llx-r.Urx) || r.isCellGap(r.Llx-candidate.Urx)) {
			return true
		}
	}
	return false
}

func (r Rect) hasColumnNeighbour(rects []Rect, index int) bool {
	for other, candidate := range rects {
		if other == index || mdAbs(r.Lly-candidate.Lly) <= cellTileTolerance {
			continue
		}
		if mdAbs(r.Llx-candidate.Llx) <= cellTileTolerance && mdAbs(r.Urx-candidate.Urx) <= cellTileTolerance {
			return true
		}
	}
	return false
}

func (r Rect) isCellGap(gap float64) bool {
	return gap >= -cellTileTolerance && gap <= cellGapTolerance
}

func (pt PageText) ruledColumnBorders(xs []float64, vmin, vmax float64) []bool {
	pageHeight := pt.pageSize.Ury - pt.pageSize.Lly
	ruled := make([]bool, len(xs))
	for _, stroke := range mdMergeCollinearVerticals(pt.strokes) {
		length := mdAbs(stroke.Y2 - stroke.Y1)
		if !stroke.IsVertical() || length <= minimumColumnRuleLength || length > pageHeight*pageFrameShare {
			continue
		}
		low, high := mdMinMax(stroke.Y1, stroke.Y2)
		if mdMin(high, vmax)-mdMax(low, vmin) <= 0 {
			continue
		}
		for index, x := range xs {
			if mdAbs(x-stroke.X1) <= 4 {
				ruled[index] = true
			}
		}
	}
	return ruled
}

const wordGap = 1.5

func mdWords(marks []TextMark, rules []float64) (words []mdWord, markWord []int) {
	markWord = make([]int, len(marks))
	var glyphs []int
	for index, mark := range marks {
		markWord[index] = -1
		if strings.TrimSpace(mark.Text) != "" {
			glyphs = append(glyphs, index)
		}
	}
	centerY := func(index int) float64 {
		return (marks[index].BBox.Lly + marks[index].BBox.Ury) / 2
	}
	sort.Slice(glyphs, func(first, second int) bool {
		return centerY(glyphs[first]) < centerY(glyphs[second])
	})
	for start := 0; start < len(glyphs); {
		end := start + 1
		for end < len(glyphs) && centerY(glyphs[end])-centerY(glyphs[end-1]) <= mdCellLineTolerance {
			end++
		}
		line := glyphs[start:end]
		sort.Slice(line, func(first, second int) bool {
			return marks[line[first]].BBox.Llx < marks[line[second]].BBox.Llx
		})
		for position, index := range line {
			box := marks[index].BBox
			last := len(words) - 1
			if position > 0 && box.Llx-words[last].x1 <= wordGap && !mdRuleInGap(rules, words[last].x1, box.Llx) {
				words[last].x1 = mdMax(words[last].x1, box.Urx)
				words[last].baseline = mdMin(words[last].baseline, box.Lly)
			} else {
				words = append(words, mdWord{x0: box.Llx, x1: box.Urx, baseline: box.Lly, y: centerY(index)})
			}
			markWord[index] = len(words) - 1
		}
		start = end
	}
	return words, markWord
}

func mdRuleInGap(rules []float64, left, right float64) bool {
	for _, rule := range rules {
		if rule >= left-0.5 && rule <= right+0.5 {
			return true
		}
	}
	return false
}

const underlineOverhang = 2.0

const lineClipOffset = 8.0

func mdUnderline(segment mdHSeg, words []mdWord, xs []float64) bool {
	if mdNearAny(xs, segment.x0, 1.5) || mdNearAny(xs, segment.x1, 1.5) {
		return false
	}
	found := false
	low, high := 0.0, 0.0
	for _, word := range words {
		if segment.y > word.baseline+1 || segment.y < word.baseline-5 || word.x1 < segment.x0 || word.x0 > segment.x1 {
			continue
		}
		if !found {
			low, high, found = word.x0, word.x1, true
			continue
		}
		low, high = mdMin(low, word.x0), mdMax(high, word.x1)
	}
	return found && segment.x0 >= low-underlineOverhang && segment.x1 <= high+underlineOverhang
}

func mdWordStraddles(words []mdWord, top, bottom, x float64) bool {
	for _, word := range words {
		if word.y >= bottom && word.y <= top && word.x0+0.5 < x && word.x1-0.5 > x {
			return true
		}
	}
	return false
}

func mdLineStraddles(words []mdWord, y float64, borders []float64) bool {
	var line []mdWord
	for _, word := range words {
		if mdAbs(word.y-y) > mdCellLineTolerance {
			continue
		}
		for _, x := range borders {
			if word.x0+0.5 < x && word.x1-0.5 > x {
				return true
			}
		}
		line = append(line, word)
	}
	sort.Slice(line, func(first, second int) bool { return line[first].x0 < line[second].x0 })
	for index := 1; index < len(line); index++ {
		previous, word := line[index-1], line[index]
		if word.x0-previous.x1 < borderlessGutter && sort.SearchFloat64s(borders, (previous.x0+previous.x1)/2) != sort.SearchFloat64s(borders, (word.x0+word.x1)/2) {
			return true
		}
	}
	return false
}

func mdSingleTextLine(words []mdWord, top, bottom float64) bool {
	first := -1.0
	for _, word := range words {
		if word.y < bottom || word.y > top {
			continue
		}
		if first < 0 {
			first = word.y
		} else if mdAbs(word.y-first) > mdCellLineTolerance {
			return false
		}
	}
	return first >= 0
}

func mdTabularLineInRow(words []mdWord, top, bottom float64, borders []float64) bool {
	if len(borders) == 0 {
		return false
	}
	for _, word := range words {
		if word.y < bottom || word.y > top || mdLineStraddles(words, word.y, borders) {
			continue
		}
		sides := map[int]bool{}
		for _, other := range words {
			if mdAbs(other.y-word.y) <= mdCellLineTolerance {
				sides[sort.SearchFloat64s(borders, (other.x0+other.x1)/2)] = true
			}
		}
		if len(sides) == len(borders)+1 {
			return true
		}
	}
	return false
}

const openRowMinimumHeight = 4.0

const implicitBorderMargin = 10.0

func mdImplicitOuterBorders(hsegs []mdHSeg, xs []float64, vmin, vmax float64) []float64 {
	left, right := xs[0], xs[len(xs)-1]
	var leftEnds, rightEnds []float64
	for _, h := range mdMergeCollinearHorizontals(hsegs) {
		if h.y < vmin-3 || h.y > vmax+3 || mdMin(h.x1, right)-mdMax(h.x0, left) < (right-left)*0.5 {
			continue
		}
		if h.x0 < left-implicitBorderMargin {
			leftEnds = append(leftEnds, h.x0)
		}
		if h.x1 > right+implicitBorderMargin {
			rightEnds = append(rightEnds, h.x1)
		}
	}
	var borders []float64
	if len(leftEnds) >= 2 {
		sort.Float64s(leftEnds)
		borders = append(borders, leftEnds[len(leftEnds)-1])
	}
	if len(rightEnds) >= 2 {
		sort.Float64s(rightEnds)
		borders = append(borders, rightEnds[0])
	}
	return borders
}

func mdMergeCollinearHorizontals(hsegs []mdHSeg) []mdHSeg {
	sorted := append([]mdHSeg(nil), hsegs...)
	sort.Slice(sorted, func(first, second int) bool {
		return sorted[first].y < sorted[second].y
	})
	var merged []mdHSeg
	for start := 0; start < len(sorted); {
		end := start + 1
		for end < len(sorted) && sorted[end].y-sorted[end-1].y <= collinearTolerance {
			end++
		}
		line := sorted[start:end]
		sort.Slice(line, func(first, second int) bool {
			return line[first].x0 < line[second].x0
		})
		lineStart := len(merged)
		for _, segment := range line {
			last := len(merged) - 1
			if last >= lineStart && segment.x0 <= merged[last].x1+collinearGap {
				merged[last].x1 = mdMax(merged[last].x1, segment.x1)
				continue
			}
			merged = append(merged, segment)
		}
		start = end
	}
	return merged
}

const collinearTolerance = 0.5

const collinearGap = 2.0

func mdMergeCollinearVerticals(strokes []Stroke) []Stroke {
	var horizontals []Stroke
	var transposedVerticals []mdHSeg
	for _, stroke := range strokes {
		if stroke.IsHorizontal() {
			horizontals = append(horizontals, stroke)
		}
		if stroke.IsVertical() {
			low, high := mdMinMax(stroke.Y1, stroke.Y2)
			transposedVerticals = append(transposedVerticals, mdHSeg{y: (stroke.X1 + stroke.X2) / 2, x0: low, x1: high})
		}
	}
	for _, vertical := range mdMergeCollinearHorizontals(transposedVerticals) {
		horizontals = append(horizontals, Stroke{X1: vertical.y, Y1: vertical.x0, X2: vertical.y, Y2: vertical.x1})
	}
	return horizontals
}

// mdMarginBandRatio is the share of the page height at the top and at the bottom
// that counts as margin. A footer typically sits between 8% and 10% of the page
// height above the edge, so a tighter band leaves the page number in the body.
const mdMarginBandRatio = 0.11

// mdMarginSegmentGapRatio is the share of the page width that separates a footer's
// independent parts. It is far wider than any word space, so splitting on it never
// breaks a running label into pieces.
const mdMarginSegmentGapRatio = 0.08

// marginBand reports whether device-y `cy` is inside the top or bottom margin
// band of the page, where running headers and page numbers live.
func (pt PageText) marginBand(cy float64) bool {
	pageHeight := pt.pageSize.Ury - pt.pageSize.Lly
	return cy < pt.pageSize.Lly+pageHeight*mdMarginBandRatio || cy > pt.pageSize.Ury-pageHeight*mdMarginBandRatio
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
		lines = append(lines, mdMarginLine{text: mdMarginText(marks, idx), indices: idx})
		lines = append(lines, pt.marginSegments(marks, idx)...)
	}
	return lines
}

func mdMarginText(marks []TextMark, idx []int) string {
	var b strings.Builder
	for _, j := range idx {
		b.WriteString(strings.TrimSpace(marks[j].Text))
	}
	return b.String()
}

// marginSegments splits a margin line at the column gaps that separate a footer's
// independent parts, so that a page number sharing its line with a running label
// ("NL/H/1575/002/IB/046      7") is still offered as a page number of its own.
// A single-segment line adds nothing the caller does not already have.
func (pt PageText) marginSegments(marks []TextMark, idx []int) []mdMarginLine {
	gap := (pt.pageSize.Urx - pt.pageSize.Llx) * mdMarginSegmentGapRatio
	var segments [][]int
	current := []int{idx[0]}
	for _, j := range idx[1:] {
		if marks[j].BBox.Llx-marks[current[len(current)-1]].BBox.Urx >= gap {
			segments = append(segments, current)
			current = nil
		}
		current = append(current, j)
	}
	segments = append(segments, current)
	if len(segments) < 2 {
		return nil
	}
	lines := make([]mdMarginLine, 0, len(segments))
	for _, segment := range segments {
		lines = append(lines, mdMarginLine{text: mdMarginText(marks, segment), indices: segment})
	}
	return lines
}

// mdPageNumberRegexp matches a page number, optionally in "N/total" form and
// optionally wrapped in dashes (e.g. "7", "2/21" or "- 2 -").
var mdPageNumberRegexp = regexp.MustCompile(`^[-–—\s]*(\d{1,4})(/\d{1,4})?[-–—\s]*$`)

// mdPageNumberValue returns the (leading) page-number value of a margin line if
// it is a page number such as "7", "2/21" or "- 2 -", and false otherwise.
// "15.04.2023" or "PT/H/026" do not match.
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

var mdLatinSupplementRegexp = regexp.MustCompile(`^[À-ÿ]$`)

var mdLatinLetterRegexp = regexp.MustCompile(`^[A-Za-z]$`)

var mdGreekLetterRegexp = regexp.MustCompile(`^\p{Greek}$`)

var mdGlyphReplacer = strings.NewReplacer("ÿý", "", "˚", "°", "º", "°", "ꞏ", "·")

// contentMarks returns the page text marks with page-number footers/headers and
// recurring running labels removed. stripPageNumbers is set when the document
// was found to number its pages; repeatedLabels holds normalized margin lines
// that recur on most pages.
func (pt PageText) contentMarks(pageNumber int, repeatedLabels map[string]bool) []TextMark {
	marks := append([]TextMark(nil), pt.Marks().Elements()...)
	for index := range marks {
		marks[index].Text = mdGlyphReplacer.Replace(marks[index].Text)
		if marks[index].Font != nil && strings.Contains(marks[index].Font.BaseFont(), "Wingdings") && strings.IndexFunc(marks[index].Text, unicode.IsLetter) >= 0 {
			marks[index].Text = "•"
		}
		if marks[index].Font != nil && strings.Contains(marks[index].Font.BaseFont(), "Symbol") && mdLatinSupplementRegexp.MatchString(marks[index].Text) && marks[index].Original != "" && !mdLatinSupplementRegexp.MatchString(marks[index].Original) {
			marks[index].Text = marks[index].Original
		}
		if marks[index].Font != nil && strings.Contains(marks[index].Font.BaseFont(), "Symbol") && mdLatinLetterRegexp.MatchString(marks[index].Text) && mdGreekLetterRegexp.MatchString(marks[index].Original) {
			marks[index].Text = marks[index].Original
		}
	}
	mdCollapseRotatedRuns(marks)
	mdMarkWordStarts(marks)
	mdInlineScripts(marks)
	mdUnderlinedComparisons(marks, pt.strokes)
	strip := make(map[int]bool)
	var strokes []Stroke
	for _, line := range pt.marginLines() {
		value, numbered := mdPageNumberValue(line.text)
		isPageNumber := numbered && pageNumber > 0 && value == pageNumber
		repeated := repeatedLabels[mdNormalizeLabel(line.text)] || repeatedLabels[line.text]
		if !isPageNumber && !repeated {
			continue
		}
		if strokes == nil {
			strokes = mdMergeCollinearVerticals(pt.cleanStrokes())
		}
		if !repeated && mdInsideRuling(marks, line.indices, strokes, (pt.pageSize.Lly+pt.pageSize.Ury)/2) {
			continue
		}
		for _, j := range line.indices {
			strip[j] = true
		}
	}
	out := make([]TextMark, 0, len(marks))
	for i, m := range marks {
		if !strip[i] {
			out = append(out, m)
		}
	}
	return mdRestoreHyphens(out, pt.lineEndHyphens)
}

const (
	lineEndHyphen    = "\u00ad"
	hyphenAttachment = 3.0
)

func mdRestoreHyphens(marks []TextMark, hyphens []TextMark) []TextMark {
	after := make(map[int][]TextMark)
	for _, hyphen := range hyphens {
		middle := (hyphen.BBox.Lly + hyphen.BBox.Ury) / 2
		height := hyphen.BBox.Ury - hyphen.BBox.Lly
		best := -1
		for index, mark := range marks {
			gap := hyphen.BBox.Llx - mark.BBox.Urx
			sameLine := mdAbs((mark.BBox.Lly+mark.BBox.Ury)/2-middle) < 0.5*height
			if strings.TrimSpace(mark.Text) == "" || !sameLine || gap < -1 || gap > hyphenAttachment {
				continue
			}
			if best < 0 || mark.BBox.Urx > marks[best].BBox.Urx {
				best = index
			}
		}
		if best >= 0 {
			hyphen.Text = lineEndHyphen
			after[best] = append(after[best], hyphen)
		}
	}
	restored := make([]TextMark, 0, len(marks)+len(hyphens))
	for index, mark := range marks {
		restored = append(restored, mark)
		restored = append(restored, after[index]...)
	}
	return restored
}

const rulingIntoBody = 30.0

func mdInsideRuling(marks []TextMark, indices []int, strokes []Stroke, middle float64) bool {
	if len(indices) == 0 {
		return false
	}
	region := marks[indices[0]].BBox
	for _, index := range indices[1:] {
		box := marks[index].BBox
		region = model.PdfRectangle{Llx: mdMin(region.Llx, box.Llx), Lly: mdMin(region.Lly, box.Lly), Urx: mdMax(region.Urx, box.Urx), Ury: mdMax(region.Ury, box.Ury)}
	}
	y := (region.Lly + region.Ury) / 2
	left, right := false, false
	for _, stroke := range strokes {
		low, high := mdMinMax(stroke.Y1, stroke.Y2)
		reachesBody := (y > middle && low < y-rulingIntoBody) || (y <= middle && high > y+rulingIntoBody)
		if !stroke.IsVertical() || y < low || y > high || !reachesBody {
			continue
		}
		left = left || stroke.X1 <= region.Llx
		right = right || stroke.X1 >= region.Urx
	}
	return left && right
}

const rotatedRunGap = 1.5

func mdCollapseRotatedRuns(marks []TextMark) {
	for start := 0; start < len(marks); {
		orient := marks[start].orient
		if orient == 0 || strings.TrimSpace(marks[start].Text) == "" {
			start++
			continue
		}
		var builder strings.Builder
		region := marks[start].BBox
		last := start
		for index := start; index < len(marks); index++ {
			mark := marks[index]
			if strings.TrimSpace(mark.Text) == "" {
				if mark.Meta || mark.orient == orient {
					continue
				}
				break
			}
			distance := mdMax(mdMax(mark.BBox.Llx-region.Urx, region.Llx-mark.BBox.Urx), mdMax(mark.BBox.Lly-region.Ury, region.Lly-mark.BBox.Ury))
			if mark.orient != orient || distance > rotatedRunGap*mark.FontSize {
				break
			}
			if index > last+1 || (index > start && marks[index-1].Meta) {
				builder.WriteString(" ")
			}
			builder.WriteString(mark.Text)
			region = model.PdfRectangle{Llx: mdMin(region.Llx, mark.BBox.Llx), Lly: mdMin(region.Lly, mark.BBox.Lly), Urx: mdMax(region.Urx, mark.BBox.Urx), Ury: mdMax(region.Ury, mark.BBox.Ury)}
			last = index
		}
		for index := start + 1; index <= last; index++ {
			marks[index].Text = ""
		}
		marks[start].Text = builder.String()
		marks[start].BBox = region
		start = last + 1
	}
}

func mdMarkWordStarts(marks []TextMark) {
	previous := -1
	spaced := false
	for index, mark := range marks {
		if strings.TrimSpace(mark.Text) == "" {
			spaced = spaced || mark.Text == " "
			continue
		}
		if spaced && previous >= 0 {
			before := marks[previous].BBox
			height := before.Ury - before.Lly
			sameLine := mdAbs((mark.BBox.Lly+mark.BBox.Ury)/2-(before.Lly+before.Ury)/2) < 0.5*height
			if sameLine && mark.BBox.Llx >= before.Urx-0.5 {
				marks[index].Text = " " + mark.Text
			}
		}
		previous = index
		spaced = false
	}
}

const scriptSizeRatio = 0.85

const scriptNeighbourhood = 4

const scriptColumnGap = 3.0

var superscripts = map[rune]rune{
	'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴', '5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	'+': '⁺', '-': '⁻', '=': '⁼', '(': '⁽', ')': '⁾',
	'a': 'ᵃ', 'b': 'ᵇ', 'c': 'ᶜ', 'd': 'ᵈ', 'e': 'ᵉ', 'f': 'ᶠ', 'g': 'ᵍ', 'h': 'ʰ', 'i': 'ⁱ', 'j': 'ʲ',
	'k': 'ᵏ', 'l': 'ˡ', 'm': 'ᵐ', 'n': 'ⁿ', 'o': 'ᵒ', 'p': 'ᵖ', 'r': 'ʳ', 's': 'ˢ', 't': 'ᵗ', 'u': 'ᵘ',
	'v': 'ᵛ', 'w': 'ʷ', 'x': 'ˣ', 'y': 'ʸ', 'z': 'ᶻ',
	'A': 'ᴬ', 'B': 'ᴮ', 'D': 'ᴰ', 'E': 'ᴱ', 'G': 'ᴳ', 'H': 'ᴴ', 'I': 'ᴵ', 'J': 'ᴶ', 'K': 'ᴷ', 'L': 'ᴸ',
	'M': 'ᴹ', 'N': 'ᴺ', 'O': 'ᴼ', 'P': 'ᴾ', 'R': 'ᴿ', 'T': 'ᵀ', 'U': 'ᵁ', 'V': 'ⱽ', 'W': 'ᵂ',
}

var subscripts = map[rune]rune{
	'0': '₀', '1': '₁', '2': '₂', '3': '₃', '4': '₄', '5': '₅', '6': '₆', '7': '₇', '8': '₈', '9': '₉',
	'+': '₊', '-': '₋', '=': '₌', '(': '₍', ')': '₎',
	'a': 'ₐ', 'e': 'ₑ', 'h': 'ₕ', 'i': 'ᵢ', 'j': 'ⱼ', 'k': 'ₖ', 'l': 'ₗ', 'm': 'ₘ', 'n': 'ₙ', 'o': 'ₒ',
	'p': 'ₚ', 'r': 'ᵣ', 's': 'ₛ', 't': 'ₜ', 'u': 'ᵤ', 'v': 'ᵥ', 'x': 'ₓ',
}

func mdInlineScripts(marks []TextMark) {
	scripts := make([]map[rune]rune, len(marks))
	bases := make([]model.PdfRectangle, len(marks))
	previous := -1
	for index, mark := range marks {
		if strings.TrimSpace(mark.Text) == "" {
			continue
		}
		opening := previous < 0 || marks[previous].BBox.Urx > mark.BBox.Llx+1 || mark.BBox.Llx-marks[previous].BBox.Urx > scriptColumnGap*marks[previous].FontSize
		preceding := previous
		previous = index
		for offset := -scriptNeighbourhood; offset <= scriptNeighbourhood && scripts[index] == nil; offset++ {
			neighbour := index + offset
			if offset == 0 || neighbour < 0 || neighbour >= len(marks) || strings.TrimSpace(marks[neighbour].Text) == "" {
				continue
			}
			if scripts[neighbour] == nil {
				scripts[index] = mdScriptOf(mark, marks[neighbour], opening, neighbour == preceding)
				bases[index] = marks[neighbour].BBox
			} else if offset < 0 && (mdAdjacent(mark.BBox, marks[neighbour].BBox) || mdScriptListContinues(marks[neighbour], mark)) &&
				mark.FontSize == marks[neighbour].FontSize && mdAbs(mark.BBox.Lly-marks[neighbour].BBox.Lly) < 1 {
				scripts[index] = scripts[neighbour]
				bases[index] = bases[neighbour]
			}
		}
		if scripts[index] == nil && (index == 0 || marks[index-1].Text == "\n") {
			for candidate, base := range marks {
				if candidate == index || scripts[candidate] != nil || strings.TrimSpace(base.Text) == "" || mark.FontSize > base.FontSize*scriptSizeRatio {
					continue
				}
				if script := mdScriptOf(mark, base, opening, false); script != nil {
					scripts[index], bases[index] = script, base.BBox
					break
				}
			}
		}
	}
	for index := len(marks) - 2; index >= 0; index-- {
		following := index + 1
		for following < len(marks)-1 && marks[following].Text == " " {
			following++
		}
		next := marks[following]
		if scripts[index] == nil && scripts[following] != nil && strings.TrimSpace(marks[index].Text) != "" &&
			((following == index+1 && mdAdjacent(marks[index].BBox, next.BBox)) || mdScriptListContinues(marks[index], next)) &&
			marks[index].FontSize == next.FontSize && mdAbs(marks[index].BBox.Lly-next.BBox.Lly) < 1 && bases[following].Llx >= next.BBox.Urx-1 {
			scripts[index] = scripts[following]
			bases[index] = bases[following]
		}
	}
	for start := 0; start < len(marks); {
		end := start + 1
		for scripts[start] != nil && end < len(marks) {
			next := end
			for next < len(marks)-1 && marks[next].Text == " " {
				next++
			}
			if scripts[next] == nil || bases[next] != bases[start] {
				break
			}
			end = next + 1
		}
		if scripts[start] != nil {
			gap := marks[start].BBox.Llx - bases[start].Urx
			var text strings.Builder
			for _, mark := range marks[start:end] {
				text.WriteString(mark.Text)
			}
			if gap <= wordGap {
				mdConvertScriptRun(marks[start:end], scripts[start], gap >= -1)
			} else if mdFootnoteListRegexp.MatchString(strings.TrimSpace(text.String())) {
				original := marks[start].Text
				mdConvertScriptRun(marks[start:end], scripts[start], true)
				last := marks[end-1]
				leadsNext := false
				if end < len(marks) && marks[end].BBox.Llx-last.BBox.Urx <= wordGap {
					nextCharacter := []rune(marks[end].Text + " ")[0]
					leadsNext = strings.ContainsRune("/⁄", nextCharacter) || unicode.IsLetter(nextCharacter) && last.FontSize <= marks[end].FontSize*scriptSizeRatio
				}
				if !leadsNext {
					marks[start].BBox.Llx = bases[start].Urx
				} else if strings.HasPrefix(original, " ") && !strings.HasPrefix(marks[start].Text, " ") {
					marks[start].Text = " " + marks[start].Text
				}
			}
		}
		start = end
	}
	for index, script := range scripts {
		if script == nil {
			continue
		}
		marks[index].BBox.Lly, marks[index].BBox.Ury = bases[index].Lly, bases[index].Ury
		for following := range marks {
			gap := marks[following].BBox.Llx - marks[index].BBox.Urx
			limit := wordGap
			if scripts[mdPrecedingGlyph(marks, following)] != nil {
				limit = scriptAttachGap * marks[following].FontSize
			}
			if scripts[following] == nil && strings.HasPrefix(marks[following].Text, " ") && gap >= -1 && gap <= limit && mdAbs(marks[following].BBox.Lly-bases[index].Lly) < 1 {
				marks[following].Text = strings.TrimPrefix(marks[following].Text, " ")
			}
		}
	}
}

const maximumScriptLength = 6

const (
	scriptPassthrough   = ",*†‡/"
	partialScriptLength = 4
)

func mdConvertScriptRun(run []TextMark, script map[rune]rune, attached bool) {
	first := run[0].Text
	if attached {
		first = strings.TrimPrefix(first, " ")
	}
	characters := []rune(first)
	for _, mark := range run[1:] {
		characters = append(characters, []rune(mark.Text)...)
	}
	mapped, unmapped, digits := 0, 0, 0
	previous := rune(0)
	for _, character := range characters {
		listSpace := character == ' ' && previous == ','
		if character != ' ' {
			previous = character
		}
		if _, ok := script[character]; ok {
			mapped++
		} else if !strings.ContainsRune(scriptPassthrough, character) && !listSpace {
			if !unicode.IsLetter(character) {
				return
			}
			unmapped++
		}
		if unicode.IsDigit(character) {
			digits++
		}
	}
	partial := unmapped == 1 && digits > 0 && len(characters) <= partialScriptLength
	if mapped == 0 || mapped > maximumScriptLength || (unmapped > 0 && !partial) {
		return
	}
	run[0].Text = first
	for index := range run {
		var builder strings.Builder
		for _, character := range run[index].Text {
			if converted, ok := script[character]; ok {
				character = converted
			}
			builder.WriteRune(character)
		}
		run[index].Text = builder.String()
	}
}

const (
	subscriptDrop    = 0.06
	scriptLeadSpace  = 0.5
	scriptTrailSpace = 0.3
	scriptAttachGap  = 0.08
)

func mdScriptOf(mark, base TextMark, opening, preceding bool) map[rune]rune {
	before, after := base.BBox.Llx-mark.BBox.Urx, mark.BBox.Llx-base.BBox.Urx
	smaller := mark.FontSize <= base.FontSize*scriptSizeRatio
	leading := opening && before >= -1 && before <= scriptLeadSpace*base.FontSize && smaller
	trailing := preceding && after > wordGap && after <= scriptTrailSpace*base.FontSize && smaller && mdFootnoteBase(base.Text)
	if mark.FontSize <= 0 || !(mdAdjacent(mark.BBox, base.BBox) || leading || trailing) {
		return nil
	}
	height := base.BBox.Ury - base.BBox.Lly
	rise := mark.BBox.Lly - base.BBox.Lly
	if mark.FontSize > base.FontSize*scriptSizeRatio {
		if after >= -1 && after <= wordGap && rise > 0.3*height && rise < 0.75*height {
			return superscripts
		}
		return nil
	}
	if rise > 0.25*height && rise < height {
		return superscripts
	}
	if rise < -subscriptDrop*height && rise > -0.6*height && mdAdjacent(mark.BBox, base.BBox) {
		return subscripts
	}
	return nil
}

func mdAdjacent(first, second model.PdfRectangle) bool {
	after := first.Llx - second.Urx
	before := second.Llx - first.Urx
	return (after >= -1 && after <= wordGap) || (before >= -1 && before <= wordGap)
}

func mdPrecedingGlyph(marks []TextMark, index int) int {
	for previous := index - 1; previous >= 0; previous-- {
		if strings.TrimSpace(marks[previous].Text) != "" {
			return previous
		}
	}
	return index
}

func mdScriptListContinues(previous, mark TextMark) bool {
	gap := mark.BBox.Llx - previous.BBox.Urx
	return strings.HasSuffix(previous.Text, ",") && gap >= -1 && gap <= scriptLeadSpace*mark.FontSize
}

func mdFootnoteBase(text string) bool {
	last, _ := utf8.DecodeLastRuneInString(text)
	return unicode.IsLetter(last) || last == ')'
}

var underlinedComparisons = map[string]string{">": "≥", "<": "≤"}

const (
	comparisonUnderlineDrop  = 4.0
	comparisonUnderlineRise  = 1.0
	comparisonUnderlineSlack = 1.5
	comparisonUnderlineSpace = 3.5
)

func mdUnderlinedComparisons(marks []TextMark, strokes []Stroke) {
	for index, mark := range marks {
		glyph := strings.TrimPrefix(mark.Text, " ")
		replacement, ok := underlinedComparisons[glyph]
		if !ok {
			continue
		}
		for _, stroke := range strokes {
			low, high := mdMinMax(stroke.X1, stroke.X2)
			y := (stroke.Y1 + stroke.Y2) / 2
			below := y > mark.BBox.Lly-comparisonUnderlineDrop && y < mark.BBox.Lly+comparisonUnderlineRise
			startsAtGlyph := low > mark.BBox.Llx-comparisonUnderlineSpace && low < mark.BBox.Llx+comparisonUnderlineSlack
			endsAtGlyph := high > mark.BBox.Urx-comparisonUnderlineSlack && high < mark.BBox.Urx+comparisonUnderlineSpace
			if stroke.IsHorizontal() && below && startsAtGlyph && endsAtGlyph {
				marks[index].Text = strings.TrimSuffix(mark.Text, glyph) + replacement
				break
			}
		}
	}
}

// blocks returns the page content as an ordered list of text and table blocks.
// Tables drawn with ruling lines are reconstructed; text that sits above, below
// or between them (including prose that ended up inside a grid) is emitted as
// text blocks in reading order.
func (pt PageText) blocks(pageNumber int, repeatedLabels map[string]bool, carry *mdLineTable) []mdBlock {
	return pt.pageBlocks(pt.contentMarks(pageNumber, repeatedLabels), carry)
}

func (pt PageText) pageBlocks(marks []TextMark, carry *mdLineTable) []mdBlock {
	strokes := pt.cleanStrokes()
	tables, consumed := pt.lineTables(marks)
	var kept []*mdLineTable
	for _, table := range tables {
		if !table.fractionCells(marks, consumed) {
			kept = append(kept, table)
			continue
		}
		for index, mark := range marks {
			center := (mark.BBox.Lly + mark.BBox.Ury) / 2
			if consumed[index] && center <= table.top() && center >= table.bottom() && mark.BBox.Llx >= table.xs[0] && mark.BBox.Urx <= table.xs[len(table.xs)-1] {
				consumed[index] = false
			}
		}
	}
	tables = kept
	var continued []mdBlock
	if carry != nil {
		limit := 0.0
		for _, table := range tables {
			limit = mdMax(limit, table.top())
		}
		var free []TextMark
		var indices []int
		for index, mark := range marks {
			if !consumed[index] && (mark.BBox.Lly+mark.BBox.Ury)/2 > limit {
				free = append(free, mark)
				indices = append(indices, index)
			}
		}
		if continuation, used := carry.continuationRows(free, strokes); continuation != nil {
			continued = []mdBlock{{table: continuation}}
			for position, index := range indices {
				consumed[index] = consumed[index] || used[position]
			}
		}
	}
	right := 0.0
	for index, mark := range marks {
		if !consumed[index] && strings.TrimSpace(mark.Text) != "" {
			right = mdMax(right, mark.BBox.Urx)
		}
	}
	if len(tables) == 0 {
		var rest []TextMark
		for index, mark := range marks {
			if !consumed[index] {
				rest = append(rest, mark)
			}
		}
		return append(continued, mdReconstructBlocks(rest, strokes, right)...)
	}

	var leftover []TextMark
	for i, m := range marks {
		if !consumed[i] {
			leftover = append(leftover, m)
		}
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].top() > tables[j].top() })

	blocks := continued
	used := make([]bool, len(leftover))
	tables[0].attachLeadingLines(leftover, used)
	for index, table := range tables {
		table.attachGroupHeading(leftover, used)
		table.attachFormulaSide(leftover, used)
		floor := 0.0
		if index+1 < len(tables) {
			floor = tables[index+1].top()
		}
		table.attachTrailingLines(leftover, used, floor, strokes)
	}
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
		blocks = append(blocks, mdReconstructBlocks(ms, strokes, right)...)
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
	return mdRenderBlocks(pt.blocks(0, nil, nil))
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
	offset, numbered := mdDetectPageNumbers(pages)
	repeatedLabels := mdDetectRepeatedLabels(pages)
	var blocks []mdBlock
	for pageIndex, pt := range pages {
		if pt == nil {
			continue
		}
		pageNumber := 0
		if numbered {
			pageNumber = pageIndex + 1 + offset
		}
		var carry *mdLineTable
		if len(blocks) > 0 {
			carry = blocks[len(blocks)-1].table
		}
		for index, blk := range pt.blocks(pageNumber, repeatedLabels, carry) {
			if len(blocks) > 0 {
				prev := &blocks[len(blocks)-1]
				if blk.table != nil && prev.table != nil && prev.table.absorb(blk.table) {
					continue
				}
				if index == 0 && prev.continuedBy(blk) {
					prev.text += " " + blk.text
					prev.tail = blk.tail
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
	return strings.ReplaceAll(mdJoinBrokenWords(out), "</u> <u>", " ")
}

var mdWordRegexp = regexp.MustCompile(`(?:\d+-)?\pL+(?:-\pL+)*`)

var mdHyphenBreakRegexp = regexp.MustCompile(`([\pL\d]+)-(?:<br>| )(\p{Ll}\pL*)`)

var mdPlainBreakRegexp = regexp.MustCompile(`(\pL+)<br>(\p{Ll}\pL*)`)

var mdNumberDashRegexp = regexp.MustCompile(`(\d)([-–]) (\d|\()`)

var mdLineEndHyphenRegexp = regexp.MustCompile(`([\pL\d]+)` + lineEndHyphen + `(</u> <u>|<br>| )([\pL\d]+)`)

var suspendedHyphenConjunctions = map[string]bool{"i": true, "lub": true, "albo": true, "oraz": true}

const (
	hyphenStemLength     = 3
	syllableFirstLength  = 3
	syllableSecondLength = 4
)

func mdLineEndHyphenEvidence(words map[string]int, first, second string) (string, bool) {
	joined, hyphenated := strings.ToLower(first+second), strings.ToLower(first+"-"+second)
	if words[joined] != words[hyphenated] {
		if words[joined] > words[hyphenated] {
			return "", true
		}
		return "-", true
	}
	secondRunes := []rune(strings.ToLower(second))
	if len(secondRunes) > hyphenStemLength {
		secondRunes = secondRunes[:hyphenStemLength]
	}
	joinedStems := mdPrefixCount(words, strings.ToLower(first)+string(secondRunes))
	hyphenatedStems := mdPrefixCount(words, strings.ToLower(first)+"-"+string(secondRunes))
	if joinedStems != hyphenatedStems {
		if joinedStems > hyphenatedStems {
			return "", true
		}
		return "-", true
	}
	return "", false
}

func mdLineEndHyphenJoint(words map[string]int, first, second string) string {
	firstRunes, secondRunes := []rune(first), []rune(second)
	last := firstRunes[len(firstRunes)-1]
	if !unicode.IsLetter(last) || !unicode.IsLetter(secondRunes[0]) || len(secondRunes) == 1 {
		return "-"
	}
	if joint, known := mdLineEndHyphenEvidence(words, first, second); known {
		return joint
	}
	syllable := len(firstRunes) > 1 && !strings.ContainsAny(first, "0123456789") &&
		unicode.Is(unicode.Latin, last) && unicode.IsLower(last) && unicode.Is(unicode.Latin, secondRunes[0]) && unicode.IsLower(secondRunes[0])
	if syllable && (len(firstRunes) <= syllableFirstLength || len(secondRunes) <= syllableSecondLength) {
		return ""
	}
	return "-"
}

func mdPrefixCount(words map[string]int, prefix string) int {
	count := 0
	for word, occurrences := range words {
		if strings.HasPrefix(word, prefix) {
			count += occurrences
		}
	}
	return count
}

func mdJoinBrokenWords(text string) string {
	words := make(map[string]int)
	for _, word := range mdWordRegexp.FindAllString(strings.ReplaceAll(text, "<br>", " "), -1) {
		words[strings.ToLower(word)]++
	}
	for resolved := ""; resolved != text; {
		resolved = text
		text = mdLineEndHyphenRegexp.ReplaceAllStringFunc(text, func(match string) string {
			parts := mdLineEndHyphenRegexp.FindStringSubmatch(match)
			if suspendedHyphenConjunctions[strings.ToLower(parts[3])] {
				return parts[1] + "-" + parts[2] + parts[3]
			}
			return parts[1] + mdLineEndHyphenJoint(words, parts[1], parts[3]) + parts[3]
		})
	}
	text = strings.ReplaceAll(text, lineEndHyphen, "-")
	text = mdNumberDashRegexp.ReplaceAllString(text, "${1}${2}${3}")
	text = mdHyphenBreakRegexp.ReplaceAllStringFunc(text, func(match string) string {
		parts := mdHyphenBreakRegexp.FindStringSubmatch(match)
		joined, hyphenated := strings.ToLower(parts[1]+parts[2]), strings.ToLower(parts[1]+"-"+parts[2])
		switch {
		case words[joined] > words[hyphenated]:
			return parts[1] + parts[2]
		case words[hyphenated] > 0:
			return parts[1] + "-" + parts[2]
		}
		return match
	})
	starts, ends := make(map[string]int), make(map[string]int)
	for _, parts := range mdPlainBreakRegexp.FindAllStringSubmatch(text, -1) {
		starts[strings.ToLower(parts[1])]++
		ends[strings.ToLower(parts[2])]++
	}
	return mdPlainBreakRegexp.ReplaceAllStringFunc(text, func(match string) string {
		parts := mdPlainBreakRegexp.FindStringSubmatch(match)
		start, end := strings.ToLower(parts[1]), strings.ToLower(parts[2])
		if words[start] != starts[start] || words[end] != ends[end] {
			return match
		}
		switch {
		case words[start+end] > words[start+"-"+end]:
			return parts[1] + parts[2]
		case words[start+"-"+end] > 0:
			return parts[1] + "-" + parts[2]
		}
		return match
	})
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

var mdInlineSectionRegexp = regexp.MustCompile(`^\d+(\.\d+)+\.?\s[^:]+:\s`)

var mdFrequencyCellRegexp = regexp.MustCompile(`(?i)^\s*((?:bardzo |niezbyt )?(?:często|rzadko)(?: \([^)]*\))?:|(?:częstość )?nieznana(?: \([^)]*\))?:)\s+(\S.*?)\s*$`)

func (t *mdLineTable) splitFrequencyLabels() {
	labelled := false
	for _, row := range t.cells {
		labelled = labelled || len(row) > 1 && row[1] != "" && mdFrequencyLabelRegexp.MatchString(row[0]) && mdFrequencyCellRegexp.FindStringSubmatch(row[0]) == nil
	}
	if !labelled {
		return
	}
	for _, row := range t.cells {
		if len(row) < 2 || row[1] != "" {
			continue
		}
		if label := mdFrequencyCellRegexp.FindStringSubmatch(row[0]); label != nil && !strings.Contains(row[0], "<br>") {
			row[0], row[1] = label[1], label[2]
		}
	}
}

var mdFrequencyLabelRegexp = regexp.MustCompile(`(?i)^(bardzo |niezbyt )?(często|rzadko)( \([^)]*\))?:|^(częstość )?nieznana( \([^)]*\))?:`)

var mdFootnoteListRegexp = regexp.MustCompile(`^(?:\d{1,2}|\p{Ll}|\*{1,3}|[†‡§#])(?:,\s*(?:\d{1,2}|\p{Ll}|\*{1,3}|[†‡§#]))*$`)

var mdFootnoteMarkerRegexp = regexp.MustCompile(`^(\p{Ll}|\d{1,2}|\*{1,3}|[†‡§#¶]|\p{No}{1,2}|[ᵃ-ᶻ])$`)

var mdNumberedItemRegexp = regexp.MustCompile(`^\d{1,2}[.)]\s+\p{Lu}\p{Ll}`)

func mdShouldJoin(prev, cur string) bool {
	prev = strings.TrimRight(prev, " ")
	cur = strings.TrimSpace(cur)
	if prev == "" || cur == "" {
		return false
	}
	if strings.HasPrefix(prev, "|") || strings.HasPrefix(strings.TrimLeft(prev, " "), "- ") {
		return false
	}
	if mdLegendEntryRegexp.MatchString(prev) && mdLegendEntryRegexp.MatchString(cur) {
		return false
	}
	if strings.HasSuffix(prev, ".") {
		return false
	}
	if mdHeadingRegexp.MatchString(prev) || mdHeadingRegexp.MatchString(cur) {
		return false
	}
	previousRunes := []rune(prev)
	if len(previousRunes) < joinableParagraphLength || !unicode.IsLower(previousRunes[len(previousRunes)-1]) {
		return false
	}
	return unicode.IsLower([]rune(cur)[0]) && !mdLabelLineRegexp.MatchString(cur) && !mdFrequencyRegexp.MatchString(cur)
}

const joinableParagraphLength = 80

var mdLegendEntryRegexp = regexp.MustCompile(`^([a-z]|\*{1,3}|[†‡§#]) \p{Lu}`)

var mdLabelLineRegexp = regexp.MustCompile(`(?i)^(tel|faks|fax|e-mail|email|ul\.|strona|www|adres)`)

// mdDetectPageNumbers reports whether the document numbers its pages in the
// margins. It looks for an offset k such that, on most pages, a digit-only
// margin line equal to (pageIndex+1+k) appears — i.e. a number that increments
// by one from page to page. Random integers that happen to sit in a margin do
// not form such a run and are left untouched.
func mdDetectPageNumbers(pages []*PageText) (int, bool) {
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
	votes := make(map[int]int)
	for i, values := range pageValues {
		voted := make(map[int]bool)
		for _, v := range values {
			if offset := v - i - 1; !voted[offset] {
				voted[offset] = true
				votes[offset]++
			}
		}
	}
	bestCount, bestOffset := 0, 0
	for offset, count := range votes {
		if count > bestCount || count == bestCount && offset < bestOffset {
			bestCount, bestOffset = count, offset
		}
	}
	// A run of margin numbers that increments page-to-page on at least a third
	// of the pages (and at least 3) confirms the document numbers its pages.
	return bestOffset, bestCount >= 3 && bestCount*3 >= len(pages)
}

// mdDetectRepeatedLabels returns the set of normalized margin lines (digit runs
// masked) that recur in the margins of most pages — i.e. running headers and
// footers such as "PT/H/0653/001/IA/026". Requiring a strong majority avoids
// pruning ordinary content. A pure page number is excluded here because it is
// handled separately and would otherwise always qualify.
const minimumLabelLength = 4

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
			if (len(norm) < minimumLabelLength && len(strings.TrimSpace(line.text)) < minimumLabelLength) || seen[norm] {
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
	for text := range mdDetectConstantMarkers(pages, nonNil) {
		repeated[text] = true
	}
	return repeated
}

// mdDetectConstantMarkers returns the margin lines, keyed by their literal text, that carry the
// same bare number on a strong majority of pages. Such a footer numbers nothing, so the
// incrementing run in mdDetectPageNumbers never confirms it, and mdNormalizeLabel masks it to a
// single "#" that mdDetectRepeatedLabels discards as too short to be a label.
func mdDetectConstantMarkers(pages []*PageText, nonNil int) map[string]bool {
	counts := make(map[string]int)
	for _, pt := range pages {
		if pt == nil {
			continue
		}
		seen := make(map[string]bool)
		for _, line := range pt.marginLines() {
			if _, isPageNumber := mdPageNumberValue(line.text); !isPageNumber || seen[line.text] {
				continue
			}
			seen[line.text] = true
			counts[line.text]++
		}
	}
	constant := make(map[string]bool)
	for text, count := range counts {
		if count >= 3 && count*3 >= nonNil*2 {
			constant[text] = true
		}
	}
	return constant
}

func mdRenderBlocks(blocks []mdBlock) string {
	var parts []string
	for _, blk := range blocks {
		if formula, ok := blk.table.formula(); ok {
			parts = append(parts, formula)
		} else if blk.table != nil {
			blk.table.splitFrequencyLabels()
			parts = append(parts, strings.TrimRight(blk.table.markdown(), "\n"))
		} else {
			parts = append(parts, blk.text)
		}
	}
	return strings.Join(parts, "\n\n") + "\n"
}

var mdMultiplicationCells = map[string]bool{"x": true, "X": true, "×": true}

func (t *mdLineTable) formula() (string, bool) {
	if t == nil || len(t.cells) != 2 {
		return "", false
	}
	numerator, denominator := t.cells[0], ""
	for _, cell := range t.cells[1] {
		if cell != "" && denominator != "" {
			return "", false
		}
		if cell != "" {
			denominator = cell
		}
	}
	result := len(numerator) - 1
	for result > 0 && numerator[result] == "" {
		result--
	}
	multiplied := false
	for _, cell := range numerator {
		multiplied = multiplied || mdMultiplicationCells[cell]
	}
	if denominator == "" || !multiplied || !strings.HasPrefix(numerator[result], "=") {
		return "", false
	}
	left := ""
	var terms []string
	for _, cell := range numerator[:result] {
		switch {
		case cell == "=":
			left, terms = strings.Join(terms, " ")+" = ", nil
		case mdMultiplicationCells[cell]:
			terms = append(terms, "×")
		case cell != "":
			terms = append(terms, cell)
		}
	}
	text := left + "(" + strings.Join(terms, " ") + ") / " + mdDenominator(strings.ReplaceAll(denominator, "<br>", " ")) + " " + numerator[result]
	return strings.TrimSpace(strings.ReplaceAll(text, "<br>", " ")), true
}

const formulaSideReach = 14.0

func (t *mdLineTable) attachFormulaSide(marks []TextMark, used []bool) {
	if _, ok := t.formula(); !ok || t.cells[0][0] != "=" {
		return
	}
	var side []TextMark
	var indices []int
	for index, mark := range marks {
		center := (mark.BBox.Lly + mark.BBox.Ury) / 2
		if !used[index] && strings.TrimSpace(mark.Text) != "" && mark.BBox.Urx <= t.xs[0] && center <= t.top()+formulaSideReach && center >= t.bottom()-formulaSideReach {
			side = append(side, mark)
			indices = append(indices, index)
		}
	}
	if len(side) == 0 {
		return
	}
	for _, index := range indices {
		used[index] = true
	}
	lines, _ := mdLines(side)
	t.xs = append([]float64{t.xs[0]}, t.xs...)
	for row := range t.cells {
		t.cells[row] = append([]string{""}, t.cells[row]...)
	}
	t.cells[0][0] = mdColumnText(lines)
}

func (t *mdLineTable) fractionCells(marks []TextMark, consumed map[int]bool) bool {
	if len(t.cells) == 1 && len(t.cells[0]) == 1 {
		return strings.HasPrefix(t.cells[0][0], "=")
	}
	if len(t.cells) != 2 || len(t.cells[0]) != 1 {
		return false
	}
	for index, mark := range marks {
		center := (mark.BBox.Lly + mark.BBox.Ury) / 2
		if !consumed[index] && strings.TrimSpace(mark.Text) == "=" && center <= t.top()+formulaSideReach && center >= t.bottom()-formulaSideReach {
			return true
		}
	}
	return false
}

// mdLineTable is a table reconstructed from the ruling lines drawn on a page.
type mdLineTable struct {
	xs, ys   []float64
	cells    [][]string
	openRows int
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

func (t *mdLineTable) absorb(next *mdLineTable) bool {
	rows := next.cells
	if mapping := t.columnsOf(next); mapping != nil {
		rows = mdPlaceColumns(next.cells, mapping, t.cols())
	} else if widened := next.columnsOf(t); widened != nil {
		t.cells = mdPlaceColumns(t.cells, widened, next.cols())
		t.xs = next.xs
	} else if centered := next.columnsByCenter(t); centered != nil && len(t.ys) > 0 && len(next.ys) > 0 && mdAbs(t.bottom()-next.top()) < tableTouchSlack {
		t.cells = mdPlaceColumns(t.cells, centered, next.cols())
		t.xs = next.xs
	} else if t.cols() != next.cols() {
		return false
	}
	rows = rows[t.repeatedHeaderRows(rows):]
	if open := len(t.cells) - t.openRows; t.openRows > 0 && open >= 0 && len(rows) > 1 && rows[0][0] != "" && rows[1][0] == "" && t.cells[open][0] == "" {
		t.cells[open][0], rows[0][0] = rows[0][0], ""
	}
	t.openRows = next.openRows
	if len(rows) > 0 && t.continueCutRow(rows[0]) {
		rows = rows[1:]
	}
	t.cells = append(t.cells, rows...)
	return true
}

const columnMatchTolerance = 4.0

const tableTouchSlack = 2.0

func (t *mdLineTable) columnsByCenter(other *mdLineTable) []int {
	if len(t.xs) != t.cols()+1 || len(other.xs) != other.cols()+1 {
		return nil
	}
	mapping := make([]int, other.cols())
	for column := range mapping {
		center := (other.xs[column] + other.xs[column+1]) / 2
		mapping[column] = sort.SearchFloat64s(t.xs, center) - 1
		if mapping[column] < 0 || mapping[column] >= t.cols() || column > 0 && mapping[column] <= mapping[column-1] {
			return nil
		}
	}
	return mapping
}

func (t *mdLineTable) columnsOf(other *mdLineTable) []int {
	if len(t.xs) != t.cols()+1 || len(other.xs) != other.cols()+1 {
		return nil
	}
	mapping := make([]int, other.cols())
	candidate := 0
	for column := range mapping {
		for candidate < t.cols() && mdAbs(t.xs[candidate]-other.xs[column]) > columnMatchTolerance {
			candidate++
		}
		if candidate == t.cols() {
			return nil
		}
		mapping[column] = candidate
		candidate++
	}
	return mapping
}

func mdPlaceColumns(cells [][]string, mapping []int, width int) [][]string {
	placed := make([][]string, len(cells))
	for r, row := range cells {
		placed[r] = make([]string, width)
		for c, cell := range row {
			placed[r][mapping[c]] = cell
		}
	}
	return placed
}

func (t *mdLineTable) repeatedHeaderRows(rows [][]string) int {
	limit := 3
	if len(t.cells) < limit {
		limit = len(t.cells)
	}
	if len(rows) < limit {
		limit = len(rows)
	}
	for count := limit; count > 0; count-- {
		repeated := true
		for r := 0; r < count && repeated; r++ {
			header := mdNormalizedRow(t.cells[r])
			repeated = strings.Trim(header, "|") != "" && header == mdNormalizedRow(rows[r])
		}
		if repeated {
			return count
		}
	}
	return 0
}

func mdNormalizedRow(row []string) string {
	normalized := make([]string, len(row))
	for c, cell := range row {
		normalized[c] = strings.Join(strings.Fields(strings.ReplaceAll(cell, "<br>", " ")), " ")
	}
	return strings.Join(normalized, "|")
}

func (t *mdLineTable) labelsCapitalised() bool {
	for _, row := range t.cells {
		if label := []rune(strings.TrimSpace(row[0])); len(label) > 0 && unicode.IsLower(label[0]) {
			return false
		}
	}
	return true
}

func (t *mdLineTable) continueCutRow(row []string) bool {
	if label := []rune(strings.TrimSpace(row[0])); len(label) > 0 && (unicode.IsLower(label[0]) || label[0] == '(') && t.labelsCapitalised() {
		anchor := t.lastFilledRow(0)
		cut := false
		for _, cell := range row[1:] {
			if mdFrequencyRegexp.MatchString(strings.TrimSpace(cell)) {
				cut = false
				break
			}
			if characters := []rune(strings.TrimSpace(cell)); len(characters) > 0 && unicode.IsLower(characters[0]) {
				cut = true
			}
		}
		if anchor >= 0 && anchor == len(t.cells)-1 && cut {
			t.appendToLastRow(row)
			return true
		}
		if anchor >= 0 {
			t.cells[anchor][0] += "<br>" + row[0]
			row[0] = ""
		}
	}
	column := -1
	filled, continued, lowercase := 0, 0, 0
	for c, cell := range row {
		characters := []rune(strings.TrimSpace(cell))
		if len(characters) == 0 {
			continue
		}
		filled++
		column = c
		if c == 0 || len(t.cells) == 0 || mdFrequencyRegexp.MatchString(cell) {
			continue
		}
		anchor := strings.TrimSpace(t.cells[len(t.cells)-1][c])
		if anchor == "" {
			continue
		}
		if unicode.IsLower(characters[0]) {
			lowercase++
			continued++
		} else if !strings.ContainsAny(anchor[len(anchor)-1:], ".;:)") {
			continued++
		}
	}
	if filled > 1 {
		if continued < filled || lowercase == 0 {
			return false
		}
		t.appendToLastRow(row)
		return true
	}
	if column < 0 {
		return true
	}
	anchor := t.lastFilledRow(column)
	if !unicode.IsLower([]rune(strings.TrimSpace(row[column]))[0]) || anchor < 0 || mdFrequencyRegexp.MatchString(strings.TrimSpace(row[column])) {
		return false
	}
	t.cells[anchor][column] += "<br>" + row[column]
	return true
}

var mdFrequencyRegexp = regexp.MustCompile(`(?i)^(bardzo |niezbyt )?(często|rzadko)|^częstość`)

func (t *mdLineTable) wrapsInto(previousEnds []float64, row []string, leads []float64) bool {
	previous := t.cells[len(t.cells)-1]
	if len(row) == 2 && row[0] == "" && row[1] != "" && previous[1] != "" && !mdFrequencyRegexp.MatchString(row[1]) {
		return true
	}
	continued := false
	for column := range row {
		if row[column] == "" {
			continue
		}
		if previous[column] == "" || mdFrequencyRegexp.MatchString(row[column]) {
			return false
		}
		limit := t.xs[column+1]
		if column+1 < len(t.xs)-1 {
			limit -= borderlessGutter
		}
		wrapped := limit-previousEnds[column] <= leads[column]+lineEndSlack
		lowercase := unicode.IsLower([]rune(row[column])[0])
		if column == 0 {
			if !wrapped || !lowercase || strings.HasSuffix(row[0], ":") {
				return false
			}
		} else if !wrapped && !lowercase {
			return false
		}
		continued = true
	}
	return continued
}

func (t *mdLineTable) appendToLastRow(row []string) {
	last := t.cells[len(t.cells)-1]
	for column, cell := range row {
		if strings.TrimSpace(cell) == "" {
			continue
		}
		if strings.TrimSpace(last[column]) != "" {
			cell = last[column] + "<br>" + cell
		}
		last[column] = cell
	}
}

const (
	leadingLinesLimit = 6
	leadingLinePitch  = 18.0
)

const (
	trailingLinePitch    = 30.0
	trailingTableColumns = 2
)

var mdCaptionRegexp = regexp.MustCompile(`(?i)^(tabela|tabl|table|rycina|wykres)`)

func (t *mdLineTable) attachGroupHeading(marks []TextMark, used []bool) {
	groups := map[string]bool{}
	for _, row := range t.cells {
		filled := 0
		for _, cell := range row {
			if cell != "" {
				filled++
			}
		}
		if len(row) > 1 && filled == 1 && row[0] != "" {
			groups[strings.Fields(row[0])[0]] = true
		}
	}
	if len(groups) == 0 {
		return
	}
	var heading []int
	nearest := 0.0
	for index, mark := range marks {
		center := (mark.BBox.Lly + mark.BBox.Ury) / 2
		if used[index] || strings.TrimSpace(mark.Text) == "" || center <= t.top() || center > t.top()+leadingLinePitch {
			continue
		}
		if mark.BBox.Llx < t.xs[0]-borderlessAlignment || mark.BBox.Urx > t.xs[len(t.xs)-1]+borderlessAlignment {
			return
		}
		if len(heading) > 0 && mdAbs(center-nearest) > mdCellLineTolerance {
			return
		}
		heading = append(heading, index)
		nearest = center
	}
	if len(heading) == 0 {
		return
	}
	cellMarks := make([]mdCellMark, len(heading))
	for position, index := range heading {
		mark := marks[index]
		cellMarks[position] = mdCellMark{mark.BBox.Llx, mark.BBox.Urx, nearest, mark.Text}
	}
	text := strings.ReplaceAll(mdJoinCell(cellMarks), "|", "¦")
	if !groups[strings.Fields(text)[0]] || mdCaptionRegexp.MatchString(text) || strings.HasSuffix(text, ":") || strings.HasSuffix(text, ".") {
		return
	}
	for _, index := range heading {
		used[index] = true
	}
	row := make([]string, len(t.cells[0]))
	row[0] = text
	t.cells = append([][]string{row}, t.cells...)
}

func (t *mdLineTable) continuationRows(marks []TextMark, strokes []Stroke) (*mdLineTable, []bool) {
	top := 0.0
	for _, mark := range marks {
		if strings.TrimSpace(mark.Text) != "" {
			top = mdMax(top, (mark.BBox.Lly+mark.BBox.Ury)/2)
		}
	}
	seed := &mdLineTable{xs: t.xs, ys: []float64{top + mdCellLineTolerance + 1}, cells: [][]string{make([]string, len(t.xs)-1)}}
	used := make([]bool, len(marks))
	seed.attachTrailingLines(marks, used, 0, strokes)
	if len(seed.cells) == 1 {
		return nil, used
	}
	return &mdLineTable{xs: t.xs, ys: seed.ys, cells: seed.cells[1:]}, used
}

func (t *mdLineTable) attachTrailingLines(marks []TextMark, used []bool, floor float64, strokes []Stroke) {
	if len(t.xs) != trailingTableColumns+1 || len(t.cells) == 0 || len(t.cells[0]) != len(t.xs)-1 {
		return
	}
	var below []int
	var centers []float64
	for index, mark := range marks {
		center := (mark.BBox.Lly + mark.BBox.Ury) / 2
		if !used[index] && strings.TrimSpace(mark.Text) != "" && center < t.bottom() && center > floor {
			below = append(below, index)
			centers = append(centers, center)
		}
	}
	lines := mdCluster(append([]float64(nil), centers...), mdCellLineTolerance)
	sort.Sort(sort.Reverse(sort.Float64Slice(lines)))
	borders := t.xs[1 : len(t.xs)-1]
	kinds := make([]int, len(lines))
	rows := make([][]string, len(lines))
	members := make([][]int, len(lines))
	for position, line := range lines {
		var lineMarks []TextMark
		for order, index := range below {
			if mdAbs(centers[order]-line) <= mdCellLineTolerance {
				lineMarks = append(lineMarks, marks[index])
				members[position] = append(members[position], index)
			}
		}
		lineWords, _ := mdLines(lineMarks)
		var words []mdWord
		for _, wordsOfLine := range lineWords {
			words = append(words, wordsOfLine...)
		}
		sort.Slice(words, func(first, second int) bool { return words[first].x0 < words[second].x0 })
		inside, crossing := true, false
		columns := make([][]mdWord, len(t.xs)-1)
		for order, word := range words {
			inside = inside && word.x0 >= t.xs[0]-borderlessAlignment && word.x1 <= t.xs[len(t.xs)-1]+borderlessAlignment
			for _, border := range borders {
				crossing = crossing || word.x0 < border-1 && word.x1 > border+1
			}
			if order > 0 {
				previousColumn := sort.SearchFloat64s(borders, (words[order-1].x0+words[order-1].x1)/2)
				column := sort.SearchFloat64s(borders, (word.x0+word.x1)/2)
				crossing = crossing || column != previousColumn && word.x0-words[order-1].x1 < borderlessGutter
			}
		}
		for _, word := range words {
			column := sort.SearchFloat64s(borders, (word.x0+word.x1)/2)
			if crossing {
				column = 0
			}
			columns[column] = append(columns[column], word)
		}
		rows[position] = make([]string, len(t.xs)-1)
		filled := 0
		for column, columnWords := range columns {
			if len(columnWords) > 0 {
				rows[position][column] = strings.ReplaceAll(mdRenderLine(columnWords, strokes), "|", "¦")
				filled++
			}
		}
		if label := mdFrequencyCellRegexp.FindStringSubmatch(strings.Join(rows[position], " ")); label != nil && len(rows[position]) > 1 {
			rows[position] = make([]string, len(t.xs)-1)
			rows[position][0], rows[position][1] = label[1], label[2]
			crossing, filled = false, 2
		}
		switch {
		case !inside:
			kinds[position] = trailingStop
		case crossing || filled == 1 && len(columns[0]) > 0 && unicode.IsUpper([]rune(rows[position][0])[0]):
			kinds[position] = trailingHeading
		case filled > 1:
			kinds[position] = trailingRow
		default:
			kinds[position] = trailingContinuation
		}
	}
	previous := t.bottom()
	for position, line := range lines {
		next := position+1 < len(lines) && kinds[position+1] == trailingRow
		if previous-line > trailingLinePitch || kinds[position] == trailingStop || kinds[position] == trailingHeading && !next || kinds[position] == trailingContinuation && position == 0 {
			return
		}
		if kinds[position] == trailingContinuation {
			t.appendToLastRow(rows[position])
		} else {
			t.cells = append(t.cells, rows[position])
		}
		for _, index := range members[position] {
			used[index] = true
		}
		previous = line
	}
}

const (
	trailingStop = iota
	trailingHeading
	trailingRow
	trailingContinuation
)

func (t *mdLineTable) attachLeadingLines(marks []TextMark, used []bool) {
	if len(t.xs) < 3 || len(t.cells) == 0 || len(t.cells[0]) != len(t.xs)-1 {
		return
	}
	var above []int
	var centers []float64
	for index, mark := range marks {
		if strings.TrimSpace(mark.Text) == "" {
			continue
		}
		center := (mark.BBox.Lly + mark.BBox.Ury) / 2
		if center <= t.top() {
			continue
		}
		if mark.BBox.Llx < t.xs[1]-borderlessAlignment || mark.BBox.Urx > t.xs[len(t.xs)-1]+borderlessAlignment {
			return
		}
		above = append(above, index)
		centers = append(centers, center)
	}
	lines := mdCluster(append([]float64(nil), centers...), mdCellLineTolerance)
	if len(above) == 0 || len(lines) > leadingLinesLimit {
		return
	}
	leading := make([]TextMark, len(above))
	for position, index := range above {
		leading[position] = marks[index]
	}
	words, _ := mdWords(leading, nil)
	for _, word := range words {
		for _, border := range t.xs[2 : len(t.xs)-1] {
			if word.x0 < border-1 && word.x1 > border+1 {
				return
			}
		}
	}
	sort.Float64s(lines)
	previous := t.top()
	for _, line := range lines {
		if line-previous > leadingLinePitch {
			return
		}
		previous = line
	}
	cellMarks := make([][]mdCellMark, len(t.xs)-1)
	for position, index := range above {
		mark := marks[index]
		column := 0
		for column+1 < len(t.xs)-1 && (mark.BBox.Llx+mark.BBox.Urx)/2 >= t.xs[column+1] {
			column++
		}
		cellMarks[column] = append(cellMarks[column], mdCellMark{mark.BBox.Llx, mark.BBox.Urx, centers[position], mark.Text})
		used[index] = true
	}
	row := make([]string, len(t.xs)-1)
	for column, cms := range cellMarks {
		if len(cms) > 0 {
			row[column] = strings.ReplaceAll(mdJoinCell(cms), "|", "¦")
		}
	}
	t.cells = append([][]string{row}, t.cells...)
}

func (t *mdLineTable) lastFilledRow(column int) int {
	for r := len(t.cells) - 1; r >= 0; r-- {
		if strings.TrimSpace(t.cells[r][column]) != "" {
			return r
		}
	}
	return -1
}

type mdVSeg struct{ x, ylo, yhi float64 }
type mdHSeg struct{ y, x0, x1 float64 }

// lineTable reconstructs the table drawn with ruling lines on the page, or
// returns nil if no such table is found.
func (pt PageText) lineTables(marks []TextMark) (tables []*mdLineTable, consumed map[int]bool) {
	var columnRules []mdVSeg
	var vsegs []mdVSeg
	var hsegs []mdHSeg
	tileStrokes := make(map[Stroke]bool)
	for _, tile := range pt.tiledCellStrokes() {
		tileStrokes[tile] = true
	}
	tileRules := make(map[mdHSeg]bool)
	for _, s := range mdMergeCollinearVerticals(pt.cleanStrokes()) {
		if s.IsVertical() {
			x := (s.X1 + s.X2) / 2
			lo, hi := mdMinMax(s.Y1, s.Y2)
			length := hi - lo
			// Long verticals are column-border candidates. Short ones (e.g. a
			// single-line header row separator) are kept only as vsegs so they can
			// still establish the table extent and per-row column structure.
			if length > minimumColumnRuleLength {
				columnRules = append(columnRules, mdVSeg{x, lo, hi})
			}
			if length > 5 {
				vsegs = append(vsegs, mdVSeg{x, lo, hi})
			}
		}
		if s.IsHorizontal() {
			lo, hi := mdMinMax(s.X1, s.X2)
			hsegs = append(hsegs, mdHSeg{(s.Y1 + s.Y2) / 2, lo, hi})
			if tileStrokes[s] {
				tileRules[hsegs[len(hsegs)-1]] = true
			}
		}
	}
	pageTables, pageConsumed := pt.ruledTables(marks, columnRules, vsegs, hsegs, tileRules)
	consumed = make(map[int]bool)
	for _, pageTable := range pageTables {
		top, bottom := pageTable.top(), pageTable.bottom()
		var tableRules, tableVsegs []mdVSeg
		for _, rule := range columnRules {
			if rule.yhi > bottom && rule.ylo < top {
				tableRules = append(tableRules, rule)
			}
		}
		for _, v := range vsegs {
			if v.yhi > bottom && v.ylo < top {
				tableVsegs = append(tableVsegs, mdVSeg{v.x, mdMax(v.ylo, bottom), mdMin(v.yhi, top)})
			}
		}
		tableTables, tableConsumed := pt.ruledTables(marks, tableRules, tableVsegs, hsegs, tileRules)
		if len(tableTables) == 0 {
			tableTables = []*mdLineTable{pageTable}
			tableConsumed = make(map[int]bool)
			for index := range pageConsumed {
				if y := (marks[index].BBox.Lly + marks[index].BBox.Ury) / 2; y >= bottom && y <= top {
					tableConsumed[index] = true
				}
			}
		}
		tables = append(tables, tableTables...)
		for index := range tableConsumed {
			consumed[index] = true
		}
	}
	for _, group := range mdOverlappingRules(columnRules) {
		low, high := group[0].ylo, group[0].yhi
		for _, rule := range group {
			low, high = mdMin(low, rule.ylo), mdMax(high, rule.yhi)
		}
		covered := false
		for _, table := range tables {
			covered = covered || (table.bottom() < high && table.top() > low)
		}
		if covered {
			continue
		}
		var groupVsegs []mdVSeg
		for _, v := range vsegs {
			if v.yhi > low && v.ylo < high {
				groupVsegs = append(groupVsegs, mdVSeg{v.x, mdMax(v.ylo, low), mdMin(v.yhi, high)})
			}
		}
		groupTables, groupConsumed := pt.ruledTables(marks, group, groupVsegs, hsegs, tileRules)
		tables = append(tables, groupTables...)
		for index := range groupConsumed {
			consumed[index] = true
		}
	}
	return tables, consumed
}

func mdOverlappingRules(rules []mdVSeg) [][]mdVSeg {
	sorted := append([]mdVSeg(nil), rules...)
	sort.Slice(sorted, func(first, second int) bool {
		return sorted[first].ylo < sorted[second].ylo
	})
	var groups [][]mdVSeg
	high := 0.0
	for _, rule := range sorted {
		if len(groups) == 0 || rule.ylo > high+collinearGap {
			groups = append(groups, nil)
			high = rule.yhi
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], rule)
		high = mdMax(high, rule.yhi)
	}
	return groups
}

func (pt PageText) ruledTables(marks []TextMark, columnRules []mdVSeg, vsegs []mdVSeg, hsegs []mdHSeg, tileRules map[mdHSeg]bool) (tables []*mdLineTable, consumed map[int]bool) {
	var vxs []float64
	for _, rule := range columnRules {
		vxs = append(vxs, rule.x)
	}
	if len(vxs) == 0 {
		return nil, nil
	}
	xs := mdCluster(vxs, 8)

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
	implicitBorders := mdImplicitOuterBorders(hsegs, xs, vmin, vmax)
	xs = append(xs, implicitBorders...)
	sort.Float64s(xs)
	if len(xs) < 3 {
		return nil, nil
	}
	ruledBorders := pt.ruledColumnBorders(xs, vmin, vmax)
	var ruledXs []float64
	for index, x := range xs {
		if ruledBorders[index] {
			ruledXs = append(ruledXs, x)
		}
	}
	words, markWord := mdWords(marks, ruledXs)
	var drawnYs []float64
	for _, h := range hsegs {
		if !tileRules[h] && h.x1-h.x0 > minimumColumnRuleLength {
			drawnYs = append(drawnYs, h.y)
		}
	}
	var rules []mdHSeg
	var hys []float64
	for _, h := range hsegs {
		lineClip := tileRules[h] && !mdNearAny(drawnYs, h.y, 2) && mdNearAny(drawnYs, h.y, lineClipOffset)
		if !mdUnderline(h, words, xs) && !lineClip {
			rules = append(rules, h)
			hys = append(hys, h.y)
		}
	}
	hsegs = rules
	var ys []float64
	for _, y := range mdPickRowBorders(mdCluster(hys, 3), hsegs, xs) {
		if y >= vmin-3 && y <= vmax+3 {
			ys = append(ys, y)
		}
	}
	if len(ys) > 0 {
		sort.Float64s(ys)
		if ys[0]-vmin > openRowMinimumHeight {
			ys = append([]float64{vmin}, ys...)
		}
		if vmax-ys[len(ys)-1] > openRowMinimumHeight {
			ys = append(ys, vmax)
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
	spanning := make([]bool, rows)
	for r := 0; r < rows; r++ {
		top, bot := ys[r], ys[r+1]
		height := top - bot
		var present []float64
		for index, x := range xs {
			covered := false
			for _, v := range vsegs {
				if mdAbs(v.x-x) <= 4 && mdMin(v.yhi, top)-mdMax(v.ylo, bot) > height*0.6 {
					covered = true
					break
				}
			}
			if covered && !mdNearAny(implicitBorders, x, 0.01) {
				present = append(present, xs[index])
			}
		}
		ownCellBorders := false
		for index, x := range xs {
			if !ruledBorders[index] && mdNearAny(present, x, 0.01) {
				ownCellBorders = true
			}
		}
		if len(present) > 0 {
			var aligned []float64
			for index, x := range xs {
				addable := !ruledBorders[index] && (!ownCellBorders || mdNearAny(implicitBorders, x, 0.01))
				if mdNearAny(present, x, 0.01) || (addable && !mdWordStraddles(words, top, bot, x)) {
					aligned = append(aligned, x)
				}
			}
			present = aligned
		}
		verticalCount[r] = len(present)
		rowBorders[r] = present
	}
	for r := 0; r < rows; r++ {
		above, below := r > 0 && verticalCount[r-1] >= 2, r+1 < rows && verticalCount[r+1] >= 2
		if verticalCount[r] > 0 || !above && !below {
			continue
		}
		var aligned []float64
		straddled := false
		for index, x := range xs {
			if ruledBorders[index] {
				continue
			}
			straddled = straddled || mdWordStraddles(words, ys[r], ys[r+1], x)
			aligned = append(aligned, x)
		}
		if len(aligned) < 2 {
			continue
		}
		tabular := mdTabularLineInRow(words, ys[r], ys[r+1], aligned[1:len(aligned)-1])
		heading := above && below && mdSingleTextLine(words, ys[r], ys[r+1]) && !mdVisibleRowBorder(pt.strokes, ys[r], ys[r+1])
		if (straddled || !above || !below) && !tabular && !heading {
			continue
		}
		if len(aligned) >= 2 {
			rowBorders[r] = aligned
			verticalCount[r] = len(aligned)
			spanning[r] = straddled
		}
	}
	for r := 0; r < rows; r++ {
		if len(rowBorders[r]) < 2 {
			rowBorders[r] = []float64{xs[0], xs[len(xs)-1]}
		}
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
		word := words[markWord[i]]
		cx := (word.x0 + word.x1) / 2
		cy := (m.BBox.Lly + m.BBox.Ury) / 2
		r := mdFindCell(ys, cy, true)
		if r < 0 || verticalCount[r] < 2 {
			continue
		}
		c := mdFindCell(rowBorders[r], cx, false)
		if (spanning[r] || len(rowBorders[r]) == trailingTableColumns+1 && !mdVisibleRowBorder(pt.strokes, ys[r], ys[r+1])) && mdLineStraddles(words, word.y, rowBorders[r][1:len(rowBorders[r])-1]) {
			c = 0
		}
		if c < 0 {
			continue
		}
		gc := mdNearestIndex(xs, rowBorders[r][c])
		rightX := rowBorders[r][c+1]
		// Merge upward across rows when no horizontal border separates this cell
		// from the one above for this column span (rowspan).
		for r > 0 && verticalCount[r-1] >= 2 && !mdHasHBorder(hsegs, ys[r], xs[gc], rightX) {
			r--
		}
		markRow[i] = r
		cellMarks[[2]int{r, gc}] = append(cellMarks[[2]int{r, gc}],
			mdCellMark{m.BBox.Llx, m.BBox.Urx, cy, m.Text})
	}
	for key, cms := range cellMarks {
		top, lowest := rows, cms[0].y
		for _, cm := range cms {
			if natural := mdFindCell(ys, cm.y, true); natural >= 0 && natural < top {
				top = natural
			}
			lowest = mdMin(lowest, cm.y)
		}
		target := [2]int{top, key[1]}
		bottomAligned := top < rows && lowest-ys[top+1] < bottomAlignedCellSlack && mdHasHBorder(hsegs, ys[top+1], xs[key[1]], xs[mdMinIndex(key[1]+1, len(xs)-1)])
		if bottomAligned && top > key[0] && len(cellMarks[target]) == 0 && mdRowBaselineMatches(cellMarks, ys, key[1], top, lowest) {
			cellMarks[target] = cms
			delete(cellMarks, key)
		}
	}
	fullCells := make([][][]string, rows)
	for r := range fullCells {
		fullCells[r] = mdSplitCellRow(cellMarks, r, cols, !mdVisibleRowBorder(pt.strokes, ys[r], ys[r+1]), spanning[r])
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
		var segCells [][]string
		for k := start; k < r; k++ {
			segCells = append(segCells, fullCells[k]...)
		}
		openRows := 0
		for k := r - 1; k >= start; k-- {
			if len(cellMarks[[2]int{k, 0}]) > 0 {
				openRows = 0
				break
			}
			for _, row := range fullCells[k] {
				if strings.Join(row, "") != "" {
					openRows++
				}
			}
			if mdHasHBorder(hsegs, ys[k], xs[0], xs[1]) {
				break
			}
			if k == start {
				openRows = 0
			}
		}
		seg := &mdLineTable{xs: xs, ys: ys[start : r+1], cells: segCells, openRows: openRows}
		seg.dropEmptyRows()
		seg.dropEmptyColumns()
		seg.mergeOrphanColumns(vsegs)
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
	if len(t.xs) == cols+1 {
		var xs []float64
		for c := 0; c < cols; c++ {
			if keep[c] {
				xs = append(xs, t.xs[c])
			}
		}
		t.xs = append(xs, t.xs[cols])
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

const (
	orphanColumnHeaderRows  = 2
	orphanColumnMinimumRows = 4
	orphanColumnCoverage    = 0.5
)

func (t *mdLineTable) mergeOrphanColumns(vsegs []mdVSeg) {
	if len(t.cells) < orphanColumnMinimumRows || len(t.xs) != len(t.cells[0])+1 || len(t.ys) < 2 {
		return
	}
	top, bottom := t.ys[0], t.ys[len(t.ys)-1]
	for column := len(t.cells[0]) - 1; column > 0; column-- {
		covered := 0.0
		for _, v := range vsegs {
			if mdAbs(v.x-t.xs[column]) <= 4 {
				covered += mdMax(0, mdMin(v.yhi, top)-mdMax(v.ylo, bottom))
			}
		}
		if covered > orphanColumnCoverage*(top-bottom) {
			continue
		}
		filled := -1
		orphan := true
		for row, cells := range t.cells {
			if strings.TrimSpace(cells[column]) == "" {
				continue
			}
			if filled >= 0 || row < orphanColumnHeaderRows || strings.TrimSpace(cells[column-1]) != "" {
				orphan = false
				break
			}
			filled = row
		}
		if !orphan || filled < 0 {
			continue
		}
		t.cells[filled][column-1] = t.cells[filled][column]
		for row := range t.cells {
			t.cells[row] = append(t.cells[row][:column], t.cells[row][column+1:]...)
		}
		t.xs = append(t.xs[:column], t.xs[column+1:]...)
	}
}

const visibleBorderSlack = 1.5

func mdVisibleRowBorder(strokes []Stroke, top, bottom float64) bool {
	for _, stroke := range strokes {
		if stroke.IsHorizontal() && (mdAbs(stroke.Y1-top) < visibleBorderSlack || mdAbs(stroke.Y1-bottom) < visibleBorderSlack) {
			return true
		}
	}
	return false
}

func mdSplitCellRow(cellMarks map[[2]int][]mdCellMark, row, cols int, splittable, spanning bool) [][]string {
	type located struct {
		column int
		mark   mdCellMark
	}
	var all []located
	for column := 0; column < cols; column++ {
		for _, mark := range cellMarks[[2]int{row, column}] {
			all = append(all, located{column, mark})
		}
	}
	sort.SliceStable(all, func(first, second int) bool { return all[first].mark.y > all[second].mark.y })
	var lines [][]located
	for _, item := range all {
		if last := len(lines) - 1; last >= 0 && lines[last][0].mark.y-item.mark.y <= mdCellLineTolerance {
			lines[last] = append(lines[last], item)
			continue
		}
		lines = append(lines, []located{item})
	}
	aligned := 0
	for _, line := range lines {
		columns := map[int]bool{}
		for _, item := range line {
			columns[item.column] = true
		}
		if len(columns) > 1 {
			aligned++
		}
	}
	var groups [][]located
	for index, line := range lines {
		label, left := "", 0.0
		for _, item := range line {
			if item.column == 0 && strings.TrimSpace(item.mark.s) != "" && (label == "" || item.mark.x0 < left) {
				label, left = strings.TrimSpace(item.mark.s), item.mark.x0
			}
		}
		opening := []rune(label + " ")[0]
		separate := aligned >= minimumAlignedCellLines || spanning && aligned > 0
		starts := index == 0 || splittable && separate && label != "" && !unicode.IsLower(opening) && opening != '('
		if starts {
			groups = append(groups, nil)
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], line...)
	}
	result := [][]string{make([]string, cols)}
	if len(groups) > 0 {
		result = nil
	}
	for _, group := range groups {
		cells := make([]string, cols)
		byColumn := make([][]mdCellMark, cols)
		for _, item := range group {
			byColumn[item.column] = append(byColumn[item.column], item.mark)
		}
		for column, marks := range byColumn {
			if len(marks) > 0 {
				cells[column] = strings.ReplaceAll(mdJoinCell(marks), "|", "¦")
			}
		}
		result = append(result, cells)
	}
	return result
}

const minimumAlignedCellLines = 2

const bottomAlignedCellSlack = 8.0

const (
	rowBaselineSlack       = 2.0
	minimumBaselineMatches = 2
)

func mdRowBaselineMatches(cellMarks map[[2]int][]mdCellMark, ys []float64, column, row int, lowest float64) bool {
	matched := 0
	for key, cms := range cellMarks {
		if key[1] == column {
			continue
		}
		other := math.Inf(1)
		for _, cm := range cms {
			if mdFindCell(ys, cm.y, true) == row {
				other = mdMin(other, cm.y)
			}
		}
		if !math.IsInf(other, 1) && mdAbs(other-lowest) <= rowBaselineSlack {
			matched++
		}
	}
	return matched >= minimumBaselineMatches
}

func mdMinIndex(first, second int) int {
	if first < second {
		return first
	}
	return second
}

type mdCellMark struct {
	x0, x1, y float64
	s         string
}

// mdCellLineTolerance is the vertical distance below which two marks of a cell belong to the
// same line of text.
const mdCellLineTolerance = 4

// mdLineOrder returns the indices of `count` marks ordered top to bottom and then left to
// right. The marks are grouped into lines before being sorted, because comparing two of them by
// "same line within a tolerance, otherwise by height" is not a strict weak ordering: three marks
// can each sit within the tolerance of their neighbour yet span more than it end to end, which
// leaves sort.Slice free to interleave the characters of adjacent lines, returning "piersiowej"
// as "piieowrsej".
func mdLineOrder(count int, y func(int) float64, x func(int) float64, tolerance float64) []int {
	order := make([]int, count)
	for index := range order {
		order[index] = index
	}
	sort.SliceStable(order, func(i, j int) bool { return y(order[i]) > y(order[j]) })
	line := 0
	lines := make([]int, count)
	for position, index := range order {
		if position > 0 && y(order[position-1])-y(index) > tolerance {
			line++
		}
		lines[index] = line
	}
	sort.SliceStable(order, func(i, j int) bool {
		if lines[order[i]] != lines[order[j]] {
			return lines[order[i]] < lines[order[j]]
		}
		return x(order[i]) < x(order[j])
	})
	return order
}

// mdRunOverlapShare is the share of its own width by which a character must be buried in the one
// preceding it before the two are read as belonging to separate runs. Kerning overlaps
// neighbouring glyph boxes slightly; a character drawn over another one is buried by most of
// itself.
const mdRunOverlapShare = 0.5

// mdSplitIntoRuns groups the marks of one line, given left to right, into the independent runs of
// text drawn on it. A cell that carries two runs stacked at the same position - a revision drawn
// over the text it replaces, or a duplicated layer - otherwise reads as the two interleaved
// character by character, turning "czynnosci" over "watroby" into "cwzaytnronboys". Within a run
// each character starts where the previous one ended, so a character that starts before the end of
// the run's last one belongs to a different run. Marks are placed first-fit, which keeps a run
// together for as long as it advances.
func mdSplitIntoRuns(marks []mdCellMark, order []int) []int {
	ends := []float64{}
	runs := make([][]int, 0, 2)
	for _, index := range order {
		placed := false
		buried := (marks[index].x1 - marks[index].x0) * mdRunOverlapShare
		for run := range runs {
			if marks[index].x0 >= ends[run]-buried {
				runs[run] = append(runs[run], index)
				ends[run] = marks[index].x1
				placed = true
				break
			}
		}
		if !placed {
			runs = append(runs, []int{index})
			ends = append(ends, marks[index].x1)
		}
	}
	if len(runs) < 2 {
		return order
	}
	split := make([]int, 0, len(order))
	for _, run := range runs {
		split = append(split, run...)
	}
	return split
}

func mdSortIntoLines(marks []mdCellMark) {
	order := mdLineOrder(len(marks),
		func(index int) float64 { return marks[index].y },
		func(index int) float64 { return marks[index].x0 },
		mdCellLineTolerance)
	ordered := make([]int, 0, len(order))
	for start := 0; start < len(order); {
		end := start + 1
		for end < len(order) && mdAbs(marks[order[end]].y-marks[order[start]].y) <= mdCellLineTolerance {
			end++
		}
		ordered = append(ordered, mdSplitIntoRuns(marks, order[start:end])...)
		start = end
	}
	sorted := make([]mdCellMark, len(marks))
	for index, position := range ordered {
		sorted[index] = marks[position]
	}
	copy(marks, sorted)
}

func mdJoinCell(marks []mdCellMark) string {
	mdSortIntoLines(marks)
	var lines []string
	var cur strings.Builder
	var lastY, lastX1 float64
	first := true
	for _, m := range marks {
		if !first && mdAbs(m.y-lastY) > 4 {
			lines = append(lines, strings.TrimSpace(cur.String()))
			cur.Reset()
		} else if !first && (m.x0-lastX1 > 1.2 || strings.HasPrefix(m.s, " ")) {
			cur.WriteString(" ")
		}
		cur.WriteString(strings.TrimLeft(m.s, " "))
		lastY = m.y
		lastX1 = m.x1
		first = false
	}
	if cur.Len() > 0 {
		lines = append(lines, strings.TrimSpace(cur.String()))
	}
	for index, line := range lines {
		for _, marker := range mdBulletMarkers {
			if strings.HasPrefix(line, marker) {
				lines[index] = "• " + strings.TrimSpace(line[len(marker):])
				break
			}
		}
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
		characters := []rune(line)
		if len(characters) == 0 || (len(characters) == 1 && unicode.IsLetter(characters[0])) {
			single++
		}
	}
	return single*2 >= len(lines)
}

type mdWord struct {
	x0, x1, baseline, y float64
	s                   string
	style               int
}

const (
	boldStyle = 1 << iota
	italicStyle
)

func mdFontStyle(font *model.PdfFont) int {
	if font == nil {
		return 0
	}
	name := strings.ToLower(font.BaseFont())
	style := 0
	if strings.Contains(name, "bold") || strings.Contains(name, "black") || strings.Contains(name, "heavy") {
		style |= boldStyle
	}
	if strings.Contains(name, "italic") || strings.Contains(name, "oblique") {
		style |= italicStyle
	}
	return style
}

func mdLineStyle(line []mdWord) int {
	style := line[0].style
	for _, word := range line[1:] {
		style &= word.style
	}
	return style
}

func mdReconstructBlocks(marks []TextMark, strokes []Stroke, right float64) []mdBlock {
	lines, lineYs := mdLines(marks)
	if len(lines) == 0 {
		return nil
	}
	return mdLineBlocks(lines, lineYs, strokes, right)
}

func mdLines(marks []TextMark) ([][]mdWord, []float64) {
	type lineMark struct {
		x0, x1, baseline, y float64
		s                   string
		style               int
	}
	var lms []lineMark
	for _, m := range marks {
		if strings.TrimSpace(m.Text) == "" {
			continue
		}
		lms = append(lms, lineMark{m.BBox.Llx, m.BBox.Urx, m.BBox.Lly,
			(m.BBox.Lly + m.BBox.Ury) / 2, m.Text, mdFontStyle(m.Font)})
	}
	if len(lms) == 0 {
		return nil, nil
	}
	order := mdLineOrder(len(lms),
		func(index int) float64 { return lms[index].y },
		func(index int) float64 { return lms[index].x0 },
		3)
	ordered := make([]lineMark, len(lms))
	for index, position := range order {
		ordered[index] = lms[position]
	}
	lms = ordered

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
		} else if !first && (m.x0-lastX1 > 1.2 || strings.HasPrefix(m.s, " ")) {
			flushWord()
		}
		if !haveW {
			curW = mdWord{x0: m.x0, x1: m.x1, baseline: m.baseline, y: m.y, s: strings.TrimLeft(m.s, " "), style: m.style}
			haveW = true
		} else {
			curW.x1 = m.x1
			curW.s += m.s
			curW.style &= m.style
		}
		lastY = m.y
		lastX1 = m.x1
		first = false
	}
	flushLine()
	for index := len(lines) - 2; index >= 0; index-- {
		label, next := lines[index], lines[index+1]
		labelled := append([]mdWord{label[0]}, next...)
		drop := lineYs[index] - lineYs[index+1]
		if len(label) == 1 && mdLegendLabelRegexp.MatchString(label[0].s) && drop > 0 && drop < hangingLabelDrop && mdLabelGap(labelled) {
			lines[index+1] = labelled
			lines = append(lines[:index], lines[index+1:]...)
			lineYs = append(lineYs[:index], lineYs[index+1:]...)
		}
	}
	return mdDropMarginJunk(lines, lineYs)
}

const (
	marginJunkGap        = 15.0
	marginJunkWordLength = 3
	marginSampleWords    = 3
	minimumJunkLines     = 3
	alignedLabelSlack    = 1.0
	minimumLabelX        = 45.0
)

func mdDropMarginJunk(lines [][]mdWord, lineYs []float64) ([][]mdWord, []float64) {
	starts := map[int]int{}
	for _, line := range lines {
		if len(line) < marginSampleWords {
			continue
		}
	segments:
		for _, segment := range mdLineSegments(line) {
			for _, word := range segment {
				if len([]rune(word.s)) > marginJunkWordLength {
					starts[int(segment[0].x0/2)]++
					break segments
				}
			}
		}
	}
	best, margin := 0, 0.0
	for start, count := range starts {
		if count > best || count == best && float64(start)*2 < margin {
			best, margin = count, float64(start)*2
		}
	}
	if best == 0 {
		return lines, lineYs
	}
	candidates := 0
	for _, line := range lines {
		if segment := mdLineSegments(line)[0]; segment[0].x0 < minimumLabelX && mdJunkSegment(segment, margin) {
			candidates++
		}
	}
	if candidates < minimumJunkLines {
		return lines, lineYs
	}
	var keptLines [][]mdWord
	var keptYs []float64
	for index, line := range lines {
		segments := mdLineSegments(line)
		kept := line
		for len(segments) > 0 && mdJunkSegment(segments[0], margin) {
			kept = kept[len(segments[0]):]
			segments = segments[1:]
		}
		if len(kept) > 0 {
			keptLines = append(keptLines, kept)
			keptYs = append(keptYs, lineYs[index])
		}
	}
	return keptLines, keptYs
}

func mdJunkSegment(segment []mdWord, margin float64) bool {
	if segment[len(segment)-1].x1 > margin-marginJunkGap {
		return false
	}
	for _, word := range segment {
		marker := len(segment) == 1 && (mdListMarkerRegexp.MatchString(word.s) || mdIsBulletMarker(word.s))
		if len([]rune(word.s)) > marginJunkWordLength || marker {
			return false
		}
	}
	return true
}

func mdLineBlocks(lines [][]mdWord, lineYs []float64, strokes []Stroke, right float64) []mdBlock {
	var blocks []mdBlock
	prose := 0
	for from := 0; from < len(lines); {
		start, end, table := mdNextBorderlessTable(lines, lineYs, from, strokes)
		if ruledStart, ruledEnd, ruled := mdNextRuledTable(lines, lineYs, from, strokes); ruled != nil && (table == nil || ruledEnd <= start || (ruledStart <= start && ruledEnd >= end)) {
			start, end, table = ruledStart, ruledEnd, ruled
		}
		fractionStart, fractionEnd, formula := mdNextFraction(lines, lineYs, from, strokes)
		if formula != "" && (table == nil || fractionStart < end) {
			if text := mdRenderProse(lines[prose:fractionStart], lineYs[prose:fractionStart], strokes, right); text != "" {
				blocks = append(blocks, mdProseBlock(text, lines[prose:fractionStart], right))
			}
			blocks = append(blocks, mdProseBlock(formula, lines[fractionStart:fractionEnd], right))
			prose, from = fractionEnd, fractionEnd
			continue
		}
		if table == nil {
			break
		}
		if text := mdRenderProse(lines[prose:start], lineYs[prose:start], strokes, right); text != "" {
			blocks = append(blocks, mdProseBlock(text, lines[prose:start], right))
		}
		blocks = append(blocks, mdBlock{table: table})
		prose, from = end, end
	}
	if text := mdRenderProse(lines[prose:], lineYs[prose:], strokes, right); text != "" {
		blocks = append(blocks, mdProseBlock(text, lines[prose:], right))
	}
	return blocks
}

const (
	lineEndSlack       = 6.0
	orphanLength       = 4
	abbreviationLength = 5
	shortLineShare     = 0.5
	itemIndentSlack    = 3.0
)

const (
	separatorShare = 0.8
	underlineDepth = 3.5
)

func mdRuleBetween(strokes []Stroke, above, below []mdWord) bool {
	left := mdMin(above[0].x0, below[0].x0)
	right := mdMax(above[len(above)-1].x1, below[len(below)-1].x1)
	bottom := above[0].baseline - underlineDepth
	top := 2*below[0].y - below[0].baseline
	for _, stroke := range strokes {
		y := (stroke.Y1 + stroke.Y2) / 2
		low, high := mdMinMax(stroke.X1, stroke.X2)
		if stroke.IsHorizontal() && y < bottom && y > top && mdMin(high, right)-mdMax(low, left) > separatorShare*(right-left) {
			return true
		}
	}
	return false
}

const (
	minimumParagraphGap = 18.0
	minimumLinePitch    = 6.0
	paragraphGapRatio   = 1.25
)

func mdParagraphGap(lineYs []float64) float64 {
	pitch := 0.0
	for index := 1; index < len(lineYs); index++ {
		if current := lineYs[index-1] - lineYs[index]; current > minimumLinePitch && (pitch == 0 || current < pitch) {
			pitch = current
		}
	}
	return mdMax(minimumParagraphGap, paragraphGapRatio*pitch)
}

func mdLeadWidth(line []mdWord, bound bool) float64 {
	first := []rune(line[0].s)
	abbreviation := len(first) <= abbreviationLength && first[len(first)-1] == '.'
	if bound && len(line) > 1 && (len(first) <= orphanLength || abbreviation || unicode.IsDigit(first[0])) {
		return line[1].x1 - line[0].x0
	}
	return line[0].x1 - line[0].x0
}

func mdProseBlock(text string, lines [][]mdWord, right float64) mdBlock {
	last := lines[len(lines)-1]
	return mdBlock{text: text, tail: right - last[len(last)-1].x1, lead: mdLeadWidth(lines[0], true)}
}

const hangingLabelDrop = 6.0

func mdRenderProse(lines [][]mdWord, lineYs []float64, strokes []Stroke, right float64) string {
	rendered := make([]string, len(lines))
	for i, lineWords := range lines {
		rendered[i] = mdRenderLine(lineWords, strokes)
	}

	paragraphGap := mdParagraphGap(lineYs)
	var b strings.Builder
	itemIndent := 0.0
	var levels [][2]float64
	labels := 0
	for _, line := range lines {
		if mdLetterLabel(line) && line[0].s != "o" {
			labels++
		}
	}
	legend := labels >= minimumLegendLabels
	for i, line := range rendered {
		content, bullet := mdBulletContent(line)
		if !bullet && !legend && strings.HasPrefix(line, "o ") && mdLetterLabel(lines[i]) {
			content, bullet = strings.TrimSpace(line[1:]), true
		}
		label := legend && mdLetterLabel(lines[i])
		if strings.HasPrefix(line, "* ") && !(i > 0 && strings.HasPrefix(rendered[i-1], "* ")) && !(i+1 < len(rendered) && strings.HasPrefix(rendered[i+1], "* ")) {
			content, bullet = line, false
		}
		if i > 0 {
			// A line that starts with a section number (e.g. "6.3. Okres ważności")
			// is a heading: keep it on its own line and don't fold the following
			// content into it, even when it doesn't end with a period.
			previous, next := lines[i-1][len(lines[i-1])-1], lines[i][0]
			terminal := strings.ContainsAny(previous.s[len(previous.s)-1:], ".:;!?")
			nextWidth := mdLeadWidth(lines[i], !terminal)
			opening := []rune(next.s)[0]
			short := right-previous.x1 > shortLineShare*(right-lines[i-1][0].x0)
			fits := right-previous.x1 > nextWidth+lineEndSlack
			gap := lineYs[i-1]-lineYs[i] > paragraphGap
			numberRange := unicode.IsDigit([]rune(previous.s)[len([]rune(previous.s))-1]) && strings.ContainsRune("-–−", []rune(line)[0]) && content != "" && unicode.IsDigit([]rune(content)[0])
			if bullet && strings.ContainsRune("-–−", []rune(line)[0]) && !terminal && (!fits || numberRange) && !gap && itemIndent == 0 {
				content, bullet = line, false
			}
			continues := (unicode.IsLower(opening) || opening == '(') && !terminal && !short || numberRange
			ended := !continues && !strings.HasSuffix(previous.s, lineEndHyphen) && fits
			listItem := mdNumberedItemRegexp.MatchString(rendered[i-1]) && continues
			heading := mdHeadingRegexp.MatchString(line) || (mdHeadingRegexp.MatchString(rendered[i-1]) && !listItem && !(continues && !fits && !mdInlineSectionRegexp.MatchString(rendered[i-1])) && !strings.HasSuffix(previous.s, lineEndHyphen))
			underlinedLine := strings.HasPrefix(rendered[i-1], "<u>") && strings.HasSuffix(rendered[i-1], "</u>") && strings.Count(rendered[i-1], "<u>") == 1 && !strings.HasPrefix(line, "<u>")
			underlinedNext := terminal && !strings.HasPrefix(rendered[i-1], "<u>") && strings.HasPrefix(line, "<u>") && strings.HasSuffix(line, "</u>") && strings.Count(line, "<u>") == 1
			outdented := itemIndent > 0 && next.x0 < itemIndent-itemIndentSlack && unicode.IsUpper(opening) && terminal
			inItem := itemIndent > 0 && next.x0 >= itemIndent-itemIndentSlack
			previousStyle, nextStyle := mdLineStyle(lines[i-1]), mdLineStyle(lines[i])
			styleChange := (previousStyle != 0 && nextStyle != previousStyle && unicode.IsUpper(opening)) || (terminal && previousStyle == 0 && nextStyle != 0)
			switch {
			case bullet:
				b.WriteString("\n")
			case label:
				b.WriteString("\n\n")
				levels = nil
			case heading || underlinedLine || underlinedNext || styleChange || outdented || gap || mdFrequencyLabelRegexp.MatchString(line) || mdRuleBetween(strokes, lines[i-1], lines[i]) || mdTabularPair(lines[i-1], lines[i]) || (terminal && i == len(lines)-1 && strings.HasSuffix(line, ":")):
				b.WriteString("\n\n")
				itemIndent, levels = 0, nil
			case inItem && ended && !terminal && len(lines[i]) > 1 && right-previous.x1 > lines[i][1].x1-next.x0+lineEndSlack && mdItemTitle(rendered[i-1], next.s):
				b.WriteString("\n\n" + strings.Repeat("  ", len(levels)))
			case inItem && !(ended && terminal):
				b.WriteString(" ")
			case ended && !mdSectionNumberRegexp.MatchString(rendered[i-1]) && !mdFootnoteMarkerRegexp.MatchString(rendered[i-1]):
				b.WriteString("\n\n")
				itemIndent, levels = 0, nil
			default:
				b.WriteString(" ")
			}
		}
		if bullet {
			level := [2]float64{lines[i][0].x0, lines[i][0].x1}
			if len(lines[i]) > 1 {
				level[1] = lines[i][1].x0
			}
			for len(levels) > 0 && level[1] < levels[len(levels)-1][1]-itemIndentSlack {
				levels = levels[:len(levels)-1]
			}
			if len(levels) == 0 || (level[0] > levels[len(levels)-1][0]+itemIndentSlack && level[1] > levels[len(levels)-1][1]+itemIndentSlack) {
				levels = append(levels, level)
			}
			b.WriteString(strings.Repeat("  ", len(levels)-1) + "- " + content)
			itemIndent = 0
			if len(lines[i]) > 1 {
				itemIndent = lines[i][1].x0
			}
		} else {
			b.WriteString(line)
		}
	}
	return b.String()
}

const borderlessGutter = 8.0

const borderlessAlignment = 3.0

const borderlessMinimumRows = 3

const borderlessRowGap = 30.0

const borderlessSectionLines = 2

const borderlessMarkerLength = 3

var mdSectionNumberRegexp = regexp.MustCompile(`^\d{1,2}(\.\d{1,2})*\.$|^\d{1,2}(\.\d{1,2})+$`)

var mdListMarkerRegexp = regexp.MustCompile(`^(\d{1,2}(\.\d{1,2})*[.)]|\d{1,2}(\.\d{1,2})+|[a-zA-Z][.)]|[ivx]{1,4}[.)])$`)

func mdLeadsTable(line string) bool {
	characters := []rune(strings.TrimSpace(line))
	if len(characters) == 0 {
		return false
	}
	return (unicode.IsUpper(characters[0]) || unicode.IsDigit(characters[0])) && !strings.ContainsRune(".,;:", characters[len(characters)-1])
}

func mdLineSegments(line []mdWord) [][]mdWord {
	segments := [][]mdWord{{line[0]}}
	for _, word := range line[1:] {
		last := segments[len(segments)-1]
		if word.x0-last[len(last)-1].x1 >= borderlessGutter {
			segments = append(segments, []mdWord{word})
		} else {
			segments[len(segments)-1] = append(last, word)
		}
	}
	return segments
}

const tabularPairGap = 24.0

func mdTabularPair(previous, next []mdWord) bool {
	var starts []float64
	for _, line := range [][]mdWord{previous, next} {
		segments := mdLineSegments(line)
		if len(segments) != 2 || segments[1][0].x0-segments[0][len(segments[0])-1].x1 < tabularPairGap {
			return false
		}
		starts = append(starts, segments[1][0].x0)
	}
	return mdAbs(starts[0]-starts[1]) < borderlessAlignment
}

func mdSegmentCrosses(segment []mdWord, columns [][2]float64) bool {
	for _, column := range columns {
		if segment[0].x0 < column[0]-borderlessAlignment && segment[len(segment)-1].x1 > column[0]+borderlessAlignment {
			return true
		}
	}
	return false
}

func mdSegmentAligned(segment []mdWord, column [2]float64) bool {
	return segment[0].x0 >= column[0]-borderlessAlignment && segment[0].x0 <= column[1]+borderlessAlignment
}

func mdSegmentColumn(segment []mdWord, columns [][2]float64) int {
	column := 0
	for index, start := range columns {
		if segment[0].x0 >= start[0]-borderlessAlignment {
			column = index + 1
		}
	}
	return column
}

func mdNextBorderlessTable(lines [][]mdWord, lineYs []float64, from int, strokes []Stroke) (int, int, *mdLineTable) {
	segmented := make([][][]mdWord, len(lines))
	for index, line := range lines {
		segmented[index] = mdLineSegments(line)
	}
	for anchor := from; anchor < len(lines); anchor++ {
		columns, last := mdBorderlessColumns(segmented, lineYs, anchor)
		if columns == nil {
			continue
		}
		start := anchor
		if from == 0 && mdContinuesColumns(segmented[:start], columns) {
			start = 0
		}
		if start > from && len(segmented[start-1]) == 1 && lineYs[start-1]-lineYs[start] <= 1.5*(lineYs[start]-lineYs[start+1]) && mdLeadsTable(mdRenderLine(segmented[start-1][0], nil)) {
			start--
		}
		left, right := segmented[start][0][0].x0, 0.0
		for index := start; index <= last; index++ {
			left = mdMin(left, segmented[index][0][0].x0)
			for _, segment := range segmented[index] {
				right = mdMax(right, segment[len(segment)-1].x1)
			}
		}
		xs := []float64{left}
		for _, column := range columns {
			xs = append(xs, column[0])
		}
		table := &mdLineTable{xs: append(xs, right), ys: []float64{lineYs[start], lineYs[last]}}
		var previousEnds []float64
		for index := start; index <= last; index++ {
			row := make([]string, len(columns)+1)
			ends := make([]float64, len(columns)+1)
			leads := make([]float64, len(columns)+1)
			for _, segment := range segmented[index] {
				column := mdSegmentColumn(segment, columns)
				if len(segmented[index]) == 1 && mdSegmentCrosses(segment, columns) {
					column = 0
				}
				if row[column] == "" {
					leads[column] = mdLeadWidth(segment, true)
				}
				row[column] = strings.TrimSpace(row[column] + " " + strings.ReplaceAll(mdRenderLine(segment, strokes), "|", "¦"))
				ends[column] = segment[len(segment)-1].x1
			}
			if label := mdFrequencyCellRegexp.FindStringSubmatch(row[0]); label != nil && strings.Join(row[1:], "") == "" {
				row[0], row[1], ends[1] = label[1], label[2], ends[0]
			}
			if len(table.cells) > 0 && table.wrapsInto(previousEnds, row, leads) {
				previous := table.cells[len(table.cells)-1]
				for column, text := range row {
					if text != "" {
						previous[column] += "<br>" + text
						previousEnds[column] = ends[column]
					}
				}
				continue
			}
			table.cells = append(table.cells, row)
			previousEnds = ends
		}
		return start, last + 1, table
	}
	return 0, 0, nil
}

const (
	ruledTableMinimumRule  = 120.0
	ruledTableRuleShare    = 0.8
	ruledTableDoubleRule   = 2.0
	ruledTableOverhang     = 6.0
	ruledTableRowTolerance = 0.5
	ruledTableJoinGap      = 1.5
)

func mdTableRules(strokes []Stroke) []Stroke {
	return mdJoinedRules(strokes, ruledTableMinimumRule)
}

func mdJoinedRules(strokes []Stroke, minimum float64) []Stroke {
	var segments []Stroke
	for _, stroke := range strokes {
		low, high := mdMinMax(stroke.X1, stroke.X2)
		if stroke.IsHorizontal() {
			segments = append(segments, Stroke{X1: low, Y1: (stroke.Y1 + stroke.Y2) / 2, X2: high, Y2: (stroke.Y1 + stroke.Y2) / 2})
		}
	}
	sort.Slice(segments, func(first, second int) bool {
		if mdAbs(segments[first].Y1-segments[second].Y1) >= ruledTableRowTolerance {
			return segments[first].Y1 > segments[second].Y1
		}
		return segments[first].X1 < segments[second].X1
	})
	var joined []Stroke
	for _, segment := range segments {
		if last := len(joined) - 1; last >= 0 && mdAbs(joined[last].Y1-segment.Y1) < ruledTableRowTolerance && segment.X1 <= joined[last].X2+ruledTableJoinGap {
			joined[last].X2 = mdMax(joined[last].X2, segment.X2)
			continue
		}
		joined = append(joined, segment)
	}
	var rules []Stroke
	for _, rule := range joined {
		if rule.X2-rule.X1 < minimum {
			continue
		}
		if last := len(rules) - 1; last >= 0 && rules[last].Y1-rule.Y1 < ruledTableDoubleRule && mdSameRuleExtent(rules[last], rule) {
			continue
		}
		rules = append(rules, rule)
	}
	return rules
}

func mdSameRuleExtent(first, second Stroke) bool {
	overlap := mdMin(first.X2, second.X2) - mdMax(first.X1, second.X1)
	return overlap >= ruledTableRuleShare*mdMax(first.X2-first.X1, second.X2-second.X1)
}

func mdColumnBoundaries(lines [][]mdWord) []float64 {
	var intervals [][2]float64
	for _, line := range lines {
		for _, word := range line {
			intervals = append(intervals, [2]float64{word.x0, word.x1})
		}
	}
	sort.Slice(intervals, func(first, second int) bool { return intervals[first][0] < intervals[second][0] })
	var boundaries []float64
	reach := 0.0
	for index, interval := range intervals {
		if index > 0 && interval[0]-reach >= borderlessGutter {
			boundaries = append(boundaries, (reach+interval[0])/2)
		}
		reach = mdMax(reach, interval[1])
	}
	return boundaries
}

func mdRuleBand(lineYs []float64, upper, lower Stroke) (int, int) {
	start := 0
	for start < len(lineYs) && lineYs[start] >= upper.Y1 {
		start++
	}
	end := start
	for end < len(lineYs) && lineYs[end] > lower.Y1 {
		end++
	}
	return start, end
}

func mdTabularLines(lines [][]mdWord, rule Stroke) bool {
	if len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		if line[0].x0 < rule.X1-ruledTableOverhang || line[len(line)-1].x1 > rule.X2+ruledTableOverhang || mdIsBulletMarker(line[0].s) {
			return false
		}
	}
	return len(mdSupportedBoundaries(lines, mdColumnBoundaries(mdMultiSegmentLines(lines)))) > 0 || len(mdColumnBoundaries(lines)) > 0
}

func mdMultiSegmentLines(lines [][]mdWord) [][]mdWord {
	var multiple [][]mdWord
	for _, line := range lines {
		if len(mdLineSegments(line)) > 1 {
			multiple = append(multiple, line)
		}
	}
	return multiple
}

func mdNextRuledTable(lines [][]mdWord, lineYs []float64, from int, strokes []Stroke) (int, int, *mdLineTable) {
	rules := mdTableRules(strokes)
	for first := 0; first+1 < len(rules); first++ {
		last := first
		for last+1 < len(rules) && mdSameRuleExtent(rules[first], rules[last+1]) {
			start, end := mdRuleBand(lineYs, rules[last], rules[last+1])
			if start < from || !mdTabularLines(lines[start:end], rules[first]) {
				break
			}
			last++
		}
		if last == first {
			continue
		}
		if start, end, table := mdRuledTable(lines, lineYs, from, rules[first:last+1], strokes); table != nil {
			return start, end, table
		}
	}
	return 0, 0, nil
}

func mdRuledTable(lines [][]mdWord, lineYs []float64, from int, rules []Stroke, strokes []Stroke) (int, int, *mdLineTable) {
	start, headerEnd := mdRuleBand(lineYs, rules[0], rules[1])
	if len(rules) == 2 {
		headerEnd = start
	}
	_, end := mdRuleBand(lineYs, rules[len(rules)-2], rules[len(rules)-1])
	boundaries := mdSupportedBoundaries(lines[start:end], mdColumnBoundaries(lines[headerEnd:end]))
	if len(boundaries) == 0 {
		boundaries = mdSupportedBoundaries(lines[start:end], mdColumnBoundaries(lines[start:end]))
	}
	if len(boundaries) == 0 {
		boundaries = mdSupportedBoundaries(lines[start:end], mdColumnBoundaries(mdMultiSegmentLines(lines[start:end])))
	}
	if len(boundaries) == 0 || end-start < minimumRuledTableLines {
		return 0, 0, nil
	}
	for start > from && lineYs[start-1]-lineYs[start] <= fractionLineStep && mdHeaderContinuation(lines[start-1], boundaries, rules[0]) {
		start--
	}
	table := &mdLineTable{xs: append(append([]float64{rules[0].X1}, boundaries...), rules[0].X2), ys: []float64{rules[0].Y1, rules[len(rules)-1].Y1}}
	band := 1
	for index := start; index < end; index++ {
		bandStart := false
		for band+1 < len(rules) && lineYs[index] < rules[band].Y1 {
			band++
			bandStart = true
		}
		row := make([]string, len(boundaries)+1)
		columns := make([][]mdWord, len(boundaries)+1)
		for _, word := range lines[index] {
			column := sort.SearchFloat64s(boundaries, (word.x0+word.x1)/2)
			columns[column] = append(columns[column], word)
		}
		for column, words := range columns {
			if len(words) > 0 {
				row[column] = strings.ReplaceAll(mdRenderLine(words, strokes), "|", "¦")
			}
		}
		if len(table.cells) > 0 && !bandStart && (mdContinuesRow(table.cells[len(table.cells)-1], row) || band == 1 && len(rules) > 2 && table.cells[len(table.cells)-1][0] == "") {
			table.appendToLastRow(row)
			continue
		}
		table.cells = append(table.cells, row)
	}
	if len(table.cells) < minimumRuledTableRows || table.markerColumn() {
		return 0, 0, nil
	}
	return start, end, table
}

var mdOpenEndingRegexp = regexp.MustCompile(`(,|\s(lub|i|oraz|albo|ani|a|do|z|ze|w|we|na|od|o|po|przez|niż|wobec|dla|przy|u|że|jak))$`)

var mdSentenceStartRegexp = regexp.MustCompile(`^\p{Lu}(\p{Ll}|$)`)

func mdItemTitle(title, following string) bool {
	return mdSentenceStartRegexp.MatchString(following) && !mdOpenEndingRegexp.MatchString(title) && strings.Count(title, "(") <= strings.Count(title, ")")
}

func mdContinuesRow(previous, row []string) bool {
	if row[0] == "" {
		return true
	}
	for column, text := range row {
		if text == "" {
			continue
		}
		opening := []rune(text)[0]
		open := previous[column] != "" && !strings.ContainsAny(previous[column][len(previous[column])-1:], ".;:")
		if !(column == 0 && (unicode.IsLower(opening) || opening == '(' || mdOpenEndingRegexp.MatchString(previous[0])) || column > 0 && (unicode.IsLower(opening) || opening == '(') && open) {
			return false
		}
	}
	return true
}

func (t *mdLineTable) markerColumn() bool {
	for _, row := range t.cells {
		for _, label := range strings.Split(row[0], "<br>") {
			if label != "" && !mdFootnoteMarkerRegexp.MatchString(label) && !mdSectionNumberRegexp.MatchString(label) {
				return false
			}
		}
	}
	return true
}

const (
	minimumRuledTableLines = 2
	minimumRuledTableRows  = 2
	minimumBoundarySupport = 2
	boundaryCrossingShare  = 3
)

func mdHeaderContinuation(line []mdWord, boundaries []float64, rule Stroke) bool {
	for _, word := range line {
		if word.x0 < boundaries[0] || word.x1 > rule.X2+ruledTableOverhang {
			return false
		}
		for _, boundary := range boundaries {
			if word.x0 < boundary && word.x1 > boundary {
				return false
			}
		}
	}
	return true
}

func mdSupportedBoundaries(lines [][]mdWord, boundaries []float64) []float64 {
	var supported []float64
	for _, boundary := range boundaries {
		support, crossing := 0, 0
		for _, line := range lines {
			left, right, crosses := false, false, false
			for _, word := range line {
				left = left || word.x1 < boundary
				right = right || word.x0 > boundary
				crosses = crosses || word.x0 < boundary && word.x1 > boundary
			}
			if crosses {
				crossing++
			} else if left && right {
				support++
			}
		}
		if support >= minimumBoundarySupport && crossing*boundaryCrossingShare <= support {
			supported = append(supported, boundary)
		}
	}
	return supported
}

const (
	fractionMinimumBar = 30.0
	fractionLineStep   = 16.0
	fractionBarSlack   = 3.0
)

var mdOperatorRegexp = regexp.MustCompile(`^(=|[xX×+−-](\s|$))`)

func mdNextFraction(lines [][]mdWord, lineYs []float64, from int, strokes []Stroke) (int, int, string) {
	for _, bar := range mdJoinedRules(strokes, fractionMinimumBar) {
		split := from
		for split < len(lines) && lineYs[split] > bar.Y1 {
			split++
		}
		start, previous := split, bar.Y1
		for start > from && mdFractionLine(lines[start-1], bar) && lineYs[start-1]-previous <= fractionLineStep {
			start--
			previous = lineYs[start]
		}
		end, previous := split, bar.Y1
		for end < len(lines) && mdFractionLine(lines[end], bar) && previous-lineYs[end] <= fractionLineStep {
			previous = lineYs[end]
			end++
		}
		if text := mdFractionText(lines[start:end], lineYs[start:end], bar); text != "" {
			return start, end, text
		}
	}
	return 0, 0, ""
}

func mdFractionLine(line []mdWord, bar Stroke) bool {
	for _, word := range line {
		inside := word.x0 >= bar.X1-fractionBarSlack && word.x1 <= bar.X2+fractionBarSlack
		if !inside && word.x1 >= bar.X1 && word.x0 <= bar.X2 {
			return false
		}
	}
	return true
}

func mdFractionText(lines [][]mdWord, lineYs []float64, bar Stroke) string {
	var left, numerator, denominator, right [][]mdWord
	for index, line := range lines {
		var leftWords, insideWords, rightWords []mdWord
		for _, word := range line {
			switch {
			case word.x1 < bar.X1:
				leftWords = append(leftWords, word)
			case word.x0 > bar.X2:
				rightWords = append(rightWords, word)
			default:
				insideWords = append(insideWords, word)
			}
		}
		left, right = append(left, leftWords), append(right, rightWords)
		if lineYs[index] > bar.Y1 {
			numerator = append(numerator, insideWords)
		} else {
			denominator = append(denominator, insideWords)
		}
	}
	return mdFormulaText(mdColumnText(left), mdColumnText(numerator), mdColumnText(denominator), mdColumnText(right))
}

func mdFormulaText(left, numerator, denominator, right string) string {
	if numerator == "" || denominator == "" || left == "" && right == "" {
		return ""
	}
	if left != "" && !strings.HasSuffix(left, "=") || right != "" && !mdOperatorRegexp.MatchString(right) {
		return ""
	}
	if !strings.Contains(left+right, "=") {
		return ""
	}
	return strings.TrimSpace(left + " (" + numerator + ") / " + mdDenominator(denominator) + " " + right)
}

func mdDenominator(text string) string {
	if strings.Contains(text, " ") {
		return "(" + text + ")"
	}
	return text
}

func mdColumnText(lines [][]mdWord) string {
	boundaries := mdColumnBoundaries(lines)
	columns := make([][]string, len(boundaries)+1)
	for _, line := range lines {
		for _, word := range line {
			text := word.s
			if mdMultiplicationCells[text] {
				text = "×"
			}
			column := sort.SearchFloat64s(boundaries, (word.x0+word.x1)/2)
			columns[column] = append(columns[column], text)
		}
	}
	var parts []string
	for _, column := range columns {
		if len(column) > 0 {
			parts = append(parts, strings.Join(column, " "))
		}
	}
	return strings.Join(parts, " ")
}

func mdContinuesColumns(segmented [][][]mdWord, columns [][2]float64) bool {
	if len(segmented) == 0 {
		return false
	}
	for _, segments := range segmented {
		if len(segments) != 1 || segments[0][0].x0 < columns[0][0]-borderlessAlignment {
			return false
		}
		aligned := false
		for _, column := range columns {
			aligned = aligned || mdSegmentAligned(segments[0], column)
		}
		if !aligned {
			return false
		}
	}
	return true
}

func mdBorderlessColumns(segmented [][][]mdWord, lineYs []float64, anchor int) ([][2]float64, int) {
	if len(segmented[anchor]) < 2 {
		return nil, 0
	}
	var starts []float64
	for index := anchor; index < len(segmented) && (index == anchor || lineYs[index-1]-lineYs[index] <= borderlessRowGap); index++ {
		for _, segment := range segmented[index][1:] {
			starts = append(starts, segment[0].x0)
		}
	}
	sort.Float64s(starts)
	var candidates [][2]float64
	for _, start := range starts {
		if last := len(candidates) - 1; last >= 0 && start-candidates[last][1] < borderlessGutter {
			candidates[last][1] = start
			continue
		}
		candidates = append(candidates, [2]float64{start, start})
	}
	var columns [][2]float64
	for _, candidate := range candidates {
		aligned := 0
		for index := anchor; index < len(segmented) && (index == anchor || lineYs[index-1]-lineYs[index] <= borderlessRowGap); index++ {
			for _, segment := range segmented[index][1:] {
				if mdSegmentAligned(segment, candidate) {
					aligned++
					break
				}
			}
		}
		if aligned >= borderlessMinimumRows {
			columns = append(columns, candidate)
		}
	}
	if len(columns) == 0 {
		return nil, 0
	}
	last, rows, sections, markers := -1, 0, 0, 0
	for index := anchor; index < len(segmented); index++ {
		if index > anchor && lineYs[index-1]-lineYs[index] > borderlessRowGap {
			break
		}
		segments := segmented[index]
		if len(segments) == 1 {
			if mdSegmentCrosses(segments[0], columns) || mdSegmentColumn(segments[0], columns) == 0 {
				sections++
				if sections > borderlessSectionLines {
					break
				}
				continue
			}
			if !mdSegmentAligned(segments[0], columns[mdSegmentColumn(segments[0], columns)-1]) {
				break
			}
			sections = 0
			last = index
			continue
		}
		if mdSectionNumberRegexp.MatchString(mdRenderLine(segments[0], nil)) {
			break
		}
		compatible := true
		for position, segment := range segments {
			aligned := position == 0
			for _, column := range columns {
				aligned = aligned || mdSegmentAligned(segment, column)
			}
			compatible = compatible && aligned && !mdSegmentCrosses(segment, columns)
		}
		if !compatible {
			break
		}
		if marker := mdRenderLine(segments[0], nil); mdListMarkerRegexp.MatchString(marker) || mdIsBulletMarker(marker) || len([]rune(marker)) <= borderlessMarkerLength {
			markers++
		}
		sections = 0
		rows++
		last = index
	}
	if rows < borderlessMinimumRows || markers*2 >= rows || markers == rows || last < anchor {
		return nil, 0
	}
	return columns, last
}

const (
	letterBulletGap      = 6.0
	letterBulletGapRatio = 2.0
)

func mdLabelGap(line []mdWord) bool {
	if len(line) < 2 {
		return false
	}
	gap := line[1].x0 - line[0].x1
	var spaces []float64
	for index := 2; index < len(line); index++ {
		spaces = append(spaces, line[index].x0-line[index-1].x1)
	}
	sort.Float64s(spaces)
	return gap >= letterBulletGap && (len(spaces) == 0 || gap >= letterBulletGapRatio*spaces[len(spaces)/2])
}

const minimumLegendLabels = 2

var mdLegendLabelRegexp = regexp.MustCompile(`^([a-z]|\*{1,3}|[†‡§#])$`)

func mdLetterLabel(line []mdWord) bool {
	return mdLegendLabelRegexp.MatchString(line[0].s) && mdLabelGap(line)
}

// mdBulletContent reports whether a physical line begins with a bullet marker
// (•, a dash, or an asterisk followed by a space) and returns the text after
// the marker. A trailing-position dash (hyphenated word wrap) is not a bullet
// because the marker must be at the very start of the line.
var mdBulletMarkers = []string{"•", "♣", "▪", "■", "●", "◦", "○", "➢", "❖", "\uf0a7", "\uf0b7", "\uf06c", "\uf0d8", "\uf076", "\uf0a8", "\uf0e8"}

func mdIsBulletMarker(text string) bool {
	for _, marker := range append(mdBulletMarkers, "-", "–", "−", "*") {
		if text == marker {
			return true
		}
	}
	return false
}

func mdBulletContent(line string) (string, bool) {
	for _, marker := range mdBulletMarkers {
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(line[len(marker):]), true
		}
	}
	for _, marker := range []string{"- ", "– ", "− ", "* "} {
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(line[len(marker):]), true
		}
	}
	return line, false
}

const (
	underlineOverrun = 6.0
	ruleCorner       = 1.5
	ruleMinimum      = 3.0
)

func mdMeetsRule(horizontal Stroke, strokes []Stroke) bool {
	low, high := mdMinMax(horizontal.X1, horizontal.X2)
	y := (horizontal.Y1 + horizontal.Y2) / 2
	left, right := false, false
	for _, vertical := range strokes {
		bottom, top := mdMinMax(vertical.Y1, vertical.Y2)
		x := (vertical.X1 + vertical.X2) / 2
		if !vertical.IsVertical() || top-bottom < ruleMinimum || y < bottom-ruleCorner || y > top+ruleCorner {
			continue
		}
		left = left || mdAbs(x-low) <= ruleCorner
		right = right || mdAbs(x-high) <= ruleCorner
	}
	return left && right
}

// mdRenderLine renders a single line, coalescing consecutive words that share
// the same styling into a single run (e.g. "Sposób podawania" -> one <u></u>).
func mdRenderLine(lineWords []mdWord, strokes []Stroke) string {
	var underlineStrokes []Stroke
	for _, stroke := range strokes {
		low, high := mdMinMax(stroke.X1, stroke.X2)
		if low >= lineWords[0].x0-underlineOverrun && high <= lineWords[len(lineWords)-1].x1+underlineOverrun && !mdMeetsRule(stroke, strokes) {
			underlineStrokes = append(underlineStrokes, stroke)
		}
	}
	underlines := make([]bool, len(lineWords))
	for index, word := range lineWords {
		underlines[index] = mdUnderlined(word, underlineStrokes)
		if word.s == "≥" || word.s == "≤" {
			underlines[index] = index > 0 && underlines[index-1]
		}
	}
	var b strings.Builder
	for j := 0; j < len(lineWords); {
		if j > 0 {
			b.WriteString(" ")
		}
		underlined := underlines[j]
		k := j + 1
		for k < len(lineWords) && underlines[k] == underlined {
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

func mdPickRowBorders(yCandidates []float64, hsegs []mdHSeg, xs []float64) []float64 {
	tableWidth := xs[len(xs)-1] - xs[0]
	var out []float64
	for _, y := range yCandidates {
		var intervals [][2]float64
		spansColumns := false
		for _, h := range hsegs {
			if mdAbs(h.y-y) <= 3 {
				intervals = append(intervals, [2]float64{h.x0, h.x1})
				spansColumns = spansColumns || (h.x1-h.x0 > minimumColumnRuleLength && mdNearAny(xs, h.x0, 3) && mdNearAny(xs, h.x1, 3))
			}
		}
		if spansColumns || mdUnionLen(intervals) > tableWidth*0.4 {
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

// mdCellTolerance is how far outside the ruled grid a mark may sit and still be taken as part of
// the cell at that edge.
const mdCellTolerance = 2

// mdFindCell returns the index of the cell of the `borders` grid that `v` falls in, or -1 when it
// falls outside the grid. The tolerance applies only at the two outer borders. Applying it at
// every border would make neighbouring cells overlap by twice its width, and the first match would
// then win, so a word starting just past a border loses its leading characters to the cell on the
// other side and "rzadko" is stored as "r" in one cell and "zadko" in the next.
func mdFindCell(borders []float64, v float64, descending bool) int {
	last := len(borders) - 1
	if last < 1 {
		return -1
	}
	if descending {
		if v > borders[0]+mdCellTolerance || v < borders[last]-mdCellTolerance {
			return -1
		}
		for i := 0; i < last-1; i++ {
			if v > borders[i+1] {
				return i
			}
		}
		return last - 1
	}
	if v < borders[0]-mdCellTolerance || v > borders[last]+mdCellTolerance {
		return -1
	}
	for i := 0; i < last-1; i++ {
		if v < borders[i+1] {
			return i
		}
	}
	return last - 1
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
