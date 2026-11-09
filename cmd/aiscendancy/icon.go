package main

import (
	"image"
	"image/draw"
	"log"

	xdraw "golang.org/x/image/draw"

	"github.com/rewk/aiscendancy/assets"
)

// The window manager's icon: suns.shp frame
// iconSun, the yellow-orange sun, cropped to its Box, centred on a
// transparent square and scaled (Catmull-Rom) to each of iconSizes. Read
// from the archives at start like every asset, never committed.
const iconSun = 3

var iconSizes = [...]int{16, 32, 48, 64}

// windowIcon is the icon's images, nil (logged) when the frame is
// missing: the window keeps the default.
func windowIcon(as *assets.Assets) []image.Image {
	b, err := as.Bank("suns")
	if err != nil || iconSun >= len(b.Frames) || b.Frames[iconSun].Empty() {
		log.Printf("window icon: suns.shp frame %d: %v", iconSun, err)
		return nil
	}
	f := b.Frames[iconSun]
	src := f.RGBA(as.Palette())
	s := max(f.Box.Dx(), f.Box.Dy())
	sq := image.NewRGBA(image.Rect(0, 0, s, s))
	at := image.Pt((s-f.Box.Dx())/2, (s-f.Box.Dy())/2)
	draw.Draw(sq, image.Rectangle{at, at.Add(f.Box.Size())}, src, f.Box.Min, draw.Src)
	out := make([]image.Image, 0, len(iconSizes))
	for _, n := range iconSizes {
		img := image.NewRGBA(image.Rect(0, 0, n, n))
		xdraw.CatmullRom.Scale(img, img.Bounds(), sq, sq.Bounds(), draw.Src, nil)
		out = append(out, img)
	}
	return out
}
