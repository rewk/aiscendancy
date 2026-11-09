package game

import "testing"

// Four shots measured in the original (test saves 12-16, day 1488): a
// Nanoshell target with 2·ShieldDraw + 1 energy after its reservation.
// Two Mass Barrage Gun hits are absorbed, each costing ShieldDraw; the
// third finds the energy short of the draw: the shields go dark, no hull
// damage, the energy kept; a Plasmatron then does its full Level. A
// Plasmatron absorbed costs the same ShieldDraw (whatever the Level).
// Off while dark refunds the reservation; the next day's refill holds it
// again, the shields up.
func TestShieldKnockDown(t *testing.T) {
	shooter := Design{Name: "Shooter", Hull: Small, Parts: []Gizmo{QuarkExpress, TonklinMotor, MassBarrageGun, Plasmatron}}
	target := Design{Name: "Testweapons", Hull: Medium, Parts: []Gizmo{Nanotwirler, TonklinMotor, Nanoshell}}
	mbg, pl, ns := Gizmos[MassBarrageGun], Gizmos[Plasmatron], Gizmos[Nanoshell]
	draw := target.ShieldDraw()
	if draw != ns.Power || draw < 2 || mbg.Level >= ns.Level || pl.Level >= ns.Level || pl.Level <= mbg.Level || target.PowerPerDay() < draw {
		t.Fatalf("fixture: draw %d, levels %d %d vs %d, supply %d", draw, mbg.Level, pl.Level, ns.Level, target.PowerPerDay())
	}
	g, sh, tg := duel(shooter, target)
	energy, hull := 2*draw+1, pl.Level+2
	tg.Power, tg.Hull = energy, hull
	shoot := func(n int, w Gizmo, wantDmg, wantEnergy int, wantDown bool) {
		t.Helper()
		sh.Power, sh.Shots = shooter.PowerPerDay(), make([]int, len(shooter.Parts))
		dmg, _, err := fire(g, sh, part(shooter, w), tg)
		if err != nil || dmg != wantDmg || tg.Power != wantEnergy || tg.ShieldsDown != wantDown || !tg.Shields {
			t.Fatalf("shot %d %s: damage %d (err %v), energy %d, down %v, shields %v; want %d, %d, %v",
				n, Gizmos[w].Name, dmg, err, tg.Power, tg.ShieldsDown, tg.Shields, wantDmg, wantEnergy, wantDown)
		}
	}
	shoot(1, MassBarrageGun, shotDamage(mbg.Level, ns.Level), energy-draw, false)
	shoot(2, MassBarrageGun, shotDamage(mbg.Level, ns.Level), energy-2*draw, false)
	shoot(3, MassBarrageGun, 0, energy-2*draw, true)
	if tg.Absorb() != 0 || tg.Hull != hull {
		t.Fatalf("knocked down: absorb %d, hull %d of %d", tg.Absorb(), tg.Hull, hull)
	}
	shoot(4, Plasmatron, pl.Level, energy-2*draw, true)

	// Off while dark gives the reservation back.
	if err := tg.SetShields(false); err != nil || tg.Shields || tg.Power != energy-draw {
		t.Fatalf("off while dark: %v, shields %v, energy %d, want %d", err, tg.Shields, tg.Power, energy-draw)
	}

	// A fresh target: a Plasmatron absorbed costs the same draw.
	g, sh, tg = duel(shooter, target)
	tg.Power = energy
	shoot(1, Plasmatron, shotDamage(pl.Level, ns.Level), energy-draw, false)

	// The next day after a knock-down, the shields left on: up again,
	// the reservation held off the refill.
	tg.Power = draw - 1
	shoot(2, MassBarrageGun, 0, draw-1, true)
	g.EndDay()
	if !tg.Shields || tg.ShieldsDown || tg.Power != target.PowerPerDay()-draw || tg.Absorb() != ns.Level {
		t.Errorf("next day: shields %v down %v energy %d absorb %d; want on, up, %d, %d",
			tg.Shields, tg.ShieldsDown, tg.Power, tg.Absorb(), target.PowerPerDay()-draw, ns.Level)
	}
}

