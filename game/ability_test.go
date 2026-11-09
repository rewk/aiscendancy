package game

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// The timed abilities (ability.go): one test each, its refusal leaving
// AbilityUsed alone, its effect, its Reports.

// charge makes i's ability ready today: last used Recharge days ago.
func charge(g *Game, i int) {
	g.Empires[i].AbilityUsed = g.Day - g.Empires[i].Species.Info().Recharge
}

// used is UseAbility(i) for a test, charged first: fatal on a refusal,
// AbilityUsed checked at Day.
func used(t *testing.T, g *Game, i int) []Report {
	t.Helper()
	charge(g, i)
	reps, err := g.UseAbility(i)
	if err != nil {
		t.Fatalf("%s: %v", g.Empires[i].Species, err)
	}
	if g.Empires[i].AbilityUsed != g.Day {
		t.Fatalf("AbilityUsed %d, day %d", g.Empires[i].AbilityUsed, g.Day)
	}
	return reps
}

// refused checks UseAbility(i), charged first, refuses with a message
// holding want, AbilityUsed left alone.
func refused(t *testing.T, g *Game, i int, want string) {
	t.Helper()
	charge(g, i)
	before := g.Empires[i].AbilityUsed
	if reps, err := g.UseAbility(i); err == nil || !strings.Contains(err.Error(), want) || reps != nil || g.Empires[i].AbilityUsed != before {
		t.Errorf("%s: refusal %v (want %q), reps %v, used %d", g.Empires[i].Species, err, want, reps, g.Empires[i].AbilityUsed)
	}
}

// abilityReport is the AbilityUsed Report of i with text (Other −1, the
// user's), or of j hit by i (Other i).
func abilityReport(j, other int, text string) Report {
	return Report{j, dipEvent(AbilityUsed, other, text)}
}

// richReport is the Govorom's use on c: empire 0's AbilityUsed placed at
// c's planet, which the news opens.
func richReport(c *Colony) Report {
	r := abilityReport(0, -1, c.Planet.Name+" made rich")
	r.Event.Star, r.Event.Planet = c.Star, c.PlanetIdx
	return r
}

// Every timed species has its function: none is ErrNotModelled.
func TestAbilitiesAllModelled(t *testing.T) {
	for sp := range NumSpecies {
		if timed := sp.Info().Recharge > 0; timed != (abilities[sp] != nil) {
			t.Errorf("%s: recharge %d, modelled %v", sp, sp.Info().Recharge, abilities[sp] != nil)
		}
	}
}

func TestAbilityHealShips(t *testing.T) {
	g := NewGame(1, []Species{Fludentri, Orfa}, Average)
	home := g.Empires[0].Colonies[0]
	a := g.Empires[0].Spawn(laneShip, home.Star, home.PlanetIdx)
	b := g.Empires[0].Spawn(laneShip, home.Star, home.PlanetIdx)
	foe := g.Empires[1].Spawn(laneShip, home.Star, home.PlanetIdx)
	refused(t, g, 0, "nothing to repair")
	a.Hull -= 3
	foe.Hull -= 2
	reps := used(t, g, 0)
	if !slices.Equal(reps, []Report{abilityReport(0, -1, "All ships repaired")}) {
		t.Errorf("reports %+v", reps)
	}
	if a.Hull != a.MaxHull() || b.Hull != b.MaxHull() || foe.Hull != foe.MaxHull()-2 {
		t.Errorf("hulls %d %d %d of %d", a.Hull, b.Hull, foe.Hull, a.MaxHull())
	}
}

func TestAbilityDoublePower(t *testing.T) {
	g := NewGame(1, []Species{Swaparamans, Orfa}, Average)
	refused(t, g, 0, "no ship")
	home := g.Empires[0].Colonies[0]
	s := g.Empires[0].Spawn(laneShip, home.Star, home.PlanetIdx)
	foe := g.Empires[1].Spawn(laneShip, home.Star, home.PlanetIdx)
	p0 := s.Power
	if p0 == 0 {
		t.Fatal("fixture: no power")
	}
	reps := used(t, g, 0)
	if !slices.Equal(reps, []Report{abilityReport(0, -1, "Ship power doubled today")}) {
		t.Errorf("reports %+v", reps)
	}
	if s.Power != 2*p0 || foe.Power != p0 {
		t.Errorf("power %d / foe %d, want %d / %d", s.Power, foe.Power, 2*p0, p0)
	}
	g.EndDay()
	if s.Power != p0 {
		t.Errorf("next day: power %d, want %d", s.Power, p0)
	}
}

