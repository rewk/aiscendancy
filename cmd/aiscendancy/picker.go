package main

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The item picker: a popup over the planet screen listing every item
// research allows at a clicked square or orbit slot (planetScreen.choices),
// the refused ones faded, no scrolling: 6x4 tiles over the whole screen
// for a square (at most 22 items), 3x3 right of the orbit column for a
// slot (at most 9). The header gives the spot and the free population,
// the footer the hovered choice's facts or refusal. A click on a
// buildable choice starts it (queues it while enqueuing: queue.go) and
// closes the popup (Ship opens a blank design screen instead); outside
// the box, a right click or Esc close it. The surface variant (the
// project panel's, placing.go) lists every surface building on the 6x4
// grid, judged on the planet's free squares; a choice turns the
// placement mode on.
// The planet screen's hover is frozen meanwhile: the project goes where
// the popup opened.

// The popup's boxes, native px: a 44 px header, a 92 px footer (verdict,
// then pickHelpLines of help), the body between in gridColW columns.
// pickFull (a square's): the screen inset 8, body 808, 6x4 cells of
// surfaceGridH. pickOrbit (a slot's): 3x3 of orbitalGridH, right of the
// orbit column (ends x 308), centred in the big area's height.
var (
	pickFull  = rect{8, 8, 1272, 952}
	pickOrbit = rect{320, 268, 954, 924}
)

// Tiles at zoom (surface 68x52 orig, orbital 70x70), each in a pickTileW
// area 12 px (surface, 120 high) or 8 px (orbital, 140 high) under its
// cell's top, the name 8 px under it (15 px smallFace line). Native px.
const (
	pickHeadH     = 44
	pickHelpLines = 4
	pickFootH     = 32 + pickHelpLines*15
	gridColW      = 210
	surfaceGridH  = 200 // 4 rows: 800 of 808
	orbitalGridH  = 172 // 3 rows: 516 of 520
	pickTileW     = 140
	surfaceGridTH = 120
	orbitalGridTH = 140
)

type picker struct {
	title    string // the header, e.g. "Build at orbit slot 2  free 3"
	orbit    bool   // an orbit slot's choices
	surface  bool   // the surface variant: a choice turns the placement mode on
	cs       []choice
	industry int    // the colony's industry a day at open time
	hover    int    // the choice under the cursor, -1
	msg      string // the footer: the hovered choice's facts or its refusal
	planet   *game.Planet
}

// built reports choice c a Unique bonus structure the planet holds, as
// planned once the queue is through while enqueuing (planetScreen.plan):
// drawn blue, still buildable, a second adding nothing.
func (p *picker) built(c choice) bool {
	return p.planet != nil && c.item.Unique() && p.planet.Count(c.item) > 0
}

func newPicker(s *planetScreen, cs []choice) *picker {
	p := &picker{cs: cs, hover: -1, orbit: !s.cellOK, planet: s.plan()}
	p.header(s)
	return p
}

// header sets the title ("Queue at" while enqueuing) and the industry
// the days are counted at.
func (p *picker) header(s *planetScreen) {
	verb := "Build"
	if s.enqueuing() {
		verb = "Queue"
	}
	switch {
	case p.surface:
		p.title = verb + " on the surface"
	case s.cellOK:
		p.title = fmt.Sprintf("%s at (%d,%d) %s square", verb, s.cell.I, s.cell.J, s.planet.Surface[s.cell].Kind)
	default:
		p.title = fmt.Sprintf("%s at orbit slot %d", verb, s.slot)
	}
	if s.col != nil { // the popup may hide the POP panel: the header says it
		p.industry, _, _ = s.col.Output()
		p.title += fmt.Sprintf("  free %d", s.col.Free())
	}
}

// refresh re-takes the verdicts and the header (the colony may change
// under the popup); with nothing buildable left it closes the picker and
// reports false.
func (p *picker) refresh(a *app, s *planetScreen) bool {
	if p.surface {
		p.cs = s.surfaceChoices(a)
	} else {
		p.cs = s.choices(a)
	}
	if !buildable(p.cs) {
		s.pick = nil
		return false
	}
	p.header(s)
	if p.hover >= len(p.cs) {
		p.hover = -1
	}
	return true
}

// box is the popup's box; bodyY and footY bound its body.
func (p *picker) box() rect {
	if p.orbit {
		return pickOrbit
	}
	return pickFull
}

func (p *picker) bodyY() int { return p.box().Y0 + pickHeadH }
func (p *picker) footY() int { return p.box().Y1 - pickFootH }

