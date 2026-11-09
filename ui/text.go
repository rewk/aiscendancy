// Package ui draws text at the window's native resolution, over the
// pixel-art the views draw at x2 (the original's 640x480 on 1280x960). The font
// is Go Mono, embedded from golang.org/x/image/font/gofont: monospaced so
// tables align by themselves, and nothing to ship beside the binary.
package ui

import (
	"bytes"
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/gomono"
)

var source = func() *text.GoTextFaceSource {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(gomono.TTF))
	if err != nil {
		log.Fatal(err) // the embedded TTF cannot be bad
	}
	return s
}()

// Face is Go Mono at a pixel size. Faces are cheap: make them where needed.
func Face(size float64) *text.GoTextFace {
	return &text.GoTextFace{Source: source, Size: size}
}

// Print draws s with its top-left corner at (x,y), lines spaced at 1.25
// times the size.
func Print(dst *ebiten.Image, s string, face *text.GoTextFace, x, y float64, c color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	op.LineSpacing = face.Size * 1.25
	text.Draw(dst, s, face, op)
}

// Width is the advance of s on one line.
func Width(s string, face *text.GoTextFace) float64 {
	w, _ := text.Measure(s, face, 0)
	return w
}
