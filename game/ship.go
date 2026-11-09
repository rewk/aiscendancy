package game

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
)

// Hull is a ship's size (static.txt: Small, Medium, Large, Enormous;
// restree.txt's "Gigantic" unlock text is Enormous).
type Hull uint8

const (
	Small Hull = iota
	Medium
	Large
	Enormous
	NumHulls
)

var hullNames = [NumHulls]string{"Small", "Medium", "Large", "Enormous"}

// hullSlots: the part slots of each hull (the design screen's grid).
var hullSlots = [NumHulls]int{5, 10, 15, 25}

// HullCost is the industry of the bare hull. ASSUMPTION: not in the files;
// Small ≈ 70 from a 2005 walkthrough (a 207 ship whose parts sum 135), the
// others scaled with the slots.
var HullCost = [NumHulls]int{70, 140, 280, 500}

// HullStrength is the damage a bare hull takes before the ship is
// destroyed (the 2005 GameFAQs guide, ships tested without shields).
var HullStrength = [NumHulls]int{10, 20, 30, 40}

// hullTech: the research allowing each hull.
var hullTech = [NumHulls]Tech{InterplanetaryExploration, InterplanetaryExploration, AdvancedExploration, LargeScaleConstruction}

func (h Hull) String() string {
	if h >= NumHulls {
		return "none"
	}
	return hullNames[h]
}

// Slots is how many parts the hull takes.
func (h Hull) Slots() int { return hullSlots[h] }

// Cost is the bare hull's industry.
func (h Hull) Cost() int { return HullCost[h] }

// Tech is the research allowing the hull.
func (h Hull) Tech() Tech { return hullTech[h] }

// Strength is the hull's damage points, HullStrength.
func (h Hull) Strength() int { return HullStrength[h] }

// Movement constants. A sugar cube is one grid step of the system display,
// 0.1 of the disc radius (the system's world unit); a drive's Level is one
// move in sugar cubes (Design.Reach).
const (
	SugarCube = 0.1
	// RangeUnit is the Range of weapons and scanners in Pos units.
	// ASSUMPTION: gizmos.txt gives Range no unit, and in sugar
	// cubes every gun covered the system; a tenth of a cube makes the Mass
	// Barrage Gun's 25 a quarter radius.
	RangeUnit = SugarCube / 10
	// LaneSpeed is the cluster units crossed per day in a blue star lane
	// per unit of LaneStrength: 10 file units of the original's saves
	// (radius 1 = 440), measured (DOSBox: the ETA
	// on the entry day is ⌈L_file × (3 if red) / (10 × strength)⌉ on 16
	// lanes and drive sets, docs/rules.md).
	LaneSpeed = 10.0 / 440
	// RedLaneFactor is the days a red lane takes over a blue one of the
	// same length (measured).
	RedLaneFactor = 3
	// specialsDrawOnUse: ASSUMPTION, a design's power budget leaves out
	// the Specials, spent per use (else the guide's Small colony ship
	// would be refused).
	specialsDrawOnUse = true
)

// Design is a ship's plan: a hull and its parts.
type Design struct {
	Name  string
	Hull  Hull
	Parts []Gizmo
	Slots []int `json:",omitempty"` // each part's hull slot, nil packed (slots.go)
}

// Cost is the hull's industry plus every part's.
func (d Design) Cost() int {
	n := d.Hull.Cost()
	for _, g := range d.Parts {
		n += Gizmos[g].Industry
	}
	return n
}

func (d Design) count(match func(Gizmo) bool) int {
	n := 0
	for _, g := range d.Parts {
		if match(g) {
			n++
		}
	}
	return n
}

// Power is the generators' supply and what the other parts draw from it
// (Specials left out, see specialsDrawOnUse).
func (d Design) Power() (supply, demand int) {
	for _, g := range d.Parts {
		in := &Gizmos[g]
		switch {
		case in.Cat == Generator:
			supply += in.Power
		case in.Cat == Special && specialsDrawOnUse:
		default:
			demand += in.Power
		}
	}
	return supply, demand
}

// Valid reports why the design cannot be built (slots, research, power,
// an engine), nil when it can.
func (d Design) Valid(r *Research) error {
	if d.Hull >= NumHulls {
		return fmt.Errorf("design %q: no hull %d", d.Name, d.Hull)
	}
	if len(d.Parts) > d.Hull.Slots() {
		return fmt.Errorf("design %q: %d parts on a %s hull of %d slots", d.Name, len(d.Parts), d.Hull, d.Hull.Slots())
	}
	if err := checkSlots(d.Slots, len(d.Parts), d.Hull); err != nil {
		return fmt.Errorf("design %q: %w", d.Name, err)
	}
	if !r.Known[d.Hull.Tech()] {
		return fmt.Errorf("design %q: the %s hull needs %s", d.Name, d.Hull, d.Hull.Tech())
	}
	for _, g := range d.Parts {
		if g >= NumGizmos {
			return fmt.Errorf("design %q: no part %d", d.Name, g)
		}
		if !r.AllowsGizmo(g) {
			return fmt.Errorf("design %q: %s needs %s", d.Name, g, Gizmos[g].ResearchReq)
		}
	}
	if supply, demand := d.Power(); supply < demand {
		return fmt.Errorf("design %q: power %d for a demand of %d", d.Name, supply, demand)
	}
	if d.count(func(g Gizmo) bool { return Gizmos[g].Cat == Drive }) == 0 {
		return fmt.Errorf("design %q: no engine", d.Name)
	}
	return nil
}

// sum is the Levels and the Powers of the parts of cat, summed.
func (d Design) sum(cat GizmoCat) (level, power int) {
	for _, g := range d.Parts {
		if Gizmos[g].Cat == cat {
			level += Gizmos[g].Level
			power += Gizmos[g].Power
		}
	}
	return level, power
}

