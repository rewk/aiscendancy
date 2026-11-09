package game

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/journal"
)

// drain is the entries buf holds, buf emptied.
func drain(t *testing.T, buf *bytes.Buffer) []journal.Entry {
	t.Helper()
	var out []journal.Entry
	for _, l := range strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n") {
		if l == "" {
			continue
		}
		var en journal.Entry
		if err := json.Unmarshal([]byte(l), &en); err != nil {
			t.Fatalf("%q: %v", l, err)
		}
		out = append(out, en)
	}
	buf.Reset()
	return out
}

// one is the single entry buf holds after a call accepted (err nil),
// checked for its act and empire; buf emptied.
func one(t *testing.T, buf *bytes.Buffer, what, act string, empire int, err error) journal.Entry {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	ens := drain(t, buf)
	if len(ens) != 1 {
		t.Fatalf("%s: %d entries, want 1: %+v", what, len(ens), ens)
	}
	if en := ens[0]; en.Act != act || en.Empire != empire {
		t.Fatalf("%s: act %q empire %d, want %q %d: %+v", what, en.Act, en.Empire, act, empire, en)
	}
	return ens[0]
}

// none checks a refused call (or the model's own resolution) wrote
// nothing.
func none(t *testing.T, buf *bytes.Buffer, what string) {
	t.Helper()
	if ens := drain(t, buf); len(ens) != 0 {
		t.Fatalf("%s: %d entries, want none: %+v", what, len(ens), ens)
	}
}

