package game

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/journal"
)

// Mid-day rounds (docs/rules.md "Combat rounds"): StepRound, Engaged,
// RoundsToday, UsableEnergy.

// stepDuel is roundDuel(twoGens, hulk) with sh ordered off its spot
// (a hop a round) and a journal.
func stepDuel(t *testing.T) (g *Game, sh, tg *Starship, buf *bytes.Buffer) {
	t.Helper()
	g, sh, tg = roundDuel(twoGens, hulk)
	sh.Order = Order{Kind: GoToPoint, Pos: Vec3{X: SystemReach}}
	buf = &bytes.Buffer{}
	g.SetJournal(journal.New(buf))
	return g, sh, tg, buf
}

// A step where the player is not engaged plays nothing and records
// nothing: no move, no energy spent, no round counted.
func TestStepRoundRefused(t *testing.T) {
	g, sh, tg, buf := stepDuel(t)
	tg.Lane = g.Cluster.Stars[tg.Star].Lanes[0] // the foe gone: no combat star
	pos, pw := sh.Pos, sh.Power
	_, _, _, err := g.StepRound(g.Empires[0], sh.Star, nil)
	if err == nil || !strings.Contains(err.Error(), "not engaged at "+g.Cluster.Stars[sh.Star].Name) {
		t.Fatalf("err %v", err)
	}
	if sh.Pos != pos || sh.Power != pw || g.RoundsToday != nil || g.RoundsAt(sh.Star) != 0 {
		t.Errorf("refused step changed the game: moved %v, energy %d of %d, rounds %v", sh.Pos != pos, sh.Power, pw, g.RoundsToday)
	}
	none(t, buf, "refused step")
}

// An accepted step is one Round: the ordered ship hops once, paying its
// MoveCost, the star's count is 1, a journal line "round" names the star.
func TestStepRoundAccepted(t *testing.T) {
	g, sh, _, buf := stepDuel(t)
	pos, pw := sh.Pos, sh.Power
	acted, _, log, err := g.StepRound(g.Empires[0], sh.Star, nil)
	if err != nil || !acted {
		t.Fatalf("acted %v, err %v", acted, err)
	}
	if len(log.Moves) != 1 || log.Moves[0].Ship != sh || log.Moves[0].From != pos || log.Moves[0].To != sh.Pos || sh.Pos == pos {
		t.Errorf("moves %+v, from %v to %v", log.Moves, pos, sh.Pos)
	}
	if want := pw - sh.Design.MoveCost(); sh.Power != want {
		t.Errorf("energy %d, want %d", sh.Power, want)
	}
	if n := g.RoundsAt(sh.Star); n != 1 {
		t.Errorf("RoundsAt %d, want 1", n)
	}
	if en := one(t, buf, "step", "round", 0, nil); en.Star != sh.Star {
		t.Errorf("journal star %d, want %d", en.Star, sh.Star)
	}
}

// RoundsAt is 0 out of range and before any round; each Round counts.
func TestRoundsAt(t *testing.T) {
	g, sh, _, _ := stepDuel(t)
	for _, st := range []int{-1, sh.Star, len(g.Cluster.Stars)} {
		if n := g.RoundsAt(st); n != 0 {
			t.Errorf("RoundsAt(%d) %d before any round", st, n)
		}
	}
	g.Round(sh.Star, nil)
	g.Round(sh.Star, nil)
	if n := g.RoundsAt(sh.Star); n != 2 || len(g.RoundsToday) != len(g.Cluster.Stars) {
		t.Errorf("RoundsAt %d, RoundsToday %d long", n, len(g.RoundsToday))
	}
	if n := g.RoundsAt(len(g.Cluster.Stars)); n != 0 {
		t.Errorf("RoundsAt past the end %d", n)
	}
}

// The close after a mid-day step refills the stepped star's ships
// without Stepping them, and sets RoundsToday to nil.
func TestStepRoundClose(t *testing.T) {
	g, sh, _, _ := stepDuel(t)
	if _, _, _, err := g.StepRound(g.Empires[0], sh.Star, nil); err != nil {
		t.Fatal(err)
	}
	pos := sh.Pos
	g.EndDay()
	if sh.Pos != pos || sh.Power != sh.Design.PowerPerDay() {
		t.Errorf("stepped star at the close: moved %v, energy %d of %d", sh.Pos != pos, sh.Power, sh.Design.PowerPerDay())
	}
	if g.RoundsToday != nil {
		t.Errorf("RoundsToday %v after EndDay", g.RoundsToday)
	}
}

