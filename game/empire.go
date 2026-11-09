package game

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/rewk/aiscendancy/journal"
)

// ShipsPerSystem caps the fleet, ships under construction included.
// ASSUMPTION: the manual says "proportional to the number of star systems
// you control"; the factor is ours.
const ShipsPerSystem = 3

// ErrShipCap refuses a ship over the fleet cap.
var ErrShipCap = errors.New("fleet at its cap")

// errNotOurs refuses acting on a ship not in e.Ships (a view may hold
// another empire's).
var errNotOurs = errors.New("not a ship of this empire")

// command reports why e cannot command s now: not its ship, or held at
// its Docks by a refit.
func (e *Empire) command(s *Starship) error {
	if !slices.Contains(e.Ships, s) {
		return errNotOurs
	}
	if c := e.Refitting(s); c != nil {
		return fmt.Errorf("%s is being refitted at %s", s.Name, c.Planet.Name)
	}
	return nil
}

// Empire is one species' colonies, ships and shared research on a cluster.
type Empire struct {
	Species Species
	Index   int // in Game.Empires, the Owner of its planets
	// Colour is 0..NumColours−1 (Green..Orange, the original's order).
	Colour   int
	Research *Research
	Cluster  *Cluster  `json:"-"` // the Game's, shared (Game.Relink)
	Colonies []*Colony // the home first, then in founding order
	Ships    []*Starship
	// Templates are the player's ship classes, in saving order.
	Templates []Template
	// Visited marks the stars explored, LanesKnown the lanes learnt on
	// their own (fog.go); both shorter than the cluster until grown.
	Visited    []bool
	LanesKnown []bool
	Day        int // days run so far (Game.EndDay keeps Game.Day equal)
	// Relations is the diplomacy with each empire, by Index (diplomacy.go);
	// nil for an empire alone.
	Relations []Relation
	// Lost is the day the empire was defeated (defeat.go), 0 while Alive.
	Lost int
	// Won is the empire's victory and WonDay its day (victory.go), the
	// first won, kept.
	Won    Victory `json:",omitempty"`
	WonDay int     `json:",omitempty"`
	// History is a Sample every HistoryEvery days (history.go), oldest
	// first; nothing in the rules reads it.
	History []Sample `json:",omitempty"`
	// AbilityUsed is the day the timed ability was last used, moved on a
	// day per close asleep (abilitiesReady); 0 at the start: it starts
	// empty (measured in DOSBox).
	AbilityUsed int
	// Invincible is the day the Capelons' SA_ShieldPlanets was last used
	// (IsInvincible), −1 never.
	Invincible int
	// BlockedUntil ends the Arbryls' SA_BlockStarLanes (LaneBlocker); 0
	// never.
	BlockedUntil int
	// HospitalityGiven is every Alien Hospitality point e's colonies made
	// (each one reaches every empire at Peace or Alliance with e).
	HospitalityGiven int `json:",omitempty"`
	// LaneBlocks is the lanes e's Lane Blockers closed: lane → the day the
	// block ends (exclusive), both ways, to every other empire.
	LaneBlocks map[int]int `json:",omitempty"`
	// ScanAll is the -reveal cheat: every ship seen is scanned. Not saved.
	ScanAll bool `json:"-"`
	// peers is the Game's Empires, e among them; nil outside a Game. Not
	// saved.
	peers []*Empire
	// journal is the Game's (SetJournal), nil none; not saved.
	journal *journal.Writer
}

// NewEmpire is the one empire of NewGame(seed, {sp}, d), no Relations.
func NewEmpire(seed uint64, sp Species, d Density) *Empire {
	e := NewGame(seed, []Species{sp}, d).Empires[0]
	e.Relations = nil
	return e
}

// Relink points every colony's Planet at the cluster's, from Star and
// PlanetIdx (a decoded save drops the pointer), and adopts it; errors on
// a bad index.
func (e *Empire) Relink() error {
	for i, c := range e.Colonies {
		if c.Star < 0 || c.Star >= len(e.Cluster.Stars) {
			return fmt.Errorf("colony %d: no star %d", i, c.Star)
		}
		pls := e.Cluster.Stars[c.Star].System.Planets
		if c.PlanetIdx < 0 || c.PlanetIdx >= len(pls) {
			return fmt.Errorf("colony %d: star %d has no planet %d", i, c.Star, c.PlanetIdx)
		}
		c.Planet = pls[c.PlanetIdx]
		c.adopt(e)
	}
	return nil
}

