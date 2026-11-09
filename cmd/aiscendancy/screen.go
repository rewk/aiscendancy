package main

import "github.com/hajimehoshi/ebiten/v2"

// screen is one display on the app's stack. Enter runs once, on push,
// and draws no sprite (preload did): tests push screens on an app with
// no assets. Update runs while on top. Render paints the pictures, then
// Text writes over them, both on the same 1280x960 frame.
type screen interface {
	Enter(a *app)
	Update(a *app) error
	Render(a *app, sc *ebiten.Image)
	Text(a *app, frame *ebiten.Image)
}
