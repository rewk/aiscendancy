package game

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/rewk/aiscendancy/journal"
)

// The colony day's unmeasured rules, named so the fuzzer can move them.
const (
	// ASSUMPTION: industry past a project's cost is lost, like research's
	// (false: credited to the next project).
	industrySurplusLost = true
	// ASSUMPTION: industry made with no project running is lost (false:
	// credited to the next project).
	idleIndustryLost = true
	// GrowthCost is the prosperity banked per population unit: measured
	// in the game, 50 days per unit at prosperity 1.
	GrowthCost = 50
	// ASSUMPTION: a fresh colony holds StartPop units.
	StartPop = 2
)

// The population capacity, read in the original's saves (planet record
// +74, exact on 1560 bare planets): CapacityBase + N/CapacityPerSquares,
// N the non-black squares; the structures add their MaxPop, a Lush Growth
// Bomb LushBonus.
const (
	CapacityBase       = 5
	CapacityPerSquares = 3
	LushBonus          = 10
)

// AutomationExtra is an Automation's cost on top of its target's Cost
// (measured in the game; planitem.txt's Cost 1 is not used).
const AutomationExtra = 110

// Why CanStart refuses, for callers that branch on the reason.
var (
	ErrProjectRunning = errors.New("a project is running")
	ErrNoFreePop      = errors.New("no free population")
	ErrNotBuildable   = errors.New("not buildable")
	ErrNotModelled    = errors.New("not modelled yet")
)

// Colony is a settled planet: its people, its running project, its day.
type Colony struct {
	Planet  *Planet  `json:"-"` // relinked from Star, PlanetIdx (Empire.Relink)
	Pop     int      // population units
	Growth  int      // prosperity banked toward the next unit, 0..GrowthCost
	Project *Project // nil when idle; an ongoing one (Item.Ongoing) gives way to any start
	// Queue is the constructions waiting for the project (queue.go), the
	// head first; started by Next.
	Queue []Queued `json:",omitempty"`
	Carry int      // industry kept for the next project (the ASSUMPTIONs)
	// Star and PlanetIdx place the planet in the Empire's Cluster (index
	// in that star's System.Planets); -1 outside a cluster.
	Star, PlanetIdx int
	// OrbitShots is the shots each orbit slot's weapon fired today, zeroed
	// at the close.
	OrbitShots [OrbitSlots]int
	// Armed is this round's armed orbital shots (Empire.ArmPlanet,
	// ArmPlanetAt; Shot.Part the slot), cleared by Game.Round and the
	// close. Not saved.
	Armed []Shot `json:"-"`
	// Tractors is the Tractor Beam pulls used this round (tractor.go),
	// reset by Round and the close. Saved, unlike Armed: a pull moves its
	// ship at once, so a reload must not give it back.
	Tractors int `json:",omitempty"`
	// BlockedBy is the Blockader at the last close plus one, 0 none
	// (Game.blockadeNews: a change is news). Saved.
	BlockedBy int `json:",omitempty"`
	// journal and owner are the owning Empire's Writer and the Empire
	// itself (journal.go: adopt). Not saved; nil journal records nothing.
	journal *journal.Writer
	owner   *Empire
	// blockaded is Blockaded at the day's start (Game.blockades), read
	// and cleared by EndDay. Not saved.
	blockaded bool
}

// Traits are the owner's species' (they bend CanStart and the
// completions: the Orfa build on black); everyone's default for a
// colony never adopted.
func (c *Colony) Traits() Traits {
	if c.owner == nil {
		return baseTraits{}
	}
	return c.owner.Species.Traits()
}

// NewLoneColony is a colony on p outside any game (the sim's), owned by
// a bare Empire of sp for its Traits.
func NewLoneColony(p *Planet, sp Species) *Colony {
	c := NewColony(p)
	c.adopt(&Empire{Species: sp})
	return c
}

// Project is a structure under construction on a square or an orbit slot,
// or the refit of a ship in orbit (Refit set, Empire.Refit).
type Project struct {
	Item     Item
	Cell     Cell // surface target, when !Orbital
	Slot     int  // orbit target, when Orbital; -1 for a refit
	Orbital  bool
	Progress int     // industry points so far, at most Cost()
	Design   *Design // a Ship project's design or the refit's target; nil for structures
	// Refit is the Name of the ship refitted (unique in its empire), ""
	// for any other project. A refit's Item is OrbitalDocks (its UsePop is
	// the worker reserved); it places nothing, Empire.endDay applies Design.
	Refit     string
	RefitCost int // a refit's industry, RefitCost(ship, *Design) at the start
	// Target is the structure an Automation automates at Cell, read off
	// the square at the start.
	Target Item `json:",omitempty"`
	// ShipName is a Ship project's ship's name (BuildShipNamed), "" for
	// Spawn's own.
	ShipName string `json:",omitempty"`
}