// cols and rowH are the grid's columns and row height.
func (p *picker) cols() int { return (p.box().X1 - p.box().X0) / gridColW }

func (p *picker) rowH() int {
	if p.orbit {
		return orbitalGridH
	}
	return surfaceGridH
}

// cell is choice i's cell, native px.
func (p *picker) cell(i int) rect {
	n, h := p.cols(), p.rowH()
	x, y := p.box().X0+gridColW*(i%n), p.bodyY()+h*(i/n)
	return rect{x, y, x + gridColW, y + h}
}

// tileArea is the area choice i's tile is centred in.
func (p *picker) tileArea(i int) rect {
	c := p.cell(i)
	x, y, h := (c.X0+c.X1-pickTileW)/2, c.Y0+12, surfaceGridTH
	if p.orbit {
		y, h = c.Y0+8, orbitalGridTH
	}
	return rect{x, y, x + pickTileW, y + h}
}

// at is the choice at native (x,y), -1 for none.
func (p *picker) at(x, y float64) int {
	b := p.box()
	if !b.has(x, y) || y < float64(p.bodyY()) || y >= float64(p.footY()) {
		return -1
	}
	cx, ry := int(x)-b.X0, int(y)-p.bodyY()
	n := p.cols()
	if cx/gridColW >= n {
		return -1
	}
	if i := (ry/p.rowH())*n + cx/gridColW; i < len(p.cs) {
		return i
	}
	return -1
}

// hoverMsg is the footer for the hovered choice: its facts, its refusal.
func (p *picker) hoverMsg() string {
	if p.hover < 0 {
		return ""
	}
	if c := p.cs[p.hover]; c.err != nil {
		return c.err.Error()
	}
	return factsLine(p.cs[p.hover].item, p.industry)
}

// choose acts on a click on choice i: a buildable one starts at the
// screen's hover (the surface variant: turns the placement mode on with
// it) and closes the picker; a refusal stays in the footer.
func (p *picker) choose(a *app, s *planetScreen, i int) error {
	c := p.cs[i]
	if c.err == nil && c.item == game.Ship { // the ship builder, at this colony
		s.pick = nil
		a.push(newDesignScreenFor(s.col, freshTemplate()))
		return nil
	}
	if c.err == nil && p.surface {
		s.placeItem, s.pick = c.item, nil
		return nil
	}
	err := c.err
	if err == nil {
		err = s.start(a, c.item)
	}
	if err != nil {
		p.msg = err.Error()
		return err
	}
	s.pick = nil
	return nil
}

// update reads the mouse while the picker is up; it takes no keys.
func (p *picker) update(a *app, s *planetScreen) error {
	if !p.refresh(a, s) {
		return nil
	}
	x, y := a.mouse()
	if h := p.at(x, y); h != p.hover { // the footer keeps a refusal until the hover moves
		p.hover = h
		p.msg = p.hoverMsg()
	}
	switch {
	case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight):
		s.pick = nil
	case !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
	case p.hover >= 0:
		p.choose(a, s, p.hover) // its error is in the footer
	case !p.box().has(x, y):
		s.pick = nil
	}
	return nil
}

// draw paints the box, its separators, the tiles (refused ones faded)
// and the hovered choice's outline (yellow buildable, red refused).
func (p *picker) draw(a *app, sc *ebiten.Image) {
	b := p.box()
	fillRect(sc, b)
	strokeRect(sc, b, textColour)
	line := dim(textColour, 0.5)
	w := float32(b.X1 - b.X0 - 2*zoom)
	vector.FillRect(sc, float32(b.X0+zoom), float32(p.bodyY()-zoom), w, zoom, line, false)
	vector.FillRect(sc, float32(b.X0+zoom), float32(p.footY()), w, zoom, line, false)
	body := clipped(sc, rect{b.X0, p.bodyY(), b.X1, p.footY()})
	for i, c := range p.cs {
		sp := pickerSprite(a, c.item)
		if sp.Image == nil {
			continue
		}
		// Placed by the frame's box, not its origin (surface origin
		// (34,18), orbital centred); scaled down when larger than the area.
		t := p.tileArea(i)
		ib := sp.Image.Bounds()
		tw, th := float64(t.X1-t.X0), float64(t.Y1-t.Y0)
		k := min(zoom, tw/float64(ib.Dx()), th/float64(ib.Dy()))
		left := float64(t.X0) + (tw-float64(ib.Dx())*k)/2
		top := float64(t.Y0) + (th-float64(ib.Dy())*k)/2
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(left-float64(sp.Offset.X)*k, top-float64(sp.Offset.Y)*k)
		if k < zoom {
			op.Filter = ebiten.FilterLinear
		}
		if c.err != nil {
			op.ColorScale.ScaleAlpha(0.35)
		} else if p.built(c) {
			op.ColorScale.Scale(bombedTint[0], bombedTint[1], bombedTint[2], 1)
		}
		sp.Draw(body, &op)
	}
	if p.hover >= 0 {
		strokeRect(body, p.cell(p.hover), okColour(p.cs[p.hover].err == nil))
	}
}

