package main

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/rewk/aiscendancy/ui"
)

// Pieces shared by the screens on the starmap's chrome and their lists.

// drawMapChrome paints the starmap's chrome over black, the big area and
// stacked boxes [0, boxes) black, the turn box.
func drawMapChrome(a *app, sc *ebiten.Image, boxes int) {
	sc.Fill(color.Black)
	drawSprite(sc, a.spr("1starmap.tmp", 0), 0, 0)
	fillRect(sc, mapArea)
	for _, r := range mapStack[:boxes] {
		fillRect(sc, r)
	}
	drawTurnBox(a, sc)
}

// putBox writes str in box r, padX/padY inside: clipped to the lines the
// box holds, each line cut to its width.
func putBox(frame *ebiten.Image, r rect, str string, face *text.GoTextFace, c color.RGBA) {
	w := float64(r.X1 - r.X0 - 2*padX)
	ls := strings.Split(clip(str, linesFit(float64(r.Y1-r.Y0), face)), "\n")
	for i := range ls {
		ls[i] = fitWidth(ls[i], w, face)
	}
	ui.Print(frame, strings.Join(ls, "\n"), face, float64(r.X0+padX), float64(r.Y0+padY), c)
}

// notch is a wheel notch's scroll of n rows, wheel up toward the top.
func notch(wy float64, n int) int {
	if wy > 0 {
		return -n
	}
	return n
}

// keyScroll is off moved by the arrows, Page Up/Down or Home; the caller
// clamps.
func keyScroll(off, page int) int {
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowUp):
		return off - 1
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowDown):
		return off + 1
	case inpututil.IsKeyJustPressed(ebiten.KeyPageUp):
		return off - page
	case inpututil.IsKeyJustPressed(ebiten.KeyPageDown):
		return off + page
	case inpututil.IsKeyJustPressed(ebiten.KeyHome):
		return 0
	}
	return off
}

// listIndex is the index row r shows in a list of n scrolled by off, −1
// none.
func listIndex(off, r, n int) int {
	if i := off + r; r >= 0 && i < n {
		return i
	}
	return -1
}
