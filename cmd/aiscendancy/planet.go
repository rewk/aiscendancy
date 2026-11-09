package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The planetary display, laid out as the original's colony screen
// (iso.go). A left click opens the picker (picker.go) on an empty spot,
// the structure dialog (structdlg.go) on a structure, the ship dialog
// (orbitships.go) on the player's ship; while a project runs or the queue
// waits they queue instead (queue.go), and a click on the project's
// target asks to cancel it. A click on the project panel places one
// building on square after square (placing.go). Uninhabited and foreign
// planets are read-only. A colony changing hands closes the popups.

var errColour = color.RGBA{255, 120, 100, 255}

// squareStyle is how one square kind is painted: the bonus squares
// tinted red, green, blue (not 17np's swatches).
type squareStyle struct {
	fill, outline color.RGBA // fill.A == 0: no fill
}

var squareStyles = [game.NumSquareKinds]squareStyle{
	game.Black:           {outline: color.RGBA{60, 60, 60, 255}},
	game.White:           {fill: color.RGBA{205, 205, 205, 255}, outline: color.RGBA{110, 110, 110, 255}},
	game.BonusIndustry:   {fill: color.RGBA{200, 60, 50, 255}, outline: color.RGBA{110, 30, 25, 255}},
	game.BonusResearch:   {fill: color.RGBA{60, 90, 220, 255}, outline: color.RGBA{30, 45, 120, 255}},
	game.BonusProsperity: {fill: color.RGBA{60, 170, 60, 255}, outline: color.RGBA{30, 90, 30, 255}},
}

const fillAlpha = 0.85 // lets the globe's texture show through the squares

var (
	yellow = color.RGBA{255, 220, 0, 255}
	red    = color.RGBA{220, 40, 40, 255}
	orange = color.RGBA{255, 140, 0, 255} // the click asks to cancel the project
)

// The project buttons, one per planet-wide project research allows: a
// column down the big area's right edge (a size-5 grid ends at x 1160,
// short of 1186), the planitem frame at scale 1. Native px.
const (
	projColX1, projColY0 = 1258, 238
	projBtnW, projBtnH   = 72, 56
	projBtnGap           = 2
)

// projButton is one project button: its item, the colony's verdict
// (CanStartProject), its cell; bombed: a used Lush Growth Bomb (blue).
type projButton struct {
	item   game.Item
	err    error
	r      rect
	bombed bool
}

// projButtons are the colony's project buttons, nil off its colony.
func (s *planetScreen) projButtons(a *app) []projButton {
	cs := s.projectChoices(a)
	bs := make([]projButton, len(cs))
	for i, c := range cs {
		y0 := projColY0 + i*(projBtnH+projBtnGap)
		bs[i] = projButton{c.item, c.err, rect{projColX1 - projBtnW, y0, projColX1, y0 + projBtnH},
			c.item == game.LushGrowthBomb && s.planet.Lush}
	}
	return bs
}

// projAt is the project button under native (x,y).
func (s *planetScreen) projAt(a *app, x, y float64) (projButton, bool) {
	for _, b := range s.projButtons(a) {
		if b.r.has(x, y) {
			return b, true
		}
	}
	return projButton{}, false
}

type planetScreen struct {
	star, idx int // the star in the cluster, the planet in its system
	planet    *game.Planet
	col       *game.Colony // the player's colony on planet, nil: read-only
	// other is another empire's colony on planet, read-only, unfogged but
	// for its cloakers (veiled); otherSp its owner's species.
	other   *game.Colony
	otherSp game.Species
	sysName string
	grid    *grid

	// veiled: the player does not see the surface (game.Empire.SeesSurface:
	// another's Surface Cloaker), orbitVeiled its orbit (SeesOrbit): no
	// structures drawn, other's detail hidden. Set by refresh.
	veiled, orbitVeiled bool

	cell    game.Cell // hovered square, valid when cellOK
	cellOK  bool
	slot    int // hovered orbit slot, valid when slotOK
	slotOK  bool
	projOn  game.Item     // the project button hovered, NoItem for none
	pick    *picker       // the item picker, nil when closed (hover frozen)
	dlg     *structDlg    // the structure dialog, nil when closed (hover frozen)
	confirm *prompt       // the project's cancel question, nil when closed
	doomed  *game.Project // the project confirm asks about
	// projAsk: confirm came from a project button (its yes leaves the
	// colony idle for it); nextDue: a yes cancelled, so the next Update
	// starts the queue's head (planetScreen.next).
	projAsk, nextDue bool
	// qOn is the queue stack's entry hovered, −1 none; qUp: on its ▼.
	// xOn: the project panel's cancel cross hovered.
	qOn      int
	qUp, xOn bool
	// refitAsk: confirm came from the ship dialog's Refit for that ship;
	// refitDue: its yes cancelled, so the next Update pushes the refit
	// screen on it (atDocks still accepting).
	refitAsk, refitDue *game.Starship
	// scrapAsk: confirm asks to abandon that ship (the ship dialog's
	// Abandon); its yes scraps it (answerScrap).
	scrapAsk *game.Starship
	msg      string // last build error, shown until msgUntil
	msgUntil int
	tick     int
	// settler is the ship Create Colony was pressed for: the next square
	// click founds the colony there (foundHere).
	settler *game.Starship
	// placeItem is the building the placement mode puts down, NoItem off;
	// panelOn: the project panel (its cancel cross aside) hovered.
	placeItem game.Item
	panelOn   bool
}

func newPlanetScreen(star, planet int) *planetScreen {
	return &planetScreen{star: star, idx: planet, projOn: game.NoItem, qOn: -1, placeItem: game.NoItem}
}

// cancel drops the settler, the question, the picker, the dialog or the
// placement mode on Esc instead of leaving the screen.
func (s *planetScreen) cancel() bool {
	if s.settler != nil {
		s.settler = nil
		return true
	}
	if s.confirm != nil {
		s.answerCancel(false)
		return true
	}
	if s.pick != nil {
		s.pick = nil
		return true
	}
	if s.dlg != nil {
		s.dlg = nil
		return true
	}
	if s.placing() {
		s.placeItem = game.NoItem
		return true
	}
	return false
}

func (s *planetScreen) Enter(a *app) {
	sys := a.cl.Stars[s.star].System
	s.planet, s.sysName = sys.Planets[s.idx], sys.Name
	s.grid = newGrid(s.planet.Size)
	s.refresh(a)
}

