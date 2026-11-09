package bot

import (
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// Mid-day rounds (docs/rules.md "Combat rounds"): bot.Step and the
// close's caps counting them.

// stepFixture is a war at the player's home (empire 0, no bot): an
// unarmed ship of the player in space, no order, and a bot warship far
// out with energy for more than RoundCap hops, heading in (dayRounds'
// mover): it hops every round it plays.
func stepFixture(t *testing.T, gn *Genome) (g *game.Game, bots []*Bot, star int, mover *game.Starship) {
	t.Helper()
	g = game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	atWar(t, g, 0, 1)
	bots = NewAll(g, Config{NoResearch: true, Genome: gn})
	bots[0] = nil
	home := g.Empires[0].Colonies[0]
	me := g.Empires[0].Spawn(unarmedScout, home.Star, home.PlanetIdx)
	me.Orbit = -1
	big := game.Design{Name: "Big", Hull: game.Enormous, Parts: []game.Gizmo{game.TonklinMotor, game.MassBarrageGun}}
	for len(big.Parts) < game.Enormous.Slots() {
		big.Parts = append(big.Parts, game.Nanotwirler)
	}
	mover = g.Empires[1].Spawn(big, home.Star, home.PlanetIdx)
	mover.Pos = game.Vec3{X: -100}
	mover.Order = game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: game.SystemReach}}
	if mover.Power < (RoundCap+1)*big.MoveCost() {
		t.Fatalf("fixture: energy %d", mover.Power)
	}
	if !g.Engaged(g.Empires[0], home.Star) {
		t.Fatal("fixture: the player is not engaged")
	}
	return g, bots, home.Star, mover
}