// Reach is the distance of ONE move: the drives' Levels in sugar cubes,
// all engines firing together (gizmos.txt, the manual). 0 with no engine.
func (d Design) Reach() float64 {
	lv, _ := d.sum(Drive)
	return float64(lv) * SugarCube
}

// MoveCost is the power one move costs, the drives' Power summed.
// ASSUMPTION: a move shorter than Reach costs it in full.
func (d Design) MoveCost() int {
	_, p := d.sum(Drive)
	return p
}

// Generators is the number of Generator parts: the ship's power in the
// rounds' sense (docs/rules.md "Combat rounds"); a move or a shot takes one.
func (d Design) Generators() int {
	return d.count(func(g Gizmo) bool { return Gizmos[g].Cat == Generator })
}

// PowerPerDay is the generators' supply, given back each day (the manual,
// battle.txt).
func (d Design) PowerPerDay() int {
	supply, _ := d.Power()
	return supply
}

// ShieldScore is the shields' Levels summed, the damage taken off every
// shot (the fan manual: 3 Ion Wraps = 3).
func (d Design) ShieldScore() int {
	lv, _ := d.sum(Shield)
	return lv
}

// ShieldDraw is the shields' Power summed, the energy their switch
// reserves while on (Starship.SetShields, Refill): taken once, not per
// combat round.
func (d Design) ShieldDraw() int {
	_, p := d.sum(Shield)
	return p
}

// LaneDrives sums the Levels of the Star Lane Drives (1) and Hyperdrives
// (2) aboard, 0 for none: "has a lane drive" (Check); speed is
// LaneStrength.
func (d Design) LaneDrives() int {
	n := 0
	for _, g := range d.Parts {
		if g == StarLaneDrive || g == StarLaneHyperdrive {
			n += Gizmos[g].Level
		}
	}
	return n
}

// LaneStrength is the lane speed factor (LaneDays): per drive kind, its
// Level × √(count aboard): n Drives √n, m Hyperdrives 2√m, summed (1D =
// 1, 2D = √2, 3D = √3, 1H = 2, 2H = 2√2, 1D+1H = 3, all measured). 0 for
// none.
func (d Design) LaneStrength() float64 {
	var n [2]int
	for _, g := range d.Parts {
		switch g {
		case StarLaneDrive:
			n[0]++
		case StarLaneHyperdrive:
			n[1]++
		}
	}
	return float64(Gizmos[StarLaneDrive].Level)*math.Sqrt(float64(n[0])) +
		float64(Gizmos[StarLaneHyperdrive].Level)*math.Sqrt(float64(n[1]))
}

// Colonizers counts the Colonizers on board.
func (d Design) Colonizers() int { return d.count(func(g Gizmo) bool { return g == Colonizer }) }

// Weapons counts the weapon parts aboard: a ship is armed with one.
func (d Design) Weapons() int { return d.count(func(g Gizmo) bool { return Gizmos[g].Cat == Weapon }) }

// InvasionModules counts the Invasion Modules aboard.
func (d Design) InvasionModules() int {
	return d.count(func(g Gizmo) bool { return g == InvasionModule })
}

// Berths is the population units d's Space Habitats carry, Special1 each.
func (d Design) Berths() int {
	return d.count(func(g Gizmo) bool { return g == SpaceHabitat }) * Gizmos[SpaceHabitat].Special1
}

// Berths is s's Design.Berths: a Space Habitat is never spent, so Aboard
// keeps them all.
func (s *Starship) Berths() int { return s.Design.Berths() }

// String is "<Hull>: part, part, part xN", runs of one part grouped.
func (d Design) String() string {
	var parts []string
	for i := 0; i < len(d.Parts); {
		j := i
		for j < len(d.Parts) && d.Parts[j] == d.Parts[i] {
			j++
		}
		s := d.Parts[i].String()
		if j-i > 1 {
			s += fmt.Sprintf(" x%d", j-i)
		}
		parts = append(parts, s)
		i = j
	}
	return d.Hull.String() + ": " + strings.Join(parts, ", ")
}

// rank reports a ranking strictly above b for a Best part of cat:
// generators by Power, scanners by Range, drives by Level then least
// Power then cheapest, the rest by Level.
func rank(cat GizmoCat) func(a, b *GizmoInfo) bool {
	switch cat {
	case Generator:
		return func(a, b *GizmoInfo) bool { return a.Power > b.Power }
	case Scanner:
		return func(a, b *GizmoInfo) bool { return a.Range > b.Range }
	case Drive:
		return func(a, b *GizmoInfo) bool {
			if a.Level != b.Level {
				return a.Level > b.Level
			}
			if a.Power != b.Power {
				return a.Power < b.Power
			}
			return a.Industry < b.Industry
		}
	}
	return func(a, b *GizmoInfo) bool { return a.Level > b.Level }
}

// best is the allowed gizmo of cat rank puts first, the higher index on
// ties; ok false when none.
func best(r *Research, cat GizmoCat) (Gizmo, bool) {
	better := rank(cat)
	found, ok := Gizmo(0), false
	for g := Gizmo(0); g < NumGizmos; g++ {
		if Gizmos[g].Cat != cat || !r.AllowsGizmo(g) {
			continue
		}
		if !ok || !better(&Gizmos[found], &Gizmos[g]) {
			found, ok = g, true
		}
	}
	return found, ok
}

// Part is one slot of a Template: a fixed gizmo, or (Best) the best
// gizmo of Cat the research allows when the template is realised.
type Part struct {
	Gizmo Gizmo
	Best  bool
	Cat   GizmoCat
}

// Template is a ship class: a hull and its parts, realised into a Design
// against the research of the day.
type Template struct {
	Name  string
	Hull  Hull
	Parts []Part
	Slots []int `json:",omitempty"` // each part's hull slot, nil packed (slots.go)
}