// refresh looks the planet's colonies up again: a change of the player's
// colony closes the popups; the dialog also closes once its subject is
// gone (dlgGone).
func (s *planetScreen) refresh(a *app) {
	prev := s.col
	s.col, s.other, s.veiled, s.orbitVeiled = nil, nil, false, false
	defer s.keepSettling(a)
	if a.g == nil { // tests may push screens on a bare cluster
		return
	}
	s.col = a.e.Colony(s.star, s.idx)
	if o := s.planet.Owner; o >= 0 && o != a.player && o < len(a.g.Empires) {
		s.other, s.otherSp = a.g.Empires[o].Colony(s.star, s.idx), a.g.Empires[o].Species
	}
	s.veiled, s.orbitVeiled = !a.e.SeesSurface(s.planet), !a.e.SeesOrbit(s.planet)
	if s.dlg != nil && s.dlg.ship == nil && (s.col != prev || s.col == nil) || s.dlgGone(a) {
		s.dlg = nil
	}
	if s.col != prev {
		s.pick, s.confirm, s.doomed, s.nextDue, s.projAsk = nil, nil, nil, false, false
		s.refitAsk, s.refitDue, s.scrapAsk, s.placeItem = nil, nil, nil, game.NoItem
	}
}

// keepSettling drops the settler once it left this orbit or the planet
// is the player's.
func (s *planetScreen) keepSettling(a *app) {
	if s.settler != nil && (s.col != nil || !s.ownHere(a, s.settler)) {
		s.settler = nil
	}
}

// foundHere founds the colony with the settler on the hovered square.
func (s *planetScreen) foundHere(a *app) error {
	if err := a.e.FoundAt(s.settler, s.cell); err != nil {
		return err
	}
	a.reported(s.star, s.idx, len(a.e.Colonies))
	s.settler = nil
	s.refresh(a)
	return nil
}

// readOnly is why the player cannot build here, "" on their colony.
func (s *planetScreen) readOnly() string {
	switch {
	case s.col != nil:
		return ""
	case s.planet.Owner < 0:
		return "uninhabited"
	}
	return "not your planet"
}

// onShipyard reports the hovered orbit slot holds a Shipyard.
func (s *planetScreen) onShipyard() bool {
	return s.slotOK && !s.orbitVeiled && s.planet.Orbit[s.slot] == game.Shipyard
}

// leftClick is a left click at the hover, without Ebiten: the settler's
// square, the placement mode's square or slot, the player's ship, the
// cancel cross, the queue stack, the project panel, a project button,
// else clickSlot.
func (s *planetScreen) leftClick(a *app) (screen, error) {
	o, onShip := s.hoveredShip(a)
	switch {
	case s.settler != nil && s.cellOK:
		return nil, s.foundHere(a)
	case s.placing() && (s.cellOK || s.slotOK):
		return nil, s.placeClick(a)
	case onShip && o.own != nil:
		s.clickShip(o)
		return nil, nil
	case s.xOn && s.cancelShown():
		s.askCancel()
		return nil, nil
	case s.qOn >= 0 && s.col != nil && s.qUp:
		return nil, s.col.QueueUp(s.qOn)
	case s.qOn >= 0 && s.col != nil:
		return nil, s.dequeue(a, s.qOn)
	case s.panelOn:
		return nil, s.clickPanel(a)
	case s.projOn != game.NoItem:
		return nil, s.clickProject(a, s.projOn)
	}
	return s.clickSlot(a)
}

// clickSlot is a left click on a square or slot, without Ebiten: the
// project's target asks to cancel, a queued spot is dequeued (its last
// entry), a structure opens its dialog, an empty spot the picker, else
// why nothing can start; off the player's colony the readOnly refusal.
func (s *planetScreen) clickSlot(a *app) (screen, error) {
	s.refresh(a)
	switch {
	case s.onShipyard() && s.col == nil:
		return nil, nil // another empire's Shipyard
	case !s.cellOK && !s.slotOK:
		return nil, nil
	case s.col == nil:
		return nil, errors.New(s.readOnly())
	case s.targeted():
		s.askCancel()
		return nil, nil
	case s.queuedAt() >= 0:
		return nil, s.dequeue(a, s.queuedAt())
	case s.onStructure():
		s.openDlg()
		return nil, nil
	}
	return nil, s.pickHere(a)
}

// pickHere opens the picker on the hovered spot, else says why nothing
// can start there.
func (s *planetScreen) pickHere(a *app) error {
	cs := s.choices(a)
	switch {
	case buildable(cs):
		s.pick = newPicker(s, cs)
		return nil
	case len(cs) > 0:
		return cs[0].err
	}
	return errors.New("nothing to build here yet")
}

// onStructure reports the hovered square or slot holds a structure.
func (s *planetScreen) onStructure() bool {
	switch {
	case s.cellOK:
		return s.planet.Surface[s.cell].Item != game.NoItem
	case s.slotOK:
		return s.planet.Orbit[s.slot] != game.NoItem
	}
	return false
}

// targeted reports the hovered square or slot is the running project's
// target (a planet-wide project and a refit have none).
func (s *planetScreen) targeted() bool {
	pj := s.col.Project
	switch {
	case !s.col.Running() || !buildsInPlace(pj):
		return false
	case pj.Orbital:
		return !s.cellOK && pj.Slot == s.slot
	}
	return s.cellOK && pj.Cell == s.cell
}

// choice is one item the picker lists, err its verdict (nil: can start).
type choice struct {
	item game.Item
	err  error
}

// choices are the items research allows at the hover, in Item order,
// with their verdicts (Offers on a square of the planned planet); nil off
// the player's colony or with nothing hovered.
func (s *planetScreen) choices(a *app) []choice {
	if s.col == nil || (!s.cellOK && !s.slotOK) {
		return nil
	}
	pl, enq := s.plan(), s.enqueuing()
	var cs []choice
	for it := game.Item(0); it < game.NumItems; it++ {
		if !a.e.Research.Allows(it) || it == game.ColonyBase { // never a project (Colony.common)
			continue
		}
		switch {
		case s.cellOK && pl.Offers(it, s.cell):
			cs = append(cs, choice{it, s.verdict(a, it)})
		case s.slotOK && it == game.Ship && !enq: // the fleet cap too, whatever the design
			cs = append(cs, choice{it, a.e.CanStartShip(s.col, s.slot)})
		case s.slotOK && it.Orbital(): // a Ship is never queued: CanEnqueueOrbital says so
			cs = append(cs, choice{it, s.verdict(a, it)})
		}
	}
	return cs
}

// verdict is the colony's verdict on it at the hovered square or slot,
// the CanEnqueue pair while enqueuing, else the CanStart pair.
func (s *planetScreen) verdict(a *app, it game.Item) error {
	r := a.e.Research
	switch enq := s.enqueuing(); {
	case s.cellOK:
		return s.cellVerdict(a, it, s.cell)
	case enq:
		return s.col.CanEnqueueOrbital(r, it, s.slot)
	}
	return s.col.CanStartOrbital(r, it, s.slot)
}

// cellVerdict is verdict on square c: CanEnqueue while enqueuing, else
// CanStart.
func (s *planetScreen) cellVerdict(a *app, it game.Item, c game.Cell) error {
	if s.enqueuing() {
		return s.col.CanEnqueue(a.e.Research, it, c)
	}
	return s.col.CanStart(a.e.Research, it, c)
}