// Planet is planet pl of star's system.
func (e *Empire) Planet(star, pl int) *Planet { return e.Cluster.Stars[star].System.Planets[pl] }

// Colony is the colony on a planet, nil when none.
func (e *Empire) Colony(star, planet int) *Colony {
	for _, c := range e.Colonies {
		if c.Star == star && c.PlanetIdx == planet {
			return c
		}
	}
	return nil
}

// Inhabited reports a colony on the planet, of any empire (Planet.Owner).
func (e *Empire) Inhabited(star, planet int) bool { return e.Planet(star, planet).Owner >= 0 }

// Owns reports p is one of e's colonies.
func (e *Empire) Owns(p *Planet) bool { return p.Owner == e.Index }

// ControlledSystems counts the stars e controls (StarSystem.Controller:
// every colony there is e's), the fleet cap's base.
func (e *Empire) ControlledSystems() int {
	seen := map[int]bool{}
	for _, c := range e.Colonies {
		if !seen[c.Star] && e.Cluster.Stars[c.Star].System.Controller() == e.Index {
			seen[c.Star] = true
		}
	}
	return len(seen)
}

// ResearchRate is the colonies' Output research summed.
func (e *Empire) ResearchRate() int {
	n := 0
	for _, c := range e.Colonies {
		_, res, _ := c.Output()
		n += res
	}
	return n
}

// Fleet is what the cap counts and the cap.
func (e *Empire) Fleet() (n, capa int) { return e.fleet(), ShipsPerSystem * e.ControlledSystems() }

// fleet counts the ships flying and the Ship projects running (a refit,
// Item OrbitalDocks, is not a ship).
func (e *Empire) fleet() int {
	n := len(e.Ships)
	for _, c := range e.Colonies {
		if c.Project != nil && c.Project.Item == Ship {
			n++
		}
	}
	return n
}

// shipSlot is the first orbit slot where c can start a Ship, or the last
// refusal.
func (e *Empire) shipSlot(c *Colony) (int, error) {
	err := fmt.Errorf("%s: no free orbit slot", c.Planet.Name)
	for slot := range OrbitSlots {
		if err = c.CanStartOrbital(e.Research, Ship, slot); err == nil {
			return slot, nil
		}
	}
	return -1, err
}

// CanBuildShip reports why c cannot start building d, nil when it can: c
// not e's, the fleet cap, d invalid, no slot CanStartOrbital accepts.
func (e *Empire) CanBuildShip(c *Colony, d Design) error {
	if !slices.Contains(e.Colonies, c) {
		return fmt.Errorf("%s is not our colony", c.Planet.Name)
	}
	if err := e.fleetCap(); err != nil {
		return err
	}
	if err := d.Valid(e.Research); err != nil {
		return err
	}
	_, err := e.shipSlot(c)
	return err
}

// CanStartShip reports why c cannot start a Ship in slot, whatever the
// design (the item picker's verdict), nil when it can.
func (e *Empire) CanStartShip(c *Colony, slot int) error {
	if err := e.fleetCap(); err != nil {
		return err
	}
	return c.CanStartOrbital(e.Research, Ship, slot)
}

// fleetCap refuses one more ship at the cap (ErrShipCap).
func (e *Empire) fleetCap() error {
	if n, capa := e.fleet(), ShipsPerSystem*e.ControlledSystems(); n >= capa {
		return fmt.Errorf("%w: %d ships of %d", ErrShipCap, n, capa)
	}
	return nil
}

// BuildShip starts the Ship project for d on c's first free orbit slot,
// costing d.Cost() and reserving the Ship item's UsePop (the crew).
func (e *Empire) BuildShip(c *Colony, d Design) error { return e.BuildShipNamed(c, d, "") }

