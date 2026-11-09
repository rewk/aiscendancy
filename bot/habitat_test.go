package bot

import (
	"bytes"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/journal"
)

// habitatFixture: seed 1, one empire, every tech known but the
// Colonizer's (no colony ship), no black square on either planet (no
// Terraforming to pre-empt a ship), the home holding a Shipyard and six Factories, full
// (a donor), a second colony at the home star at Busy+1 (a recipient);
// the genome's HabitatShare share, HabitatFill 0.5; journalled into buf.
func habitatFixture(t *testing.T, share float64) (g *game.Game, b *Bot, home, second *game.Colony, buf *bytes.Buffer) {
	t.Helper()
	g = game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	gn := genomeWith(func(g *Genome) { g.HabitatShare, g.HabitatFill = share, 0.5 })
	b = NewAll(g, Config{NoResearch: true, NoDiplomacy: true, Genome: gn})[0]
	e := b.E
	e.Research = allKnown()
	home = e.Colonies[0]
	home.Planet.Orbit[0] = game.Shipyard
	for range 6 {
		at, ok := home.Planet.BestCell(game.Factory, home.Traits())
		if !ok {
			t.Fatal("no Factory square")
		}
		if err := home.Planet.Build(at, game.Factory); err != nil {
			t.Fatal(err)
		}
	}
	second = secondColony(t, b, home)
	e.Research.Known[game.Gizmos[game.Colonizer].ResearchReq] = false
	for _, p := range []*game.Planet{home.Planet, second.Planet} {
		for _, c := range p.Cells() {
			if sq := p.Surface[c]; sq.Kind == game.Black {
				sq.Kind = game.White
			}
		}
	}
	home.Pop = home.Capacity()
	if !donor(home) || !recipient(second, 0.5) || home.Free() < 2 {
		t.Fatalf("fixture: home %d/%d free %d, second %d/%d", home.Pop, home.Capacity(), home.Free(), second.Pop, second.Capacity())
	}
	buf = &bytes.Buffer{}
	g.SetJournal(journal.New(buf))
	return g, b, home, second, buf
}

// acts counts the journal's entries of act on colony name.
func acts(buf *bytes.Buffer, act, name string) int {
	n := 0
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, `"act":"`+act+`"`) && strings.Contains(l, `"colony":"`+name+`"`) {
			n++
		}
	}
	return n
}

// habitatShips counts b's ships with berths and its colonies' Ship
// projects building one.
func habitatShips(b *Bot) int {
	n := 0
	for _, s := range b.E.Ships {
		if s.Design.Berths() > 0 {
			n++
		}
	}
	for _, c := range b.E.Colonies {
		if d := building(c); d != nil && d.Berths() > 0 {
			n++
		}
	}
	return n
}

// The Habitat in play: a shipyard starts a Habitat ship with a full
// donor and a small recipient, the ship loads at the donor and unloads
// on the recipient; at HabitatShare 0 none is ever started.
func TestHabitatFerries(t *testing.T) {
	const days = 300
	for _, share := range []float64{0.5, 0} {
		g, b, home, second, buf := habitatFixture(t, share)
		bots := []*Bot{b}
		built := false
		for range days {
			Day(g, bots)
			built = built || habitatShips(b) > 0
			home.Pop = max(home.Pop, home.Capacity()) // its growth, stood in: a crew launched leaves it short
		}
		loads, unloads := acts(buf, "load", home.Planet.Name), acts(buf, "unload", second.Planet.Name)
		if share == 0 {
			if built || loads+unloads > 0 {
				t.Errorf("HabitatShare 0: built %v, %d loads, %d unloads", built, loads, unloads)
			}
			continue
		}
		if !built || loads == 0 || unloads == 0 {
			t.Errorf("HabitatShare %v, %d days: built %v, %d loads at %s, %d unloads at %s", share, days, built, loads, home.Planet.Name, unloads, second.Planet.Name)
		}
	}
}