// The Shevar drain every alien ship out of the lanes where they are
// Present: at their home (a colony), at the Orfa's home (a Shevar ship
// there); not the Mebes' ship at the Mebes' home (no Shevar there), nor
// one in a lane leaving the Shevar home. A Report per empire hit.
func TestAbilitySapAlienPower(t *testing.T) {
	g := NewGame(1, []Species{Shevar, Orfa, Mebes}, Average)
	e0, e1, e2 := g.Empires[0], g.Empires[1], g.Empires[2]
	h0, h1, h2 := e0.Colonies[0], e1.Colonies[0], e2.Colonies[0]
	refused(t, g, 0, "no alien ship in our systems")
	atHome := e1.Spawn(fightFixture, h0.Star, h0.PlanetIdx) // shields on from Spawn
	atHome.Orbit = -1
	atHome.Armed = []Shot{{Part: 0}}
	e0.Spawn(laneShip, h1.Star, h1.PlanetIdx)
	atTheirs := e1.Spawn(laneShip, h1.Star, h1.PlanetIdx)
	away := e2.Spawn(laneShip, h2.Star, h2.PlanetIdx)
	inLane := e2.Spawn(laneShip, h0.Star, h0.PlanetIdx)
	inLane.Orbit, inLane.Lane, inLane.Arrive = -1, g.Cluster.Stars[h0.Star].Lanes[0], 5
	p0 := away.Power
	reps := used(t, g, 0)
	want := []Report{abilityReport(0, -1, "Alien ships drained"), abilityReport(1, 0, "Ships drained by Shevar")}
	if !slices.Equal(reps, want) {
		t.Errorf("reports %+v", reps)
	}
	if atHome.Power != 0 || atHome.Armed != nil || atTheirs.Power != 0 || away.Power != p0 || inLane.Power != p0 {
		t.Errorf("power %d (armed %v) %d, untouched %d %d, want %d", atHome.Power, atHome.Armed, atTheirs.Power, away.Power, inLane.Power, p0)
	}
	// The reservation went with the energy: dark, and off gives nothing
	// back; the Refill clears the flag.
	if d := atHome.Design.ShieldDraw(); d == 0 || !atHome.Shields || !atHome.ShieldsDown || atHome.Absorb() != 0 {
		t.Fatalf("sapped shields: draw %d, on %v, down %v, absorb %d", d, atHome.Shields, atHome.ShieldsDown, atHome.Absorb())
	}
	if err := atHome.SetShields(false); err != nil || atHome.Power != 0 {
		t.Errorf("off after the sap: %v, power %d, want 0", err, atHome.Power)
	}
	atHome.Refill()
	if atHome.Sapped || atHome.Power != atHome.Design.PowerPerDay() {
		t.Errorf("the Refill after: sapped %v, power %d", atHome.Sapped, atHome.Power)
	}
}

// The Dubtaks get exactly the techs two other empires or more know: not
// one a single empire knows, nor one they know already.
func TestAbilityStealResearch(t *testing.T) {
	g := NewGame(1, []Species{Dubtaks, Orfa, Mebes, Snovemdomas}, Average)
	refused(t, g, 0, "nothing to steal")
	two, three, one, mine := Tech(3), Tech(40), Tech(10), Tech(20)
	for _, k := range []struct {
		t  Tech
		es []int
	}{{two, []int{1, 3}}, {three, []int{1, 2, 3}}, {one, []int{2}}, {mine, []int{0, 1, 2}}} {
		for _, j := range k.es {
			g.Empires[j].Research.Grant(k.t)
		}
	}
	reps := used(t, g, 0)
	stolen := func(t Tech) Report {
		return Report{0, Event{Kind: Discovered, Item: NoItem, Tech: t, Star: -1, Planet: -1, Other: -1, Text: t.String() + " stolen"}}
	}
	want := []Report{abilityReport(0, -1, "Techs stolen"), stolen(two), stolen(three)}
	if !slices.Equal(reps, want) {
		t.Errorf("reports %+v, want %+v", reps, want)
	}
	r := g.Empires[0].Research
	for tc := range NumTechs {
		if wantKnown := tc == two || tc == three || tc == mine; r.Known[tc] != wantKnown {
			t.Errorf("%s known %v", tc, r.Known[tc])
		}
	}
}

// The Chamachies' topic completes at the next close, with no research
// at all.
func TestAbilityFreeResearch(t *testing.T) {
	g := NewGame(1, []Species{Chamachies, Orfa}, Average)
	e := g.Empires[0]
	refused(t, g, 0, "no topic under study")
	if e.ResearchRate() != 0 {
		t.Fatalf("fixture: research %d", e.ResearchRate())
	}
	topic := e.Research.Available()[0]
	if err := e.Research.Study(topic); err != nil {
		t.Fatal(err)
	}
	reps := used(t, g, 0)
	if !slices.Equal(reps, []Report{abilityReport(0, -1, topic.String()+" due tomorrow")}) {
		t.Errorf("reports %+v", reps)
	}
	if e.Research.Known[topic] {
		t.Fatal("known before the close")
	}
	evs := g.EndDay()
	if d := kinds(evs[0], Discovered); len(d) != 1 || d[0].Tech != topic || !e.Research.Known[topic] || e.Research.Topic != NoTech {
		t.Errorf("discovered %+v, topic %v", d, e.Research.Topic)
	}
}

