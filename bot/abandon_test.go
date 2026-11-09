package bot

import (
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// strandedFixture: seed 1, one empire, every star known, AbandonIdle
// days, a loaded colony ship without a lane drive in orbit at a star
// with nothing foundable (it can go nowhere).
func strandedFixture(t *testing.T, days float64) (g *game.Game, b *Bot, s *game.Starship) {
	t.Helper()
	g = game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	b = NewAll(g, Config{NoResearch: true, Genome: genomeWith(func(g *Genome) { g.AbandonIdle = days })})[0]
	for st := range g.Cluster.Stars {
		b.E.Visit(st)
	}
	claimed := b.claimed()
	bare := -1
	for st := range g.Cluster.Stars {
		if st != b.E.Colonies[0].Star && b.targets(st, claimed) == 0 {
			bare = st
			break
		}
	}
	if bare < 0 {
		t.Fatal("no star without a target on seed 1")
	}
	s = b.E.Spawn(game.Design{Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.Colonizer}}, bare, 0)
	if s.Cargo < 1 || s.Design.LaneDrives() != 0 {
		t.Fatalf("fixture ship cargo %d, %d lane drives", s.Cargo, s.Design.LaneDrives())
	}
	return g, b, s
}

// A stranded colony ship is abandoned on the day its idle count reaches
// AbandonIdle, not a day earlier; AbandonIdle 0 never abandons it.
func TestAbandonStranded(t *testing.T) {
	const n = 5
	g, b, s := strandedFixture(t, n)
	bots := []*Bot{b}
	for day := 1; day < n; day++ {
		Day(g, bots)
		if !slices.Contains(b.E.Ships, s) {
			t.Fatalf("day %d of %d: abandoned early", day, n)
		}
		if s.Order.Kind != game.NoOrder || b.idleDays[s] != day {
			t.Fatalf("day %d: order %v, idle count %d: the fixture ship is not idle", day, s.Order.Kind, b.idleDays[s])
		}
	}
	Day(g, bots)
	if slices.Contains(b.E.Ships, s) {
		t.Errorf("day %d: not abandoned", n)
	}
	if _, ok := b.idleDays[s]; ok {
		t.Error("abandoned ship still counted")
	}

	g, b, s = strandedFixture(t, 0)
	bots = []*Bot{b}
	for range 3 * n {
		Day(g, bots)
	}
	if !slices.Contains(b.E.Ships, s) {
		t.Error("AbandonIdle 0: abandoned")
	}
}

// Not idle restarts the count: an order, a lane, a refit. A warship
// without Colonizer or Invasion Module is never counted; an armed one
// with a Colonizer is.
func TestAbandonIdleReset(t *testing.T) {
	const n = 4
	_, b, s := strandedFixture(t, n)
	e := b.E
	run := func(k int) {
		for range k {
			b.abandonStranded()
		}
	}
	run(n - 1)
	for _, tc := range []struct {
		name       string
		set, unset func()
	}{
		{"an order", func() { s.Order = game.Order{Kind: game.GoToPlanet, Planet: 1} }, func() { s.Order = game.Order{} }},
		{"a lane", func() { s.Lane = 0 }, func() { s.Lane = -1 }},
		{"a refit", func() { e.Colonies[0].Project = &game.Project{Item: game.Ship, Refit: s.Name} }, func() { e.Colonies[0].Project = nil }},
	} {
		tc.set()
		run(n) // never counted meanwhile (an Abandon refused would also clear it, but on its n-th Act)
		if !slices.Contains(e.Ships, s) {
			t.Fatalf("%s: abandoned", tc.name)
		}
		if _, ok := b.idleDays[s]; ok {
			t.Errorf("%s: still counted (%d)", tc.name, b.idleDays[s])
		}
		tc.unset()
		run(n - 1)
		if !slices.Contains(e.Ships, s) {
			t.Fatalf("%s then idle %d: abandoned, the count did not restart", tc.name, n-1)
		}
	}
	run(1)
	if slices.Contains(e.Ships, s) {
		t.Error("idle n Acts after the last reset: not abandoned")
	}

	war := e.Spawn(game.Design{Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun}}, s.Star, 0)
	armedCol := e.Spawn(game.Design{Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.MassBarrageGun, game.Colonizer}}, s.Star, 0)
	armedCol.Cargo = 0 // emptied: still a colony ship
	inv := e.Spawn(game.Design{Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.InvasionModule}}, s.Star, 0)
	run(n)
	if !slices.Contains(e.Ships, war) {
		t.Error("a warship without Colonizer or Invasion Module abandoned")
	}
	if slices.Contains(e.Ships, armedCol) {
		t.Error("an armed, emptied colony ship not abandoned")
	}
	if inv.Invaders < 1 || slices.Contains(e.Ships, inv) {
		t.Errorf("an invasion ship (%d modules) not abandoned", inv.Invaders)
	}
}

// The fixed policy and Neutral wait abandonIdle days; Clamp bounds the
// gene to [0, 1000].
func TestAbandonIdleGene(t *testing.T) {
	if g := Neutral(); g.AbandonIdle != abandonIdle || (&Bot{Cfg: Config{Genome: g}}).abandonAfter() != abandonIdle {
		t.Errorf("Neutral AbandonIdle %v, want %d", g.AbandonIdle, abandonIdle)
	}
	if got := (&Bot{}).abandonAfter(); got != abandonIdle {
		t.Errorf("fixed policy: %d, want %d", got, abandonIdle)
	}
	g := Neutral()
	for _, c := range [][2]float64{{-5, 0}, {0, 0}, {60.4, 60.4}, {5000, 1000}} {
		g.AbandonIdle = c[0]
		if g.Clamp(); g.AbandonIdle != c[1] {
			t.Errorf("Clamp(%v) = %v, want %v", c[0], g.AbandonIdle, c[1])
		}
	}
	if got := (&Bot{Cfg: Config{Genome: &Genome{AbandonIdle: 59.6}}}).abandonAfter(); got != 60 {
		t.Error("AbandonIdle 59.6 not rounded to 60")
	}
}
