// Package render draws a metrics.Snapshot as a grayscale PNG sized for an
// e-ink screen. The layout is written for a 1236x1648 Kindle Paperwhite and
// scaled for other resolutions.
package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"time"

	"github.com/fogleman/gg"

	"github.com/JonathanLemes/nodedesk-eink/internal/metrics"
)

const refW, refH = 1236, 1648

// Options describe the target screen and the device showing the dashboard.
type Options struct {
	Width, Height int
	Lang          string
	// Battery is the Kindle battery level (0-100), or -1 when unknown.
	Battery  int
	Charging bool
}

// Render draws the dashboard and returns an 8-bit grayscale image.
func Render(s metrics.Snapshot, o Options) *image.Gray {
	if o.Width <= 0 || o.Height <= 0 {
		o.Width, o.Height = refW, refH
	}
	c := &canvas{
		Context: gg.NewContext(o.Width, o.Height),
		s:       math.Min(float64(o.Width)/refW, float64(o.Height)/refH),
	}
	l := LangFor(o.Lang)
	c.SetColor(paper)
	c.Clear()

	// Center the reference layout when the aspect ratio differs.
	ox := (float64(o.Width) - c.u(refW)) / 2
	oy := (float64(o.Height) - c.u(refH)) / 2
	m, gap := c.u(44), c.u(24)
	left, right := ox+m, ox+c.u(refW)-m
	width := right - left
	bottom := oy + c.u(refH) - c.u(64) // footer area

	y := oy + c.u(40)
	y = header(c, l, s, left, right, y)
	y = status(c, l, s, left, width, y+c.u(22))
	y = system(c, l, s, left, width, y+gap)

	// Storage and the bottom row have fixed heights; the chart takes the rest.
	rowH := c.u(92)
	minChart := c.u(220)
	bottomH := c.u(150)
	disks := s.Disks
	for len(disks) > 1 && bottom-y-gap*3-bottomH-c.u(84)-rowH*float64(len(disks)) < minChart {
		disks = disks[:len(disks)-1]
	}
	storageH := c.u(84) + rowH*float64(len(disks)) + c.u(8)
	chartH := bottom - y - gap*3 - bottomH - storageH

	y = history(c, l, s, left, width, y+gap, chartH)
	y = storage(c, l, disks, left, width, y+gap, rowH)
	bottomRow(c, l, s, left, width, y+gap, bottomH)
	footer(c, l, s, o, ox+c.u(refW)/2, oy+c.u(refH)-c.u(26))

	img := image.NewGray(image.Rect(0, 0, o.Width, o.Height))
	draw.Draw(img, img.Bounds(), c.Image(), image.Point{}, draw.Src)
	return img
}

func header(c *canvas, l *Lang, s metrics.Snapshot, left, right, y float64) float64 {
	base := y + c.u(56)
	c.text(s.Hostname, left, base, 56, ExtraBold, ink, 0)
	c.text(s.Time.Format("15:04"), right, base, 56, ExtraBold, ink, 1)

	sub := base + c.u(44)
	parts := []string{}
	if s.OS != "" {
		parts = append(parts, s.OS)
	}
	if s.Uptime > 0 {
		parts = append(parts, fmt.Sprintf(l.Up, duration(s.Uptime)))
	}
	c.text(strings.Join(parts, "  ·  "), left, sub, 28, SemiBold, muted, 0)
	c.text(l.date(s.Time), right, sub, 28, SemiBold, muted, 1)
	return sub + c.u(14)
}

func status(c *canvas, l *Lang, s metrics.Snapshot, left, width, y float64) float64 {
	issues := alerts(l, s)
	size := c.u(150)
	h := c.u(150)
	c.mascot(left+c.u(4), y, size, len(issues) > 0)

	bx := left + size + c.u(46)
	c.bubble(bx, y, left+width-bx, h, y+h*0.42)

	title, body := l.CalmTitle, l.CalmBody
	if len(issues) > 0 {
		title, body = l.WarnTitle, strings.Join(issues, "  ·  ")
	}
	c.text(title, bx+c.u(34), y+c.u(66), 38, ExtraBold, ink, 0)
	c.text(fit(c, body, left+width-bx-c.u(68), 27, SemiBold), bx+c.u(34), y+c.u(112), 27, SemiBold, soft, 0)
	return y + h
}

