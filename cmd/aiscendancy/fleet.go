package main

import (
	"fmt"
	"image/color"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The fleet screen: a three-line row per ship in Ships order, its sprite
// at its left in a small scene (fleetScene), then a row per ship being
// built (fleetBuilds). A click opens the ship's system (the star it
// left, in a lane) with it selected, a build's colony. Row r shows entry
// off+r, looked up at every use: no index is kept. Stacked boxes: the
// help, the count and cap, the Designs button (a blank design screen
// building nowhere: templates).
type fleetScreen struct {
	off   int // rows scrolled: the top row shows Ships[off]
	hover int // row under the cursor, −1 none
	btn   int // fleetButton under the cursor
	news  newsView
}

func newFleetScreen() *fleetScreen { return &fleetScreen{hover: -1, news: newNewsView()} }

func (s *fleetScreen) Enter(*app) {}

const fleetHelp = "wheel/arrows: scroll  S: species\nclick: go to the ship\nEsc: back"

// Row geometry, native px: a row is three bigFace lines plus
// fleetRowGap, the lines centred in it. Left to right, fleetGap apart:
// the icon cell (fleetIconW: the ship fitted in fleetShipW x fleetShipH
// and its scene), the text column (name / where / order), the design's
// five mini gauges (fleetMiniW, a letter and a bar each, the three
// lines' height), then fleetGaugeW at the right margin: the hull and
// energy gauges (drawBar's label and numbers included) over the
// consumables left (Design.Consumables: a small icon and its count each).
const (
	fleetShipW, fleetShipH = 64, 45
	fleetIconW             = fleetShipW + 60
	fleetCellPad           = 1   // the cell's clip inside its row band, top and bottom
	fleetBubbleAlpha       = 0.6 // the lane scene's exit bubble, the ship seen through it
	fleetRowGap            = 10  // 13 rows of 70 in the 920 px under fleetTop
	fleetGap               = 8
	fleetLines             = 3
	fleetGaugeW            = 260
	fleetMiniW             = 110
)

// fleetRowH is a row's height, for row(), Text and Render alike.
var fleetRowH = fleetLines*logLineH + fleetRowGap

// fleetTop is row 0's native y, at the map area's top as the planets
// list's.
func fleetTop() float64 { return logTop() }

func fleetBox() rect { return rect{mapArea.X0, int(fleetTop()), mapArea.X1, mapArea.Y1} }

func fleetRows() int { return int((float64(mapArea.Y1-padY) - fleetTop()) / fleetRowH) }

// fleetRowY0 is row r's top; its text lines start fleetRowGap/2 under.
func fleetRowY0(r int) float64 { return fleetTop() + float64(r)*fleetRowH }

func fleetLineY(r, line int) float64 {
	return fleetRowY0(r) + fleetRowGap/2 + float64(line)*logLineH
}

// The text column's faces: the name in fleetNameFace, the place and
// order in fleetSubFace; the three lines fit fleetLines*logLineH.
var fleetNameFace, fleetSubFace = discNameFace, discTextFace

func fleetFace(line int) *text.GoTextFace {
	if line == 0 {
		return fleetNameFace
	}
	return fleetSubFace
}

// fleetTextY is row r's text line top: the name's line, then the others
// a fleetSubFace line apart.
func fleetTextY(r, line int) float64 {
	y := fleetLineY(r, 0)
	if line > 0 {
		y += fleetNameFace.Size*1.25 + float64(line-1)*fleetSubFace.Size*1.25
	}
	return y
}

func fleetIconX() float64 { return float64(mapArea.X0 + padX) }

// fleetTextX is the text column's left edge, fleetTextW its width.
func fleetTextX() float64 { return fleetIconX() + fleetIconW + fleetGap }

func fleetTextW() float64 { return fleetMiniX() - fleetGap - fleetTextX() }

func fleetMiniX() float64 { return fleetGaugeX() - 2*fleetGap - fleetMiniW }

func fleetGaugeX() float64 { return float64(mapArea.X1-padX) - fleetGaugeW }

// row is the row under native y, −1 outside the rows.
func (s *fleetScreen) row(y float64) int {
	if y < float64(mapArea.Y0) || y >= float64(mapArea.Y1) {
		return -1
	}
	r := y - fleetTop()
	if r < 0 {
		return -1
	}
	if k := int(r / fleetRowH); k < fleetRows() {
		return k
	}
	return -1
}

// fleetBuilds are e's colonies building a ship, in Colonies order (the
// entries after the ships).
func fleetBuilds(e *game.Empire) []*game.Colony {
	var out []*game.Colony
	for _, c := range e.Colonies {
		if c.Project != nil && c.Project.Item == game.Ship {
			out = append(out, c)
		}
	}
	return out
}

// fleetEntries is the list's length: the ships, then the builds.
func fleetEntries(e *game.Empire) int { return len(e.Ships) + len(fleetBuilds(e)) }

// ship is the entry row r shows, −1 when none: a Ships index below
// len(Ships), else len(Ships) plus a fleetBuilds index.
func (s *fleetScreen) ship(a *app, r int) int { return listIndex(s.off, r, fleetEntries(a.e)) }

// build is entry i's colony when i is a build, nil for a ship.
func build(e *game.Empire, i int) *game.Colony {
	if i < len(e.Ships) {
		return nil
	}
	return fleetBuilds(e)[i-len(e.Ships)]
}

func (s *fleetScreen) lineAt(a *app, y float64) int { return s.ship(a, s.row(y)) }

func (s *fleetScreen) scroll(a *app, d int) {
	s.off = min(max(s.off+d, 0), max(0, fleetEntries(a.e)-fleetRows()))
}

// open is what a click on entry i pushes: a ship's system, the ship
// selected; for a ship in orbit (hidden there) its planet's screen
// over it, the ship's dialog open; a build's planet over its system.
func (s *fleetScreen) open(a *app, i int) []screen {
	if c := build(a.e, i); c != nil {
		return []screen{newSystemScreen(c.Star), newPlanetScreen(c.Star, c.PlanetIdx)}
	}
	sh := a.e.Ships[i]
	if sh.Lane < 0 && sh.Orbit >= 0 {
		ps := newPlanetScreen(sh.Star, sh.Orbit)
		ps.openShipDlg(sh)
		return []screen{newSystemScreen(sh.Star), ps}
	}
	return []screen{newSystemScreenAt(sh.Star, sh)}
}

// whereText is where sh is: "orbit <planet>", "lane to <star>" or
// "space at <star>".
func whereText(e *game.Empire, sh *game.Starship) string {
	cl := e.Cluster
	switch {
	case sh.Lane >= 0:
		return "lane to " + e.StarLabel(cl.Other(sh.Lane, sh.Star))
	case sh.Orbit >= 0:
		return "orbit " + e.Planet(sh.Star, sh.Orbit).Name
	}
	return "space at " + e.StarLabel(sh.Star)
}

// orderText is sh's order with its ETA when it has one ("-> <planet>,
// N days"), "colonise", invadeText or "idle"; in a lane, whose star the
// where column names, the days left alone ("arrival N days"): the
// order's "-> <star>, " would cut them off the column. A refit's hold
// comes first: "being refitted, N days" (projectDays).
func orderText(e *game.Empire, sh *game.Starship) string {
	if c := e.Refitting(sh); c != nil {
		return "being refitted, " + projectDays(c)
	}
	cl := e.Cluster
	eta := ""
	if days, ok := sh.ETA(cl, e.Day); ok {
		if sh.Lane >= 0 {
			return "arrival " + etaText(days)
		}
		eta = ", " + etaText(days)
	}
	switch o := sh.Order; o.Kind {
	case game.GoToSystem:
		return navOrder(e, sh)
	case game.Colonise:
		return "colonise"
	case game.Invade:
		return invadeText(e, sh)
	case game.GoToPlanet:
		return "-> " + e.Planet(sh.Star, o.Planet).Name + eta
	case game.GoToStar:
		return "-> " + e.StarLabel(o.Star) + eta
	case game.GoToPoint:
		return "-> point" + eta
	}
	return "idle"
}

// projectDays is c's project's days left at today's industry ("N days",
// at least 1: it completes at a day's close), "no industry", "blockaded".
func projectDays(c *game.Colony) string {
	d, ok := c.DaysLeft()
	switch {
	case c.Blockaded():
		return "blockaded"
	case !ok:
		return "no industry"
	}
	return dayCount(max(1, d))
}

// shipLines are a ship's three lines: its name, whereText, orderText.
func shipLines(e *game.Empire, sh *game.Starship) [fleetLines]string {
	return [fleetLines]string{sh.Name, whereText(e, sh), orderText(e, sh)}
}

// buildLines are a build's three lines: its ship's name (the class when
// Spawn will name it), "yard <planet>", "being built, N days".
func buildLines(c *game.Colony) [fleetLines]string {
	pj := c.Project
	name := pj.ShipName
	if name == "" {
		name = pj.Design.Name
	}
	return [fleetLines]string{name, "yard " + c.Planet.Name, "being built, " + projectDays(c)}
}

// fleetCounts is box 1's text: the ships, the builds when any, the cap
// (which counts both).
func fleetCounts(e *game.Empire) string {
	n, word := len(e.Ships), "ships"
	if n == 1 {
		word = "ship"
	}
	line := fmt.Sprintf("%d %s", n, word)
	if b := len(fleetBuilds(e)); b > 0 {
		line += fmt.Sprintf(", %d building", b)
	}
	return fmt.Sprintf("%s\ncap %d", line, game.ShipsPerSystem*e.ControlledSystems())
}

// fleetButton is the button under (x, y): turnButton's, designsButton
// in stacked box 2, else noButton.
func fleetButton(x, y float64) int {
	if b := turnButton(x, y); b != noButton {
		return b
	}
	if mapStack[2].has(x, y) {
		return designsButton
	}
	return noButton
}

// press acts on button b: the turn box's, or Designs.
func (s *fleetScreen) press(a *app, b int) {
	if a.pressTurn(b) {
		return
	}
	if b == designsButton {
		a.push(newDesignScreen())
	}
}

func (s *fleetScreen) Update(a *app) error {
	x, y := a.mouse()
	_, wy := ebiten.Wheel()
	press := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	wheeled, pressed := s.news.input(a, x, y, wy, press)
	if wy != 0 && !wheeled {
		s.scroll(a, notch(wy, 3))
	}
	s.off = keyScroll(s.off, fleetRows())
	s.scroll(a, 0) // ships may have gone since
	if onSpeciesKey(a, inpututil.IsKeyJustPressed(ebiten.KeyS)) {
		return nil
	}
	s.btn = fleetButton(x, y)
	if pressed {
		return nil
	}
	s.hover = -1
	if mapArea.has(x, y) {
		s.hover = s.row(y)
	}
	if press || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		if i := s.ship(a, s.hover); i >= 0 {
			for _, sc := range s.open(a, i) {
				a.push(sc)
			}
		}
	}
	if press {
		s.press(a, s.btn)
	}
	return nil
}

