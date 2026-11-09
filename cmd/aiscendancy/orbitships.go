package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The planet screen's orbit: the ships in orbit the player sees (its
// own, then Game.ShipsSeen's) fill the free orbit slots in slot order, a
// view mapping only (the model gives a ship no slot; extras not drawn).
// A click on one of the player's opens its dialog in the structure
// dialog's box.

// orbitShip is one ship drawn in a slot: own nil for a sighted foreign one.
type orbitShip struct {
	slot    int
	own     *game.Starship
	empire  int
	species game.Species
	hull    game.Hull
}

// freeSlots are the slots with no structure, rising orbital project nor
// queued orbital structure.
func (s *planetScreen) freeSlots() []int { return freeSlotsOf(s.planet, s.col) }

// freeSlotsOf is freeSlots for p and the player's colony c (nil none).
func freeSlotsOf(p *game.Planet, c *game.Colony) []int {
	var out []int
	for k, it := range p.Orbit {
		if it != game.NoItem {
			continue
		}
		if c != nil && c.Project != nil && c.Project.Orbital && buildsInPlace(c.Project) && c.Project.Slot == k {
			continue
		}
		if c != nil && slices.ContainsFunc(c.Queue, func(q game.Queued) bool { return q.Orbital && q.Slot == k }) {
			continue
		}
		out = append(out, k)
	}
	return out
}

// orbitShips are the ships drawn in the orbit; none without a game.
func (s *planetScreen) orbitShips(a *app) []orbitShip {
	return orbitShipsAt(a, s.star, s.idx, s.planet, s.col)
}

// orbitShipsAt is orbitShips for planet idx of star (p, colony c or nil).
func orbitShipsAt(a *app, star, idx int, p *game.Planet, c *game.Colony) []orbitShip {
	if a.g == nil {
		return nil
	}
	var out []orbitShip
	var ids []*game.Starship // out's ships, the slots' memory key (a foreign one never read)
	for _, sh := range a.e.Ships {
		if inOrbitOf(sh, star, idx) {
			out = append(out, orbitShip{own: sh, empire: a.player, species: a.e.Species, hull: sh.Design.Hull})
			ids = append(ids, sh)
		}
	}
	for _, sg := range a.g.ShipsSeen(a.e, star) {
		if sg.Orbit == idx {
			out = append(out, orbitShip{empire: sg.Empire, species: a.g.Empires[sg.Empire].Species, hull: sg.Hull})
			k := slices.IndexFunc(a.g.Empires[sg.Empire].Ships, sg.Of)
			ids = append(ids, a.g.Empires[sg.Empire].Ships[k])
		}
	}
	free := freeSlotsOf(p, c)
	if !a.e.SeesOrbit(p) { // its structures unseen: any slot takes a ship
		free = free[:0]
		for k := range game.OrbitSlots {
			free = append(free, k)
		}
	}
	return a.placeOrbit(out, ids, free, orbitSpot{star, idx, 0})
}

// orbitSpot is a ship's slot in the orbit of planet idx of star.
type orbitSpot struct{ star, idx, slot int }

// placeOrbit gives out[i] (ship ids[i]) a slot of free: the one it was
// last drawn in at the same planet (at's star and idx) while still free,
// then, in order, the first free one left; those past the free slots are
// dropped. The slots given are remembered (a.orbitSlots).
func (a *app) placeOrbit(out []orbitShip, ids []*game.Starship, free []int, at orbitSpot) []orbitShip {
	if a.orbitSlots == nil {
		a.orbitSlots = map[*game.Starship]orbitSpot{}
	}
	taken := map[int]bool{}
	slot := make([]int, len(out))
	for i, id := range ids {
		slot[i] = -1
		if o, ok := a.orbitSlots[id]; ok && o.star == at.star && o.idx == at.idx && slices.Contains(free, o.slot) && !taken[o.slot] {
			slot[i], taken[o.slot] = o.slot, true
		}
	}
	kept := out[:0]
	for i := range out {
		if slot[i] < 0 {
			k := slices.IndexFunc(free, func(f int) bool { return !taken[f] })
			if k < 0 {
				continue
			}
			slot[i], taken[free[k]] = free[k], true
		}
		at.slot = slot[i]
		a.orbitSlots[ids[i]] = at
		out[i].slot = slot[i]
		kept = append(kept, out[i])
	}
	return kept
}

// inOrbitOf reports sh is in the orbit of planet pl of star.
func inOrbitOf(sh *game.Starship, star, pl int) bool {
	return sh.Star == star && sh.Lane < 0 && sh.Orbit == pl
}