// Realise picks every Best part (best) and copies the fixed ones; Valid
// judges the result. ok false when a Best part finds no allowed gizmo:
// that part is left out, the rest realised (a view still shows a cost).
func (t Template) Realise(r *Research) (Design, bool) {
	d := Design{Name: t.Name, Hull: t.Hull, Parts: make([]Gizmo, 0, len(t.Parts))}
	if t.Slots != nil {
		d.Slots = make([]int, 0, len(t.Parts))
	}
	ok := true
	for i, p := range t.Parts {
		g := p.Gizmo
		if p.Best {
			var found bool
			if g, found = best(r, p.Cat); !found {
				ok = false
				continue
			}
		}
		d.Parts = append(d.Parts, g)
		if t.Slots != nil {
			d.Slots = append(d.Slots, t.Slots[i])
		}
	}
	return d, ok
}

// Fixed turns a Design into a Template of fixed parts (Cat set to the
// gizmo's).
func (d Design) Fixed() Template {
	t := Template{Name: d.Name, Hull: d.Hull, Parts: make([]Part, len(d.Parts)), Slots: slices.Clone(d.Slots)}
	for i, g := range d.Parts {
		t.Parts[i] = Part{Gizmo: g, Cat: Gizmos[g].Cat}
	}
	return t
}

// Aboard is s's Design with the spent cargo dropped: the first Cargo
// Colonizers and Invaders modules stay, the later ones go (a fresh Parts).
func (s *Starship) Aboard() Design {
	d := s.Design
	d.Parts = make([]Gizmo, 0, len(s.Design.Parts))
	if s.Design.Slots != nil {
		d.Slots = make([]int, 0, len(s.Design.Parts))
	}
	cargo, inv := s.Cargo, s.Invaders
	for i, g := range s.Design.Parts {
		switch g {
		case Colonizer:
			if cargo < 1 {
				continue
			}
			cargo--
		case InvasionModule:
			if inv < 1 {
				continue
			}
			inv--
		}
		d.Parts = append(d.Parts, g)
		if d.Slots != nil {
			d.Slots = append(d.Slots, s.Design.Slots[i])
		}
	}
	return d
}

// ColonyTemplate is the colony ship class, named "Colonizer": a Small
// hull, the best engine, the best generator, a Star Lane Drive when
// withLaneDrive, Colonizers in the remaining slots.
func ColonyTemplate(withLaneDrive bool) Template {
	t := Template{Name: "Colonizer", Hull: Small, Parts: []Part{{Best: true, Cat: Drive}, {Best: true, Cat: Generator}}}
	if withLaneDrive {
		t.Parts = append(t.Parts, Part{Gizmo: StarLaneDrive, Cat: Special})
	}
	for len(t.Parts) < t.Hull.Slots() {
		t.Parts = append(t.Parts, Part{Gizmo: Colonizer, Cat: Special})
	}
	return t
}

// ColonyShip is the policy's design helper, ColonyTemplate fitted on the
// smallest hull research allows (a cheap ship sails sooner, as the 2005
// guide's): the best engine, the best generator as often as the demand
// needs (powered), a Star Lane Drive when withLaneDrive and allowed,
// Colonizers in every remaining slot. ok false when a part or a hull is
// missing or no slot is left for a Colonizer.
func ColonyShip(r *Research, withLaneDrive bool) (Design, bool) {
	hull, ok := knownHull(r, false)
	if !ok {
		return Design{}, false
	}
	return ColonyShipOn(r, hull, withLaneDrive, hull.Slots())
}

// ColonyShipOn is ColonyShip on hull, at most most Colonizers (the bot's
// larger colony ships). ok false as ColonyShip's, the hull unknown or
// most < 1 too.
func ColonyShipOn(r *Research, hull Hull, withLaneDrive bool, most int) (Design, bool) {
	okHull := hull < NumHulls && r.Known[hull.Tech()] && most >= 1
	lane := withLaneDrive && r.AllowsGizmo(StarLaneDrive)
	t, okT := ColonyTemplate(lane).Realise(r)
	if !okHull || !okT || !r.AllowsGizmo(Colonizer) {
		return Design{}, false
	}
	engine, gen := t.Parts[0], t.Parts[1]
	d := Design{Name: t.Name, Hull: hull, Parts: []Gizmo{engine}}
	if lane {
		d.Parts = append(d.Parts, StarLaneDrive)
	}
	// The demand before any Colonizer is final: Specials draw on use.
	_, demand := d.Power()
	if !specialsDrawOnUse {
		demand += Gizmos[Colonizer].Power // at least one; more are checked by the caller's Valid
	}
	d.Parts = powered(engine, gen, demand, d.Parts[1:])
	free := min(most, hull.Slots()-len(d.Parts))
	if free < 1 {
		return Design{}, false
	}
	for range free {
		d.Parts = append(d.Parts, Colonizer)
	}
	return d, true
}

// HabitatShipOn is the policy's population transport on hull, built as
// ColonyShipOn: the best engine, the best generator as often as the
// demand needs (powered), a Star Lane Drive when withLaneDrive and
// allowed, Space Habitats in every remaining slot. ok false when the hull
// is unknown, an engine, a generator or the Space Habitat is not allowed,
// or no slot is left for a Habitat.
func HabitatShipOn(r *Research, hull Hull, withLaneDrive bool) (Design, bool) {
	okHull := hull < NumHulls && r.Known[hull.Tech()]
	engine, okE := best(r, Drive)
	gen, okG := best(r, Generator)
	if !okHull || !okE || !okG || !r.AllowsGizmo(SpaceHabitat) {
		return Design{}, false
	}
	d := Design{Name: HabitatName, Hull: hull, Parts: []Gizmo{engine}}
	if withLaneDrive && r.AllowsGizmo(StarLaneDrive) {
		d.Parts = append(d.Parts, StarLaneDrive)
	}
	// The demand before any Habitat is final: it draws no power.
	_, demand := d.Power()
	d.Parts = powered(engine, gen, demand, d.Parts[1:])
	free := hull.Slots() - len(d.Parts)
	if free < 1 {
		return Design{}, false
	}
	for range free {
		d.Parts = append(d.Parts, SpaceHabitat)
	}
	return d, true
}