// sceneKind is where an entry's ship is, as its icon cell shows it.
type sceneKind int

const (
	sceneSpace sceneKind = iota // in a system's space: a quarter of its star, top-left
	sceneOrbit                  // in orbit: half its planet's globe, left
	sceneLane                   // in a lane: half its exit sphere, right, the ship entering it
	sceneBuild                  // being built: the Shipyard at the left, the ship night-blue
)

// fleetScene is an entry's cell backdrop: its kind and the sprite drawn
// at x1 with the ship.
type fleetScene struct {
	kind  sceneKind
	bank  string
	frame int
}

// sceneOf is entry i's scene, read off the ship's Lane, Orbit and Star
// in whereText's order (a lane before an orbit). A lane's sphere is its
// exit bank's frame 0, the flare's start, closest to lanes.shp's at
// rest; a globe is the system screen's planets.shp frame.
func sceneOf(e *game.Empire, i int) fleetScene {
	if build(e, i) != nil {
		return fleetScene{sceneBuild, "planitem", int(game.Shipyard)}
	}
	sh, cl := e.Ships[i], e.Cluster
	switch {
	case sh.Lane >= 0:
		if cl.Lanes[sh.Lane].Red {
			return fleetScene{sceneLane, redExitBank, 0}
		}
		return fleetScene{sceneLane, blueExitBank, 0}
	case sh.Orbit >= 0:
		p := e.Planet(sh.Star, sh.Orbit)
		return fleetScene{sceneOrbit, "planets", int(p.Type)*game.MaxSize + p.Size - 1}
	}
	return fleetScene{sceneSpace, "suns", int(cl.Stars[sh.Star].System.Star)}
}