// shipIn is the ship drawn in orbit slot k.
func (s *planetScreen) shipIn(a *app, k int) (orbitShip, bool) {
	for _, o := range s.orbitShips(a) {
		if o.slot == k {
			return o, true
		}
	}
	return orbitShip{}, false
}

// ownHere reports sh is still the player's, in this orbit.
func (s *planetScreen) ownHere(a *app, sh *game.Starship) bool {
	return a.g != nil && slices.Contains(a.e.Ships, sh) && inOrbitOf(sh, s.star, s.idx)
}

// clickShip is a click on slot k holding o: the player's ship's dialog.
func (s *planetScreen) clickShip(o orbitShip) {
	if o.own != nil {
		s.openShipDlg(o.own)
	}
}

// Text button sizes (the species screen's): a bigFace label plus
// shipBtnPad a side, shipBtnH high, shipBtnGap apart; native px.
const (
	shipBtnH   = 40
	shipBtnPad = 8
	shipBtnGap = 8
)

// The ship dialog's nebula, native px from dlgBox's top-left: its inner
// edges (the bar's x, original px 160 and 480), the title's bottom
// (dlgTitleY plus a 20 px bigFace line), the bar's top. Right: the parts
// grid, dlgGridCols² cells (25, the Enormous hull's slots) at the design
// list's pitch, tiles at scale 1 (dlgGridW includes the last column's
// cell), centred between title and bar. Left: the picture fitted to a
// dlgPicW band (dlgShipPicH at most, top on the grid's), the order line
// dlgOrderGap under it wrapped at dlgOrderCols (smallFace: 7.2 px a
// char). dlgGap separates edge, picture, grid and edge.
const (
	dlgInnerX0, dlgInnerX1 = (160 - 152) * zoom, (480 - 152) * zoom
	dlgTitleBottom         = dlgTitleY + 20
	dlgBarTop              = 217 * zoom
	dlgGridCols            = 5
	dlgGridW, dlgGridH     = (dlgGridCols-1)*listPitchX + listCellW, dlgGridCols * listPitchY
	dlgPicW                = 160
	dlgGap                 = (dlgInnerX1 - dlgInnerX0 - dlgGridW - dlgPicW) / 3
	dlgPicX0, dlgPicX1     = dlgInnerX0 + dlgGap, dlgInnerX0 + dlgGap + dlgPicW
	dlgGridX               = dlgPicX1 + dlgGap
	dlgGridY               = dlgTitleBottom + (dlgBarTop-dlgTitleBottom-dlgGridH)/2
	dlgShipPicH            = 80 * zoom
	dlgOrderGap            = 4 * zoom
	dlgOrderCols           = (dlgPicX1 - dlgPicX0) * 10 / 72
)

// dlgCell is a parts-grid cell: a part aboard or a free hull slot.
type dlgCell struct {
	r    rect
	g    game.Gizmo
	part bool
}

// dlgGridCell is the grid's cell k (row by row), in native px.
func dlgGridCell(k int) rect {
	x := dlgBox.X0 + dlgGridX + listPitchX*(k%dlgGridCols)
	y := dlgBox.Y0 + dlgGridY + listPitchY*(k/dlgGridCols)
	return rect{x, y, x + listCellW, y + listCellH}
}

// shipDlgCells are Starship.Aboard in order, then the free slots.
func shipDlgCells(sh *game.Starship) []dlgCell {
	parts := sh.Aboard().Parts
	n := min(sh.Design.Hull.Slots(), dlgGridCols*dlgGridCols)
	out := make([]dlgCell, n)
	for k := range out {
		out[k].r = dlgGridCell(k)
		if k < len(parts) {
			out[k].g, out[k].part = parts[k], true
		}
	}
	return out
}

// dlgGridAt is the part under native (x,y) in d's grid, false off one.
func dlgGridAt(d *structDlg, x, y float64) (game.Gizmo, bool) {
	if d == nil || d.ship == nil {
		return 0, false
	}
	for _, c := range shipDlgCells(d.ship) {
		if c.part && c.r.has(x, y) {
			return c.g, true
		}
	}
	return 0, false
}

