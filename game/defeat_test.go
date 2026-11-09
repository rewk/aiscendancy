package game

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
)

// settleAt settles empire i on planet pl of star as foundAt does, without
// a ship: the Colony Base on the planet's best square, Owner set, the
// colony adopted and appended.
func settleAt(t *testing.T, g *Game, i, star, pl int) *Colony {
	t.Helper()
	e := g.Empires[i]
	p := g.Cluster.Stars[star].System.Planets[pl]
	if p.Owner >= 0 {
		t.Fatalf("fixture: %s owned by %d", p.Name, p.Owner)
	}
	at, ok := p.BestCell(ColonyBase, e.Species.Traits())
	if !ok {
		t.Fatalf("fixture: no square for a Colony Base on %s", p.Name)
	}
	p.Surface[at].Item = ColonyBase
	p.Owner = i
	c := NewColony(p)
	c.Star, c.PlanetIdx = star, pl
	c.adopt(e)
	e.Colonies = append(e.Colonies, c)
	return c
}

// freeStar is the first star with at least n planets and no owner on
// any, −1 when none.
func freeStar(g *Game, n int) int {
	for st, s := range g.Cluster.Stars {
		if len(s.System.Planets) >= n && s.System.Controller() < 0 &&
			!slices.ContainsFunc(s.System.Planets, func(p *Planet) bool { return p.Owner >= 0 }) {
			return st
		}
	}
	return -1
}

// capGame is seed 1, Snovemdomas (A, 0) and Orfa (B, 1): A has the
// colony-ship techs, a Shipyard and two Factories at home, and a second
// system (a colony on planet 0 of a free two-planet star, returned):
// cap 2 × ShipsPerSystem.
func capGame(t *testing.T) (g *Game, a, b *Empire, second int) {
	t.Helper()
	g = NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	a, b = g.Empires[0], g.Empires[1]
	for _, tc := range earlyShipTechs {
		a.Research.Grant(tc)
	}
	a.Research.Grant(SpacetimeSurfing)
	home := a.Colonies[0]
	for range 2 {
		at, ok := home.Planet.BestCell(Factory, Base)
		if !ok || home.Planet.Build(at, Factory) != nil {
			t.Fatal("no square for a Factory")
		}
	}
	if err := home.Planet.BuildOrbital(0, Shipyard); err != nil {
		t.Fatal(err)
	}
	home.Pop = 8
	second = freeStar(g, 2)
	if second < 0 {
		t.Fatal("fixture: no free two-planet star")
	}
	settleAt(t, g, 0, second, 0)
	if n, capa := a.Fleet(); n != 0 || capa != 2*ShipsPerSystem {
		t.Fatalf("fixture: fleet %d of %d", n, capa)
	}
	return g, a, b, second
}

// scrapped is the Scrapped events' texts in evs.
func scrapped(evs []Event) []string {
	var out []string
	for _, x := range evs {
		if x.Kind == Scrapped {
			out = append(out, x.Text)
		}
	}
	return out
}

// Losing a system to a foreign colony scraps the newest ships down to
// the cap: the first ones kept in order, one Scrapped each, newest first.
func TestScrapNewest(t *testing.T) {
	g, a, _, second := capGame(t)
	home := a.Colonies[0]
	n := 2*ShipsPerSystem - 1
	for range n {
		a.Spawn(FixtureColonyShip, home.Star, home.PlanetIdx)
	}
	ships := slices.Clone(a.Ships)
	settleAt(t, g, 1, second, 1)
	capa := ShipsPerSystem * a.ControlledSystems()
	if capa >= n {
		t.Fatalf("fixture: cap %d still holds %d ships", capa, n)
	}
	evs := g.EndDay()
	if !slices.Equal(a.Ships, ships[:capa]) {
		t.Fatalf("kept %d ships, want the first %d", len(a.Ships), capa)
	}
	var want []string
	for i := n - 1; i >= capa; i-- {
		want = append(want, ships[i].Name+" scrapped: system lost")
	}
	if got := scrapped(evs[0]); !slices.Equal(got, want) {
		t.Errorf("scrapped %q, want %q", got, want)
	}
	for _, x := range evs[0] {
		if x.Kind == Scrapped && (x.Star != home.Star || x.Planet != home.PlanetIdx || x.Other != -1 || x.Item != NoItem || x.Tech != NoTech) {
			t.Errorf("event %+v: want the ship's star and orbit, nobody else", x)
		}
	}
	if len(scrapped(evs[1])) != 0 || !a.Alive() {
		t.Errorf("B scrapped %q, A alive %v", scrapped(evs[1]), a.Alive())
	}
	if evs := g.EndDay(); len(scrapped(evs[0])) != 0 {
		t.Errorf("at the cap, scrapped %q", scrapped(evs[0]))
	}
}