// BuildShipNamed is BuildShip with the ship's name chosen as is ("": Spawn
// names it). Refused on a blank name or one a ship or ship project of e
// carries (ships are found by Name).
func (e *Empire) BuildShipNamed(c *Colony, d Design, name string) error {
	if name != "" && strings.TrimSpace(name) == "" {
		return errors.New("a ship's name needs a letter")
	}
	if name != "" && e.shipNameTaken(name) {
		return fmt.Errorf("a ship is named %s already", name)
	}
	if err := e.CanBuildShip(c, d); err != nil {
		return err
	}
	slot, _ := e.shipSlot(c)
	if err := c.startOrbital(e.Research, Ship, slot); err != nil {
		return err
	}
	d.Parts, d.Slots = slices.Clone(d.Parts), slices.Clone(d.Slots)
	c.Project.Design, c.Project.ShipName = &d, name
	c.credit() // startOrbital credited up to the item's cost; the design costs more
	if e.journal != nil {
		en := e.entry("build")
		en.Colony, en.Design, en.Slot = c.Planet.Name, d.Name, slot
		e.journal.Record(en)
	}
	return nil
}

// AddTemplate saves a copy of t under its Name, replacing a namesake in
// place. Refused on a blank name, too many parts, or anything research
// does not allow yet; power and engines are judged at build.
func (e *Empire) AddTemplate(t Template) error {
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("a template needs a name")
	}
	if t.Hull >= NumHulls {
		return fmt.Errorf("template %q: no hull %d", t.Name, t.Hull)
	}
	if len(t.Parts) > t.Hull.Slots() {
		return fmt.Errorf("template %q: %d parts on a %s hull of %d slots", t.Name, len(t.Parts), t.Hull, t.Hull.Slots())
	}
	if err := checkSlots(t.Slots, len(t.Parts), t.Hull); err != nil {
		return fmt.Errorf("template %q: %w", t.Name, err)
	}
	r := e.Research
	if !r.Known[t.Hull.Tech()] {
		return fmt.Errorf("template %q: the %s hull needs %s", t.Name, t.Hull, t.Hull.Tech())
	}
	for _, p := range t.Parts {
		switch {
		case p.Best:
			if _, ok := best(r, p.Cat); !ok {
				return fmt.Errorf("template %q: no %s allowed yet", t.Name, strings.ToLower(p.Cat.String()))
			}
		case p.Gizmo >= NumGizmos:
			return fmt.Errorf("template %q: no part %d", t.Name, p.Gizmo)
		case !r.AllowsGizmo(p.Gizmo):
			return fmt.Errorf("template %q: %s needs %s", t.Name, p.Gizmo, Gizmos[p.Gizmo].ResearchReq)
		}
	}
	t.Parts, t.Slots = slices.Clone(t.Parts), slices.Clone(t.Slots)
	for i := range e.Templates {
		if e.Templates[i].Name == t.Name {
			e.Templates[i] = t
			return nil
		}
	}
	e.Templates = append(e.Templates, t)
	return nil
}

// shipNameTaken reports name a ship of e's or a ship project's.
func (e *Empire) shipNameTaken(name string) bool {
	if e.ship(name) != nil {
		return true
	}
	return slices.ContainsFunc(e.Colonies, func(c *Colony) bool { return c.Project != nil && c.Project.ShipName == name })
}

// ShipNameFor is the naming box's proposal and Spawn's default: class
// with the first numeral no ship or ship project bears
// (Small I, Small II → Small III; a numeral freed by a loss is taken
// again).
func (e *Empire) ShipNameFor(class string) string {
	for n := 1; ; n++ {
		if name := shipName(class, n); !e.shipNameTaken(name) {
			return name
		}
	}
}

// shipName is "<design> <n in roman>", decimal past 39.
func shipName(design string, n int) string {
	if n <= 39 {
		return design + " " + roman(n)
	}
	return design + " " + strconv.Itoa(n)
}

// CanFound reports why s cannot found a colony on the planet it orbits
// (not in orbit, no Colonizer, inhabited, an ally's system), nil when it
// can.
func (e *Empire) CanFound(s *Starship) error {
	switch {
	case !slices.Contains(e.Ships, s):
		return errNotOurs
	case s.Lane >= 0 || s.Orbit < 0:
		return fmt.Errorf("%s is not in orbit", s.Name)
	case s.Cargo < 1:
		return fmt.Errorf("%s has no Colonizer left", s.Name)
	case e.Inhabited(s.Star, s.Orbit):
		return fmt.Errorf("%s is inhabited", e.Planet(s.Star, s.Orbit).Name)
	}
	if a := e.ally(s.Star); a != nil {
		return fmt.Errorf("%w: the %s hold %s", ErrAllySystem, a.Species, e.Cluster.Stars[s.Star].Name)
	}
	return nil
}