// The Nimbuloids' project gains a day's industry now: one a day short of
// its cost completes at the close; the gain is capped at the cost; an
// ongoing project is no project running.
func TestAbilityIndustryBoost(t *testing.T) {
	g := NewGame(1, []Species{Nimbuloids, Orfa}, Average)
	e := g.Empires[0]
	c := e.Colonies[0]
	refused(t, g, 0, "no project running")
	c.Project = &Project{Item: EndlessParty, Slot: -1}
	refused(t, g, 0, "no project running")
	c.Project = nil
	at, ok := c.Planet.BestCell(Factory, e.Species.Traits())
	if !ok {
		t.Fatal("no square for a Factory")
	}
	if err := c.Start(e.Research, Factory, at); err != nil {
		t.Fatal(err)
	}
	ind, _, _ := c.Output()
	pj := c.Project
	if ind < 1 || pj.Cost() <= 2*ind {
		t.Fatalf("fixture: industry %d, cost %d", ind, pj.Cost())
	}
	pj.Progress = pj.Cost() - ind - 1 // the close alone leaves it one short
	reps := used(t, g, 0)
	if !slices.Equal(reps, []Report{abilityReport(0, -1, "Projects gain a day's industry")}) {
		t.Errorf("reports %+v", reps)
	}
	if pj.Progress != pj.Cost()-1 {
		t.Errorf("progress %d, want %d", pj.Progress, pj.Cost()-1)
	}
	if evs := g.EndDay(); len(kinds(evs[0], Built)) != 1 || c.Project != nil {
		t.Errorf("not completed: %+v", evs[0])
	}

	g = NewGame(1, []Species{Nimbuloids, Orfa}, Average)
	c = g.Empires[0].Colonies[0]
	if err := c.Start(g.Empires[0].Research, Factory, at); err != nil {
		t.Fatal(err)
	}
	c.Project.Progress = c.Project.Cost() // full, still to complete: nothing more to gain
	used(t, g, 0)
	if c.Project.Progress != c.Project.Cost() {
		t.Errorf("capped: progress %d of %d", c.Project.Progress, c.Project.Cost())
	}
}

// The Mebes' colonies, every one, gain MebesPopBonus of Capacity per use,
// theirs only.
func TestAbilityMoreMaxPop(t *testing.T) {
	g := NewGame(1, []Species{Mebes, Orfa}, Average)
	e := g.Empires[0]
	settleAt(t, g, 0, freeStar(g, 1), 0)
	c, foe := e.Colonies[0], g.Empires[1].Colonies[0]
	var c0 []int
	for _, x := range e.Colonies {
		c0 = append(c0, x.Capacity())
	}
	f0 := foe.Capacity()
	reps := used(t, g, 0)
	if !slices.Equal(reps, []Report{abilityReport(0, -1, fmt.Sprintf("Colony capacity +%d", MebesPopBonus))}) {
		t.Errorf("reports %+v", reps)
	}
	for k, x := range e.Colonies {
		if x.Planet.PopBonus != MebesPopBonus || x.Capacity() != c0[k]+MebesPopBonus {
			t.Errorf("%s: capacity %d (bonus %d), want %d", x.Planet.Name, x.Capacity(), x.Planet.PopBonus, c0[k]+MebesPopBonus)
		}
	}
	if foe.Capacity() != f0 || foe.Planet.PopBonus != 0 {
		t.Errorf("foe %d (bonus %d), want %d", foe.Capacity(), foe.Planet.PopBonus, f0)
	}
	used(t, g, 0)
	if c.Planet.PopBonus != 2*MebesPopBonus || c.Capacity() != c0[0]+2*MebesPopBonus {
		t.Errorf("second use: capacity %d, want %d", c.Capacity(), c0[0]+2*MebesPopBonus)
	}
}

func TestIsInvincible(t *testing.T) {
	e := &Empire{Invincible: -1}
	for _, d := range []int{-1, 0, 1} {
		if e.IsInvincible(d) {
			t.Errorf("never used: invincible on day %d", d)
		}
	}
	e.Invincible = 4
	for d := 2; d < 8; d++ {
		if got := e.IsInvincible(d); got != (d == 4 || d == 5) {
			t.Errorf("used day 4: day %d %v", d, got)
		}
	}
}

