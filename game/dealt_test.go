package game

import (
	"slices"
	"testing"
)

// The damage ledger (Starship.Dealt): a round's hit lands in the target's
// Dealt for the shooter's empire only, both ways in one round; the
// Sighting carries the seeing empire's entry, scanned or not.
func TestDealtRound(t *testing.T) {
	g, s0, s1 := roundDuel(oneGen, oneGen)
	gun := part(oneGen, MassBarrageGun)
	if err := arm(t, g, s0, gun, s1); err != nil {
		t.Fatal(err)
	}
	if err := arm(t, g, s1, gun, s0); err != nil {
		t.Fatal(err)
	}
	g.Round(s0.Star, nil)
	d0, d1 := s1.MaxHull()-s1.Hull, s0.MaxHull()-s0.Hull // what 0 and 1 dealt
	if d0 <= 0 || d1 <= 0 {
		t.Fatalf("fixture: no damage (%d, %d)", d0, d1)
	}
	if !slices.Equal(s1.Dealt, []int{d0}) || !slices.Equal(s0.Dealt, []int{0, d1}) {
		t.Errorf("ledgers %v %v, want [%d] [0 %d]", s1.Dealt, s0.Dealt, d0, d1)
	}
	for _, c := range []struct {
		e    *Empire
		t    *Starship
		want int
	}{{g.Empires[0], s1, d0}, {g.Empires[1], s0, d1}} {
		sg, ok := g.SightingOf(c.e, c.t)
		if !ok || sg.Dealt != c.want {
			t.Errorf("empire %d sees %s: %v Dealt %d, want %d", c.e.Index, c.t.Name, ok, sg.Dealt, c.want)
		}
	}
}

// A planet's orbital weapon books its damage for the colony's empire.
func TestDealtPlanet(t *testing.T) {
	g, foe, home := siege(t)
	e0 := g.Empires[0]
	p := home.Planet
	lr := 4
	if err := p.BuildOrbital(lr, LongRangeOrbitalWhopper); err != nil {
		t.Fatal(err)
	}
	inSpaceAt(foe, Vec3{X: p.Pos.X + 0.05, Y: p.Pos.Y, Z: p.Pos.Z})
	sg, _ := g.SightingOf(e0, foe)
	if err := e0.ArmPlanet(home, lr, sg); err != nil {
		t.Fatal(err)
	}
	g.Round(home.Star, nil)
	if d := foe.MaxHull() - foe.Hull; d <= 0 || !slices.Equal(foe.Dealt, []int{d}) {
		t.Errorf("hull %d of %d, Dealt %v", foe.Hull, foe.MaxHull(), foe.Dealt)
	}
}

// Every heal clamps each ledger entry to MaxHull − Hull: the Remote
// Repair Facility (part way), the refit (full hull).
func TestDealtHealed(t *testing.T) {
	d := Design{Hull: Medium, Parts: []Gizmo{Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, TonklinMotor, RemoteRepairFacility}}
	_, e, s := specialShip(t, d)
	in := &Gizmos[RemoteRepairFacility]
	full := s.MaxHull()
	s.Hull = full - in.Special1 - 2
	s.Dealt = []int{0, in.Special1 + 2, 1}
	useOK(t, e, s, part(d, RemoteRepairFacility))
	if gone := full - s.Hull; !slices.Equal(s.Dealt, []int{0, gone, 1}) {
		t.Errorf("repaired to %d of %d: Dealt %v, want [0 %d 1]", s.Hull, full, s.Dealt, gone)
	}
	applyRefit(s, d)
	if !slices.Equal(s.Dealt, []int{0, 0, 0}) {
		t.Errorf("refitted: Dealt %v", s.Dealt)
	}
}

// A special's hull damage (hurt: the Backfirer here) books it for the
// user's empire.
func TestDealtSpecial(t *testing.T) {
	d := aimer(Backfirer)
	_, e, s, foe, tg := aimGame(t, d, foeShip, 0.1)
	useAt(t, e, s, part(d, Backfirer), tg)
	if dm := foe.MaxHull() - foe.Hull; dm <= 0 || !slices.Equal(foe.Dealt, []int{dm}) {
		t.Errorf("hull %d of %d, Dealt %v", foe.Hull, foe.MaxHull(), foe.Dealt)
	}
}