// Every decision entry point records exactly one entry when it accepts,
// with the acting empire (empire 1, a bot's, as often as possible) and
// its key fields; a refusal records nothing; a colony founded (FoundAt)
// and one captured (Invade) record their later projects under their new
// owner; the day's own resolution (EndDay: an offer expiring, ships
// stepping) and a granted request's act record nothing.
func TestJournal(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Chamachies, Mebes}, Average)
	atWar(t, g, 0, 1) // before the journal: setup
	var buf bytes.Buffer
	g.SetJournal(journal.New(&buf))
	e0, e1 := g.Empires[0], g.Empires[1]
	r := e1.Research

	// Research and the ability, on the fresh tree.
	av := r.Available()
	if en := one(t, &buf, "Study", "study", 1, e1.Study(av[0])); en.Tech != av[0].String() || en.Day != 0 {
		t.Errorf("Study: %+v", en)
	}
	if err := e1.Study(av[0]); err != nil || len(drain(t, &buf)) != 0 { // the topic again: no decision
		t.Errorf("Study of the topic again: %v, entries written", err)
	}
	locked := NoTech
	for tc := range NumTechs {
		if !r.CanStudy(tc) && !r.Known[tc] {
			locked = tc
			break
		}
	}
	if e1.Study(locked) == nil {
		t.Fatal("a locked tech studied")
	}
	none(t, &buf, "Study refused")
	charge(g, 1)
	if _, err := g.UseAbility(1); err != nil {
		t.Fatal(err)
	}
	if en := one(t, &buf, "UseAbility", "ability", 1, nil); en.Other != -1 {
		t.Errorf("UseAbility: %+v", en)
	}
	if _, err := g.UseAbility(1); err == nil {
		t.Fatal("ability used twice")
	}
	none(t, &buf, "UseAbility refused")

	for tc := range NumTechs {
		r.Grant(tc)
	}
	home := e1.Colonies[0]
	p := home.Planet
	home.Pop = home.Busy() + 4

	// The colony's projects.
	at, ok := p.BestCell(Agriplot, home.Traits())
	if !ok {
		t.Fatal("no square")
	}
	if en := one(t, &buf, "Start", "start", 1, home.Start(r, Agriplot, at)); en.Colony != p.Name || en.Item != "Agriplot" || en.Cell != at.String() {
		t.Errorf("Start: %+v", en)
	}
	if home.Start(r, Agriplot, Cell{-9, -9}) == nil {
		t.Fatal("start off the grid")
	}
	none(t, &buf, "Start refused")
	home.Cancel()
	if en := one(t, &buf, "Cancel", "cancel", 1, nil); en.Item != "Agriplot" || en.Colony != p.Name {
		t.Errorf("Cancel: %+v", en)
	}
	home.Cancel()
	none(t, &buf, "Cancel idle")
	if err := p.BuildOrbital(0, Shipyard); err != nil {
		t.Fatal(err)
	}
	if err := p.BuildOrbital(1, OrbitalDocks); err != nil {
		t.Fatal(err)
	}
	if en := one(t, &buf, "StartOrbital", "start", 1, home.StartOrbital(r, Shipyard, 2)); en.Item != "Shipyard" || en.Slot != 2 || en.Cell != "" {
		t.Errorf("StartOrbital: %+v", en)
	}
	home.Cancel()
	one(t, &buf, "Cancel orbital", "cancel", 1, nil)
	if en := one(t, &buf, "StartProject", "start", 1, home.StartProject(r, EndlessParty)); en.Item != EndlessParty.String() || en.Slot != -1 {
		t.Errorf("StartProject: %+v", en)
	}
	home.Cancel()
	one(t, &buf, "Cancel ongoing", "cancel", 1, nil)
	f, ok := p.BestCell(Factory, home.Traits())
	if !ok || p.Build(f, Factory) != nil {
		t.Fatal("no Factory")
	}
	if en := one(t, &buf, "Abandon", "demolish", 1, home.Abandon(f)); en.Item != "Factory" || en.Cell != f.String() {
		t.Errorf("Abandon: %+v", en)
	}
	if home.Abandon(f) == nil {
		t.Fatal("an empty square abandoned")
	}
	none(t, &buf, "Abandon refused")
	if err := p.BuildOrbital(3, Shipyard); err != nil {
		t.Fatal(err)
	}
	if en := one(t, &buf, "AbandonOrbital", "demolish", 1, home.AbandonOrbital(3)); en.Item != "Shipyard" || en.Slot != 3 {
		t.Errorf("AbandonOrbital: %+v", en)
	}
	if home.AbandonOrbital(3) == nil {
		t.Fatal("an empty slot abandoned")
	}
	none(t, &buf, "AbandonOrbital refused")

	// Ships: the refit, the build.
	d, ok := ColonyShip(r, false)
	if !ok {
		t.Fatal("no colony ship")
	}
	s := e1.Spawn(d, home.Star, home.PlanetIdx)
	none(t, &buf, "Spawn")
	if en := one(t, &buf, "Refit", "refit", 1, e1.Refit(home, s, withGen(s))); en.Ship != s.Name || en.Colony != p.Name || en.Design != d.Name {
		t.Errorf("Refit: %+v", en)
	}
	home.Cancel()
	if en := one(t, &buf, "Cancel refit", "cancel", 1, nil); en.Item != OrbitalDocks.String() || en.Ship != s.Name {
		t.Errorf("Cancel refit: %+v", en)
	}
	if en := one(t, &buf, "BuildShip", "build", 1, e1.BuildShip(home, d)); en.Colony != p.Name || en.Design != d.Name {
		t.Errorf("BuildShip: %+v", en)
	}
	home.Cancel()
	one(t, &buf, "Cancel ship", "cancel", 1, nil)

	// Orders, orbit, shields.
	if e0.Order(s, Order{Kind: GoToPlanet, Planet: home.PlanetIdx}) == nil {
		t.Fatal("an order to another empire's ship")
	}
	none(t, &buf, "Order refused")
	if en := one(t, &buf, "Order", "order", 1, e1.Order(s, Order{Kind: GoToPlanet, Planet: 0})); en.Ship != s.Name || en.Kind != "planet" ||
		en.Star != home.Star || en.Planet != 0 {
		t.Errorf("Order: %+v", en)
	}
	if en := one(t, &buf, "LeaveOrbit", "leave", 1, e1.LeaveOrbit(s, nil)); en.Ship != s.Name || en.Planet != home.PlanetIdx {
		t.Errorf("LeaveOrbit: %+v", en)
	}
	if e1.LeaveOrbit(s, nil) == nil {
		t.Fatal("left orbit twice")
	}
	none(t, &buf, "LeaveOrbit refused")
	if en := one(t, &buf, "SetShields", "shields", 1, e1.SetShields(s, false)); en.On || en.Ship != s.Name || s.Shields {
		t.Errorf("SetShields: %+v, shields %v", en, s.Shields)
	}
	if err := e1.SetShields(s, false); err != nil {
		t.Errorf("SetShields unchanged: %v", err)
	}
	none(t, &buf, "SetShields unchanged")
	if err := e0.SetShields(s, true); err == nil || s.Shields {
		t.Errorf("SetShields on another empire's ship: %v, shields %v", err, s.Shields)
	}
	none(t, &buf, "SetShields on another empire's ship")
	wrapped := e1.Spawn(fightFixture, home.Star, home.PlanetIdx)
	if err := e1.SetShields(wrapped, false); err != nil {
		t.Fatal(err)
	}
	drain(t, &buf)
	wrapped.Power = fightFixture.ShieldDraw() - 1
	if err := e1.SetShields(wrapped, true); err == nil || wrapped.Shields {
		t.Errorf("SetShields short of the draw: %v, shields %v", err, wrapped.Shields)
	}
	none(t, &buf, "SetShields refused")

	// Arm and a special, in space at empire 1's home.
	gun := e1.Spawn(foeShip, home.Star, home.PlanetIdx)
	foe := e0.Spawn(hulk, home.Star, home.PlanetIdx)
	gun.Shields, foe.Shields = false, false
	inSpaceAt(gun, gun.Pos)
	inSpaceAt(foe, gun.Pos)
	gun.Refill()
	sg, ok := g.SightingOf(e1, foe)
	if !ok {
		t.Fatal("foe unseen")
	}
	k := part(foeShip, MassBarrageGun)
	if en := one(t, &buf, "Arm", "arm", 1, e1.Arm(gun, k, sg)); en.Ship != gun.Name || en.Part != k || en.Target != foe.Name || en.Other != 0 {
		t.Errorf("Arm: %+v", en)
	}
	if e1.Arm(gun, k, sg) == nil {
		t.Fatal("armed twice")
	}
	none(t, &buf, "Arm refused")
	hd := Design{Name: "Fuel", Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, Hyperfuel, MassBarrageGun}}
	fuel := e1.Spawn(hd, home.Star, home.PlanetIdx)
	inSpaceAt(fuel, fuel.Pos)
	fuel.Weapon = part(hd, MassBarrageGun)
	k = part(hd, Hyperfuel)
	_, err := e1.UseSpecial(fuel, k, Target{})
	if en := one(t, &buf, "UseSpecial", "special", 1, err); en.Ship != fuel.Name || en.Part != k || en.Item != Gizmos[Hyperfuel].Name {
		t.Errorf("UseSpecial: %+v", en)
	}
	if _, err := e1.UseSpecial(fuel, k, Target{}); err == nil {
		t.Fatal("a spent Hyperfuel used")
	}
	none(t, &buf, "UseSpecial refused")

	// A colony founded, then captured: each records under its owner.
	type spot struct {
		star, pl int
		cell     Cell
	}
	var free []spot // two free planets of two stars no empire holds
	for st := range g.Cluster.Stars {
		if len(free) == 2 || st == home.Star {
			continue
		}
		for i, q := range g.Cluster.Stars[st].System.Planets {
			if c, ok := q.BestCell(ColonyBase, e1.Species.Traits()); q.Owner < 0 && ok {
				free = append(free, spot{st, i, c})
				break
			}
		}
	}
	if len(free) < 2 {
		t.Fatal("no free planets")
	}
	star, pl, cell := free[0].star, free[0].pl, free[0].cell
	settler := e1.Spawn(d, star, pl)
	if en := one(t, &buf, "FoundAt", "found", 1, e1.FoundAt(settler, cell)); en.Ship != settler.Name || en.Cell != cell.String() ||
		en.Star != star || en.Planet != pl || en.Colony != e1.Planet(star, pl).Name {
		t.Errorf("FoundAt: %+v", en)
	}
	if e1.FoundAt(settler, cell) == nil {
		t.Fatal("founded twice")
	}
	none(t, &buf, "FoundAt refused")
	col := e1.Colony(star, pl)
	col.Pop = col.Busy() + 2
	at, ok = col.Planet.BestCell(Agriplot, e1.Species.Traits())
	if !ok {
		t.Fatal("no square on the new colony")
	}
	if en := one(t, &buf, "Start on a founded colony", "start", 1, col.Start(r, Agriplot, at)); en.Colony != col.Planet.Name || en.Day != e1.Day {
		t.Errorf("Start on a founded colony: %+v", en)
	}
	if en := one(t, &buf, "Abandon ship", "abandon", 1, e1.Abandon(settler)); en.Ship != settler.Name {
		t.Errorf("Abandon ship: %+v", en)
	}
	hab := e1.Spawn(Design{Name: "Hab", Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, SpaceHabitat}}, star, pl)
	col.Pop = col.Busy() + 2
	if en := one(t, &buf, "Load", "load", 1, e1.Load(hab)); en.Ship != hab.Name || en.Colony != col.Planet.Name || en.Star != star || en.Planet != pl {
		t.Errorf("Load: %+v", en)
	}
	if en := one(t, &buf, "Unload", "unload", 1, e1.Unload(hab)); en.Ship != hab.Name || en.Colony != col.Planet.Name || en.Star != star || en.Planet != pl {
		t.Errorf("Unload: %+v", en)
	}
	if e1.Unload(hab) == nil {
		t.Fatal("unloaded an empty habitat")
	}
	none(t, &buf, "Unload refused")
	one(t, &buf, "Abandon the habitat", "abandon", 1, e1.Abandon(hab)) // it would guard the orbit
	inv := e0.Spawn(invader(2), star, pl)
	if en := one(t, &buf, "Order invade", "order", 0, e0.Order(inv, Order{Kind: Invade})); en.Kind != "invade" || en.Star != star || en.Planet != pl {
		t.Errorf("Order invade: %+v", en)
	}
	if _, err := g.Invade(e0, inv); err != nil || e0.Colony(star, pl) != col {
		t.Fatalf("invasion: %v", err)
	}
	none(t, &buf, "Invade (the resolution, its project dropped)")
	at, _ = col.Planet.BestCell(Agriplot, e0.Species.Traits())
	if en := one(t, &buf, "Start on a captured colony", "start", 0, col.Start(e0.Research, Agriplot, at)); en.Colony != col.Planet.Name {
		t.Errorf("Start on a captured colony: %+v", en)
	}
	col.Cancel()
	one(t, &buf, "Cancel on a captured colony", "cancel", 0, nil)

	// Diplomacy.
	g.Contact(1, 2)
	g.Contact(0, 2)
	none(t, &buf, "Contact")
	_, err = g.DeclareWar(1, 2)
	if en := one(t, &buf, "DeclareWar", "war", 1, err); en.Other != 2 {
		t.Errorf("DeclareWar: %+v", en)
	}
	if _, err := g.DeclareWar(1, 2); err == nil {
		t.Fatal("war declared twice")
	}
	none(t, &buf, "DeclareWar refused")
	_, err = g.OfferPeace(1, 2)
	if en := one(t, &buf, "OfferPeace", "offer", 1, err); en.Other != 2 || en.Kind != "peace" {
		t.Errorf("OfferPeace: %+v", en)
	}
	_, err = g.Answer(g.PendingOffers(2)[0], true)
	if en := one(t, &buf, "Answer", "answer", 2, err); en.Other != 1 || en.Kind != "peace" || !en.Accept {
		t.Errorf("Answer: %+v", en)
	}
	_, err = g.OfferAlliance(2, 1)
	if en := one(t, &buf, "OfferAlliance", "offer", 2, err); en.Other != 1 || en.Kind != "alliance" {
		t.Errorf("OfferAlliance: %+v", en)
	}
	_, err = g.Answer(g.PendingOffers(1)[0], true)
	one(t, &buf, "Answer alliance", "answer", 1, err)
	_, err = g.OfferTrade(0, 2, TradeStars)
	if en := one(t, &buf, "OfferTrade", "offer", 0, err); en.Other != 2 || en.Kind != "stars" {
		t.Errorf("OfferTrade: %+v", en)
	}
	if _, err := g.OfferTrade(0, 2, TradeLanes); err == nil {
		t.Fatal("two offers pending 0 to 2")
	}
	none(t, &buf, "OfferTrade refused")
	_, err = g.Answer(g.PendingOffers(2)[0], false)
	if en := one(t, &buf, "Answer refused", "answer", 2, err); en.Accept || en.Other != 0 {
		t.Errorf("Answer refusing: %+v", en)
	}
	_, err = g.OfferTrade(2, 0, TradeStars)
	one(t, &buf, "OfferTrade left to expire", "offer", 2, err)

	// A Colonise order (founded at the close) and a refit whose ship is
	// lost (cancelled at the close).
	colonist := e1.Spawn(d, free[1].star, free[1].pl)
	if en := one(t, &buf, "Order colonise", "order", 1, e1.Order(colonist, Order{Kind: Colonise})); en.Kind != "colonise" || en.Planet != free[1].pl {
		t.Errorf("Order colonise: %+v", en)
	}
	lost := e1.Spawn(d, home.Star, home.PlanetIdx)
	one(t, &buf, "Refit a ship to lose", "refit", 1, e1.Refit(home, lost, withGen(lost)))
	destroyIn([]*Empire{e1}, lost)

	// The days pass: the colony founded, the refit cancelled, the offer
	// expired, the ships stepped: nothing recorded.
	for range OfferExpiry + 1 {
		g.EndDay()
	}
	if len(g.PendingOffers(0)) != 0 || home.Project != nil {
		t.Fatalf("the offer did not expire (%v) or the lost refit run on (%+v)", g.PendingOffers(0), home.Project)
	}
	none(t, &buf, "EndDay")
	settled := e1.Colony(free[1].star, free[1].pl)
	if settled == nil {
		t.Fatal("the Colonise order founded nothing")
	}
	settled.Pop = settled.Busy() + 2
	at, _ = settled.Planet.BestCell(Agriplot, e1.Species.Traits())
	if en := one(t, &buf, "Start on a colony founded at the close", "start", 1, settled.Start(r, Agriplot, at)); en.Day != g.Day {
		t.Errorf("Start on a colony founded at the close: %+v", en)
	}

	// A request granted: the answer only, not To's act.
	_, err = g.Request(1, 2, AskWarOn, 0)
	if en := one(t, &buf, "Request", "offer", 1, err); en.Kind != AskWarOn.String() || en.Other != 2 || en.Target != "empire 0" || en.Day != g.Day {
		t.Errorf("Request: %+v", en)
	}
	_, err = g.Answer(g.PendingOffers(2)[0], true)
	one(t, &buf, "Answer a request", "answer", 2, err)
	if g.Empires[2].StanceWith(0) != War {
		t.Fatal("the request's war not declared")
	}
	_, err = g.BreakAlliance(1, 2)
	if en := one(t, &buf, "BreakAlliance", "break", 1, err); en.Other != 2 {
		t.Errorf("BreakAlliance: %+v", en)
	}
	if _, err := g.BreakAlliance(1, 2); err == nil {
		t.Fatal("broken twice")
	}
	none(t, &buf, "BreakAlliance refused")
}