// Cost is the industry the project takes: RefitCost, the design's,
// AutomationExtra plus the Target's, else the item's.
func (pj *Project) Cost() int {
	switch {
	case pj.Refit != "":
		return pj.RefitCost
	case pj.Design != nil:
		return pj.Design.Cost()
	case pj.Item == Automation:
		return AutomationExtra + pj.Target.Info().Cost
	}
	return pj.Item.Info().Cost
}

// NewColony settles p with StartPop units. The planet must already hold a
// structure: the colony base is placed by the coloniser, not here.
func NewColony(p *Planet) *Colony {
	if !p.HasStructure() {
		panic(fmt.Sprintf("colony on %q: no structure on the surface", p.Name))
	}
	return &Colony{Planet: p, Pop: StartPop, Star: -1, PlanetIdx: -1}
}

// Busy is the population employed: the structures' Staff
// (Planet.Population) plus the project's UsePop, reserved from its start
// (ASSUMPTION).
func (c *Colony) Busy() int {
	used, _ := c.Planet.Population()
	if c.Project != nil {
		used += c.Project.Item.Info().UsePop
	}
	return used
}

// Free is the population neither employed nor reserved.
func (c *Colony) Free() int { return c.Pop - c.Busy() }

// Capacity is the population room of its planet (Planet.Capacity).
func (c *Colony) Capacity() int { return c.Planet.Capacity() }

// Capacity is the population room a colony here has: CapacityBase plus a
// unit per CapacityPerSquares non-black squares, the placed structures'
// MaxPop, LushBonus once bombed, PopBonus. Defined without a colony.
func (p *Planet) Capacity() int {
	n := 0
	for _, sq := range p.Surface {
		if sq.Kind != Black {
			n++
		}
	}
	_, room := p.Population()
	capa := CapacityBase + n/CapacityPerSquares + room
	if p.Lush {
		capa += LushBonus
	}
	return capa + p.PopBonus // the Mebes' SA_MoreMaxPop, MebesPopBonus a use (ability.go)
}

// OngoingShare divides the industry an ongoing project diverts: q =
// floor(Σ industry × hyperpower) / OngoingShare (the manual's quarter,
// the party's and the takeover's); Alien Hospitality's is the curved
// industry's (Colony.industry) / OngoingShare, ours.
const OngoingShare = 4

// SingularityShare divides the research that goes to industry once the
// owner's tree is complete (Output): res / SingularityShare, floored.
const SingularityShare = 2

// Output is the colony's daily industry, research, prosperity (the
// formulas below). An ongoing project switches the industry off and its q
// (Diverted) goes to prosperity before the curve (Endless Party), to
// research (Scientist Takeover) or out (Alien Hospitality: Game.diplomacy).
// Past the singularity (Singular) res / SingularityShare of the final
// research moves to the industry, switched off with it by an ongoing
// project; q stays the planet's industry's.
func (c *Colony) Output() (ind, res, pros int) {
	ind, res, pros, off := c.output()
	if off {
		ind = 0
	}
	return ind, res, pros
}

// output is Output with the industry kept under an ongoing project, off
// reporting it switched off.
func (c *Colony) output() (ind, res, pros int, off bool) {
	sumInd, sumRes, sumPros := c.Planet.Output()
	ind = c.industry(sumInd)
	party, takeover := 0, 0
	it, q := c.divert(sumInd)
	switch it {
	case EndlessParty:
		party = q
	case ScientistTakeover:
		takeover = q
	}
	pros = power(math.Floor(float64(sumPros)*plant(c.Planet, FertilizationPlant))+float64(party)) - c.Pop/4
	res = int(math.Floor(float64(sumRes)*plant(c.Planet, Internet))) + takeover
	if c.Singular() {
		conv := res / SingularityShare
		res -= conv
		ind += conv
	}
	return ind, res, pros, it != NoItem // every Ongoing item switches it off
}