// projectChoices are the planet-wide projects research allows, in Item
// order, each with CanStartProject's verdict; nil off the player's colony.
func (s *planetScreen) projectChoices(a *app) []choice {
	if s.col == nil {
		return nil
	}
	var cs []choice
	for it := game.FirstProject; it < game.NumItems; it++ {
		if it.PlanetWide() && a.e.Research.Allows(it) {
			cs = append(cs, choice{it, s.col.CanStartProject(a.e.Research, it)})
		}
	}
	return cs
}

// clickProject is a click on it's button: with a project running it
// asks to cancel, else starts it or returns the refusal.
func (s *planetScreen) clickProject(a *app, it game.Item) error {
	s.refresh(a)
	switch {
	case s.col == nil:
		return nil
	case s.col.Running() || s.running(it):
		s.askCancel()
		s.projAsk = true
		return nil
	}
	return s.col.StartProject(a.e.Research, it)
}

// buildable reports one of cs can start.
func buildable(cs []choice) bool {
	for _, c := range cs {
		if c.err == nil {
			return true
		}
	}
	return false
}

// start starts it at the hovered square or slot, or queues it while
// enqueuing; off the player's colony the readOnly refusal.
func (s *planetScreen) start(a *app, it game.Item) error {
	s.refresh(a)
	switch {
	case !s.cellOK && !s.slotOK:
		return nil
	case s.col == nil:
		return errors.New(s.readOnly())
	case s.cellOK:
		return s.startAt(a, it, s.cell)
	case s.enqueuing():
		return s.col.EnqueueOrbital(a.e.Research, it, s.slot)
	}
	return s.col.StartOrbital(a.e.Research, it, s.slot)
}

// startAt starts it at cell at, or queues it there while enqueuing.
func (s *planetScreen) startAt(a *app, it game.Item, at game.Cell) error {
	if s.enqueuing() {
		return s.col.Enqueue(a.e.Research, it, at)
	}
	return s.col.Start(a.e.Research, it, at)
}

// askCancel puts up the cancel question: what and where, the days left
// and what is sunk in it.
func (s *planetScreen) askCancel() {
	pj := s.col.Project
	where := fmt.Sprintf("Building %s at (%d,%d)", projectName(pj), pj.Cell.I, pj.Cell.J)
	switch {
	case pj.Refit != "": // no slot: the ship in orbit is the work
		where = projectName(pj)
	case pj.Orbital:
		where = fmt.Sprintf("Building %s in orbit slot %d", projectName(pj), pj.Slot)
	case pj.Item.PlanetWide(): // its button is lit
		where = "Building " + projectName(pj)
	}
	left := "no industry: it never ends"
	if d, ok := s.col.DaysLeft(); ok {
		left = fmt.Sprintf("%d days left (%d of %d industry done)", d, pj.Progress, pj.Cost())
	} else if s.col.Blockaded() {
		left = fmt.Sprintf("blockaded (%d of %d industry done)", pj.Progress, pj.Cost())
	} else if f, ok := ongoingFoot(s.col); ok {
		left = f
	}
	s.confirm = &prompt{[]string{where, left, "Cancel it? Its industry is lost."}, "Yes, cancel", "No (Esc)"}
	s.doomed, s.projAsk, s.refitAsk, s.scrapAsk = pj, false, nil, nil
}

// askScrap opens the question for the ship dialog's Abandon on sh.
func (s *planetScreen) askScrap(sh *game.Starship) {
	s.confirm = &prompt{[]string{"Abandon " + sh.Name + "?", "It is scrapped now, nothing comes back."}, "Yes, abandon", "No (Esc)"}
	s.doomed, s.projAsk, s.refitAsk, s.scrapAsk = nil, false, nil, sh
}

// answerScrap closes the abandon question: yes scraps the ship asked
// about (a refusal, colonists aboard, goes to the status line).
func (s *planetScreen) answerScrap(a *app, yes bool) {
	sh := s.scrapAsk
	s.answerCancel(false)
	if !yes {
		return
	}
	if err := a.e.Abandon(sh); err != nil {
		s.msg, s.msgUntil = err.Error(), s.tick+120
		return
	}
	s.refresh(a)
}

// answerCancel closes the question: yes cancels the project asked about
// unless it is over already (a day ran under the box), then the refit
// screen is due if asked from Refit, nothing from a project button, else
// the queue's head (nextDue).
func (s *planetScreen) answerCancel(yes bool) {
	if yes && s.col != nil && s.col.Project == s.doomed {
		s.col.Cancel()
		s.refitDue = s.refitAsk
		s.nextDue = s.refitAsk == nil && !s.projAsk
	}
	s.confirm, s.doomed, s.projAsk, s.refitAsk, s.scrapAsk = nil, nil, false, nil, nil
}

// open reports a square is drawn full and answers the mouse: a
// structure or Reachable on the planned planet (plan); the others are the
// original's markers. A bare planet is all markers, except while the
// settler picks its square.
func (s *planetScreen) open(c game.Cell) bool { return s.openIn(s.plan(), c) }

// openIn is open on planet pl (the plan, taken once by the caller).
func (s *planetScreen) openIn(pl *game.Planet, c game.Cell) bool {
	if !pl.HasStructure() {
		return s.settler != nil
	}
	return pl.Surface[c].Item != game.NoItem || pl.Reachable(c)
}

// canOpen reports the picker would open at the hover (clickSlot's rules).
func (s *planetScreen) canOpen(a *app) bool { return buildable(s.choices(a)) }

// hoverColour is the hover's outline: the settler's verdict, dim where
// read-only, orange on the project's target, hoverColor on a queued spot
// or a structure, else the picker's verdict.
func (s *planetScreen) hoverColour(a *app) color.RGBA {
	switch {
	case s.settler != nil && s.cellOK:
		return okColour(s.planet.CanSettle(s.cell))
	case s.col == nil:
		return dim(textColour, 0.5)
	case s.placing() && s.queuedAt() >= 0:
		return hoverColor
	case s.placing():
		return okColour(s.cellOK && s.placeErr(a) == nil)
	case s.targeted():
		return orange
	case s.queuedAt() >= 0 || s.onStructure():
		return hoverColor
	}
	return okColour(s.canOpen(a))
}

func globeBank(t game.PlanetType) string { return fmt.Sprintf("planal%02d", int(t)) }