// A fight ended mid-day (the foe gone after a step) holds nothing at the
// close: the star is no CombatStar there, its ships Step
// (not refilled in place).
func TestStepRoundFightEnded(t *testing.T) {
	g, sh, tg, _ := stepDuel(t)
	if _, _, _, err := g.StepRound(g.Empires[0], sh.Star, nil); err != nil {
		t.Fatal(err)
	}
	tg.Lane = g.Cluster.Stars[tg.Star].Lanes[0] // the foe left
	pos := sh.Pos
	g.EndDay()
	if sh.Pos == pos {
		t.Error("the fight ended mid-day: not Stepped at the close")
	}
}

// A war a bot declares at the close's start (after BeginClose, ships
// already face to face) is a fight opened at the close: an Engaged line;
// one declared before it, during the day, is not.
func TestOpenedAtCloseStart(t *testing.T) {
	for _, c := range []struct {
		name    string
		atClose bool
	}{{"declared at the close", true}, {"declared during the day", false}} {
		g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
		home := g.Empires[0].Colonies[0]
		for _, e := range g.Empires {
			s := e.Spawn(hulk, home.Star, home.PlanetIdx)
			s.Orbit = -1
		}
		g.Empires[1].Visit(home.Star)
		if !c.atClose {
			atWar(t, g, 1, 0)
		}
		g.BeginClose()
		if c.atClose {
			atWar(t, g, 1, 0)
		}
		n := 0
		for _, x := range g.EndDay()[0] {
			if x.Kind == Engaged {
				n++
			}
		}
		if want := map[bool]int{true: 1, false: 0}[c.atClose]; n != want {
			t.Errorf("%s: %d Engaged lines, want %d", c.name, n, want)
		}
	}
}

// Engaged: e knows the star, it is a combat star, e sees a hostile
// there, and e has a colony there or a ship out of the lanes. Empires 0
// and 1 fight at 0's home; empire 2 (Mebes) comes as a third party.
func TestEngaged(t *testing.T) {
	type fix struct {
		g          *Game
		sh, tg, th *Starship
		star       int
	}
	mk := func() fix {
		g := NewGame(1, []Species{Snovemdomas, Orfa, Mebes}, Average)
		atWar(t, g, 0, 1)
		home := g.Empires[0].Colonies[0]
		f := fix{g: g, star: home.Star}
		f.sh = g.Empires[0].Spawn(twoGens, home.Star, home.PlanetIdx)
		f.tg = g.Empires[1].Spawn(hulk, home.Star, home.PlanetIdx)
		f.th = g.Empires[2].Spawn(hulk, home.Star, home.PlanetIdx)
		f.sh.Orbit, f.tg.Orbit, f.th.Orbit = -1, -1, -1
		g.Empires[2].Visit(home.Star)
		return f
	}
	lane := func(f fix) int { return f.g.Cluster.Stars[f.star].Lanes[0] }
	for _, c := range []struct {
		name  string
		e     int
		setup func(t *testing.T, f fix)
		want  bool
	}{
		{"both sides", 0, nil, true},
		{"the foe", 1, nil, true},
		{"a fight between others", 2, nil, false},
		{"third party at war", 2, func(t *testing.T, f fix) { atWar(t, f.g, 2, 1) }, true},
		{"third party, star unknown", 2, func(t *testing.T, f fix) {
			atWar(t, f.g, 2, 1)
			f.g.Empires[2].Visited[f.star] = false
		}, false},
		{"third party in a lane", 2, func(t *testing.T, f fix) {
			atWar(t, f.g, 2, 1)
			f.th.Lane = lane(f)
		}, false},
		{"colony alone, no defence: no fight", 0, func(t *testing.T, f fix) { f.sh.Lane = lane(f) }, false},
		{"colony, ship in a lane", 0, func(t *testing.T, f fix) {
			atWar(t, f.g, 2, 1) // the fight goes on without sh
			f.sh.Lane = lane(f)
		}, true},
		{"no combat star", 0, func(t *testing.T, f fix) { f.tg.Lane = lane(f) }, false},
		// 1's armed ship at 0's shielded home, no ship of 0's there: the
		// planet alone is the fight, unless cloaked.
		{"a shielded colony alone", 1, func(t *testing.T, f fix) { shieldedAlone(f.g, f.sh, f.tg, lane(f)) }, true},
		{"a shielded colony alone, cloaked", 1, func(t *testing.T, f fix) {
			shieldedAlone(f.g, f.sh, f.tg, lane(f))
			f.g.Empires[0].Colonies[0].Planet.Orbit[1] = OrbitalCloaker
		}, false},
	} {
		f := mk()
		if c.setup != nil {
			c.setup(t, f)
		}
		if got := f.g.Engaged(f.g.Empires[c.e], f.star); got != c.want {
			t.Errorf("%s: Engaged(%d) %v, want %v", c.name, c.e, got, c.want)
		}
	}
}

