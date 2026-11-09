// Package assets is the Ebiten-side cache of the game's sprites.
//
// It opens the installed game (cob.OpenGame), reads data/game.pal once, and
// decodes SHP banks on first use. A Sprite is one frame turned into an
// *ebiten.Image holding the frame's drawn Box only (canvases run up to
// 640x480 and ships' boxes spill past theirs), plus the Offset from the
// frame's origin to the Box's top-left: drawing the Image at (x,y)+Offset
// puts the frame's origin at (x,y). Banks and sprites live as long as the
// Assets; nothing is evicted.
package assets

import (
	"fmt"
	"image"
	"image/color"
	"path"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/shp"
)

type key struct {
	bank  string // normalised path, "data/planitem.shp"
	frame int
	ramp  int // SpriteIn's colour, 0 for the frame as drawn
}

// Assets holds the open game and what has been decoded from it.
type Assets struct {
	Game    *cob.Game
	pal     color.Palette
	banks   map[string]*shp.Bank
	sprites map[key]*Sprite
}

// Open opens the game in dir and reads its main palette.
func Open(dir string) (*Assets, error) {
	g, err := cob.OpenGame(dir)
	if err != nil {
		return nil, err
	}
	b, err := g.Read("data/game.pal")
	if err == nil {
		var pal color.Palette
		if pal, err = shp.Palette(b); err == nil {
			return &Assets{Game: g, pal: pal, banks: map[string]*shp.Bank{}, sprites: map[key]*Sprite{}}, nil
		}
	}
	g.Close()
	return nil, err
}

// Close releases the archives. Sprites already made stay usable.
func (a *Assets) Close() { a.Game.Close() }

// Ramp is entry i (0..3, dark to light) of colour's ramp in game.pal.
func (a *Assets) Ramp(colour, i int) color.RGBA {
	return a.pal[rampBase+rampLen*colour+i].(color.RGBA)
}

// PaletteIn is game.pal with the green ramp (entries 16..19) replaced by
// colour's, SpriteIn's; colour 0 is game.pal itself.
func (a *Assets) PaletteIn(colour int) color.Palette {
	if colour == 0 {
		return a.pal
	}
	pal := slices.Clone(a.pal)
	for i := range rampLen {
		pal[rampBase+i] = a.pal[rampBase+rampLen*colour+i]
	}
	return pal
}

// Palette is data/game.pal.
func (a *Assets) Palette() color.Palette { return a.pal }

// bankPath turns "planitem", "planitem.shp" or "data/planitem.shp" into the
// archive name: no directory means data/, no extension means .shp.
func bankPath(name string) string {
	n := cob.Normalise(name)
	if !strings.Contains(n, "/") {
		n = "data/" + n
	}
	if path.Ext(n) == "" {
		n += ".shp"
	}
	return n
}

// Bank decodes a sprite bank once and keeps it.
func (a *Assets) Bank(name string) (*shp.Bank, error) {
	n := bankPath(name)
	if b, ok := a.banks[n]; ok {
		return b, nil
	}
	raw, err := a.Game.Read(n)
	if err != nil {
		return nil, err
	}
	b, err := shp.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", n, err)
	}
	a.banks[n] = b
	return b, nil
}

// Sprite is one frame ready to draw. Image is nil for an empty frame.
type Sprite struct {
	Image  *ebiten.Image
	Offset image.Point // Box.Min - Center: the Image's top-left relative to the origin
	Frame  *shp.Frame
}

// Sprite makes (once) the drawable form of one frame of a bank.
func (a *Assets) Sprite(name string, frame int) (*Sprite, error) { return a.SpriteIn(name, frame, 0) }

// The colour ramps of game.pal: 4 entries each from rampBase, green
// first, in the empires' colour order (game.Green..Orange); the flags
// (raceflag.shp) are drawn in the green one.
const rampBase, rampLen = 16, 4