// ErrAllySystem: allies do not colonise each other's systems (species.txt:
// allies "will neither attack without provocation nor colonize systems
// you control").
var ErrAllySystem = errors.New("an ally's system")

// AllySystem reports star holding a colony of an empire e is allied with.
func (e *Empire) AllySystem(star int) bool { return e.ally(star) != nil }

// ally is the first ally of e holding a colony at star, nil when none.
func (e *Empire) ally(star int) *Empire {
	for _, o := range e.peerList() {
		if o == e || e.StanceWith(o.Index) != Alliance {
			continue
		}
		for _, c := range o.Colonies {
			if c.Star == star {
				return o
			}
		}
	}
	return nil
}

// Found settles the planet s orbits with one of its Colonizers, the
// Colony Base on the square BestCell picks (the bots' Colonise order).
func (e *Empire) Found(s *Starship) error {
	at, err := e.foundCell(s)
	if err != nil {
		return err
	}
	return e.FoundAt(s, at)
}

// foundCell is CanFound, then BestCell for the Colony Base.
func (e *Empire) foundCell(s *Starship) (Cell, error) {
	if err := e.CanFound(s); err != nil {
		return Cell{}, err
	}
	p := e.Planet(s.Star, s.Orbit)
	at, ok := p.BestCell(ColonyBase, e.Species.Traits())
	if !ok {
		return Cell{}, fmt.Errorf("%s has no square for a Colony Base", p.Name)
	}
	return at, nil
}

// FoundAt settles the planet s orbits now, its Colony Base on at
// (Planet.CanSettle: a black square too), one Colonizer spent.
func (e *Empire) FoundAt(s *Starship, at Cell) error {
	if err := e.foundAt(s, at); err != nil {
		return err
	}
	if e.journal != nil {
		en := e.shipEntry("found", s)
		en.Colony, en.Cell, en.Star, en.Planet = e.Planet(s.Star, s.Orbit).Name, at.String(), s.Star, s.Orbit
		e.journal.Record(en)
	}
	return nil
}

// foundAt is FoundAt unrecorded: the day's Colonise orders (endDay).
func (e *Empire) foundAt(s *Starship, at Cell) error {
	if err := e.CanFound(s); err != nil {
		return err
	}
	p := e.Planet(s.Star, s.Orbit)
	if !p.CanSettle(at) {
		return fmt.Errorf("no Colony Base at (%d,%d) on %s", at.I, at.J, p.Name)
	}
	e.settle(s.Star, s.Orbit, at)
	s.Cargo--
	return nil
}

// settle makes planet pl of star e's colony, its Colony Base on at, no
// check: the planet's Owner, a NewColony adopted and appended to
// Colonies, the encroach tallied (foundAt, ApplyScenario).
func (e *Empire) settle(star, pl int, at Cell) *Colony {
	p := e.Planet(star, pl)
	p.Surface[at].Item = ColonyBase
	p.Owner = e.Index
	col := NewColony(p)
	col.Star, col.PlanetIdx = star, pl
	col.adopt(e)
	e.Colonies = append(e.Colonies, col)
	e.encroach(star, pl)
	return col
}

// encroach tallies the colony e just founded at planet pl of star in the
// ledger of every other empire that holds a colony there and has met e
// (Relation.Encroached): the original's help has allies "never colonize
// systems you control", an affront in peace.
func (e *Empire) encroach(star, pl int) {
	var done []int
	for k, p := range e.Cluster.Stars[star].System.Planets {
		j := p.Owner
		if k == pl || j < 0 || j == e.Index || j >= len(e.peers) || slices.Contains(done, j) {
			continue
		}
		done = append(done, j)
		if r := e.peers[j].Rel(e.Index); r != nil && r.Stance != Unknown {
			r.Encroached++
		}
	}
}