// alerts lists what deserves attention, most urgent first.
func alerts(l *Lang, s metrics.Snapshot) []string {
	var out []string
	if s.CPU.Temp >= 85 {
		out = append(out, fmt.Sprintf(l.HotCPU, l.temp(s.CPU.Temp)))
	}
	if s.GPU != nil && s.GPU.Temp >= 85 {
		out = append(out, fmt.Sprintf(l.HotGPU, l.temp(s.GPU.Temp)))
	}
	for _, d := range s.Disks {
		if d.Percent >= 90 {
			out = append(out, fmt.Sprintf(l.FullDisk, diskLabel(l, d), l.pct(d.Percent)))
		}
	}
	if s.Memory.Percent >= 90 {
		out = append(out, fmt.Sprintf(l.FullRAM, l.pct(s.Memory.Percent)))
	}
	if s.Docker != nil {
		for _, name := range s.Docker.Unhealthy {
			out = append(out, fmt.Sprintf(l.Unhealthy, name))
		}
	}
	return out
}

func system(c *canvas, l *Lang, s metrics.Snapshot, left, width, y float64) float64 {
	h := c.u(420)
	c.card(left, y, width, h)
	load := fmt.Sprintf("%s  %s  %s  %s", l.Load, l.num(s.Load[0], 2), l.num(s.Load[1], 2), l.num(s.Load[2], 2))
	c.cardHeader(left, y, width, "activity", l.System, load)

	type gauge struct {
		label, icon string
		pct, temp   float64
		detail      string
		missing     bool
	}
	gauges := []gauge{
		{label: "CPU", icon: "cpu", pct: s.CPU.Percent, temp: s.CPU.Temp, detail: fmt.Sprintf(l.Threads, s.CPU.Threads)},
		{label: "RAM", icon: "memory-stick", pct: s.Memory.Percent, detail: l.bytes(s.Memory.Used) + " / " + l.bytes(s.Memory.Total)},
		{label: "GPU", icon: "gpu", missing: s.GPU == nil, detail: l.NoGPU},
	}
	if g := s.GPU; g != nil {
		gauges[2].pct, gauges[2].temp = g.Percent, g.Temp
		gauges[2].detail = l.bytes(g.MemUsed) + " / " + l.bytes(g.MemTotal)
	}

	col := width / 3
	r, rw := c.u(98), c.u(22)
	cy := y + c.u(100) + r
	for i, g := range gauges {
		cx := left + col*float64(i) + col/2
		c.ring(cx, cy, r, rw, g.pct)
		value := l.pct(g.pct)
		if g.missing {
			value = "-"
		}
		c.text(value, cx, cy+c.u(19), 54, ExtraBold, ink, 0.5)

		base := cy + r + c.u(62)
		lw := c.measure(g.label, 30, ExtraBold)
		tw := 0.0
		if g.temp > 0 {
			tw = c.u(30) + c.measure(l.temp(g.temp), 28, SemiBold)
		}
		gapW := 0.0
		if tw > 0 {
			gapW = c.u(18)
		}
		x0 := cx - (lw+gapW+tw)/2
		c.text(g.label, x0, base, 30, ExtraBold, ink, 0)
		if g.temp > 0 {
			tx := x0 + lw + gapW
			c.iconAt("thermometer", tx, base-c.u(26), 28, soft)
			c.text(l.temp(g.temp), tx+c.u(30), base, 28, SemiBold, soft, 0)
		}
		c.text(g.detail, cx, base+c.u(38), 24, SemiBold, muted, 0.5)
	}
	return y + h
}

func history(c *canvas, l *Lang, s metrics.Snapshot, left, width, y, h float64) float64 {
	c.card(left, y, width, h)
	c.cardHeader(left, y, width, "chart-spline", l.LastHour, "")

	// Legend, right-aligned in the header. Series differ by tone and dash
	// because e-ink has no colour.
	type entry struct {
		name string
		col  color.Gray
		dash bool
	}
	legend := []entry{{"CPU", ink, false}, {"RAM", ink, true}}
	if len(s.History.GPU) > 0 {
		legend = append(legend, entry{"GPU", accent, false})
	}
	lx := left + width - c.u(30)
	base := y + c.u(58)
	for i := len(legend) - 1; i >= 0; i-- {
		it := legend[i]
		tw := c.measure(it.name, 25, SemiBold)
		c.text(it.name, lx, base, 25, SemiBold, soft, 1)
		lx -= tw + c.u(12)
		c.SetColor(it.col)
		c.SetLineWidth(c.u(5))
		c.SetLineCapRound()
		if it.dash {
			c.SetDash(c.u(8), c.u(7))
		}
		c.DrawLine(lx-c.u(34), base-c.u(9), lx, base-c.u(9))
		c.Stroke()
		c.SetDash()
		lx -= c.u(34) + c.u(30)
	}

	px, py := left+c.u(30), y+c.u(96)
	pw, ph := width-c.u(60), h-c.u(96)-c.u(30)

	// Grid at 0, 50 and 100 %.
	c.SetColor(track)
	c.SetLineWidth(c.u(2))
	for _, f := range []float64{0, 0.5, 1} {
		gy := py + ph*f
		c.SetDash(c.u(3), c.u(9))
		c.DrawLine(px, gy, px+pw, gy)
		c.Stroke()
	}
	c.SetDash()

	capacity := int(time.Hour / max(s.History.Interval, time.Second))
	c.chart(px, py, pw, ph, capacity, s.History.CPU, ink, c.u(5), true, false)
	if len(s.History.GPU) > 0 {
		c.chart(px, py, pw, ph, capacity, s.History.GPU, accent, c.u(4), false, false)
	}
	c.chart(px, py, pw, ph, capacity, s.History.Memory, ink, c.u(4), false, true)
	return y + h
}

