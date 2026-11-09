package main

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The structure dialog (captures image0041-43): a click on a structure
// of the player's colony, on a square or in orbit, opens a box titled
// "<structure> on <Kind> Square" (static.txt 84-85), buttons along the
// bottom (dlgActs). It takes every input while up and closes when the
// colony changes hands or the structure is gone. The player's ship in
// orbit opens the same box (shipDlgActs).

// The box is help.shp frame 0 (336x256, origin (0,0)), the button bar
// the hole at rows [217,248), at original px (152,120)-(488,376).
var (
	dlgBox = orig(152, 120, 488, 376)
	dlgBar = orig(160, 120+217, 480, 120+248)
)

const dlgBank = "help" // frame 0

// dlgTitleY is the title's y in the box, dlgPicY the picture's centre.
const dlgTitleY, dlgPicY = 30 * zoom, 120 * zoom

// dlgCancelPicY is the cancel question's picture centre, dlgCancelPicH
// its height at most.
const dlgCancelPicY, dlgCancelPicH = 160 * zoom, 90 * zoom

// dlgAct is a dialog button.
type dlgAct uint8

const (
	dlgBuildShip dlgAct = iota
	dlgTerraform
	dlgAbandon
	dlgAutomate
	dlgCancel
	dlgLeaveOrbit // the ship dialog's (orbitships.go)
	dlgCreateColony
	dlgInvade
	dlgRefit
	dlgShow
	dlgLoad // a Space Habitat's, one unit a press (orbitships.go)
	dlgUnload
)

var dlgLabels = [...]string{dlgBuildShip: "Build Ship", dlgTerraform: "Terraform", dlgAbandon: "Abandon", dlgAutomate: "Automate", dlgCancel: "Cancel",
	dlgLeaveOrbit: "Leave Orbit", dlgCreateColony: "Create Colony", dlgInvade: "Invade", dlgRefit: "Refit", dlgShow: "Show",
	dlgLoad: "Load", dlgUnload: "Unload"}

// structDlg is the open dialog: a square, an orbit slot, or a ship.
type structDlg struct {
	cell    game.Cell
	slot    int
	orbital bool
	ship    *game.Starship
}

// item is the structure the dialog is about, NoItem once gone.
func (d *structDlg) item(p *game.Planet) game.Item {
	if d.orbital {
		return p.Orbit[d.slot]
	}
	return p.Surface[d.cell].Item
}

// squareKinds are the original's square names (static.txt 75-81).
var squareKinds = [game.NumSquareKinds]string{
	game.Black: "Black", game.White: "White", game.BonusIndustry: "Red",
	game.BonusProsperity: "Green", game.BonusResearch: "Blue",
}

// structName is "Automated <item>" once automated (static.txt 84).
func structName(sq *game.Square) string {
	if sq.Automated && sq.Item != game.NoItem {
		return "Automated " + sq.Item.String()
	}
	return itemName(sq.Item)
}

// title is "<structure> on <Kind> Square" ("Orbital" for a slot), or
// Ship "<name>".
func (d *structDlg) title(p *game.Planet) string {
	if d.ship != nil {
		return `Ship "` + d.ship.Name + `"`
	}
	if d.orbital {
		return p.Orbit[d.slot].String() + " on Orbital Square"
	}
	sq := p.Surface[d.cell]
	return structName(sq) + " on " + squareKinds[sq.Kind] + " Square"
}

// dlgActs are the buttons in order: Build Ship on a Shipyard, Terraform
// on Transport Tubes on black, Abandon (never the Colony Base), Automate
// once research allows it, Cancel.
func (s *planetScreen) dlgActs(a *app) []dlgAct {
	d := s.dlg
	if d.ship != nil {
		return s.shipDlgActs(a, d.ship)
	}
	var out []dlgAct
	it := d.item(s.planet)
	if d.orbital {
		if it == game.Shipyard {
			out = append(out, dlgBuildShip)
		}
		return append(out, dlgAbandon, dlgCancel)
	}
	r := a.e.Research
	if it == game.TransportTubes && r.Allows(game.Terraforming) && s.planet.Offers(game.Terraforming, d.cell) {
		out = append(out, dlgTerraform)
	}
	if it != game.ColonyBase {
		out = append(out, dlgAbandon)
	}
	if !s.planet.Surface[d.cell].Automated && it.Staff() > 0 && r.Allows(game.Automation) { // nobody to free: not offered
		out = append(out, dlgAutomate)
	}
	return append(out, dlgCancel)
}