// habitatColony is the colony of e's that s orbits, as Load and Unload
// need it: s commanded by e (command), in orbit, the planet e's colony,
// s carrying Space Habitats.
func (e *Empire) habitatColony(s *Starship) (*Colony, error) {
	if err := e.command(s); err != nil {
		return nil, err
	}
	if s.Lane >= 0 || s.Orbit < 0 {
		return nil, fmt.Errorf("%s is not in orbit", s.Name)
	}
	c := e.Colony(s.Star, s.Orbit)
	switch {
	case c == nil:
		return nil, fmt.Errorf("%s is not our colony", e.Planet(s.Star, s.Orbit).Name)
	case s.Berths() < 1:
		return nil, fmt.Errorf("%s has no Space Habitat", s.Name)
	}
	return c, nil
}

// CanLoad reports why one population unit of the colony s orbits cannot
// board s, nil when it can: habitatColony, a free berth, a free unit
// (Colony.Free: neither employed nor reserved), and one left behind.
func (e *Empire) CanLoad(s *Starship) error {
	c, err := e.habitatColony(s)
	switch {
	case err != nil:
		return err
	case s.Pop >= s.Berths():
		return fmt.Errorf("%s is full (%d of %d)", s.Name, s.Pop, s.Berths())
	case c.Free() < 1:
		return fmt.Errorf("%s has no free population", c.Planet.Name)
	case c.Pop < 2:
		return fmt.Errorf("the last colonist stays on %s", c.Planet.Name)
	}
	return nil
}

// Load moves one population unit from the colony s orbits aboard s, as
// CanLoad allows.
func (e *Empire) Load(s *Starship) error {
	if err := e.CanLoad(s); err != nil {
		return err
	}
	c := e.Colony(s.Star, s.Orbit)
	c.Pop--
	s.Pop++
	e.recordHabitat("load", s, c)
	return nil
}

// CanUnload reports why one population unit aboard s cannot land on the
// colony it orbits, nil when it can: habitatColony, someone aboard, room
// under the colony's Capacity.
func (e *Empire) CanUnload(s *Starship) error {
	c, err := e.habitatColony(s)
	switch {
	case err != nil:
		return err
	case s.Pop < 1:
		return fmt.Errorf("nobody aboard %s", s.Name)
	case c.Pop >= c.Capacity():
		return fmt.Errorf("%s is full (%d of %d)", c.Planet.Name, c.Pop, c.Capacity())
	}
	return nil
}

// Unload lands one population unit of s on the colony it orbits, free
// (nothing else changes: the Growth bank is kept), as CanUnload allows.
func (e *Empire) Unload(s *Starship) error {
	if err := e.CanUnload(s); err != nil {
		return err
	}
	c := e.Colony(s.Star, s.Orbit)
	c.Pop++
	s.Pop--
	e.recordHabitat("unload", s, c)
	return nil
}

// recordHabitat journals a Load or Unload: the ship, the colony, its
// star and planet.
func (e *Empire) recordHabitat(act string, s *Starship, c *Colony) {
	if e.journal == nil {
		return
	}
	en := e.shipEntry(act, s)
	en.Colony, en.Star, en.Planet = c.Planet.Name, c.Star, c.PlanetIdx
	e.journal.Record(en)
}

// stopTakeovers cancels every running Scientist Takeover once e's tree is
// complete (CanStartProject refuses them past the singularity), wherever
// the last tech came from: none outlives the singularity's day.
func (e *Empire) stopTakeovers() {
	for _, c := range e.Colonies {
		if c.ongoing() == ScientistTakeover && e.Research.Complete() {
			c.cancel()
		}
	}
}

// EndDay runs one day of the empire, Day first incremented:
//
//  0. The refits whose ship is gone are cancelled.
//  1. Every colony's EndDay in order, all feeding the one Research (a
//     discovery mid-loop loses the later colonies' points). A completed
//     Ship is launched (it moves from tomorrow), a refit applied.
//  2. Every ship already flying Steps in Ships order, against the live
//     positions, guards (Barred) and lane blocks (holdAtBlock); a
//     GoToSystem order flies its Route's next lane (steer), or is dropped
//     with a NoRoute.
//  3. Every ship in orbit with a Colonise order Founds (the order cleared
//     either way): Built, Item ColonyBase.
//
// The Invade orders are the Game's. Alone, the digs draw on seed 0.
func (e *Empire) EndDay() []Event { return e.endDay(0, nil, []*Empire{e}) }

