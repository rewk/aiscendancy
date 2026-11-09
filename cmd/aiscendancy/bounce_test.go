package main

import (
	"math"
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// A hop that Hop stops short of a shielded planet's orbit is drawn as a
// bounce: in to bounceTouch of the planet at bounceIn of a longer anim,
// back out to the model's stop, the bounce's sound at the turn and the
// status line saying why; the ship held there hops no more.
func TestBounceHop(t *testing.T) {
	g := game.NewGame(2, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	e := g.Empires[0]
	home := e.Cluster.Home
	sh := e.Spawn(fixtureShip, home, 0)
	sh.Orbit = -1
	pls := g.Cluster.Stars[home].System.Planets
	pl := slices.IndexFunc(pls, func(p *game.Planet) bool { return p.Owner < 0 })
	if pl < 0 {
		t.Fatal("no free planet at home")
	}
	p := pls[pl]
	p.Owner, p.Orbit[0] = 1, game.OrbitalShield
	if p.Name == "" {
		p.Name = "Target"
	}
	if !e.Barred(home, pl) {
		t.Fatal("not barred")
	}
	v := newSystemScreen(home)
	v.Enter(testApp(g))
	if err := e.Order(sh, game.Order{Kind: game.GoToPlanet, Planet: pl}); err != nil {
		t.Fatal(err)
	}
	dist := func(a, b game.Vec3) float64 { return math.Hypot(math.Hypot(a.X-b.X, a.Y-b.Y), a.Z-b.Z) }
	for k := 0; !v.anim.bounce; k++ {
		if k > 100 {
			t.Fatalf("no bounce after 100 hops: at %v, %v from the planet", sh.Pos, dist(sh.Pos, p.Pos))
		}
		v.cues = nil
		sh.Power = sh.Design.PowerPerDay()
		if !v.hop(e, sh) {
			t.Fatalf("hop %d: no move at %v", k, sh.Pos)
		}
		if !v.anim.bounce {
			chain(v, e)
		}
	}
	m := v.anim
	if d := dist(m.touch, p.Pos); math.Abs(d-bounceTouch) > 1e-9 || m.to != sh.Pos || m.ticks() != hopTicks*3/2 {
		t.Errorf("touch %v from the planet, want %v; to %v, ship at %v; %d ticks", d, bounceTouch, m.to, sh.Pos, m.ticks())
	}
	turn := m.t0 + int(bounceIn*float64(m.ticks()))
	if !slices.Contains(v.cues, cue{name: bounceSound, t: turn}) {
		t.Errorf("cues %+v, want %s at %d", v.cues, bounceSound, turn)
	}
	if want := p.Name + "'s orbit is shielded"; v.msg != want {
		t.Errorf("status %q, want %q", v.msg, want)
	}
	v.tick = turn
	if d := dist(v.animPos(), m.touch); d > 1e-9 {
		t.Errorf("at the turn %v off the touch", d)
	}
	chain(v, e)
	sh.Power = sh.Design.PowerPerDay()
	if v.hop(e, sh) || sh.Order.Kind != game.GoToPlanet {
		t.Errorf("held ship hopped again, order %+v", sh.Order)
	}
}