// fleetCell is row r's icon cell, the scene's clip: fleetIconW wide,
// the row's band less fleetCellPad, so two rows' scenes never touch.
func fleetCell(r int) rect {
	x0, y0 := int(fleetIconX()), fleetRowY0(r)
	return rect{x0, int(math.Ceil(y0)) + fleetCellPad, x0 + fleetIconW, int(y0+fleetRowH) - fleetCellPad}
}

// drawScene paints entry i's cell: the backdrop at x1 (its origin is
// its centre in every bank used), the ship fitted in fleetShipW x
// fleetShipH at the cell's right, or at the sphere's rim in a lane, the
// sphere drawn over its half.
func (s *fleetScreen) drawScene(a *app, dst *ebiten.Image, r, i int) {
	cr := fleetCell(r)
	cell := clipped(dst, cr)
	x0, x1, y0 := float64(cr.X0), float64(cr.X1), float64(cr.Y0)
	cy := float64(cr.Y0+cr.Y1) / 2
	sc := sceneOf(a.e, i)
	back := a.spr(sc.bank, sc.frame)
	shipX := x1 - fleetShipW/2
	if c := build(a.e, i); c != nil {
		if back.Image != nil {
			back.DrawAt(cell, x0+float64(back.Image.Bounds().Dx())/2, cy)
		}
		drawProjectPic(a, cell, c, shipX, cy, fleetShipW, fleetShipH)
		return
	}
	ship := a.spr(shipBank(a.e.Species), int(a.e.Ships[i].Design.Hull))
	switch sc.kind {
	case sceneSpace:
		back.DrawAt(cell, x0, y0)
	case sceneOrbit:
		back.DrawAt(cell, x0, cy)
	case sceneLane:
		if back.Image != nil {
			shipX = x1 - float64(back.Image.Bounds().Dx())/2
		}
		drawShipFit(cell, ship, shipX, cy, fleetShipW, fleetShipH)
		var op ebiten.DrawImageOptions // the bubble see-through: the ship's nose shows inside
		op.GeoM.Translate(x1, cy)
		op.ColorScale.ScaleAlpha(fleetBubbleAlpha)
		back.Draw(cell, &op)
		return
	}
	drawShipFit(cell, ship, shipX, cy, fleetShipW, fleetShipH)
}