func storage(c *canvas, l *Lang, disks []metrics.Disk, left, width, y, rowH float64) float64 {
	h := c.u(84) + rowH*float64(len(disks)) + c.u(8)
	c.card(left, y, width, h)
	c.cardHeader(left, y, width, "hard-drive", l.Storage, "")

	ry := y + c.u(84)
	for _, d := range disks {
		x := left + c.u(30)
		c.iconAt("hard-drive", x, ry+c.u(8), 40, soft)
		tx := x + c.u(60)
		c.text(diskLabel(l, d), tx, ry+c.u(36), 29, Bold, ink, 0)
		pctX := left + width - c.u(30)
		c.text(l.pct(d.Percent), pctX, ry+c.u(36), 29, ExtraBold, ink, 1)
		pw := c.measure("100%", 29, ExtraBold) + c.u(24)
		c.text(l.bytes(d.Used)+" / "+l.bytes(d.Total), pctX-pw, ry+c.u(36), 24, SemiBold, muted, 1)
		c.bar(tx, ry+c.u(52), pctX-tx, c.u(14), d.Percent)
		ry += rowH
	}
	return y + h
}

func bottomRow(c *canvas, l *Lang, s metrics.Snapshot, left, width, y, h float64) {
	gap := c.u(24)
	w := (width - gap) / 2

	c.card(left, y, w, h)
	c.cardHeader(left, y, w, "wifi", l.Network, "")
	base := y + c.u(116)
	x := left + c.u(30)
	c.iconAt("arrow-down", x, base-c.u(28), 32, ink)
	c.text(l.rate(s.Net.RxRate), x+c.u(38), base, 30, ExtraBold, ink, 0)
	x2 := left + w/2 + c.u(10)
	c.iconAt("arrow-up", x2, base-c.u(28), 32, soft)
	c.text(l.rate(s.Net.TxRate), x2+c.u(38), base, 30, ExtraBold, soft, 0)

	dx := left + w + gap
	c.card(dx, y, w, h)
	c.cardHeader(dx, y, w, "container", l.Docker, "")
	if d := s.Docker; d != nil {
		n := fmt.Sprintf("%d", d.Running)
		nw := c.text(n, dx+c.u(30), base, 40, ExtraBold, ink, 0)
		c.text(fmt.Sprintf(" / %d  %s %s", d.Total, l.Containers, l.Running), dx+c.u(30)+nw, base, 26, SemiBold, muted, 0)
	} else {
		c.text("-", dx+c.u(30), base, 40, ExtraBold, muted, 0)
	}
}

func footer(c *canvas, l *Lang, s metrics.Snapshot, o Options, cx, base float64) {
	label := "NodeDesk E-Ink"
	if o.Battery >= 0 {
		label = fmt.Sprintf("%s %d%%", l.Kindle, o.Battery)
		if o.Charging {
			label += " · " + l.Charging
		}
		label += "  ·  NodeDesk E-Ink"
	}
	iconName := "battery"
	if o.Charging {
		iconName = "battery-charging"
	}
	tw := c.measure(label, 23, SemiBold)
	iw := 0.0
	if o.Battery >= 0 {
		iw = c.u(40)
		c.iconAt(iconName, cx-(tw+iw)/2, base-c.u(25), 32, muted)
	}
	c.text(label, cx-(tw+iw)/2+iw, base, 23, SemiBold, muted, 0)
}

func diskLabel(l *Lang, d metrics.Disk) string {
	if d.Label == "" {
		return l.RootDisk
	}
	return d.Label
}

// fit shortens s with an ellipsis until it fits in width.
func fit(c *canvas, s string, width, px float64, w Weight) string {
	if c.measure(s, px, w) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 1 && c.measure(string(r)+"…", px, w) > width {
		r = r[:len(r)-1]
	}
	return strings.TrimSpace(string(r)) + "…"
}