// knownHull is the smallest hull research allows, the largest when
// largest; ok false when none.
func knownHull(r *Research, largest bool) (h Hull, ok bool) {
	for k := Hull(0); k < NumHulls; k++ {
		if r.Known[k.Tech()] {
			h, ok = k, true
			if !largest {
				break
			}
		}
	}
	return h, ok
}

// powered is engine, gen ⌈demand / gen's supply⌉ times (at least once),
// then rest (a fresh slice).
func powered(engine, gen Gizmo, demand int, rest []Gizmo) []Gizmo {
	gens := max(1, (demand+Gizmos[gen].Power-1)/Gizmos[gen].Power)
	parts := make([]Gizmo, 0, 1+gens+len(rest))
	parts = append(parts, engine)
	for range gens {
		parts = append(parts, gen)
	}
	return append(parts, rest...)
}

// Warship is the policy's armed design helper (the bots' choice, not a
// rule), WarshipOn on the LARGEST hull research allows.
func Warship(r *Research, withLaneDrive bool) (Design, bool) {
	hull, ok := knownHull(r, true)
	if !ok {
		return Design{}, false
	}
	return WarshipOn(r, hull, withLaneDrive)
}

// WarshipOn fills hull: the best engine, a Star Lane Drive when
// withLaneDrive and allowed, on a Medium or larger hull the best Scanner
// when one is allowed (its Scan reads the foe's hull), then one part at a
// time the best weapon and the best shield alternating, weapon first,
// each with the generators its draw needs (powered); the first part that
// no longer fits stops the fill, a shield giving way to a weapon first.
// ok false when the hull or a part is not allowed, or no weapon fits.
func WarshipOn(r *Research, hull Hull, withLaneDrive bool) (Design, bool) {
	return WarshipWith(r, hull, withLaneDrive, nil)
}

// WarshipWith is WarshipOn with extra after the scanner, before the arms
// (the bot's specials).
func WarshipWith(r *Research, hull Hull, withLaneDrive bool, extra []Gizmo) (Design, bool) {
	okHull := hull < NumHulls && r.Known[hull.Tech()]
	engine, okE := best(r, Drive)
	gen, okG := best(r, Generator)
	weapon, okW := best(r, Weapon)
	if !okHull || !okE || !okG || !okW {
		return Design{}, false
	}
	shield, okS := best(r, Shield)
	var head []Gizmo // what follows the generators: the lane drive, the scanner, the arms
	if withLaneDrive && r.AllowsGizmo(StarLaneDrive) {
		head = append(head, StarLaneDrive)
	}
	if sc, ok := best(r, Scanner); ok && hull > Small {
		head = append(head, sc)
	}
	head = append(head, extra...)
	// fit is the design with g added after arms, ok false when invalid.
	fit := func(arms []Gizmo, g Gizmo) (Design, bool) {
		rest := append(slices.Clone(arms), g)
		_, demand := Design{Hull: hull, Parts: append([]Gizmo{engine}, rest...)}.Power()
		d := Design{Name: "Warship", Hull: hull, Parts: powered(engine, gen, demand, rest)}
		return d, len(d.Parts) <= hull.Slots() && d.Valid(r) == nil
	}
	var d Design
	arms, wantShield, armed := head, false, false
	for {
		g := weapon
		if wantShield && okS {
			g = shield
		}
		next, ok := fit(arms, g)
		if !ok && g == shield {
			g = weapon
			next, ok = fit(arms, g)
		}
		if !ok {
			break
		}
		d, arms = next, append(slices.Clone(arms), g)
		armed = armed || g == weapon
		wantShield = g == weapon
	}
	return d, armed
}

// ScoutName names ScoutShip's design: the policy tells its scouts by it,
// as their parts change (LosePart).
const ScoutName = "Scout"

// HabitatName names HabitatShipOn's design: the policy's population
// transport.
const HabitatName = "Habitat"

// ScoutShip is the policy's explorer, ScoutName: on the smallest hull
// research allows, the best engine, a Star Lane Drive, the best scanner
// when one is allowed and fits, the generators their draw needs
// (powered). ok false without a hull, an engine, a generator or the lane
// drive.
func ScoutShip(r *Research) (Design, bool) {
	hull, okH := knownHull(r, false)
	engine, okE := best(r, Drive)
	gen, okG := best(r, Generator)
	if !okH || !okE || !okG || !r.AllowsGizmo(StarLaneDrive) {
		return Design{}, false
	}
	fit := func(rest []Gizmo) (Design, bool) {
		_, demand := Design{Hull: hull, Parts: append([]Gizmo{engine}, rest...)}.Power()
		d := Design{Name: ScoutName, Hull: hull, Parts: powered(engine, gen, demand, rest)}
		return d, len(d.Parts) <= hull.Slots() && d.Valid(r) == nil
	}
	if sc, ok := best(r, Scanner); ok {
		if d, ok := fit([]Gizmo{StarLaneDrive, sc}); ok {
			return d, true
		}
	}
	return fit([]Gizmo{StarLaneDrive})
}

// InvasionShip is the policy's invasion design helper, "Invader": the best
// engine, the generators (powered), a Star Lane Drive when withLaneDrive
// and allowed, then modules Invasion Modules, on the smallest allowed hull
// holding them. ok false when modules < 1, a part is not allowed or no
// hull has the slots.
func InvasionShip(r *Research, withLaneDrive bool, modules int) (Design, bool) {
	engine, okE := best(r, Drive)
	gen, okG := best(r, Generator)
	if modules < 1 || !okE || !okG || !r.AllowsGizmo(InvasionModule) {
		return Design{}, false
	}
	var rest []Gizmo
	if withLaneDrive && r.AllowsGizmo(StarLaneDrive) {
		rest = append(rest, StarLaneDrive)
	}
	for range modules {
		rest = append(rest, InvasionModule)
	}
	_, demand := Design{Parts: append([]Gizmo{engine}, rest...)}.Power()
	parts := powered(engine, gen, demand, rest)
	for h := Hull(0); h < NumHulls; h++ {
		if r.Known[h.Tech()] && len(parts) <= h.Slots() {
			return Design{Name: "Invader", Hull: h, Parts: parts}, true
		}
	}
	return Design{}, false
}

