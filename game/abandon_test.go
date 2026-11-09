package game

import (
	"strings"
	"testing"
)

// Abandon refuses the Colony Base and an empty square and demolishes any
// other structure; AbandonOrbital keeps the last Shipyard while a ship
// is built and the last Orbital Docks while a refit runs, and lets one
// go when another stands.
func TestAbandon(t *testing.T) {
	e, home := shipEmpire(t)
	p := home.Planet
	base := Cell{I: Centre, J: Centre}
	if err := home.Abandon(base); err == nil || p.Surface[base].Item != ColonyBase {
		t.Errorf("the Colony Base: %v, %s", err, p.Surface[base].Item)
	}
	empty, ok := p.BestCell(Agriplot, Base)
	if !ok {
		t.Fatal("no empty square")
	}
	if err := home.Abandon(empty); err == nil {
		t.Error("an empty square abandoned")
	}
	var f Cell
	found := false
	for _, c := range p.Cells() {
		if p.Surface[c].Item == Factory {
			f, found = c, true
			break
		}
	}
	if !found {
		t.Fatal("no Factory")
	}
	if err := home.Abandon(f); err != nil || p.Surface[f].Item != NoItem {
		t.Errorf("a Factory: %v, %s", err, p.Surface[f].Item)
	}
	// The Shipyard (slot 0) under a Ship project.
	d, ok := ColonyShip(e.Research, false)
	if !ok {
		t.Fatal("no colony ship")
	}
	if err := e.BuildShip(home, d); err != nil {
		t.Fatal(err)
	}
	if err := home.AbandonOrbital(0); err == nil || !strings.Contains(err.Error(), "in use") || p.Orbit[0] != Shipyard {
		t.Errorf("the last Shipyard while a ship is built: %v, slot %s", err, p.Orbit[0])
	}
	if err := home.AbandonOrbital(OrbitSlots - 1); err == nil {
		t.Error("an empty slot abandoned")
	}
	if err := p.BuildOrbital(OrbitSlots-2, Shipyard); err != nil {
		t.Fatal(err)
	}
	if err := home.AbandonOrbital(0); err != nil || p.Orbit[0] != NoItem {
		t.Errorf("a second Shipyard: %v, slot %s", err, p.Orbit[0])
	}
	// The Docks (slot 1) under a refit.
	e2, c2, s := refitFixture(t)
	if err := e2.Refit(c2, s, withGen(s)); err != nil {
		t.Fatal(err)
	}
	if err := c2.AbandonOrbital(1); err == nil || !strings.Contains(err.Error(), "in use") || c2.Planet.Orbit[1] != OrbitalDocks {
		t.Errorf("the last Docks while a refit runs: %v, slot %s", err, c2.Planet.Orbit[1])
	}
	if err := c2.Planet.BuildOrbital(OrbitSlots-2, OrbitalDocks); err != nil {
		t.Fatal(err)
	}
	if err := c2.AbandonOrbital(1); err != nil || c2.Planet.Orbit[1] != NoItem {
		t.Errorf("a second Docks: %v, slot %s", err, c2.Planet.Orbit[1])
	}
}

// Order, CanFound and LeaveOrbit refuse another empire's ship, which
// stays as it was.
func TestNotOurShip(t *testing.T) {
	e, home := shipEmpire(t)
	d, ok := ColonyShip(e.Research, false)
	if !ok {
		t.Fatal("no colony ship")
	}
	s := e.Spawn(d, home.Star, home.PlanetIdx)
	cargo := s.Cargo
	other := NewEmpire(2, Orfa, Average)
	for name, err := range map[string]error{
		"Order":      other.Order(s, Order{Kind: Colonise}),
		"CanFound":   other.CanFound(s),
		"LeaveOrbit": other.LeaveOrbit(s, nil),
	} {
		if err != errNotOurs {
			t.Errorf("%s: %v", name, err)
		}
	}
	if s.Orbit != home.PlanetIdx || s.Order != (Order{}) || s.Cargo != cargo {
		t.Errorf("the ship changed: orbit %d, order %+v, cargo %d", s.Orbit, s.Order, s.Cargo)
	}
}
