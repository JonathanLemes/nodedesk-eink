package render

import (
	"image/color"
	"math"

	"github.com/fogleman/gg"
)

// E-ink panels show 16 gray levels; every tone here is a multiple of 17 so
// nothing gets dithered.
var (
	ink    = color.Gray{Y: 0x11} // text, gauges
	soft   = color.Gray{Y: 0x55} // secondary text
	muted  = color.Gray{Y: 0x88} // captions
	line   = color.Gray{Y: 0xBB} // card borders
	track  = color.Gray{Y: 0xDD} // empty part of gauges and bars
	paper  = color.Gray{Y: 0xEE} // page background
	card   = color.Gray{Y: 0xFF} // card background
	blush  = color.Gray{Y: 0xCC} // mascot cheeks
	accent = color.Gray{Y: 0x66} // second chart series
)

// canvas wraps gg with a scale factor so the layout is written once for the
// 1236x1648 reference screen and works on any Kindle resolution.
type canvas struct {
	*gg.Context
	s float64
}

func (c *canvas) u(v float64) float64 { return v * c.s }

// text draws s with its baseline at y. align: 0 left, 0.5 center, 1 right.
func (c *canvas) text(s string, x, y, px float64, w Weight, col color.Gray, align float64) float64 {
	c.SetFontFace(face(w, c.u(px)))
	c.SetColor(col)
	width, _ := c.MeasureString(s)
	c.DrawString(s, x-width*align, y)
	return width
}

func (c *canvas) measure(s string, px float64, w Weight) float64 {
	c.SetFontFace(face(w, c.u(px)))
	width, _ := c.MeasureString(s)
	return width
}

// card is the NodeDesk widget shape: white, rounded, hairline border.
func (c *canvas) card(x, y, w, h float64) {
	r := c.u(30)
	c.DrawRoundedRectangle(x, y, w, h, r)
	c.SetColor(card)
	c.FillPreserve()
	c.SetColor(line)
	c.SetLineWidth(c.u(2))
	c.Stroke()
}

// cardHeader draws the icon + title row used by every widget and returns the
// baseline it used.
func (c *canvas) cardHeader(x, y, w float64, iconName, title, right string) float64 {
	base := y + c.u(58)
	size := int(c.u(38))
	c.DrawImage(icon(iconName, size, ink), int(x+c.u(30)), int(base-c.u(31)))
	c.text(title, x+c.u(82), base, 34, Bold, ink, 0)
	if right != "" {
		c.text(right, x+w-c.u(30), base, 25, SemiBold, muted, 1)
	}
	return base
}

func (c *canvas) iconAt(name string, x, y, px float64, col color.Gray) {
	c.DrawImage(icon(name, int(c.u(px)), col), int(x), int(y))
}

// ring is the circular gauge of the NodeDesk System widget.
func (c *canvas) ring(cx, cy, r, width, pct float64) {
	c.SetLineCapRound()
	c.SetLineWidth(width)
	c.SetColor(track)
	c.DrawCircle(cx, cy, r)
	c.Stroke()
	if pct > 0.5 {
		start := -math.Pi / 2
		c.SetColor(ink)
		c.DrawArc(cx, cy, r, start, start+2*math.Pi*math.Min(pct, 100)/100)
		c.Stroke()
	}
}

// bar is a rounded progress bar.
func (c *canvas) bar(x, y, w, h, pct float64) {
	c.DrawRoundedRectangle(x, y, w, h, h/2)
	c.SetColor(track)
	c.Fill()
	fill := w * math.Min(pct, 100) / 100
	if fill > 0 {
		c.DrawRoundedRectangle(x, y, math.Max(fill, h), h, h/2)
		c.SetColor(ink)
		c.Fill()
	}
}

// chart plots percent series (0..100) over the last len(series) samples,
// right-aligned so a short history grows from the right edge.
func (c *canvas) chart(x, y, w, h float64, capacity int, series []float64, col color.Gray, width float64, area bool, dash bool) {
	if len(series) < 2 || capacity < 2 {
		return
	}
	step := w / float64(capacity-1)
	startX := x + w - step*float64(len(series)-1)
	pt := func(i int) (float64, float64) {
		v := math.Max(0, math.Min(series[i], 100))
		return startX + step*float64(i), y + h - h*v/100
	}

	if area {
		c.NewSubPath()
		c.MoveTo(startX, y+h)
		for i := range series {
			c.LineTo(pt(i))
		}
		c.LineTo(x+w, y+h)
		c.ClosePath()
		c.SetColor(track)
		c.Fill()
	}

	c.NewSubPath()
	for i := range series {
		c.LineTo(pt(i))
	}
	c.SetColor(col)
	c.SetLineWidth(width)
	c.SetLineCapRound()
	c.SetLineJoinRound()
	if dash {
		c.SetDash(c.u(10), c.u(9))
	}
	c.Stroke()
	c.SetDash()
}