// The fixtures (mksave sets the journal before them) are setup, not
// actions: their war records nothing.
func TestJournalFixtures(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	var buf bytes.Buffer
	g.SetJournal(journal.New(&buf))
	if err := SpawnFixtures(g, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := SpawnArena(g, []Design{hulk}, []Design{hulk}); err != nil {
		t.Fatal(err)
	}
	if g.Empires[0].StanceWith(1) != War {
		t.Fatal("no fixture war")
	}
	none(t, &buf, "fixtures")
}

// The planets' shots are decisions too: a ship
// armed at a planet (ArmAtPlanet) and a colony's orbital weapon armed at
// a ship (ArmPlanet) record one "arm" each; a refusal nothing.
func TestJournalPlanetShots(t *testing.T) {
	g, sh, home := siege(t)
	var buf bytes.Buffer
	g.SetJournal(journal.New(&buf))
	e0, e1 := g.Empires[0], g.Empires[1]
	p := home.Planet
	const slot = 4
	if err := p.BuildOrbital(slot, LongRangeOrbitalWhopper); err != nil { // something to hit, a gun to fire
		t.Fatal(err)
	}
	sh.Refill()
	gun := slices.Index(sh.Design.Parts, Nanomanipulator)
	if en := one(t, &buf, "ArmAtPlanet", "arm", 1, e1.ArmAtPlanet(sh, gun, home.PlanetIdx)); en.Ship != sh.Name ||
		en.Part != gun || en.Star != home.Star || en.Planet != home.PlanetIdx || en.Other != 0 {
		t.Errorf("ArmAtPlanet: %+v", en)
	}
	if e1.ArmAtPlanet(sh, 0, home.PlanetIdx) == nil { // a generator
		t.Fatal("a generator armed")
	}
	none(t, &buf, "ArmAtPlanet refused")
	// Disarming writes "disarm" once; disarming what holds no shot nothing.
	e1.Disarm(sh)
	if en := one(t, &buf, "Disarm", "disarm", 1, nil); en.Ship != sh.Name || len(sh.Armed) != 0 {
		t.Errorf("Disarm: %+v, armed %d", en, len(sh.Armed))
	}
	e1.Disarm(sh)
	none(t, &buf, "Disarm of nothing")

	inSpaceAt(sh, Vec3{X: p.Pos.X + 0.1, Y: p.Pos.Y, Z: p.Pos.Z})
	sg, _ := g.SightingOf(e0, sh)
	if en := one(t, &buf, "ArmPlanet", "arm", 0, e0.ArmPlanet(home, slot, sg)); en.Colony != p.Name ||
		en.Slot != slot || en.Star != home.Star || en.Planet != home.PlanetIdx || en.Target != sh.Name || en.Other != 1 {
		t.Errorf("ArmPlanet: %+v", en)
	}
	if e0.ArmPlanet(home, slot, sg) == nil {
		t.Fatal("armed twice")
	}
	none(t, &buf, "ArmPlanet refused")
	// The colony's shots likewise.
	e0.DisarmPlanet(home)
	if en := one(t, &buf, "DisarmPlanet", "disarm", 0, nil); en.Colony != p.Name || len(home.Armed) != 0 {
		t.Errorf("DisarmPlanet: %+v, armed %d", en, len(home.Armed))
	}
	e0.DisarmPlanet(home)
	none(t, &buf, "DisarmPlanet of nothing")
}

// A colony's orbital weapon armed at a planet (ArmPlanetAt) records one
// "arm": the colony, the slot, the target planet's index and name, its
// owner; a refusal (CanArmPlanetAt's check too) nothing.
func TestJournalPlanetAtPlanet(t *testing.T) {
	g, home, foe, lr, _ := p2pFixture(t)
	if err := foe.Planet.BuildOrbital(0, OrbitalShield); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	g.SetJournal(journal.New(&buf))
	e0 := g.Empires[0]
	if e0.CanArmPlanetAt(home, lr, foe.PlanetIdx) != nil {
		t.Fatal("fixture: refused")
	}
	none(t, &buf, "CanArmPlanetAt")
	if en := one(t, &buf, "ArmPlanetAt", "arm", 0, e0.ArmPlanetAt(home, lr, foe.PlanetIdx)); en.Colony != home.Planet.Name ||
		en.Slot != lr || en.Star != home.Star || en.Planet != foe.PlanetIdx || en.Target != foe.Planet.Name || en.Other != 1 {
		t.Errorf("ArmPlanetAt: %+v", en)
	}
	if e0.ArmPlanetAt(home, lr, foe.PlanetIdx) == nil {
		t.Fatal("armed twice")
	}
	none(t, &buf, "ArmPlanetAt refused")
}