// drawShipDlg paints the ship dialog's nebula: the picture, the order
// line, the grid's tiles as drawListTile does. With the cursor on a part
// the left band shows its name and help paragraph instead, if any.
func (s *planetScreen) drawShipDlg(a *app, frame *ebiten.Image, sh *game.Starship) {
	b := dlgBox
	cx := float64(b.X0 + (dlgPicX0+dlgPicX1)/2)
	mx, my := a.mouse()
	var help []string
	if g, ok := dlgGridAt(s.dlg, mx, my); ok {
		help = shipDlgHelpLines(a, g)
	}
	dots, top := berthDots(sh)
	if dots != nil { // the help's lines stop over the Aboard row
		help = help[:min(len(help), fitLines(dlgHelpBandY0, float64(top)))]
	}
	if len(help) > 0 {
		ui.Print(frame, strings.Join(help, "\n"), smallFace, float64(b.X0+dlgPicX0), float64(b.Y0+dlgHelpBandY0), textColour)
	} else {
		drawShipFit(frame, a.spr(shipBank(a.e.Species), int(sh.Design.Hull)), cx, float64(b.Y0+dlgGridY+dlgShipPicH/2), float64(dlgPicW), dlgShipPicH)
		y := float64(b.Y0 + dlgGridY + dlgShipPicH + dlgOrderGap)
		for _, l := range strings.Split(wrap(s.dlgOrderLine(a), dlgOrderCols), "\n") {
			l = fitWidth(l, float64(dlgPicX1-dlgPicX0), smallFace)
			ui.Print(frame, l, smallFace, cx-ui.Width(l, smallFace)/2, y, textColour)
			y += smallFace.Size * 1.25
		}
	}
	if dots != nil {
		l := fmt.Sprintf("Aboard %d/%d", sh.Pop, sh.Berths())
		ui.Print(frame, l, smallFace, cx-ui.Width(l, smallFace)/2, float64(b.Y0+top), textColour)
		for _, d := range dots {
			x, y := float32(b.X0)+d.x, float32(b.Y0)+d.y
			if d.full {
				vector.FillCircle(frame, x, y, berthR, textColour, true)
			} else {
				vector.StrokeCircle(frame, x, y, berthR-zoom/2, zoom, dim(textColour, 0.8), true)
			}
		}
	}
	for _, c := range shipDlgCells(sh) {
		if c.part {
			a.spr(gizmoBank, int(c.g)).DrawAt(frame, float64(c.r.X0+listOX), float64(c.r.Y0+listOY))
		} else {
			a.spr(desBank, emptySlot).DrawAt(frame, float64(c.r.X0+c.r.X1)/2, float64(c.r.Y0+c.r.Y1)/2)
		}
	}
}

// The Aboard row of a ship with Space Habitats, its bottom at
// berthBottom (the nebula's inner edge: dlgBarTop is the bar frame's
// top): a smallFace label, then a circle per berth (filled a passenger,
// hollow empty) berthPitch apart, the module's berths (Special1)
// grouped berthGroupGap apart, wrapped by whole modules.
const (
	berthR        = 3 * zoom
	berthPitch    = 8 * zoom
	berthGroupGap = 4 * zoom
	berthBottom   = dlgBarTop - 10*zoom
)

// berthDot is one berth's centre, box-relative native px.
type berthDot struct {
	x, y float32
	full bool
}

// berthDots are sh's berths in order, the first Pop full, and the row's
// top (its label's, box-relative); nil without a Space Habitat.
func berthDots(sh *game.Starship) ([]berthDot, int) {
	per, n := game.Gizmos[game.SpaceHabitat].Special1, sh.Berths()
	if n < 1 || per < 1 {
		return nil, berthBottom
	}
	group := per*berthPitch + berthGroupGap
	perLine := max(1, (dlgPicW+berthGroupGap)/group)
	modules := (n + per - 1) / per
	lines := (modules + perLine - 1) / perLine
	top := berthBottom - lines*berthPitch - int(smallH)
	out := make([]berthDot, n)
	for i := range out {
		m := i / per
		line, col := m/perLine, m%perLine
		inLine := min(perLine, modules-line*perLine) // modules on this line, centred
		x0 := (dlgPicX0+dlgPicX1)/2 - (inLine*group-berthGroupGap)/2
		x := x0 + col*group + (i%per)*berthPitch + berthPitch/2
		y := top + int(smallH) + line*berthPitch + berthPitch/2
		out[i] = berthDot{float32(x), float32(y), i < sh.Pop}
	}
	return out, top
}

// The help band: dlgPicX0..dlgPicX1 between title and bar, padY inside.
const dlgHelpBandY0, dlgHelpBandY1 = dlgTitleBottom + padY, dlgBarTop - padY