// mascot draws Nodi, a little server with a face. worried swaps the smile for
// a wobbly mouth and adds a sweat drop.
func (c *canvas) mascot(x, y, size float64, worried bool) {
	w, h := size*0.78, size
	bx := x + (size-w)/2
	stroke := c.u(5)

	// Body.
	c.DrawRoundedRectangle(bx, y, w, h, size*0.2)
	c.SetColor(card)
	c.FillPreserve()
	c.SetColor(ink)
	c.SetLineWidth(stroke)
	c.Stroke()

	// Drive bays with LEDs.
	c.SetLineCapRound()
	for i, by := range []float64{y + h*0.70, y + h*0.84} {
		c.SetLineWidth(stroke)
		c.SetColor(ink)
		c.DrawLine(bx+w*0.2, by, bx+w*0.58, by)
		c.Stroke()
		c.DrawCircle(bx+w*0.78, by, size*0.035)
		if i == 0 && !worried {
			c.Fill()
		} else {
			c.SetLineWidth(c.u(3))
			c.Stroke()
		}
	}

	// Face.
	cx, ey := bx+w/2, y+h*0.32
	eyeDX := w * 0.2
	c.SetColor(blush)
	c.DrawEllipse(cx-eyeDX-w*0.07, ey+h*0.1, w*0.08, h*0.035)
	c.DrawEllipse(cx+eyeDX+w*0.07, ey+h*0.1, w*0.08, h*0.035)
	c.Fill()

	c.SetColor(ink)
	for _, ex := range []float64{cx - eyeDX, cx + eyeDX} {
		c.DrawEllipse(ex, ey, size*0.045, size*0.06)
		c.Fill()
	}
	c.SetColor(card)
	for _, ex := range []float64{cx - eyeDX, cx + eyeDX} {
		c.DrawCircle(ex+size*0.015, ey-size*0.022, size*0.016)
		c.Fill()
	}

	c.SetColor(ink)
	c.SetLineWidth(c.u(4.5))
	my := ey + h*0.13
	if worried {
		c.MoveTo(cx-w*0.12, my+h*0.01)
		c.QuadraticTo(cx-w*0.06, my-h*0.03, cx, my+h*0.01)
		c.QuadraticTo(cx+w*0.06, my+h*0.05, cx+w*0.12, my+h*0.01)
		c.Stroke()
		// Sweat drop on the top right corner.
		dx, dy, r := bx+w+size*0.06, y+h*0.12, size*0.055
		c.MoveTo(dx, dy-r*2.2)
		c.QuadraticTo(dx+r*1.1, dy-r*0.4, dx+r, dy+r*0.2)
		c.DrawArc(dx, dy+r*0.2, r, 0, math.Pi)
		c.QuadraticTo(dx-r*1.1, dy-r*0.4, dx, dy-r*2.2)
		c.ClosePath()
		c.SetColor(card)
		c.FillPreserve()
		c.SetColor(ink)
		c.SetLineWidth(c.u(3.5))
		c.Stroke()
	} else {
		c.DrawArc(cx, my-h*0.035, w*0.11, math.Pi*0.15, math.Pi*0.85)
		c.Stroke()
	}
}

// bubble is a speech bubble with its tail pointing left at tailY.
func (c *canvas) bubble(x, y, w, h, tailY float64) {
	r := c.u(30)
	t := c.u(26)
	c.NewSubPath()
	c.MoveTo(x+r, y)
	c.LineTo(x+w-r, y)
	c.DrawArc(x+w-r, y+r, r, -math.Pi/2, 0)
	c.LineTo(x+w, y+h-r)
	c.DrawArc(x+w-r, y+h-r, r, 0, math.Pi/2)
	c.LineTo(x+r, y+h)
	c.DrawArc(x+r, y+h-r, r, math.Pi/2, math.Pi)
	c.LineTo(x, tailY+t*0.7)
	c.LineTo(x-t, tailY)
	c.LineTo(x, tailY-t*0.7)
	c.LineTo(x, y+r)
	c.DrawArc(x+r, y+r, r, math.Pi, math.Pi*1.5)
	c.ClosePath()
	c.SetColor(card)
	c.FillPreserve()
	c.SetColor(line)
	c.SetLineWidth(c.u(2))
	c.SetLineJoinRound()
	c.Stroke()
}