// endDay is EndDay where the ships out of the lanes at a star with
// rounds played today and a fight still there (rounds[star] > 0,
// Game.EndDay's mask of Game.RoundsToday) are refilled
// instead of Stepped; es are the empires whose ships are obstacles, seed
// the game's (DigKey).
func (e *Empire) endDay(seed uint64, rounds []int, es []*Empire) []Event {
	e.Day++
	e.cancelLostRefits()
	var ev, fresh []Event
	flying := e.Ships
	for _, c := range e.Colonies {
		c.OrbitShots, c.Armed, c.Tractors = [OrbitSlots]int{}, nil, 0
		pj := c.Project
		for _, x := range c.EndDay(e.Research, DigKey(seed, e.Index, e.Day)) {
			if x.Kind == Built && x.Item == Ship {
				s := e.launch(c, pj)
				x.Text = fmt.Sprintf("%s launched at %s", s.Name, c.Planet.Name)
			}
			if x.Kind == Refitted {
				if s := e.ship(pj.Refit); s != nil { // cancelLostRefits kept it
					applyRefit(s, *pj.Design)
				}
			}
			ev = append(ev, x)
		}
		ev = append(ev, c.Next(e.Research)...) // after a launch (Colony.EndDay); else a no-op
	}
	e.stopTakeovers() // a colony's tech completed the tree: those before it in Colonies too
	// flying predates today's launches: they are not stepped.
	for _, s := range flying {
		if s.Lane < 0 && s.Star >= 0 && s.Star < len(rounds) && rounds[s.Star] > 0 {
			s.Refill()
		} else {
			near := func(star int) []Vec3 { return spaceAt(es, star, s) }
			nav, hop, routed, lost := e.steer(s)
			if lost != nil {
				fresh = append(fresh, *lost)
			}
			o, held := e.holdAtBlock(s, e.Day, near)
			inLane, star, orbit := s.Lane >= 0, s.Star, s.Orbit
			arrived := s.Step(e.Cluster, e.Day, near, e.barredIn(es))
			if held {
				s.Order = o
			}
			if routed {
				unsteer(s, nav, hop, star)
			}
			// A route's stop on the way is no news: only its last.
			if x, ok := e.arrival(s, inLane, star, orbit, arrived); ok && s.Order.Kind != GoToSystem {
				fresh = append(fresh, x)
			}
		}
		if s.Order.Kind != Colonise || s.Orbit < 0 {
			continue
		}
		s.Order = Order{}
		at, err := e.foundCell(s)
		if err == nil {
			err = e.foundAt(s, at) // the order's resolution: not journalled
		}
		if err == nil {
			fresh = append(fresh, Event{Kind: Built, Item: ColonyBase, Tech: NoTech, Star: s.Star, Planet: s.Orbit, Other: -1,
				Text: "Colony founded on " + e.Planet(s.Star, s.Orbit).Name})
		}
	}
	e.Revisit()
	return append(ev, fresh...)
}

// arrival is the Arrived event of s's Step at the close (the
// offline moves are news), given inLane, star and orbit before it: out
// of a lane (or a one-day lane crossed), or into another orbit; none for a
// point, nor an orbit a Colonise order founds on.
func (e *Empire) arrival(s *Starship, inLane bool, star, orbit int, arrived bool) (Event, bool) {
	switch {
	case !arrived:
	case (inLane || s.Star != star) && s.Lane < 0:
		return Event{Kind: Arrived, Item: NoItem, Tech: NoTech, Star: s.Star, Planet: -1, Other: -1,
			Text: fmt.Sprintf("%s arrived at %s", s.Name, e.Cluster.Stars[s.Star].Name)}, true
	case !inLane && s.Orbit >= 0 && s.Orbit != orbit && s.Order.Kind != Colonise:
		return Event{Kind: Arrived, Item: NoItem, Tech: NoTech, Star: s.Star, Planet: s.Orbit, Other: -1,
			Text: fmt.Sprintf("%s in orbit of %s", s.Name, e.Planet(s.Star, s.Orbit).Name)}, true
	}
	return Event{}, false
}

