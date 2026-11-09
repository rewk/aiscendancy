package game

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// war puts a and b at War (Contact, DeclareWar), the reports dropped; nil
// when they already are.
func war(g *Game, a, b int) error {
	g.Contact(a, b)
	if g.Empires[a].StanceWith(b) == War {
		return nil
	}
	_, err := g.DeclareWar(a, b)
	return err
}

// atWar is war for a test: a fight staged on a NewGame needs its pair
// Hostile.
func atWar(t testing.TB, g *Game, a, b int) {
	t.Helper()
	if err := war(g, a, b); err != nil {
		t.Fatalf("war %d-%d: %v", a, b, err)
	}
}

// allAtWar puts every pair of g's empires at War: the rule before
// diplomacy, every other empire Hostile.
func allAtWar(t testing.TB, g *Game) {
	t.Helper()
	for a := range g.Empires {
		for b := a + 1; b < len(g.Empires); b++ {
			atWar(t, g, a, b)
		}
	}
}

// kinds lists evs' events of kind k.
func kinds(evs []Event, k EventKind) []Event {
	var out []Event
	for _, e := range evs {
		if e.Kind == k {
			out = append(out, e)
		}
	}
	return out
}

// dipGame is a three-empire game (Snovemdomas, Orfa, Mebes): nobody met.
func dipGame(t *testing.T) *Game {
	t.Helper()
	g := NewGame(1, []Species{Snovemdomas, Orfa, Mebes}, Average)
	for a, e := range g.Empires {
		for b := range g.Empires {
			if r := e.Rel(b); r.Stance != Unknown || r.Met != -1 || r.LastOffer != -1 {
				t.Fatalf("%d about %d at start: %+v", a, b, *r)
			}
		}
	}
	return g
}

// Two empires' ships at one star meet at the close: Peace both ways, Met
// the day, a Contacted each naming the other; the third, elsewhere,
// meets nobody.
func TestContactAtStar(t *testing.T) {
	g := dipGame(t)
	home := g.Empires[0].Colonies[0]
	if g.Empires[2].Present(home.Star) {
		t.Fatal("empire 2 at empire 0's home: the fixture proves nothing")
	}
	g.Empires[1].Spawn(orbitShip, home.Star, home.PlanetIdx)
	evs := g.EndDay()
	for a, b := range []int{1, 0} {
		r := g.Empires[a].Rel(b)
		if r.Stance != Peace || r.Met != g.Day {
			t.Errorf("%d about %d: %+v, want Peace met day %d", a, b, *r, g.Day)
		}
		c := kinds(evs[a], Contacted)
		if len(c) != 1 || c[0].Other != b || c[0].Star != -1 || c[0].Planet != -1 ||
			c[0].Text != "Met the "+g.Empires[b].Species.String() {
			t.Errorf("%d's contacts %+v", a, c)
		}
	}
	for b := range 2 {
		if st := g.Empires[2].StanceWith(b); st != Unknown {
			t.Errorf("empire 2 with %d: %v, never together", b, st)
		}
	}
	if len(kinds(evs[2], Contacted)) != 0 {
		t.Error("empire 2 contacted")
	}
	met := g.Day
	if evs := g.EndDay(); len(kinds(evs[0], Contacted)) != 0 || g.Empires[0].Rel(1).Met != met {
		t.Error("a second contact")
	}
}

