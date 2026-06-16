/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package extractor

import (
	"github.com/matisiekpl/unipdf/v3/internal/transform"
)

// Stroke is a straight line segment drawn on a page in device coordinates.
// It is used to reconstruct tables that are drawn with ruling lines.
type Stroke struct {
	X1, Y1 float64
	X2, Y2 float64
}

// IsHorizontal returns true if the stroke is (approximately) horizontal.
func (s Stroke) IsHorizontal() bool {
	dy := s.Y2 - s.Y1
	if dy < 0 {
		dy = -dy
	}
	return dy <= lineTolerance
}

// IsVertical returns true if the stroke is (approximately) vertical.
func (s Stroke) IsVertical() bool {
	dx := s.X2 - s.X1
	if dx < 0 {
		dx = -dx
	}
	return dx <= lineTolerance
}

// lineTolerance is the maximum deviation (device units) for a stroke to be
// considered axis aligned.
const lineTolerance = 1.0

// Rect is an axis aligned rectangle drawn on a page in device coordinates.
type Rect struct {
	Llx, Lly, Urx, Ury float64
}

// pathBuilder accumulates path construction operators and converts them into
// axis aligned line segments in device coordinates.
type pathBuilder struct {
	current  transform.Point
	subStart transform.Point
	strokes  []Stroke
	rects    []Rect
}

func (pb *pathBuilder) moveTo(ctm transform.Matrix, x, y float64) {
	px, py := ctm.Transform(x, y)
	pb.current = transform.Point{X: px, Y: py}
	pb.subStart = pb.current
}

func (pb *pathBuilder) lineTo(ctm transform.Matrix, x, y float64) {
	px, py := ctm.Transform(x, y)
	next := transform.Point{X: px, Y: py}
	pb.addSegment(pb.current, next)
	pb.current = next
}

func (pb *pathBuilder) closePath() {
	pb.addSegment(pb.current, pb.subStart)
	pb.current = pb.subStart
}

func (pb *pathBuilder) rect(ctm transform.Matrix, x, y, w, h float64) {
	x0, y0 := ctm.Transform(x, y)
	x1, y1 := ctm.Transform(x+w, y)
	x2, y2 := ctm.Transform(x+w, y+h)
	x3, y3 := ctm.Transform(x, y+h)
	p0 := transform.Point{X: x0, Y: y0}
	p1 := transform.Point{X: x1, Y: y1}
	p2 := transform.Point{X: x2, Y: y2}
	p3 := transform.Point{X: x3, Y: y3}
	pb.addSegment(p0, p1)
	pb.addSegment(p1, p2)
	pb.addSegment(p2, p3)
	pb.addSegment(p3, p0)
	pb.current = p0
	pb.subStart = p0

	llx, urx := minMax(x0, x1, x2, x3)
	lly, ury := minMax(y0, y1, y2, y3)
	pb.rects = append(pb.rects, Rect{Llx: llx, Lly: lly, Urx: urx, Ury: ury})
}

func minMax(vals ...float64) (float64, float64) {
	mn, mx := vals[0], vals[0]
	for _, v := range vals[1:] {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	return mn, mx
}

func (pb *pathBuilder) addSegment(a, b transform.Point) {
	s := Stroke{X1: a.X, Y1: a.Y, X2: b.X, Y2: b.Y}
	if s.IsHorizontal() || s.IsVertical() {
		pb.strokes = append(pb.strokes, s)
	}
}