func (s *planetScreen) Update(a *app) error {
	s.tick++
	s.refresh(a)
	if s.confirm != nil { // the question takes every input
		if ok, yes := s.confirm.update(a); ok {
			if s.scrapAsk != nil {
				s.answerScrap(a, yes)
			} else {
				s.answerCancel(yes)
			}
		}
		return nil
	}
	if s.nextDue {
		s.nextDue = false
		s.next(a)
	}
	if sh := s.refitDue; sh != nil {
		s.refitDue = nil
		if c := atDocks(a.e, sh); c != nil {
			a.push(newRefitScreen(c, sh))
		}
		return nil
	}
	if s.dlg != nil { // so does the dialog; the hover stays on its structure or ship
		return s.dlgUpdate(a)
	}
	if s.pick != nil { // the hover stays where the picker opened
		return s.pick.update(a, s)
	}
	if s.placing() && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		s.rightClick()
		return nil
	}
	if onSpeciesKey(a, keyJustPressed(ebiten.KeyS)) {
		return nil
	}
	x, y := a.mouse()
	s.hoverAt(a, x, y)
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	push, err := s.leftClick(a)
	if push != nil {
		a.push(push)
		return nil
	}
	if err != nil {
		s.msg, s.msgUntil = err.Error(), s.tick+60 // one second at 60 ticks
	}
	return nil
}

// hoverAt sets the hover from native (x,y), without Ebiten: a project
// button, the queue stack, the cancel cross, the project panel, else an
// orbit slot, else an open square.
func (s *planetScreen) hoverAt(a *app, x, y float64) {
	s.slot, s.slotOK = SlotAt(x, y)
	s.cell, s.cellOK = s.grid.CellAt(x, y)
	s.cellOK = s.cellOK && !s.slotOK && !s.veiled && s.open(s.cell)
	s.projOn, s.qOn, s.qUp, s.xOn, s.panelOn = game.NoItem, -1, false, false, false
	if b, ok := s.projAt(a, x, y); ok {
		s.projOn, s.cellOK, s.slotOK = b.item, false, false
	}
	if k, up := s.queueAt(x, y); k >= 0 {
		s.qOn, s.qUp, s.cellOK, s.slotOK = k, up, false, false
	}
	if s.cancelShown() && cancelRect.has(x, y) {
		s.xOn, s.cellOK, s.slotOK = true, false, false
	} else if s.col != nil && panels[4].has(x, y) {
		s.panelOn, s.cellOK, s.slotOK = true, false, false
	}
}

// hoveredShip is the ship drawn in the hovered orbit slot.
func (s *planetScreen) hoveredShip(a *app) (orbitShip, bool) {
	if !s.slotOK {
		return orbitShip{}, false
	}
	return s.shipIn(a, s.slot)
}

// Render paints back to front: starfield, globe, squares, structures,
// orbit, hover, then the chrome (trimming spills) and the panels.
func (s *planetScreen) Render(a *app, sc *ebiten.Image) {
	sc.Fill(color.Black)
	drawSprite(sc, a.spr("17np", 4), float64(planetArea.X0), float64(planetArea.Y0))
	s.drawGlobe(a, sc)

	cells, pl := s.planet.Cells(), s.plan()
	for _, c := range cells {
		x, y := s.grid.Centre(c)
		if s.veiled || !s.openIn(pl, c) { // a veiled surface: markers only
			drawMarker(sc, x, y, s.planet.Surface[c].Kind)
			continue
		}
		st := squareStyles[s.planet.Surface[c].Kind]
		if st.fill.A != 0 {
			fillDiamond(sc, x, y, st.fill, fillAlpha)
		}
		strokeDiamond(sc, x, y, st.outline, zoom)
	}
	for _, c := range cells {
		sq := s.planet.Surface[c]
		fr := int(sq.Item)
		switch {
		case s.veiled:
			continue
		case sq.Ruins && sq.Item == game.NoItem && !s.digging(c): // a running dig draws itself, below
			fr = ruinsFrame
		case sq.Item == game.NoItem:
			continue
		}
		x, y := s.grid.Centre(c)
		drawSprite(sc, a.spr("planitem", fr), x, y-halfH)
		// The original hides a structure's square colour: keep its outline.
		if st := squareStyles[sq.Kind]; st.fill.A != 0 && sq.Kind != game.White {
			strokeDiamond(sc, x, y, st.fill, zoom)
		}
	}
	for k, it := range s.planet.Orbit {
		if it != game.NoItem && !s.orbitVeiled {
			// Orbital frames are 70x70 with the origin at their centre.
			x, y := slotCentre(k)
			drawSprite(sc, a.spr("planitem", int(it)), x, y)
		}
	}
	s.drawOrbitShips(a, sc)

	if s.col != nil && s.col.Project != nil && buildsInPlace(s.col.Project) {
		pj := s.col.Project
		done := float64(pj.Progress) / float64(max(1, pj.Cost()))
		if pj.Orbital { // a ship rises as its hull (drawProjectPic)
			x, y := slotCentre(pj.Slot)
			drawProjectPic(a, sc, s.col, x, y, slotSize-4*zoom, slotSize-4*zoom)
			strokeSquare(sc, x, y, yellow, zoom)
		} else {
			x, y := s.grid.Centre(pj.Cell)
			drawBuilding(sc, a.spr("planitem", int(projItem(s.col, pj))), x, y-halfH, done)
			strokeDiamond(sc, x, y, yellow, zoom)
		}
	}
	s.drawGhosts(a, sc)
	s.drawPlaceGhost(a, sc)
	o, onShip := s.hoveredShip(a)
	switch {
	case s.cellOK:
		x, y := s.grid.Centre(s.cell)
		strokeDiamond(sc, x, y, s.hoverColour(a), 2*zoom)
	case onShip && o.own != nil:
		x, y := slotCentre(s.slot)
		strokeSquare(sc, x, y, hoverColor, 2*zoom)
	case onShip:
		x, y := slotCentre(s.slot)
		strokeSquare(sc, x, y, dim(textColour, 0.5), 2*zoom)
	case s.slotOK:
		x, y := slotCentre(s.slot)
		strokeSquare(sc, x, y, s.hoverColour(a), 2*zoom)
	}

	drawSprite(sc, a.spr("17np", 0), 0, 0)
	for i, v := range outputValues(s.colony()) {
		drawSprite(panelImage(sc, i), a.spr(outputBanks[i], outputFrame(v)), float64(panels[i].X0), float64(panels[i].Y0))
	}
	if d := shiftCue(s.colony()); d != 0 { // in the gap from industry's panel (1) to the other's
		g := panels[1].X0 + panels[0].X1
		if d > 0 {
			g = panels[1].X1 + panels[2].X0
		}
		drawShift(sc, float64(g)/2, float64(panels[1].Y0+panels[1].Y1)/2, shiftW, shiftH, d)
	}
	if c := s.colony(); c != nil && c.Singular() { // research's share to industry: the takeover's gap, pointing right
		drawShift(sc, float64(panels[1].X0+panels[0].X1)/2, float64(panels[1].Y0+panels[1].Y1)/2, shiftW, shiftH, 1)
	}
	if hospitable(s.colony()) { // its share leaves the planet: no triangle, its icon on the industry
		drawPlanitem(sc, a, game.AlienHospitality, panels[1].X0+hospInset, panels[1].Y0+hospInset)
	}
	s.drawPop(a, sc)
	s.drawProject(a, sc)
	s.drawPanelCue(sc)
	s.drawCancelCross(sc)
	s.drawProjButtons(a, sc)
	s.drawQueueStack(a, sc)
	if s.pick != nil {
		s.pick.draw(a, sc)
	}
}