// Hostile is War only; DeclareWar refuses the unmet and the warring,
// counts WarsOn and drops the offers either way.
func TestDeclareWar(t *testing.T) {
	g := dipGame(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	if _, err := g.DeclareWar(0, 1); err == nil || !strings.Contains(err.Error(), "never met") {
		t.Errorf("unmet war: %v", err)
	}
	g.Contact(0, 1)
	if g.Hostile(e0, e1) || g.Hostile(e1, e0) {
		t.Error("hostile at peace")
	}
	g.Offers = []Offer{{From: 1, To: 0, Kind: PeaceOffer, Third: -1}, {From: 2, To: 0, Kind: PeaceOffer, Third: -1}, {From: 0, To: 1, Kind: PeaceOffer, Third: -1}}
	reps, err := g.DeclareWar(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Hostile(e0, e1) || !g.Hostile(e1, e0) || e0.StanceWith(1) != War || e1.StanceWith(0) != War {
		t.Error("not hostile at war")
	}
	if e1.Rel(0).WarsOn != 1 || e0.Rel(1).WarsOn != 0 {
		t.Errorf("WarsOn %d (victim), %d (declarer)", e1.Rel(0).WarsOn, e0.Rel(1).WarsOn)
	}
	if want := []Offer{{From: 2, To: 0, Kind: PeaceOffer, Third: -1}}; !slices.Equal(g.Offers, want) {
		t.Errorf("offers %+v, want %+v", g.Offers, want)
	}
	want := []Report{
		{0, dipEvent(WarDeclared, 1, "War declared on Orfa")},
		{1, dipEvent(WarDeclared, 0, "War declared by Snovemdomas")},
	}
	if !slices.Equal(reps, want) {
		t.Errorf("reports %+v, want %+v", reps, want)
	}
	if _, err := g.DeclareWar(1, 0); err == nil || !strings.Contains(err.Error(), "not at peace") {
		t.Errorf("war at war: %v", err)
	}
}

// OfferPeace: at war only, one pending per direction, then OfferCooldown
// days from the last offer; Answer accept is Peace, refuse counts Refused.
func TestOfferPeace(t *testing.T) {
	g := dipGame(t)
	if _, err := g.OfferPeace(0, 1); err == nil {
		t.Error("offer to the unmet")
	}
	g.Contact(0, 1)
	if _, err := g.OfferPeace(0, 1); err == nil {
		t.Error("offer at peace")
	}
	atWar(t, g, 0, 1)
	reps, err := g.OfferPeace(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	o := Offer{From: 0, To: 1, Kind: PeaceOffer, Day: g.Day, Third: -1}
	if !slices.Equal(g.PendingOffers(1), []Offer{o}) || len(g.PendingOffers(0)) != 0 || g.Empires[0].Rel(1).LastOffer != g.Day {
		t.Errorf("pending %+v, LastOffer %d", g.Offers, g.Empires[0].Rel(1).LastOffer)
	}
	if len(reps) != 2 || reps[0].Empire != 1 || reps[0].Event.Kind != PeaceOffered || reps[0].Event.Text != "Snovemdomas offers peace" ||
		reps[1].Empire != 0 || reps[1].Event.Text != "Peace offered to Orfa" {
		t.Errorf("reports %+v", reps)
	}
	if _, err := g.OfferPeace(0, 1); err == nil || !strings.Contains(err.Error(), "already offered") {
		t.Errorf("second pending offer: %v", err)
	}
	if reps, err := g.Answer(o, false); err != nil || len(reps) != 2 || reps[0].Event.Kind != OfferRefused ||
		reps[0].Event.Text != "Orfa refuses peace" || reps[1].Event.Text != "Refused Snovemdomas: peace" {
		t.Fatalf("refuse: %+v, %v", reps, err)
	}
	if g.Empires[0].Rel(1).Refused != 1 || g.Empires[0].StanceWith(1) != War || len(g.Offers) != 0 {
		t.Errorf("after refusal: %+v, offers %+v", *g.Empires[0].Rel(1), g.Offers)
	}
	if _, err := g.Answer(o, true); err == nil {
		t.Error("answered an offer no longer pending")
	}
	for range OfferCooldown - 1 {
		g.Day++
		if _, err := g.OfferPeace(0, 1); err == nil || !strings.Contains(err.Error(), "days ago") {
			t.Fatalf("day %d after the offer: %v", g.Day-o.Day, err)
		}
	}
	g.Day++
	if _, err := g.OfferPeace(0, 1); err != nil {
		t.Fatalf("after the cooldown: %v", err)
	}
	o = g.PendingOffers(1)[0]
	reps, err = g.Answer(o, true)
	if err != nil {
		t.Fatal(err)
	}
	if g.Empires[0].StanceWith(1) != Peace || g.Empires[1].StanceWith(0) != Peace || g.Hostile(g.Empires[0], g.Empires[1]) {
		t.Error("accepted, not at peace")
	}
	if len(reps) != 2 || reps[0].Event.Kind != PeaceMade || reps[0].Event.Text != "Peace with Orfa" ||
		reps[1].Event.Text != "Peace with Snovemdomas" {
		t.Errorf("peace reports %+v", reps)
	}
}

// Two crossed peace offers (0 to 1, 1 to 0): accepting one drops the
// other, so it neither expires as a refusal at peace (a Refused in the
// ledger, a grudge for the bots) nor makes a second PeaceMade; the
// offers between other pairs stay.
func TestAcceptDropsCrossedOffer(t *testing.T) {
	g := dipGame(t)
	atWar(t, g, 0, 1)
	atWar(t, g, 0, 2)
	for _, p := range [][2]int{{0, 1}, {1, 0}, {2, 0}} {
		if _, err := g.OfferPeace(p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := g.Answer(g.PendingOffers(1)[0], true); err != nil {
		t.Fatal(err)
	}
	if want := []Offer{{From: 2, To: 0, Kind: PeaceOffer, Day: g.Day, Third: -1}}; !slices.Equal(g.Offers, want) {
		t.Errorf("offers %+v, want %+v", g.Offers, want)
	}
	for range OfferExpiry {
		g.EndDay()
	}
	if r := g.Empires[1].Rel(0); r.Refused != 0 || r.Stance != Peace {
		t.Errorf("1 about 0 after the expiry: %+v", *r)
	}
}

// An offer unanswered expires at EndDay on day Day+OfferExpiry, refused.
func TestOfferExpiry(t *testing.T) {
	g := dipGame(t)
	atWar(t, g, 0, 1)
	if _, err := g.OfferPeace(0, 1); err != nil {
		t.Fatal(err)
	}
	made := g.Day
	for g.Day+1 < made+OfferExpiry {
		if evs := g.EndDay(); len(g.Offers) != 1 || len(kinds(evs[0], OfferRefused)) != 0 {
			t.Fatalf("day %d: offers %+v", g.Day, g.Offers)
		}
	}
	evs := g.EndDay()
	if g.Day != made+OfferExpiry || len(g.Offers) != 0 || g.Empires[0].Rel(1).Refused != 1 {
		t.Errorf("day %d: offers %+v, refused %d", g.Day, g.Offers, g.Empires[0].Rel(1).Refused)
	}
	if len(kinds(evs[0], OfferRefused)) != 1 || len(kinds(evs[1], OfferRefused)) != 1 {
		t.Errorf("expiry events %+v / %+v", evs[0], evs[1])
	}
}

// The ledger: WarDays every day at war; at peace, an armed ship of b at a
// star where a holds a colony is one Intrusion a day, an unarmed one none;
// none at war.
func TestLedgerDays(t *testing.T) {
	g := dipGame(t)
	home := g.Empires[0].Colonies[0]
	g.Empires[1].Spawn(siegeGun, home.Star, home.PlanetIdx)
	g.Empires[1].Spawn(orbitShip, home.Star, home.PlanetIdx)
	if siegeGun.Weapons() == 0 || orbitShip.Weapons() != 0 {
		t.Fatal("fixture: siegeGun armed, orbitShip not")
	}
	const days = 3
	for range days {
		g.EndDay() // day 1 meets (Contact) before the ledger counts
	}
	r := g.Empires[0].Rel(1)
	if r.Stance != Peace || r.Intrusions != days || r.WarDays != 0 {
		t.Errorf("at peace: %+v, want %d intrusions", *r, days)
	}
	if o := g.Empires[1].Rel(0); o.Intrusions != 0 {
		t.Errorf("empire 1 counts %d intrusions, holds no colony there", o.Intrusions)
	}
	// Empire 1 settles a planet of the same system: its armed ship is at
	// home there, no intrusion from now on.
	sys := g.Cluster.Stars[home.Star].System
	if len(sys.Planets) < 2 {
		t.Fatal("fixture: a one-planet home system")
	}
	// Empire 0 keeps a system of its own: its only one shared, it would
	// be defeated at the close (defeat.go).
	settleAt(t, g, 0, freeStar(g, 1), 0)
	p := sys.Planets[1]
	at, ok := p.BestCell(ColonyBase, g.Empires[1].Species.Traits())
	if !ok {
		t.Fatalf("fixture: no square for a Colony Base on %s", p.Name)
	}
	p.Surface[at].Item = ColonyBase
	p.Owner = 1
	c := NewColony(p)
	c.Star, c.PlanetIdx = home.Star, 1
	c.adopt(g.Empires[1])
	g.Empires[1].Colonies = append(g.Empires[1].Colonies, c)
	g.EndDay()
	if r.Intrusions != days {
		t.Errorf("a colony of theirs in the system: %d intrusions, want %d still", r.Intrusions, days)
	}
	atWar(t, g, 0, 1)
	for range days {
		g.EndDay()
	}
	if r.Intrusions != days || r.WarDays != days || g.Empires[1].Rel(0).WarDays != days {
		t.Errorf("at war: %+v / %+v", *r, *g.Empires[1].Rel(0))
	}
}

// Invade needs War with the owner: refused at peace, accepted at war.
func TestInvadeAtPeace(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Orfa)
	g.setStance(0, 1, Peace)
	s := g.Empires[0].Spawn(invader(1), col.Star, col.PlanetIdx)
	err := g.Empires[0].invadeErr(s)
	if err == nil || err.Error() != "we are not at war with the Orfa" {
		t.Errorf("at peace: %v", err)
	}
	g.setStance(0, 1, War)
	if err := g.Empires[0].invadeErr(s); err != nil {
		t.Errorf("at war: %v", err)
	}
}

// A destroyed ship and a captured colony are counted in the loser's
// ledger about the winner.
func TestLedgerLosses(t *testing.T) {
	g, sh, tg := roundDuel(oneGen, hulk)
	reps := g.destroyedBy(g.Empires[0], sh.Name, true, g.Empires[1], tg)
	if len(reps) != 2 || g.Empires[1].Rel(0).ShipsLost != 1 || g.Empires[0].Rel(1).ShipsLost != 0 {
		t.Errorf("ships lost %d (loser), %d (winner)", g.Empires[1].Rel(0).ShipsLost, g.Empires[0].Rel(1).ShipsLost)
	}
	// Each side's line names its own ship alone, the other's by its
	// species, and the star.
	at := g.Cluster.Stars[tg.Star].Name
	if w0, w1 := sh.Name+" destroys "+aShip(g.Empires, 1)+" at "+at, tg.Name+" lost to "+aShip(g.Empires, 0)+" at "+at; reps[0].Event.Text != w0 || reps[1].Event.Text != w1 {
		t.Errorf("texts %q / %q, want %q / %q", reps[0].Event.Text, reps[1].Event.Text, w0, w1)
	}
	if r := g.destroyedBy(g.Empires[0], "Home's Long Range Orbital Whopper", false, g.Empires[1], sh); !strings.Contains(r[1].Event.Text, "lost to Home's Long Range Orbital Whopper") {
		t.Errorf("a planet's kill: %q", r[1].Event.Text)
	}
	g, col := invadeFixture(t, Snovemdomas, Orfa)
	s := g.Empires[0].Spawn(invader(1), col.Star, col.PlanetIdx)
	reps, err := g.Invade(g.Empires[0], s)
	if err != nil || len(reps) == 0 || reps[0].Event.Kind != Invaded {
		t.Fatalf("invasion: %+v, %v", reps, err)
	}
	if g.Empires[1].Rel(0).PlanetsLost != 1 || g.Empires[0].Rel(1).PlanetsLost != 0 {
		t.Errorf("planets lost %d (loser), %d (winner)", g.Empires[1].Rel(0).PlanetsLost, g.Empires[0].Rel(1).PlanetsLost)
	}
}

// The Hanshaks meet everybody at day 0, the others only them.
func TestHanshaksMeetAll(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Hanshaks, Orfa}, Average)
	for a, e := range g.Empires {
		for b := range g.Empires {
			want := Unknown
			if a != b && (a == 1 || b == 1) {
				want = Peace
			}
			if r := e.Rel(b); r.Stance != want || want == Peace && r.Met != 0 {
				t.Errorf("%d about %d: %+v, want %v", a, b, *r, want)
			}
		}
	}
}

// Relink refuses an empire whose Relations are not one per empire.
func TestRelinkRelations(t *testing.T) {
	g := dipGame(t)
	if err := g.Relink(); err != nil {
		t.Fatal(err)
	}
	g.Empires[1].Relations = g.Empires[1].Relations[:2]
	if err := g.Relink(); err == nil {
		t.Error("short Relations relinked")
	}
}

// StanceWith and Rel outside the Relations: Unknown and nil.
func TestStanceOutOfRange(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	if e.StanceWith(0) != Unknown || e.Rel(0) != nil || dipGame(t).Empires[0].Rel(3) != nil {
		t.Error("out of range not Unknown/nil")
	}
}

// At peace two armed ships at one star make no combat star, see each
// other but are not Hostiles; at war they are.
func TestPeaceNoFight(t *testing.T) {
	g := dipGame(t)
	home := g.Empires[0].Colonies[0]
	for i := range 2 {
		g.Empires[i].Spawn(FixtureRaider, home.Star, home.PlanetIdx).Orbit = -1
	}
	g.Contact(0, 1)
	if got := g.CombatStars(); len(got) != 0 {
		t.Errorf("combat stars at peace %v", got)
	}
	if len(g.ShipsSeen(g.Empires[0], home.Star)) != 1 || len(g.Hostiles(g.Empires[0], home.Star)) != 0 {
		t.Error("at peace: the foreign ship unseen or hostile")
	}
	atWar(t, g, 0, 1)
	if got := g.CombatStars(); !slices.Equal(got, []int{home.Star}) || len(g.Hostiles(g.Empires[0], home.Star)) != 1 {
		t.Errorf("at war: combat stars %v", got)
	}
}

// The fixtures are fights: foes or an arena put empires 0 and 1 at War,
// the fixtures without foes leave them unmet.
func TestFixturesAtWar(t *testing.T) {
	g := dipGame(t)
	if err := SpawnFixtures(g, 1, 1, 0); err != nil || g.Empires[0].StanceWith(1) != Unknown {
		t.Fatalf("no foes: %v, %v", err, g.Empires[0].StanceWith(1))
	}
	if err := SpawnFixtures(g, 0, 0, 1); err != nil || !g.Hostile(g.Empires[0], g.Empires[1]) || g.Empires[2].StanceWith(0) != Unknown {
		t.Errorf("foes: %v, %v", err, g.Empires[0].StanceWith(1))
	}
	if err := SpawnFixtures(g, 0, 0, 1); err != nil || g.Empires[1].Rel(0).WarsOn != 1 {
		t.Errorf("foes again: %v, a second war declared", err)
	}
	g = dipGame(t)
	if err := SpawnArena(g, []Design{FixtureRaider}, []Design{FixtureRaider}); err != nil || !g.Hostile(g.Empires[1], g.Empires[0]) {
		t.Errorf("arena: %v, %v", err, g.Empires[1].StanceWith(0))
	}
}

// A fixture's war holds on a loaded save whatever the pair's state: 0's
// Friendship toward 1 at FriendBar or more, or 0 and 1 allied, would
// have refused declareWar and left the foes at Peace, unable to fight.
func TestFixtureWarOnAFriend(t *testing.T) {
	g := dipGame(t)
	g.Contact(0, 1)
	g.Empires[0].Relations[1].Friendship = 2 * FriendBar
	if err := SpawnFixtures(g, 0, 0, 1); err != nil || !g.Hostile(g.Empires[0], g.Empires[1]) {
		t.Errorf("friends: %v, %v", err, g.Empires[0].StanceWith(1))
	}
	g = dipGame(t)
	ally(t, g, 0, 1)
	if err := SpawnFixtures(g, 0, 0, 1); err != nil || !g.Hostile(g.Empires[0], g.Empires[1]) {
		t.Errorf("allies: %v, %v", err, g.Empires[0].StanceWith(1))
	}
}

// A combat star made by a war (0 and 1) does not open fire on a third
// empire at peace with the shooter: Arm and ArmPlanet refuse a ship of
// empire 2 (seen, not Hostile), so the round cannot hit it.
func TestArmRefusesPeace(t *testing.T) {
	g := dipGame(t)
	e0 := g.Empires[0]
	home := e0.Colonies[0]
	p := home.Planet
	atWar(t, g, 0, 1)
	g.Contact(0, 2)
	sh := e0.Spawn(siegeGun, home.Star, home.PlanetIdx)
	inSpaceAt(sh, Vec3{X: p.Pos.X + 0.1, Y: p.Pos.Y, Z: p.Pos.Z})
	foe := g.Empires[1].Spawn(FixtureRaider, home.Star, home.PlanetIdx)
	inSpaceAt(foe, Vec3{X: p.Pos.X - 0.1, Y: p.Pos.Y, Z: p.Pos.Z})
	friend := g.Empires[2].Spawn(FixtureRaider, home.Star, home.PlanetIdx)
	inSpaceAt(friend, Vec3{X: p.Pos.X + 0.15, Y: p.Pos.Y, Z: p.Pos.Z})
	if !slices.Equal(g.CombatStars(), []int{home.Star}) {
		t.Fatalf("combat stars %v: the war makes none", g.CombatStars())
	}
	sg, ok := g.SightingOf(e0, friend)
	if !ok {
		t.Fatal("the peaceful ship unseen")
	}
	if err := e0.Arm(sh, part(siegeGun, Nanomanipulator), sg); err == nil || !strings.Contains(err.Error(), "not hostile") {
		t.Errorf("Arm at peace: %v", err)
	}
	if err := p.BuildOrbital(4, LongRangeOrbitalWhopper); err != nil {
		t.Fatal(err)
	}
	if err := e0.ArmPlanet(home, 4, sg); err == nil || !strings.Contains(err.Error(), "not hostile") {
		t.Errorf("ArmPlanet at peace: %v", err)
	}
	hull := friend.Hull
	g.Round(home.Star, nil)
	if friend.Hull != hull || g.Empires[2].Rel(0).ShipsLost != 0 {
		t.Errorf("the peaceful ship hit: hull %d of %d", friend.Hull, hull)
	}
	if fg, _ := g.SightingOf(e0, foe); e0.Arm(sh, part(siegeGun, Nanomanipulator), fg) != nil {
		t.Error("the foe at war refused")
	}
}

// ally makes a and b allies (Contact, OfferAlliance, Answer), the reports
// dropped.
func ally(t *testing.T, g *Game, a, b int) {
	t.Helper()
	g.Contact(a, b)
	if _, err := g.OfferAlliance(a, b); err != nil {
		t.Fatalf("alliance %d-%d: %v", a, b, err)
	}
	o := g.PendingOffers(b)
	if _, err := g.Answer(o[len(o)-1], true); err != nil {
		t.Fatal(err)
	}
}

// The alliance: offered at Peace only, accepted both ways with an
// AllianceMade each, a day counted on both sides; war refused while
// allied; broken back to Peace, Broken counted on the victim's side only,
// the pending offers between them dropped either way.
func TestAllianceLifecycle(t *testing.T) {
	g := dipGame(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	if _, err := g.OfferAlliance(0, 1); err == nil || !strings.Contains(err.Error(), "never met") {
		t.Errorf("unmet: %v", err)
	}
	atWar(t, g, 0, 2)
	if _, err := g.OfferAlliance(0, 2); err == nil || !strings.Contains(err.Error(), "not at peace") {
		t.Errorf("at war: %v", err)
	}
	g.Contact(0, 1)
	reps, err := g.OfferAlliance(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	o := Offer{From: 0, To: 1, Kind: AllianceOffer, Day: g.Day, Third: -1}
	if !slices.Equal(g.PendingOffers(1), []Offer{o}) || e0.Rel(1).LastOffer != g.Day {
		t.Errorf("pending %+v", g.Offers)
	}
	if len(reps) != 2 || reps[0].Empire != 1 || reps[0].Event.Kind != AllianceOffered || reps[0].Event.Other != 0 ||
		reps[0].Event.Text != "Snovemdomas offers alliance" || reps[1].Empire != 0 || reps[1].Event.Text != "Alliance offered to Orfa" {
		t.Errorf("offer reports %+v", reps)
	}
	// A crossed trade is not moot in an alliance: kept.
	g.Offers = append(g.Offers, Offer{From: 1, To: 0, Kind: TradeStars, Day: g.Day, Third: -1})
	if reps, err = g.Answer(o, true); err != nil {
		t.Fatal(err)
	}
	if e0.StanceWith(1) != Alliance || e1.StanceWith(0) != Alliance || g.Hostile(e0, e1) || g.Hostile(e1, e0) {
		t.Errorf("accepted: %v / %v", e0.StanceWith(1), e1.StanceWith(0))
	}
	if want := []Offer{{From: 1, To: 0, Kind: TradeStars, Day: g.Day, Third: -1}}; !slices.Equal(g.Offers, want) {
		t.Errorf("offers %+v, want %+v", g.Offers, want)
	}
	g.Offers = nil
	if len(reps) != 2 || reps[0].Empire != 0 || reps[0].Event.Kind != AllianceMade || reps[0].Event.Text != "Alliance with Orfa" ||
		reps[1].Empire != 1 || reps[1].Event.Kind != AllianceMade || reps[1].Event.Text != "Alliance with Snovemdomas" {
		t.Errorf("made reports %+v", reps)
	}
	g.EndDay()
	if e0.Rel(1).AllyDays != 1 || e1.Rel(0).AllyDays != 1 || e0.Rel(1).WarDays != 0 {
		t.Errorf("a day allied: %+v / %+v", *e0.Rel(1), *e1.Rel(0))
	}
	if _, err := g.DeclareWar(0, 1); err == nil || e0.StanceWith(1) != Alliance {
		t.Errorf("war on an ally: %v", err)
	}
	if _, err := g.OfferAlliance(0, 1); err == nil {
		t.Error("an alliance offered to an ally")
	}
	g.Offers = []Offer{{From: 0, To: 1, Kind: TradeLanes, Third: -1}, {From: 1, To: 0, Kind: TradeStars, Third: -1}, {From: 2, To: 0, Kind: PeaceOffer, Third: -1}}
	if reps, err = g.BreakAlliance(0, 1); err != nil {
		t.Fatal(err)
	}
	if e0.StanceWith(1) != Peace || e1.StanceWith(0) != Peace {
		t.Errorf("broken: %v / %v", e0.StanceWith(1), e1.StanceWith(0))
	}
	if e1.Rel(0).Broken != 1 || e0.Rel(1).Broken != 0 {
		t.Errorf("Broken: 1 about 0 %d (want 1), 0 about 1 %d (want 0)", e1.Rel(0).Broken, e0.Rel(1).Broken)
	}
	if want := []Offer{{From: 2, To: 0, Kind: PeaceOffer, Third: -1}}; !slices.Equal(g.Offers, want) {
		t.Errorf("offers %+v, want %+v", g.Offers, want)
	}
	if len(reps) != 2 || reps[0].Empire != 0 || reps[0].Event.Kind != AllianceBroken || reps[0].Event.Text != "Alliance with Orfa broken" ||
		reps[1].Empire != 1 || reps[1].Event.Text != "Snovemdomas broke our alliance" {
		t.Errorf("broken reports %+v", reps)
	}
	if _, err := g.BreakAlliance(0, 1); err == nil || !strings.Contains(err.Error(), "not allied") {
		t.Errorf("broken twice: %v", err)
	}
	if _, err := g.DeclareWar(0, 1); err != nil {
		t.Errorf("war after the break: %v", err)
	}
}

// Crossed alliance offers: accepting one drops the other (no expiry as a
// refusal between allies); a refused alliance counts Refused and says so.
func TestCrossedAlliance(t *testing.T) {
	g := dipGame(t)
	g.Contact(0, 1)
	g.Contact(0, 2)
	for _, p := range [][2]int{{0, 1}, {1, 0}, {0, 2}} {
		if _, err := g.OfferAlliance(p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	accept(t, g, 0, 1)
	if want := []Offer{{From: 0, To: 2, Kind: AllianceOffer, Day: g.Day, Third: -1}}; !slices.Equal(g.Offers, want) {
		t.Errorf("offers %+v, want %+v", g.Offers, want)
	}
	reps, err := g.Answer(g.Offers[0], false)
	if err != nil || len(reps) != 2 || reps[0].Empire != 0 || reps[0].Event.Kind != OfferRefused || reps[0].Event.Text != "Mebes refuses alliance" ||
		reps[1].Event.Text != "Refused Snovemdomas: alliance" || g.Empires[0].Rel(2).Refused != 1 || g.Empires[0].StanceWith(2) != Peace {
		t.Errorf("refused: %+v %v", reps, err)
	}
}

// One pending offer per ordered pair whatever the kind, and the cooldown
// counts any offer: an alliance offer from 0 to 1 blocks 0's trade offer
// to 1, not 1's to 0.
func TestOneOfferPerPair(t *testing.T) {
	g := dipGame(t)
	g.Contact(0, 1)
	g.Empires[1].Research.Grant(Tech(0)) // something to exchange both ways
	g.Empires[0].Research.Grant(Tech(1))
	if _, err := g.OfferAlliance(0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := g.OfferTrade(0, 1, TradeResearch); err == nil || !strings.Contains(err.Error(), "already offered") {
		t.Errorf("trade behind an alliance offer: %v", err)
	}
	if _, err := g.OfferTrade(1, 0, TradeResearch); err != nil {
		t.Errorf("the other direction: %v", err)
	}
	g.Offers = nil
	if _, err := g.OfferTrade(0, 1, TradeResearch); err == nil || !strings.Contains(err.Error(), "days ago") {
		t.Errorf("trade within the cooldown of an alliance offer: %v", err)
	}
}

// Kinds and their names; only the three exchanges are trades, and a
// trade's Gain is nothing for any other kind.
func TestOfferKinds(t *testing.T) {
	names := []string{"peace", "alliance", "stars", "lanes", "research"}
	for k := PeaceOffer; k <= TradeResearch; k++ {
		if k.String() != names[k] || k.Trade() != (k >= TradeStars) {
			t.Errorf("%d: %q trade %v", k, k, k.Trade())
		}
	}
	g := dipGame(t)
	g.Empires[1].Research.Grant(Tech(0))
	g.Empires[1].Visit(g.Empires[2].Colonies[0].Star)
	for _, k := range []OfferKind{PeaceOffer, AllianceOffer} {
		if gn := g.Gain(0, 1, k); !gn.Zero() || gn.Tech != NoTech {
			t.Errorf("%s: gain %+v", k, gn)
		}
	}
	if (Gain{Tech: NoTech}).Zero() != true || (Gain{Stars: 1, Tech: NoTech}).Zero() || (Gain{Tech: 0}).Zero() {
		t.Error("Zero")
	}
}

// diff lists what to would learn of from's knowledge, by knows (an index
// predicate per empire) over n indices: the test's oracle for Gain.
func diff(n int, knows func(e *Empire, i int) bool, to, from *Empire) []int {
	var out []int
	for i := range n {
		if knows(from, i) && !knows(to, i) {
			out = append(out, i)
		}
	}
	return out
}

func knowsStar(e *Empire, i int) bool { return e.Knows(i) }
func knowsLane(e *Empire, i int) bool { return e.KnowsLane(i) }

// cheapest is the tech from knows and to does not with the lowest Cost,
// ties by index, NoTech for none: the test's oracle for TradeResearch.
func cheapest(to, from *Empire) Tech {
	best := NoTech
	for _, t := range diff(int(NumTechs), func(e *Empire, i int) bool { return e.Research.Known[i] }, to, from) {
		if best == NoTech || to.Research.Cost(Tech(t)) < to.Research.Cost(best) {
			best = Tech(t)
		}
	}
	return best
}

// accept answers the one pending offer from a to b, accepted.
func accept(t *testing.T, g *Game, a, b int) []Report {
	t.Helper()
	for _, o := range g.PendingOffers(b) {
		if o.From == a {
			reps, err := g.Answer(o, true)
			if err != nil {
				t.Fatal(err)
			}
			return reps
		}
	}
	t.Fatalf("no offer from %d to %d", a, b)
	return nil
}

// The three exchanges on a hand-made asymmetry: Gain both ways equals the
// knowledge difference, the accepted trade applies both, each side's
// TradeMade says what it learnt and what it taught, a Trade on both ledgers; a side with
// nothing to learn learns nothing; nothing to exchange both ways is
// refused; at War and unmet refused; a pending trade accepted at war is
// refused and stays.
func TestTrades(t *testing.T) {
	g := dipGame(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	ns, nl := len(g.Cluster.Stars), len(g.Cluster.Lanes)
	g.Contact(0, 1)
	// A star neither knows, visited by 1; a lane with both ends unknown to
	// both, learnt by 1.
	far, lane := -1, -1
	for st := range ns {
		if !e0.Knows(st) && !e1.Knows(st) && !g.Empires[2].Present(st) {
			far = st
			break
		}
	}
	e1.Visit(far)
	for li, l := range g.Cluster.Lanes {
		if !e0.Knows(l.A) && !e0.Knows(l.B) && !e1.Knows(l.A) && !e1.Knows(l.B) {
			lane = li
			break
		}
	}
	if far < 0 || lane < 0 {
		t.Fatal("fixture: no far star or lane")
	}
	e1.LearnLane(lane)
	if !e1.KnowsLane(lane) || e0.KnowsLane(lane) {
		t.Fatal("LearnLane")
	}

	// Stars.
	s0, s1 := diff(ns, knowsStar, e0, e1), diff(ns, knowsStar, e1, e0)
	if !slices.Contains(s0, far) || len(s1) == 0 {
		t.Fatalf("fixture: stars %v / %v", s0, s1)
	}
	if g0, g1 := g.Gain(0, 1, TradeStars), g.Gain(1, 0, TradeStars); g0.Stars != len(s0) || g1.Stars != len(s1) || g0.Lanes != 0 || g0.Tech != NoTech {
		t.Errorf("star gains %+v / %+v, want %d / %d", g0, g1, len(s0), len(s1))
	}
	reps, err := g.OfferTrade(0, 1, TradeStars)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) != 2 || reps[0].Empire != 1 || reps[0].Event.Kind != TradeOffered || reps[0].Event.Text != "Snovemdomas offers trade of stars" ||
		reps[1].Event.Text != "Trade of stars offered to Orfa" {
		t.Errorf("offer reports %+v", reps)
	}
	reps = accept(t, g, 0, 1)
	for _, st := range s0 {
		if !e0.Knows(st) {
			t.Errorf("0 does not know star %d", st)
		}
	}
	for _, st := range s1 {
		if !e1.Knows(st) {
			t.Errorf("1 does not know star %d", st)
		}
	}
	if e0.Rel(1).Trades != 1 || e1.Rel(0).Trades != 1 || e0.Rel(1).Friendship != FriendTrade || e1.Rel(0).Friendship != FriendTrade {
		t.Errorf("Trades %d / %d, friendship %v / %v", e0.Rel(1).Trades, e1.Rel(0).Trades, e0.Rel(1).Friendship, e1.Rel(0).Friendship)
	}
	if len(reps) != 2 || reps[0].Empire != 0 || reps[0].Event.Kind != TradeMade || reps[0].Event.Other != 1 ||
		reps[0].Event.Text != countOf(len(s0), "star")+" from Orfa; taught them "+countOf(len(s1), "star") ||
		reps[1].Event.Text != countOf(len(s1), "star")+" from Snovemdomas; taught them "+countOf(len(s0), "star") {
		t.Errorf("made reports %+v", reps)
	}
	g.Day += OfferCooldown
	if _, err := g.OfferTrade(0, 1, TradeStars); err == nil || !strings.Contains(err.Error(), "nothing to exchange") {
		t.Errorf("stars again: %v", err)
	}

	// Lanes: after the stars, only the learnt lane differs: 1 learns
	// nothing, the trade still goes.
	l0, l1 := diff(nl, knowsLane, e0, e1), diff(nl, knowsLane, e1, e0)
	if !slices.Equal(l0, []int{lane}) || len(l1) != 0 {
		t.Fatalf("lanes %v / %v, want [%d] / []", l0, l1, lane)
	}
	if gn := g.Gain(0, 1, TradeLanes); gn.Lanes != 1 || gn.Stars != 0 {
		t.Errorf("lane gain %+v", gn)
	}
	if _, err := g.OfferTrade(1, 0, TradeLanes); err != nil {
		t.Fatal(err)
	}
	if reps, err := g.Answer(g.PendingOffers(0)[0], false); err != nil || reps[0].Event.Text != "Snovemdomas refuses trade of lanes" ||
		reps[1].Event.Text != "Refused Orfa: trade of lanes" || e1.Rel(0).Refused != 1 {
		t.Errorf("refused: %+v %v", reps, err)
	}
	g.Day += OfferCooldown
	if _, err := g.OfferTrade(1, 0, TradeLanes); err != nil {
		t.Fatal(err)
	}
	reps = accept(t, g, 1, 0)
	if !e0.KnowsLane(lane) || e0.Knows(g.Cluster.Lanes[lane].A) || e0.Knows(g.Cluster.Lanes[lane].B) {
		t.Errorf("0 learnt the lane %v, its ends known %v", e0.KnowsLane(lane), e0.Knows(g.Cluster.Lanes[lane].A))
	}
	if reps[0].Empire != 1 || reps[0].Event.Text != "Nothing learnt from Snovemdomas; taught them 1 lane" ||
		reps[1].Event.Text != "1 lane from Orfa; taught them nothing" || e0.Rel(1).Trades != 2 || e1.Rel(0).Trades != 2 {
		t.Errorf("lane reports %+v", reps)
	}

	// Research: two techs on 1's side, one on 0's, of different costs.
	var unk []Tech
	for tc := Tech(0); tc < NumTechs && len(unk) < 3; tc++ {
		if !e0.Research.Known[tc] && !e1.Research.Known[tc] && (len(unk) == 0 || e0.Research.Cost(tc) != e0.Research.Cost(unk[0])) {
			unk = append(unk, tc)
		}
	}
	e1.Research.Grant(unk[2])
	e1.Research.Grant(unk[1])
	e0.Research.Grant(unk[0])
	w0, w1 := cheapest(e0, e1), cheapest(e1, e0)
	if w0 == NoTech || w1 == NoTech {
		t.Fatal("fixture: no tech to exchange")
	}
	if g0, g1 := g.Gain(0, 1, TradeResearch), g.Gain(1, 0, TradeResearch); g0.Tech != w0 || g1.Tech != w1 || g0.Stars != 0 {
		t.Errorf("research gains %v / %v, want %v / %v", g0.Tech, g1.Tech, w0, w1)
	}
	known0 := e0.Research.Known
	if _, err := g.OfferTrade(0, 1, TradeResearch); err != nil {
		t.Fatal(err)
	}
	reps = accept(t, g, 0, 1)
	known0[w0] = true
	if e0.Research.Known != known0 || !e1.Research.Known[w1] {
		t.Errorf("research: 0 learnt more than %v, or 1 not %v", w0, w1)
	}
	if reps[0].Event.Text != w0.String()+" from Orfa; taught them "+w1.String() ||
		reps[1].Event.Text != w1.String()+" from Snovemdomas; taught them "+w0.String() {
		t.Errorf("research reports %+v", reps)
	}

	// Refusals: not a trade, unmet, at war; a pending trade accepted at war.
	if _, err := g.OfferTrade(0, 1, AllianceOffer); err == nil || !strings.Contains(err.Error(), "not an exchange") {
		t.Errorf("an alliance as a trade: %v", err)
	}
	if _, err := g.OfferTrade(1, 2, TradeStars); err == nil || !strings.Contains(err.Error(), "never met") {
		t.Errorf("unmet: %v", err)
	}
	atWar(t, g, 0, 2)
	if _, err := g.OfferTrade(0, 2, TradeStars); err == nil || !strings.Contains(err.Error(), "only peace") {
		t.Errorf("at war: %v", err)
	}
	g.Day += OfferCooldown
	if _, err := g.OfferTrade(1, 0, TradeResearch); err != nil {
		t.Fatal(err)
	}
	o := g.PendingOffers(0)[0]
	g.setStance(0, 1, War) // a state DeclareWar never leaves (it drops the offers)
	if _, err := g.Answer(o, true); err == nil || !slices.Contains(g.Offers, o) {
		t.Errorf("trade accepted at war: %v, offers %+v", err, g.Offers)
	}
}

// KnowsLane past a short LanesKnown (a save before any lane was learnt,
// or a grown cluster) reads false, no panic.
func TestLanesKnownShort(t *testing.T) {
	e := dipGame(t).Empires[0]
	li := -1
	for i, l := range e.Cluster.Lanes {
		if !e.Knows(l.A) && !e.Knows(l.B) {
			li = i
		}
	}
	e.LanesKnown = []bool{true}
	if li < 1 || e.KnowsLane(li) || !e.KnowsLane(0) {
		t.Errorf("lane %d: %v, lane 0 %v", li, e.KnowsLane(li), e.KnowsLane(0))
	}
	e.LearnLane(li)
	if len(e.LanesKnown) != len(e.Cluster.Lanes) || !e.KnowsLane(li) {
		t.Errorf("grown to %d, knows %v", len(e.LanesKnown), e.KnowsLane(li))
	}
}

// hospitable gives g's empire a home colony running Alien Hospitality,
// with industry enough for a share; q is the share it diverts.
func hospitable(t *testing.T, g *Game, a int) (c *Colony, q int) {
	t.Helper()
	e := g.Empires[a]
	c = e.Colonies[0]
	e.Research.Grant(Tech(AlienHospitality.Info().ResearchReq))
	for range 4 {
		at, ok := c.Planet.BestCell(Factory, e.Species.Traits())
		if !ok || c.Planet.Build(at, Factory) != nil {
			t.Fatal("no square for a Factory")
		}
	}
	c.Pop = c.Busy() + 2
	ind, res, pros := c.Output()
	if err := c.StartProject(e.Research, AlienHospitality); err != nil {
		t.Fatal(err)
	}
	it, q := c.Diverted()
	if it != AlienHospitality || q <= 0 {
		t.Fatalf("diverted %v %d", it, q)
	}
	// The industry is off; research and prosperity are untouched (no
	// party, no takeover).
	if i2, r2, p2 := c.Output(); i2 != 0 || r2 != res || p2 != pros || ind == 0 {
		t.Errorf("output %d %d %d, want 0 %d %d (was ind %d)", i2, r2, p2, res, pros, ind)
	}
	return c, q
}

// Alien Hospitality: a's diverted q a day goes to the Hospitality every
// empire at Peace or Alliance holds about a; nothing at War or unmet;
// a's own ledger unmoved.
func TestHospitality(t *testing.T) {
	g := dipGame(t)
	_, q := hospitable(t, g, 0)
	g.Contact(0, 1)
	g.EndDay()
	e0, e1, e2 := g.Empires[0], g.Empires[1], g.Empires[2]
	if e1.Rel(0).Hospitality != q || e2.Rel(0).Hospitality != 0 || e0.Rel(1).Hospitality != 0 {
		t.Errorf("at peace: 1 about 0 %d (want %d), 2 (unmet) %d, 0 about 1 %d",
			e1.Rel(0).Hospitality, q, e2.Rel(0).Hospitality, e0.Rel(1).Hospitality)
	}
	atWar(t, g, 0, 1)
	ally(t, g, 0, 2)
	g.EndDay()
	if e1.Rel(0).Hospitality != q || e2.Rel(0).Hospitality != q || e0.Rel(2).Hospitality != 0 {
		t.Errorf("1 at war %d (want %d), 2 allied %d (want %d), 0 about 2 %d",
			e1.Rel(0).Hospitality, q, e2.Rel(0).Hospitality, q, e0.Rel(2).Hospitality)
	}
	if e0.HospitalityGiven != 2*q || e1.HospitalityGiven != 0 {
		t.Errorf("made: %d (want %d over two days), the others' %d", e0.HospitalityGiven, 2*q, e1.HospitalityGiven)
	}
}

// Friendship: 1 about 0 holds FriendHosp × the day's q after a close,
// fades by FriendFade a close once 0 gives nothing more; at FriendBar or
// more war on 0 is refused, WarBarred closes ahead, also as an ally's
// granted request (no Helped, no friendship for the ally); once faded
// under the bar, war is declared.
func TestFriendshipBarsWar(t *testing.T) {
	g := dipGame(t)
	_, q := hospitable(t, g, 0)
	g.Contact(0, 1)
	g.Contact(1, 2)
	g.Contact(0, 2)
	g.EndDay()
	e0, e1 := g.Empires[0], g.Empires[1]
	if f := e1.Rel(0).Friendship; f != FriendHosp*float64(q) {
		t.Fatalf("friendship %v after a day, want %v", f, FriendHosp*float64(q))
	}
	e0.Colonies[0].Project = nil // the hospitality stops
	e1.Relations[0].Friendship = 55
	want := 0
	for f := 55.0; f >= FriendBar; f *= 1 - FriendFade {
		want++
	}
	if n := g.WarBarred(1, 0); n != want || want != 10 {
		t.Fatalf("WarBarred %d, want %d (10)", n, want)
	}
	if _, err := g.DeclareWar(1, 0); err == nil || !strings.Contains(err.Error(), "are your friends: no war on them for 10 days") || e1.StanceWith(0) != Peace {
		t.Errorf("barred war: %v, stance %v", err, e1.StanceWith(0))
	}
	ally(t, g, 2, 1)
	g.Empires[2].Relations[1].LastOffer = -1 // past OfferCooldown
	if _, err := g.Request(2, 1, AskWarOn, 0); err != nil {
		t.Fatalf("request: %v", err)
	}
	o := g.PendingOffers(1)[0]
	ally0 := g.Empires[2].Rel(1).Friendship
	reps, err := g.Answer(o, true)
	if r := g.Empires[2].Rel(1); err != nil || e1.StanceWith(0) != Peace || r.Helped != 0 || r.Refused != 1 || r.Friendship != ally0 || len(reps) != 2 {
		t.Errorf("granted request: %v, stance %v, ledger %+v, reports %+v", err, e1.StanceWith(0), *r, reps)
	}
	if len(reps) == 2 && (!strings.HasSuffix(reps[1].Event.Text, "are our friends)") || strings.Contains(reps[0].Event.Text, "friends")) {
		t.Errorf("the reason told the refuser only: %q / %q", reps[0].Event.Text, reps[1].Event.Text)
	}
	for range want - 1 {
		g.EndDay()
	}
	if n := g.WarBarred(1, 0); n != 1 {
		t.Errorf("a close left: WarBarred %d", n)
	}
	g.EndDay()
	e0.Relations[1].Friendship = 80
	if _, err := g.DeclareWar(1, 0); err != nil || g.WarBarred(1, 0) != 0 || e1.Rel(0).Friendship >= FriendBar {
		t.Errorf("faded: %v, friendship %v", err, e1.Rel(0).Friendship)
	}
	// The victim forgets its friendship toward the attacker.
	if f := e0.Rel(1).Friendship; f != 0 {
		t.Errorf("war declared on 0: its friendship toward 1 %v", f)
	}
}

// Allies do not colonise each other's systems: an ally's colony at the
// star refuses Found and FoundAt (ErrAllySystem), the Colonizer kept; at
// Peace the same ship founds, an encroachment in the holder's ledger.
func TestFoundAllySystem(t *testing.T) {
	g := dipGame(t)
	home := g.Empires[0].Colonies[0]
	e1 := g.Empires[1]
	pl := -1
	for k, p := range g.Cluster.Stars[home.Star].System.Planets {
		if _, ok := p.BestCell(ColonyBase, e1.Species.Traits()); ok && p.Owner < 0 {
			pl = k
			break
		}
	}
	if pl < 0 {
		t.Fatal("fixture: no free planet at empire 0's home")
	}
	s := e1.Spawn(Design{Name: "Col", Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, Colonizer}}, home.Star, pl)
	ally(t, g, 1, 0)
	if !e1.AllySystem(home.Star) || g.Empires[0].AllySystem(home.Star) || g.Empires[2].AllySystem(home.Star) {
		t.Error("AllySystem")
	}
	at, _ := e1.Planet(home.Star, pl).BestCell(ColonyBase, e1.Species.Traits())
	if err := e1.FoundAt(s, at); !errors.Is(err, ErrAllySystem) || !strings.Contains(err.Error(), "the Snovemdomas hold") {
		t.Errorf("FoundAt: %v", err)
	}
	if err := e1.Found(s); !errors.Is(err, ErrAllySystem) || s.Cargo != 1 || len(e1.Colonies) != 1 {
		t.Errorf("Found: %v, cargo %d", err, s.Cargo)
	}
	if _, err := g.BreakAlliance(1, 0); err != nil {
		t.Fatal(err)
	}
	if err := e1.Found(s); err != nil || len(e1.Colonies) != 2 {
		t.Errorf("at peace: %v", err)
	}
	// At peace the founding beside empire 0's colony is in 0's ledger,
	// once; nobody else's.
	if n := g.Empires[0].Rel(1).Encroached; n != 1 {
		t.Errorf("empire 0's ledger: Encroached %d, want 1", n)
	}
	if n, m := e1.Rel(0).Encroached, g.Empires[2].Rel(1).Encroached; n != 0 || m != 0 {
		t.Errorf("Encroached elsewhere: %d %d", n, m)
	}
}

// A crossed exchange of the same kind (1 to 0 while 0's to 1 is
// pending) IS the exchange 1 accepts: it dies with it, else it would be
// answered for nothing (a second Trade counted) or expire as a refusal
// (Refused, a bot's grudge). A crossed exchange of another kind stays.
func TestAcceptDropsCrossedTrade(t *testing.T) {
	for _, crossed := range []OfferKind{TradeStars, TradeLanes} {
		g := dipGame(t)
		g.Contact(0, 1)
		if _, err := g.OfferTrade(0, 1, TradeStars); err != nil {
			t.Fatal(err)
		}
		if _, err := g.OfferTrade(1, 0, crossed); err != nil {
			t.Fatal(err)
		}
		accept(t, g, 0, 1)
		var want []Offer
		if crossed != TradeStars {
			want = []Offer{{From: 1, To: 0, Kind: crossed, Day: g.Day, Third: -1}}
		}
		if !slices.Equal(g.Offers, want) {
			t.Errorf("crossed %s: offers %+v, want %+v", crossed, g.Offers, want)
		}
		for g.Day < OfferExpiry+1 {
			g.EndDay()
		}
		if r := g.Empires[1].Rel(0); crossed == TradeStars && (r.Refused != 0 || r.Trades != 1) {
			t.Errorf("crossed %s: 1 about 0 %+v", crossed, *r)
		}
	}
}

// reqGame is dipGame with 0 and 1 allied (1 offered, so 0's LastOffer
// stays −1) and everybody met: 1 at Peace with 2.
func reqGame(t *testing.T) *Game {
	t.Helper()
	g := dipGame(t)
	ally(t, g, 1, 0)
	g.Contact(0, 2)
	g.Contact(1, 2)
	return g
}

// The war request: pending with Third, HelpAsked both; accepted, 1 and 2
// at War both ways as DeclareWar(1, 2) makes them (2's WarsOn), Helped on
// the ASKER's ledger about the helper, HelpGiven both then the war's own
// Reports; refused, Refused on 0's side and OfferRefused texts.
func TestRequestLifecycle(t *testing.T) {
	g := reqGame(t)
	reps, err := g.Request(0, 1, AskWarOn, 2)
	if err != nil {
		t.Fatal(err)
	}
	o := Offer{From: 0, To: 1, Kind: AskWarOn, Day: g.Day, Third: 2}
	if !slices.Equal(g.Offers, []Offer{o}) || g.Empires[0].Rel(1).LastOffer != g.Day {
		t.Fatalf("pending %+v", g.Offers)
	}
	want := []Report{
		{1, dipEvent(HelpAsked, 0, "Snovemdomas asks: war on Mebes")},
		{0, dipEvent(HelpAsked, 1, "Asked Orfa: war on Mebes")},
	}
	if !slices.Equal(reps, want) {
		t.Errorf("asked %+v", reps)
	}
	twin := reqGame(t)
	war, err := twin.DeclareWar(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	reps, err = g.Answer(o, true)
	if err != nil {
		t.Fatal(err)
	}
	want = append([]Report{
		{0, dipEvent(HelpGiven, 1, "Orfa grants: war on Mebes")},
		{1, dipEvent(HelpGiven, 0, "Granted Snovemdomas: war on Mebes")},
	}, war...)
	if !slices.Equal(reps, want) {
		t.Errorf("granted %+v\nwant %+v", reps, want)
	}
	e0, e1, e2 := g.Empires[0], g.Empires[1], g.Empires[2]
	if e1.StanceWith(2) != War || e2.StanceWith(1) != War || e2.Rel(1).WarsOn != 1 || e1.Rel(2).WarsOn != 0 {
		t.Errorf("war: %+v / %+v", *e1.Rel(2), *e2.Rel(1))
	}
	if e0.Rel(1).Helped != 1 || e1.Rel(0).Helped != 0 || e0.Rel(1).Friendship != FriendHelp || e1.Rel(0).Friendship != 0 || e0.StanceWith(2) != Peace || len(g.Offers) != 0 {
		t.Errorf("ledger: 0 about 1 %+v, 1 about 0 %+v, offers %+v", *e0.Rel(1), *e1.Rel(0), g.Offers)
	}

	g = reqGame(t)
	if _, err := g.Request(0, 1, AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	reps, err = g.Answer(g.Offers[0], false)
	want = []Report{
		{0, dipEvent(OfferRefused, 1, "Orfa refuses war on Mebes")},
		{1, dipEvent(OfferRefused, 0, "Refused Snovemdomas: war on Mebes")},
	}
	if err != nil || !slices.Equal(reps, want) {
		t.Errorf("refused %+v, %v", reps, err)
	}
	if r := g.Empires[0].Rel(1); r.Refused != 1 || r.Helped != 0 || g.Empires[1].StanceWith(2) != Peace || len(g.Offers) != 0 {
		t.Errorf("after refusal: %+v, offers %+v", *r, g.Offers)
	}
}

// Each request accepted performs its act on behalf of the ally: the
// game after it equals a twin where To acted itself (the act's Reports,
// To's and Third's ledgers, the Offers), Helped aside.
func TestRequestKinds(t *testing.T) {
	cases := []struct {
		k     OfferKind
		setup func(*Game) // 1's stance with 2
		act   func(*Game) ([]Report, error)
		check func(*Game) bool
	}{
		{AskPeaceWith, func(g *Game) { atWar(t, g, 1, 2) }, func(g *Game) ([]Report, error) { return g.OfferPeace(1, 2) },
			func(g *Game) bool {
				return slices.Contains(g.Offers, Offer{From: 1, To: 2, Kind: PeaceOffer, Day: g.Day, Third: -1})
			}},
		{AskWarOn, func(*Game) {}, func(g *Game) ([]Report, error) { return g.DeclareWar(1, 2) },
			func(g *Game) bool { return g.Empires[2].StanceWith(1) == War }},
		{AskBreakWith, func(g *Game) { ally(t, g, 2, 1) }, func(g *Game) ([]Report, error) { return g.BreakAlliance(1, 2) },
			func(g *Game) bool { return g.Empires[2].StanceWith(1) == Peace && g.Empires[2].Rel(1).Broken == 1 }},
		{AskAllianceWith, func(*Game) {}, func(g *Game) ([]Report, error) { return g.OfferAlliance(1, 2) },
			func(g *Game) bool {
				return slices.Contains(g.Offers, Offer{From: 1, To: 2, Kind: AllianceOffer, Day: g.Day, Third: -1})
			}},
	}
	for _, c := range cases {
		g, twin := reqGame(t), reqGame(t)
		c.setup(g)
		c.setup(twin)
		if _, err := g.Request(0, 1, c.k, 2); err != nil {
			t.Fatalf("%s: %v", c.k, err)
		}
		want, err := c.act(twin)
		if err != nil {
			t.Fatalf("%s twin: %v", c.k, err)
		}
		reps := accept(t, g, 0, 1)
		if len(reps) != 2+len(want) || reps[0].Event.Kind != HelpGiven || !slices.Equal(reps[2:], want) {
			t.Errorf("%s: reports %+v, act %+v", c.k, reps, want)
		}
		for _, k := range []int{1, 2} {
			if !slices.Equal(g.Empires[k].Relations, twin.Empires[k].Relations) {
				t.Errorf("%s: %d's relations %+v, twin %+v", c.k, k, g.Empires[k].Relations, twin.Empires[k].Relations)
			}
		}
		if !slices.Equal(g.Offers, twin.Offers) || !c.check(g) || g.Empires[0].Rel(1).Helped != 1 {
			t.Errorf("%s: offers %+v, twin %+v, 0 about 1 %+v", c.k, g.Offers, twin.Offers, *g.Empires[0].Rel(1))
		}
	}
}

// Request's refusals: no request kind, not allied, the third the asker
// or the ally or unmet by the asker, the ally's stance not fitting the
// act, one pending offer per pair whatever its kind, the cooldown.
func TestRequestRefused(t *testing.T) {
	refused := func(g *Game, k OfferKind, third int, want string) {
		t.Helper()
		if _, err := g.Request(0, 1, k, third); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s about %d: %v, want %q", k, third, err, want)
		}
		if slices.ContainsFunc(g.Offers, func(o Offer) bool { return o.Kind.Request() && o.Kind == k }) {
			t.Errorf("%s about %d: pending %+v", k, third, g.Offers)
		}
	}
	g := dipGame(t)
	g.Contact(0, 1)
	g.Contact(0, 2)
	g.Contact(1, 2)
	refused(g, AskWarOn, 2, "not allied with the Orfa")
	g = reqGame(t)
	refused(g, PeaceOffer, 2, "not a request")
	refused(g, AskWarOn, 0, "never met")
	refused(g, AskWarOn, 1, "never met")
	refused(g, AskWarOn, 3, "never met")
	g = dipGame(t)
	ally(t, g, 1, 0)
	g.Contact(1, 2)
	refused(g, AskWarOn, 2, "never met the Mebes")

	g = reqGame(t) // 1 at Peace with 2
	refused(g, AskPeaceWith, 2, "the Orfa are not at war with the Mebes")
	refused(g, AskBreakWith, 2, "the Orfa are not allied with the Mebes")
	atWar(t, g, 1, 2)
	refused(g, AskWarOn, 2, "the Orfa are not at peace with the Mebes")
	refused(g, AskAllianceWith, 2, "the Orfa are not at peace with the Mebes")
	refused(g, AskBreakWith, 2, "not allied with the Mebes")
	g = reqGame(t)
	ally(t, g, 2, 1)
	refused(g, AskWarOn, 2, "the Orfa are not at peace with the Mebes")
	refused(g, AskAllianceWith, 2, "not at peace")

	g = reqGame(t)
	if _, err := g.Request(0, 1, AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	refused(g, AskAllianceWith, 2, "already offered")
	if _, err := g.OfferTrade(0, 1, TradeStars); err == nil || !strings.Contains(err.Error(), "already offered") {
		t.Errorf("an exchange beside a pending request: %v", err)
	}
	if _, err := g.Answer(g.Offers[0], false); err != nil {
		t.Fatal(err)
	}
	refused(g, AskAllianceWith, 2, "days ago")
	g = reqGame(t)
	if _, err := g.OfferTrade(0, 1, TradeStars); err != nil {
		t.Fatal(err)
	}
	refused(g, AskWarOn, 2, "already offered")
}

// A war request made moot (the ally declares war on the third by
// itself): accepted, it errors "moot", gone, nothing counted; left
// pending, the close drops it silently, no refusal. A request still
// valid survives the close and expires refused as any offer.
func TestRequestMoot(t *testing.T) {
	g := reqGame(t)
	if _, err := g.Request(0, 1, AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	o := g.Offers[0]
	if _, err := g.DeclareWar(1, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Answer(o, true); err == nil || !strings.Contains(err.Error(), "moot") {
		t.Errorf("moot accepted: %v", err)
	}
	if r := g.Empires[0].Rel(1); len(g.Offers) != 0 || r.Helped != 0 || r.Refused != 0 || g.Empires[2].Rel(1).WarsOn != 1 {
		t.Errorf("after moot: offers %+v, 0 about 1 %+v", g.Offers, *r)
	}

	g = reqGame(t)
	if _, err := g.Request(0, 1, AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := g.DeclareWar(1, 2); err != nil {
		t.Fatal(err)
	}
	evs := g.EndDay()
	if r := g.Empires[0].Rel(1); len(g.Offers) != 0 || r.Refused != 0 || len(kinds(evs[0], OfferRefused))+len(kinds(evs[1], OfferRefused)) != 0 {
		t.Errorf("moot at the close: offers %+v, 0 about 1 %+v", g.Offers, *r)
	}

	g = reqGame(t)
	if _, err := g.Request(0, 1, AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	g.EndDay()
	if len(g.Offers) != 1 {
		t.Fatalf("a valid request dropped at the close: %+v", g.Offers)
	}
	for g.Day < OfferExpiry {
		g.EndDay()
	}
	if len(g.Offers) != 0 || g.Empires[0].Rel(1).Refused != 1 {
		t.Errorf("expiry: offers %+v, 0 about 1 %+v", g.Offers, *g.Empires[0].Rel(1))
	}
}

// A moot request REFUSED is dropped as the close drops it: no Refused on
// the asker's ledger, no OfferRefused, the "moot" error. The bots answer
// a request at the asker's turn in their Act, after the turns of lower
// indexes: an alliance with the third accepted there (or the third's
// war declared on them earlier in the day) makes it moot, and their
// Decide then refuses it.
func TestRequestMootRefused(t *testing.T) {
	g := reqGame(t)
	if _, err := g.Request(0, 1, AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	o := g.Offers[0]
	if _, err := g.DeclareWar(2, 1); err != nil {
		t.Fatal(err)
	}
	reps, err := g.Answer(o, false)
	if err == nil || !strings.Contains(err.Error(), "moot") || len(reps) != 0 {
		t.Errorf("moot refused: %v, %+v", err, reps)
	}
	if r := g.Empires[0].Rel(1); len(g.Offers) != 0 || r.Refused != 0 {
		t.Errorf("after moot refused: offers %+v, 0 about 1 %+v", g.Offers, *r)
	}
}

// The Baliflids force peace on the two at war with them (both ways, the
// offers between them dropped, the others kept, the war days kept), a
// Report each side after the AbilityUsed; then 100 days of recharge.
// Refused: a second use, at war with nobody; a passive species.
func TestAbilityForcePeace(t *testing.T) {
	g := NewGame(1, []Species{Baliflids, Snovemdomas, Orfa, Mebes}, Average)
	atWar(t, g, 0, 1)
	atWar(t, g, 0, 2)
	g.Contact(0, 3)
	g.Contact(1, 2)
	g.EndDay()
	for _, f := range []func() ([]Report, error){
		func() ([]Report, error) { return g.OfferPeace(1, 0) },
		func() ([]Report, error) { return g.OfferPeace(0, 2) },
		func() ([]Report, error) { return g.OfferTrade(1, 2, TradeStars) },
		func() ([]Report, error) { return g.OfferAlliance(0, 3) },
	} {
		if _, err := f(); err != nil {
			t.Fatal(err)
		}
	}
	kept := slices.DeleteFunc(slices.Clone(g.Offers), func(o Offer) bool { return o.Kind == PeaceOffer })
	wd := g.Empires[0].Rel(1).WarDays
	charge(g, 0)
	reps, err := g.UseAbility(0)
	if err != nil {
		t.Fatal(err)
	}
	want := []Report{
		{0, dipEvent(AbilityUsed, -1, "Peace forced on all")},
		{0, dipEvent(PeaceMade, 1, "Peace forced on Snovemdomas")},
		{1, dipEvent(PeaceMade, 0, "Peace forced by Baliflids")},
		{0, dipEvent(PeaceMade, 2, "Peace forced on Orfa")},
		{2, dipEvent(PeaceMade, 0, "Peace forced by Baliflids")},
	}
	if !slices.Equal(reps, want) {
		t.Errorf("reports %+v", reps)
	}
	for j := 1; j < 4; j++ {
		if g.Empires[0].StanceWith(j) != Peace || g.Empires[j].StanceWith(0) != Peace {
			t.Errorf("0-%d: %s / %s", j, g.Empires[0].StanceWith(j), g.Empires[j].StanceWith(0))
		}
	}
	if !slices.Equal(g.Offers, kept) || g.Empires[0].Rel(1).WarDays != wd || wd == 0 {
		t.Errorf("offers %+v, want %+v; war days %d, want %d", g.Offers, kept, g.Empires[0].Rel(1).WarDays, wd)
	}
	used := g.Day
	rc := Baliflids.Info().Recharge
	if ready, days := g.AbilityState(0); g.Empires[0].AbilityUsed != used || ready || days != rc {
		t.Errorf("after use: %d, %v %d", g.Empires[0].AbilityUsed, ready, days)
	}
	atWar(t, g, 0, 1)
	if _, err := g.UseAbility(0); err == nil || !strings.Contains(err.Error(), "ready in 100 days") {
		t.Errorf("second use: %v", err)
	}
	for g.Day < used+rc-1 {
		g.EndDay()
	}
	if ready, days := g.AbilityState(0); ready || days != 1 {
		t.Errorf("a day before: %v %d", ready, days)
	}
	g.EndDay()
	if ready, days := g.AbilityState(0); !ready || days != 0 {
		t.Errorf("recharged: %v %d", ready, days)
	}
	if _, err := g.UseAbility(0); err != nil || g.Empires[0].AbilityUsed != g.Day {
		t.Errorf("third use: %v", err)
	}
	charge(g, 0)
	if _, err := g.UseAbility(0); err == nil || !strings.Contains(err.Error(), "at war with nobody") || g.Empires[0].AbilityUsed != g.Day-rc {
		t.Errorf("at war with nobody: %v, used %d", err, g.Empires[0].AbilityUsed)
	}
	if _, err := g.UseAbility(2); err == nil || !strings.Contains(err.Error(), "the Orfa's ability is passive") {
		t.Errorf("Orfa: %v", err)
	}
	if ready, days := g.AbilityState(2); ready || days != 0 {
		t.Errorf("passive state: %v %d", ready, days)
	}
}

// The Marmosians' ability: each enemy defamed (Defamed on the LISTENER's
// ledger about the enemy) to every other empire that met it, nobody
// else's count moving; with two enemies each is defamed to everyone who
// met it, one another included. Only the AbilityUsed Report.
func TestAbilityCauseWar(t *testing.T) {
	defamed := func(g *Game) (n int) {
		for _, e := range g.Empires {
			for _, r := range e.Relations {
				n += r.Defamed
			}
		}
		return n
	}
	g := NewGame(1, []Species{Marmosians, Snovemdomas, Orfa, Mebes}, Average)
	charge(g, 0)
	if _, err := g.UseAbility(0); err == nil || !strings.Contains(err.Error(), "at war with nobody") {
		t.Errorf("no war: %v", err)
	}
	atWar(t, g, 0, 1)
	g.Contact(2, 1)
	g.Contact(0, 3)
	reps, err := g.UseAbility(0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Report{{0, dipEvent(AbilityUsed, -1, "Aliens turned on our enemies")}}; !slices.Equal(reps, want) {
		t.Errorf("reports %+v", reps)
	}
	if g.Empires[2].Rel(1).Defamed != 1 || defamed(g) != 1 || g.Empires[0].StanceWith(1) != War {
		t.Errorf("one enemy: 2 about 1 %+v, total %d", *g.Empires[2].Rel(1), defamed(g))
	}

	g = NewGame(1, []Species{Marmosians, Snovemdomas, Orfa, Mebes}, Average)
	charge(g, 0)
	atWar(t, g, 0, 1)
	atWar(t, g, 0, 2)
	g.Contact(1, 2)
	g.Contact(3, 1)
	g.Contact(3, 2)
	if _, err := g.UseAbility(0); err != nil {
		t.Fatal(err)
	}
	for y, ey := range g.Empires {
		for x := range g.Empires {
			want := 0
			if (x == 1 || x == 2) && y != 0 && y != x {
				want = 1
			}
			if got := ey.Relations[x].Defamed; got != want {
				t.Errorf("%d about %d: Defamed %d, want %d", y, x, got, want)
			}
		}
	}
}

// A peace or alliance request whose ally already made that very offer
// to the third is granted as it stands: no second
// offer, Helped counted, HelpGiven reported, the pending offer kept.
func TestRequestAlreadyOffered(t *testing.T) {
	g := reqGame(t)
	atWar(t, g, 1, 2)
	if _, err := g.OfferPeace(1, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Request(0, 1, AskPeaceWith, 2); err != nil {
		t.Fatal(err)
	}
	o := g.PendingOffers(1)[0]
	reps, err := g.Answer(o, true)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(g.PendingOffers(2)); n != 1 || g.Empires[0].Rel(1).Helped != 1 || len(reps) != 2 || reps[0].Event.Kind != HelpGiven {
		t.Errorf("offers to 2: %d, helped %d, reports %+v", n, g.Empires[0].Rel(1).Helped, reps)
	}
}