// shipDlgHelpLines is part g's name and paragraph wrapped to the band.
func shipDlgHelpLines(a *app, g game.Gizmo) []string {
	t := a.gizmoHelp(g)
	if t == "" {
		return nil
	}
	w, n := float64(dlgPicX1-dlgPicX0), fitLines(dlgHelpBandY0, dlgHelpBandY1)
	ls := helpLines(g.String(), w, n)
	return append(ls, helpLines(t, w, n-len(ls))...)
}

// openShipDlg opens sh's dialog, dropping a pending Create Colony.
func (s *planetScreen) openShipDlg(sh *game.Starship) {
	s.settler = nil
	s.dlg = &structDlg{ship: sh}
}

// shipDlgActs are the ship dialog's buttons in bar order: Leave Orbit
// unless refitted; ONE middle button, the first the rule accepts now:
// Refit (atDocks), Create Colony (CanFound), Invade (invasion modules
// over a foreign planet: greyed while CanInvade refuses, dlgOff),
// Abandon unless refitted; Load and Unload for Space Habitats over a
// colony of ours (greyed while refused); Show and Cancel always.
func (s *planetScreen) shipDlgActs(a *app, sh *game.Starship) []dlgAct {
	refitted := a.e.Refitting(sh) != nil
	var out []dlgAct
	if !refitted {
		out = append(out, dlgLeaveOrbit)
	}
	switch {
	case atDocks(a.e, sh) != nil:
		out = append(out, dlgRefit)
	case a.e.CanFound(sh) == nil:
		out = append(out, dlgCreateColony)
	case sh.Invaders > 0 && foreignOrbit(a.e, sh):
		out = append(out, dlgInvade)
	case !refitted:
		out = append(out, dlgAbandon)
	}
	if sh.Berths() > 0 && orbitColony(a.e, sh) != nil {
		out = append(out, dlgLoad, dlgUnload)
	}
	return append(out, dlgShow, dlgCancel)
}

// foreignOrbit reports sh orbits a planet another empire holds.
func foreignOrbit(e *game.Empire, sh *game.Starship) bool {
	p := e.Planet(sh.Star, sh.Orbit)
	return p.Owner >= 0 && p.Owner != e.Index
}

// atDocks is e's colony with Orbital Docks sh orbits when sh is not
// being refitted, nil else: Refit opens the design screen's refit mode
// there (what can be refitted is that screen's verdict, CanRefit), so
// Abandon is not offered at the Docks.
func atDocks(e *game.Empire, sh *game.Starship) *game.Colony {
	c := orbitColony(e, sh)
	if c == nil || !c.Planet.HasDocks() || e.Refitting(sh) != nil {
		return nil
	}
	return c
}

// pressShip is button k of sh's dialog: a refusal keeps the box, a
// success closes it but Load's and Unload's (one unit a press: the
// player presses again; an Unload's freed worker may start the queue's
// head, next); Show returns the read-only design screen to push,
// Refit the refit mode's on sh's parts (with a project running, the
// cancel question first: its yes opens it, answerCancel); Abandon asks
// first (askScrap).
func (s *planetScreen) pressShip(a *app, sh *game.Starship, k dlgAct) (screen, error) {
	var err error
	switch k {
	case dlgShow:
		s.dlg = nil
		return newShipViewScreen(sh), nil
	case dlgLeaveOrbit:
		err = s.leave(a, sh)
	case dlgCreateColony: // the square is the next click's (foundHere)
		if err = a.e.CanFound(sh); err == nil {
			s.settler = sh
		}
	case dlgInvade:
		if s.dlgOff(a, k) { // greyed: its hover says why
			return nil, nil
		}
		var reps []game.Report
		if reps, err = a.g.InvadeNow(a.e, sh); err == nil { // resolved now: captured or repelled
			a.record(reps)
			s.msg, s.msgUntil = reps[0].Event.Text, s.tick+120
			s.refresh(a)
		}
	case dlgRefit:
		c := atDocks(a.e, sh)
		if c == nil {
			err = fmt.Errorf("%s is at no Orbital Docks of ours", sh.Name)
			break
		}
		s.dlg = nil
		if c == s.col && c.Running() { // the cancel question first; a yes opens the screen
			s.askCancel()
			s.refitAsk = sh
			return nil, nil
		}
		return newRefitScreen(c, sh), nil
	case dlgAbandon: // a refusal keeps the box; else the question, its yes scraps it (answerScrap)
		if err = a.e.CanAbandon(sh); err == nil {
			s.askScrap(sh)
		}
	case dlgLoad, dlgUnload:
		if s.dlgOff(a, k) { // greyed: its hover says why
			return nil, nil
		}
		if k == dlgLoad {
			err = a.e.Load(sh)
		} else if err = a.e.Unload(sh); err == nil {
			s.next(a)
		}
		if err == nil {
			s.refresh(a) // the box stays: drawPop and the Aboard row read the counts live
		}
		return nil, err
	}
	if err == nil {
		s.dlg = nil
	}
	return nil, err
}

