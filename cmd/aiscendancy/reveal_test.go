package main

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// -reveal: a game set on the window has every star explored, and every
// foreign ship out of the lanes is seen and scanned, out of any scanner's
// range; without it the fog stands.
func TestReveal(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	o := g.Empires[1]
	foe := o.Spawn(foeShip, o.Colonies[0].Star, 0)
	a := testApp(g)
	if a.e.Knows(foe.Star) {
		t.Fatal("the fixture: the foe's home is known without the cheat")
	}
	a.reveal = true
	a.setGame(g, 0)
	for st := range g.Cluster.Stars {
		if !a.e.Knows(st) {
			t.Fatalf("star %d unexplored with -reveal", st)
		}
	}
	if sg, ok := g.SightingOf(a.e, foe); !ok || sg.Scan == nil {
		t.Errorf("the foe's ship with -reveal: seen %v, scanned %v", ok, ok && sg.Scan != nil)
	}
	a.reveal = false
	a.setGame(g, 0)
	if sg, _ := g.SightingOf(a.e, foe); sg.Scan != nil {
		t.Error("still scanned once the cheat is off")
	}
}