// Singular reports the owner's tree complete (Research.Complete): Output
// turns part of the research to industry. False for a colony with no
// owner's Research (the sim's lone colony).
func (c *Colony) Singular() bool {
	return c.owner != nil && c.owner.Research != nil && c.owner.Research.Complete()
}

// Diverted is the running ongoing project and the industry share q it
// diverts today (Output); NoItem, 0 when none runs.
func (c *Colony) Diverted() (Item, int) {
	if c.ongoing() == NoItem {
		return NoItem, 0
	}
	sumInd, _, _ := c.Planet.Output()
	return c.divert(sumInd)
}

// Shown is the output as the views show it, antag.exe's way: Output's
// research and prosperity (plants, curve, upkeep, the diverted share
// in), and the industry curved (the singularity's share in) but kept
// under an ongoing project, which the views mark as moved.
func (c *Colony) Shown() (ind, res, pros int) {
	ind, res, pros, _ = c.output()
	return ind, res, pros
}

// divert is Diverted on the planet's industry sum sumInd.
func (c *Colony) divert(sumInd int) (Item, int) {
	it := c.ongoing()
	if it == NoItem {
		return NoItem, 0
	}
	if it == AlienHospitality { // ours: the curved industry's quarter
		return it, c.industry(sumInd) / OngoingShare
	}
	return it, int(math.Floor(float64(sumInd)*plant(c.Planet, HyperpowerPlant))) / OngoingShare
}

// ongoing is the running ongoing project's Item, NoItem when the colony
// is idle or builds something that completes.
func (c *Colony) ongoing() Item {
	if c.Project != nil && c.Project.Item.Ongoing() {
		return c.Project.Item
	}
	return NoItem
}

// Running reports a project that completes; an ongoing one gives way to
// any start.
func (c *Colony) Running() bool { return c.Project != nil && !c.Project.Item.Ongoing() }

// The fan manual's formulas (planets-buildpower.html, Markus Ramikin's
// measurements), checked on the test saves' 870 player colonies
// (docs/rules.md "Colony output", savedump -colonies):
//
//	industry   = round(floor(Σ industry × hyperpower)^0.85)
//	prosperity = round(floor(Σ prosperity × fertilization + party)^0.85) − floor(pop/4)
//	research   = floor(Σ research × internet) + takeover
//
// party and takeover are the diverted q (Output), else 0. The −pop/4 is
// the upkeep. Each plant multiplies by 1.5 once, whatever its count. The
// Cloning Plant doubles a positive prosperity into the bank (banked).

// power is the sub-linear curve of industry and prosperity, with the
// original's one quirk: a floored sum of 9 gives 7, not round(6.47) = 6
// (every other sum measured, 1 to 60, is the round).
func power(sum float64) int {
	if math.Floor(sum) == 9 {
		return 7
	}
	return int(math.Round(math.Pow(math.Floor(sum), 0.85)))
}

// plant is 1.5 when the planet holds that plant, else 1.
func plant(p *Planet, it Item) float64 {
	if p.Count(it) > 0 {
		return 1.5
	}
	return 1
}

// common checks what any start needs: not a Colony Base (it lands with
// the colony), no project Running, the research, the workers.
func (c *Colony) common(r *Research, it Item) error {
	if it == ColonyBase {
		return fmt.Errorf("%w: a Colony Base comes with the colony", ErrNotBuildable)
	}
	if c.Running() {
		return fmt.Errorf("%w: %s", ErrProjectRunning, c.Project.what())
	}
	if err := allowed(r, it); err != nil {
		return err
	}
	if need := it.Info().UsePop; c.Free() < need {
		return fmt.Errorf("%w for %s (free %d, needs %d)", ErrNoFreePop, it, c.Free(), need)
	}
	return nil
}

// allowed reports why research r does not allow it yet, nil when it does.
func allowed(r *Research, it Item) error {
	if r.Allows(it) {
		return nil
	}
	req := it.Info().ResearchReq
	if req >= 0 && req < int(NumTechs) {
		return fmt.Errorf("%s needs %s", it, Tech(req))
	}
	return fmt.Errorf("%s needs research %d", it, req)
}

// CanStart reports why an Item.OnSquare project cannot start at a cell,
// nor an Automation on the structure there, nil when it can. The dig goes
// only on the (empty, reachable) ruins square, and nothing else does. An
// Automation goes on any surface structure not Automated yet with Staff,
// the Colony Base included.
func (c *Colony) CanStart(r *Research, it Item, at Cell) error {
	if !it.OnSquare() && it != Automation {
		return fmt.Errorf("%s is not a surface structure", it)
	}
	if err := c.common(r, it); err != nil {
		return err
	}
	return c.squareErr(c.Planet, it, at)
}

