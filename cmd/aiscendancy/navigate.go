package main

import (
	"fmt"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// Navigate: the system panel's Navigate… (navRect, key N) pushes a
// starmap in pick mode for the selected ship (newNavMap): hovering a star
// shows Empire.Route to it, its lanes yellow over the others and its
// lanes and days on the map's bottom line; a click on a reachable star
// orders GoToSystem there and pops back, Esc, the back button or a right
// click pop without an order. The route comes from the model, cached by
// hovered star and day (navPick.at), never recomputed per frame.

// navPick is the starmap's pick mode: the ship, and the route to the
// star hovered last (star −1 none) as computed on day.
type navPick struct {
	ship  *game.Starship
	star  int
	day   int
	route []int
	lanes []int // the route's cluster lanes (RouteLanes), drawn yellow
	days  int
	err   error
}

// newNavMap is a starmap picking ship's destination.
func newNavMap(ship *game.Starship) *starmapScreen {
	s := newStarmapScreen()
	s.nav = &navPick{ship: ship, star: -1}
	return s
}

// at points the cache at star (−1: none) for e: Route and RouteLanes
// run only when the star or the day changed.
func (n *navPick) at(e *game.Empire, star int) {
	if star == n.star && e.Day == n.day {
		return
	}
	n.star, n.day = star, e.Day
	n.route, n.lanes, n.days, n.err = nil, nil, 0, nil
	if star < 0 {
		return
	}
	n.route, n.err = e.Route(n.ship, star)
	if n.err == nil {
		n.lanes, n.days = e.RouteLanes(n.ship, n.route)
	}
}

// navUpdate is the pick mode's input before the map's own: the ship
// gone, lost or in a lane since (a day passed) or a right click pops.
func (s *starmapScreen) navUpdate(a *app, right bool) (popped bool, err error) {
	if e := a.e.CanNavigate(s.nav.ship); e != nil {
		a.toastFor(e.Error(), true)
		return true, a.pop()
	}
	if right {
		return true, a.pop()
	}
	return false, nil
}

// navClick is a click on star in pick mode: unreachable, its refusal
// toasted and the mode kept; else the GoToSystem order (a refusal
// toasted) and the pop back, the system screen under dropping the ship
// once ordered, as its own destinations (systemScreen.order).
func (s *starmapScreen) navClick(a *app, star int) error {
	n := s.nav
	n.at(a.e, star)
	if n.err != nil {
		a.toastFor(n.err.Error(), true)
		return nil
	}
	err := a.e.Order(n.ship, game.Order{Kind: game.GoToSystem, Star: star})
	if err != nil {
		a.toastFor(err.Error(), true)
	}
	if perr := a.pop(); perr != nil {
		return perr
	}
	if sys, ok := a.top().(*systemScreen); ok && err == nil && sys.sel == n.ship && sys.handHops() {
		sys.sel, sys.last = nil, n.ship
	}
	return nil
}

// navLine is the hovered star's line in pick mode: "<star>: N lanes, ~D
// days", else the route's refusal (bad true).
func (n *navPick) line(e *game.Empire) (string, bool) {
	if n.err != nil {
		return n.err.Error(), true
	}
	return fmt.Sprintf("%s: %d lane%s, ~%s", e.StarLabel(n.star), len(n.route), plural(len(n.route)), dayCount(n.days)), false
}

// navText writes the pick mode's lines: the hint on the map's top, the
// hovered star's route on its bottom.
func (s *starmapScreen) navText(a *app, frame *ebiten.Image) {
	n := s.nav
	ui.Print(frame, fmt.Sprintf("Navigate %s: click a system (Esc: cancel)", n.ship.Name), bigFace,
		float64(mapArea.X0+padX), float64(mapArea.Y0+padY), yellow)
	if s.hover < 0 || n.star != s.hover {
		return
	}
	l, bad := n.line(a.e)
	c := footColour
	if bad {
		c = errColour
	}
	ui.Print(frame, l, bigFace, float64(mapArea.X0+padX), float64(mapArea.Y1-padY)-bigFace.Size*1.25, c)
}

// navLane reports lane li on the hovered route.
func (s *starmapScreen) navLane(li int) bool {
	return s.nav != nil && s.nav.star == s.hover && slices.Contains(s.nav.lanes, li)
}

// navOrder is a GoToSystem order's text: "-> <dest> (N lanes)", N the
// lanes left on Empire.Route, "(no route)" when it has none today.
func navOrder(e *game.Empire, sh *game.Starship) string {
	o := sh.Order
	r, err := e.Route(sh, o.Star)
	if err != nil {
		return fmt.Sprintf("-> %s (no route)", e.StarLabel(o.Star))
	}
	return fmt.Sprintf("-> %s (%d lane%s)", e.StarLabel(o.Star), len(r), plural(len(r)))
}

// navRect is the panel's Navigate… button, left of Stop (stopRect), as
// tall.
func navRect() rect {
	r := stopRect()
	return rect{r.X0 - stopGap - navW, r.Y0, r.X0 - stopGap, r.Y1}
}

// navW is the Navigate… button's width (bigFace label + padding).
const navW = 104

const navLabel = "Navigate…"

// navigate is Navigate… (N): the pick map pushed for the selected ship,
// or CanNavigate's refusal.
func (s *systemScreen) navigate(a *app) error {
	if s.sel == nil {
		return nil
	}
	if err := a.e.CanNavigate(s.sel); err != nil {
		return err
	}
	s.aim, s.move = false, moveMode{}
	a.push(newNavMap(s.sel))
	return nil
}
