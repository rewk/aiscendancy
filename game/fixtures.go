package game

import (
	"fmt"
	"slices"
	"strings"
)

// The fixture designs (cmd/mksave), research not consulted.
//
// FixtureColonyShip (-ships) is the 2005 guide's Small colony ship with a
// Star Lane Drive for one Colonizer.
var FixtureColonyShip = Design{Name: "Colony Ship", Hull: Small,
	Parts: []Gizmo{ProtonShaver, TonklinMotor, StarLaneDrive, Colonizer, Colonizer}}

// FixtureRaider is the -foes design (and -warships', as FixtureGunboat):
// supply 4 for parts drawing 3; the Ion Wrap, while on, takes 1 more at
// each combat round.
var FixtureRaider = Design{Name: "Raider", Hull: Small,
	Parts: []Gizmo{ProtonShaver, ProtonShaver, TonklinMotor, MassBarrageGun, IonWrap}}

// FixtureLancer is FixtureRaider with Fourier Missiles (Level 2) for the
// Mass Barrage Gun: the one arena design that hurts a Raider.
var FixtureLancer = Design{Name: "Lancer", Hull: Small,
	Parts: []Gizmo{ProtonShaver, ProtonShaver, TonklinMotor, FourierMissiles, IonWrap}}

// FixtureGunboat is FixtureRaider named "Gunboat", for the player.
var FixtureGunboat = Design{Name: "Gunboat", Hull: FixtureRaider.Hull, Parts: FixtureRaider.Parts}

// The warship archetypes, one per hull, each of one tech era's parts, no
// Special but the lane drive. Composition rule: the day's supply pays one
// round of everything at once (the shield draw, every weapon firing once,
// one move) plus one more round's draw; Generators ≥ Weapons + 1 (a
// generator per shot and one for the move); the slots full. Each one's
// shields and hull outclass the next smaller one's (archetype_test.go).
// "round" below is that first round's cost: draw + weapons + move.

// FixtureCorvette (Small, early era) is FixtureLancer's parts: supply 4,
// 2 gens, draw 1, round 3 (4 with the second draw). The five slots leave
// no room for a scanner or a lane drive: two Proton Shavers are the energy.
var FixtureCorvette = Design{Name: "Corvette", Hull: Small,
	Parts: []Gizmo{ProtonShaver, ProtonShaver, TonklinMotor, FourierMissiles, IonWrap}}

// FixtureFrigate (Medium, second era): supply 16, 4 gens, draw 2, round 7
// (9). The Concussion Shield's 2 nullifies the corvette's Fourier Missiles.
var FixtureFrigate = Design{Name: "Frigate", Hull: Medium,
	Parts: []Gizmo{SubatomicScoop, SubatomicScoop, SubatomicScoop, SubatomicScoop, IonBanger,
		MolecularDisassociator, MolecularDisassociator, ConcussionShield, SubspacePhaseArray, StarLaneDrive}}

// FixtureCruiser (Large, third era): supply 36, 6 gens, draw 2, round 17
// (19): four Ueberlasers (2 shots a day) and a long Plasmatron.
var FixtureCruiser = Design{Name: "Cruiser", Hull: Large,
	Parts: []Gizmo{QuarkExpress, QuarkExpress, QuarkExpress, QuarkExpress, QuarkExpress, QuarkExpress, InertiaNegator,
		Ueberlaser, Ueberlaser, Ueberlaser, Ueberlaser, Plasmatron, Deactotron, AuralCloudConstructor, StarLaneDrive}}

// FixtureBattleship (Enormous, late era): supply 110, 11 gens, draw 3,
// round 68 (71): six Nanomanipulators (3 shots) and four longer
// Hypersphere Drivers (2).
var FixtureBattleship = Design{Name: "Battleship", Hull: Enormous,
	Parts: []Gizmo{Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler,
		Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, NanowaveSpaceBender,
		Nanomanipulator, Nanomanipulator, Nanomanipulator, Nanomanipulator, Nanomanipulator, Nanomanipulator,
		HypersphereDriver, HypersphereDriver, HypersphereDriver, HypersphereDriver,
		Nanoshell, HyperwaveTympanum, StarLaneHyperdrive}}

// SpawnFixtures puts the fixture ships at empire 0's first colony: ships
// colony ships in its orbit, then warships Gunboats of empire 0 and foes
// Raiders of empire 1 in space at FreeSpot by the planet (a fight needs
// them out of orbit), empires 0 and 1 at War with foes. An error, nothing
// spawned, without a second empire for foes or a colony of empire 0.
func SpawnFixtures(g *Game, ships, warships, foes int) error {
	if foes > 0 && len(g.Empires) < 2 {
		return fmt.Errorf("-foes %d needs a second empire (-empires >= 2)", foes)
	}
	if ships+warships+foes <= 0 {
		return nil
	}
	e := g.Empires[0]
	if len(e.Colonies) == 0 {
		return fmt.Errorf("fixtures: empire 0 has no colony to spawn them at")
	}
	home := e.Colonies[0]
	if foes > 0 {
		g.fixtureWar()
	}
	for range ships {
		e.Spawn(FixtureColonyShip, home.Star, home.PlanetIdx)
	}
	inSpace := func(s *Starship) {
		s.Orbit = -1
		s.Pos = FreeSpot(s.Pos, spaceAt(g.Empires, home.Star, s))
	}
	for range warships {
		inSpace(e.Spawn(FixtureGunboat, home.Star, home.PlanetIdx))
	}
	for range foes {
		inSpace(g.Empires[1].Spawn(FixtureRaider, home.Star, home.PlanetIdx))
	}
	return nil
}