// leave is Leave Orbit for sh: out now and free (Empire.LeaveOrbit),
// then the selected ship of the system screen under it, its planet no
// longer selected (the fire mode would shoot from the planet).
func (s *planetScreen) leave(a *app, sh *game.Starship) error {
	if err := a.e.LeaveOrbit(sh, a.g.Obstacles(sh)); err != nil {
		return err
	}
	if n := len(a.stack); n >= 2 {
		if sys, ok := a.stack[n-2].(*systemScreen); ok && sys.star == s.star {
			sys.sel, sys.psel, sys.move, sys.placing = sh, nil, moveMode{}, true
		}
	}
	s.refresh(a)
	return nil
}

// shipDlgText is the status line over button k of sh's dialog: what the
// press does, not its outcome.
func (s *planetScreen) shipDlgText(a *app, sh *game.Starship, k dlgAct) string {
	switch k {
	case dlgCreateColony:
		return fmt.Sprintf("Create Colony: %s founds a colony now, on the square you click next", sh.Name)
	case dlgInvade:
		if err := a.e.CanInvade(sh); err != nil {
			return "Invade: " + err.Error()
		}
		return fmt.Sprintf("Invade: %s lands now, the fewest of its %d invasion module%s that beat the shields", sh.Name, sh.Invaders, plural(sh.Invaders))
	case dlgRefit:
		return refitFoot(a.e, sh)
	case dlgLeaveOrbit:
		return fmt.Sprintf("Leave Orbit: %s leaves the orbit now, to be moved on the system screen", sh.Name)
	case dlgAbandon:
		return fmt.Sprintf("Abandon: %s is scrapped now, nothing back", sh.Name)
	case dlgShow:
		return fmt.Sprintf("Show: %s's parts, read-only", sh.Name)
	case dlgLoad:
		if err := a.e.CanLoad(sh); err != nil {
			return "Load: " + err.Error()
		}
		return "Load: one colonist boards " + sh.Name
	case dlgUnload:
		if err := a.e.CanUnload(sh); err != nil {
			return "Unload: " + err.Error()
		}
		return "Unload: one colonist lands on " + s.planet.Name
	}
	return ""
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// shipLine is sh's order line (shipOrder), "in orbit" without one.
func shipLine(a *app, sh *game.Starship) string {
	if o := shipOrder(a.e, sh); o != "" {
		return o
	}
	return "in orbit"
}

// slotShipText is the status line over a slot holding o.
func (s *planetScreen) slotShipText(a *app, o orbitShip) string {
	if o.own == nil {
		return fmt.Sprintf("orbit %d - %s ship, %s hull", o.slot, o.species, o.hull)
	}
	return fmt.Sprintf("orbit %d - %s (%s) (click: its dialog)", o.slot, o.own.Name, o.own.Design.Name)
}

// drawOrbitShips paints each orbit ship's hull fitted to its slot.
func (s *planetScreen) drawOrbitShips(a *app, sc *ebiten.Image) {
	for _, o := range s.orbitShips(a) {
		x, y := slotCentre(o.slot)
		sp := a.spr(shipBank(o.species), int(o.hull))
		if sp.Image != nil {
			b := sp.Image.Bounds()
			w, h := float64(b.Dx()), float64(b.Dy())
			k := min(zoom, float64(slotSize-8*zoom)/max(w, h))
			var op ebiten.DrawImageOptions
			op.GeoM.Scale(k, k)
			op.GeoM.Translate(x-k*(float64(sp.Offset.X)+w/2), y-k*(float64(sp.Offset.Y)+h/2))
			op.Filter = ebiten.FilterLinear
			sp.Draw(sc, &op)
		}
	}
}

// drawShipFit paints sp centred at (cx,cy), fitted in w x h, ≤ zoom.
func drawShipFit(dst *ebiten.Image, sp *assets.Sprite, cx, cy, w, h float64) {
	if sp.Image == nil {
		return
	}
	b := sp.Image.Bounds()
	bw, bh := float64(b.Dx()), float64(b.Dy())
	k := min(zoom, w/bw, h/bh)
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(cx-k*(float64(sp.Offset.X)+bw/2), cy-k*(float64(sp.Offset.Y)+bh/2))
	op.Filter = ebiten.FilterLinear
	sp.Draw(dst, &op)
}
