//go:build ignore

// Tray icon rasteriser (S02 CONTRACTS §4). Renders assets/tray-icon.svg — the
// mockup's menu-bar glyph (demo.dc.html line 45: three descending rules plus a
// circle on a 16-unit viewBox, stroke-width 1.5, round caps) — to the
// black-on-transparent template PNGs the host embeds (assets.go).
//
// Why a hand-rolled rasteriser and not `rsvg-convert`: rsvg/inkscape/
// ImageMagick are all absent from this machine and are not guaranteed on a
// build box, the glyph is four primitives, and — crucially — a generic SVG
// rasteriser gives no control over pixel-grid alignment. The rasters this
// replaces were produced that way: every horizontal rule straddled two rows at
// ~20% alpha, which is most of why the menu-bar item read as a grey smudge.
//
// CANVAS SIZE follows what Wails actually does with the bytes on macOS:
//
//	systemtray_darwin.m:160  [image setSize:NSMakeSize(thickness, thickness)];
//
// where `thickness` is [[NSStatusBar systemStatusBar] thickness] == 22.0pt
// (measured on this machine). The NSImage is therefore ALWAYS resized to 22x22
// points whatever raster we hand it, so the raster must be a whole multiple of
// 22 — not the usual 18pt template size — or AppKit resamples it. The old
// 18x18 raster was being blown up 22/18 = 1.22x and then a further 2x for
// retina, i.e. every stroke smeared across ~2.4 device pixels.
//
//	1x = 22x22, 2x = 44x44, 3x = 66x66
//
// LAYOUT fits the glyph's ink bounding box (stroke included, so the round caps
// are counted) into the canvas with 2pt of padding on every side, preserving
// aspect. The SVG's viewBox padding is asymmetric — fine inside the mockup's
// pill, wrong for a menu-bar slot where AppKit centres the whole 22pt image —
// so we centre the ink, not the viewBox.
//
// GRID ALIGNMENT: the stroke is forced to an even number of device pixels
// (2/4/6) and every rule centre, cap end and circle centre is snapped to a
// whole pixel, so each rule's edges land exactly on pixel boundaries and it
// renders at full alpha with no half-covered rows. The three rules are spaced
// by one snapped gap so rounding cannot make them unevenly spaced. The circle
// keeps real anti-aliasing — a snapped circle looks faceted at 22px.
//
// Run from the desktop cmd directory (output dir defaults to ./assets):
//
//	go run assets/gen_tray_icon.go
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// Glyph geometry in the SVG's 16-unit viewBox (assets/tray-icon.svg).
const (
	ruleX0      = 2.2  // shared left end of all three rules
	rule1X1     = 13.8 // "M2.2 4.6h11.6"
	rule2X1     = 9.6  // "M2.2 8h7.4"
	rule3X1     = 6.4  // "M2.2 11.4h4.2"
	rule1Y      = 4.6
	rule2Y      = 8.0
	rule3Y      = 11.4
	circleCX    = 12.4
	circleR     = 2.1
	strokeUnits = 1.5
)

// Canvas geometry in points: 22pt is the macOS status bar thickness Wails
// resizes the image to; padPt keeps the glyph off the menu bar's edges.
const (
	canvasPt = 22.0
	padPt    = 2.0
)

// samplesPerAxis is the supersampling rate per pixel axis (16x16 = 256 samples
// per pixel), enough that the circle's curve shows no stair-stepping.
const samplesPerAxis = 16

// target is one output raster.
type target struct {
	scale int
	name  string
}

func main() {
	dir := "assets"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	for _, t := range []target{
		{1, "tray-iconTemplate.png"},
		{2, "tray-iconTemplate@2x.png"},
		{3, "tray-iconTemplate@3x.png"},
	} {
		img := render(t.scale)
		path := filepath.Join(dir, t.name)
		if err := write(path, img); err != nil {
			fmt.Fprintf(os.Stderr, "gen_tray_icon: %v\n", err)
			os.Exit(1)
		}
		st, _ := os.Stat(path)
		fmt.Printf("%-28s %dx%d  %d bytes\n", t.name, img.Bounds().Dx(), img.Bounds().Dy(), st.Size())
	}
}

// segment is a round-capped stroke from (x0,y0) to (x1,y1), all in device px.
type segment struct{ x0, y0, x1, y1 float64 }

// ring is a stroked circle centred (cx,cy) with radius r, in device px.
type ring struct{ cx, cy, r float64 }