// arenaNames is ArenaDesign's catalogue, in the order its error lists it.
var arenaNames = []string{"raider", "lancer", "colony", "corvette", "frigate", "cruiser", "battleship"}

// ArenaDesign is the arena's design by name, any case: "raider", "lancer",
// the archetypes "corvette", "frigate", "cruiser", "battleship", or
// "colony", ColonyTemplate(true) as "Lane Colonizer" realised against
// the least research allowing its hull and fixed parts (learn). The parts
// are a copy.
func ArenaDesign(name string) (Design, error) {
	var d Design
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "raider":
		d = FixtureRaider
	case "lancer":
		d = FixtureLancer
	case "corvette":
		d = FixtureCorvette
	case "frigate":
		d = FixtureFrigate
	case "cruiser":
		d = FixtureCruiser
	case "battleship":
		d = FixtureBattleship
	case "colony":
		t := ColonyTemplate(true)
		t.Name = "Lane Colonizer"
		r := NewResearch()
		learn(r, t.Hull.Tech())
		for _, p := range t.Parts {
			if !p.Best {
				learn(r, Gizmos[p.Gizmo].ResearchReq)
			}
		}
		var ok bool
		if d, ok = t.Realise(r); !ok {
			panic("the lane colony template does not realise on its own techs")
		}
	default:
		return Design{}, fmt.Errorf("no arena design %q (%s)", name, strings.Join(arenaNames, ", "))
	}
	d.Parts = slices.Clone(d.Parts)
	return d, nil
}

// learn grants t and, recursively, every prerequisite (NoTech: nothing).
func learn(r *Research, t Tech) {
	if t == NoTech || r.Known[t] {
		return
	}
	r.Grant(t)
	for _, p := range Techs[t].Requires {
		learn(r, p)
	}
}

// ParseArena reads an arena spec, "raider,raider:colony": side A's
// designs before the colon, side B's after, each a comma list of
// ArenaDesign names, neither side empty.
func ParseArena(spec string) (a, b []Design, err error) {
	sa, sb, ok := strings.Cut(spec, ":")
	if !ok {
		return nil, nil, fmt.Errorf("arena %q: no colon between side A and side B", spec)
	}
	side := func(s, label string) ([]Design, error) {
		var out []Design
		for _, f := range strings.Split(s, ",") {
			if f = strings.TrimSpace(f); f == "" {
				continue
			}
			d, err := ArenaDesign(f)
			if err != nil {
				return nil, fmt.Errorf("arena %q: %w", spec, err)
			}
			out = append(out, d)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("arena %q: side %s empty", spec, label)
		}
		return out, nil
	}
	if a, err = side(sa, "A"); err != nil {
		return nil, nil, err
	}
	if b, err = side(sb, "B"); err != nil {
		return nil, nil, err
	}
	return a, b, nil
}

// SpawnArena sets a fight at empire 0's first colony: side A Spawned in
// orbit of it, side B (empire 1) out of orbit at the FreeSpot of the
// endpoint of the star's first lane, as a lane arrival; empires 0 and 1
// at War. Errors (nothing done) without a second empire or a lane.
func SpawnArena(g *Game, a, b []Design) error {
	if len(g.Empires) < 2 {
		return fmt.Errorf("an arena needs two empires, the game has %d", len(g.Empires))
	}
	e := g.Empires[0]
	home := e.Colonies[0]
	c := g.Cluster
	lanes := c.Stars[home.Star].Lanes
	if len(lanes) == 0 {
		return fmt.Errorf("arena: star %d has no lane", home.Star)
	}
	_, end, ok := laneEnd(c, home.Star, c.Other(lanes[0], home.Star))
	if !ok {
		panic("a star's own lane is not adjacent")
	}
	at := c.Stars[home.Star].System.Lanes[end].Pos
	g.fixtureWar()
	for _, d := range a {
		e.Spawn(d, home.Star, home.PlanetIdx)
	}
	for _, d := range b {
		s := g.Empires[1].Spawn(d, home.Star, home.PlanetIdx)
		s.Orbit = -1
		s.Pos = FreeSpot(at, spaceAt(g.Empires, home.Star, s))
	}
	return nil
}

// fixtureWar puts empires 0 and 1 at War, met first (Contact), the
// Reports dropped; nothing when already at war. A loaded save's alliance
// is set back to Peace and 0's Friendship toward 1 forgotten first, or
// declareWar would refuse and leave the foes unable to fight.
func (g *Game) fixtureWar() {
	g.Contact(0, 1)
	if g.Empires[0].StanceWith(1) != War {
		g.setStance(0, 1, Peace)
		g.Empires[0].Relations[1].Friendship = 0
		g.declareWar(0, 1) // a fixture's war: not journalled
	}
}