// squareErr is CanStart's verdict on the target alone: at on planet p
// (the colony's, or the one its queue plans: Planned).
func (c *Colony) squareErr(p *Planet, it Item, at Cell) error {
	if it == Automation {
		sq, ok := p.Surface[at]
		switch {
		case !ok || sq.Item == NoItem:
			return fmt.Errorf("no structure at %v to automate", at)
		case sq.Automated:
			return fmt.Errorf("%s at %v is automated already", sq.Item, at)
		case sq.Item.Staff() == 0:
			return fmt.Errorf("%s employs nobody: nothing to automate", sq.Item)
		}
		return nil
	}
	if it == XenoArcheologicalDig {
		sq, ok := p.Surface[at]
		switch {
		case !ok || !sq.Ruins:
			return fmt.Errorf("no Xeno ruins at %v", at)
		case sq.Item != NoItem:
			return fmt.Errorf("%v holds %s", at, sq.Item)
		case !p.Reachable(at):
			return fmt.Errorf("%v is not reachable", at)
		}
		return nil
	}
	if it == Terraforming && p.CanTerraform(at) || it != Terraforming && p.CanBuild(at, it, c.Traits()) {
		return nil
	}
	sq, ok := p.Surface[at]
	switch {
	case !ok:
		return fmt.Errorf("no square at %v", at)
	case it == Terraforming && sq.Kind != Black:
		return fmt.Errorf("%v is not black", at)
	case sq.Item != NoItem:
		return fmt.Errorf("%v holds %s", at, sq.Item)
	case sq.Ruins:
		return fmt.Errorf("%v holds Xeno ruins: dig them first", at)
	case !p.Reachable(at):
		return fmt.Errorf("%v is not reachable", at)
	case sq.Kind == Black:
		return fmt.Errorf("%v is black", at)
	}
	return fmt.Errorf("cannot build %s at %v", it, at)
}

// CanStartOrbital reports why the orbital structure it cannot be started
// in a slot, nil when it can.
func (c *Colony) CanStartOrbital(r *Research, it Item, slot int) error {
	if !it.Orbital() {
		return fmt.Errorf("%s is not an orbital structure", it)
	}
	if err := c.common(r, it); err != nil {
		return err
	}
	return orbitErr(c.Planet, it, slot)
}

// orbitErr is CanStartOrbital's verdict on the target alone: slot of
// planet p.
func orbitErr(p *Planet, it Item, slot int) error {
	switch {
	case p.CanBuildOrbital(slot, it):
		return nil
	case slot < 0 || slot >= OrbitSlots:
		return fmt.Errorf("no orbit slot %d", slot)
	case p.Orbit[slot] != NoItem:
		return fmt.Errorf("orbit slot %d holds %s", slot, p.Orbit[slot])
	case it == Ship && !p.HasShipyard():
		return fmt.Errorf("%s needs a shipyard", it)
	}
	return fmt.Errorf("cannot build %s in orbit slot %d", it, slot)
}

// CanStartProject reports why the planet-wide project it cannot start,
// nil when it can: the Lush Growth Bomb once per planet, an ongoing one
// unless it runs already (Scientist Takeover never once the tree is
// Complete), every other ErrNotModelled.
func (c *Colony) CanStartProject(r *Research, it Item) error {
	switch {
	case !it.Project():
		return fmt.Errorf("%s is not a project", it)
	case it.Targeted():
		return fmt.Errorf("%s starts on a square or a structure, not on the planet", it)
	}
	if err := c.common(r, it); err != nil {
		return err
	}
	if it == LushGrowthBomb {
		if c.Planet.Lush {
			return fmt.Errorf("%s has already been bombed", c.Planet.Name)
		}
		return nil
	}
	if it == ScientistTakeover && r.Complete() {
		return fmt.Errorf("%s: research is complete, nothing left to discover", it)
	}
	if it.Ongoing() {
		if c.ongoing() == it {
			return fmt.Errorf("%w: %s is already running", ErrProjectRunning, it)
		}
		return nil
	}
	return fmt.Errorf("%s: %w", it, ErrNotModelled)
}

