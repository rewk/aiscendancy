// shpview browses the game's sprite banks straight out of the COB archives.
//
//	shpview                      starts on the first bank
//	shpview lgrace00             starts on data/lgrace00.shp
//	shpview -pal data/logo.pal   the logo screens use their own palette
//	shpview -shot x.png -frame 3 dkship00   writes one frame as PNG, no window
//	shpview -sheet x.png planitem           every frame of the bank on one PNG
//
// Keys: left/right frame, up/down bank, PgUp/PgDn ten banks, +/- zoom,
// A autoplay the bank's frames, B dark/checker background, C canvas and
// origin, Esc quit. -game picks the install ($ASCEND_DIR or ~/dosbox/Ascend).
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/shp"
)

type viewer struct {
	game   *cob.Game
	pal    color.Palette
	names  []string
	bank   int // index into names
	frame  int
	zoom   int
	play   bool
	tick   int
	dark   bool
	canvas bool

	loaded  *shp.Bank
	images  []*ebiten.Image // one per frame, nil when empty
	loadErr error
	checker *ebiten.Image
}

func (v *viewer) load() {
	v.loaded, v.images, v.loadErr = nil, nil, nil
	b, err := v.game.Read(v.names[v.bank])
	if err == nil {
		v.loaded, err = shp.Decode(b)
	}
	if err != nil {
		v.loadErr = err
		return
	}
	v.images = make([]*ebiten.Image, len(v.loaded.Frames))
	for i, f := range v.loaded.Frames {
		if f.Empty() {
			continue
		}
		v.images[i] = ebiten.NewImageFromImage(f.RGBA(v.pal))
	}
	if v.frame < 0 || v.frame >= len(v.images) {
		v.frame = 0
	}
}

func (v *viewer) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	step := func(d int) {
		v.bank = (v.bank + d + len(v.names)*4) % len(v.names)
		v.frame = 0
		v.load()
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowDown):
		step(1)
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowUp):
		step(-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyPageDown):
		step(10)
	case inpututil.IsKeyJustPressed(ebiten.KeyPageUp):
		step(-10)
	}
	n := len(v.images)
	if n > 0 {
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			v.frame = (v.frame + 1) % n
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			v.frame = (v.frame + n - 1) % n
		}
		if v.play {
			v.tick++
			if v.tick%6 == 0 { // 10 frames a second at 60 ticks
				v.frame = (v.frame + 1) % n
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) || inpututil.IsKeyJustPressed(ebiten.KeyKPAdd) {
		v.zoom = min(v.zoom+1, 8)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) || inpututil.IsKeyJustPressed(ebiten.KeyKPSubtract) {
		v.zoom = max(v.zoom-1, 1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyA) {
		v.play = !v.play
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		v.dark = !v.dark
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		v.canvas = !v.canvas
	}
	return nil
}

const top = 48 // the text band above the sprite

func (v *viewer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{24, 24, 32, 255})
	name := v.names[v.bank]
	head := fmt.Sprintf("%s  [%d/%d]   zoom x%d   up/down bank  left/right frame  +/- zoom  A play  B bg  C canvas  Esc",
		name, v.bank+1, len(v.names), v.zoom)
	ebitenutil.DebugPrintAt(screen, head, 8, 8)
	if v.loadErr != nil {
		ebitenutil.DebugPrintAt(screen, v.loadErr.Error(), 8, 24)
		return
	}
	f := v.loaded.Frames[v.frame]
	info := fmt.Sprintf("frame %d/%d   canvas %dx%d   origin (%d,%d)   box %v",
		v.frame+1, len(v.loaded.Frames), f.Width, f.Height, f.Center.X, f.Center.Y, f.Box)
	if f.Empty() {
		info += "   (empty)"
	}
	ebitenutil.DebugPrintAt(screen, info, 8, 24)

	z := float32(v.zoom)
	ox, oy := float32(8), float32(top)
	if !v.dark {
		v.drawChecker(screen, ox, oy, float32(f.Width)*z, float32(f.Height)*z)
	}
	if img := v.images[v.frame]; img != nil {
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(float64(z), float64(z))
		op.GeoM.Translate(float64(ox), float64(oy))
		screen.DrawImage(img, &op)
	}
	if v.canvas {
		vector.StrokeRect(screen, ox, oy, float32(f.Width)*z, float32(f.Height)*z, 1, color.RGBA{255, 255, 0, 160}, false)
		cx, cy := ox+float32(f.Center.X)*z, oy+float32(f.Center.Y)*z
		vector.StrokeLine(screen, cx-8, cy, cx+8, cy, 1, color.RGBA{255, 64, 64, 255}, false)
		vector.StrokeLine(screen, cx, cy-8, cx, cy+8, 1, color.RGBA{255, 64, 64, 255}, false)
		if !f.Empty() {
			b := f.Box
			vector.StrokeRect(screen, ox+float32(b.Min.X)*z, oy+float32(b.Min.Y)*z, float32(b.Dx())*z, float32(b.Dy())*z, 1, color.RGBA{64, 255, 64, 160}, false)
		}
	}
}

