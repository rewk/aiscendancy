package main

import (
	"errors"
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The placement mode, as the design screen's
// parts: a click on the project panel (panels[4], its cancel cross
// aside) of the player's colony opens the surface picker
// (surfaceChoices); a choice turns the mode on with it (s.placeItem). Each
// left click on a square accepting it then does what picking it in that
// square's picker does: starts it, or queues it while enqueuing; the
// mode stays. A queued ghost's square still dequeues; any other square,
// slot or ship does nothing. The project buttons, the queue stack and
// the cancel cross keep working. A right click, Esc (cancel) or the
// panel again end the mode; so does the colony changing hands (refresh).

// panelHoverLine is the status line over the project panel, mode off.
const panelHoverLine = "Build: pick a surface building, then click squares"

// placing reports the placement mode on.
func (s *planetScreen) placing() bool { return s.placeItem != game.NoItem }

// placingFoot is the project panel's bottom line while placing it: the
// first of the long, the short and the bare line fitting the panel
// (projPanel.text drops to smallFace past bigFace).
func placingFoot(it game.Item) string {
	w := float64(panels[4].X1 - panels[4].X0 - 12)
	long := fmt.Sprintf("Placing %s: click squares, right click ends", it)
	short := fmt.Sprintf("Placing %s: right click ends", it)
	switch {
	case ui.Width(long, bigFace) <= w:
		return long
	case ui.Width(short, smallFace) <= w:
		return short
	}
	return "Placing " + it.String()
}

// rightClick ends the mode (Update's right click).
func (s *planetScreen) rightClick() { s.placeItem = game.NoItem }

// freeSquares are the empty open squares of the plan, in Cells order.
func (s *planetScreen) freeSquares(pl *game.Planet) []game.Cell {
	var fs []game.Cell
	for _, c := range pl.Cells() {
		if pl.Surface[c].Item == game.NoItem && s.openIn(pl, c) {
			fs = append(fs, c)
		}
	}
	return fs
}

// surfaceChoices are the surface buildings research allows (never the
// Colony Base), in Item order, each nil when a free square offering it
// accepts it (cellVerdict, the per-square picker's), else the first
// such square's refusal; nil off the player's colony.
func (s *planetScreen) surfaceChoices(a *app) []choice {
	if s.col == nil {
		return nil
	}
	pl := s.plan()
	free := s.freeSquares(pl)
	var cs []choice
	for it := game.Item(0); it < game.NumItems; it++ {
		if !it.Surface() || it == game.ColonyBase || !a.e.Research.Allows(it) {
			continue
		}
		var err error = errNoFreeSquare
		for _, c := range free {
			if !pl.Offers(it, c) {
				continue
			}
			e := s.cellVerdict(a, it, c)
			if e == nil {
				err = nil
				break
			}
			if err == errNoFreeSquare {
				err = e
			}
		}
		cs = append(cs, choice{it, err})
	}
	return cs
}

var errNoFreeSquare = errors.New("no free square")

// clickPanel is a click on the project panel: it ends the mode, else
// opens the surface picker, else says why nothing can start.
func (s *planetScreen) clickPanel(a *app) error {
	s.refresh(a)
	switch {
	case s.col == nil:
		return nil
	case s.placing():
		s.placeItem = game.NoItem
		return nil
	}
	cs := s.surfaceChoices(a)
	switch {
	case buildable(cs):
		s.pick = &picker{cs: cs, hover: -1, surface: true, planet: s.plan()}
		s.pick.header(s)
		return nil
	case len(cs) > 0:
		return cs[0].err
	}
	return errors.New("no surface building researched yet")
}

// placeErr is the colony's verdict on the placed item at the hovered
// square: the per-square picker's (Offers, then cellVerdict).
func (s *planetScreen) placeErr(a *app) error {
	if !s.plan().Offers(s.placeItem, s.cell) {
		return fmt.Errorf("no %s on the Xeno ruins", s.placeItem)
	}
	return s.cellVerdict(a, s.placeItem, s.cell)
}

// placeClick is the mode's left click on a square or slot: a queued
// square dequeues, an accepting one starts or queues the item (startAt),
// anything else does nothing.
func (s *planetScreen) placeClick(a *app) error {
	s.refresh(a)
	switch {
	case !s.placing() || !s.cellOK:
		return nil
	case s.queuedAt() >= 0:
		return s.dequeue(a, s.queuedAt())
	case s.placeErr(a) != nil:
		return nil
	}
	return s.startAt(a, s.placeItem, s.cell)
}

// placeText is the status line on a square while placing: its verdict.
func (s *planetScreen) placeText(a *app) string {
	at := fmt.Sprintf("(%d,%d) %s - ", s.cell.I, s.cell.J, s.planet.Surface[s.cell].Kind)
	if err := s.placeErr(a); err != nil {
		return at + err.Error()
	}
	verb := "build"
	if s.enqueuing() {
		verb = "queue"
	}
	return at + "click: " + verb + " " + s.placeItem.String()
}

// drawPlaceGhost paints the placed item's ghost on the hovered empty
// square (its outline is the hover's: hoverColour).
func (s *planetScreen) drawPlaceGhost(a *app, sc *ebiten.Image) {
	if !s.placing() || !s.cellOK || s.queuedAt() >= 0 || s.plan().Surface[s.cell].Item != game.NoItem {
		return
	}
	s.drawGhost(a, sc, game.Queued{Item: s.placeItem, Cell: s.cell, Slot: -1})
}

// drawPanelCue frames the project panel: yellow placing, white with a
// faint wash hovered.
func (s *planetScreen) drawPanelCue(sc *ebiten.Image) {
	p := panels[4]
	switch {
	case s.placing():
		strokeRect(sc, p, yellow)
	case s.panelOn:
		vector.FillRect(sc, float32(p.X0), float32(p.Y0), float32(p.X1-p.X0), float32(p.Y1-p.Y0), panelWash, false)
		strokeRect(sc, p, textColour)
	}
}

// panelWash is the hovered project panel's light highlight: white at
// 8%, premultiplied.
var panelWash = color.RGBA{20, 20, 20, 20}