// habitatDesign: wanted is ⌈HabitatShare × min(surplus, room) / berths⌉
// Habitat ships, flying and building; none at 0, none filling the
// fleet's last slot, none without a recipient.
func TestHabitatDesign(t *testing.T) {
	_, b, home, second, _ := habitatFixture(t, 0.5)
	d, ok := dayDesign(b, home)
	if !ok || d.Berths() == 0 || d.Name != game.HabitatName {
		t.Fatalf("full donor, small recipient: %v %v", ok, d)
	}
	surplus, room := home.Free(), second.Capacity()-second.Pop
	for _, share := range []float64{0.5, 1, 4} {
		b.Cfg.Genome.HabitatShare = share
		wanted := int(math.Ceil(share * float64(min(surplus, room)) / float64(d.Berths())))
		var spawned []*game.Starship
		for n := 0; ; n++ {
			_, ok := dayDesign(b, home)
			if fleet, capa := b.E.Fleet(); fleet+1 >= capa {
				if ok {
					t.Errorf("share %v: a Habitat ship filling the fleet's last slot", share)
				}
				break
			}
			if ok != (n < wanted) {
				t.Errorf("share %v, %d flying, %d wanted: %v", share, n, wanted, ok)
			}
			spawned = append(spawned, b.E.Spawn(d, home.Star, home.PlanetIdx))
		}
		for _, s := range spawned {
			if err := b.E.Abandon(s); err != nil {
				t.Fatal(err)
			}
		}
	}
	b.Cfg.Genome.HabitatShare = 0
	if _, ok := dayDesign(b, home); ok {
		t.Error("HabitatShare 0 wants a Habitat ship")
	}
	b.Cfg.Genome.HabitatShare = 0.5
	second.Pop = second.Capacity()/2 + 1 // over HabitatFill: no recipient
	if _, ok := dayDesign(b, home); ok {
		t.Errorf("no recipient (%d of %d) wants a Habitat ship", second.Pop, second.Capacity())
	}
}

// ferry (HabitatFill 1: a colony under Capacity is a recipient): an
// empty ship in the donor's orbit loads until refused and heads for the
// recipient; a loaded ship in space waits while the people already sent
// fill the recipient's room, and goes once a unit of room is left;
// unloading fills the recipient to its Capacity, then the empty ship
// heads back to the donor. An idle Habitat ship is never abandoned.
func TestFerry(t *testing.T) {
	_, b, home, second, _ := habitatFixture(t, 0.5)
	b.Cfg.Genome.HabitatFill = 1
	d, ok := dayDesign(b, home)
	if !ok {
		t.Fatal("no Habitat design")
	}
	n := d.Berths()
	second.Pop = second.Capacity() - n // room for one ship-load
	s1 := b.E.Spawn(d, home.Star, home.PlanetIdx)
	s2 := b.E.Spawn(d, home.Star, home.PlanetIdx)
	s2.Orbit, s2.Pos, s2.Pop = -1, game.Vec3{X: home.Planet.Pos.X + 0.3, Y: home.Planet.Pos.Y, Z: home.Planet.Pos.Z}, n
	if free := home.Free(); free < n {
		t.Fatalf("fixture: home free %d < %d berths", free, n)
	}
	b.ferry(s1, b.ferrying())
	if s1.Pop != n || s1.Order.Kind != game.GoToPlanet || s1.Order.Planet != second.PlanetIdx {
		t.Fatalf("s1: Pop %d (want %d), order %+v (want planet %d)", s1.Pop, n, s1.Order, second.PlanetIdx)
	}
	home.Pop = home.Capacity() // no recipient: s2's only one is second
	if b.ferry(s2, b.ferrying()); s2.Order.Kind != game.NoOrder {
		t.Errorf("s2 sent to a room s1 fills: %+v", s2.Order)
	}
	second.Pop--
	if b.ferry(s2, b.ferrying()); s2.Order.Kind != game.GoToPlanet || s2.Order.Planet != second.PlanetIdx {
		t.Errorf("s2 kept back from a unit of room: %+v", s2.Order)
	}
	second.Pop++
	// In the recipient's orbit: it unloads to Capacity, then heads home.
	s1.Order, s1.Orbit, s1.Pos = game.Order{}, second.PlanetIdx, second.Planet.Pos
	b.ferry(s1, b.ferrying())
	if second.Pop != second.Capacity() || s1.Pop != 0 || s1.Order.Kind != game.GoToPlanet || s1.Order.Planet != home.PlanetIdx {
		t.Errorf("unload: second %d of %d, s1 Pop %d, order %+v", second.Pop, second.Capacity(), s1.Pop, s1.Order)
	}
	// s2's target is full now: its order is dropped, it idles, it stays.
	if b.ferry(s2, b.ferrying()); s2.Order.Kind != game.NoOrder {
		t.Errorf("s2 kept an order to a full colony: %+v", s2.Order)
	}
	b.Cfg.Genome.AbandonIdle = 1
	for range 3 {
		b.abandonStranded()
	}
	if b.G.OwnerOf(s2) == nil {
		t.Error("an idle Habitat ship abandoned")
	}
}

