package render

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

//go:embed assets/fonts/*.ttf assets/icons/*.svg
var assets embed.FS

// Weight selects one of the embedded Nunito instances.
type Weight string

const (
	Regular   Weight = "Regular"
	SemiBold  Weight = "SemiBold"
	Bold      Weight = "Bold"
	ExtraBold Weight = "ExtraBold"
)

var (
	fontMu    sync.Mutex
	fonts     = map[Weight]*opentype.Font{}
	faces     = map[string]font.Face{}
	iconMu    sync.Mutex
	iconCache = map[string]image.Image{}
)

// face returns a cached font face of the given weight and pixel size.
func face(w Weight, px float64) font.Face {
	fontMu.Lock()
	defer fontMu.Unlock()
	key := fmt.Sprintf("%s/%.1f", w, px)
	if f, ok := faces[key]; ok {
		return f
	}
	f, ok := fonts[w]
	if !ok {
		data, err := assets.ReadFile("assets/fonts/Nunito-" + string(w) + ".ttf")
		if err != nil {
			panic(err) // embedded at build time, cannot fail at run time
		}
		if f, err = opentype.Parse(data); err != nil {
			panic(err)
		}
		fonts[w] = f
	}
	ff, err := opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	faces[key] = ff
	return ff
}

// icon rasterises a Lucide icon (stroke icons drawn with currentColor) at
// size px in color c.
func icon(name string, px int, c color.Gray) image.Image {
	iconMu.Lock()
	defer iconMu.Unlock()
	key := fmt.Sprintf("%s/%d/%d", name, px, c.Y)
	if img, ok := iconCache[key]; ok {
		return img
	}
	data, err := assets.ReadFile("assets/icons/" + name + ".svg")
	if err != nil {
		panic(err)
	}
	hex := fmt.Sprintf("#%02x%02x%02x", c.Y, c.Y, c.Y)
	data = bytes.ReplaceAll(data, []byte("currentColor"), []byte(hex))
	ic, err := oksvg.ReadIconStream(strings.NewReader(string(data)))
	if err != nil {
		panic(err)
	}
	ic.SetTarget(0, 0, float64(px), float64(px))
	img := image.NewRGBA(image.Rect(0, 0, px, px))
	scanner := rasterx.NewScannerGV(px, px, img, img.Bounds())
	ic.Draw(rasterx.NewDasher(px, px, scanner), 1)
	iconCache[key] = img
	return img
}