// The Capelons' colonies cannot be invaded through the close of the day
// of use (Day already incremented there) nor on the next day; the close
// after that captures.
func TestAbilityShieldPlanetsInvade(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Capelons)
	e0, e1 := g.Empires[0], g.Empires[1]
	reps := used(t, g, 1)
	if !slices.Equal(reps, []Report{abilityReport(1, -1, "Colonies invincible today")}) || e1.Invincible != g.Day || e0.Invincible != -1 {
		t.Errorf("reports %+v, invincible %d / %d", reps, e1.Invincible, e0.Invincible)
	}
	s := e0.Spawn(invader(3), col.Star, col.PlanetIdx)
	const why = "the Capelons's colonies are invincible today"
	if err := e0.Order(s, Order{Kind: Invade}); err == nil || err.Error() != why {
		t.Errorf("order on the day of use: %v", err)
	}
	s.Order = Order{Kind: Invade} // an old state: the close refuses it
	evs := g.EndDay()
	if col.Planet.Owner != 1 || s.Invaders != 3 || s.Order.Kind != NoOrder || len(kinds(evs[0], Invaded))+len(kinds(evs[0], Repelled)) != 0 {
		t.Fatalf("close of the day of use: owner %d, modules %d, order %v", col.Planet.Owner, s.Invaders, s.Order)
	}
	if err := e0.Order(s, Order{Kind: Invade}); err == nil || err.Error() != why {
		t.Errorf("order the next day: %v", err)
	}
	s.Order = Order{Kind: Invade}
	g.EndDay()
	if col.Planet.Owner != 0 || s.Invaders != 2 { // no shields: one of 3 lands
		t.Errorf("two days on: owner %d, modules %d", col.Planet.Owner, s.Invaders)
	}
}

// A shot at an invincible Capelons structure is spent and lands nothing,
// on the day of use and the next; the day after, it razes.
func TestAbilityShieldPlanetsRounds(t *testing.T) {
	g := NewGame(1, []Species{Capelons, Orfa}, Average)
	atWar(t, g, 0, 1)
	home := g.Empires[0].Colonies[0]
	sh := g.Empires[1].Spawn(siegeGun, home.Star, home.PlanetIdx)
	sh.Orbit = -1
	p := home.Planet
	if err := p.BuildOrbital(0, OrbitalShield); err != nil {
		t.Fatal(err)
	}
	dmg := OrbitalDefences[OrbitalShield].Strength - Gizmos[Nanomanipulator].Level
	p.OrbitDamage[0] = dmg // one shot razes it
	used(t, g, 0)
	gun := slices.Index(siegeGun.Parts, Nanomanipulator)
	for day := range 2 {
		_, log := volley(t, g, sh, home.PlanetIdx, 1)
		if p.Orbit[0] != OrbitalShield || p.OrbitDamage[0] != dmg || len(log.Razed) != 0 || sh.Shots[gun] != 1 ||
			len(log.Shots) != 1 || log.Shots[0].Damage != 0 || log.Shots[0].HitSlot != 0 {
			t.Errorf("day %d: orbit %v damage %v razed %v shots %v %+v", day, p.Orbit, p.OrbitDamage, log.Razed, sh.Shots, log.Shots)
		}
		g.EndDay()
	}
	if _, log := volley(t, g, sh, home.PlanetIdx, 1); p.Orbit[0] != NoItem || len(log.Razed) != 1 {
		t.Errorf("after: orbit %v, razed %v", p.Orbit, log.Razed)
	}
}

// The Ungooma bump every ship in a lane back where it entered: two at
// one entry are placed apart (FreeSpot), out of the lanes, their lane
// orders kept (GoToStar, GoToSystem): both fly on to the far star with
// no new order. A Report for the other empire hit.
func TestAbilityBumpShipsInLanes(t *testing.T) {
	g := NewGame(1, []Species{Ungooma, Orfa}, Average)
	c := g.Cluster
	refused(t, g, 0, "no ship in a lane")
	home := g.Empires[0].Colonies[0]
	star := home.Star
	lane := c.Stars[star].Lanes[0]
	far := c.Other(lane, star)
	_, end, _ := laneEnd(c, star, far)
	entry := c.Stars[star].System.Lanes[end].Pos
	var ships []*Starship
	orders := []Order{{Kind: GoToStar, Star: far}, {Kind: GoToSystem, Star: far}}
	for i, e := range g.Empires {
		s := e.Spawn(laneShip, star, home.PlanetIdx)
		s.Orbit, s.Pos, s.Lane, s.Arrive, s.Order = -1, entry, lane, 10, orders[i]
		ships = append(ships, s)
	}
	reps := used(t, g, 0)
	want := []Report{abilityReport(0, -1, "Lane ships bumped"), abilityReport(1, 0, "Ships bumped by Ungooma")}
	if !slices.Equal(reps, want) {
		t.Errorf("reports %+v", reps)
	}
	a, b := ships[0], ships[1]
	for i, s := range ships {
		if s.Star != star || s.Lane != -1 || s.Orbit != -1 || s.Arrive != 0 || s.Order != orders[i] {
			t.Errorf("%s: star %d lane %d orbit %d arrive %d order %v", s.Name, s.Star, s.Lane, s.Orbit, s.Arrive, s.Order)
		}
	}
	if a.Pos != entry || b.Pos != FreeSpot(entry, []Vec3{a.Pos}) || dist(a.Pos, b.Pos) < ShipGap-1e-9 {
		t.Errorf("placed at %v and %v, entry %v", a.Pos, b.Pos, entry)
	}
	for range 60 {
		g.EndDay()
		if a.Star == far && a.Lane < 0 && b.Star == far && b.Lane < 0 {
			break
		}
	}
	for _, s := range ships {
		if s.Star != far || s.Lane != -1 {
			t.Errorf("after: %s at star %d lane %d, want %d", s.Name, s.Star, s.Lane, far)
		}
	}
}