// drawChecker tiles a 16-pixel checkerboard under the sprite so that
// transparency reads as such.
func (v *viewer) drawChecker(screen *ebiten.Image, x, y, w, h float32) {
	if v.checker == nil {
		v.checker = ebiten.NewImage(32, 32)
		v.checker.Fill(color.RGBA{72, 72, 80, 255})
		vector.DrawFilledRect(v.checker, 0, 0, 16, 16, color.RGBA{96, 96, 104, 255}, false)
		vector.DrawFilledRect(v.checker, 16, 16, 16, 16, color.RGBA{96, 96, 104, 255}, false)
	}
	area := screen.SubImage(image.Rect(int(x), int(y), int(x+w), int(y+h))).(*ebiten.Image)
	for ty := y; ty < y+h; ty += 32 {
		for tx := x; tx < x+w; tx += 32 {
			var op ebiten.DrawImageOptions
			op.GeoM.Translate(float64(tx), float64(ty))
			area.DrawImage(v.checker, &op)
		}
	}
}

func (v *viewer) Layout(w, h int) (int, int) { return w, h }

func main() {
	game := flag.String("game", cob.DefaultDir(), "game directory holding ascend*.cob")
	palName := flag.String("pal", "data/game.pal", "palette file inside the archives")
	zoom := flag.Int("zoom", 3, "initial zoom")
	shot := flag.String("shot", "", "write the chosen frame to this PNG and exit")
	frame := flag.Int("frame", 0, "frame to start on (and to shoot)")
	sheet := flag.String("sheet", "", "write every frame of the bank to this PNG (a contact sheet) and exit")
	flag.Parse()

	g, err := cob.OpenGame(*game)
	if err != nil {
		log.Fatal(err)
	}
	defer g.Close()
	pal, err := shp.Palette(must(g.Read(*palName)))
	if err != nil {
		log.Fatal(err)
	}
	var names []string
	for _, n := range g.Names("") {
		if strings.HasSuffix(n, ".shp") || strings.HasSuffix(n, ".tmp") {
			names = append(names, n)
		}
	}
	v := &viewer{game: g, pal: pal, names: names, zoom: *zoom, canvas: true}
	if want := flag.Arg(0); want != "" {
		want = cob.Normalise(want)
		for i, n := range names {
			if n == want || strings.HasSuffix(n, "/"+want) || strings.HasSuffix(n, "/"+want+".shp") {
				v.bank = i
			}
		}
	}
	v.frame = *frame
	v.load()
	if *sheet != "" {
		if v.loadErr != nil {
			log.Fatal(v.loadErr)
		}
		writePNG(*sheet, contactSheet(v.loaded, pal))
		fmt.Printf("%s: %d frames -> %s\n", v.names[v.bank], len(v.loaded.Frames), *sheet)
		return
	}
	if *shot != "" {
		if v.loadErr != nil {
			log.Fatal(v.loadErr)
		}
		f := v.loaded.Frames[v.frame]
		img := f.RGBA(pal)
		// Boxes may spill past the canvas on ships and explosions.
		full := image.NewRGBA(f.Box.Union(img.Bounds()))
		f.Draw(full, pal)
		writePNG(*shot, full)
		fmt.Printf("%s[%d] canvas %dx%d origin %v box %v -> %s\n", v.names[v.bank], v.frame, f.Width, f.Height, f.Center, f.Box, *shot)
		return
	}
	ebiten.SetWindowSize(1280, 800)
	ebiten.SetWindowTitle("shpview")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(v); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		log.Fatal(err)
	}
	return b
}

// contactSheet lays every frame out on a grid of cells the size of the
// largest canvas, each frame at its own canvas position, on a dark ground
// with a 1-pixel gutter.
func contactSheet(bank *shp.Bank, pal color.Palette) *image.RGBA {
	cw, ch := 1, 1
	for _, f := range bank.Frames {
		r := f.Box.Union(image.Rect(0, 0, f.Width, f.Height))
		cw, ch = max(cw, r.Max.X), max(ch, r.Max.Y)
	}
	cols := max(1, min(len(bank.Frames), 1600/(cw+1)))
	rows := (len(bank.Frames) + cols - 1) / cols
	img := image.NewRGBA(image.Rect(0, 0, cols*(cw+1), rows*(ch+1)))
	for i := range img.Pix {
		img.Pix[i] = 40
	}
	for i, f := range bank.Frames {
		x, y := (i%cols)*(cw+1), (i/cols)*(ch+1)
		for yy := 0; yy < ch; yy++ {
			for xx := 0; xx < cw; xx++ {
				img.SetRGBA(x+xx, y+yy, color.RGBA{0, 0, 0, 255})
			}
		}
		if f.Empty() {
			continue
		}
		cell := img.SubImage(image.Rect(x, y, x+cw, y+ch)).(*image.RGBA)
		shifted := *f
		shifted.Box = f.Box.Add(image.Pt(x, y))
		shifted.Draw(cell, pal)
	}
	return img
}

func writePNG(path string, img image.Image) {
	out, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(out, img); err != nil {
		log.Fatal(err)
	}
	out.Close()
}