// A ship in a lane and one being refitted are scrapped all the same; the
// refit is cancelled at the next close, with no panic.
func TestScrapLaneAndRefit(t *testing.T) {
	g, a, _, second := capGame(t)
	home := a.Colonies[0]
	a.Research.Grant(Tech(Items[OrbitalDocks].ResearchReq))
	if err := home.Planet.BuildOrbital(1, OrbitalDocks); err != nil {
		t.Fatal(err)
	}
	for range ShipsPerSystem {
		a.Spawn(FixtureColonyShip, home.Star, home.PlanetIdx)
	}
	lane := a.Spawn(FixtureColonyShip, home.Star, home.PlanetIdx)
	li := g.Cluster.Stars[home.Star].Lanes[0]
	if err := a.Order(lane, Order{Kind: GoToStar, Star: g.Cluster.Other(li, home.Star)}); err != nil {
		t.Fatal(err)
	}
	for d := 0; lane.Lane < 0; d++ {
		if d > 50 {
			t.Fatal("the ship never entered the lane")
		}
		g.EndDay()
	}
	fit := a.Spawn(FixtureColonyShip, home.Star, home.PlanetIdx)
	fit.Hull = 1 // a repair of days
	if err := a.Refit(home, fit, fit.Design); err != nil {
		t.Fatal(err)
	}
	settleAt(t, g, 1, second, 1)
	evs := g.EndDay()
	if slices.Contains(a.Ships, lane) || slices.Contains(a.Ships, fit) || len(a.Ships) != ShipsPerSystem {
		t.Fatalf("%d ships left, lane ship kept %v, refitted kept %v", len(a.Ships),
			slices.Contains(a.Ships, lane), slices.Contains(a.Ships, fit))
	}
	if got := scrapped(evs[0]); len(got) != 2 || !strings.Contains(got[0], fit.Name) || !strings.Contains(got[1], lane.Name) {
		t.Errorf("scrapped %q", got)
	}
	for _, x := range evs[0] {
		if x.Kind == Scrapped && strings.Contains(x.Text, lane.Name) && x.Planet != -1 {
			t.Errorf("lane ship's event at planet %d, want −1", x.Planet)
		}
	}
	if home.Project == nil || home.Project.Refit != fit.Name {
		t.Fatal("the refit should run until the next close")
	}
	g.EndDay()
	if home.Project != nil {
		t.Errorf("refit of a scrapped ship still running: %+v", *home.Project)
	}
}

// A Ship project over the cap is kept; its ship is scrapped on the day
// it completes.
func TestScrapCompletedShip(t *testing.T) {
	g, a, _, second := capGame(t)
	home := a.Colonies[0]
	for range ShipsPerSystem {
		a.Spawn(FixtureColonyShip, home.Star, home.PlanetIdx)
	}
	d, ok := ColonyShip(a.Research, false)
	if !ok {
		t.Fatal("no colony ship")
	}
	if err := a.BuildShip(home, d); err != nil {
		t.Fatal(err)
	}
	settleAt(t, g, 1, second, 1)
	g.EndDay()
	if home.Project == nil || home.Project.Item != Ship || len(a.Ships) != ShipsPerSystem {
		t.Fatalf("project %v, %d ships", home.Project, len(a.Ships))
	}
	home.Project.Progress = home.Project.Cost() - 1
	next := a.ShipNameFor(d.Name)
	evs := g.EndDay()
	built := slices.ContainsFunc(evs[0], func(x Event) bool { return x.Kind == Built && x.Item == Ship })
	if !built || home.Project != nil {
		t.Fatalf("not completed: %v", evs[0])
	}
	want := []string{next + " scrapped: system lost"}
	if got := scrapped(evs[0]); !slices.Equal(got, want) || len(a.Ships) != ShipsPerSystem {
		t.Errorf("scrapped %q, want %q; %d ships", got, want, len(a.Ships))
	}
}

// defeated is the Defeated events in evs.
func defeated(evs []Event) []Event {
	return slices.DeleteFunc(slices.Clone(evs), func(x Event) bool { return x.Kind != Defeated })
}