// The Habitat genes: a nil genome and Neutral play 0.5 and 0.5; a genome
// file without them reads 0 (no Habitat), one with them keeps them.
func TestHabitatGenes(t *testing.T) {
	if s, f := (&Bot{}).habitatGenes(); s != 0.5 || f != 0.5 {
		t.Errorf("nil genome: %v %v", s, f)
	}
	if g := Neutral(); g.HabitatShare != 0.5 || g.HabitatFill != 0.5 {
		t.Errorf("Neutral: %v %v", g.HabitatShare, g.HabitatFill)
	}
	dir := t.TempDir()
	for _, c := range []struct {
		json        string
		share, fill float64
	}{{`{"FoundFloor": 3}`, 0, 0}, {`{"HabitatShare": 2, "HabitatFill": 0.25}`, 2, 0.25}, {`{"HabitatShare": 9, "HabitatFill": -1}`, 4, 0}} {
		path := dir + "/g.json"
		if err := os.WriteFile(path, []byte(c.json), 0o644); err != nil {
			t.Fatal(err)
		}
		g, err := ReadGenome(path)
		if err != nil {
			t.Fatal(err)
		}
		if g.HabitatShare != c.share || g.HabitatFill != c.fill {
			t.Errorf("%s: %v %v, want %v %v", c.json, g.HabitatShare, g.HabitatFill, c.share, c.fill)
		}
	}
}

// dayDesign is habitatDesign on a new Act's tally: each call a day.
func dayDesign(b *Bot, col *game.Colony) (game.Design, bool) {
	b.hab = nil
	return b.habitatDesign(col)
}

// A ship loads at a donor only with a recipient elsewhere to go to: else
// it waits there loaded, the donor drained into a recipient, lands its
// people back the next Act and loads them again the one after, forever
// (mksave -seed 2 -empires 4, from day 1392: a Capacity 10 colony, five
// aboard).
func TestFerryNoPingPong(t *testing.T) {
	_, b, home, second, _ := habitatFixture(t, 0.5)
	d, ok := dayDesign(b, home)
	if !ok {
		t.Fatal("no Habitat design")
	}
	second.Planet.PopBonus -= second.Capacity() - 2*d.Berths() + 2 // one ship-load crosses HabitatFill
	second.Pop = second.Capacity()
	s := b.E.Spawn(d, second.Star, second.PlanetIdx)
	if !donor(second) || second.Free() < d.Berths() {
		t.Fatalf("fixture: second %d of %d, free %d, berths %d", second.Pop, second.Capacity(), second.Free(), d.Berths())
	}
	for i := range 4 {
		pop := second.Pop
		b.ferry(s, b.ferrying())
		if second.Pop > pop || recipient(second, 0.5) {
			t.Fatalf("Act %d: second %d -> %d of %d, ship %d", i, pop, second.Pop, second.Capacity(), s.Pop)
		}
	}
	if s.Pop != 0 {
		t.Errorf("no recipient elsewhere: %d loaded", s.Pop)
	}
	home.Pop, s.Order = home.Busy()+1, game.Order{} // a recipient now
	if b.ferry(s, b.ferrying()); s.Pop == 0 || s.Order.Kind != game.GoToPlanet || s.Order.Planet != home.PlanetIdx {
		t.Errorf("a recipient elsewhere: %d loaded, order %+v", s.Pop, s.Order)
	}
}
