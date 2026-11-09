package main

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// scopeApp is seed 1, one empire, the starmap open; home its star, other
// another star (neither home nor holding a colony of the player's).
func scopeApp(t *testing.T) (a *app, home, other int) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	a = testApp(g)
	a.push(newStarmapScreen())
	home = a.cl.Home
	if len(a.cl.Stars[home].System.Planets) < 2 || a.e.Colony(home, 0) == nil {
		t.Fatal("fixture: the home star needs its colony on planet 0 and a second planet")
	}
	other = (home + 1) % len(a.cl.Stars)
	return a, home, other
}

// ev is an event of kind k at (star, planet), Other −1, no item.
func ev(k game.EventKind, star, planet int) game.Event {
	return game.Event{Kind: k, Item: game.NoItem, Tech: game.NoTech, Star: star, Planet: planet, Other: -1}
}

// Which screen covers which event: the starmap and the log all; a
// system its star's (planets included); a planet its own; the planets
// list the colony kinds and what happens on a planet of ours; the fleet
// the ship kinds; the species screens the diplomacy; other screens
// nothing. Root kinds (diplomacy, AbilityUsed, Defeated) are no
// system's, whatever their Star.
func TestCovers(t *testing.T) {
	a, home, other := scopeApp(t)
	builtShip := ev(game.Built, other, 0)
	builtShip.Item = game.Ship
	war := ev(game.WarDeclared, -1, -1)
	war.Other = 1
	defeated := ev(game.Defeated, home, -1)
	defeated.Other = 1
	ours := ev(game.Defeated, home, -1) // our own defeat: Other −1
	events := []struct {
		name string
		ev   game.Event
	}{
		{"same planet", ev(game.Built, home, 0)},
		{"other planet, same star", ev(game.Built, home, 1)},
		{"same star, no planet", ev(game.Arrived, home, -1)},
		{"other star", ev(game.Built, other, 0)},
		{"root diplomacy", war},
		{"their defeat", defeated},
		{"our defeat", ours},
		{"ability", ev(game.AbilityUsed, -1, -1)},
		{"ship kind elsewhere", ev(game.Destroyed, other, -1)},
		{"ship built elsewhere", builtShip},
		{"colony kind elsewhere", ev(game.Grew, other, 0)},
		{"arrival at our colony", ev(game.Arrived, home, 0)},
		{"arrival elsewhere", ev(game.Arrived, other, 0)},
	}
	screens := []struct {
		name string
		sc   screen
		want []bool // by events
	}{
		{"starmap", newStarmapScreen(), []bool{true, true, true, true, true, true, true, true, true, true, true, true, true}},
		{"log", newLogScreen(), []bool{true, true, true, true, true, true, true, true, true, true, true, true, true}},
		{"system home", newSystemScreen(home), []bool{true, true, true, false, false, false, false, false, false, false, false, true, false}},
		{"planet home 0", newPlanetScreen(home, 0), []bool{true, false, false, false, false, false, false, false, false, false, false, true, false}},
		{"planets list", newPlanetsScreen(), []bool{true, true, false, true, false, false, false, false, false, true, true, true, false}},
		{"fleet", newFleetScreen(), []bool{false, false, true, false, false, false, false, false, true, true, false, true, true}},
		{"species", newSpeciesScreen(-1), []bool{false, false, false, false, true, true, false, false, false, false, false, false, false}},
		{"negotiation", newNegotiateScreen(1), []bool{false, false, false, false, true, true, false, false, false, false, false, false, false}},
		{"research", newResearchScreen(), make([]bool, len(events))},
		{"design", newDesignScreen(), make([]bool, len(events))},
	}
	for _, s := range screens {
		for k, e := range events {
			if got := covers(a, s.sc, e.ev); got != s.want[k] {
				t.Errorf("%s covers %s: %v, want %v", s.name, e.name, got, s.want[k])
			}
		}
	}
	if navigable(ev(game.Discovered, -1, -1)) || !navigable(ev(game.Discovered, home, 0)) || !navigable(war) {
		t.Error("navigable: only a Discovered with no place is ignored")
	}
	for _, c := range []struct {
		ev   game.Event
		want evPlace
	}{
		{ev(game.Built, home, 1), evPlace{home, 1}}, {ev(game.Arrived, home, -1), evPlace{home, -1}},
		{war, evPlace{-1, -1}}, {defeated, evPlace{-1, -1}}, {ev(game.Built, -1, -1), evPlace{-1, -1}},
	} {
		if got := placeOf(c.ev); got != c.want {
			t.Errorf("placeOf(%v at %d,%d) = %v, want %v", c.ev.Kind, c.ev.Star, c.ev.Planet, got, c.want)
		}
	}
}