// Text writes the panels' and the status line's text.
func (s *planetScreen) Text(a *app, frame *ebiten.Image) {
	if s.confirm != nil {
		defer func() {
			s.confirm.draw(a, frame)
			if s.col != nil && s.col.Project == s.doomed && s.doomed != nil {
				b := dlgBox
				drawProjectPic(a, frame, s.col, float64(b.X0+b.X1)/2, float64(b.Y0+dlgCancelPicY), float64(b.X1-b.X0-8*padX), dlgCancelPicH)
			}
		}()
	}
	if s.pick != nil && s.pick.box() == pickFull { // the popup covers the whole screen
		s.pick.text(a, frame)
		return
	}
	at := func(x, y int) (float64, float64) { return float64(x), float64(y) }
	pop, pp := s.panelTexts()
	x, y := at(panels[3].X0+zoom, panels[3].Y0+popRows*popPitch)
	ui.Print(frame, pop, bigFace, x, y, textColour)
	pp.text(frame)
	for i, v := range outputValues(s.colony()) {
		outputBadge(frame, v, panels[i].X1, panels[i].Y1)
	}
	s.queueText(frame)

	here, colour := "", textColour
	o, onShip := s.hoveredShip(a)
	switch {
	case s.dlg != nil && s.dlg.ship != nil: // the hovered button's line, else the hovered part's name
		mx, my := a.mouse()
		if k, ok := s.dlgAt(a, mx, my); ok {
			here = s.shipDlgText(a, s.dlg.ship, k)
		} else if g, ok := dlgGridAt(s.dlg, mx, my); ok {
			here = g.String()
		}
	case s.settler != nil && s.cellOK && s.planet.CanSettle(s.cell):
		here = s.squareText(a) + " - click: the Colony Base here"
	case s.settler != nil && s.cellOK:
		here = s.squareText(a) + " - no Colony Base here"
	case s.settler != nil && !onShip && s.projOn == game.NoItem:
		here = "Create Colony: click a square for the Colony Base (Esc: cancel)"
	case s.panelOn && s.placing():
		here = "click: stop placing " + s.placeItem.String()
	case s.panelOn:
		here = panelHoverLine
	case s.placing() && s.cellOK && s.queuedAt() < 0:
		here = s.placeText(a)
	case s.xOn:
		here = "click: cancel the project"
	case s.qOn >= 0 && s.col != nil && s.qOn < len(s.col.Queue):
		here = queueLine(s.col.Queue[s.qOn], s.qOn, s.qUp)
	case s.cellOK:
		here = s.squareText(a)
	case onShip:
		here = s.slotShipText(a, o)
	case s.slotOK && s.orbitVeiled:
		here = orbitCloakedLine
	case s.slotOK:
		here = fmt.Sprintf("orbit %d - %s", s.slot, itemName(s.planet.Orbit[s.slot]))
	case s.projOn != game.NoItem:
		here = s.projOn.String()
		if s.col == nil { // another's planet or none: read-only, no verdict
			break
		}
		if err := s.col.CanStartProject(a.e.Research, s.projOn); err != nil && (!s.col.Running() || s.col.Project.Item == s.projOn) {
			here += " - " + err.Error()
		}
	}
	switch {
	case here == "" || onShip || s.col == nil || s.dlg != nil:
	case s.placing() && (s.cellOK || s.slotOK) && s.queuedAt() < 0: // the mode's click, said above
	case s.targeted() || s.projOn != game.NoItem && (s.col.Running() || s.running(s.projOn)):
		here += " (click: cancel the project)"
	case s.queuedAt() >= 0:
		here += fmt.Sprintf(" (queue %d: click removes it)", s.queuedAt()+1)
	}
	if s.tick < s.msgUntil {
		here, colour = s.msg, errColour
	}
	if s.pick != nil { // the orbit grid's box: the panels show, not the status line under it
		s.pick.text(a, frame)
		return
	}
	if s.dlg != nil {
		s.drawDlg(a, frame)
	}
	// Starts past the orbit column, whose last slots reach the area's bottom.
	x, y = at(planetArea.X0+(planetArea.X1-planetArea.X0)/3, planetArea.Y1-12*zoom)
	ui.Print(frame, here, bigFace, x, y, colour)
}

// The output pictures (108x100, origin top-left): 21 frames per bank,
// one per 2 points of daily output, 20 the last (40 and up).
var outputBanks = [3]string{"planres", "planind", "planpro"}

// outputValues are p's daily outputs in outputBanks' order: its
// colony's Shown, nothing without a colony (the raw sums are only the
// rule's input).
func outputValues(c *game.Colony) [3]int {
	if c == nil {
		return [3]int{}
	}
	ind, res, pros := c.Shown()
	return [3]int{res, ind, pros}
}

// shiftCue is where c's ongoing project moves the industry: −1 to
// research's picture (left of industry's), 1 to prosperity's (right), 0
// nowhere shown (none, or Alien Hospitality's species).
func shiftCue(c *game.Colony) int {
	if c == nil {
		return 0
	}
	switch it, _ := c.Diverted(); it {
	case game.ScientistTakeover:
		return -1
	case game.EndlessParty:
		return 1
	}
	return 0
}

// hospitable reports c running Alien Hospitality, whose industry share
// goes to the species met (its icon on the industry
// picture, the industry shown kept).
func hospitable(c *game.Colony) bool {
	if c == nil {
		return false
	}
	it, _ := c.Diverted()
	return it == game.AlienHospitality
}

// hospInset is the hospitality icon's inset in the industry picture.
const hospInset = 4 * zoom