// Render paints the chrome, then each row's icon cell (drawScene).
func (s *fleetScreen) Render(a *app, sc *ebiten.Image) {
	drawMapChrome(a, sc, 3)
	drawNewsBox(sc, newsBox)
	dst := clipped(sc, mapArea)
	for r := range fleetRows() {
		i := s.ship(a, r)
		if i < 0 {
			break
		}
		s.drawScene(a, dst, r, i)
	}
	drawScrollCues(a, sc, fleetBox(), s.off, fleetRows(), fleetEntries(a.e))
}

// Text writes each row's lines (each cut), its design's mini gauges, a
// ship's hull and energy gauges on the first two lines, the consumables
// (a ship's left, a build's design's) on the third, the boxes, the news.
func (s *fleetScreen) Text(a *app, frame *ebiten.Image) {
	if fleetEntries(a.e) == 0 {
		ui.Print(frame, "no ships yet", bigFace, fleetTextX(), fleetLineY(0, 0), textColour)
	}
	for r := range fleetRows() {
		i := s.ship(a, r)
		if i < 0 {
			break
		}
		c := textColour
		if r == s.hover {
			c = yellow
		}
		var lines [fleetLines]string
		var sh *game.Starship
		var d, left game.Design
		if b := build(a.e, i); b != nil {
			lines, d = buildLines(b), *b.Project.Design
			left = d
		} else {
			sh = a.e.Ships[i]
			lines, d, left = shipLines(a.e, sh), sh.Design, sh.Aboard()
		}
		for line, str := range lines {
			f := fleetFace(line)
			ui.Print(frame, fitWidth(str, fleetTextW(), f), f, fleetTextX(), fleetTextY(r, line), c)
		}
		y0 := fleetLineY(r, 0)
		for k, b := range designBars(d) {
			drawGaugeMini(frame, b, gaugeLetters[k], fleetMiniX(), y0+float64(k)*gaugePitch, fleetMiniW)
		}
		a.drawConsumables(frame, left.Consumables(), fleetGaugeX(), fleetLineY(r, fleetLines-1), fleetGaugeW)
		if sh == nil {
			continue
		}
		// drawBar centres its gauge in a 8*zoom slot: centred on the line.
		for line, b := range shipBars(sh) {
			drawBar(frame, b, fleetGaugeX(), fleetLineY(r, line)+(logLineH-8*zoom)/2, fleetGaugeW)
		}
	}
	putBox(frame, mapStack[0], fleetHelp, smallFace, textColour)
	putBox(frame, mapStack[1], fleetCounts(a.e), bigFace, textColour)
	designs := textColour
	if s.btn == designsButton {
		designs = yellow
	}
	putBox(frame, mapStack[2], "Designs", bigFace, designs)
	s.news.text(a, frame)
}