// Start sets the project, crediting any carried industry.
func (c *Colony) Start(r *Research, it Item, at Cell) error {
	if err := c.CanStart(r, it, at); err != nil {
		return err
	}
	c.Project = &Project{Item: it, Cell: at}
	if it == Automation {
		c.Project.Target = c.Planet.Surface[at].Item
	}
	c.credit()
	if c.journal != nil {
		en := c.entry("start")
		en.Item, en.Cell = it.String(), at.String()
		c.journal.Record(en)
	}
	return nil
}

// StartOrbital sets an orbital project.
func (c *Colony) StartOrbital(r *Research, it Item, slot int) error {
	if err := c.startOrbital(r, it, slot); err != nil {
		return err
	}
	if c.journal != nil {
		en := c.entry("start")
		en.Item, en.Slot = it.String(), slot
		c.journal.Record(en)
	}
	return nil
}

// startOrbital is StartOrbital unrecorded (BuildShip records "build").
func (c *Colony) startOrbital(r *Research, it Item, slot int) error {
	if err := c.CanStartOrbital(r, it, slot); err != nil {
		return err
	}
	c.Project = &Project{Item: it, Slot: slot, Orbital: true}
	c.credit()
	return nil
}

// StartProject sets a planet-wide project; an ongoing one keeps the carry.
func (c *Colony) StartProject(r *Research, it Item) error {
	if err := c.CanStartProject(r, it); err != nil {
		return err
	}
	c.Project = &Project{Item: it, Slot: -1}
	if !it.Ongoing() {
		c.credit()
	}
	if c.journal != nil {
		en := c.entry("start")
		en.Item = it.String()
		c.journal.Record(en)
	}
	return nil
}

// DaysLeft is the days the project still takes at today's industry;
// false when not Running, making no industry or Blockaded.
func (c *Colony) DaysLeft() (int, bool) {
	ind, _, _ := c.Output()
	if !c.Running() || ind <= 0 || c.Blockaded() {
		return 0, false
	}
	left := max(0, c.Project.Cost()-c.Project.Progress)
	return (left + ind - 1) / ind, true
}

// industry is Output's industry on the planet's sum sumInd, before the
// singularity's share and an ongoing project's switch-off.
func (c *Colony) industry(sumInd int) int {
	return power(float64(sumInd) * plant(c.Planet, HyperpowerPlant))
}

// DaysFor is the days a project of cost started today would take, at
// the industry an ongoing project's switch-off leaves out (the start
// replaces it, so its diverted share is out too: a takeover's research,
// whose half goes to industry past the singularity); false when the
// colony makes no industry.
func (c *Colony) DaysFor(cost int) (int, bool) {
	if c.ongoing() != NoItem { // read as the start leaves it, put back
		pj := c.Project
		c.Project = nil
		defer func() { c.Project = pj }()
	}
	ind, _, _ := c.Output()
	if ind <= 0 {
		return 0, false
	}
	return (max(0, cost) + ind - 1) / ind, true
}

// banked is what pros puts into the growth bank: doubled when positive
// by a Cloning Plant ("doubles your population growth", the planets guide).
func (c *Colony) banked(pros int) int {
	if pros > 0 && c.Planet.Count(CloningPlant) > 0 {
		return 2 * pros
	}
	return pros
}

// GrowthDays is ⌈(GrowthCost − Growth) / banked prosperity⌉, at least 1:
// the days to the next unit; false when full or making no prosperity.
func (c *Colony) GrowthDays() (int, bool) {
	_, _, pros := c.Output()
	pros = c.banked(pros)
	if c.Pop >= c.Capacity() || pros <= 0 {
		return 0, false
	}
	return max(1, (GrowthCost-c.Growth+pros-1)/pros), true
}

// credit moves the carried industry into the project, up to what it still
// needs. Always zero with both lost-industry ASSUMPTIONs true.
func (c *Colony) credit() {
	n := min(c.Carry, c.Project.Cost()-c.Project.Progress)
	c.Project.Progress += n
	c.Carry -= n
}

// what names the project in a refusal: its item, or "refit of <ship>".
func (pj *Project) what() string {
	if pj.Refit != "" {
		return "refit of " + pj.Refit
	}
	return pj.Item.String()
}

// Cancel drops the project, its industry lost, its workers
// freed; recorded (the model's own drops call cancel).
func (c *Colony) Cancel() {
	pj := c.Project
	c.cancel()
	if pj != nil && c.journal != nil {
		en := c.entry("cancel")
		en.Item = pj.Item.String()
		if pj.Refit != "" {
			en.Ship = pj.Refit
		}
		c.journal.Record(en)
	}
}