// pickerSprite is item it's picture: its planitem frame, but the Ship's
// (planitem 23 repeats the Orbital Missile Base) is the player's Medium
// hull (smshipNN frame 1, 104x74: scaled down).
func pickerSprite(a *app, it game.Item) *assets.Sprite {
	if it == game.Ship && a.e != nil {
		return a.spr(shipBank(a.e.Species), int(game.Medium))
	}
	return a.spr("planitem", int(it))
}

// strokeRect outlines r 1 original px (zoom native) inside its edges.
func strokeRect(sc *ebiten.Image, r rect, c color.RGBA) {
	const h = zoom / 2.0 // the line's centre, inside the edge
	vector.StrokeRect(sc, float32(r.X0)+h, float32(r.Y0)+h, float32(r.X1-r.X0-zoom), float32(r.Y1-r.Y0-zoom), zoom, c, false)
}

// text writes the title, the names under the tiles (refused ones dimmed)
// and the footer.
func (p *picker) text(a *app, frame *ebiten.Image) {
	b := p.box()
	ui.Print(frame, p.title, bigFace, float64(b.X0+padX), float64(b.Y0+padY), textColour)
	for i, c := range p.cs {
		col := textColour
		if c.err != nil {
			col = dim(textColour, 0.5)
		}
		r := p.cell(i)
		name := fitWidth(c.item.String(), float64(r.X1-r.X0-8), smallFace)
		x := float64(r.X0+r.X1)/2 - ui.Width(name, smallFace)/2
		ui.Print(frame, name, smallFace, x, float64(p.tileArea(i).Y1+8), col)
	}
	w := float64(b.X1 - b.X0 - 2*padX)
	ui.Print(frame, fitWidth(p.msg, w, smallFace), smallFace, float64(b.X0+padX), float64(p.footY()+6), textColour)
	ui.Print(frame, strings.Join(p.helpLines(a), "\n"), smallFace, float64(b.X0+padX), p.helpTop(), textColour)
}

// helpTop is the native y of the footer's help, a line under the verdict.
func (p *picker) helpTop() float64 { return float64(p.footY()+6) + smallH }

// helpLines is the hovered choice's help paragraph wrapped to the box,
// at most pickHelpLines that fit; none without a hover.
func (p *picker) helpLines(a *app) []string {
	if p.hover < 0 || p.hover >= len(p.cs) {
		return nil
	}
	b := p.box()
	n := min(pickHelpLines, fitLines(p.helpTop(), float64(b.Y1-padY)))
	return helpLines(a.itemHelp(p.cs[p.hover].item), float64(b.X1-b.X0-2*padX), n)
}

// itemFacts are it's non-zero numbers on two lines: its outputs and the
// room it adds; its cost, the days at industry a day (at least 1), a
// weapon's or shield's flag.
func itemFacts(it game.Item, industry int) (effects, cost string) {
	if it == game.Ship { // planitem's cost 20 is not a ship's: its design's is
		return "", "cost: the design's (opens the ship builder)"
	}
	in := it.Info()
	var fs, cs []string
	add := func(label string, n int) {
		if n != 0 {
			fs = append(fs, fmt.Sprintf("%s +%d", label, n))
		}
	}
	add("ind", in.Industry)
	add("res", in.Research)
	add("pros", in.Prosp)
	add("room", in.MaxPop)
	if in.Cost != 0 {
		ind := max(1, industry)
		cs = append(cs, fmt.Sprintf("cost %d", in.Cost), fmt.Sprintf("%d days", (in.Cost+ind-1)/ind))
	}
	if it.Has(game.FlagWeapon) {
		cs = append(cs, "weapon")
	}
	if it.Has(game.FlagShield) {
		cs = append(cs, "shield")
	}
	return strings.Join(fs, "  "), strings.Join(cs, "  ")
}

// factsLine is itemFacts on one line, for the footer.
func factsLine(it game.Item, industry int) string {
	e, c := itemFacts(it, industry)
	if e == "" {
		return c
	}
	return e + "  " + c
}
