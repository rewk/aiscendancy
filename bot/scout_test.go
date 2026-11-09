package bot

import (
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// The scout: ScoutShip is one (a lane drive, a scanner, nothing else
// carried); none starts at one system; an idle one heads for an
// unexplored star, claimed, else home; once every star is known it is
// abandoned. Told by its name: parts lost change no role.
func TestScout(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Orfa, game.Mebes}, game.Dense)
	b := NewAll(g, Config{NoResearch: true})[0]
	b.E.Research = allKnown()
	d, ok := game.ScoutShip(b.E.Research)
	if !ok || !explorer(d) || d.LaneDrives() == 0 || !slices.ContainsFunc(d.Parts, func(p game.Gizmo) bool { return game.Gizmos[p].Cat == game.Scanner }) {
		t.Fatalf("ScoutShip %v %v", d, ok)
	}
	if c, _ := game.ColonyShip(b.E.Research, true); explorer(c) {
		t.Error("a colony ship taken for a scout")
	}
	w, _ := game.Warship(b.E.Research, true)
	if explorer(w) {
		t.Error("a warship taken for a scout")
	}
	// Parts lost (LosePart): a disarmed warship stays a warship, a scout
	// without its scanner a scout.
	ws, ss := b.E.Spawn(w, b.E.Colonies[0].Star, 0), b.E.Spawn(d, b.E.Colonies[0].Star, 0)
	for k := len(ws.Design.Parts) - 1; k >= 0; k-- {
		if game.Gizmos[ws.Design.Parts[k]].Cat == game.Weapon {
			ws.LosePart(k)
		}
	}
	ss.LosePart(slices.IndexFunc(ss.Design.Parts, func(p game.Gizmo) bool { return game.Gizmos[p].Cat == game.Scanner }))
	if ws.Design.Weapons() != 0 || explorer(ws.Design) || !explorer(ss.Design) {
		t.Errorf("parts lost: warship %v (scout %v), scout %v", ws.Design.Parts, explorer(ws.Design), explorer(ss.Design))
	}
	b.E.Abandon(ws)
	b.E.Abandon(ss)
	b.assess()
	claimed := b.claimed()
	if b.wantScout(claimed) {
		t.Error("a scout wanted at one system")
	}
	home := b.E.Colonies[0]
	s := b.E.Spawn(d, home.Star, home.PlanetIdx)
	b.explore(s, claimed)
	to := b.dest[s]
	if s.Order.Kind != game.GoToStar || b.E.Knows(to) || !claimed[place{to, -1}] {
		t.Fatalf("explore: order %v, dest %d known %v", s.Order, to, b.E.Knows(to))
	}
	s.Order = game.Order{}
	b.retireScouts()
	if !slices.Contains(b.E.Ships, s) {
		t.Fatal("retired with stars unknown")
	}
	// No frontier left (all claimed) away from home: it heads back to a
	// colony's star; at home it waits.
	for st := range g.Cluster.Stars {
		claimed[place{st, -1}] = true
	}
	away := g.Cluster.Other(g.Cluster.Stars[home.Star].Lanes[0], home.Star)
	s.Star, s.Orbit = away, -1
	b.explore(s, claimed)
	if s.Order.Kind != game.GoToStar || s.Order.Star != home.Star {
		t.Errorf("no frontier, one lane from home: order %+v, want home %d", s.Order, home.Star)
	}
	s.Order, s.Star = game.Order{}, home.Star
	if b.explore(s, claimed); s.Order.Kind != game.NoOrder {
		t.Errorf("no frontier, at home: order %+v", s.Order)
	}
	for st := range g.Cluster.Stars {
		b.E.Visit(st)
	}
	b.retireScouts()
	if slices.Contains(b.E.Ships, s) {
		t.Error("not retired, every star known")
	}
}