// consIconW is a consumable's icon box, native px: gizmos.shp's tile
// shrunk to a line's height.
const consIconW = 28

// drawConsumables draws pcs left to right from (x, y), a line high: each
// gizmo's tile fitted in consIconW x logLineH, its count after; those
// past w dropped.
func (a *app) drawConsumables(frame *ebiten.Image, pcs []game.PartCount, x, y, w float64) {
	x1 := x + w
	for _, pc := range pcs {
		n := strconv.Itoa(pc.N)
		step := consIconW + 2 + ui.Width(n, bigFace)
		if x+step > x1 {
			return
		}
		fitSprite(frame, a.spr(gizmoBank, int(pc.Gizmo)), x+consIconW/2, y+logLineH/2, consIconW, logLineH)
		ui.Print(frame, n, bigFace, x+consIconW+2, y, textColour)
		x += step + fleetGap
	}
}

// The mini gauges: designBars' five, one letter each (weapon, shield,
// drive, radar for the scanners, energy), stacked gaugePitch apart over
// the row's three lines, each bar gaugeBarH tall.
var (
	gaugeLetters = [5]string{"W", "S", "D", "R", "E"}
	gaugePitch   = fleetLines * logLineH / 5
)

const (
	gaugeBarH    = 8
	gaugeLetterW = 14
)

// drawGaugeMini is drawBar shrunk, no number: b's letter, then its gauge
// w − gaugeLetterW wide in a 1 px outline, filled to frac, graduated,
// open and fading at the right past 1.
func drawGaugeMini(frame *ebiten.Image, b bar, letter string, x, y, w float64) {
	ui.Print(frame, letter, smallFace, x, y+(gaugePitch-smallFace.Size*1.2)/2, dim(textColour, 0.8))
	gx, gw, gy := x+gaugeLetterW, w-gaugeLetterW, y+(gaugePitch-gaugeBarH)/2
	vector.FillRect(frame, float32(gx), float32(gy), float32(gw), gaugeBarH, color.RGBA{0, 0, 0, 200}, false)
	in := gw - 2
	if b.frac > 0 {
		vector.FillRect(frame, float32(gx+1), float32(gy+1), float32(in*min(b.frac, 1)), gaugeBarH-2, b.c, false)
	}
	for f := b.tick; b.tick > 0 && f < 1; f += b.tick {
		c := color.RGBA{60, 60, 60, 255}
		if f < b.frac {
			c = lighten(b.c, 0.35)
		}
		vector.FillRect(frame, float32(math.Round(gx+1+in*f)), float32(gy+1), 1, gaugeBarH-2, c, false)
	}
	edge := dim(textColour, 0.5)
	if b.frac <= 1 {
		strokeRect1(frame, gx, gy, gw, gaugeBarH, edge)
		return
	}
	x0, x1, y0, y1 := float32(gx), float32(gx+gw), float32(gy), float32(gy+gaugeBarH)
	vector.FillRect(frame, x0, y0, x1-x0, 1, edge, false)
	vector.FillRect(frame, x0, y1-1, x1-x0, 1, edge, false)
	vector.FillRect(frame, x0, y0, 1, y1-y0, edge, false)
	for i := range barBreak / 2 {
		vector.FillRect(frame, x1-1+float32(i), y0+1, 1, y1-y0-2, dim(b.c, 1-float64(i+1)/(barBreak/2+1)), false)
	}
}

// strokeRect1 outlines the w x h box at (x, y) in 1 px lines.
func strokeRect1(frame *ebiten.Image, x, y, w, h float64, c color.RGBA) {
	x0, y0, x1, y1 := float32(x), float32(y), float32(x+w), float32(y+h)
	vector.FillRect(frame, x0, y0, x1-x0, 1, c, false)
	vector.FillRect(frame, x0, y1-1, x1-x0, 1, c, false)
	vector.FillRect(frame, x0, y0, 1, y1-y0, c, false)
	vector.FillRect(frame, x1-1, y0, 1, y1-y0, c, false)
}
