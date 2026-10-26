// Command mockup puts a dashboard PNG inside a drawn e-reader for the README.
//
//	go run ./tools/mockup -in examples/dashboard.png -out examples/kindle.png
package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"

	"github.com/fogleman/gg"
)

func main() {
	in := flag.String("in", "examples/dashboard.png", "dashboard PNG")
	out := flag.String("out", "examples/kindle.png", "output PNG")
	flag.Parse()

	f, err := os.Open(*in)
	if err != nil {
		log.Fatal(err)
	}
	screen, err := png.Decode(f)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}

	sw, sh := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())
	side, top, bottom := sw*0.09, sw*0.09, sw*0.16 // bezels, Paperwhite-like
	bw, bh := sw+2*side, sh+top+bottom
	pad := sw * 0.16
	W, H := int(bw+2*pad), int(bh+2*pad)
	dc := gg.NewContext(W, H)

	// Backdrop, like the NodeDesk showcase.
	bg := gg.NewLinearGradient(0, 0, 0, float64(H))
	bg.AddColorStop(0, color.RGBA{0x1d, 0x20, 0x27, 0xff})
	bg.AddColorStop(1, color.RGBA{0x0b, 0x0c, 0x0f, 0xff})
	dc.SetFillStyle(bg)
	dc.DrawRectangle(0, 0, float64(W), float64(H))
	dc.Fill()

	bx, by := pad, pad
	radius := sw * 0.075

	// Soft shadow: stacked translucent shapes are enough at this size.
	for i := 24; i > 0; i-- {
		g := float64(i) * sw * 0.004
		dc.SetColor(color.RGBA{0, 0, 0, 7})
		dc.DrawRoundedRectangle(bx-g, by-g+sw*0.03, bw+2*g, bh+2*g, radius+g)
		dc.Fill()
	}

	// Body with a faint edge highlight.
	body := gg.NewLinearGradient(bx, by, bx+bw, by+bh)
	body.AddColorStop(0, color.RGBA{0x2b, 0x2c, 0x30, 0xff})
	body.AddColorStop(1, color.RGBA{0x15, 0x16, 0x19, 0xff})
	dc.SetFillStyle(body)
	dc.DrawRoundedRectangle(bx, by, bw, bh, radius)
	dc.Fill()
	dc.SetColor(color.RGBA{0x45, 0x47, 0x4d, 0xff})
	dc.SetLineWidth(sw * 0.004)
	dc.DrawRoundedRectangle(bx, by, bw, bh, radius)
	dc.Stroke()

	// Screen: map gray levels to the warm, slightly dim tones of real e-ink.
	sx, sy := int(bx+side), int(by+top)
	ink := [3]float64{0x24, 0x24, 0x26}
	paper := [3]float64{0xe8, 0xe6, 0xdf}
	tinted := image.NewRGBA(image.Rect(0, 0, int(sw), int(sh)))
	for y := 0; y < int(sh); y++ {
		for x := 0; x < int(sw); x++ {
			r, _, _, _ := screen.At(x, y).RGBA()
			t := float64(r) / 0xffff
			tinted.Set(x, y, color.RGBA{
				uint8(ink[0] + (paper[0]-ink[0])*t),
				uint8(ink[1] + (paper[1]-ink[1])*t),
				uint8(ink[2] + (paper[2]-ink[2])*t),
				0xff,
			})
		}
	}
	dc.DrawImage(tinted, sx, sy)
	dc.SetColor(color.RGBA{0x0a, 0x0a, 0x0b, 0xff})
	dc.SetLineWidth(sw * 0.006)
	dc.DrawRectangle(float64(sx), float64(sy), sw, sh)
	dc.Stroke()

	o, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer o.Close()
	if err := png.Encode(o, dc.Image()); err != nil {
		log.Fatal(err)
	}
}