// cancel is Cancel unrecorded.
func (c *Colony) cancel() { c.Project = nil }

// CanAbandon reports why the structure at c cannot be abandoned, nil
// when it can: an empty square, the Colony Base, or a structure whose
// MaxPop the population needs (as the original greys it).
func (c *Colony) CanAbandon(at Cell) error {
	sq, ok := c.Planet.Surface[at]
	switch {
	case !ok || sq.Item == NoItem:
		return fmt.Errorf("nothing to abandon at %v", at)
	case sq.Item == ColonyBase:
		return errors.New("the Colony Base cannot be abandoned")
	case c.Pop > c.Capacity()-sq.Item.Info().MaxPop:
		return fmt.Errorf("the population needs the room of %s", sq.Item)
	}
	return nil
}

// FreeAtCompletion is Free once the running project completes: its UsePop
// back, less the placed structure's Staff, plus an Automation target's; a
// ship's crew leaves with its reservation. Free when not Running.
func (c *Colony) FreeAtCompletion() int {
	pj := c.Project
	if !c.Running() {
		return c.Free()
	}
	free := c.Free() + pj.Item.Info().UsePop
	switch {
	case pj.Item == Ship:
		free -= pj.Item.Info().UsePop
	case pj.Refit != "":
	case pj.Item == Automation:
		free += pj.Target.Staff()
	case pj.Item == Terraforming || pj.Item == XenoArcheologicalDig: // they place nothing
	case pj.Item.OnSquare() || pj.Orbital:
		free -= pj.Item.Staff()
	}
	return free
}

// Abandon demolishes the structure at c, refused as CanAbandon says. The
// project keeps running (an Automation losing its target is dropped).
func (c *Colony) Abandon(at Cell) error {
	if err := c.CanAbandon(at); err != nil {
		return err
	}
	it := c.Planet.Surface[at].Item
	c.Planet.Demolish(at)
	if c.journal != nil {
		en := c.entry("demolish")
		en.Item, en.Cell = it.String(), at.String()
		c.journal.Record(en)
	}
	return nil
}

// AbandonOrbital empties orbit slot, refused on an empty slot and on the
// last structure the project needs.
func (c *Colony) AbandonOrbital(slot int) error {
	if slot < 0 || slot >= OrbitSlots || c.Planet.Orbit[slot] == NoItem {
		return fmt.Errorf("nothing to abandon in orbit slot %d", slot)
	}
	if it, pj := c.Planet.Orbit[slot], c.Project; pj != nil && c.needs(it) && c.orbitCount(it) == 1 {
		return fmt.Errorf("%s is in use: %s", it, pj.what())
	}
	it := c.Planet.Orbit[slot]
	c.Planet.DemolishOrbital(slot)
	if c.journal != nil {
		en := c.entry("demolish")
		en.Item, en.Slot = it.String(), slot
		c.journal.Record(en)
	}
	return nil
}

// needs reports whether the running project depends on an orbital it: a
// Ship on a Shipyard, a refit on Orbital Docks.
func (c *Colony) needs(it Item) bool {
	pj := c.Project
	return it == Shipyard && pj.Item == Ship || it == OrbitalDocks && pj.Refit != ""
}

// orbitCount counts the slots holding it.
func (c *Colony) orbitCount(it Item) int {
	n := 0
	for _, o := range c.Planet.Orbit {
		if o == it {
			n++
		}
	}
	return n
}

// EventKind tells what a day produced.
type EventKind uint8