// drawPlanitem draws it's planitem frame at scale 1, as the project
// buttons, its top-left at (x, y).
func drawPlanitem(sc *ebiten.Image, a *app, it game.Item, x, y int) {
	sp := a.spr("planitem", int(it))
	if sp.Image == nil {
		return
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(float64(x-sp.Offset.X), float64(y-sp.Offset.Y))
	sp.Draw(sc, &op)
}

// drawShift draws the cue: a yellow triangle w x h, outlined dark,
// centred on (cx, cy), its apex toward dir (−1 left, 1 right).
func drawShift(dst *ebiten.Image, cx, cy, w, h float64, dir int) {
	tip, base := float32(cx+float64(dir)*w/2), float32(cx-float64(dir)*w/2)
	var p vector.Path
	p.MoveTo(tip, float32(cy))
	p.LineTo(base, float32(cy-h/2))
	p.LineTo(base, float32(cy+h/2))
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(yellow)
	vector.FillPath(dst, &p, &vector.FillOptions{}, op)
	op.ColorScale.Reset()
	op.ColorScale.ScaleWithColor(color.RGBA{0, 0, 0, 255})
	vector.StrokePath(dst, &p, &vector.StrokeOptions{Width: 2, LineJoin: vector.LineJoinMiter}, op)
}

// colony is the planet's colony, whoever's; nil uninhabited or veiled.
func (s *planetScreen) colony() *game.Colony {
	if s.col != nil {
		return s.col
	}
	if s.veiled {
		return nil
	}
	return s.other
}

// The shift cue's size on the planet screen, native px; the planets
// list's rows take it at two thirds.
const shiftW, shiftH = 16, 28

func outputFrame(v int) int { return max(0, min(20, v/2)) }

// outputBadge writes v at its picture's bottom-right (x1, y1), on a badge.
func outputBadge(frame *ebiten.Image, v, x1, y1 int) {
	n := strconv.Itoa(v)
	w, h := ui.Width(n, bigFace), bigFace.Size*1.2
	x, y := float64(x1-4*zoom)-w, float64(y1-3*zoom)-h
	vector.FillRect(frame, float32(x-4), float32(y-1), float32(w+8), float32(h+2), color.RGBA{0, 0, 0, 170}, false)
	ui.Print(frame, n, bigFace, x, y, textColour)
}

// panelTexts are the POP panel's line ("POP n/capacity", "+N" when
// icons do not fit; another's veiled colony its species and the cloaks
// instead, orbitCloakedLine last on a veiled orbit) and the project
// panel (empty on a veiled colony).
func (s *planetScreen) panelTexts() (pop string, pp projPanel) {
	busy, free, room := s.popCounts()
	_, more, _ := popLayout(busy, free, room)
	n, second, third := busy+free, "", ""
	pp.title = s.planet.Name + " - " + s.sysName
	switch {
	case s.col != nil:
		second = fmt.Sprintf("free %d  ", free)
		pp.fill(s.col, "click a square or slot to build")
		pp.cross = s.cancelShown()
		if s.placing() {
			pp.foot = placingFoot(s.placeItem)
		}
		third = "\n" + growthText(s.col)
	case s.other != nil && s.veiled: // no people, no project: the owner and the cloaks
		l := s.otherSp.String() + "\n" + surfaceCloakedLine
		if s.orbitVeiled {
			l += "\n" + orbitCloakedLine
		}
		return l, pp
	case s.other != nil:
		second = s.otherSp.String() + "  "
		pp.fill(s.other, "")
		if s.orbitVeiled {
			third = "\n" + orbitCloakedLine
		}
	default:
		pp.name = "uninhabited"
	}
	if more > 0 {
		second += fmt.Sprintf("+%d", more)
	}
	return fmt.Sprintf("POP %d/%d\n%s%s", n, n+room, strings.TrimSpace(second), third), pp
}

// growthText is "growth in N days" (Colony.GrowthDays), "full" at
// capacity, "no growth" without prosperity.
func growthText(c *game.Colony) string {
	if d, ok := c.GrowthDays(); ok {
		return "growth in " + dayCount(d)
	}
	if c.Pop >= c.Capacity() {
		return "full"
	}
	return "no growth"
}

// The POP panel's icons (colony-home-dosbox.png): 17np frames 1-3,
// 10x10, origin top-left: busy (1), free (2), free room up to capacity
// (3), popCols a row at popPitch orig px. The original fills 10 rows;
// ours keeps popRows, the rest is the text's. Native px. More than fit
// draws them at popDense (150% rather than 200%) on
// the same area, popDenseCols a row; past that the rest is "+N".
const (
	popPitch         = 10 * zoom
	popCols, popRows = 10, 7

	popDense      = 1.5
	popDensePitch = popPitch * 3 / 4 // 10 asset px at popDense
	popDenseCols  = popCols * popPitch / popDensePitch
	popDenseRows  = popRows * popPitch / popDensePitch
)

type popKind int

const (
	popBusy popKind = iota + 1 // = the 17np frame
	popFree
	popRoom
)

// popIcon is one POP icon, (x,y) its origin from the panel's corner.
type popIcon struct {
	kind popKind
	x, y int
}

// popLayout lays the icons out at scale k: zoom, or popDense when they
// overflow it.
func popLayout(busy, free, room int) (icons []popIcon, more int, k float64) {
	if max(0, busy)+max(0, free)+max(0, room) > popCols*popRows {
		icons, more = popGrid(busy, free, room, popDenseCols, popDenseRows, popDensePitch)
		return icons, more, popDense
	}
	icons, more = popGrid(busy, free, room, popCols, popRows, popPitch)
	return icons, more, zoom
}

// popGrid lays busy, free and room icons out in that order, cols a row
// at pitch px; past cols*rows the rest is more, not drawn.
func popGrid(busy, free, room, cols, rows, pitch int) (icons []popIcon, more int) {
	for k, n := range [...]int{busy, free, room} {
		for range max(0, n) {
			i := len(icons)
			if i == cols*rows {
				more++
				continue
			}
			icons = append(icons, popIcon{popKind(k + 1), i % cols * pitch, i / cols * pitch})
		}
	}
	return icons, more
}

// popCounts are the POP panel's busy, free and room counts; another's
// colony shows all its people free (none veiled), a bare planet room
// only.
func (s *planetScreen) popCounts() (busy, free, room int) {
	switch {
	case s.col != nil:
		return colonyPop(s.col)
	case s.veiled:
		return 0, 0, 0
	case s.other != nil:
		return 0, s.other.Pop, max(0, s.other.Capacity()-s.other.Pop)
	}
	return 0, 0, s.planet.Capacity()
}

func colonyPop(c *game.Colony) (busy, free, room int) {
	busy = min(c.Busy(), c.Pop)
	return busy, c.Pop - busy, max(0, c.Capacity()-c.Pop)
}

// drawPop paints the POP icons, filtered linear under zoom.
func (s *planetScreen) drawPop(a *app, sc *ebiten.Image) {
	icons, _, k := popLayout(s.popCounts())
	p := panels[3]
	for _, ic := range icons {
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(float64(p.X0+ic.x), float64(p.Y0+ic.y))
		if k < zoom {
			op.Filter = ebiten.FilterLinear
		}
		a.spr("17np", int(ic.kind)).Draw(sc, &op)
	}
}

// bombedTint cuts red and green: a used Lush Growth Bomb reads blue.
var bombedTint = [3]float32{0.25, 0.45, 1}

// drawProjButtons paints the project buttons: faded refused, blue
// bombed, framed yellow running, white hovered, dim otherwise.
func (s *planetScreen) drawProjButtons(a *app, sc *ebiten.Image) {
	for _, b := range s.projButtons(a) {
		fillRect(sc, b.r)
		if sp := a.spr("planitem", int(b.item)); sp.Image != nil {
			ib := sp.Image.Bounds()
			var op ebiten.DrawImageOptions
			op.GeoM.Translate(float64(b.r.X0+(projBtnW-ib.Dx())/2-sp.Offset.X), float64(b.r.Y0+(projBtnH-ib.Dy())/2-sp.Offset.Y))
			switch {
			case b.bombed: // refused too, but a mark rather than a fade
				op.ColorScale.Scale(bombedTint[0], bombedTint[1], bombedTint[2], 1)
			case b.err != nil && !s.running(b.item):
				op.ColorScale.ScaleAlpha(0.35)
			}
			sp.Draw(sc, &op)
		}
		c := dim(textColour, 0.5)
		switch {
		case s.running(b.item):
			c = yellow
		case s.projOn == b.item:
			c = textColour
		}
		strokeRect(sc, b.r, c)
	}
}

// buildsInPlace reports pj rises at its square or slot (a planet-wide
// project lights its button, a refit places nothing).
func buildsInPlace(pj *game.Project) bool { return !pj.Item.PlanetWide() && pj.Refit == "" }

func (s *planetScreen) running(it game.Item) bool {
	return s.col != nil && s.col.Project != nil && s.col.Project.Item == it && s.col.Project.Refit == ""
}

// unbuiltTint is the night-blue of an unbuilt part
// (colony-full-orbit-dosbox.png).
var unbuiltTint = [3]float32{0.1, 0.15, 0.45}

// drawBuilding draws a project's item at zoom, origin (x,y), coloured
// from the bottom up to done (0..1), unbuilt-tinted above.
func drawBuilding(sc *ebiten.Image, sp *assets.Sprite, x, y float64, done float64) {
	drawBuildingK(sc, sp, x, y, zoom, done)
}

// drawBuildingK is drawBuilding at scale k, filtered linear under zoom.
func drawBuildingK(sc *ebiten.Image, sp *assets.Sprite, x, y, k float64, done float64) {
	if sp.Image == nil {
		return
	}
	var base ebiten.DrawImageOptions
	base.GeoM.Scale(k, k)
	base.GeoM.Translate(x, y)
	if k < zoom {
		base.Filter = ebiten.FilterLinear
	}
	sp.Draw(sc, &base)
	b := sp.Image.Bounds()
	rows := int(math.Round(float64(b.Dy()) * (1 - min(max(done, 0), 1))))
	if rows <= 0 {
		return
	}
	top := sp.Image.SubImage(image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+rows)).(*ebiten.Image)
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(x+k*float64(sp.Offset.X), y+k*float64(sp.Offset.Y))
	op.Filter = base.Filter
	op.ColorScale.Scale(unbuiltTint[0], unbuiltTint[1], unbuiltTint[2], 1)
	sc.DrawImage(top, &op)
}