// A shot stronger than the shield passes its excess, the draw paid
// (measured, test saves 12-13 day 1502: a Nanomanipulator, Level 13, on a
// Medium behind a Nanoshell on, energy 7 → 4, hull 20 → 12); an
// Invulnerable target takes nothing and its shields pay nothing.
func TestShieldExcess(t *testing.T) {
	shooter := Design{Name: "Shooter", Hull: Medium, Parts: []Gizmo{Nanotwirler, Nanotwirler, TonklinMotor, Nanomanipulator}}
	target := Design{Name: "Testweapons", Hull: Medium, Parts: []Gizmo{Nanotwirler, TonklinMotor, Nanoshell}}
	nm, ns := Gizmos[Nanomanipulator], Gizmos[Nanoshell]
	draw := target.ShieldDraw()
	if nm.Level <= ns.Level || draw < 1 || shooter.PowerPerDay() < nm.Power {
		t.Fatalf("fixture: Nanomanipulator %d vs Nanoshell %d, draw %d, supply %d", nm.Level, ns.Level, draw, shooter.PowerPerDay())
	}
	g, sh, tg := duel(shooter, target)
	energy, hull := 2*draw+1, tg.Hull
	tg.Power = energy
	if dmg, _, err := fire(g, sh, part(shooter, Nanomanipulator), tg); err != nil || dmg != nm.Level-ns.Level || tg.Power != energy-draw || tg.ShieldsDown || tg.Hull != hull-dmg {
		t.Errorf("excess: damage %d (err %v), energy %d, down %v; want %d, %d, up", dmg, err, tg.Power, tg.ShieldsDown, nm.Level-ns.Level, energy-draw)
	}
	g, sh, tg = duel(shooter, target)
	tg.Power, tg.Invulnerable = energy, true
	if dmg, _, err := fire(g, sh, part(shooter, Nanomanipulator), tg); err != nil || dmg != 0 || tg.Power != energy || tg.ShieldsDown {
		t.Errorf("invulnerable: damage %d (err %v), energy %d, down %v", dmg, err, tg.Power, tg.ShieldsDown)
	}
}

// The round's hits land after every shot is paid: two ships firing at
// each other in one round, each with the energy for its shot or its
// shields' draw but not both, both fire whatever the seat order, then
// both shields go dark.
func TestShieldDrawAfterShots(t *testing.T) {
	d := Design{Name: "Duellist", Hull: Small, Parts: []Gizmo{QuarkExpress, TonklinMotor, Plasmatron, IonWrap}}
	pl, draw := Gizmos[Plasmatron], d.ShieldDraw()
	g, a, b := duel(d, d)
	a.Power, b.Power = pl.Power+draw-1, pl.Power+draw-1
	if err := arm(t, g, a, part(d, Plasmatron), b); err != nil {
		t.Fatal(err)
	}
	if err := arm(t, g, b, part(d, Plasmatron), a); err != nil {
		t.Fatal(err)
	}
	ha, hb := a.Hull, b.Hull
	_, _, log := g.Round(a.Star, nil)
	if len(log.Shots) != 2 || a.Hull != ha || b.Hull != hb || !a.ShieldsDown || !b.ShieldsDown || a.Power != draw-1 || b.Power != draw-1 {
		t.Errorf("%d shots, hulls %d/%d %d/%d, down %v %v, energy %d %d", len(log.Shots), a.Hull, ha, b.Hull, hb, a.ShieldsDown, b.ShieldsDown, a.Power, b.Power)
	}
}

// A shot at a planet whose cascade fell earlier in the round is given
// back at no cost: its energy pays the shooter's shields' draw for a hit
// landing on it in the same round. Empire 1's siege gun kills the only
// ship in orbit of empire 0's home (Hull 1); empire 1's A, armed at that
// planet after it, gets its shot back; empire 0's B hits A, whose energy
// with the shot refunded pays the draw: absorbed, not knocked down.
func TestRefundedShotPaysTheDraw(t *testing.T) {
	g, sh, home := siege(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	d := Design{Name: "Duellist", Hull: Small, Parts: []Gizmo{QuarkExpress, TonklinMotor, Plasmatron, IonWrap}}
	pl, draw := Gizmos[Plasmatron], d.ShieldDraw()
	pp := home.Planet.Pos
	x := e0.Spawn(orbitShip, home.Star, home.PlanetIdx)
	x.Hull = 1
	a := e1.Spawn(d, home.Star, home.PlanetIdx)
	inSpaceAt(a, Vec3{X: pp.X + 0.02, Y: pp.Y, Z: pp.Z})
	b := e0.Spawn(d, home.Star, home.PlanetIdx)
	inSpaceAt(b, Vec3{X: pp.X + 0.04, Y: pp.Y, Z: pp.Z})
	a.Power = pl.Power + draw - 1
	if err := e1.ArmAtPlanet(sh, part(siegeGun, Nanomanipulator), home.PlanetIdx); err != nil {
		t.Fatal(err)
	}
	if err := e1.ArmAtPlanet(a, part(d, Plasmatron), home.PlanetIdx); err != nil {
		t.Fatal(err)
	}
	if err := arm(t, g, b, part(d, Plasmatron), a); err != nil {
		t.Fatal(err)
	}
	hull := a.Hull
	_, _, log := g.Round(home.Star, nil)
	if g.OwnerOf(x) != nil || a.Shots[part(d, Plasmatron)] != 0 {
		t.Fatalf("fixture: x alive %v, a's shots %v, log %+v", g.OwnerOf(x) != nil, a.Shots, log.Shots)
	}
	if a.ShieldsDown || a.Power != pl.Power-1 || a.Hull != hull-shotDamage(pl.Level, Gizmos[IonWrap].Level) {
		t.Errorf("a: down %v, energy %d (want %d), hull %d of %d", a.ShieldsDown, a.Power, pl.Power-1, a.Hull, hull)
	}
}