// SpriteIn is Sprite with the green ramp moved to colour's: palette
// entries 16..19 drawn as 16+4·colour.., the original's flags in an
// empire's colour (its Player Color hole draws them so).
func (a *Assets) SpriteIn(name string, frame, colour int) (*Sprite, error) {
	k := key{bankPath(name), frame, colour}
	if s, ok := a.sprites[k]; ok {
		return s, nil
	}
	b, err := a.Bank(name)
	if err != nil {
		return nil, err
	}
	if frame < 0 || frame >= len(b.Frames) {
		return nil, fmt.Errorf("%s: frame %d of %d", k.bank, frame, len(b.Frames))
	}
	if colour < 0 || rampBase+rampLen*(colour+1) > len(a.pal) {
		return nil, fmt.Errorf("%s: no colour ramp %d", k.bank, colour)
	}
	s := newSprite(b.Frames[frame], a.PaletteIn(colour))
	a.sprites[k] = s
	return s, nil
}

// newSprite draws f in pal: its Box moved to (0,0) on a Box-sized image.
func newSprite(f *shp.Frame, pal color.Palette) *Sprite {
	s := &Sprite{Frame: f}
	if !f.Empty() {
		moved := *f
		moved.Box = f.Box.Sub(f.Box.Min)
		img := image.NewRGBA(moved.Box)
		moved.Draw(img, pal)
		s.Image = ebiten.NewImageFromImage(img)
		s.Offset = f.Box.Min.Sub(f.Center)
	}
	return s
}

// Graft registers frame dstFrame of bank dst, one past the bank's own
// frames, as frame srcFrame of bank src set on a w×h canvas whose origin
// is origin: the source's origin lands at at (canvas coordinates), the
// pixels outside the canvas are cut, every palette index in remap is
// replaced (the recolouring stays in game.pal). Sprite(dst, dstFrame)
// then hits the cache; SpriteIn's other colours do not. An index the
// bank already has is refused.
func (a *Assets) Graft(dst string, dstFrame int, src string, srcFrame int, w, h int, origin, at image.Point, remap map[uint8]uint8) error {
	db, err := a.Bank(dst)
	if err != nil {
		return err
	}
	if dstFrame < len(db.Frames) {
		return fmt.Errorf("%s: frame %d is the bank's own (%d)", bankPath(dst), dstFrame, len(db.Frames))
	}
	sb, err := a.Bank(src)
	if err != nil {
		return err
	}
	if srcFrame < 0 || srcFrame >= len(sb.Frames) {
		return fmt.Errorf("%s: frame %d of %d", bankPath(src), srcFrame, len(sb.Frames))
	}
	sf := sb.Frames[srcFrame]
	shift := at.Sub(sf.Center) // source canvas -> dst canvas
	box := sf.Box.Add(shift).Intersect(image.Rect(0, 0, w, h))
	f := &shp.Frame{Width: w, Height: h, Center: origin, Box: box,
		Index: make([]uint8, box.Dx()*box.Dy()), Opaque: make([]bool, box.Dx()*box.Dy())}
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			sp := image.Pt(x, y).Sub(shift).Sub(sf.Box.Min)
			k, d := sp.Y*sf.Box.Dx()+sp.X, (y-box.Min.Y)*box.Dx()+x-box.Min.X
			if !sf.Opaque[k] {
				continue
			}
			ix := sf.Index[k]
			if r, ok := remap[ix]; ok {
				ix = r
			}
			f.Index[d], f.Opaque[d] = ix, true
		}
	}
	a.sprites[key{bankPath(dst), dstFrame, 0}] = newSprite(f, a.pal)
	return nil
}

// DrawAt draws the sprite with its origin at (x,y).
func (s *Sprite) DrawAt(dst *ebiten.Image, x, y float64) {
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(x, y)
	s.Draw(dst, &op)
}

// Draw draws the sprite with its origin at (0,0) transformed by op.GeoM, so a
// scale in op scales about the origin. op is not modified.
func (s *Sprite) Draw(dst *ebiten.Image, op *ebiten.DrawImageOptions) {
	if s.Image == nil {
		return
	}
	o := *op
	o.GeoM.Reset()
	o.GeoM.Translate(float64(s.Offset.X), float64(s.Offset.Y))
	o.GeoM.Concat(op.GeoM)
	dst.DrawImage(s.Image, &o)
}