// render draws the glyph at the given scale factor (1, 2 or 3) and returns an
// NRGBA image: black pixels whose alpha carries the coverage, which is all a
// macOS template image reads.
func render(scale int) *image.NRGBA {
	k := float64(scale)
	size := int(canvasPt) * scale
	avail := (canvasPt - 2*padPt) * k

	// Ink bounding box in viewBox units, stroke included. The circle is placed
	// on the third rule's baseline (the mockup has cy 11.2 vs the rule's 11.4 —
	// half a device pixel apart at 1x), so the box is measured against that.
	halfUnits := strokeUnits / 2
	inkMinX := ruleX0 - halfUnits
	inkMaxX := math.Max(rule1X1, circleCX+circleR) + halfUnits
	inkMinY := rule1Y - halfUnits
	inkMaxY := rule3Y + circleR + halfUnits

	unit := math.Min(avail/(inkMaxX-inkMinX), avail/(inkMaxY-inkMinY))
	xOff := padPt*k + (avail-(inkMaxX-inkMinX)*unit)/2 - inkMinX*unit
	yOff := padPt*k + (avail-(inkMaxY-inkMinY)*unit)/2 - inkMinY*unit

	// Even stroke width => a stroke centred on a whole pixel has both edges on
	// pixel boundaries, so no row is partially covered.
	half := k // stroke = 2*k device px

	snapX := func(v float64) float64 { return math.Round(xOff + v*unit) }

	// One snapped gap for both rule intervals: rounding each rule independently
	// can yield 5px/4px spacing at 1x, which reads as a wobble.
	gap := math.Round((rule2Y - rule1Y) * unit)
	y1 := math.Round(yOff + rule1Y*unit)
	y2, y3 := y1+gap, y1+2*gap

	x0 := snapX(ruleX0)
	x1a, x1b, x1c := snapX(rule1X1), snapX(rule2X1), snapX(rule3X1)
	cx, r := snapX(circleCX), math.Round(circleR*unit)

	// Re-centre the SNAPPED ink in the canvas: rounding each edge to the pixel
	// grid moves the artwork by up to half a pixel per axis, which is visible
	// as an off-centre glyph in a 22px slot.
	dx := math.Round((float64(size)-(math.Max(x1a+half, cx+r+half)-(x0-half)))/2 - (x0 - half))
	dy := math.Round((float64(size)-((y3+r+half)-(y1-half)))/2 - (y1 - half))
	x0, x1a, x1b, x1c, cx = x0+dx, x1a+dx, x1b+dx, x1c+dx, cx+dx
	y1, y2, y3 = y1+dy, y2+dy, y3+dy

	segments := []segment{
		{x0, y1, x1a, y1},
		{x0, y2, x1b, y2},
		{x0, y3, x1c, y3},
	}
	circle := ring{cx: cx, cy: y3, r: r}

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	step := 1.0 / float64(samplesPerAxis)
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			hits := 0
			for sy := 0; sy < samplesPerAxis; sy++ {
				y := float64(py) + (float64(sy)+0.5)*step
				for sx := 0; sx < samplesPerAxis; sx++ {
					x := float64(px) + (float64(sx)+0.5)*step
					if covered(x, y, segments, circle, half) {
						hits++
					}
				}
			}
			if hits == 0 {
				continue
			}
			a := uint8(math.Round(float64(hits) * 255 / float64(samplesPerAxis*samplesPerAxis)))
			img.SetNRGBA(px, py, color.NRGBA{R: 0, G: 0, B: 0, A: a})
		}
	}
	return img
}

// covered reports whether the sample point lies under any stroke. Round caps
// and joins fall out of using the distance-to-segment form directly.
func covered(x, y float64, segs []segment, c ring, half float64) bool {
	for _, s := range segs {
		if distToSegment(x, y, s) <= half {
			return true
		}
	}
	return math.Abs(math.Hypot(x-c.cx, y-c.cy)-c.r) <= half
}

// distToSegment is the Euclidean distance from (x,y) to the segment s.
func distToSegment(x, y float64, s segment) float64 {
	dx, dy := s.x1-s.x0, s.y1-s.y0
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(x-s.x0, y-s.y0)
	}
	t := ((x-s.x0)*dx + (y-s.y0)*dy) / l2
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(x-(s.x0+t*dx), y-(s.y0+t*dy))
}

// write encodes img as a PNG at path.
func write(path string, img *image.NRGBA) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	return enc.Encode(f, img)
}