// The Frutmaka warp the alien ships out of their colonies' stars, in
// orbit or in space, to the far end of the star's lowest lane, apart; a
// ship in a lane and their own stay. A lane order goes on as a route from
// there (dropped when it landed where it went): the ship flies on. A
// Report per empire hit.
func TestAbilityRepelAlienShips(t *testing.T) {
	g := NewGame(1, []Species{Frutmaka, Orfa, Mebes}, Average)
	c := g.Cluster
	e0, e1, e2 := g.Empires[0], g.Empires[1], g.Empires[2]
	home := e0.Colonies[0]
	star := home.Star
	refused(t, g, 0, "no alien ship at our colonies")
	lane := slices.Min(c.Stars[star].Lanes)
	far := c.Other(lane, star)
	_, end, _ := laneEnd(c, far, star)
	exit := c.Stars[far].System.Lanes[end].Pos
	if obs := spaceAt(g.Empires, far, nil); len(obs) != 0 {
		t.Fatalf("fixture: ships in space at %d", far)
	}
	mine := e0.Spawn(laneShip, star, home.PlanetIdx)
	orbiting := e1.Spawn(laneShip, star, home.PlanetIdx)
	orbiting.Order = Order{Kind: GoToPlanet, Planet: 1}
	spacer := e2.Spawn(laneShip, star, home.PlanetIdx)
	spacer.Orbit, spacer.Pos = -1, Vec3{X: 0.3}
	inLane := e2.Spawn(laneShip, star, home.PlanetIdx)
	inLane.Orbit, inLane.Lane, inLane.Arrive = -1, lane, 9
	// Lane orders: one toward the star it lands at (dropped), one beyond.
	other := -1
	for _, li := range c.Stars[star].Lanes {
		if o := c.Other(li, star); o != far {
			other = o
		}
	}
	if other < 0 {
		t.Fatal("fixture: one lane at home")
	}
	toFar := e2.Spawn(laneShip, star, home.PlanetIdx)
	toFar.Order = Order{Kind: GoToStar, Star: far}
	toOther := e2.Spawn(laneShip, star, home.PlanetIdx)
	toOther.Order = Order{Kind: GoToStar, Star: other}
	reps := used(t, g, 0)
	text := "Frutmaka warp us out of " + c.Stars[star].Name
	want := []Report{abilityReport(0, -1, "Alien ships warped away"), abilityReport(1, 0, text), abilityReport(2, 0, text)}
	if !slices.Equal(reps, want) {
		t.Errorf("reports %+v", reps)
	}
	for _, s := range []*Starship{orbiting, spacer} {
		if s.Star != far || s.Orbit != -1 || s.Lane != -1 || s.Order != (Order{}) {
			t.Errorf("%s: star %d orbit %d lane %d order %v, want star %d", s.Name, s.Star, s.Orbit, s.Lane, s.Order, far)
		}
	}
	if toFar.Star != far || toFar.Order != (Order{}) || toOther.Star != far || toOther.Order != (Order{Kind: GoToSystem, Star: other}) {
		t.Errorf("lane orders: %v at %d, %v at %d", toFar.Order, toFar.Star, toOther.Order, toOther.Star)
	}
	if orbiting.Pos != exit || spacer.Pos != FreeSpot(exit, []Vec3{exit}) {
		t.Errorf("placed %v %v, exit %v", orbiting.Pos, spacer.Pos, exit)
	}
	if mine.Star != star || mine.Orbit != home.PlanetIdx || inLane.Star != star || inLane.Lane != lane {
		t.Errorf("moved: mine %d/%d, in lane %d/%d", mine.Star, mine.Orbit, inLane.Star, inLane.Lane)
	}
	e2.Ships = []*Starship{toOther} // under the fleet cap: the newest would go
	for range 400 {
		if g.EndDay(); toOther.Star == other && toOther.Lane < 0 {
			break
		}
	}
	if toOther.Star != other || toOther.Lane != -1 {
		t.Errorf("rerouted ship at %d lane %d, want %d", toOther.Star, toOther.Lane, other)
	}
}

