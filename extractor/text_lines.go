/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package extractor

import (
	"github.com/matisiekpl/unipdf/v3/internal/transform"
	"github.com/matisiekpl/unipdf/v3/model"
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
	current         transform.Point
	subStart        transform.Point
	strokes         []Stroke
	rects           []Rect
	cellRects       []Rect
	whiteCellRects  []Rect
	pendingSubpaths []pendingSubpath
	clipping        bool
}

type pendingSubpath struct {
	segments  []Stroke
	bounds    Rect
	rectangle bool
}

func (pb *pathBuilder) moveTo(ctm transform.Matrix, x, y float64) {
	px, py := ctm.Transform(x, y)
	pb.current = transform.Point{X: px, Y: py}
	pb.subStart = pb.current
	pb.startSubpath(pb.current, false)
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
	pb.startSubpath(p0, true)
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
	if len(pb.pendingSubpaths) == 0 {
		pb.startSubpath(a, false)
	}
	subpath := &pb.pendingSubpaths[len(pb.pendingSubpaths)-1]
	subpath.bounds = subpath.bounds.extended(a).extended(b)
	s := Stroke{X1: a.X, Y1: a.Y, X2: b.X, Y2: b.Y}
	if s.IsHorizontal() || s.IsVertical() {
		subpath.segments = append(subpath.segments, s)
	}
}

func (pb *pathBuilder) startSubpath(point transform.Point, rectangle bool) {
	pb.pendingSubpaths = append(pb.pendingSubpaths, pendingSubpath{
		bounds:    Rect{Llx: point.X, Lly: point.Y, Urx: point.X, Ury: point.Y},
		rectangle: rectangle,
	})
}

func (pb *pathBuilder) clip() {
	pb.clipping = true
}

func (pb *pathBuilder) endPath(stroke, fill, fillVisible bool) (clip Rect, clipping bool) {
	clipping = pb.clipping && len(pb.pendingSubpaths) > 0
	for index, subpath := range pb.pendingSubpaths {
		if index == 0 {
			clip = subpath.bounds
		}
		clip = clip.extended(transform.Point{X: subpath.bounds.Llx, Y: subpath.bounds.Lly}).extended(transform.Point{X: subpath.bounds.Urx, Y: subpath.bounds.Ury})
		shadedArea := fill && fillVisible && !subpath.bounds.isThin()
		if subpath.rectangle && (pb.clipping || shadedArea) {
			pb.cellRects = append(pb.cellRects, subpath.bounds)
		}
		if subpath.rectangle && fill && !fillVisible && !subpath.bounds.isThin() {
			pb.whiteCellRects = append(pb.whiteCellRects, subpath.bounds)
		}
		if stroke || (fill && subpath.bounds.isThin()) {
			pb.strokes = append(pb.strokes, subpath.segments...)
		}
	}
	pb.pendingSubpaths = nil
	pb.clipping = false
	return clip, clipping
}

const maximumRuleThickness = 2.5

const whiteThreshold = 0.99

func isWhite(color model.PdfColor) bool {
	switch typed := color.(type) {
	case *model.PdfColorDeviceGray:
		return float64(*typed) >= whiteThreshold
	case *model.PdfColorDeviceRGB:
		return typed[0] >= whiteThreshold && typed[1] >= whiteThreshold && typed[2] >= whiteThreshold
	case *model.PdfColorDeviceCMYK:
		return typed[0] <= 1-whiteThreshold && typed[1] <= 1-whiteThreshold && typed[2] <= 1-whiteThreshold && typed[3] <= 1-whiteThreshold
	}
	return false
}

func (r Rect) isThin() bool {
	return r.Urx-r.Llx <= maximumRuleThickness || r.Ury-r.Lly <= maximumRuleThickness
}

func (r Rect) extended(point transform.Point) Rect {
	if point.X < r.Llx {
		r.Llx = point.X
	}
	if point.X > r.Urx {
		r.Urx = point.X
	}
	if point.Y < r.Lly {
		r.Lly = point.Y
	}
	if point.Y > r.Ury {
		r.Ury = point.Y
	}
	return r
}
