package bot

import (
	"math"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// ruinsFixture: seed 1, one Snovemdomas empire playing Default (research
// off) with genome gn, every star known, every generated ruin cleared:
// the tests lay their own. The floor is set as claimed would (setFloor):
// the oracles below (targets, foundable) read it.
func ruinsFixture(t *testing.T, gn *Genome) (b *Bot, home *game.Colony) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	cfg := Default()
	cfg.NoResearch, cfg.Genome = true, gn
	b = New(g.Empires[0], cfg)
	for v, st := range g.Cluster.Stars {
		b.E.Visit(v)
		for _, p := range st.System.Planets {
			for _, sq := range p.Surface {
				sq.Ruins = false
			}
		}
	}
	b.setFloor()
	return b, b.E.Colonies[0]
}

// layRuins puts ruins on the first empty square of p that reach says
// (Planet.Reachable or its negation), the square.
func layRuins(t *testing.T, p *game.Planet, reach bool) game.Cell {
	t.Helper()
	for _, x := range p.Cells() {
		if sq := p.Surface[x]; sq.Item == game.NoItem && p.Reachable(x) == reach {
			sq.Ruins = true
			return x
		}
	}
	t.Fatalf("%s: no empty square reachable %v", p.Name, reach)
	return game.Cell{}
}

func neutralRuins(w float64) *Genome {
	g := Neutral()
	g.RuinWorth = w
	return g
}

// planColony starts the dig on its colony's ruins before anything else
// it would start, once research allows the dig; not without the tech, not
// on unreachable ruins (the colony then starts something else).
func TestPlanColonyDigsFirst(t *testing.T) {
	dig := game.XenoArcheologicalDig
	for _, tc := range []struct {
		name          string
		research, far bool
		want          bool
	}{
		{"researched", true, false, true},
		{"unresearched", false, false, false},
		{"unreachable", true, true, false},
	} {
		b, home := ruinsFixture(t, nil)
		col := secondColony(t, b, home)
		at := layRuins(t, col.Planet, !tc.far)
		if tc.research {
			b.E.Research.Grant(game.Tech(dig.Info().ResearchReq))
		}
		if !tc.far && b.E.Research.Allows(dig) != (col.CanStart(b.E.Research, dig, at) == nil) {
			t.Fatalf("%s: CanStart %v", tc.name, col.CanStart(b.E.Research, dig, at))
		}
		b.planColony(1, col, b.claimed())
		pj := col.Project
		switch {
		case pj == nil:
			t.Errorf("%s: nothing started (the dig must preempt a choice, not fill a void)", tc.name)
		case tc.want && (pj.Item != dig || pj.Cell != at):
			t.Errorf("%s: started %s at %v, want the dig at %v", tc.name, pj.Item, pj.Cell, at)
		case !tc.want && pj.Item == dig:
			t.Errorf("%s: started the dig", tc.name)
		}
	}
}