// OrderKind is what a ship has been told to do.
type OrderKind uint8

const (
	NoOrder    OrderKind = iota
	GoToPlanet           // fly to Order.Planet of the current system and orbit it
	GoToStar             // fly to the lane towards Order.Star (adjacent) and through it
	Colonise             // found a colony on the orbited planet (Empire.EndDay)
	GoToPoint            // fly to Order.Pos in the current system and stop there, in space
	Invade               // launch every Invasion Module at the orbited planet (Game.EndDay: Game.Invade)
	GoToSystem           // fly to Order.Star, any star, lane after lane along Empire.Route (route.go)
)

func (k OrderKind) String() string {
	if k > GoToSystem {
		return fmt.Sprintf("order %d", k)
	}
	return [...]string{"none", "planet", "star", "colonise", "point", "invade", "system"}[k]
}

// SystemReach is the radius (the grid disc's is 1) within which a
// GoToPoint order may lie: lanes sit within 1.15, bodies below 1.2. Ours.
const SystemReach = 1.5

// ShipGap is the exclusion radius of a ship in space (InSpace): a hop
// never ends within it of another (Hop), a ship placed in space goes to
// FreeSpot. ASSUMPTION: ours. Ships in orbit are no obstacle.
const ShipGap = 0.05

// PlanetClear is how far from a planet's Pos a ship stopped short of its
// barred orbit stands, outside the views' orbit ring (≤ 0.16). ASSUMPTION.
const PlanetClear = 2 * SugarCube

// planetClear is p pushed back PlanetClear towards from (+X when from is p).
func planetClear(from, p Vec3) Vec3 {
	d := dist(from, p)
	if d < 1e-12 {
		return Vec3{p.X + PlanetClear, p.Y, p.Z}
	}
	return lerp(p, from, PlanetClear/d)
}

// LeavePoint is the GoToPoint target of the views' Leave Orbit (ours):
// PlanetClear from the planet towards its star, at the origin.
func LeavePoint(c *Cluster, star, planet int) Vec3 {
	return planetClear(Vec3{}, c.Stars[star].System.Planets[planet].Pos)
}

// Order is a ship's standing order; Planet, Star and Pos apply per Kind.
type Order struct {
	Kind   OrderKind
	Planet int
	Star   int
	Pos    Vec3
}

// PartCount is a group of identical parts: the gizmo and how many.
type PartCount struct {
	Gizmo Gizmo
	N     int
}

// Counts groups the parts as a ship screen lists them, in countRank order,
// ties by first appearance.
func (d Design) Counts() []PartCount {
	var out []PartCount
next:
	for _, g := range d.Parts {
		for i := range out {
			if out[i].Gizmo == g {
				out[i].N++
				continue next
			}
		}
		out = append(out, PartCount{g, 1})
	}
	slices.SortStableFunc(out, func(a, b PartCount) int { return cmp.Compare(countRank(a.Gizmo), countRank(b.Gizmo)) })
	return out
}

// countRank is Counts' order: Colonizer 0, Invasion Module 1, the other
// Specials 2+index, then 100 per category from Weapon on.
func countRank(g Gizmo) int {
	switch {
	case g == Colonizer:
		return 0
	case g == InvasionModule:
		return 1
	case Gizmos[g].Cat == Special:
		return 2 + int(g)
	}
	return 100 * (1 + int(Gizmos[g].Cat))
}

// Starship is a ship in space, in orbit or in a lane (Ship is the orbital
// item a colony builds it as).
type Starship struct {
	Name   string
	Design Design
	Star   int  // the star system it is in (the one it left, while in a lane)
	Pos    Vec3 // position in that system, or the lane entry point while travelling
	Orbit  int  // planet index it orbits, -1 in space
	Lane   int  // -1, or the cluster lane it is travelling, exiting on day Arrive
	Arrive int
	Cargo  int // Colonizers left (the Design keeps the list)
	// Invaders is the Invasion Modules left, as Cargo; Game.Invade spends
	// them all at once.
	Invaders int
	// Pop is the population units aboard, ≤ Berths (Empire.Load, Unload).
	Pop   int `json:",omitempty"`
	Order Order
	// Power is the ENERGY left today, refilled by Refill: a move spends
	// MoveCost, a shot its weapon's Power (the rounds' power is Generators);
	// while Shields is on their ShieldDraw is held off it.
	Power int
	// Species is the owner's: its Traits bend the lane speed and MaxHull.
	Species Species
	Hull    int // hull points left (MaxHull at Spawn); destroyed at 0 or below
	// Dealt is the damage ledger, by empire Index (grown on demand): the
	// hull points that empire's shots took off this ship (it saw them
	// land), ships' and planets' alike. Each stays ≤ MaxHull − Hull: every
	// heal calls healed.
	Dealt []int `json:",omitempty"`
	// Shields is the owner's switch (on at Spawn): while on, ShieldDraw
	// is already off Power (the reservation, SetShields and Refill).
	Shields bool
	// Shots fired today per part, parallel to Design.Parts: a weapon's
	// shots, a special's uses.
	Shots []int
	// The day flags, cleared by Refill: Boosted (the Accutron: WeaponRange
	// doubles), Invulnerable (Round lands nothing), TiedDown (Hop moves
	// nothing), Scrambled (Arm refuses, Round drops its armed shots),
	// ShieldsDown (dark: Absorb 0, Shields and their reservation kept; a
	// Shield Blaster, or a shot the energy could not pay the draw for,
	// takeShot), Sapped (the Shevar's sapAlienPower took the energy and
	// the reservation with it: dark, and off refunds nothing).
	Boosted      bool
	Invulnerable bool
	TiedDown     bool
	Scrambled    bool
	ShieldsDown  bool
	Sapped       bool
	// Weapon is the part the owner chose to fire (ReadyWeapon), −1 for
	// the first that can.
	Weapon int
	// Armed is this round's armed shots (Empire.Arm), resolved and cleared
	// by Game.Round and Refill. Not saved (a Shot's target is a pointer).
	Armed []Shot `json:"-"`
}

