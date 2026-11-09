package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/save"
	"github.com/rewk/aiscendancy/ui"
)

// The opening screen, the original's main menu: the binary starts on it
// without -load, never comes back to it. The picture is data/0opening.gif
// (640x480, its own palette) with seven blue slots, 319x31 at (159,
// 200+40k), filled in the original's order with its labels
// (0opening.shp), redrawn in Go Mono (the original's font is
// ugly) in the colours of a capture of the original's: white on navy,
// the key letter in the frame's teal. Ours plays five: Play Introduction
// (movie.go), New Game, Resume Last Game (lastFile, saveLast's), Load
// Saved Game, Settings (settings.go) in Save Game's slot, Exit to DOS;
// Tutorial is drawn dimmed, dead.
const (
	openPicture = "data/0opening.gif"
	openSlotX   = 159
	openSlotY   = 200
	openPitch   = 40
	openW       = 319
	openH       = 31
	openDim     = 0.45 // a dead label's brightness
)

// The labels, in the slots' order.
const (
	openIntro = iota
	openTutorial
	openNew
	openResume
	openLoad
	openSettings // the original's Save Game
	openExit
	numOpenLabels
)

// The labels' face and colours, sampled on a capture of the original.
var (
	openFace = ui.Face(20)
	openFill = color.RGBA{12, 32, 49, 255}
	openInk  = color.RGBA{255, 255, 255, 255}
	openKey  = color.RGBA{125, 182, 182, 255}
)

// openWords are the labels, the key letter's index after each: the
// original's red letters but Load's and Exit's, whose Alt-L and Alt-X
// line is dropped for their letter in the word.
var openWords = [numOpenLabels]struct {
	text string
	key  int
}{
	{"Play Introduction", 5}, {"Tutorial", 0}, {"New Game", 0}, {"Resume Last Game", 0},
	{"Load Saved Game", 0}, {"Settings", 0}, {"Exit to DOS", 1},
}

// openKeys are the live labels' keys, the key letters (Alt is ignored).
var openKeys = map[int]ebiten.Key{openIntro: ebiten.KeyI, openNew: ebiten.KeyN, openResume: ebiten.KeyR, openLoad: ebiten.KeyL, openSettings: ebiten.KeyS, openExit: ebiten.KeyX}

// openSlot is label k's slot.
func openSlot(k int) rect {
	y := openSlotY + openPitch*k
	return orig(openSlotX, y, openSlotX+openW, y+openH)
}

// openingArt is the picture, made by preload.
type openingArt struct {
	picture *ebiten.Image
}

// loadOpening decodes the picture.
func loadOpening(as *assets.Assets) (*openingArt, error) {
	raw, err := as.Game.Read(openPicture)
	if err != nil {
		return nil, err
	}
	pic, err := gif.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", openPicture, err)
	}
	return &openingArt{picture: ebiten.NewImageFromImage(pic)}, nil
}

// openingScreen is the main menu; resume is whether lastPath loads.
type openingScreen struct {
	resume bool
}

func newOpeningScreen() *openingScreen { return &openingScreen{} }

// lastFile is saveLast's file, beside the quick save.
const lastFile = "last.json.gz"

// Enter peeks lastPath: Resume is live only for this version's.
func (s *openingScreen) Enter(a *app) {
	v, _, err := save.Peek(a.lastPath)
	s.resume = err == nil && v == save.Version
}

// live reports label k plays.
func (s *openingScreen) live(k int) bool {
	_, ok := openKeys[k]
	return ok && (k != openResume || s.resume)
}

func (s *openingScreen) Update(a *app) error {
	x, y := a.mouse()
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	for k, key := range openKeys {
		if s.live(k) && (keyJustPressed(key) || click && openSlot(k).has(x, y)) {
			return s.choose(a, k)
		}
	}
	return nil
}

// choose plays label k: the introduction and Load open over the opening,
// New Game replaces it (no way back), Resume loads lastPath, Exit
// quits.
func (s *openingScreen) choose(a *app, k int) error {
	switch k {
	case openIntro:
		a.push(newMovieScreen())
	case openNew:
		clear(a.stack)
		a.stack = nil
		a.open([]screen{newNewGameScreen()})
	case openResume:
		a.loadPath(a.lastPath)
	case openLoad:
		a.push(newSlotsScreen(false))
	case openSettings:
		a.openSettings()
	case openExit:
		return ebiten.Termination
	}
	return nil
}

func (s *openingScreen) Render(a *app, sc *ebiten.Image) {
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(zoom, zoom)
	sc.DrawImage(a.opening.picture, &op)
	for k := range numOpenLabels {
		r := openSlot(k)
		sc.SubImage(image.Rect(r.X0, r.Y0, r.X1, r.Y1)).(*ebiten.Image).Fill(s.tone(k, openFill))
	}
}

// tone is c, dimmed on a dead label.
func (s *openingScreen) tone(k int, c color.RGBA) color.RGBA {
	if s.live(k) {
		return c
	}
	f := func(v uint8) uint8 { return uint8(float64(v) * openDim) }
	return color.RGBA{f(c.R), f(c.G), f(c.B), 255}
}

// Text writes each label centred in its slot, the key letter teal.
func (s *openingScreen) Text(_ *app, frame *ebiten.Image) {
	for k, w := range openWords {
		r := openSlot(k)
		x := float64(r.X0+r.X1)/2 - ui.Width(w.text, openFace)/2
		y := float64(r.Y0+r.Y1)/2 - openFace.Size/2 - zoom
		for _, part := range []struct {
			s string
			c color.RGBA
		}{{w.text[:w.key], openInk}, {w.text[w.key : w.key+1], openKey}, {w.text[w.key+1:], openInk}} {
			ui.Print(frame, part.s, openFace, x, y, s.tone(k, part.c))
			x += ui.Width(part.s, openFace)
		}
	}
}
