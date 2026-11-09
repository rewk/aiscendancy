package bot

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// colonyShip: none without spare; with Colonizers in flight the smallest
// hull's, within inFlightMax; with none in flight at a new home the
// smallest hull's (no larger one builds within ColonyDays), and with
// Megafacilities everywhere the hull carrying the most within it, at
// most spare.
func TestColonyShipHull(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Orfa}, game.Average)
	b := New(g.Empires[0], Config{NoResearch: true})
	b.E.Research = allKnown()
	home := b.E.Colonies[0]
	small, _ := game.ColonyShip(b.E.Research, true)
	b.spare, b.inFlight = 0, 0
	if _, ok := b.colonyShip(home); ok {
		t.Error("a colony ship without spare")
	}
	b.spare, b.inFlight = 20, 1
	if d, ok := b.colonyShip(home); ok != (1+small.Colonizers() <= b.inFlightMax()) || ok && d.Hull != small.Hull {
		t.Errorf("one in flight: %v %v", d.Hull, ok)
	}
	b.inFlight = 0
	if d, ok := b.colonyShip(home); !ok || d.Hull != small.Hull {
		t.Errorf("a new home: %v %v, want %v", d.Hull, ok, small.Hull)
	}
	for _, q := range home.Planet.Surface {
		if q.Item == game.NoItem {
			q.Item, q.Automated = game.IndustrialMegafacility, true
		}
	}
	most, hull := 0, game.Hull(0)
	for h := game.Hull(0); h < game.NumHulls; h++ {
		d, ok := game.ColonyShipOn(b.E.Research, h, true, b.spare)
		if n, ok2 := home.DaysFor(d.Cost()); ok && ok2 && n <= ColonyDays && d.Colonizers() > most {
			most, hull = d.Colonizers(), h
		}
	}
	if hull == small.Hull {
		t.Fatal("Megafacilities everywhere: no larger hull within ColonyDays")
	}
	if d, ok := b.colonyShip(home); !ok || d.Hull != hull || d.Colonizers() != most || most > b.spare {
		t.Errorf("Megafacilities everywhere: %v with %d, want %v with %d", d.Hull, d.Colonizers(), hull, most)
	}
}
