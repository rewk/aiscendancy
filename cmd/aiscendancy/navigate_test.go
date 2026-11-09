package main

import (
	"fmt"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// navFixture is panelFixture under a starmap, the system screen on top
// with the colony ship selected, out of orbit (rebuild drops a ship in
// orbit); every star explored when reveal.
func navFixture(t *testing.T, reveal bool) (a *app, v *systemScreen, colony, gun, raider *game.Starship) {
	t.Helper()
	a, v, colony, gun, raider = panelFixture(t)
	if err := a.e.LeaveOrbit(colony, nil); err != nil {
		t.Fatal(err)
	}
	if reveal {
		for i := range a.cl.Stars {
			a.e.Visit(i)
		}
	}
	a.open([]screen{newStarmapScreen(), v})
	v.sel = colony
	if colony.Design.LaneDrives() < 1 || gun.Design.LaneDrives() > 0 {
		t.Fatal("fixture: the colony ship has no lane drive, or the Gunboat one")
	}
	return a, v, colony, gun, raider
}

// reachable is the first star Route takes ship to over 2+ lanes, with
// that route.
func reachable(t *testing.T, e *game.Empire, sh *game.Starship) (int, []int) {
	t.Helper()
	for i := range e.Cluster.Stars {
		if r, err := e.Route(sh, i); err == nil && len(r) >= 2 {
			return i, r
		}
	}
	t.Fatal("fixture: no star 2 lanes away")
	return -1, nil
}

// Navigate… sits left of Stop, in the info box above the tiles, its
// label fitting; shown for the player's ships, lit for one CanNavigate
// accepts: a click pushes the pick map for it; greyed (no lane drive),
// or a foreign ship's panel, a click does nothing; N is the same.
func TestNavigateButton(t *testing.T) {
	a, v, colony, gun, raider := navFixture(t, false)
	r, st := navRect(), stopRect()
	if r.X1 > st.X0 || r.Y0 != st.Y0 || r.Y1 != st.Y1 || r.X0 < panelX0 || r.Y1 > panelBandsTop {
		t.Errorf("navRect %+v beside stopRect %+v, from x %d, above y %d", r, st, panelX0, panelBandsTop)
	}
	if w := ui.Width(navLabel, bigFace); w+4*zoom > navW {
		t.Errorf("label %.0f px in a %d px button", w, navW)
	}
	cx, cy := float64(r.X0+r.X1)/2, float64(r.Y0+r.Y1)/2

	v.sel = gun
	if a.e.CanNavigate(gun) == nil {
		t.Fatal("a Gunboat navigates")
	}
	if err := v.panelClick(a, cx, cy); err != nil || a.top() != v {
		t.Errorf("greyed click: %v, top %T", err, a.top())
	}
	if err := v.navigate(a); err == nil || a.top() != v {
		t.Errorf("N on the Gunboat: %v, top %T", err, a.top())
	}
	v.sel = raider
	if err := v.panelClick(a, cx, cy); err != nil || a.top() != v {
		t.Errorf("foreign click: %v, top %T", err, a.top())
	}

	v.sel = colony
	if err := v.panelClick(a, cx, cy); err != nil {
		t.Fatal(err)
	}
	m := top[*starmapScreen](t, a)
	if m.nav == nil || m.nav.ship != colony || len(a.stack) != 3 || m.place() == "starmap" {
		t.Fatalf("pick map: nav %+v, stack %d, place %q", m.nav, len(a.stack), m.place())
	}
	if err := a.back(); err != nil || a.top() != v {
		t.Fatalf("Esc: %v, top %T", err, a.top())
	}
	pressing(t, ebiten.KeyN)
	if err := v.Update(a); err != nil || top[*starmapScreen](t, a).nav.ship != colony {
		t.Errorf("N: %v", err)
	}
}

// The pick mode: hover caches Route by star and day; the highlighted
// lanes are exactly RouteLanes'; the line gives the lanes and days; a
// right click cancels; a click on a reachable star gives the GoToSystem
// order and pops back to the system.
func TestNavigatePick(t *testing.T) {
	a, v, colony, _, raider := navFixture(t, true)
	// Out of combat (the Raider gone): the order drops the ship.
	foe := a.g.Empires[1]
	foe.Ships = slices.DeleteFunc(foe.Ships, func(s *game.Starship) bool { return s == raider })
	if err := v.navigate(a); err != nil {
		t.Fatal(err)
	}
	m := top[*starmapScreen](t, a)
	if popped, err := m.navUpdate(a, true); !popped || err != nil || a.top() != v {
		t.Fatalf("right click: popped %v, %v, top %T", popped, err, a.top())
	}
	if colony.Order.Kind != game.NoOrder || v.sel != colony {
		t.Fatalf("cancel gave an order: %+v, or dropped the ship (sel %v)", colony.Order, v.sel)
	}

	if err := v.navigate(a); err != nil {
		t.Fatal(err)
	}
	m = top[*starmapScreen](t, a)
	dest, route := reachable(t, a.e, colony)
	lanes, days := a.e.RouteLanes(colony, route)
	m.hover = dest
	m.nav.at(a.e, dest)
	if !slices.Equal(m.nav.route, route) || !slices.Equal(m.nav.lanes, lanes) || m.nav.days != days {
		t.Fatalf("cache: route %v lanes %v days %d; model %v %v %d", m.nav.route, m.nav.lanes, m.nav.days, route, lanes, days)
	}
	for li := range a.cl.Lanes {
		if m.navLane(li) != slices.Contains(lanes, li) {
			t.Errorf("lane %d: highlighted %v", li, m.navLane(li))
		}
	}
	if l, bad := m.nav.line(a.e); bad || l != fmt.Sprintf("%s: %d lanes, ~%d days", a.cl.Stars[dest].Name, len(route), days) {
		t.Errorf("line %q (bad %v)", l, bad)
	}
	m.hover = -1
	if m.navLane(lanes[0]) {
		t.Error("a lane highlighted with no star hovered")
	}
	m.hover = dest

	// The same star on the same day reads the cache; a new day recomputes.
	m.nav.route = []int{-7}
	m.nav.at(a.e, dest)
	if !slices.Equal(m.nav.route, []int{-7}) {
		t.Error("recomputed for the same star and day")
	}
	a.e.Day++
	m.nav.at(a.e, dest)
	if !slices.Equal(m.nav.route, route) {
		t.Errorf("not recomputed on a new day: %v", m.nav.route)
	}
	a.e.Day--

	ps := m.project(a)
	if pickStar(ps, ps[dest].X, ps[dest].Y) != dest {
		t.Fatal("fixture: the destination is not the star under its own point")
	}
	if err := m.click(a, ps[dest].X, ps[dest].Y); err != nil {
		t.Fatal(err)
	}
	if colony.Order != (game.Order{Kind: game.GoToSystem, Star: dest}) || a.top() != v || v.sel != nil || v.last != colony {
		t.Errorf("click: order %+v, top %T, sel %v: the order drops it", colony.Order, a.top(), v.sel)
	}
	if got, want := shipOrder(a.e, colony), fmt.Sprintf("-> %s (%d lanes)", a.cl.Stars[dest].Name, len(route)); got != want {
		t.Errorf("panel order %q, want %q", got, want)
	}
	if got := orderText(a.e, colony); got != shipOrder(a.e, colony) {
		t.Errorf("fleet order %q", got)
	}
}

// Unreachable stars refuse: an unexplored one toasts the refusal, no
// order, the pick mode kept. A ship gone meanwhile ends the mode.
func TestNavigateRefusals(t *testing.T) {
	a, v, colony, _, _ := navFixture(t, false)
	if err := v.navigate(a); err != nil {
		t.Fatal(err)
	}
	m := top[*starmapScreen](t, a)
	ps := m.project(a)
	far := -1
	for i := range a.cl.Stars {
		if !a.e.Knows(i) && pickStar(ps, ps[i].X, ps[i].Y) == i {
			far = i
			break
		}
	}
	if far < 0 {
		t.Fatal("fixture: no unexplored star")
	}
	if err := m.click(a, ps[far].X, ps[far].Y); err != nil || a.top() != m || colony.Order.Kind != game.NoOrder {
		t.Fatalf("unexplored click: %v, top %T, order %+v", err, a.top(), colony.Order)
	}
	if m.nav.err == nil || a.toast != m.nav.err.Error() || !a.toastErr {
		t.Errorf("toast %q (err %v), cache %v", a.toast, a.toastErr, m.nav.err)
	}
	if _, bad := m.nav.line(a.e); !bad {
		t.Error("an unreachable star's line not in errColour")
	}
	if err := a.e.Abandon(colony); err != nil {
		t.Fatal(err)
	}
	if popped, err := m.navUpdate(a, false); !popped || err != nil || a.top() != v {
		t.Errorf("ship gone: popped %v, %v, top %T", popped, err, a.top())
	}
}