// Usable reports whether part is a Special usable today, with the
// action's name ("colonise", "invade": in orbit with cargo left; "use":
// specialReady), else false and why. Empire.Order's and the special's own
// refusals are not seen here.
func (s *Starship) Usable(part int) (bool, string) {
	if part < 0 || part >= len(s.Design.Parts) {
		return false, fmt.Sprintf("no part %d", part)
	}
	g := s.Design.Parts[part]
	if Gizmos[g].Cat != Special {
		return false, Gizmos[g].Name + " is not a special"
	}
	if g != Colonizer && g != InvasionModule {
		if err := s.specialReady(part); err != nil {
			return false, err.Error()
		}
		return true, "use"
	}
	switch {
	case s.Orbit < 0:
		return false, "not in orbit"
	case g == InvasionModule && s.Invaders < 1:
		return false, "no invasion module left"
	case g == InvasionModule:
		return true, "invade"
	case s.Cargo < 1:
		return false, "no colonizer left"
	}
	return true, "colonise"
}

// MaxHull is the hull's Strength times the owner's HullFactor.
func (s *Starship) MaxHull() int { return s.Design.Hull.Strength() * s.Species.Traits().HullFactor() }

// dealt books d hull points of empire e's shot in s's Dealt ledger.
func (s *Starship) dealt(e, d int) {
	if d <= 0 || e < 0 {
		return
	}
	if e >= len(s.Dealt) {
		s.Dealt = append(s.Dealt, make([]int, e+1-len(s.Dealt))...)
	}
	s.Dealt[e] += d
}

// healed clamps every Dealt entry to MaxHull − Hull after the hull went
// up: an empire cannot have seen more damage than is left.
func (s *Starship) healed() {
	gone := max(0, s.MaxHull()-s.Hull)
	for e, d := range s.Dealt {
		s.Dealt[e] = min(d, gone)
	}
}

// Absorb is the damage the shields take off a shot: ShieldScore while
// Shields is on, else 0; 0 too while ShieldsDown (dark: the Shield
// Blaster, or knocked down by a shot, takeShot).
func (s *Starship) Absorb() int {
	if !s.Shields || s.ShieldsDown {
		return 0
	}
	return s.Design.ShieldScore()
}

// ShieldHits is the hits s's shields can still take before falling, as
// takeShot pays them: ShieldDraw off Power each, the one short of it
// knocking them down; 0 while Absorb is 0 (off, dark, no shield part);
// NoShieldFall when they draw nothing (a Wave Scatterer alone, Power 0):
// never knocked down.
func (s *Starship) ShieldHits() int {
	d := s.Design.ShieldDraw()
	switch {
	case s.Absorb() == 0:
		return 0
	case d <= 0:
		return NoShieldFall
	}
	return s.Power / d
}

// NoShieldFall is ShieldHits for shields no hit can knock down.
const NoShieldFall = -1

// takeShot is the hull damage a weapon's shot of level does on s, its
// cost to the shields taken (measured in the original, test saves 12-16
// day 1488): shields up (Absorb > 0) pay ShieldDraw off Power for each
// shot they take, the reservation spent and held again at once, and take
// Absorb off it (shotDamage); with Power short of the draw the shot knocks
// them down instead (ShieldsDown, Power and the reservation kept) and does
// no hull damage; dark or off, the full level. The damage is the
// caller's to land (Round's pending).
func (s *Starship) takeShot(level int) int {
	a := s.Absorb()
	switch {
	case a == 0:
		return level
	case s.Power < s.Design.ShieldDraw():
		s.ShieldsDown = true
		return 0
	}
	s.Power -= s.Design.ShieldDraw()
	return shotDamage(level, a)
}

// SetShields switches s's shields, combat or not (measured in the
// original): on takes ShieldDraw off Power at once, refused when Power
// is short of it; off gives it back in full, dark (ShieldsDown) too,
// the reservation being held while dark (measured), except Sapped (the
// Shevar took it). Setting the state it has is a no-op.
func (s *Starship) SetShields(on bool) error {
	if s.Shields == on {
		return nil
	}
	d := s.Design.ShieldDraw()
	if !on {
		if !s.Sapped {
			s.Power += d
		}
		s.Shields, s.Sapped = false, false
		return nil
	}
	if s.Power < d {
		return fmt.Errorf("%s: not enough energy for the shields (%d of %d)", s.Name, s.Power, d)
	}
	s.Power -= d
	s.Shields = true
	return nil
}

// Refill sets the day's energy to PowerPerDay (not added to what is
// left, the generators do not store), then takes the reservation of
// shields still on: when PowerPerDay cannot carry it they are switched
// off. It zeroes Shots (reallocated when not parallel to Parts), drops
// Armed and ends the day flags.
func (s *Starship) Refill() {
	s.Power = s.Design.PowerPerDay()
	if d := s.Design.ShieldDraw(); s.Shields && d <= s.Power {
		s.Power -= d
	} else {
		s.Shields = false
	}
	s.Armed = nil
	s.Boosted, s.Invulnerable = false, false
	s.TiedDown, s.Scrambled, s.ShieldsDown, s.Sapped = false, false, false, false
	if len(s.Shots) != len(s.Design.Parts) {
		s.Shots = make([]int, len(s.Design.Parts))
	} else {
		clear(s.Shots)
	}
}