// arbrylsFixture is a game (Arbryls, Orfa), their home star A, a star n
// one lane from it with another lane to a star m (not A), and the lane
// n→A.
func arbrylsFixture(t *testing.T) (g *Game, a, n, m, lane int) {
	t.Helper()
	g = NewGame(1, []Species{Arbryls, Orfa}, Average)
	c := g.Cluster
	a = g.Empires[0].Colonies[0].Star
	for _, li := range c.Stars[a].Lanes {
		n = c.Other(li, a)
		for _, lj := range c.Stars[n].Lanes {
			if m = c.Other(lj, n); m != a && m != g.Empires[1].Colonies[0].Star {
				return g, a, n, m, li
			}
		}
	}
	t.Fatal("no star two lanes out")
	return
}

// The Arbryls close the lanes into their systems to the others for
// BlockDays: an order in is refused (theirs accepted, a lane elsewhere
// accepted), a ship ordered before waits at the entry spending nothing,
// a ship already in the lane arrives, and once the block ends the
// waiting ship goes through.
func TestAbilityBlockStarLanes(t *testing.T) {
	g, a, n, m, lane := arbrylsFixture(t)
	c := g.Cluster
	e0, e1 := g.Empires[0], g.Empires[1]
	spawn := func(e *Empire, pos Vec3) *Starship {
		s := e.Spawn(laneShip, n, 0)
		s.Orbit, s.Pos = -1, pos
		return s
	}
	_, end, _ := laneEnd(c, n, a)
	entry := c.Stars[n].System.Lanes[end].Pos
	waiter := spawn(e1, Vec3{})
	if err := e1.Order(waiter, Order{Kind: GoToStar, Star: a}); err != nil {
		t.Fatal(err)
	}
	inLane := spawn(e1, entry)
	inLane.Lane, inLane.Arrive, inLane.Order = lane, 3, Order{Kind: GoToStar, Star: a}
	reps := used(t, g, 0)
	if !slices.Equal(reps, []Report{abilityReport(0, -1, "Lanes blocked for 20 days")}) || e0.BlockedUntil != g.Day+BlockDays {
		t.Errorf("reports %+v, until %d", reps, e0.BlockedUntil)
	}
	other := spawn(e1, Vec3{Z: 0.5})
	if err := e1.Order(other, Order{Kind: GoToStar, Star: a}); err == nil || err.Error() != "the lane is blocked by the Arbryls" {
		t.Errorf("blocked order: %v", err)
	}
	if err := e1.Order(other, Order{Kind: GoToStar, Star: m}); err != nil {
		t.Errorf("a lane elsewhere: %v", err)
	}
	theirs := spawn(e0, Vec3{Z: -0.5})
	if err := e0.Order(theirs, Order{Kind: GoToStar, Star: a}); err != nil {
		t.Errorf("their own ship: %v", err)
	}
	want := Order{Kind: GoToStar, Star: a}
	for g.Day+1 < e0.BlockedUntil {
		g.EndDay()
		if waiter.Lane >= 0 || waiter.Star != n || waiter.Order != want {
			t.Fatalf("day %d: the waiter entered: star %d lane %d order %v", g.Day, waiter.Star, waiter.Lane, waiter.Order)
		}
	}
	if dist(waiter.Pos, entry) > ShipGap+1e-9 {
		t.Errorf("waiting at %v, entry %v", waiter.Pos, entry)
	}
	if pw := waiter.Design.PowerPerDay(); waiter.Power != pw {
		t.Errorf("waiting spends: power %d, want %d", waiter.Power, pw)
	}
	if inLane.Star != a || theirs.Star != a || other.Star != m {
		t.Errorf("not through: in lane at %d, theirs at %d, other at %d", inLane.Star, theirs.Star, other.Star)
	}
	for range 60 {
		g.EndDay()
		if waiter.Star == a {
			break
		}
	}
	if waiter.Star != a {
		t.Errorf("after the block: at %d lane %d", waiter.Star, waiter.Lane)
	}
}

// A round's move holds a ship at a blocked lane's entry too, its order
// kept.
func TestAbilityBlockStarLanesRound(t *testing.T) {
	g, a, n, _, _ := arbrylsFixture(t)
	used(t, g, 0)
	e1 := g.Empires[1]
	_, end, _ := laneEnd(g.Cluster, n, a)
	s := e1.Spawn(laneShip, n, 0)
	s.Orbit, s.Pos = -1, g.Cluster.Stars[n].System.Lanes[end].Pos
	s.Order = Order{Kind: GoToStar, Star: a}
	pw := s.Power
	if acted, _, _ := g.Round(n, nil); acted || s.Lane >= 0 || s.Power != pw || s.Order != (Order{Kind: GoToStar, Star: a}) {
		t.Errorf("acted %v, lane %d, power %d of %d, order %v", acted, s.Lane, s.Power, pw, s.Order)
	}
	g.Empires[0].BlockedUntil = g.Day
	if _, _, _ = g.Round(n, nil); s.Lane < 0 {
		t.Errorf("unblocked: lane %d", s.Lane)
	}
}

