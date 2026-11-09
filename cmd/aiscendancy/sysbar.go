package main

import "github.com/hajimehoshi/ebiten/v2"

// The system display's top-right panel: 16battle.shp frame 10 (174x142,
// origin (0,0)) drawn at (466,0) in the corner the chrome leaves open.
// Its top strip [466,640)x[0,55) is an opaque starfield under the title;
// below, five holes measured on the frame: three 44x31 buttons, frames 16
// (End Turn), 17 (Repeat Move), 18 (Auto), origin (0,0); two 70x31 holes
// under them (the original's ship filters, frames 7-9), ours the lists'
// tabs (drawFilters). Original px (orig).
var (
	sysPanel   = orig(466, 0, 640, 142)
	sysStrip   = orig(466, 0, 640, 55) // the title's room
	sysEndTurn = orig(479, 55, 523, 86)
	sysRepeat  = orig(531, 55, 575, 86)
	sysAuto    = orig(583, 55, 627, 86)
	sysFilters = [2]rect{orig(479, 103, 549, 134), orig(557, 103, 627, 134)} // planetsTab, shipsTab
	// stripIcon holds the star's suns.shp frame, shrunk, the controller's
	// flag on its top-left corner. The strip's opaque part is orig [4,44)
	// high (at 7 the star overflowed the frame).
	stripIcon = orig(474, 8, 500, 40)
)

// stripTextX is where the title's lines start: past the star's picture.
func stripTextX() float64 { return float64(stripIcon.X1 + 4*zoom) }

const (
	battleBank   = "16battle"
	panelFrame   = 10
	endTurnFrame = 16
	repeatFrame  = 17
	autoFrame    = 18
)

// The panel's buttons, numbered after starmap's enum: no two buttons of
// the app share a value.
const (
	endTurnButton = designsButton + 1 + iota
	repeatButton
	autoButton
	planetsFilter // sysFilters[planetsTab]
	shipsFilter   // sysFilters[shipsTab]
)

// sysButton is the panel button under (x,y), noButton none.
func sysButton(x, y float64) int {
	switch {
	case sysEndTurn.has(x, y):
		return endTurnButton
	case sysRepeat.has(x, y):
		return repeatButton
	case sysAuto.has(x, y):
		return autoButton
	case sysFilters[planetsTab].has(x, y):
		return planetsFilter
	case sysFilters[shipsTab].has(x, y):
		return shipsFilter
	}
	return noButton
}

// drawSysPanel paints the panel and the three buttons, the hovered one
// framed yellow (the filters are drawFilters').
func drawSysPanel(a *app, sc *ebiten.Image, hover int) {
	drawSprite(sc, a.spr(battleBank, panelFrame), float64(sysPanel.X0), float64(sysPanel.Y0))
	buttons := []struct {
		r     rect
		frame int
		b     int
	}{{sysEndTurn, endTurnFrame, endTurnButton}, {sysRepeat, repeatFrame, repeatButton}, {sysAuto, autoFrame, autoButton}}
	for _, b := range buttons {
		drawSprite(sc, a.spr(battleBank, b.frame), float64(b.r.X0), float64(b.r.Y0))
		if b.b == hover {
			strokeRect(sc, b.r, yellow)
		}
	}
}

// drawStrip paints the star in stripIcon and the controller's flag on
// its corner, when the player knows the star.
func (s *systemScreen) drawStrip(a *app, sc *ebiten.Image) {
	drawMini(sc, a.spr("suns", int(s.sys.Star)), stripIcon)
	if s.g == nil || !knows(a.e, s.star) {
		return
	}
	if i := s.sys.Controller(); i >= 0 {
		a.flag(s.g.Empires[i].Species, colourOf(s.g, i)).DrawAt(sc, float64(stripIcon.X0), float64(stripIcon.Y0))
	}
}