// An empire whose only system is shared lives on, its ships scrapped
// (cap 0), its colony and structures kept; once that colony is captured
// it is out at the close: its ships gone, the offers to it dropped, a
// Defeated for it and for B, the planet B's with its structures; the
// next close defeats no one again, diplomacy and abilities refuse it.
func TestDefeat(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	a, b := g.Empires[0], g.Empires[1]
	g.Contact(0, 1)
	home := a.Colonies[0]
	p := home.Planet
	if err := p.BuildOrbital(0, Shipyard); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(slices.Collect(maps.Values(p.Surface)), func(s *Square) bool { return s.Item != NoItem }) {
		t.Fatal("fixture: a bare home")
	}
	a.Spawn(FixtureColonyShip, home.Star, home.PlanetIdx)
	if len(g.Cluster.Stars[home.Star].System.Planets) < 2 {
		t.Fatal("fixture: a one-planet home system")
	}
	bc := settleAt(t, g, 1, home.Star, 1)
	evs := g.EndDay()
	if !a.Alive() || len(defeated(evs[0])) != 0 || len(a.Ships) != 0 || len(scrapped(evs[0])) != 1 ||
		!slices.Contains(a.Colonies, home) || p.Owner != a.Index || p.Orbit[0] != Shipyard {
		t.Fatalf("shared: alive %v, %d ships, scrapped %v, owner %d, orbit %v", a.Alive(), len(a.Ships), scrapped(evs[0]), p.Owner, p.Orbit)
	}

	// B captures A's home as Invade does.
	a.Spawn(FixtureColonyShip, home.Star, home.PlanetIdx)
	a.Colonies = nil
	home.adopt(b)
	b.Colonies = append(b.Colonies, home)
	p.Owner = b.Index
	items := map[Cell]Item{}
	for at, sq := range p.Surface {
		items[at] = sq.Item
	}
	g.Offers = append(g.Offers, Offer{From: 1, To: 0, Kind: AllianceOffer, Day: g.Day, Third: -1})
	evs = g.EndDay()
	if a.Lost != g.Day || a.Alive() || len(a.Ships) != 0 {
		t.Fatalf("A: lost %d (day %d), %d ships", a.Lost, g.Day, len(a.Ships))
	}
	if p.Owner != b.Index || p.Orbit[0] != Shipyard {
		t.Errorf("planet: owner %d, orbit %v", p.Owner, p.Orbit)
	}
	for at, sq := range p.Surface {
		if sq.Item != items[at] {
			t.Errorf("square %v: %v, was %v", at, sq.Item, items[at])
		}
	}
	if len(g.Offers) != 0 {
		t.Errorf("offers to the loser kept: %v", g.Offers)
	}
	homeStar := home.Star
	da, db := defeated(evs[0]), defeated(evs[1])
	if len(da) != 1 || da[0].Other != -1 || da[0].Star != homeStar || da[0].Planet != -1 || da[0].Item != NoItem || da[0].Tech != NoTech {
		t.Errorf("A's Defeated: %+v", da)
	}
	if len(db) != 1 || db[0].Other != a.Index || db[0].Star != homeStar || db[0].Planet != -1 ||
		db[0].Text != "Snovemdomas defeated" {
		t.Errorf("B's Defeated: %+v", db)
	}
	if !b.Alive() || len(b.Colonies) != 3 {
		t.Errorf("B alive %v, %d colonies", b.Alive(), len(b.Colonies))
	}
	lost := a.Lost
	evs = g.EndDay()
	if len(defeated(evs[0])) != 0 || len(defeated(evs[1])) != 0 || a.Lost != lost || !slices.Contains(b.Colonies, bc) || bc.Planet.Owner != b.Index {
		t.Errorf("second close: %v / %v, lost %d", evs[0], evs[1], a.Lost)
	}
	for name, err := range map[string]error{
		"offer from": func() error { _, err := g.OfferAlliance(0, 1); return err }(),
		"offer to":   func() error { _, err := g.OfferAlliance(1, 0); return err }(),
		"war":        func() error { _, err := g.DeclareWar(1, 0); return err }(),
		"trade":      func() error { _, err := g.OfferTrade(1, 0, TradeStars); return err }(),
		"answer": func() error {
			o := Offer{From: 0, To: 1, Kind: PeaceOffer, Day: g.Day, Third: -1}
			g.Offers = append(g.Offers, o)
			_, err := g.Answer(o, true)
			return err
		}(),
		"ability": func() error { _, err := g.UseAbility(0); return err }(),
	} {
		if !errors.Is(err, ErrDefeated) {
			t.Errorf("%s: %v, want ErrDefeated", name, err)
		}
	}
	if err := a.CanBuildShip(home, FixtureColonyShip); err == nil {
		t.Error("a defeated empire may build a ship")
	}
	if _, capa := a.Fleet(); capa != 0 {
		t.Errorf("cap %d, want 0", capa)
	}
}

// Two empires losing their last colonies on one day are both out at the
// same close: neither is told of the other's defeat.
func TestDefeatTogether(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	for _, e := range g.Empires { // both homes lost, the planets freed
		e.Colonies[0].Planet.Owner = -1
		e.Colonies = nil
	}
	evs := g.EndDay()
	for i, e := range g.Empires {
		if e.Alive() || len(defeated(evs[i])) != 1 || defeated(evs[i])[0].Other != -1 {
			t.Errorf("empire %d: alive %v, %+v", i, e.Alive(), defeated(evs[i]))
		}
	}
}

// A request about a defeated empire is refused (ErrDefeated): its
// stances stay, so requestValid alone let it pend, and a refusal counted
// against the asker (Refused) for a premise gone.
func TestRequestAboutDefeated(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa, Mebes}, Average)
	g.Contact(0, 1)
	g.Contact(0, 2)
	g.Contact(1, 2)
	g.setStance(0, 1, Alliance)
	g.setStance(0, 2, War)
	g.Empires[2].Lost = 1
	if _, err := g.Request(0, 1, AskWarOn, 2); !errors.Is(err, ErrDefeated) || len(g.Offers) != 0 {
		t.Errorf("request about a defeated third: %v, offers %v", err, g.Offers)
	}
}