// steps plays k mid-day Steps at star, each accepted.
func steps(t *testing.T, g *game.Game, bots []*Bot, star, k int) {
	t.Helper()
	for i := range k {
		if _, _, _, err := Step(g, bots, star); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
}

// closeRounds is the rounds the close (Begin, Finish) plays at star.
func closeRounds(g *game.Game, bots []*Bot, star int) int {
	_, _, logs := Begin(g, bots).Finish()
	n := 0
	for _, l := range logs {
		if l.Star == star {
			n++
		}
	}
	return n
}

// Step is refused once the day's rounds at the star reach RoundCap:
// nothing plays, nothing is counted.
func TestStepRefusedAtCap(t *testing.T) {
	g, bots, star, mover := stepFixture(t, nil)
	g.RoundsToday = make([]int, len(g.Cluster.Stars))
	g.RoundsToday[star] = RoundCap
	pos := mover.Pos
	_, _, _, err := Step(g, bots, star)
	if err == nil || !strings.Contains(err.Error(), "no round left today at "+g.Cluster.Stars[star].Name) {
		t.Fatalf("err %v", err)
	}
	if g.RoundsAt(star) != RoundCap || mover.Pos != pos {
		t.Errorf("refused: rounds %d, mover moved %v", g.RoundsAt(star), mover.Pos != pos)
	}
}

// Step refused where the player is not engaged (the star unknown to
// it): no bot plays its Round (no new order, nothing armed), no round.
func TestStepNotEngaged(t *testing.T) {
	g, bots, star, mover := stepFixture(t, nil)
	g.Empires[0].Visited[star] = false
	if !slices.ContainsFunc(g.Hostiles(g.Empires[1], star), func(sg game.Sighting) bool { return sg.Empire == 0 }) {
		t.Fatal("fixture: the bot does not see the player's ship")
	}
	pos, o := mover.Pos, mover.Order
	if _, _, _, err := Step(g, bots, star); err == nil || !strings.Contains(err.Error(), "not engaged") {
		t.Fatalf("err %v", err)
	}
	if mover.Pos != pos || mover.Order != o || len(mover.Armed) != 0 || g.RoundsAt(star) != 0 {
		t.Errorf("refused: mover moved %v, order %+v, armed %d, rounds %d", mover.Pos != pos, mover.Order, len(mover.Armed), g.RoundsAt(star))
	}
	// The control: engaged, the bot's Round re-orders the mover.
	g.Empires[0].Visited[star] = true
	bots[1].Round(star)
	if mover.Order == o && len(mover.Armed) == 0 {
		t.Error("fixture: the bot's Round changes nothing here")
	}
}

// A bot whose Rounds cap the day's rounds at the star reached passes a
// Step (no Round of its own, no move); below it, it plays.
func TestStepBotAtCap(t *testing.T) {
	gn := genomeWith(func(g *Genome) { g.Rounds = 3 })
	for _, tc := range []struct {
		before int
		plays  bool
	}{{2, true}, {3, false}} {
		g, bots, star, mover := stepFixture(t, gn)
		if c := bots[1].rounds(); c != 3 {
			t.Fatalf("fixture: cap %d", c)
		}
		g.RoundsToday = make([]int, len(g.Cluster.Stars))
		g.RoundsToday[star] = tc.before
		pos := mover.Pos
		_, _, log, err := Step(g, bots, star)
		if err != nil {
			t.Fatal(err)
		}
		if moved := mover.Pos != pos; moved != tc.plays || (len(log.Moves) > 0) != tc.plays {
			t.Errorf("%d rounds before: mover moved %v, moves %d, want plays %v", tc.before, moved, len(log.Moves), tc.plays)
		}
		if n := g.RoundsAt(star); n != tc.before+1 {
			t.Errorf("%d rounds before: RoundsAt %d", tc.before, n)
		}
	}
}

// After k mid-day Steps the close plays what is left of RoundCap at the
// star (the mover hops every round: without Steps it plays all of it).
func TestStepThenClose(t *testing.T) {
	g, bots, star, _ := stepFixture(t, nil)
	if n := closeRounds(g, bots, star); n != RoundCap {
		t.Fatalf("fixture: the close alone plays %d rounds, want RoundCap %d", n, RoundCap)
	}
	for _, k := range []int{1, RoundCap - 2} {
		g, bots, star, _ := stepFixture(t, nil)
		steps(t, g, bots, star, k)
		if n := closeRounds(g, bots, star); n != RoundCap-k {
			t.Errorf("%d steps: the close played %d rounds, want %d", k, n, RoundCap-k)
		}
	}
}

// A star stepped to RoundCap mid-day plays no round at the close, its
// ships refilled, not Stepped.
func TestStepCapCloseSkips(t *testing.T) {
	g, bots, star, mover := stepFixture(t, nil)
	steps(t, g, bots, star, RoundCap)
	pos := mover.Pos
	if n := closeRounds(g, bots, star); n != 0 {
		t.Errorf("the close played %d rounds at a star at RoundCap", n)
	}
	want := mover.Design.PowerPerDay()
	if mover.Shields {
		want -= mover.Design.ShieldDraw()
	}
	if mover.Pos != pos || mover.Power != want {
		t.Errorf("mover at the close: moved %v, energy %d of %d", mover.Pos != pos, mover.Power, want)
	}
}

// A bot's Rounds cap counts the mid-day Steps: cap 3 after 2 Steps, the
// close's mover hops once (one round left to its cap).
func TestStepBotCapAtClose(t *testing.T) {
	const cap, k = 3, 2
	g, bots, star, mover := stepFixture(t, genomeWith(func(g *Genome) { g.Rounds = cap }))
	steps(t, g, bots, star, k)
	_, _, logs := Begin(g, bots).Finish()
	hops := 0
	for _, l := range logs {
		for _, m := range l.Moves {
			if m.Ship == mover {
				hops++
			}
		}
	}
	if hops != cap-k {
		t.Errorf("the close: %d hops of the bot's mover, want %d", hops, cap-k)
	}
}

// A close stepped by hand (Round(true)) ends the star once the day's
// rounds there reach RoundCap, the mid-day Steps counted, though the
// player still acts.
func TestStepCloseEndsAtCap(t *testing.T) {
	g, bots, star, _ := stepFixture(t, nil)
	steps(t, g, bots, star, RoundCap-1)
	me := g.Empires[0].Ships[0]
	me.Order = game.Order{Kind: game.GoToPoint, Pos: game.Vec3{Z: game.SystemReach}}
	d := Begin(g, bots)
	if st, _, ok := d.Star(); !ok || st != star {
		t.Fatalf("Star() %d %v, want %d", st, ok, star)
	}
	if acted, _, _ := d.Round(true); !acted {
		t.Fatal("fixture: the last round did nothing")
	}
	if st, _, ok := d.Star(); ok && st == star {
		t.Errorf("the star goes on past RoundCap (%d rounds today)", g.RoundsAt(star))
	}
}