// dlgOff reports button k greyed and inert: Abandon refused by
// CanAbandon, a ship's Invade by CanInvade, Load by CanLoad, Unload by
// CanUnload (its hover says why).
func (s *planetScreen) dlgOff(a *app, k dlgAct) bool {
	d := s.dlg
	if d != nil && d.ship != nil {
		switch k {
		case dlgInvade:
			return a.e.CanInvade(d.ship) != nil
		case dlgLoad:
			return a.e.CanLoad(d.ship) != nil
		case dlgUnload:
			return a.e.CanUnload(d.ship) != nil
		}
	}
	if k != dlgAbandon || d == nil || d.orbital || d.ship != nil || s.col == nil {
		return false
	}
	_, ok := s.planet.Surface[d.cell]
	return ok && s.col.CanAbandon(d.cell) != nil
}

// dlgButton is one button and its rect: the bar cut in equal parts.
type dlgButton struct {
	act dlgAct
	r   rect
}

func (s *planetScreen) dlgButtons(a *app) []dlgButton {
	acts := s.dlgActs(a)
	bs := make([]dlgButton, len(acts))
	// Each cell is its label's width plus an equal share of the rest, so
	// a long label (Leave Orbit) fits beside five others.
	lw := make([]int, len(acts))
	rest := dlgBar.X1 - dlgBar.X0
	for i, k := range acts {
		lw[i] = int(ui.Width(dlgLabels[k], bigFace))
		rest -= lw[i]
	}
	x0 := dlgBar.X0
	for i, k := range acts {
		w := lw[i] + rest/len(acts)
		bs[i] = dlgButton{k, rect{x0 + 4*zoom, dlgBar.Y0, x0 + w - 4*zoom, dlgBar.Y1}}
		x0 += w
	}
	return bs
}

// dlgAt is the dialog button under native (x,y).
func (s *planetScreen) dlgAt(a *app, x, y float64) (dlgAct, bool) {
	for _, b := range s.dlgButtons(a) {
		if b.r.has(x, y) {
			return b.act, true
		}
	}
	return 0, false
}

// openDlg opens the dialog on the hovered square's or slot's structure.
func (s *planetScreen) openDlg() {
	s.dlg = &structDlg{cell: s.cell, slot: s.slot, orbital: !s.cellOK}
}

// press is a dialog button pressed: the screen to push (Build Ship), or
// a refusal to flash (the box kept); a success closes it.
func (s *planetScreen) press(a *app, k dlgAct) (screen, error) {
	d := s.dlg
	if d != nil && d.ship != nil {
		return s.pressShip(a, d.ship, k)
	}
	if d == nil || s.col == nil {
		s.dlg = nil
		return nil, nil
	}
	var err error
	switch k {
	case dlgBuildShip:
		s.dlg = nil
		return newDesignScreenFor(s.col, freshTemplate()), nil
	case dlgTerraform: // queued while enqueuing (startAt)
		err = s.startAt(a, game.Terraforming, d.cell)
	case dlgAutomate:
		err = s.startAt(a, game.Automation, d.cell)
	case dlgAbandon:
		if s.dlgOff(a, k) { // greyed: nothing happens
			return nil, nil
		}
		if d.orbital {
			err = s.col.AbandonOrbital(d.slot)
		} else {
			err = s.col.Abandon(d.cell)
		}
		if err == nil { // a worker freed: a waiting queue's head starts
			s.next(a)
		}
	}
	if err == nil {
		s.dlg = nil
	}
	return nil, err
}

// dlgUpdate reads the dialog's input: Esc is planetScreen.cancel's.
func (s *planetScreen) dlgUpdate(a *app) error {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	x, y := a.mouse()
	push, err := s.dlgClick(a, x, y)
	if push != nil {
		a.push(push)
	}
	if err != nil {
		s.msg, s.msgUntil = err.Error(), s.tick+60
	}
	return nil
}

