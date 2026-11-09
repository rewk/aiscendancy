package bot

import (
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// abilityGame is a seed-1 game of sp then others, all met, one bot each
// (gn, no research), built after the contacts; the bot is sp's, its
// ability charged.
func abilityGame(t *testing.T, gn *Genome, sp game.Species, others ...game.Species) (*game.Game, *Bot) {
	t.Helper()
	g := game.NewGame(3, append([]game.Species{sp}, others...), game.Average)
	for j := 1; j < len(g.Empires); j++ {
		g.Contact(0, j)
	}
	if g.Empires[0].Species.Info().Recharge == 0 {
		t.Fatalf("fixture: the %v's ability is passive", sp)
	}
	charge(g, 0)
	return g, NewAll(g, Config{NoResearch: true, Genome: gn})[0]
}

// charge makes empire i's ability ready today: last used Recharge days
// ago (a timed ability starts empty).
func charge(g *game.Game, i int) {
	g.Empires[i].AbilityUsed = g.Day - g.Empires[i].Species.Info().Recharge
}

// acted runs b's Act and reports b's ability used today: AbilityUsed
// moved to Day, with an AbilityUsed Report of b's own in its News (both
// or neither, else an error).
func acted(t *testing.T, g *game.Game, b *Bot) bool {
	t.Helper()
	before := b.E.AbilityUsed
	b.Act()
	used := b.E.AbilityUsed != before && b.E.AbilityUsed == g.Day
	news := slices.ContainsFunc(b.News(), func(r game.Report) bool {
		return r.Empire == b.E.Index && r.Event.Kind == game.AbilityUsed && r.Event.Other < 0
	})
	if used != news {
		t.Errorf("%v: AbilityUsed %d on day %d, news %v", b.E.Species, b.E.AbilityUsed, g.Day, news)
	}
	return used
}

// recharged checks the day after a use: the ability is recharging, so
// Act does not use it again (AbilityUsed stays).
func recharged(t *testing.T, g *game.Game, b *Bot) {
	t.Helper()
	day := g.Day
	g.Day++
	if acted(t, g, b) || b.E.AbilityUsed != day {
		t.Errorf("%v: used again the next day (AbilityUsed %d)", b.E.Species, b.E.AbilityUsed)
	}
}

// home is e's first colony.
func homeOf(e *game.Empire) *game.Colony { return e.Colonies[0] }

// intruder spawns a FixtureRaider of e in space at col's planet.
func intruder(e *game.Empire, col *game.Colony) *game.Starship {
	s := e.Spawn(game.FixtureRaider, col.Star, col.PlanetIdx)
	s.Orbit = -1
	return s
}

// The Baliflids force peace on an empire at War they would make peace
// with (Start over AcceptPeace: AbilityUsed set, Peace both ways), not
// with every threshold out of reach (Temper +1000); the Marmosians
// defame at war, not at peace; a passive species (Orfa) never uses.
func TestAbilities(t *testing.T) {
	if p := Personalities[game.Baliflids]; p.Start < p.AcceptPeace {
		t.Fatalf("fixture: Baliflids Start %d under AcceptPeace %d", p.Start, p.AcceptPeace)
	}
	for _, tc := range []struct {
		sp     game.Species
		temper float64
		war    bool
		used   bool
	}{
		{game.Baliflids, 0, true, true},
		{game.Baliflids, 1000, true, false},
		{game.Marmosians, 0, true, true},
		{game.Marmosians, 0, false, false},
		{game.Orfa, 0, true, false},
	} {
		g := game.NewGame(1, []game.Species{tc.sp, game.Snovemdomas}, game.Average)
		g.Contact(0, 1)
		charge(g, 0)
		charged := g.Empires[0].AbilityUsed
		if tc.war {
			atWar(t, g, 0, 1)
		}
		bots := NewAll(g, Config{NoResearch: true, Genome: genomeWith(func(g *Genome) { g.Temper = tc.temper })})
		if used := acted(t, g, bots[0]); used != tc.used || (!tc.used && g.Empires[0].AbilityUsed != charged) {
			t.Errorf("%v temper %v war %v: AbilityUsed %d", tc.sp, tc.temper, tc.war, g.Empires[0].AbilityUsed)
		}
		e := g.Empires[0]
		if tc.sp == game.Baliflids && tc.used && (e.StanceWith(1) != game.Peace || g.Empires[1].StanceWith(0) != game.Peace) {
			t.Errorf("forced peace: %v", e.StanceWith(1))
		}
	}
}

// The Fludentri repair when a ship is below RepairAt × its MaxHull (every
// ship back at MaxHull), not with every ship full, nor below a RepairAt
// of 0.
func TestAbilityFludentri(t *testing.T) {
	for _, tc := range []struct {
		repairAt float64
		damage   bool
		used     bool
	}{{0.5, true, true}, {0.5, false, false}, {0, true, false}} {
		g, b := abilityGame(t, genomeWith(func(g *Genome) { g.RepairAt = tc.repairAt }), game.Fludentri, game.Snovemdomas)
		arm(b.E)
		s := b.E.Ships[0]
		if tc.damage {
			s.Hull = 1
		}
		if acted(t, g, b) != tc.used {
			t.Errorf("RepairAt %v damage %v: used %v", tc.repairAt, tc.damage, !tc.used)
		}
		if tc.used {
			if s.Hull != s.MaxHull() {
				t.Errorf("hull %d, want %d", s.Hull, s.MaxHull())
			}
			s.Hull = 1
			recharged(t, g, b)
		}
	}
}

// The Swaparamans double their power with an armed ship at a combat star
// (its Power doubled), not with the foe absent, nor with only an unarmed
// ship there.
func TestAbilitySwaparamans(t *testing.T) {
	for _, tc := range []struct {
		foe, armed, used bool
	}{{true, true, true}, {false, true, false}, {true, false, false}} {
		g, b := abilityGame(t, nil, game.Swaparamans, game.Snovemdomas)
		atWar(t, g, 0, 1)
		home := homeOf(b.E)
		d := game.FixtureColonyShip
		if tc.armed {
			d = game.FixtureRaider
		}
		s := b.E.Spawn(d, home.Star, home.PlanetIdx)
		s.Orbit = -1
		if tc.foe {
			intruder(g.Empires[1], home)
		}
		power := s.Power
		if acted(t, g, b) != tc.used {
			t.Errorf("foe %v armed %v: used %v", tc.foe, tc.armed, !tc.used)
		}
		if tc.used {
			if s.Power != 2*power {
				t.Errorf("power %d, want %d", s.Power, 2*power)
			}
			recharged(t, g, b)
		}
	}
}

// The Shevar drain a hostile ship at my colony's star (its Power 0) or at
// a star where only a ship of mine stands, not one at peace, nor a
// hostile colony alone.
func TestAbilityShevar(t *testing.T) {
	for _, tc := range []struct {
		name            string
		war, atHome, my bool
		used            bool
	}{
		{"at my colony", true, true, false, true},
		{"by my ship", true, false, true, true},
		{"at peace", false, true, false, false},
		{"their colony alone", true, false, false, false},
	} {
		g, b := abilityGame(t, nil, game.Shevar, game.Snovemdomas)
		e1 := g.Empires[1]
		if tc.war {
			atWar(t, g, 0, 1)
		}
		var foe *game.Starship
		if tc.atHome {
			foe = intruder(e1, homeOf(b.E))
		} else {
			b.E.Visit(homeOf(e1).Star)
		}
		if tc.my {
			intruder(b.E, homeOf(e1))
			foe = intruder(e1, homeOf(e1))
		}
		if acted(t, g, b) != tc.used {
			t.Errorf("%s: used %v", tc.name, !tc.used)
		}
		if tc.used {
			if foe.Power != 0 {
				t.Errorf("%s: foe power %d", tc.name, foe.Power)
			}
			recharged(t, g, b)
		}
	}
}

// The Dubtaks steal whenever ready: a tech two others know is granted;
// with nothing to steal the game refuses (AbilityUsed stays).
func TestAbilityDubtaks(t *testing.T) {
	for _, steal := range []bool{true, false} {
		g, b := abilityGame(t, nil, game.Dubtaks, game.Snovemdomas, game.Orfa)
		tech := game.NoTech
		for k := range game.NumTechs {
			if !b.E.Research.Known[k] && !g.Empires[1].Research.Known[k] && !g.Empires[2].Research.Known[k] {
				tech = k
				break
			}
		}
		if steal {
			g.Empires[1].Research.Grant(tech)
			g.Empires[2].Research.Grant(tech)
		}
		if acted(t, g, b) != steal {
			t.Errorf("steal %v: used %v", steal, !steal)
		}
		if b.E.Research.Known[tech] != steal {
			t.Errorf("steal %v: %v known %v", steal, tech, !steal)
		}
		if steal {
			recharged(t, g, b)
		} else if b.E.AbilityUsed != g.Day-b.E.Species.Info().Recharge {
			t.Errorf("refused: AbilityUsed %d", b.E.AbilityUsed)
		}
	}
}

// The Chamachies finish the topic when its days left at my research rate
// reach FreeResearchDays (its Progress at its Cost), not one day short,
// nor with no topic; at rate 0 (never done) always.
func TestAbilityChamachies(t *testing.T) {
	for _, tc := range []struct {
		lab   bool
		slack int // FreeResearchDays − days left
		topic bool
		used  bool
	}{{true, 0, true, true}, {true, 1, true, false}, {true, 0, false, false}, {false, 399, true, true}} {
		g, b := abilityGame(t, nil, game.Chamachies, game.Snovemdomas)
		r, home := b.E.Research, homeOf(b.E)
		if tc.lab {
			at, ok := home.Planet.BestCell(game.Laboratory, b.E.Species.Traits())
			if !ok || home.Planet.Build(at, game.Laboratory) != nil {
				t.Fatal("fixture: no Laboratory")
			}
		}
		rate := 0
		for _, c := range b.E.Colonies {
			_, res, _ := c.Output()
			rate += res
		}
		if (rate > 0) != tc.lab {
			t.Fatalf("fixture: rate %d with lab %v", rate, tc.lab)
		}
		topic := r.Available()[0]
		if tc.topic {
			if err := r.Study(topic); err != nil {
				t.Fatal(err)
			}
		}
		left := max(r.Days(topic, rate), 1)
		b.Cfg.Genome = genomeWith(func(g *Genome) { g.FreeResearchDays = float64(left + tc.slack) })
		if acted(t, g, b) != tc.used {
			t.Errorf("%+v: used %v (days left %d)", tc, !tc.used, left)
		}
		if got := r.Progress[topic] == r.Cost(topic); got != tc.used {
			t.Errorf("%+v: progress %d of %d", tc, r.Progress[topic], r.Cost(topic))
		}
		if tc.used {
			recharged(t, g, b)
		}
	}
}

// The Nimbuloids boost a running project (its Progress gains the day's
// industry), not an idle colony.
func TestAbilityNimbuloids(t *testing.T) {
	for _, running := range []bool{true, false} {
		g, b := abilityGame(t, nil, game.Nimbuloids, game.Snovemdomas)
		home := homeOf(b.E)
		if running {
			at, ok := home.Planet.BestCell(game.Factory, b.E.Species.Traits())
			if !ok {
				t.Fatal("fixture: no cell")
			}
			if err := home.Start(b.E.Research, game.Factory, at); err != nil {
				t.Fatal(err)
			}
		}
		ind, _, _ := home.Output()
		pj := home.Project
		if acted(t, g, b) != running {
			t.Errorf("running %v: used %v", running, !running)
		}
		if running {
			if pj.Progress != min(ind, pj.Cost()) || ind <= 0 {
				t.Errorf("progress %d, want %d", pj.Progress, ind)
			}
			recharged(t, g, b)
		}
	}
}

// The Mebes grow every colony's capacity whenever ready (PopBonus
// MebesPopBonus).
func TestAbilityMebes(t *testing.T) {
	g, b := abilityGame(t, nil, game.Mebes, game.Snovemdomas)
	if !acted(t, g, b) || homeOf(b.E).Planet.PopBonus != game.MebesPopBonus {
		t.Errorf("PopBonus %d", homeOf(b.E).Planet.PopBonus)
	}
	recharged(t, g, b)
	if homeOf(b.E).Planet.PopBonus != game.MebesPopBonus {
		t.Errorf("PopBonus %d after the recharge day", homeOf(b.E).Planet.PopBonus)
	}
}

// The Capelons and the Frutmaka act on a hostile ship at a star holding
// a colony of mine (Invincible today; the intruder warped away), not on
// one at peace, nor on a hostile at its own home.
func TestAbilityIntruders(t *testing.T) {
	for _, sp := range []game.Species{game.Capelons, game.Frutmaka} {
		for _, tc := range []struct {
			war, atHome, used bool
		}{{true, true, true}, {false, true, false}, {true, false, false}} {
			g, b := abilityGame(t, nil, sp, game.Snovemdomas)
			if tc.war {
				atWar(t, g, 0, 1)
			}
			at := homeOf(g.Empires[1])
			if tc.atHome {
				at = homeOf(b.E)
			}
			foe := intruder(g.Empires[1], at)
			if acted(t, g, b) != tc.used {
				t.Errorf("%v %+v: used %v", sp, tc, !tc.used)
			}
			switch {
			case sp == game.Capelons && b.E.IsInvincible(g.Day) != tc.used:
				t.Errorf("%v %+v: invincible %v", sp, tc, !tc.used)
			case sp == game.Frutmaka && (foe.Star != at.Star) != tc.used:
				t.Errorf("%v %+v: foe at %d", sp, tc, foe.Star)
			}
			if tc.used {
				if sp == game.Frutmaka {
					intruder(g.Empires[1], at)
				}
				recharged(t, g, b)
			}
		}
	}
}

// inLane puts s in cluster lane li, entered at its star.
func inLane(s *game.Starship, li int) {
	s.Orbit, s.Lane, s.Arrive = -1, li, s.Arrive+5
}

// The Ungooma bump the ships in a lane at my colony's star (out of it),
// not while a ship of mine travels a lane too, nor with no traffic, nor
// with traffic only in a lane far from my colonies (the game would bump
// it).
func TestAbilityUngooma(t *testing.T) {
	for _, tc := range []struct {
		traffic, far, mine, used bool
	}{{true, false, false, true}, {true, false, true, false}, {false, false, false, false}, {true, true, false, false}} {
		g, b := abilityGame(t, nil, game.Ungooma, game.Snovemdomas)
		home, c := homeOf(b.E), g.Cluster
		li, at := c.Stars[home.Star].Lanes[0], home
		if tc.far {
			at, li = homeOf(g.Empires[1]), -1
			for _, l := range c.Stars[at.Star].Lanes {
				if c.Other(l, at.Star) != home.Star {
					li = l
					break
				}
			}
			if li < 0 || at.Star == home.Star {
				t.Fatal("fixture: no far lane")
			}
		}
		foe := intruder(g.Empires[1], at)
		if tc.traffic {
			inLane(foe, li)
		}
		if tc.mine {
			arm(b.E)
			inLane(b.E.Ships[0], li)
		}
		if acted(t, g, b) != tc.used {
			t.Errorf("%+v: used %v", tc, !tc.used)
		}
		if tc.used {
			if foe.Lane >= 0 {
				t.Errorf("foe still in lane %d", foe.Lane)
			}
			inLane(foe, li)
			recharged(t, g, b)
		}
	}
}

// The Arbryls block the lanes at war with traffic into a colony star of
// mine (BlockedUntil Day + BlockDays), not at peace, nor with no traffic.
func TestAbilityArbryls(t *testing.T) {
	for _, tc := range []struct {
		war, traffic, used bool
	}{{true, true, true}, {false, true, false}, {true, false, false}} {
		g, b := abilityGame(t, nil, game.Arbryls, game.Snovemdomas)
		if tc.war {
			atWar(t, g, 0, 1)
		}
		home := homeOf(b.E)
		foe := intruder(g.Empires[1], homeOf(g.Empires[1]))
		if tc.traffic {
			foe.Star = home.Star
			inLane(foe, g.Cluster.Stars[home.Star].Lanes[0])
		}
		if acted(t, g, b) != tc.used {
			t.Errorf("%+v: used %v", tc, !tc.used)
		}
		if tc.used {
			if b.E.BlockedUntil != g.Day+game.BlockDays {
				t.Errorf("BlockedUntil %d on day %d", b.E.BlockedUntil, g.Day)
			}
			recharged(t, g, b)
		}
	}
}

// The Govorom wait with the home alone, and with a second colony turn the
// least populated (the young one, under a grown home) into a Cornucopia.
func TestAbilityGovorom(t *testing.T) {
	for _, second := range []bool{false, true} {
		g, b := abilityGame(t, nil, game.Govorom, game.Snovemdomas)
		e, home := b.E, homeOf(b.E)
		home.Pop = 4 // a grown home: the young colony (StartPop) is the least populated
		if second {
			for pl := range g.Cluster.Stars[home.Star].System.Planets {
				if len(e.Colonies) == 1 && b.foundable(home.Star, pl) {
					if err := e.Found(e.Spawn(game.FixtureColonyShip, home.Star, pl)); err != nil {
						t.Fatal(err)
					}
					e.Ships = e.Ships[:len(e.Ships)-1]
				}
			}
			if len(e.Colonies) != 2 || e.Colonies[1].Pop >= home.Pop || e.Colonies[1].Planet.Type == game.Cornucopia {
				t.Fatalf("fixture: %d colonies", len(e.Colonies))
			}
		}
		if acted(t, g, b) != second {
			t.Errorf("second %v: used %v", second, !second)
		}
		if second {
			if e.Colonies[1].Planet.Type != game.Cornucopia {
				t.Errorf("young colony %v", e.Colonies[1].Planet.Type)
			}
			recharged(t, g, b)
		}
	}
}