// shieldedAlone leaves 0's home with an Orbital Shield, sh (0's) in a
// lane and tg (1's) armed.
func shieldedAlone(g *Game, sh, tg *Starship, lane int) {
	sh.Lane = lane
	tg.Design = twoGens
	g.Empires[0].Colonies[0].Planet.Orbit[0] = OrbitalShield
}

// opened: a fight new since before is an Engaged Report for each side
// (Other the foe, the foes named), none for an empire watching others
// fight; EndDay, the fight already open at its start, reports none.
func TestOpened(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa, Mebes}, Average)
	atWar(t, g, 0, 1)
	home := g.Empires[0].Colonies[0]
	for _, e := range g.Empires {
		s := e.Spawn(hulk, home.Star, home.PlanetIdx)
		s.Orbit = -1
	}
	g.Empires[2].Visit(home.Star)
	name := g.Cluster.Stars[home.Star].Name
	reps := g.opened(nil)
	want := []Report{
		{0, Event{Kind: Engaged, Item: NoItem, Tech: NoTech, Star: home.Star, Planet: -1, Other: 1, Text: "Combat at " + name + ": Orfa ships"}},
		{1, Event{Kind: Engaged, Item: NoItem, Tech: NoTech, Star: home.Star, Planet: -1, Other: 0, Text: "Combat at " + name + ": Snovemdomas ships"}},
	}
	if !slices.Equal(reps, want) {
		t.Errorf("opened %+v, want %+v", reps, want)
	}
	atWar(t, g, 0, 2)
	if reps := g.opened(map[[2]int]bool{{1, home.Star}: true}); len(reps) != 2 || reps[0].Empire != 0 ||
		reps[0].Event.Text != "Combat at "+name+": Orfa, Mebes ships" || reps[1].Empire != 2 {
		t.Errorf("two foes, empire 1's fight open: %+v", reps)
	}
	for i, evs := range g.EndDay() {
		for _, x := range evs {
			if x.Kind == Engaged {
				t.Errorf("empire %d: %q, the fight open at the start", i, x.Text)
			}
		}
	}
}

// UsableEnergy: enough for a hop (MoveCost, with a drive) or a shot of
// one weapon; neither; a drive-less ship's 0 MoveCost is no hop.
func TestUsableEnergy(t *testing.T) {
	unarmed := Design{Name: "Unarmed", Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor}}
	gunOnly := Design{Name: "Turret", Hull: Small, Parts: []Gizmo{Nanotwirler, Ueberlaser}}
	both := Design{Name: "Both", Hull: Small, Parts: []Gizmo{Nanotwirler, TonklinMotor, Ueberlaser}}
	mv, gun := both.MoveCost(), Gizmos[Ueberlaser].Power
	if mv != unarmed.MoveCost() || mv < 1 || gun <= mv+1 {
		t.Fatalf("degenerate fixture: MoveCost %d, gun %d", mv, gun)
	}
	if Gizmos[Ueberlaser].Uses < 1 {
		t.Fatal("degenerate fixture: Ueberlaser has no daily limit")
	}
	for _, c := range []struct {
		name  string
		d     Design
		power int
		want  bool
		spent bool // gunOnly's Ueberlaser out of shots today
		scram bool
	}{
		{"a hop's worth", unarmed, mv, true, false, false},
		{"short of a hop", unarmed, mv - 1, false, false, false},
		{"a shot's worth", gunOnly, gun, true, false, false},
		{"short of a shot", gunOnly, gun - 1, false, false, false},
		{"a hop, not a shot", both, mv, true, false, false},
		{"neither", both, mv - 1, false, false, false},
		{"no drive, no gun", hulk, 100, false, false, false},
		{"a shot's worth, its uses spent", gunOnly, gun, false, true, false},
		{"a shot's worth, Scrambled", gunOnly, gun, false, false, true},
		{"a hop, Scrambled", both, mv, true, false, true},
	} {
		s := &Starship{Design: c.d, Power: c.power, Scrambled: c.scram}
		if c.spent {
			s.Shots = make([]int, len(c.d.Parts))
			s.Shots[part(c.d, Ueberlaser)] = Gizmos[Ueberlaser].Uses
		}
		if got := s.UsableEnergy(); got != c.want {
			t.Errorf("%s: UsableEnergy at %d %v, want %v", c.name, c.power, got, c.want)
		}
	}
}