// dlgClick is a click with the dialog up: a button pressed, outside the
// box closes it.
func (s *planetScreen) dlgClick(a *app, x, y float64) (screen, error) {
	if k, ok := s.dlgAt(a, x, y); ok {
		return s.press(a, k)
	}
	if !dlgBox.has(x, y) {
		s.dlg = nil
	}
	return nil, nil
}

// dlgGone reports the structure gone, or the ship no longer ownHere.
func (s *planetScreen) dlgGone(a *app) bool {
	switch d := s.dlg; {
	case d == nil:
		return false
	case d.ship != nil:
		return !s.ownHere(a, d.ship)
	}
	return s.dlg.item(s.planet) == game.NoItem
}

// drawDlg paints the dialog: the box, the title, the structure's picture
// at zoom (a ship's: drawShipDlg), the buttons' labels unboxed.
func (s *planetScreen) drawDlg(a *app, frame *ebiten.Image) {
	d := s.dlg
	b := dlgBox
	fillRectWith(frame, b, dlgFill)
	drawSprite(frame, a.spr(dlgBank, 0), float64(b.X0), float64(b.Y0))
	t := fitWidth(d.title(s.planet), float64(b.X1-b.X0-4*padX), bigFace)
	cx := float64(b.X0+b.X1) / 2
	ui.Print(frame, t, bigFace, cx-ui.Width(t, bigFace)/2, float64(b.Y0+dlgTitleY), textColour)
	if d.ship != nil { // its sprite, order line and parts grid
		s.drawShipDlg(a, frame, d.ship)
	} else {
		it := d.item(s.planet)
		if sp := a.spr("planitem", int(it)); sp.Image != nil {
			ib := sp.Image.Bounds()
			drawSprite(frame, sp, cx-zoom*(float64(sp.Offset.X)+float64(ib.Dx())/2), float64(b.Y0+dlgPicY)-zoom*(float64(sp.Offset.Y)+float64(ib.Dy())/2))
		}
		if ls := dlgHelpLines(a, it); len(ls) > 0 {
			ui.Print(frame, strings.Join(ls, "\n"), smallFace, float64(b.X0+dlgInnerX0+dlgGap), dlgHelpTop(a, it), textColour)
		}
	}
	mx, my := a.mouse()
	for _, btn := range s.dlgButtons(a) {
		c := textColour
		switch {
		case s.dlgOff(a, btn.act):
			c = dim(textColour, 0.4)
		case btn.r.has(mx, my):
			c = yellow
		}
		l := dlgLabels[btn.act]
		ui.Print(frame, l, bigFace, float64(btn.r.X0+btn.r.X1)/2-ui.Width(l, bigFace)/2, float64(btn.r.Y0+btn.r.Y1)/2-bigFace.Size*0.6, c)
	}
}

// dlgHelpGap is the gap between the picture and the help paragraph.
const dlgHelpGap = 6 * zoom

// dlgHelpTop is the native y of item it's paragraph: dlgHelpGap under
// its picture centred on dlgPicY (35 original px down without one).
func dlgHelpTop(a *app, it game.Item) float64 {
	half := 35 * zoom
	if sp := a.spr("planitem", int(it)); sp.Image != nil {
		half = zoom * sp.Image.Bounds().Dy() / 2
	}
	return float64(dlgBox.Y0 + dlgPicY + half + dlgHelpGap)
}

// dlgHelpLines is it's paragraph wrapped to the nebula, the lines that
// fit; without one it reads no sprite.
func dlgHelpLines(a *app, it game.Item) []string {
	t := a.itemHelp(it)
	if t == "" {
		return nil
	}
	return helpLines(t, float64(dlgInnerX1-dlgInnerX0-2*dlgGap), fitLines(dlgHelpTop(a, it), float64(dlgBox.Y0+dlgBarTop-padY)))
}

// dlgOrderLine is the ship's shipLine, "" for a structure.
func (s *planetScreen) dlgOrderLine(a *app) string {
	if d := s.dlg; d != nil && d.ship != nil {
		return shipLine(a, d.ship)
	}
	return ""
}

// dlgFill is the original bar's night blue (image0041).
var dlgFill = color.RGBA{10, 20, 36, 255}
