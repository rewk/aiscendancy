package bot

import (
	"math"
	"reflect"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// atWar makes a and b meet and puts them at War (the Reports dropped):
// the fixtures that stage fights on a bare NewGame (Hostile is War only).
func atWar(t testing.TB, g *game.Game, a, b int) {
	t.Helper()
	g.Contact(a, b)
	if _, err := g.DeclareWar(a, b); err != nil {
		t.Fatalf("war %d-%d: %v", a, b, err)
	}
}

// dipGame is a two-empire seed-1 game of species a and b, met (Contact),
// one bot each (gn, no research), built AFTER the contact: their seen
// ledgers hold it.
func dipGame(t *testing.T, a, b game.Species, gn *Genome) (*game.Game, []*Bot) {
	t.Helper()
	g := game.NewGame(2, []game.Species{a, b}, game.Average)
	g.Contact(0, 1)
	return g, NewAll(g, Config{NoResearch: true, Genome: gn})
}

// speciesWhere is the first species (Species order) other than not whose
// Personality satisfies ok.
func speciesWhere(t *testing.T, not game.Species, ok func(p Personality) bool) game.Species {
	t.Helper()
	for sp, p := range Personalities {
		if game.Species(sp) != not && ok(p) {
			return game.Species(sp)
		}
	}
	t.Fatal("no species fits")
	return 0
}

// arm gives e one armed ship (FixtureRaider) in orbit of its home: a war
// needs an armed ship.
func arm(e *game.Empire) { e.Spawn(game.FixtureRaider, e.Colonies[0].Star, e.Colonies[0].PlanetIdx) }

// mySystems is the stars holding a colony of e.
func mySystems(e *game.Empire) int {
	at := map[int]bool{}
	for _, c := range e.Colonies {
		at[c.Star] = true
	}
	return len(at)
}

// At contact, no grudge and no envy: the attitude is the species' Start,
// before any Act too (read from the zero grudge, nothing kept); toward
// myself or an empire out of range, none.
func TestAttitudeAtContact(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b := bots[0]
	if a, ok := b.Attitude(1); !ok || a != Personalities[game.Snovemdomas].Start || b.attOK[1] {
		t.Errorf("before any Act: attitude %d %v, want %d, not kept", a, ok, Personalities[game.Snovemdomas].Start)
	}
	b.Act()
	if a, ok := b.Attitude(1); !ok || a != Personalities[game.Snovemdomas].Start {
		t.Errorf("attitude %d %v, want %d", a, ok, Personalities[game.Snovemdomas].Start)
	}
	if _, ok := b.Attitude(0); ok {
		t.Error("an attitude toward myself")
	}
	if _, ok := b.Attitude(len(g.Empires)); ok {
		t.Error("an attitude out of range")
	}
	for _, r := range b.News() { // an exchange proposed, nothing else
		if r.Event.Kind != game.TradeOffered {
			t.Errorf("news at contact: %v", r)
		}
	}
}

// Envy: EnvyW per system the other holds beyond mine, its systems read
// off Planet.Owner at the stars I Know (one per star, whatever the
// planets).
func TestAttitudeEnvy(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b, e := bots[0], g.Empires[0]
	home := e.Colonies[0].Star
	foreign := g.Empires[1].Colonies[0].Star
	k := 0
	for _, st := range neighbours(g.Cluster, home) {
		if st == foreign || len(g.Cluster.Stars[st].System.Planets) == 0 {
			continue
		}
		e.Visit(st)
		for _, p := range g.Cluster.Stars[st].System.Planets {
			p.Owner = 1 // every planet: still one system
		}
		k++
	}
	if k < 2 {
		t.Fatalf("only %d neighbour stars with planets", k)
	}
	if e.Knows(foreign) {
		k++
	}
	b.diplomacy()
	want := Personalities[game.Snovemdomas].Start - int(NeutralDiplomacy().EnvyW)*max(0, k-mySystems(e))
	if a, _ := b.Attitude(1); a != want || k-mySystems(e) < 1 {
		t.Errorf("attitude %d, want %d (%d systems vs %d)", a, want, k, mySystems(e))
	}
}

// A ship of mine lost to j adds GrudgeShip, which then fades by Forgive
// a day: after n Acts, Start − GrudgeShip × (1 − Forgive)^(n−1).
func TestGrudgeShipFades(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b := bots[0]
	g.Empires[0].Rel(1).ShipsLost++
	dg, start := NeutralDiplomacy(), float64(Personalities[game.Snovemdomas].Start)
	for n := 1; n <= 6; n++ {
		b.Act()
		want := int(math.Round(start - dg.GrudgeShip*math.Pow(1-dg.Forgive, float64(n-1))))
		if a, _ := b.Attitude(1); a != want {
			t.Errorf("day %d: attitude %d, want %d", n, a, want)
		}
	}
}

// The intrusions weigh IntrudeW × 50 / ShipPatience each, the others
// their own gene, all on one day; read once (seen), never again.
func TestGrudgeLedger(t *testing.T) {
	g, bots := dipGame(t, game.Orfa, game.Snovemdomas, nil)
	b := bots[0]
	r := g.Empires[0].Rel(1)
	r.PlanetsLost, r.Refused, r.Intrusions = 1, 2, 3
	dg, p := NeutralDiplomacy(), Personalities[game.Orfa]
	grudge := dg.GrudgePlanet + 2*dg.GrudgeRefusal + 3*dg.IntrudeW*50/float64(p.ShipPatience)
	// Day 2 adds nothing (the ledger as seen): the day-1 grudge fades.
	for day, g := range []float64{grudge, grudge * (1 - dg.Forgive)} {
		b.diplomacy()
		want := int(math.Round(float64(p.Start) - g))
		if a, _ := b.Attitude(1); a != max(0, want) {
			t.Errorf("day %d: attitude %d, want %d", day+1, a, want)
		}
	}
}

// A species whose Start − GrudgeWar falls under DeclareWar declares war
// on Act once j has declared one (then made peace); unless j's ships
// seen outnumber my armed ones beyond WarCaution.
func TestDeclareWar(t *testing.T) {
	dg := NeutralDiplomacy()
	sp := speciesWhere(t, game.NumSpecies, func(p Personality) bool { return float64(p.Start)-dg.GrudgeWar < float64(p.DeclareWar) })
	other := game.Orfa
	if sp == other {
		other = game.Snovemdomas
	}
	for _, caution := range []bool{false, true} {
		g := game.NewGame(1, []game.Species{sp, other}, game.Average)
		bots := NewAll(g, Config{NoResearch: true}) // seen: before the war
		g.Contact(0, 1)
		if _, err := g.DeclareWar(1, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := g.OfferPeace(1, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := g.Answer(g.PendingOffers(0)[0], true); err != nil {
			t.Fatal(err)
		}
		e0, e1 := g.Empires[0], g.Empires[1]
		arm(e0)
		if caution {
			e1.Spawn(game.FixtureRaider, e0.Colonies[0].Star, 0)
			e1.Spawn(game.FixtureRaider, e0.Colonies[0].Star, 0)
		}
		bots[0].Act()
		news := bots[0].News()
		if hostile := g.Hostile(e0, e1); hostile == caution {
			t.Errorf("caution %v: hostile %v", caution, hostile)
		}
		if caution {
			if len(news) != 0 {
				t.Errorf("caution: news %v", news)
			}
			continue
		}
		if len(news) != 2 || news[0].Event.Kind != game.WarDeclared || news[1].Event.Kind != game.WarDeclared ||
			news[0].Empire != 0 || news[1].Empire != 1 {
			t.Errorf("news %+v", news)
		}
		if len(bots[0].News()) != 0 {
			t.Error("News did not drain")
		}
	}
}

// At War, a bot at ProposePeace or more offers peace; the other answers
// on its Act: accepted at AcceptPeace or more (Peace), else refused (the
// offerer's Refused grows).
func TestPeace(t *testing.T) {
	a := speciesWhere(t, game.NumSpecies, func(p Personality) bool { return p.Start >= p.ProposePeace })
	acc := speciesWhere(t, a, func(p Personality) bool { return p.Start >= p.AcceptPeace })
	ref := speciesWhere(t, a, func(p Personality) bool { return p.Start < p.AcceptPeace })
	for _, tc := range []struct {
		b      game.Species
		accept bool
	}{{acc, true}, {ref, false}} {
		g := game.NewGame(1, []game.Species{a, tc.b}, game.Average)
		atWar(t, g, 0, 1)
		bots := NewAll(g, Config{NoResearch: true}) // seen: after the war
		bots[0].Act()
		if o := g.PendingOffers(1); len(o) != 1 || o[0].From != 0 {
			t.Fatalf("%s: offers %+v", tc.b, o)
		}
		if n := bots[0].News(); len(n) != 2 || n[0].Event.Kind != game.PeaceOffered {
			t.Errorf("%s: offerer's news %+v", tc.b, n)
		}
		bots[1].Act()
		want := game.War
		if tc.accept {
			want = game.Peace
		}
		if st := g.Empires[0].StanceWith(1); st != want {
			t.Errorf("%s: stance %v, want %v", tc.b, st, want)
		}
		if r := g.Empires[0].Rel(1).Refused; (r == 1) == tc.accept {
			t.Errorf("%s: Refused %d", tc.b, r)
		}
		kind := game.OfferRefused
		if tc.accept {
			kind = game.PeaceMade
		}
		if n := bots[1].News(); len(n) != 2 || n[0].Event.Kind != kind {
			t.Errorf("%s: answerer's news %+v", tc.b, n)
		}
		if len(g.PendingOffers(1)) != 0 {
			t.Errorf("%s: offer still pending", tc.b)
		}
	}
}

// Temper moves the six thresholds: lifting DeclareWar over Start makes
// the bot declare war at contact (no grudge, nothing seen); 0 does not.
func TestTemper(t *testing.T) {
	p := Personalities[game.Snovemdomas]
	for _, tc := range []struct {
		temper float64
		war    bool
	}{{0, false}, {float64(p.Start-p.DeclareWar) + 1, true}} {
		gn := genomeWith(func(g *Genome) { g.Temper = tc.temper })
		g, bots := dipGame(t, game.Snovemdomas, game.Orfa, gn)
		th := bots[0].Thresholds()
		d := int(tc.temper)
		if th.DeclareWar != p.DeclareWar+d || th.AcceptPeace != p.AcceptPeace+d || th.ProposePeace != p.ProposePeace+d ||
			th.BreakAlliance != p.BreakAlliance+d || th.AcceptAlliance != p.AcceptAlliance+d || th.ProposeAlliance != p.ProposeAlliance+d ||
			th.Start != p.Start || th.ShipPatience != p.ShipPatience {
			t.Errorf("temper %v: thresholds %+v of %+v", tc.temper, th, p)
		}
		arm(g.Empires[0])
		bots[0].Act()
		if war := g.Empires[0].StanceWith(1) == game.War; war != tc.war {
			t.Errorf("temper %v: war %v", tc.temper, war)
		}
	}
}

// The game's atmosphere shifts the six thresholds AtmosphereShift a
// step: Hostile puts the Snovemdomas (Start 90 over DECLARE_WAR) at
// war on contact, Peaceful not; Start and ShipPatience stay.
func TestAtmosphere(t *testing.T) {
	p := Personalities[game.Snovemdomas]
	for _, tc := range []struct {
		at  game.Atmosphere
		war bool
	}{{game.Peaceful, false}, {game.Neutral, false}, {game.Hostile, true}} {
		g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
		g.Atmosphere = tc.at
		th := bots[0].Thresholds()
		d := int(tc.at) * AtmosphereShift
		if th.DeclareWar != p.DeclareWar+d || th.ProposeAlliance != p.ProposeAlliance+d || th.Start != p.Start || th.ShipPatience != p.ShipPatience {
			t.Errorf("%s: thresholds %+v of %+v", tc.at, th, p)
		}
		arm(g.Empires[0])
		bots[0].Act()
		if war := g.Empires[0].StanceWith(1) == game.War; war != tc.war {
			t.Errorf("%s: war %v", tc.at, war)
		}
	}
}

// Save/Resume carry the grudge and the seen ledger, the attitude read
// from them before the first Act; a State whose slices do not hold one
// per empire resumes fresh.
func TestDiplomacySaveResume(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b, e := bots[0], g.Empires[0]
	e.Rel(1).ShipsLost += 2
	b.Act()
	st := b.Save()
	if st.Grudge[1] == 0 || st.Seen[1].ShipsLost != 2 || len(st.Grudge) != len(g.Empires) || len(st.Weary) != len(g.Empires) {
		t.Fatalf("state %v %v %+v", st.Grudge, st.Weary, st.Seen)
	}
	r := Resume(e, st)
	if again := r.Save(); !reflect.DeepEqual(again, st) {
		t.Errorf("round trip:\n%+v\n%+v", again, st)
	}
	// att is not saved: before its first Act the resumed bot answers from
	// the saved grudge, the same attitude, and keeps nothing.
	want, _ := b.Attitude(1)
	if a, ok := r.Attitude(1); !ok || a != want || want == Personalities[game.Snovemdomas].Start || r.attOK[1] {
		t.Errorf("resumed attitude %d %v, want %d (kept %v)", a, ok, want, r.attOK[1])
	}
	if _, ok := r.Attitude(0); ok {
		t.Error("a resumed attitude toward myself")
	}
	st.Grudge = st.Grudge[:1]
	f := Resume(e, st).Save()
	if f.Grudge[1] != 0 || !reflect.DeepEqual(f.Seen, e.Relations) {
		t.Errorf("mismatched resume: %v %+v", f.Grudge, f.Seen)
	}
}

// Day gathers the bots' News into its Reports, before the rounds.
func TestDayNews(t *testing.T) {
	p := Personalities[game.Snovemdomas]
	gn := genomeWith(func(g *Genome) { g.Temper = float64(p.Start-p.DeclareWar) + 1 })
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, gn)
	arm(g.Empires[0])
	_, reps, _ := Day(g, bots)
	if len(reps) < 2 || reps[0].Event.Kind != game.WarDeclared || reps[1].Event.Kind != game.WarDeclared {
		t.Errorf("reports %+v", reps)
	}
}

// The diplomacy genes: Neutral holds NeutralDiplomacy, which a nil genome
// plays; Clamp bounds each.
func TestDiplomacyGenes(t *testing.T) {
	n := Neutral()
	if n.Diplomacy() != NeutralDiplomacy() || (&Bot{}).dipGenes() != NeutralDiplomacy() {
		t.Errorf("neutral %+v", n.Diplomacy())
	}
	g := &Genome{}
	g.setDiplomacy(Diplomacy{-1e9, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1})
	g.Clamp()
	if g.Diplomacy() != (Diplomacy{Temper: -300, TradeZeal: 10, AskZeal: 10, FreeResearchDays: 1}) {
		t.Errorf("low clamp %+v", g.Diplomacy())
	}
	g.setDiplomacy(Diplomacy{1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9, 1e9})
	g.Clamp()
	if g.Diplomacy() != (Diplomacy{300, 100, 400, 1000, 1000, 200, 50, 0.2, 5, 20, 200, 2, 1000, 3, 400, 1, 500, 400, 1000, 400, 1000, 1000, 200}) {
		t.Errorf("high clamp %+v", g.Diplomacy())
	}
}

// At war without losses the weariness lifts the attitude: Day k (Act
// then EndDay) sees the war declared on day 0 at k = 1 (GrudgeWar) and
// one WarDay from k = 2 on (WearyW), both fading by Forgive; the other
// empire has no bot, so the war goes on.
func TestWeary(t *testing.T) {
	g, bots := dipGame(t, game.Minions, game.Orfa, nil)
	bots[1] = nil
	if _, err := g.DeclareWar(1, 0); err != nil {
		t.Fatal(err)
	}
	dg, start := NeutralDiplomacy(), float64(Personalities[game.Minions].Start)
	grudge, weary := 0.0, 0.0
	for k := 1; k <= 6; k++ {
		Day(g, bots)
		grudge *= 1 - dg.Forgive
		weary *= 1 - dg.Forgive
		if k == 1 {
			grudge += dg.GrudgeWar
		} else {
			weary += dg.WearyW
		}
		want := int(math.Round(start - grudge + weary))
		if att, ok := bots[0].Attitude(1); !ok || att != want {
			t.Errorf("day %d: attitude %d %v, want %d", k, att, ok, want)
		}
	}
	if weary <= 0 || g.Hostile(g.Empires[0], g.Empires[1]) != true {
		t.Fatalf("fixture: weary %v, hostile %v", weary, g.Hostile(g.Empires[0], g.Empires[1]))
	}
}

// A bot under DeclareWar with no armed ship declares no war; one armed
// ship, seeing none of theirs, does.
func TestWarNeedsArms(t *testing.T) {
	gn := Neutral()
	gn.Temper = 300 // every threshold over Start: war wanted at contact
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, gn)
	e0, e1 := g.Empires[0], g.Empires[1]
	if th := bots[0].Thresholds(); th.DeclareWar <= Personalities[game.Snovemdomas].Start {
		t.Fatalf("fixture: DeclareWar %d not over Start", th.DeclareWar)
	}
	bots[0].Act()
	if g.Hostile(e0, e1) || len(bots[0].News()) != 0 {
		t.Fatal("war with no ship at all")
	}
	arm(e0)
	bots[0].Act()
	if !g.Hostile(e0, e1) {
		t.Error("one armed ship, none of theirs seen: no war")
	}
}

// ally makes a and b allies through the game (an offer accepted).
func ally(t testing.TB, g *game.Game, a, b int) {
	t.Helper()
	if _, err := g.OfferAlliance(a, b); err != nil {
		t.Fatal(err)
	}
	for _, o := range g.PendingOffers(b) {
		if o.From == a && o.Kind == game.AllianceOffer {
			if _, err := g.Answer(o, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	if st := g.Empires[a].StanceWith(b); st != game.Alliance {
		t.Fatalf("fixture: stance %v", st)
	}
}

// tradeable is the first exchange kind empire from may offer to: one
// where to would learn something.
func tradeable(t testing.TB, g *game.Game, from, to int) game.OfferKind {
	t.Helper()
	for _, k := range []game.OfferKind{game.TradeStars, game.TradeLanes, game.TradeResearch} {
		if !g.Gain(to, from, k).Zero() {
			return k
		}
	}
	t.Fatal("fixture: nothing to exchange")
	return 0
}

// Goodwill: an exchange concluded with j (offered by j, accepted through
// the game) lifts the attitude by TradeW on the next Act, then fades by
// Forgive a day; Hospitality points from j lift it by HospW each, a
// request of mine j granted by HelpW.
func TestGoodwill(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b := bots[0]
	dg, start := NeutralDiplomacy(), float64(Personalities[game.Snovemdomas].Start)
	b.Act()
	if a, _ := b.Attitude(1); a != int(start) {
		t.Fatalf("before: attitude %d", a)
	}
	k := tradeable(t, g, 1, 0)
	if _, err := g.OfferTrade(1, 0, k); err != nil {
		t.Fatal(err)
	}
	for _, o := range g.PendingOffers(0) {
		if _, err := g.Answer(o, true); err != nil {
			t.Fatal(err)
		}
	}
	if g.Empires[0].Rel(1).Trades != 1 {
		t.Fatalf("fixture: %d trades", g.Empires[0].Rel(1).Trades)
	}
	for n := 0; n < 4; n++ {
		b.Act()
		want := int(math.Round(start + dg.TradeW*math.Pow(1-dg.Forgive, float64(n))))
		if a, _ := b.Attitude(1); a != want {
			t.Errorf("Act %d after the exchange: attitude %d, want %d", n+1, a, want)
		}
	}

	g, bots = dipGame(t, game.Snovemdomas, game.Orfa, nil)
	g.Empires[0].Rel(1).Hospitality += 75
	bots[0].Act()
	if a, _ := bots[0].Attitude(1); a != int(math.Round(start+75*dg.HospW)) {
		t.Errorf("hospitality: attitude %d, want %v", a, start+75*dg.HospW)
	}

	g, bots = dipGame(t, game.Snovemdomas, game.Orfa, nil)
	g.Empires[0].Rel(1).Helped++
	bots[0].Act()
	if a, _ := bots[0].Attitude(1); a != int(math.Round(start+dg.HelpW)) || dg.HelpW == 0 {
		t.Errorf("granted: attitude %d, want %v", a, start+dg.HelpW)
	}
}

// An alliance j broke with me (through the game) weighs GrudgeBreak.
func TestGrudgeBreak(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	ally(t, g, 0, 1)
	if _, err := g.BreakAlliance(1, 0); err != nil {
		t.Fatal(err)
	}
	bots[0].Act()
	want := max(0, Personalities[game.Snovemdomas].Start-int(NeutralDiplomacy().GrudgeBreak))
	if a, _ := bots[0].Attitude(1); a != want {
		t.Errorf("attitude %d, want %d", a, want)
	}
}

// Every threshold under the attitude (Temper −1000, unclamped): the
// first bot offers an alliance on its Act, the other accepts on its own.
func TestAllianceMade(t *testing.T) {
	gn := genomeWith(func(g *Genome) { g.Temper = -1000 })
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, gn)
	bots[0].Act()
	if o := g.PendingOffers(1); len(o) != 1 || o[0].Kind != game.AllianceOffer || o[0].From != 0 {
		t.Fatalf("offers %+v", o)
	}
	if n := bots[0].News(); len(n) != 2 || n[0].Event.Kind != game.AllianceOffered {
		t.Errorf("offerer's news %+v", n)
	}
	bots[1].Act()
	if st := g.Empires[0].StanceWith(1); st != game.Alliance {
		t.Errorf("stance %v", st)
	}
	if n := bots[1].News(); len(n) < 2 || n[0].Event.Kind != game.AllianceMade || n[1].Event.Kind != game.AllianceMade {
		t.Errorf("answerer's news %+v", n)
	}
}

// Allied, under BreakAlliance the bot breaks the alliance on its Act and
// declares no war that same Act; under DeclareWar too (Temper +1000) it
// declares war on its next Act, from Peace; between the two (the
// Snovemdomas' Start, neutral) it stays at Peace.
func TestAllianceBroken(t *testing.T) {
	p := Personalities[game.Snovemdomas]
	if p.Start < p.DeclareWar || p.Start >= p.BreakAlliance {
		t.Fatalf("fixture: Start %d not in [DeclareWar, BreakAlliance)", p.Start)
	}
	for _, tc := range []struct {
		temper float64
		then   game.Stance
	}{{0, game.Peace}, {1000, game.War}} {
		gn := genomeWith(func(g *Genome) { g.Temper = tc.temper })
		g, bots := dipGame(t, game.Snovemdomas, game.Orfa, gn)
		arm(g.Empires[0])
		ally(t, g, 1, 0)
		bots[0].Act()
		if st := g.Empires[0].StanceWith(1); st != game.Peace || g.Empires[1].Rel(0).Broken != 1 {
			t.Errorf("temper %v: stance %v, Broken %d after the first Act", tc.temper, st, g.Empires[1].Rel(0).Broken)
		}
		if n := bots[0].News(); len(n) != 2 || n[0].Event.Kind != game.AllianceBroken {
			t.Errorf("temper %v: news %+v", tc.temper, n)
		}
		bots[0].Act()
		if st := g.Empires[0].StanceWith(1); st != tc.then {
			t.Errorf("temper %v: stance %v after the second Act, want %v", tc.temper, st, tc.then)
		}
	}
}

// j's exchange is accepted when what I learn is worth TradeFair × what
// j learns: one learning nothing (every star j Knows known already) is
// refused at TradeFair 0.5, accepted at 0.
func TestTradeFair(t *testing.T) {
	for _, tc := range []struct {
		fair   float64
		accept bool
	}{{0.5, false}, {0, true}} {
		g, bots := dipGame(t, game.Snovemdomas, game.Orfa, genomeWith(func(g *Genome) { g.TradeFair = tc.fair }))
		e0, e1 := g.Empires[0], g.Empires[1]
		for st := range g.Cluster.Stars {
			if e1.Knows(st) {
				e0.Visit(st)
			}
		}
		if mine, theirs := worth(g.Gain(0, 1, game.TradeStars)), worth(g.Gain(1, 0, game.TradeStars)); mine != 0 || theirs == 0 {
			t.Fatalf("fixture: worth %d to me, %d to them", mine, theirs)
		}
		if _, err := g.OfferTrade(1, 0, game.TradeStars); err != nil {
			t.Fatal(err)
		}
		bots[0].Act()
		if got := e1.Rel(0).Trades == 1; got != tc.accept || e1.Rel(0).Refused == 1 == tc.accept {
			t.Errorf("TradeFair %v: Trades %d Refused %d", tc.fair, e1.Rel(0).Trades, e1.Rel(0).Refused)
		}
		if len(g.PendingOffers(0)) != 0 {
			t.Errorf("TradeFair %v: still pending", tc.fair)
		}
	}
}

// The proposal is the kind worth the most to me (Gain through the
// game), ties to the first: as the game stands at contact, once I know
// every lane j does, and once I know all j knows but one lane and one
// tech (lanes and research tie at 1: lanes).
func TestTradeProposalKind(t *testing.T) {
	var kinds []game.OfferKind
	for _, fx := range []string{"contact", "lanes", "tie"} {
		g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
		e0, e1 := g.Empires[0], g.Empires[1]
		if fx != "contact" {
			for li := range g.Cluster.Lanes {
				if e1.KnowsLane(li) {
					e0.LearnLane(li)
				}
			}
		}
		if fx == "tie" {
			for st := range g.Cluster.Stars {
				if e1.Knows(st) {
					e0.Visit(st)
				}
			}
			for li := range g.Cluster.Lanes {
				if !e0.KnowsLane(li) {
					e1.LearnLane(li)
					break
				}
			}
			for tc := game.Tech(0); tc < game.NumTechs && g.Gain(0, 1, game.TradeResearch).Tech == game.NoTech; tc++ {
				if !e0.Research.Known[tc] {
					e1.Research.Grant(tc)
				}
			}
			if s, l, r := worth(g.Gain(0, 1, game.TradeStars)), worth(g.Gain(0, 1, game.TradeLanes)), worth(g.Gain(0, 1, game.TradeResearch)); s != 0 || l != 1 || r != 1 {
				t.Fatalf("tie fixture: worth %d %d %d", s, l, r)
			}
		}
		want, best := game.TradeStars, -1
		for _, k := range []game.OfferKind{game.TradeStars, game.TradeLanes, game.TradeResearch} {
			if w := worth(g.Gain(0, 1, k)); w > best {
				want, best = k, w
			}
		}
		bots[0].Act()
		if o := g.PendingOffers(1); len(o) != 1 || o[0].Kind != want {
			t.Errorf("%s: offers %+v, want %s (worth %d)", fx, o, want, best)
		}
		kinds = append(kinds, want)
	}
	if kinds[0] == kinds[1] || kinds[2] != game.TradeLanes {
		t.Errorf("fixture: kinds %v", kinds)
	}
}

// TradeZeal spaces the proposals to one empire: none the day after one
// (answered: refused), the next round(TradeZeal) days later; a proposal
// the game refuses (one of mine pending) still counts.
func TestTradeZeal(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b, zeal := bots[0], int(math.Round(NeutralDiplomacy().TradeZeal))
	refuse := func() int {
		n := 0
		for _, o := range g.PendingOffers(1) {
			if _, err := g.Answer(o, false); err != nil {
				t.Fatal(err)
			}
			n++
		}
		return n
	}
	b.Act()
	if refuse() != 1 || b.lastTrade[1] != 0 {
		t.Fatalf("day 0: no proposal (lastTrade %d)", b.lastTrade[1])
	}
	for g.Day < zeal {
		g.EndDay()
		b.Act()
		if n := refuse(); (n == 1) != (g.Day == zeal) || n > 1 {
			t.Errorf("day %d: %d proposals", g.Day, n)
		}
	}

	g, bots = dipGame(t, game.Snovemdomas, game.Orfa, nil)
	if _, err := g.OfferTrade(0, 1, tradeable(t, g, 0, 1)); err != nil {
		t.Fatal(err)
	}
	g.EndDay()
	bots[0].Act() // its OfferTrade refused: already offered
	if bots[0].lastTrade[1] != g.Day || len(g.PendingOffers(1)) != 1 {
		t.Errorf("refused proposal: lastTrade %d on day %d, offers %+v", bots[0].lastTrade[1], g.Day, g.PendingOffers(1))
	}
}

// hospBot is empire 0 of a dipGame (gn), its home granted the techs of
// grant (none given: Alien Hospitality, Endless Party and Scientist
// Takeover), plus extra colonies
// founded on free planets of its home star, all idle.
func hospBot(t *testing.T, gn *Genome, extra int, grant ...game.Item) (*game.Game, *Bot) {
	t.Helper()
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, gn)
	b, e := bots[0], g.Empires[0]
	if grant == nil {
		grant = []game.Item{game.AlienHospitality, game.EndlessParty, game.ScientistTakeover}
	}
	for _, it := range grant {
		e.Research.Grant(game.Tech(it.Info().ResearchReq))
	}
	home := e.Colonies[0].Star
	for pl := range g.Cluster.Stars[home].System.Planets {
		if len(e.Colonies) > extra || !b.foundable(home, pl) {
			continue
		}
		if err := e.Found(e.Spawn(game.FixtureColonyShip, home, pl)); err != nil {
			t.Fatal(err)
		}
		e.Ships = e.Ships[:len(e.Ships)-1]
	}
	if len(e.Colonies) != 1+extra {
		t.Fatalf("fixture: %d colonies", len(e.Colonies))
	}
	return g, b
}

// projectOf is col's project item, NoItem when idle.
func projectOf(c *game.Colony) game.Item {
	if c.Project == nil {
		return game.NoItem
	}
	return c.Project.Item
}

// An idle colony runs Alien Hospitality when researched, an empire met
// at Peace, and fewer of my other colonies on it than ceil(Hospitable ×
// colonies): Hospitable 1 yes, 0 the party as before; no empire at
// Peace (allied, or at war) the party; not researched the party;
// Hospitable 0.5 or 0.25 over two colonies (ceil: one): the first only,
// day after day.
func TestIdleHospitality(t *testing.T) {
	for _, tc := range []struct {
		hosp   float64
		stance game.Stance
		grant  []game.Item
		want   game.Item
	}{
		{1, game.Peace, nil, game.AlienHospitality},
		{0, game.Peace, nil, game.EndlessParty},
		{1, game.Alliance, nil, game.EndlessParty},
		{1, game.War, nil, game.EndlessParty},
		{1, game.Peace, []game.Item{game.EndlessParty, game.ScientistTakeover}, game.EndlessParty},
	} {
		g, b := hospBot(t, genomeWith(func(g *Genome) { g.Hospitable = tc.hosp }), 0, tc.grant...)
		switch tc.stance {
		case game.Alliance:
			ally(t, g, 0, 1)
		case game.War:
			atWar(t, g, 0, 1)
		}
		col := b.E.Colonies[0]
		if b.idle(col); projectOf(col) != tc.want {
			t.Errorf("Hospitable %v, %v, granted %v: %s, want %s", tc.hosp, tc.stance, tc.grant, projectOf(col), tc.want)
		}
	}

	for _, share := range []float64{0.5, 0.25} {
		_, b := hospBot(t, genomeWith(func(g *Genome) { g.Hospitable = share }), 1)
		for day := range 3 {
			for _, col := range b.E.Colonies {
				b.idle(col)
			}
			if a, c := projectOf(b.E.Colonies[0]), projectOf(b.E.Colonies[1]); a != game.AlienHospitality || c == game.AlienHospitality {
				t.Errorf("Hospitable %v, pass %d: %s, %s", share, day, a, c)
			}
		}
	}
}

// No Colonizer for an ally's system: foundable is false at a star where
// an ally holds a colony; at peace nor while courting it (courted), true
// once I want war (the alliance broken).
func TestFoundableAlly(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b, e1 := bots[0], g.Empires[1]
	home := g.Empires[0].Colonies[0].Star
	var free []int
	for pl := range g.Cluster.Stars[home].System.Planets {
		if b.foundable(home, pl) {
			free = append(free, pl)
		}
	}
	if len(free) < 2 {
		t.Fatalf("fixture: %d free planets at home", len(free))
	}
	if err := e1.Found(e1.Spawn(game.FixtureColonyShip, home, free[0])); err != nil {
		t.Fatal(err)
	}
	e1.Ships = nil
	pl := free[1]
	b.Act()
	b.att[1] = b.Thresholds().DeclareWar // courting (the founding beside me, an encroachment, brought it lower)
	if b.foundable(home, pl) {
		t.Fatal("at peace, courting: foundable")
	}
	ally(t, g, 0, 1)
	if b.foundable(home, pl) {
		t.Error("an ally's system: foundable")
	}
	if _, err := g.BreakAlliance(1, 0); err != nil {
		t.Fatal(err)
	}
	b.att[1], b.attOK[1] = b.Thresholds().DeclareWar-1, true
	if !b.foundable(home, pl) {
		t.Error("alliance broken, wanting war: not foundable")
	}
}

// Save/Resume carry the goodwill and lastTrade; one slice short resumes
// the whole diplomacy fresh (lastTrade −1).
func TestDiplomacySaveResumeLot2(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b, e := bots[0], g.Empires[0]
	e.Rel(1).Trades++
	b.Act()
	st := b.Save()
	if st.Goodwill[1] == 0 || st.LastTrade[1] != 0 || len(st.Goodwill) != len(g.Empires) || len(st.LastTrade) != len(g.Empires) {
		t.Fatalf("state %v %v", st.Goodwill, st.LastTrade)
	}
	if again := Resume(e, st).Save(); !reflect.DeepEqual(again, st) {
		t.Errorf("round trip:\n%+v\n%+v", again, st)
	}
	b.goodwill, b.lastTrade = b.goodwill[:1], nil // a bot out of step: Act starts the diplomacy fresh
	b.Act()
	if f := b.Save(); len(f.Goodwill) != len(g.Empires) || len(f.LastTrade) != len(g.Empires) {
		t.Errorf("out of step: %v %v", f.Goodwill, f.LastTrade)
	}
	for _, cut := range []func(s *State){func(s *State) { s.Goodwill = s.Goodwill[:1] }, func(s *State) { s.LastTrade = nil }} {
		c := *b.Save()
		cut(&c)
		f := Resume(e, &c).Save()
		if f.Goodwill[1] != 0 || f.LastTrade[1] != -1 || f.Grudge[1] != 0 {
			t.Errorf("mismatched resume: %v %v %v", f.Goodwill, f.LastTrade, f.Grudge)
		}
	}
}

// reqGame3 is a three-empire seed-1 game of species a, b and the Mebes,
// every pair met, 0 and 1 allied (1 offered: 0's cooldown toward 1
// untouched, so 0 may ask 1 at once); no bots.
func reqGame3(t *testing.T, a, b game.Species) *game.Game {
	t.Helper()
	g := game.NewGame(1, []game.Species{a, b, game.Mebes}, game.Average)
	g.Contact(0, 1)
	ally(t, g, 1, 0)
	g.Contact(0, 2)
	g.Contact(1, 2)
	return g
}

// Decide on an ally's request reads the attitude toward Third (here
// none yet: the Orfa's Start), whatever the attitude toward the asker (0
// or 1000), against the act's threshold bent by FavourW: a Temper puts
// Start just outside the plain one, so FavourW 0 refuses, 500 accepts.
// The war request also needs arms (warReady): unarmed, refused even at
// FavourW 500.
func TestDecideRequests(t *testing.T) {
	p := Personalities[game.Orfa]
	for _, tc := range []struct {
		kind   game.OfferKind
		temper int // the plain threshold just past Start
	}{
		{game.AskWarOn, p.Start - 10 - p.DeclareWar},             // DeclareWar = Start − 10: Start not under it
		{game.AskPeaceWith, p.Start + 10 - p.ProposePeace},       // ProposePeace = Start + 10
		{game.AskBreakWith, p.Start - 10 - p.BreakAlliance},      // BreakAlliance = Start − 10
		{game.AskAllianceWith, p.Start + 10 - p.ProposeAlliance}, // ProposeAlliance = Start + 10
	} {
		for _, fav := range []float64{0, 500} {
			for _, armed := range []bool{false, true} {
				g := reqGame3(t, game.Snovemdomas, game.Orfa)
				if tc.kind == game.AskBreakWith {
					ally(t, g, 2, 1) // a break is asked of an empire allied with Third
				}
				if armed {
					arm(g.Empires[1])
				}
				gn := genomeWith(func(g *Genome) { g.Temper, g.FavourW = float64(tc.temper), fav })
				b := NewAll(g, Config{NoResearch: true, Genome: gn})[1]
				want := fav > 0 && (armed || tc.kind != game.AskWarOn)
				o := game.Offer{From: 0, To: 1, Kind: tc.kind, Day: g.Day, Third: 2}
				for _, from := range []int{0, 1000} {
					b.att[0], b.attOK[0] = from, true
					if got := b.Decide(o); got != want {
						t.Errorf("%v, FavourW %v, armed %v, attitude to the asker %d: %v, want %v", tc.kind, fav, armed, from, got, want)
					}
				}
			}
		}
	}
}

// A request answered through Act: granted once (the war on Third
// declared by the asked, Helped on the asker's ledger), judged on Third
// (Start) while the attitude toward the asker is 1000 (goodwill).
func TestRequestGrantedByAct(t *testing.T) {
	p := Personalities[game.Orfa]
	g := reqGame3(t, game.Snovemdomas, game.Orfa)
	e0, e1 := g.Empires[0], g.Empires[1]
	arm(e1)
	gn := genomeWith(func(g *Genome) { g.Temper, g.FavourW = float64(p.Start-10-p.DeclareWar), 500 })
	bots := NewAll(g, Config{NoResearch: true, Genome: gn})
	e1.Rel(0).Trades += 20 // goodwill: no break, and an attitude toward the asker that would refuse
	if _, err := g.Request(0, 1, game.AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	bots[1].Act()
	if a, _ := bots[1].Attitude(0); a != 1000 {
		t.Fatalf("fixture: attitude to the asker %d", a)
	}
	if e1.StanceWith(2) != game.War || e0.Rel(1).Helped != 1 || e1.StanceWith(0) != game.Alliance {
		t.Errorf("stances %v %v, Helped %d", e1.StanceWith(2), e1.StanceWith(0), e0.Rel(1).Helped)
	}
	given := 0
	for _, r := range bots[1].News() {
		if r.Event.Kind == game.HelpGiven {
			given++
		}
	}
	if given != 2 {
		t.Errorf("%d HelpGiven reports", given)
	}
	for _, o := range g.PendingOffers(1) {
		if o.From == 0 {
			t.Errorf("still pending %+v", o)
		}
	}
}

// A war request on a Third the asked may not fight (game.WarBarred: its
// Friendship toward Third) is refused by Decide, not accepted into
// Answer's error: the offer otherwise vanished with neither HelpGiven
// nor OfferRefused, the asker shown the barred war's text.
func TestDecideBarredWar(t *testing.T) {
	p := Personalities[game.Orfa]
	g := reqGame3(t, game.Snovemdomas, game.Orfa)
	e0, e1 := g.Empires[0], g.Empires[1]
	arm(e1)
	gn := genomeWith(func(g *Genome) { g.Temper, g.FavourW = float64(p.Start-10-p.DeclareWar), 500 })
	b := NewAll(g, Config{NoResearch: true, Genome: gn})[1]
	o := game.Offer{From: 0, To: 1, Kind: game.AskWarOn, Day: g.Day, Third: 2}
	if !b.Decide(o) {
		t.Fatal("fixture: the request refused unbarred")
	}
	e1.Rel(2).Friendship = 2 * game.FriendBar
	if b.Decide(o) {
		t.Error("a barred war request accepted")
	}
	if _, err := g.Request(0, 1, game.AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	b.Act()
	if e1.StanceWith(2) != game.Peace || e0.Rel(1).Refused != 1 {
		t.Errorf("stance %v, Refused %d", e1.StanceWith(2), e0.Rel(1).Refused)
	}
}

// A bot wanting war on an empire it may not declare war on (WarBarred)
// acts as if it did not: no declaration, an exchange offered instead.
func TestBarredWarNotWanted(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b := bots[0]
	arm(b.E)
	b.Cfg.Genome = genomeWith(func(gn *Genome) { gn.Temper = 1000 }) // everyone under DeclareWar
	b.E.Rel(1).Friendship = 2 * game.FriendBar
	tradeable(t, g, 1, 0) // something for me to learn
	b.Act()
	if st := b.E.StanceWith(1); st != game.Peace {
		t.Fatalf("barred: stance %v", st)
	}
	if os := g.PendingOffers(1); len(os) != 1 || !os[0].Kind.Trade() {
		t.Errorf("barred: offers to 1 %+v, want an exchange", os)
	}
}

// requests is the pending requests to empire to, dropped (refused).
func requests(t *testing.T, g *game.Game, to int) []game.Offer {
	t.Helper()
	var out []game.Offer
	for _, o := range g.PendingOffers(to) {
		if _, err := g.Answer(o, false); err != nil {
			t.Fatal(err)
		}
		if o.Kind.Request() {
			out = append(out, o)
		}
	}
	return out
}

// The bot (Dubtaks: Start over BreakAlliance, no break) at War with 2
// asks its ally 1, at Peace with 2, for war on 2 once (and proposes no
// exchange that Act: one offer per pair), then not before round(AskZeal)
// days (each refused).
func TestAskWarZeal(t *testing.T) {
	g := reqGame3(t, game.Dubtaks, game.Orfa)
	atWar(t, g, 0, 2)
	b, zeal := NewAll(g, Config{NoResearch: true})[0], int(math.Round(NeutralDiplomacy().AskZeal))
	b.Act()
	want := game.Offer{From: 0, To: 1, Kind: game.AskWarOn, Day: 0, Third: 2}
	if rs := requests(t, g, 1); len(rs) != 1 || rs[0] != want || b.lastAsk[1] != 0 || b.lastTrade[1] != -1 {
		t.Fatalf("day 0: requests %+v, lastAsk %d, lastTrade %d (the request first, no exchange)", rs, b.lastAsk[1], b.lastTrade[1])
	}
	for g.Day < zeal {
		g.EndDay()
		b.Act()
		if n := len(requests(t, g, 1)); (n == 1) != (g.Day == zeal) || n > 1 {
			t.Errorf("day %d: %d requests", g.Day, n)
		}
	}
	if b.lastAsk[1] != zeal {
		t.Errorf("lastAsk %d", b.lastAsk[1])
	}
}

// The request's other cases: refused by the game (an offer of mine to 1
// pending) it still sets lastAsk; an ally at War with 2 already is not
// asked (lastAsk −1); not allied (Peace), no request.
func TestAskWarCases(t *testing.T) {
	g := reqGame3(t, game.Dubtaks, game.Orfa)
	atWar(t, g, 0, 2)
	if _, err := g.OfferTrade(0, 1, tradeable(t, g, 0, 1)); err != nil {
		t.Fatal(err)
	}
	g.EndDay()
	b := NewAll(g, Config{NoResearch: true})[0]
	b.Act()
	if b.lastAsk[1] != g.Day || len(requests(t, g, 1)) != 0 {
		t.Errorf("refused request: lastAsk %d on day %d", b.lastAsk[1], g.Day)
	}

	g = reqGame3(t, game.Dubtaks, game.Orfa)
	atWar(t, g, 0, 2)
	atWar(t, g, 1, 2)
	b = NewAll(g, Config{NoResearch: true})[0]
	b.Act()
	if b.lastAsk[1] != -1 || len(requests(t, g, 1)) != 0 {
		t.Errorf("ally at war already: lastAsk %d", b.lastAsk[1])
	}

	g = game.NewGame(1, []game.Species{game.Dubtaks, game.Orfa, game.Mebes}, game.Average)
	g.Contact(0, 1)
	g.Contact(1, 2)
	atWar(t, g, 0, 2)
	b = NewAll(g, Config{NoResearch: true})[0]
	b.Act()
	if b.lastAsk[1] != -1 || len(requests(t, g, 1)) != 0 {
		t.Errorf("not allied: lastAsk %d", b.lastAsk[1])
	}
}

// A defamation of j heard (the Marmosians' pheromones, Relation.Defamed)
// lowers the attitude toward j by DefameW on the next Act.
func TestDefamed(t *testing.T) {
	start := Personalities[game.Snovemdomas].Start
	for _, w := range []float64{NeutralDiplomacy().DefameW, 250} {
		g, bots := dipGame(t, game.Snovemdomas, game.Orfa, genomeWith(func(g *Genome) { g.DefameW = w }))
		g.Empires[0].Rel(1).Defamed++
		bots[0].Act()
		if a, _ := bots[0].Attitude(1); a != start-int(w) {
			t.Errorf("DefameW %v: attitude %d, want %d", w, a, start-int(w))
		}
	}
}

// Save/Resume carry lastAsk; any one of the six diplomacy slices short
// resumes all six fresh (lastAsk −1); a bot whose lastAsk is out of step
// starts the diplomacy fresh on its next Act.
func TestDiplomacySaveResumeLot3(t *testing.T) {
	g := reqGame3(t, game.Dubtaks, game.Orfa)
	atWar(t, g, 0, 2)
	b, e := NewAll(g, Config{NoResearch: true})[0], g.Empires[0]
	e.Rel(1).ShipsLost++
	e.Rel(1).Trades++
	b.Act()
	st := b.Save()
	if st.LastAsk[1] != 0 || len(st.LastAsk) != len(g.Empires) {
		t.Fatalf("state %v", st.LastAsk)
	}
	if again := Resume(e, st).Save(); !reflect.DeepEqual(again, st) {
		t.Errorf("round trip:\n%+v\n%+v", again, st)
	}
	fresh := Resume(e, &State{Cfg: st.Cfg}).Save()
	for k, cut := range []func(s *State){
		func(s *State) { s.Grudge = s.Grudge[:1] }, func(s *State) { s.Weary = nil }, func(s *State) { s.Goodwill = s.Goodwill[:2] },
		func(s *State) { s.Seen = nil }, func(s *State) { s.LastTrade = nil }, func(s *State) { s.LastAsk = s.LastAsk[:1] },
	} {
		c := *st
		cut(&c)
		f := Resume(e, &c).Save()
		if !reflect.DeepEqual(f.Grudge, fresh.Grudge) || !reflect.DeepEqual(f.Weary, fresh.Weary) || !reflect.DeepEqual(f.Goodwill, fresh.Goodwill) ||
			!reflect.DeepEqual(f.Seen, fresh.Seen) || !reflect.DeepEqual(f.LastTrade, fresh.LastTrade) || !reflect.DeepEqual(f.LastAsk, fresh.LastAsk) {
			t.Errorf("cut %d: resumed %+v, want fresh %+v", k, f, fresh)
		}
	}
	b.lastAsk = nil
	b.Act()
	if len(b.lastAsk) != len(g.Empires) {
		t.Errorf("out of step: lastAsk %v", b.lastAsk)
	}
}

// An encroachment in the ledger costs GrudgeEncroach of attitude; the
// courted empire's system is no colony target while I mean to keep the
// peace, and is again once I want war.
func TestEncroachment(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b, e := bots[0], g.Empires[0]
	orfa := g.Empires[1].Colonies[0].Star
	e.Rel(1).Encroached++
	b.Act()
	start := Personalities[game.Snovemdomas].Start
	if a, ok := b.Attitude(1); !ok || a != start-int(NeutralDiplomacy().GrudgeEncroach) {
		t.Errorf("attitude %d %v, want %d", a, ok, start-int(NeutralDiplomacy().GrudgeEncroach))
	}
	if !b.courted(orfa) == (start-int(NeutralDiplomacy().GrudgeEncroach) >= b.Thresholds().DeclareWar) {
		t.Error("courted against the attitude")
	}
	b.att[1] = b.Thresholds().DeclareWar
	if !b.courted(orfa) {
		t.Error("the Orfa's home not courted at peace over the war threshold")
	}
	b.Cfg.NoDiplomacy = true
	if b.courted(orfa) {
		t.Error("courted without diplomacy")
	}
	b.Cfg.NoDiplomacy = false
	if b.courted(e.Colonies[0].Star) {
		t.Error("my own home courted")
	}
	b.att[1] = b.Thresholds().DeclareWar - 1 // wanting war
	if b.courted(orfa) {
		t.Error("courted while wanting war")
	}
}

// warTarget: none at peace with the attitude over the war threshold, nor
// wary (under ProposePeace) with no armed ship seen; under it, or at war,
// or wary with one seen, one armed ship more than WarCaution × theirs
// seen, at least 2; wantWarship then asks a warship though the share is met.
func TestWarTarget(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b := bots[0]
	b.Act()
	if n := b.warTarget(); n != 0 {
		t.Errorf("at peace, cordial: target %d", n)
	}
	b.att[1] = b.Thresholds().DeclareWar - 1
	if n := b.warTarget(); n != 2 {
		t.Errorf("wanting war, no ship seen: target %d, want 2", n)
	}
	b.E.Rel(1).Friendship = game.FriendBar // a war the game bars is not wanted
	if n := b.warTarget(); n != 0 {
		t.Errorf("barred, no ship seen: target %d", n)
	}
	b.E.Rel(1).Friendship = 0
	th := b.Thresholds()
	b.att[1] = th.ProposePeace - 1 // wary
	if th.ProposePeace-1 < th.DeclareWar {
		t.Fatal("no wary band")
	}
	if n := b.warTarget(); n != 0 {
		t.Errorf("wary, no ship seen: target %d", n)
	}
	o, home := g.Empires[1], b.E.Colonies[0]
	for range 3 {
		o.Spawn(game.FixtureRaider, home.Star, home.PlanetIdx)
	}
	b.seenArmed = nil // the day's count is cached
	seen := b.armedSeen()[1]
	if seen != 3 {
		t.Fatal("their armed ships unseen")
	}
	want := max(int(math.Ceil(b.dipGenes().WarCaution*float64(seen)))+1, 2)
	if b.warTarget() != want {
		t.Errorf("wary, %d seen: target %d, want %d", seen, b.warTarget(), want)
	}
	b.att[1] = th.ProposePeace // cordial
	if n := b.warTarget(); n != 0 {
		t.Errorf("cordial, %d seen: target %d", seen, n)
	}
	b.att[1] = th.DeclareWar - 1
	if b.warTarget() != want {
		t.Errorf("wanting war, %d seen: target %d, want %d", seen, b.warTarget(), want)
	}
	b.cg, b.contact = b.genes(), true // Act's assess reads contact from the map
	if col := b.E.Colonies[0]; !b.wantWarship(col) {
		t.Error("no warship wanted for the war")
	}
}

// warReady counts their armed ships near my systems only: three at my
// home hold me back, the same far away do not; in a rage (under
// DeclareWar − Rage) one armed ship of mine is enough.
func TestWarReadyNearAndRage(t *testing.T) {
	g, bots := dipGame(t, game.Snovemdomas, game.Orfa, nil)
	b, o := bots[0], g.Empires[1]
	arm(b.E)
	b.Act()
	home := b.E.Colonies[0]
	for range 3 {
		o.Spawn(game.FixtureRaider, home.Star, home.PlanetIdx)
	}
	b.seenArmed = nil
	th := b.Thresholds()
	b.att[1] = th.DeclareWar - 1
	if b.armedSeen()[1] != 3 || b.warReady(1) {
		t.Errorf("three Raiders at my home: seen %d, ready %v", b.armedSeen()[1], b.warReady(1))
	}
	b.att[1] = th.DeclareWar - int(NeutralDiplomacy().Rage) - 1
	if !b.warReady(1) {
		t.Error("in a rage: not ready")
	}
	b.att[1] = th.DeclareWar - 1
	far := o.Colonies[0]
	for _, s := range o.Ships {
		s.Star, s.Orbit, s.Pos = far.Star, far.PlanetIdx, o.Planet(far.Star, far.PlanetIdx).Pos
	}
	b.seenArmed = nil
	if n := b.armedSeen()[1]; n != 0 || !b.warReady(1) {
		t.Errorf("the Raiders at their home: seen %d near me, ready %v", n, b.warReady(1))
	}
}