// foundableAt lists the unclaimed foundable planets of star, nearest to
// pos first, under the floor the bot's genome sets now (setFloor, as
// claimed does: a fresh bot's zero floor would list the small ones too).
func foundableAt(b *Bot, star int, pos game.Vec3) []int {
	b.setFloor()
	var out []int
	ps := b.E.Cluster.Stars[star].System.Planets
	for pl := range ps {
		if b.foundable(star, pl) {
			out = append(out, pl)
		}
	}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if dist(pos, ps[out[j]].Pos) < dist(pos, ps[out[i]].Pos) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// In its system a colony ship flies to the planet with ruins rather than
// a nearer one without; at RuinWorth 0 to the nearest.
func TestPlanShipRuinsInSystem(t *testing.T) {
	for _, gn := range []*Genome{nil, neutralRuins(0)} {
		b, home := ruinsFixture(t, gn)
		s := b.E.Spawn(game.FixtureColonyShip, home.Star, home.PlanetIdx)
		pls := foundableAt(b, home.Star, s.Pos)
		if len(pls) < 2 {
			t.Fatalf("home star: %d foundable planets", len(pls))
		}
		far := pls[len(pls)-1]
		layRuins(t, b.E.Planet(home.Star, far), true)
		b.planShip(s, b.claimed())
		want := far
		if gn != nil {
			want = pls[0]
		}
		if s.Order.Kind != game.GoToPlanet || s.Order.Planet != want {
			t.Errorf("RuinWorth %v: order %+v, want GoToPlanet %d (ruins on %d)", b.ruinW(), s.Order, want, far)
		}
	}
}

// routeStars: from the home (its planets claimed), the days to every
// star (oracle) and the target stars by (days, hops, index).
func routeStars(b *Bot, claimed map[place]bool) (days []int, targets []int) {
	c, home := b.E.Cluster, b.E.Colonies[0].Star
	days, hops := oracle(b.E, c, home, 1)
	left := map[int]bool{}
	for v := range c.Stars {
		if v != home && days[v] >= 0 && b.targets(v, claimed) > 0 {
			left[v] = true
		}
	}
	for len(left) > 0 {
		v := nearestWhere(days, hops, home, func(u int) bool { return left[u] })
		targets = append(targets, v)
		delete(left, v)
	}
	return days, targets
}

// route takes a star with ruins over the nearest target star when it is
// fewer than RuinWorth lane days farther, not when it is more (a much
// nearer world wins), not at RuinWorth 0, not when the empire does not
// Know the ruins' star (the fog). RuinWorth is ruinWorth's value here.
func TestRouteRuinWorth(t *testing.T) {
	// FoundFloor 1, every size: under foundFloor, seed 1 has no target
	// star within ruinWorth days past the nearest.
	floor1 := func(w float64) *Genome {
		g := neutralRuins(w)
		g.FoundFloor = 1
		return g
	}
	b, home := ruinsFixture(t, floor1(ruinWorth))
	claimed := map[place]bool{}
	claimAll(b.E.Cluster, home.Star, claimed)
	days, targets := routeStars(b, claimed)
	if len(targets) < 2 {
		t.Fatalf("%d target stars", len(targets))
	}
	a := targets[0]
	near, far := -1, -1 // ruins stars within, and beyond, ruinWorth days past a
	for _, v := range targets[1:] {
		switch d := days[v] - days[a]; {
		case d > 0 && d < ruinWorth && near < 0:
			near = v
		case d > ruinWorth && far < 0:
			far = v
		}
	}
	if near < 0 || far < 0 {
		t.Fatalf("no star 1..%d days past %d (%d days) or beyond: near %d far %d", ruinWorth-1, a, days[a], near, far)
	}
	pl := foundableAt(b, near, game.Vec3{})[0]
	at := layRuins(t, b.E.Planet(near, pl), true)
	if to, _ := checkRoute(t, b, home.Star, claimed); to != near {
		t.Errorf("ruins %d days past %d: route to %d, want %d", days[near]-days[a], a, to, near)
	}
	b.Cfg.Genome = floor1(0)
	if to, _ := checkRoute(t, b, home.Star, claimed); to != a {
		t.Errorf("RuinWorth 0: route to %d, want %d", to, a)
	}
	b.Cfg.Genome = floor1(ruinWorth)
	b.E.Visited[near] = false
	if b.ruins(near, pl) || b.ruinsTarget(near, claimed) {
		t.Error("ruins seen through the fog")
	}
	if to, _ := checkRoute(t, b, home.Star, claimed); to != a {
		t.Errorf("ruins star unknown: route to %d, want %d", to, a)
	}
	b.E.Visit(near)
	b.E.Planet(near, pl).Surface[at].Ruins = false
	layRuins(t, b.E.Planet(far, foundableAt(b, far, game.Vec3{})[0]), true)
	if to, _ := checkRoute(t, b, home.Star, claimed); to != a {
		t.Errorf("ruins %d days past %d: route to %d, want %d", days[far]-days[a], a, to, a)
	}
}

// A lane ship with a planet of its own system left (no ruins) leaves for
// a ruins star of fewer lane days than RuinWorth; at RuinWorth equal to
// the days, it stays (ties: the system's planet).
func TestPlanShipLeavesForRuins(t *testing.T) {
	b, home := ruinsFixture(t, nil)
	claimed := map[place]bool{}
	pls := foundableAt(b, home.Star, game.Vec3{})
	if len(pls) == 0 {
		t.Fatal("home star: no foundable planet")
	}
	claimAll(b.E.Cluster, home.Star, claimed)
	days, targets := routeStars(b, claimed)
	r := targets[len(targets)-1]
	layRuins(t, b.E.Planet(r, foundableAt(b, r, game.Vec3{})[0]), true)
	// A star nearer than r unknown, off r's paths: an unexplored star
	// never draws the ship from its system's planet.
	near := -1
	for v := range days {
		if v == home.Star || v == r || days[v] <= 0 || days[v] >= days[r] {
			continue
		}
		b.E.Visited[v] = false
		if d, _ := oracle(b.E, b.E.Cluster, home.Star, 1); d[r] == days[r] && b.unexplored(v, claimed) {
			near = v
			break
		}
		b.E.Visit(v)
	}
	if near < 0 {
		t.Fatalf("no star nearer than %d (%d days) off its paths", r, days[r])
	}
	for _, w := range []float64{float64(days[r]) + 1, float64(days[r])} {
		b.Cfg.Genome = neutralRuins(w)
		s := b.E.Spawn(scout, home.Star, home.PlanetIdx)
		cl := b.claimed()
		for _, pl := range pls[1:] {
			cl[place{home.Star, pl}] = true // one planet of the home system left
		}
		b.planShip(s, cl)
		switch {
		case w > float64(days[r]) && (s.Order.Kind != game.GoToStar || b.dest[s] != r):
			t.Errorf("RuinWorth %v, ruins %d days out: order %+v dest %d, want toward %d", w, days[r], s.Order, b.dest[s], r)
		case w <= float64(days[r]) && (s.Order.Kind != game.GoToPlanet || s.Order.Planet != pls[0]):
			t.Errorf("RuinWorth %v, ruins %d days out: order %+v, want GoToPlanet %d", w, days[r], s.Order, pls[0])
		}
		b.E.Ships = b.E.Ships[:len(b.E.Ships)-1]
	}
}

// RuinWorth: Neutral's is ruinWorth, a nil genome plays it too; Clamp
// keeps it in [0, 63], NaN at 0.
func TestRuinWorthGene(t *testing.T) {
	if Neutral().RuinWorth != ruinWorth || (&Bot{}).ruinW() != ruinWorth {
		t.Errorf("neutral %v, nil genome %v, want %v", Neutral().RuinWorth, (&Bot{}).ruinW(), float64(ruinWorth))
	}
	for _, c := range [][2]float64{{-1, 0}, {12, 12}, {99, 63}, {math.NaN(), 0}} {
		g := Neutral()
		g.RuinWorth = c[0]
		if g.Clamp(); g.RuinWorth != c[1] {
			t.Errorf("Clamp(%v) = %v, want %v", c[0], g.RuinWorth, c[1])
		}
	}
}