// The Govorom's least populated colony (the first on a tie) turns
// Cornucopia, its black squares white: tubes removed, a structure kept.
func TestAbilityCornucopia(t *testing.T) {
	g := NewGame(2, []Species{Govorom, Orfa}, Average)
	e := g.Empires[0]
	home := e.Colonies[0]
	pl := -1
	for k, p := range g.Cluster.Stars[home.Star].System.Planets {
		if p.Owner < 0 {
			pl = k
			break
		}
	}
	if pl < 0 {
		t.Fatal("fixture: no free planet at home")
	}
	s := e.Spawn(laneShip, home.Star, pl)
	if err := e.Found(s); err != nil {
		t.Fatal(err)
	}
	col := e.Colonies[1]
	p := col.Planet
	var empty []Cell
	for _, at := range p.Cells() {
		if sq := p.Surface[at]; sq.Item == NoItem && !sq.Ruins {
			empty = append(empty, at)
		}
	}
	if len(empty) < 3 {
		t.Fatal("fixture: no room")
	}
	tubes, factory, bare := empty[0], empty[1], empty[2]
	for _, at := range empty[:3] {
		p.Surface[at].Kind = Black
	}
	// Every other square red but the Colony Base's: the reds stay, the
	// three bare ones go green and blue.
	for _, at := range p.Cells() {
		if sq := p.Surface[at]; !slices.Contains(empty[:3], at) && sq.Item != ColonyBase {
			sq.Kind = BonusIndustry
		}
	}
	reds := kindsOf(p)[BonusIndustry]
	p.Surface[tubes].Item = TransportTubes
	p.Surface[factory].Item = Factory
	homeKinds := map[Cell]SquareKind{}
	for at, sq := range home.Planet.Surface {
		homeKinds[at] = sq.Kind
	}
	homeType := home.Planet.Type

	home.Planet.Type = Tycoon // the Govorom's home is a Cornucopia already: the tie must change something
	col.Pop = home.Pop
	reps := used(t, g, 0)
	if !slices.Equal(reps, []Report{richReport(home)}) {
		t.Errorf("tie: reports %+v", reps)
	}
	for at, k := range homeKinds {
		home.Planet.Surface[at].Kind = k
	}
	home.Planet.Type = homeType
	col.Pop = home.Pop - 1
	reps = used(t, g, 0)
	if !slices.Equal(reps, []Report{richReport(col)}) {
		t.Errorf("reports %+v", reps)
	}
	if p.Type != Cornucopia {
		t.Errorf("type %v", p.Type)
	}
	// Rich: every square but the Colony Base's a bonus one, the reds
	// kept, the bare ones the two kinds short of a third.
	if !rich(p) {
		t.Errorf("not a rich world: %v", kindsOf(p))
	}
	got := kindsOf(p)
	if got[BonusIndustry] != reds || got[BonusProsperity]+got[BonusResearch] != 3 ||
		got[BonusProsperity] == 0 || got[BonusResearch] == 0 {
		t.Errorf("reds %d, then %v", reds, got)
	}
	if p.Surface[tubes].Item != NoItem || p.Surface[factory].Item != Factory || p.Surface[bare].Kind < BonusIndustry {
		t.Errorf("tubes %v factory %v bare %v", p.Surface[tubes].Item, p.Surface[factory].Item, p.Surface[bare].Kind)
	}
	if base := p.Surface[Cell{Centre, Centre}]; base.Item == ColonyBase && base.Kind != White {
		t.Errorf("the Colony Base's square went %v", base.Kind)
	}
	for at, k := range homeKinds {
		if home.Planet.Surface[at].Kind != k {
			t.Errorf("home %v changed", at)
		}
	}
	if home.Planet.Type != homeType {
		t.Errorf("home type %v", home.Planet.Type)
	}
	// col is a rich world now: the next use passes it for the home, the
	// one colony left to enrich; both rich, a use is refused.
	home.Planet.Type = Tycoon
	col.Pop = home.Pop - 1
	reps = used(t, g, 0)
	if !slices.Equal(reps, []Report{richReport(home)}) {
		t.Errorf("rich passed over: reports %+v", reps)
	}
	refused(t, g, 0, "rich world already")
	e.Colonies = nil
	refused(t, g, 0, "no system controlled") // asleep before Cornucopia's own "no colony"
}

// kindsOf counts p's squares by kind.
func kindsOf(p *Planet) map[SquareKind]int {
	n := map[SquareKind]int{}
	for _, at := range p.Cells() {
		n[p.Surface[at].Kind]++
	}
	return n
}

// readies counts i's AbilityReady events in evs, checking each one's
// fields.
func readies(t *testing.T, evs [][]Event, i int) int {
	t.Helper()
	n := 0
	for j, es := range evs {
		for _, x := range es {
			if x.Kind != AbilityReady {
				continue
			}
			if j != i || x != dipEvent(AbilityReady, -1, "Special ability ready") {
				t.Errorf("empire %d: %+v", j, x)
			}
			n++
		}
	}
	return n
}