const (
	Built           EventKind = iota // a project completed: Item
	Discovered                       // a tech became known: Tech
	Grew                             // population +1
	Destroyed                        // a ship destroyed in combat
	Refitted                         // a ship's refit completed: Item OrbitalDocks
	Razed                            // a structure destroyed in combat: Item, orbital or surface (the Phase Bomb)
	Invaded                          // a colony captured by an invasion, for both sides
	Repelled                         // an invasion failed, for both sides
	Contacted                        // first contact with Other
	WarDeclared                      // war declared, by or on Other
	PeaceOffered                     // a peace offer, from or to Other
	PeaceMade                        // an offer accepted: peace with Other
	OfferRefused                     // an offer refused or expired, ours or Other's
	AllianceOffered                  // an alliance offer, from or to Other
	AllianceMade                     // an alliance offer accepted
	AllianceBroken                   // an alliance broken, by or on Other
	TradeOffered                     // a knowledge exchange offered, from or to Other
	TradeMade                        // an exchange accepted, what we learnt and taught
	HelpAsked                        // a third-party request, from or to the ally Other
	HelpGiven                        // a request granted, by or to the ally Other
	AbilityUsed                      // our timed ability used, Other −1
	SpecialUsed                      // a ship special used, the user's report
	Scrapped                         // a ship scrapped over the fleet cap at the close
	Defeated                         // an empire out of the game: ours (Other −1) or Other's
	Arrived                          // a ship in orbit of Planet, or out of a lane at Star (Planet −1), at the close
	AbilityReady                     // our timed ability recharged at the close, Other −1
	NoRoute                          // a GoToSystem order dropped at the close: no known route left (route.go), at the ship's Star
	Unqueued                         // a queue entry dropped by Next: it could not start (queue.go), Item
	Won                              // a victory at the close (victory.go): ours (Other −1) or Other's
	Engaged                          // a fight opened at Star at the close (Game.opened): Other the first hostile
	Blockade                         // a blockade of a colony begun or lifted at the close (blockade.go), for both sides
)

func (k EventKind) String() string {
	return [...]string{"built", "discovered", "grew", "destroyed", "refitted", "razed", "invaded", "repelled",
		"contacted", "war declared", "peace offered", "peace made", "offer refused",
		"alliance offered", "alliance made", "alliance broken", "trade offered", "trade made",
		"help asked", "help given", "ability used", "special used",
		"scrapped", "defeated", "arrived", "ability ready", "no route", "unqueued", "won", "engaged", "blockade"}[k]
}

// Event is what a day produced, for the report dialogs. Item NoItem, Tech
// NoTech, Star and Planet (index in its System.Planets) −1, Other (the
// other empire) −1 when they do not apply: every emission sets them all,
// the zero value means star 0, empire 0.
type Event struct {
	Kind         EventKind
	Item         Item
	Tech         Tech
	Text         string
	Star, Planet int
	Other        int
	// Use is a SpecialUsed report's use as data (UseSpecial), for the
	// window's round log; never saved.
	Use *SpecialUse `json:"-"`
}