// launch makes the ship pj completed on c: Spawn, the crew off the Pop.
func (e *Empire) launch(c *Colony, pj *Project) *Starship {
	var d Design
	if pj.Design != nil {
		d = *pj.Design
	}
	c.Pop -= pj.Item.Info().UsePop
	return e.spawn(d, c.Star, c.PlanetIdx, pj.ShipName)
}

// Order gives s the order o, a view's one entry point: refused as command,
// Check, an inhabited planet to Colonise, invadeErr, blockedOrder, a
// GoToSystem's Route say.
func (e *Empire) Order(s *Starship, o Order) error {
	if err := e.command(s); err != nil {
		return err
	}
	if err := s.Check(e.Cluster, o); err != nil {
		return err
	}
	if o.Kind == Colonise && e.Inhabited(s.Star, s.Orbit) {
		return fmt.Errorf("%s is already inhabited", e.Planet(s.Star, s.Orbit).Name)
	}
	if o.Kind == Invade {
		if err := e.invadeErr(s); err != nil {
			return err
		}
	}
	if err := e.blockedOrder(s, o); err != nil {
		return err
	}
	if o.Kind == GoToSystem {
		if _, err := e.Route(s, o.Star); err != nil {
			return err
		}
	}
	s.Order = o
	if e.journal != nil {
		e.recordOrder(s, o)
	}
	return nil
}

// LeaveOrbit takes s out of its orbit now, for free (as the original): to
// the FreeSpot of LeavePoint off obstacles (Game.Obstacles), its order
// cleared.
func (e *Empire) LeaveOrbit(s *Starship, obstacles []Vec3) error {
	if err := e.command(s); err != nil {
		return err
	}
	if s.Lane >= 0 || s.Orbit < 0 {
		return fmt.Errorf("%s is not in orbit", s.Name)
	}
	pl := s.Orbit
	s.Pos = FreeSpot(LeavePoint(e.Cluster, s.Star, s.Orbit), obstacles)
	s.Orbit, s.Order = -1, Order{}
	if e.journal != nil {
		en := e.shipEntry("leave", s)
		en.Star, en.Planet = s.Star, pl
		e.journal.Record(en)
	}
	return nil
}

// CanAbandon reports why Abandon would refuse s, nil when it would not:
// not commandable, in a star lane, colonists aboard (Unload them first).
func (e *Empire) CanAbandon(s *Starship) error {
	if err := e.command(s); err != nil {
		return err
	}
	switch {
	case s.Lane >= 0:
		return fmt.Errorf("%s is in a star lane", s.Name)
	case s.Pop > 0:
		return fmt.Errorf("%s carries %d %s: unload first", s.Name, s.Pop, plural(s.Pop, "colonist", "colonists"))
	}
	return nil
}

// Abandon scraps s now, for nothing back (the original's button), unless
// CanAbandon refuses.
func (e *Empire) Abandon(s *Starship) error {
	if err := e.CanAbandon(s); err != nil {
		return err
	}
	destroyIn([]*Empire{e}, s)
	if e.journal != nil {
		en := e.shipEntry("abandon", s)
		en.Star = s.Star
		e.journal.Record(en)
	}
	return nil
}

// Spawn appends a new ship of d in orbit of planet pl of star, named
// ShipNameFor its design ("Ship" unnamed), loaded, hull full, shields on,
// no weapon chosen, refilled. No cost, cap or research: launch pays first.
func (e *Empire) Spawn(d Design, star, pl int) *Starship { return e.spawn(d, star, pl, "") }

// spawn is Spawn named name, ShipNameFor's when "".
func (e *Empire) spawn(d Design, star, pl int, name string) *Starship {
	if d.Name == "" {
		d.Name = "Ship"
	}
	if name == "" {
		name = e.ShipNameFor(d.Name)
	}
	s := &Starship{
		Name: name, Design: d,
		Star: star, Pos: e.Planet(star, pl).Pos, Orbit: pl, Lane: -1,
		Cargo: d.Colonizers(), Invaders: d.InvasionModules(), Species: e.Species, Shields: true, Weapon: -1,
	}
	s.Hull = s.MaxHull()
	s.Refill() // allocates Shots
	e.Ships = append(e.Ships, s)
	e.Visit(star)
	return s
}
