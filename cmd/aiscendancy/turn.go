package main

import "github.com/hajimehoshi/ebiten/v2"

// The turn box: the starmap chrome's day counter (a button toggling the
// log) and two turn buttons, on every map-canvas screen.

// turnButton is the turn button under (x,y): nextButton, skipButton,
// logButton (the counter) or noButton.
func turnButton(x, y float64) int {
	switch {
	case mapNext.has(x, y):
		return nextButton
	case mapSkip.has(x, y):
		return skipButton
	case mapTop.has(x, y):
		return logButton
	}
	return noButton
}

// pressTurn acts on a turn button: a day, or days until an event, then
// report, the day's end warning first (askDay); or the log toggled.
// false for any other button.
func (a *app) pressTurn(b int) bool {
	switch b {
	case nextButton:
		a.askDay(a.nextDay)
	case skipButton:
		a.askDay(a.skipDays)
	case logButton:
		a.toggleLog() // never quits: the log is never the last screen
	default:
		return false
	}
	return true
}

// drawTurnBox paints the day counter and the turn buttons in their
// holes.
func drawTurnBox(a *app, sc *ebiten.Image) {
	drawSprite(sc, a.spr(mapBank, dayBox), float64(mapTop.X0), float64(mapTop.Y0))
	for k, f := range dayFrames(a.e.Day) {
		drawSprite(sc, a.spr(mapBank, f), float64(mapTop.X0+zoom*(2+digitW*k)), float64(mapTop.Y0))
	}
	drawSprite(sc, a.spr(mapBank, nextFrame), float64(mapNext.X0), float64(mapNext.Y0))
	drawSprite(sc, a.spr(mapBank, skipFrame), float64(mapSkip.X0), float64(mapSkip.Y0))
}
