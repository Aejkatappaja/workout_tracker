package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Aejkatappaja/go-gym/internal/store"
)

// chart geometry (SVG user units); the <svg> scales to its container width.
const (
	chartW = 640
	chartH = 220
	padL   = 44
	padR   = 14
	padT   = 14
	padB   = 26
)

type chartDot struct {
	Cx, Cy string
	Title  string // hover/accessibility label
}

type chartTick struct {
	Y     string
	Label string
}

// LineChart holds the precomputed SVG geometry for a progression line chart.
type LineChart struct {
	Empty   bool
	Line    string // polyline points
	Area    string // filled area path (d)
	Dots    []chartDot
	Ticks   []chartTick
	X0, X1  string // first / last date labels
	Summary string // aria-label / caption
}

type chartBar struct {
	X, Y, W, H string
	LabelX     string // centered x for the week tick label
	Label      string // short week label (MM-DD)
	Title      string // hover/accessibility label
}

// BarChart holds the precomputed SVG geometry for a weekly-volume bar chart.
type BarChart struct {
	Empty   bool
	Bars    []chartBar
	Ticks   []chartTick
	Summary string
}

func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

// BuildLineChart maps progression points to SVG coordinates. Values are the daily
// best e1RM; x is spread evenly by index (days are already ordered oldest-first).
func BuildLineChart(points []store.ProgressPoint) LineChart {
	if len(points) == 0 {
		return LineChart{Empty: true}
	}

	minV, maxV := points[0].E1RM, points[0].E1RM
	for _, p := range points {
		if p.E1RM < minV {
			minV = p.E1RM
		}
		if p.E1RM > maxV {
			maxV = p.E1RM
		}
	}
	// pad the range so a flat/near-flat series isn't a line on the edge
	span := maxV - minV
	if span < 1 {
		span = maxV
		if span < 1 {
			span = 1
		}
	}
	lo := minV - span*0.15
	hi := maxV + span*0.15

	innerW := float64(chartW - padL - padR)
	innerH := float64(chartH - padT - padB)
	x := func(i int) float64 {
		if len(points) == 1 {
			return padL + innerW/2
		}
		return padL + innerW*float64(i)/float64(len(points)-1)
	}
	y := func(v float64) float64 {
		return padT + innerH*(1-(v-lo)/(hi-lo))
	}

	var line strings.Builder
	dots := make([]chartDot, 0, len(points))
	for i, p := range points {
		px, py := x(i), y(p.E1RM)
		if i > 0 {
			line.WriteByte(' ')
		}
		fmt.Fprintf(&line, "%s,%s", f1(px), f1(py))
		dots = append(dots, chartDot{
			Cx:    f1(px),
			Cy:    f1(py),
			Title: fmt.Sprintf("%s · %.0f", p.Day, p.E1RM),
		})
	}

	area := fmt.Sprintf("M%s,%s L%s L%s,%s Z",
		f1(x(0)), f1(chartH-padB),
		line.String(),
		f1(x(len(points)-1)), f1(chartH-padB))

	// 3 horizontal ticks (lo-ish, mid, hi-ish) using the padded range
	ticks := make([]chartTick, 0, 3)
	for _, frac := range []float64{0, 0.5, 1} {
		v := lo + (hi-lo)*frac
		ticks = append(ticks, chartTick{Y: f1(y(v)), Label: strconv.FormatFloat(v, 'f', 0, 64)})
	}

	return LineChart{
		Line:    line.String(),
		Area:    area,
		Dots:    dots,
		Ticks:   ticks,
		X0:      points[0].Day,
		X1:      points[len(points)-1].Day,
		Summary: fmt.Sprintf("estimated 1RM over %d sessions, from %.0f to %.0f", len(points), points[0].E1RM, points[len(points)-1].E1RM),
	}
}

// BuildBarChart maps weekly volume points to SVG bars, oldest week first. The y
// axis runs from 0 so bar heights read as absolute volume.
func BuildBarChart(points []store.VolumePoint) BarChart {
	if len(points) == 0 {
		return BarChart{Empty: true}
	}

	maxV := points[0].Volume
	var total float64
	for _, p := range points {
		total += p.Volume
		if p.Volume > maxV {
			maxV = p.Volume
		}
	}
	hi := maxV * 1.1 // headroom above the tallest bar
	if hi < 1 {
		hi = 1
	}

	innerW := float64(chartW - padL - padR)
	innerH := float64(chartH - padT - padB)
	baseY := float64(chartH - padB)
	slot := innerW / float64(len(points))
	bw := slot * 0.6 // bar width, leaving gaps between weeks

	bars := make([]chartBar, 0, len(points))
	for i, p := range points {
		h := innerH * (p.Volume / hi)
		bx := padL + slot*float64(i) + (slot-bw)/2
		by := baseY - h
		bars = append(bars, chartBar{
			X:      f1(bx),
			Y:      f1(by),
			W:      f1(bw),
			H:      f1(h),
			LabelX: f1(bx + bw/2),
			Label:  weekLabel(p.Week),
			Title:  fmt.Sprintf("week of %s · %.0f", p.Week, p.Volume),
		})
	}

	ticks := make([]chartTick, 0, 3)
	for _, frac := range []float64{0, 0.5, 1} {
		v := hi * frac
		y := padT + innerH*(1-frac)
		ticks = append(ticks, chartTick{Y: f1(y), Label: strconv.FormatFloat(v, 'f', 0, 64)})
	}

	return BarChart{
		Bars:    bars,
		Ticks:   ticks,
		Summary: fmt.Sprintf("weekly training volume over %d weeks, %.0f total", len(points), total),
	}
}

// weekLabel shortens a YYYY-MM-DD week start to MM-DD for the x axis.
func weekLabel(week string) string {
	if len(week) == 10 {
		return week[5:]
	}
	return week
}