// EndDay runs one day on Output taken at its start (a structure
// completing today produces from tomorrow): industry to the project (at
// its cost: placed, Built, cleared, the surplus lost), research to the
// empire's r (Discovered), prosperity to Growth (one Grew per unit bought
// under Capacity, counted after today's build; the bank clamped at
// GrowthCost).
//
// A completion whose target became unbuildable meanwhile is dropped with
// no event. Terraforming whitens its square, the Lush Growth Bomb sets
// Planet.Lush, the dig clears the Ruins and grants a tech (dig: Built,
// then Discovered; the tech completing the tree adds the singularity's
// Discovered, Tech NoTech), an Automation marks its Target Automated. A
// running Scientist Takeover past the singularity is dropped first, no
// event. A Ship is
// Built and placed nowhere, a refit Refitted (Item OrbitalDocks): the
// Empire makes the ship or applies the design. Last, the queue's head
// starts (Next, after the growth: a worker born today counts), unless a
// Ship completed: Empire.endDay calls Next once the crew has left. key is
// the dig's draw key (DigKey), any value outside a game.
func (c *Colony) EndDay(r *Research, key uint64) []Event {
	var ev []Event
	if c.ongoing() == ScientistTakeover && r.Complete() {
		c.cancel() // refused past the singularity (CanStartProject): a running one stops
	}
	ind, res, pros := c.Output()
	launching := false // a Ship completed: its crew is still in Pop
	blockaded := c.blockaded
	c.blockaded = false

	if pj := c.Project; pj != nil && pj.Item.Ongoing() {
		// Output switched the industry off: nothing to add, nothing carried.
	} else if pj != nil && blockaded && pj.InOrbit() {
		// Blockaded at the day's start: the industry is lost, the work kept.
	} else if pj != nil {
		pj.Progress += ind
		if cost := pj.Cost(); pj.Progress >= cost {
			if !industrySurplusLost {
				c.Carry += pj.Progress - cost
			}
			pj.Progress = cost
			c.Project = nil
			var err error
			switch {
			case pj.Refit != "":
				// Taken first: a refit's Item is OrbitalDocks, never built.
				ev = append(ev, Event{Kind: Refitted, Item: OrbitalDocks, Tech: NoTech, Star: c.Star, Planet: c.PlanetIdx, Other: -1,
					Text: fmt.Sprintf("%s refitted at %s", pj.Refit, c.Planet.Name)})
			case pj.Item == Ship: // the Empire launches it
				launching = true
			case pj.Item == Terraforming:
				err = c.Planet.Terraform(pj.Cell)
			case pj.Item == LushGrowthBomb:
				c.Planet.Lush = true
			case pj.Item == XenoArcheologicalDig:
				c.Planet.Surface[pj.Cell].Ruins = false
			case pj.Item == Automation:
				err = c.Planet.Automate(pj.Cell, pj.Target)
			case pj.Orbital:
				err = c.Planet.BuildOrbital(pj.Slot, pj.Item)
			default:
				err = c.Planet.BuildWith(pj.Cell, pj.Item, c.Traits())
			}
			// The news box's terse texts (the original: "Construction of %s
			// is complete on %s").
			text := fmt.Sprintf("%s built on %s", pj.Item, c.Planet.Name)
			switch pj.Item {
			case XenoArcheologicalDig:
				text = "Dig done on " + c.Planet.Name
			case Automation: // the original has only the generic text (help.txt event01)
				text = fmt.Sprintf("%s automated on %s", pj.Target, c.Planet.Name)
			}
			if err == nil && pj.Refit == "" {
				ev = append(ev, Event{Kind: Built, Item: pj.Item, Tech: NoTech, Star: c.Star, Planet: c.PlanetIdx, Other: -1, Text: text})
			}
			if pj.Item == XenoArcheologicalDig {
				if t, ok := c.dig(r, key); ok {
					r.Grant(t)
					ev = append(ev, Event{Kind: Discovered, Item: NoItem, Tech: t, Star: c.Star, Planet: c.PlanetIdx, Other: -1,
						Text: t.String() + " found in ruins"})
					if x, ok := singularity(r, t); ok {
						ev = append(ev, x)
					}
				}
			}
		}
	} else if !idleIndustryLost {
		c.Carry += ind
	}

	if t, ok := r.Advance(res); ok {
		ev = append(ev, Event{Kind: Discovered, Item: NoItem, Tech: t, Star: -1, Planet: -1, Other: -1,
			Text: fmt.Sprintf("%s discovered", t)})
		if x, ok := singularity(r, t); ok {
			ev = append(ev, x)
		}
	}

	pros = c.banked(pros)
	c.Growth += pros
	c.Growth = max(c.Growth, 0) // ASSUMPTION: negative prosperity starves the bank, never the people
	for capa := c.Capacity(); c.Growth >= GrowthCost && c.Pop < capa; {
		c.Pop++
		c.Growth -= GrowthCost
		ev = append(ev, Event{Kind: Grew, Item: NoItem, Tech: NoTech, Star: c.Star, Planet: c.PlanetIdx, Other: -1,
			Text: fmt.Sprintf("%s grows to %d", c.Planet.Name, c.Pop)})
	}
	c.Growth = min(c.Growth, GrowthCost)
	if !launching { // else Empire.endDay calls Next once the crew is off the Pop
		ev = append(ev, c.Next(r)...)
	}
	return ev
}

// dig draws the tech a completed dig grants: one r does not know,
// uniformly, prerequisites ignored (as in the original); false when all
// are known. A fresh PCG seeded from key, Star and PlanetIdx: no PRNG
// state outlives the day, so a loaded game digs what the run would have.
func (c *Colony) dig(r *Research, key uint64) (Tech, bool) {
	var unknown []Tech
	for t := range NumTechs {
		if !r.Known[t] {
			unknown = append(unknown, t)
		}
	}
	if len(unknown) == 0 {
		return NoTech, false
	}
	return unknown[rand.New(rand.NewPCG(c.drawKey(key), 0x510e527fade682d1)).IntN(len(unknown))], true
}

// drawKey is key mixed with the colony's Star and PlanetIdx.
func (c *Colony) drawKey(key uint64) uint64 {
	return mix64(mix64(key^uint64(c.Star)) ^ uint64(c.PlanetIdx))
}

// DigKey is empire's draw key on day for its digs: a hash of the seed,
// the empire and the day.
func DigKey(seed uint64, empire, day int) uint64 {
	return mix64(mix64(mix64(seed)^uint64(empire)) ^ uint64(day))
}

// mix64 is SplitMix64's finaliser: neighbouring inputs draw unrelated.
func mix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}