// The climb on stacks: it stays when the top covers the day, else pops
// down to the nearest covering ancestor in the tree, kept as it was, or
// to the starmap with a fresh system screen over it.
func TestNavigate(t *testing.T) {
	a, home, other := scopeApp(t)
	m := a.stack[0]
	sys := newSystemScreen(home)
	pl := newPlanetScreen(home, 0)
	reset := func() {
		a.stack = []screen{m}
		a.push(sys)
		a.push(pl)
		sys.cam.Dist = 7 // a camera the climb must keep
	}
	reset()
	a.navigate([]game.Event{ev(game.Built, home, 0), ev(game.Discovered, -1, -1)})
	if len(a.stack) != 3 || a.top() != screen(pl) {
		t.Fatalf("same planet: stack %v", a.stack)
	}
	a.navigate([]game.Event{ev(game.Built, home, 1), ev(game.Built, home, 0)})
	if len(a.stack) != 2 || a.top() != screen(sys) || sys.cam.Dist != 7 {
		t.Fatalf("other planet of the star: stack %v, want the same system screen", a.stack)
	}
	reset()
	a.navigate([]game.Event{ev(game.Arrived, home, -1)})
	if len(a.stack) != 2 || a.top() != screen(sys) {
		t.Fatalf("the star, no planet: stack %v, want the system screen", a.stack)
	}
	for _, evs := range [][]game.Event{
		{ev(game.Built, other, 0)},
		{ev(game.Built, home, 0), ev(game.Built, other, 0)},
		{{Kind: game.WarDeclared, Star: -1, Planet: -1, Other: 1}},
		{{Kind: game.Defeated, Star: home, Planet: -1, Other: 1}},
	} {
		reset()
		a.navigate(evs)
		if len(a.stack) != 1 || a.top() != m {
			t.Errorf("%v: stack %v, want the starmap alone", evs, a.stack)
		}
	}
	// From a system screen anything elsewhere climbs to the starmap.
	a.stack = []screen{m, sys}
	a.navigate([]game.Event{ev(game.Built, other, 0)})
	if len(a.stack) != 1 || a.top() != m {
		t.Errorf("system screen, another star: stack %v", a.stack)
	}

	// starmap > planets list > planet: the system is not in the stack, a
	// fresh one goes over the starmap.
	a.stack = []screen{m}
	a.push(newPlanetsScreen())
	a.push(newPlanetScreen(home, 0))
	a.navigate([]game.Event{ev(game.Built, home, 1)})
	if len(a.stack) != 2 || a.stack[0] != m {
		t.Fatalf("planets list > planet: stack %v", a.stack)
	}
	if v, ok := a.top().(*systemScreen); !ok || v.star != home || v == sys || v.sys == nil {
		t.Errorf("planets list > planet: top %#v, want a fresh, entered system screen of %d", a.top(), home)
	}

	// The lists and the species screens keep what they cover.
	for _, c := range []struct {
		sc screen
		ev game.Event
	}{
		{newPlanetsScreen(), ev(game.Grew, other, 0)},
		{newFleetScreen(), ev(game.Destroyed, other, -1)},
		{newSpeciesScreen(-1), game.Event{Kind: game.PeaceOffered, Star: -1, Planet: -1, Other: 1}},
		{newResearchScreen(), ev(game.Discovered, -1, -1)},
	} {
		a.stack = []screen{m}
		a.push(c.sc)
		if a.navigate([]game.Event{c.ev}); len(a.stack) != 2 || a.top() != c.sc {
			t.Errorf("%T with %v: stack %v", c.sc, c.ev.Kind, a.stack)
		}
		if a.navigate([]game.Event{c.ev, ev(game.Refitted, other, 0), ev(game.Invaded, other, 1)}); len(a.stack) != 1 {
			t.Errorf("%T with an uncovered event: stack %v", c.sc, a.stack)
		}
	}

	// No starmap at the bottom (defensive): nothing changes.
	a.stack = []screen{sys, pl}
	a.navigate([]game.Event{ev(game.Built, other, 0)})
	if len(a.stack) != 2 || a.top() != screen(pl) {
		t.Errorf("no starmap: stack %v", a.stack)
	}
}