// LaneDays is ⌈length × (RedLaneFactor if red) / (LaneSpeed × strength ×
// tr.LaneFactor())⌉, at least 1: a cluster lane's days at a Design's
// LaneStrength. It panics for strength ≤ 0 (no lane drive).
func LaneDays(c *Cluster, lane int, strength float64, tr Traits) int {
	if !(strength > 0) {
		panic(fmt.Sprintf("lane days at lane strength %g", strength))
	}
	l := c.Lanes[lane]
	d := c.Dist(l.A, l.B)
	if l.Red {
		d *= RedLaneFactor
	}
	return max(1, int(math.Ceil(d/(LaneSpeed*strength*tr.LaneFactor())-1e-9)))
}

// LaneProgress is how far along its lane s is on day, 0 (at Star, the
// star it left) to 1 (on Arrive): 1 − days to Arrive over the lane's
// LaneDays, clamped; 0.5 without a lane drive.
func (s *Starship) LaneProgress(c *Cluster, day int) float64 {
	strength := s.Design.LaneStrength()
	if strength <= 0 {
		return 0.5
	}
	n := LaneDays(c, s.Lane, strength, s.Species.Traits())
	return min(1, max(0, 1-float64(s.Arrive-day)/float64(n)))
}

// laneEnd is the cluster lane from star to to and its endpoint index in
// star's System.Lanes (Star.Lanes is in endpoint order); ok false when
// they are not adjacent.
func laneEnd(c *Cluster, star, to int) (lane, end int, ok bool) {
	for k, li := range c.Stars[star].Lanes {
		if c.Other(li, star) == to {
			return li, k, true
		}
	}
	return -1, -1, false
}

// hopEnd is target (reached) when within reach of a, else reach along
// the way.
func hopEnd(a, target Vec3, reach float64) (end Vec3, reached bool) {
	d := dist(a, target)
	if d <= reach+1e-9 {
		return target, true
	}
	if reach <= 0 {
		return a, false
	}
	return lerp(a, target, reach/d), false
}

// Hop is ONE move towards the order's target on day (the days run so
// far), the unit the system display animates; moved reports power spent,
// arrived the target reached or the lane entered.
//
// Nothing in a lane, TiedDown, on a non-move order, or below MoveCost (the
// order kept); an order Check refuses is dropped; GoToPlanet already in
// that orbit arrives free. Else the move goes Reach towards the target; a
// move ending in space is shortened to keep ShipGap from obstacles
// (clearEnd), and one shortened to nothing costs nothing. An occupied
// GoToPoint target is replaced by its FreeSpot. MoveCost is paid
// (payMove), the ship leaves its orbit; GoToPlanet enters the orbit,
// GoToPoint stops (both clear the order), GoToStar enters the lane (Arrive
// = day + LaneDays). A GoToPlanet ending within PlanetClear of a barred
// planet (barred, nil bars none) BOUNCES (the original: shields
// down first): it stops at FreeSpot(planetClear), its order kept, and
// holds there free while it stands bounced already.
func (s *Starship) Hop(c *Cluster, day int, obstacles []Vec3, barred func(star, pl int) bool) (moved, arrived bool) {
	if s.Lane >= 0 || s.TiedDown {
		return false, false
	}
	switch s.Order.Kind {
	case GoToPlanet, GoToStar, GoToPoint:
	default:
		return false, false
	}
	if s.Check(c, s.Order) != nil {
		s.Order = Order{}
		return false, false
	}
	sys := c.Stars[s.Star].System
	if s.Order.Kind == GoToPlanet && s.Orbit == s.Order.Planet {
		s.Order = Order{}
		return false, true
	}
	cost := s.Design.MoveCost()
	if s.Power < cost {
		return false, false
	}
	var target Vec3
	lane := -1
	switch s.Order.Kind {
	case GoToPlanet:
		target = sys.Planets[s.Order.Planet].Pos
	case GoToStar:
		var end int
		lane, end, _ = laneEnd(c, s.Star, s.Order.Star)
		target = sys.Lanes[end].Pos
	case GoToPoint:
		target = s.Order.Pos
	}
	end, reached := hopEnd(s.Pos, target, s.Design.Reach())
	if s.Order.Kind == GoToPlanet && dist(end, target) < PlanetClear && barred != nil && barred(s.Star, s.Order.Planet) {
		// Bounced already: hold, or each day's clear point would walk it
		// round the planet.
		if d := dist(s.Pos, target); d >= PlanetClear-1e-9 && d < PlanetClear+ShipGap {
			return false, false
		}
		end = FreeSpot(planetClear(s.Pos, target), obstacles)
		if dist(s.Pos, end) < 1e-9 {
			return false, false
		}
		s.payMove(cost)
		s.Orbit, s.Pos = -1, end
		return true, false
	}
	if !reached || s.Order.Kind == GoToPoint {
		e := clearEnd(s.Pos, end, obstacles)
		if e != end && s.Order.Kind == GoToPoint && crowds(target, obstacles) {
			// An occupied point can never be reached: head for its
			// FreeSpot. A free one merely blocked midway goes on tomorrow.
			end, reached = hopEnd(s.Pos, FreeSpot(target, obstacles), s.Design.Reach())
			e = clearEnd(s.Pos, end, obstacles)
		}
		if e != end {
			if dist(s.Pos, e) < 1e-9 {
				return false, false
			}
			end, reached = e, false
		}
	}
	s.payMove(cost)
	s.Orbit = -1
	s.Pos = end
	if !reached {
		return true, false
	}
	switch s.Order.Kind {
	case GoToPlanet:
		s.Orbit = s.Order.Planet
		s.Order = Order{}
	case GoToStar:
		s.Lane = lane
		s.Arrive = day + LaneDays(c, lane, s.Design.LaneStrength(), s.Species.Traits())
	case GoToPoint:
		s.Order = Order{}
	}
	return true, true
}