// A timed ability starts empty: not ready on day 0, Recharge days left;
// the close reaching Day == Recharge readies it with one AbilityReady
// (none before, none on the next close); after a use, one more Recharge
// days later. A passive species never gets one, a defeated empire
// neither.
func TestAbilityReady(t *testing.T) {
	g := NewGame(1, []Species{Mebes, Orfa}, Average)
	rc := Mebes.Info().Recharge
	if ready, days := g.AbilityState(0); ready || days != rc {
		t.Fatalf("day 0: ready %v, %d days, want %d", ready, days, rc)
	}
	if _, err := g.UseAbility(0); err == nil {
		t.Error("used on day 0")
	}
	until := func(day int) int {
		n := 0
		for g.Day < day {
			n += readies(t, g.EndDay(), 0)
		}
		return n
	}
	if n := until(rc - 1); n != 0 {
		t.Errorf("%d AbilityReady before day %d", n, rc)
	}
	if ready, _ := g.AbilityState(0); ready {
		t.Error("ready a day early")
	}
	if n := until(rc); n != 1 {
		t.Errorf("%d AbilityReady on day %d", n, rc)
	}
	if ready, days := g.AbilityState(0); !ready || days != 0 {
		t.Errorf("day %d: ready %v, %d days", rc, ready, days)
	}
	if n := until(rc + 1); n != 0 {
		t.Errorf("%d AbilityReady the day after", n)
	}
	if _, err := g.UseAbility(0); err != nil {
		t.Fatal(err)
	}
	day := g.Day
	if n := until(day + rc - 1); n != 0 {
		t.Errorf("%d AbilityReady while recharging", n)
	}
	if n := until(day + rc); n != 1 {
		t.Errorf("%d AbilityReady on recharge", n)
	}
	if n := until(day + 2*rc); n != 0 {
		t.Errorf("%d AbilityReady unused", n)
	}

	g = NewGame(1, []Species{Mebes, Orfa}, Average) // a defeated one neither
	e := g.Empires[0]
	e.Lost = 1
	g.defeat(e)
	if n := until(rc); n != 0 {
		t.Errorf("defeated: %d AbilityReady", n)
	}
}

// With no system controlled (its home shared) the timed ability sleeps:
// not ready, refused (ErrAbilityAsleep), no AbilityReady at the close;
// the home its own again, it is ready and used.
func TestAbilityAsleep(t *testing.T) {
	g := NewGame(1, []Species{Ungooma, Orfa}, Average)
	e := g.Empires[0]
	charge(g, 0)
	home := e.Colonies[0]
	var other *Planet
	for k, p := range g.Cluster.Stars[home.Star].System.Planets {
		if k != home.PlanetIdx && p.Owner < 0 {
			other = p
			break
		}
	}
	if other == nil {
		t.Fatal("fixture: the home star has no other planet")
	}
	other.Owner = 1 // shared: nobody controls it
	if !g.AbilityAsleep(0) || e.ControlledSystems() != 0 {
		t.Fatalf("asleep %v, systems %d", g.AbilityAsleep(0), e.ControlledSystems())
	}
	if ready, _ := g.AbilityState(0); ready {
		t.Error("ready while asleep")
	}
	if _, err := g.UseAbility(0); !errors.Is(err, ErrAbilityAsleep) {
		t.Errorf("use while asleep: %v", err)
	}
	e.AbilityUsed = g.Day + 1 - e.Species.Info().Recharge // ready at this close
	for _, x := range g.EndDay()[0] {
		if x.Kind == AbilityReady {
			t.Error("AbilityReady while asleep")
		}
	}
	// Asleep, the recharge pauses: a charge full stays one day short.
	rc := e.Species.Info().Recharge
	for range 5 {
		g.EndDay()
	}
	other.Owner = -1
	if ready, days := g.AbilityState(0); g.AbilityAsleep(0) || ready || days != 1 {
		t.Fatalf("awake: asleep %v ready %v in %d days, want 1", g.AbilityAsleep(0), ready, days)
	}
	readies := 0
	for _, x := range g.EndDay()[0] {
		if x.Kind == AbilityReady {
			readies++
		}
	}
	if ready, _ := g.AbilityState(0); readies != 1 || !ready {
		t.Fatalf("the close after waking: %d AbilityReady, ready %v", readies, ready)
	}
	if _, err := g.UseAbility(0); err != nil && errors.Is(err, ErrAbilityAsleep) {
		t.Errorf("awake: %v", err)
	}

	// A charge half done before the sleep resumes where it stopped.
	e.AbilityUsed = g.Day - rc/2
	_, left := g.AbilityState(0)
	other.Owner = 1
	for range 7 {
		g.EndDay()
	}
	other.Owner = -1
	if _, days := g.AbilityState(0); days != left {
		t.Errorf("after 7 days asleep: ready in %d days, want %d (paused)", days, left)
	}
}