// drawGlobe centres the planet's globe on G at zoom: one frame per
// size, so a Small looks small under its grid.
func (s *planetScreen) drawGlobe(a *app, sc *ebiten.Image) {
	sp := a.spr(globeBank(s.planet.Type), s.planet.Size-1)
	if sp.Image == nil {
		return
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(s.grid.gx, s.grid.gy)
	sp.Draw(sc, &op)
}

func panelImage(sc *ebiten.Image, i int) *ebiten.Image {
	p := panels[i]
	return sc.SubImage(image.Rect(p.X0, p.Y0, p.X1, p.Y1)).(*ebiten.Image)
}

func okColour(ok bool) color.RGBA {
	if ok {
		return yellow
	}
	return red
}

// projPanel is the top-right project panel: name, picture, "N days
// until completion", the planet's and system's names on top; col nil
// when uninhabited.
type projPanel struct {
	title, name, foot string
	col               *game.Colony
	cross             bool // the cancel cross sits left of the title (cancelRect)
}

// fill sets the panel from col's project: its name and the days left
// or ongoingFoot; with none, the queue's head waiting for a free worker
// (game.Colony.Next), else "No Project".
func (pp *projPanel) fill(col *game.Colony, idle string) {
	pp.col = col
	pj := col.Project
	switch {
	case pj == nil && len(col.Queue) > 0:
		pp.name, pp.foot = "Waiting: "+col.Queue[0].Item.String(), "for a free worker"
		return
	case pj == nil:
		pp.name, pp.foot = "No Project", idle
		return
	}
	pp.name = projectName(pj)
	if f, ok := ongoingFoot(col); ok {
		pp.foot = f
		return
	}
	switch n, ok := col.DaysLeft(); {
	case col.Blockaded():
		pp.foot = "blockaded: no progress"
	case !ok:
		pp.foot = "no industry"
	default:
		pp.foot = dayCount(max(1, n)) + " until completion"
	}
}

// ongoingFoot is "ongoing: +N <output>" for a running ongoing project,
// N the industry it diverts.
func ongoingFoot(col *game.Colony) (string, bool) {
	switch it, n := col.Diverted(); it {
	case game.ScientistTakeover:
		return fmt.Sprintf("ongoing: +%d research", n), true
	case game.EndlessParty:
		return fmt.Sprintf("ongoing: +%d prosperity", n), true
	case game.AlienHospitality:
		return fmt.Sprintf("ongoing: +%d hospitality", n), true
	}
	return "", false
}

// Project panel geometry, native px, from panels[4]'s top: the title
// line, the name's, the picture's box centre, the foot's.
const (
	projTitleY, projNameY = 6, 26
	projPicY, projPicH    = 106, 96
	projFootDY            = 30 // above the panel's bottom
)

// text writes the title small at the left, the name and foot centred;
// a line too wide for bigFace in smallFace (the placement mode's foot).
func (pp projPanel) text(frame *ebiten.Image) {
	p := panels[4]
	cx := float64(p.X0+p.X1) / 2
	x0 := p.X0 + 6
	if pp.cross {
		x0 = cancelRect.X1 + 6
	}
	ui.Print(frame, fitWidth(pp.title, float64(p.X1-x0-6), smallFace), smallFace, float64(x0), float64(p.Y0+projTitleY), dim(textColour, 0.7))
	for _, l := range []struct {
		s string
		y int
	}{{pp.name, p.Y0 + projNameY}, {pp.foot, p.Y1 - projFootDY}} {
		w, face := float64(p.X1-p.X0-12), bigFace
		if ui.Width(l.s, face) > w {
			face = smallFace
		}
		t := fitWidth(l.s, w, face)
		ui.Print(frame, t, face, cx-ui.Width(t, face)/2, float64(l.y), textColour)
	}
}

// drawProject paints the running project's picture (drawProjectPic).
func (s *planetScreen) drawProject(a *app, sc *ebiten.Image) {
	_, pp := s.panelTexts()
	if pp.col == nil || pp.col.Project == nil {
		return
	}
	p := panels[4]
	drawProjectPic(a, panelImage(sc, 4), pp.col, float64(p.X0+p.X1)/2, float64(p.Y0+projPicY), float64(p.X1-p.X0-24), projPicH)
}

// colonyShips is the ship bank col builds in: its owner's.
func colonyShips(g *game.Game, col *game.Colony) string {
	return shipBank(g.Empires[col.Planet.Owner].Species)
}

// drawProjectPic paints col's project centred on (cx,cy): a ship or
// refit as its hull fitted to w x h, else its planitem frame at zoom.
func drawProjectPic(a *app, dst *ebiten.Image, col *game.Colony, cx, cy, w, h float64) {
	pj := col.Project
	done := projDone(pj)
	k := float64(zoom)
	sp := a.spr("planitem", int(projItem(col, pj)))
	if pj.Design != nil {
		sp = a.spr(colonyShips(a.g, col), int(pj.Design.Hull))
	}
	if sp.Image == nil {
		return
	}
	b := sp.Image.Bounds()
	bw, bh := float64(b.Dx()), float64(b.Dy())
	if pj.Design != nil {
		k = min(k, w/bw, h/bh)
	}
	drawBuildingK(dst, sp, cx-k*(float64(sp.Offset.X)+bw/2), cy-k*(float64(sp.Offset.Y)+bh/2), k, done)
}

// projItem is the item pj is drawn as: an Automation as the structure it
// automates (as antag shows it), else its own.
func projItem(col *game.Colony, pj *game.Project) game.Item {
	if pj.Item == game.Automation {
		if sq, ok := col.Planet.Surface[pj.Cell]; ok && sq.Item != game.NoItem {
			return sq.Item
		}
	}
	return pj.Item
}

// projDone is how much of pj's picture is coloured, 0 to 1; whole for
// an ongoing one, which never completes.
func projDone(pj *game.Project) float64 {
	if pj.Item.Ongoing() {
		return 1
	}
	return float64(pj.Progress) / float64(max(1, pj.Cost()))
}

// projectName is the project's item, a ship's design name, or "Refit of
// <ship>" (a refit's Item is the Docks it occupies, its Design the target).
func projectName(pj *game.Project) string {
	if pj.Refit != "" {
		return "Refit of " + pj.Refit
	}
	if pj.Design != nil {
		return pj.Design.Name
	}
	return pj.Item.String()
}

// ruinsFrame is planitem's bare Xeno ruins tile (68x52; frame 38 is the
// dig over them), drawn on every ruins square but the one dug.
const ruinsFrame = 39

func (s *planetScreen) digging(c game.Cell) bool {
	return s.col != nil && s.col.Project != nil && s.col.Project.Item == game.XenoArcheologicalDig && s.col.Project.Cell == c
}

// squareText is the hovered square's status, "(i,j) <kind> - <item>",
// plus the dig's verdict on ruins the picker offers.
func (s *planetScreen) squareText(a *app) string {
	sq := s.planet.Surface[s.cell]
	what := structName(sq)
	if sq.Ruins && sq.Item == game.NoItem {
		what = "Xeno ruins"
		it := game.XenoArcheologicalDig
		if s.col != nil && !s.targeted() && a.e.Research.Allows(it) && s.planet.Offers(it, s.cell) {
			switch err := s.verdict(a, it); {
			case err != nil:
				what += " - " + err.Error()
			case s.enqueuing():
				what += " (the dig can be queued)"
			default:
				what += " (the dig can start)"
			}
		}
	}
	return fmt.Sprintf("(%d,%d) %s - %s", s.cell.I, s.cell.J, sq.Kind, what)
}

func itemName(it game.Item) string {
	if it == game.NoItem {
		return "empty"
	}
	return it.String()
}

func diamond(x, y float64) *vector.Path { return diamondSized(x, y, halfW, halfH) }

// diamondSized is the diamond of half axes hw, hh centred on (x,y).
func diamondSized(x, y, hw, hh float64) *vector.Path {
	var p vector.Path
	fx, fy, w, h := float32(x), float32(y), float32(hw), float32(hh)
	p.MoveTo(fx, fy-h)
	p.LineTo(fx+w, fy)
	p.LineTo(fx, fy+h)
	p.LineTo(fx-w, fy)
	p.Close()
	return &p
}

// The original's marker for a square no structure reaches yet: a 13x7
// orig px diamond, 1 px outline in the kind's colour, filled grey on
// white, dark on black, empty on a bonus square (measured on
// colony-home-dosbox.png; colours: docs/screens.md's census). Native px.
const markerHW, markerHH = 6.5 * zoom, 3.5 * zoom

var markerStyles = [game.NumSquareKinds]squareStyle{
	game.Black:           {fill: color.RGBA{0, 0, 0, 160}, outline: color.RGBA{0, 0, 0, 255}},
	game.White:           {fill: color.RGBA{90, 90, 90, 255}, outline: color.RGBA{219, 219, 219, 255}},
	game.BonusIndustry:   {outline: color.RGBA{190, 45, 16, 255}},
	game.BonusResearch:   {outline: color.RGBA{0, 0, 255, 255}},
	game.BonusProsperity: {outline: color.RGBA{45, 194, 12, 255}},
}

func drawMarker(dst *ebiten.Image, x, y float64, k game.SquareKind) {
	st := markerStyles[k]
	p := diamondSized(x, y, markerHW, markerHH)
	if st.fill.A != 0 {
		op := &vector.DrawPathOptions{AntiAlias: true}
		op.ColorScale.ScaleWithColor(st.fill)
		vector.FillPath(dst, p, &vector.FillOptions{}, op)
	}
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(st.outline)
	vector.StrokePath(dst, p, &vector.StrokeOptions{Width: zoom, LineJoin: vector.LineJoinMiter, MiterLimit: 4}, op)
}

func fillDiamond(dst *ebiten.Image, x, y float64, c color.RGBA, alpha float32) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	op.ColorScale.ScaleAlpha(alpha)
	vector.FillPath(dst, diamond(x, y), &vector.FillOptions{}, op)
}

// strokeSquare outlines an orbit slot's cell, its line centred 1 original
// px inside the edges; width is native px.
func strokeSquare(dst *ebiten.Image, x, y float64, c color.RGBA, width float32) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	var p vector.Path
	h := float32(slotHalf - zoom)
	p.MoveTo(float32(x)-h, float32(y)-h)
	p.LineTo(float32(x)+h, float32(y)-h)
	p.LineTo(float32(x)+h, float32(y)+h)
	p.LineTo(float32(x)-h, float32(y)+h)
	p.Close()
	vector.StrokePath(dst, &p, &vector.StrokeOptions{Width: width, LineJoin: vector.LineJoinMiter, MiterLimit: 4}, op)
}

func strokeDiamond(dst *ebiten.Image, x, y float64, c color.RGBA, width float32) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.StrokePath(dst, diamond(x, y), &vector.StrokeOptions{Width: width, LineJoin: vector.LineJoinMiter, MiterLimit: 4}, op)
}