// payMove pays a Hop's MoveCost, called before the ship leaves its orbit:
// leaving one with a Gyro-Inductor aboard is free, whatever their number
// (gizhelp: it "creates power" when the ship leaves orbit; ASSUMPTION, no
// number in the table).
func (s *Starship) payMove(cost int) {
	s.Power -= cost
	if s.Orbit >= 0 && s.Design.count(func(g Gizmo) bool { return g == GyroInductor }) > 0 {
		s.Power += cost
	}
}

// Step closes day−1 for the ship (Empire.EndDay) and reports an arrival
// (orbit, point, or out of a lane): the power left goes on the order
// first (an order given off the display moves the same day), then Refill,
// then the new day's power proceeds Hop after Hop (the original's Auto);
// the rest is the player's. In a lane the ship waits for Arrive, then
// appears at the far endpoint (its FreeSpot when crowded), order done; a
// lane entry is not an arrival. near(star) is the other ships' Pos in
// space at star (nil: none), read once for the hops and once at a lane
// exit; barred is Hop's.
func (s *Starship) Step(c *Cluster, day int, near func(star int) []Vec3, barred func(star, pl int) bool) (arrived bool) {
	var obs []Vec3
	if near != nil && s.Lane < 0 {
		switch s.Order.Kind {
		case GoToPlanet, GoToStar, GoToPoint:
			obs = near(s.Star)
		}
	}
	if s.Lane < 0 {
		arrived = s.proceed(c, day-1, obs, barred)
	}
	s.Refill()
	if s.Lane >= 0 {
		if day < s.Arrive {
			return false
		}
		far := c.Other(s.Lane, s.Star)
		_, end, _ := laneEnd(c, far, s.Star)
		s.Star, s.Lane, s.Orbit = far, -1, -1
		s.Pos = c.Stars[far].System.Lanes[end].Pos
		if near != nil {
			s.Pos = FreeSpot(s.Pos, near(far))
		}
		s.Order = Order{}
		return true
	}
	return arrived || s.proceed(c, day, obs, barred)
}

// proceed Hops on day until the order completes or the power runs out,
// reporting an arrival in orbit or on a point.
func (s *Starship) proceed(c *Cluster, day int, obstacles []Vec3, barred func(star, pl int) bool) (arrived bool) {
	for {
		moved, hit := s.Hop(c, day, obstacles, barred)
		if hit {
			return s.Lane < 0
		}
		if !moved {
			return false
		}
	}
}

// Check reports why the ship cannot be given o, nil when it can: a bad
// target, no lane drive, a ship that can never move (too little power
// LEFT today is no refusal), Colonise or Invade out of orbit or without
// cargo; in a lane, any order but NoOrder.
func (s *Starship) Check(c *Cluster, o Order) error {
	if o.Kind == NoOrder {
		return nil
	}
	if s.Lane >= 0 {
		return errors.New("in a star lane")
	}
	switch o.Kind {
	case GoToPlanet:
		if o.Planet < 0 || o.Planet >= len(c.Stars[s.Star].System.Planets) {
			return errors.New("no such planet")
		}
	case GoToStar:
		if _, _, ok := laneEnd(c, s.Star, o.Star); !ok {
			if o.Star < 0 || o.Star >= len(c.Stars) {
				return fmt.Errorf("no star %d", o.Star)
			}
			return fmt.Errorf("%s is not adjacent", c.Stars[o.Star].Name)
		}
		if s.Design.LaneDrives() < 1 {
			return errors.New("no star lane drive")
		}
	case GoToSystem: // the route is the Empire's (Order, Route)
		if o.Star < 0 || o.Star >= len(c.Stars) {
			return fmt.Errorf("no star %d", o.Star)
		}
		if s.Design.LaneDrives() < 1 {
			return errors.New("no star lane drive")
		}
	case Colonise:
		if s.Orbit < 0 {
			return errors.New("not in orbit")
		}
		if s.Cargo < 1 {
			return errors.New("no colonizer")
		}
	case Invade:
		if s.Orbit < 0 {
			return errors.New("not in orbit")
		}
		if s.Invaders < 1 {
			return errors.New("no invasion module")
		}
	case GoToPoint:
		if dist(o.Pos, Vec3{}) > SystemReach {
			return errors.New("out of the system")
		}
	default:
		return fmt.Errorf("no order kind %d", o.Kind)
	}
	if o.Kind != Colonise && o.Kind != Invade {
		if s.Design.Reach() == 0 {
			return errors.New("no engine")
		}
		if s.Design.MoveCost() > s.Design.PowerPerDay() {
			return errors.New("not enough power to move")
		}
	}
	return nil
}

// ETA is the days from day until the order completes, simulated on a
// copy: Arrive − day in a lane; else 0 when today's power arrives, then
// d for the Step on day+d that does (counting down by one per EndDay).
// The copy flies alone (no ShipGap, no guards). ok false for a non-move
// order, one Check refuses, or past 1000 days.
func (s *Starship) ETA(c *Cluster, day int) (days int, ok bool) {
	if s.Lane >= 0 {
		return max(0, s.Arrive-day), true
	}
	switch s.Order.Kind {
	case GoToPlanet, GoToStar, GoToPoint:
	default:
		return 0, false
	}
	if s.Check(c, s.Order) != nil {
		return 0, false
	}
	cp := *s
	cp.Shots = nil // the copy's Refill must not zero the ship's own
	for {
		moved, arrived := cp.Hop(c, day, nil, nil)
		if arrived && cp.Lane < 0 {
			return 0, true
		}
		if arrived || !moved {
			break
		}
	}
	for d := 1; d <= 1000; d++ {
		if cp.Step(c, day+d, nil, nil) {
			return d, true
		}
	}
	return 0, false
}
