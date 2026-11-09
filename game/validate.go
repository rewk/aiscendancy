package game

import (
	"errors"
	"fmt"
	"math"
)

// The enums with no Num* constant, bounded by their last value: each
// String indexes an array of exactly that many names (TestEnumBounds).
const (
	numStances    = Alliance + 1
	numOfferKinds = AskAllianceWith + 1
	numOrderKinds = GoToSystem + 1
	numVictories  = UniteVictory + 1
	numEventKinds = Blockade + 1
)

// maxCoord bounds every saved coordinate: the cluster is the unit ball, a
// system lies within SystemReach; anything far past is a tampered file
// (positions in the 1e300s overflow the distances to Inf).
const maxCoord = 1000

// Validate checks every saved number of g that the rules (and the views)
// later use as an index or a table key against its range, and the slices
// whose lengths they assume, so that a decoded save (save.Read) cannot
// panic in play: no nil pointer, the enums under their Num* bounds, the
// star, lane, planet, slot and empire indices, each planet's squares
// exactly its Layout, the designs' parts within their hull's slots
// (checkSlots), Shots parallel to the parts, the counts not negative, a
// planet's Owner holding a colony there and each colony its planet. It
// returns the first problem, with a path ("empire 1: colony 3: ...").
//
// It reads Colony.Star and PlanetIdx, never Colony.Planet or any other
// pointer Relink sets: it runs before Relink, on a decoded save, and on
// a live game alike.
func (g *Game) Validate() error {
	n := len(g.Empires)
	switch {
	case g.Cluster == nil:
		return errors.New("no cluster")
	case n == 0:
		return errors.New("no empire")
	}
	if err := g.Cluster.validate(n); err != nil {
		return err
	}
	switch {
	case n > int(NumSpecies) || n > len(g.Cluster.Stars):
		return fmt.Errorf("%d empires on %d stars, at most %d species", n, len(g.Cluster.Stars), NumSpecies)
	case g.Day < 0:
		return fmt.Errorf("day %d", g.Day)
	case g.Atmosphere < Peaceful || g.Atmosphere > Hostile:
		return fmt.Errorf("atmosphere %d", g.Atmosphere)
	case len(g.RoundsToday) != 0 && len(g.RoundsToday) != len(g.Cluster.Stars):
		return fmt.Errorf("rounds today for %d stars of %d", len(g.RoundsToday), len(g.Cluster.Stars))
	}
	for st, r := range g.RoundsToday {
		if r < 0 {
			return fmt.Errorf("star %d: %d rounds today", st, r)
		}
	}
	for i, o := range g.Offers {
		switch {
		case o.From < 0 || o.From >= n || o.To < 0 || o.To >= n:
			return fmt.Errorf("offer %d: from %d to %d of %d empires", i, o.From, o.To, n)
		case o.Kind >= numOfferKinds:
			return fmt.Errorf("offer %d: kind %d", i, o.Kind)
		case o.Third < -1 || o.Third >= n || o.Kind.Request() && o.Third < 0: // the views name a request's Third
			return fmt.Errorf("offer %d: third %d of %d empires", i, o.Third, n)
		}
	}
	for i, e := range g.Empires {
		if e == nil {
			return fmt.Errorf("empire %d: missing", i)
		}
		if err := g.validEmpire(i, e); err != nil {
			return fmt.Errorf("empire %d: %w", i, err)
		}
	}
	// The planets' owners against the colonies: each Owner holds a colony
	// there (validEmpire checked the converse, and no place twice).
	for st, s := range g.Cluster.Stars {
		for pl, p := range s.System.Planets {
			if p.Owner >= 0 && !g.Empires[p.Owner].hasColony(st, pl) {
				return fmt.Errorf("star %d: planet %d: owner %d has no colony there", st, pl, p.Owner)
			}
		}
	}
	return nil
}

// hasColony reports a colony of e at planet pl of star st, by index
// (Colony.Planet may not be linked yet).
func (e *Empire) hasColony(st, pl int) bool {
	for _, c := range e.Colonies {
		if c.Star == st && c.PlanetIdx == pl {
			return true
		}
	}
	return false
}

// span is nil when lo ≤ v ≤ hi, else what is out of which range.
func span(what string, v, lo, hi int) error {
	if v < lo || v > hi {
		return fmt.Errorf("%s %d out of %d..%d", what, v, lo, hi)
	}
	return nil
}

// finite refuses a position off maxCoord (JSON holds no NaN nor Inf).
func finite(what string, v Vec3) error {
	for _, x := range [3]float64{v.X, v.Y, v.Z} {
		if math.IsNaN(x) || math.Abs(x) > maxCoord {
			return fmt.Errorf("%s %v out of reach", what, v)
		}
	}
	return nil
}

// validate checks the cluster: its stars' types, lanes (both ends real
// and distinct, A < B), each star's Lanes naming lanes that touch it and
// matching its System.Lanes one for one (laneEnd: endpoint k is lane
// Lanes[k], To its other end), Home, and every planet (validPlanet, owners
// among n empires).
func (c *Cluster) validate(n int) error {
	ns := len(c.Stars)
	switch {
	case c.Density >= NumDensities:
		return fmt.Errorf("density %d", c.Density)
	case ns == 0:
		return errors.New("no star")
	}
	if err := span("home star", c.Home, 0, ns-1); err != nil {
		return err
	}
	for i, l := range c.Lanes {
		if l.A < 0 || l.A >= l.B || l.B >= ns {
			return fmt.Errorf("lane %d: stars %d-%d of %d", i, l.A, l.B, ns)
		}
	}
	for i, s := range c.Stars {
		if err := c.validStar(i, &s, n); err != nil {
			return fmt.Errorf("star %d: %w", i, err)
		}
	}
	return nil
}

// validStar checks star i of c (validate).
func (c *Cluster) validStar(i int, s *Star, n int) error {
	sys := s.System
	switch {
	case s.Type >= NumStarTypes:
		return fmt.Errorf("type %d", s.Type)
	case sys == nil:
		return errors.New("no system")
	case sys.Star >= NumStarTypes:
		return fmt.Errorf("system type %d", sys.Star)
	case len(s.Lanes) != len(sys.Lanes):
		return fmt.Errorf("%d lanes, %d endpoints", len(s.Lanes), len(sys.Lanes))
	case len(sys.Planets) > MaxSystemPlanets:
		return fmt.Errorf("%d planets, at most %d", len(sys.Planets), MaxSystemPlanets)
	}
	if err := finite("position", s.Pos); err != nil {
		return err
	}
	for k, li := range s.Lanes {
		if err := span(fmt.Sprintf("lane %d: index", k), li, 0, len(c.Lanes)-1); err != nil {
			return err
		}
		l := c.Lanes[li]
		if l.A != i && l.B != i {
			return fmt.Errorf("lane %d: %d-%d does not touch it", li, l.A, l.B)
		}
		if to := c.Other(li, i); sys.Lanes[k].To != to {
			return fmt.Errorf("endpoint %d: to %d, its lane %d leads to %d", k, sys.Lanes[k].To, li, to)
		}
		if err := finite(fmt.Sprintf("endpoint %d", k), sys.Lanes[k].Pos); err != nil {
			return err
		}
	}
	for pl, p := range sys.Planets {
		if p == nil {
			return fmt.Errorf("planet %d: missing", pl)
		}
		if err := p.validate(n); err != nil {
			return fmt.Errorf("planet %d: %w", pl, err)
		}
	}
	return nil
}

// validate checks p: type, size, its Surface exactly Layout(Size) (Cells
// and their indexing assume every cell present, nothing else), each
// square's kind and surface item, each orbit slot's orbital item, the
// owner among n empires, no negative count.
func (p *Planet) validate(n int) error {
	switch {
	case p.Type >= NumPlanetTypes:
		return fmt.Errorf("type %d", p.Type)
	case p.Size < MinSize || p.Size > MaxSize:
		return fmt.Errorf("size %d out of %d..%d", p.Size, MinSize, MaxSize)
	case p.PopBonus < 0:
		return fmt.Errorf("pop bonus %d", p.PopBonus)
	}
	if err := span("owner", p.Owner, -1, n-1); err != nil {
		return err
	}
	if err := finite("position", p.Pos); err != nil {
		return err
	}
	cells := Layout(p.Size)
	if len(p.Surface) != len(cells) {
		return fmt.Errorf("%d squares, size %d has %d", len(p.Surface), p.Size, len(cells))
	}
	for _, c := range cells {
		sq := p.Surface[c]
		switch {
		case sq == nil:
			return fmt.Errorf("square %s missing", c)
		case sq.Kind >= NumSquareKinds:
			return fmt.Errorf("square %s: kind %d", c, sq.Kind)
		case sq.Item != NoItem && !sq.Item.Surface():
			return fmt.Errorf("square %s: item %d not a surface structure", c, sq.Item)
		}
	}
	for k, it := range p.Orbit {
		if it != NoItem && !it.Orbital() {
			return fmt.Errorf("orbit slot %d: item %d not orbital", k, it)
		}
		if p.OrbitDamage[k] < 0 {
			return fmt.Errorf("orbit slot %d: damage %d", k, p.OrbitDamage[k])
		}
	}
	return nil
}

// validEmpire checks empire i of g (Validate).
func (g *Game) validEmpire(i int, e *Empire) error {
	c, n := g.Cluster, len(g.Empires)
	switch {
	case e.Index != i:
		return fmt.Errorf("index %d", e.Index)
	case e.Species >= NumSpecies:
		return fmt.Errorf("species %d", e.Species)
	case e.Colour < 0 || e.Colour >= NumColours:
		return fmt.Errorf("colour %d", e.Colour)
	case e.Research == nil:
		return errors.New("no research")
	case len(e.Visited) > len(c.Stars):
		return fmt.Errorf("%d stars visited of %d", len(e.Visited), len(c.Stars))
	case len(e.LanesKnown) > len(c.Lanes):
		return fmt.Errorf("%d lanes known of %d", len(e.LanesKnown), len(c.Lanes))
	case len(e.Relations) != n:
		return fmt.Errorf("%d relations for %d empires", len(e.Relations), n)
	case e.Won >= numVictories:
		return fmt.Errorf("victory %d", e.Won)
	case e.Day < 0 || e.Lost < 0 || e.WonDay < 0 || e.HospitalityGiven < 0:
		return fmt.Errorf("day %d, lost %d, won %d, hospitality %d", e.Day, e.Lost, e.WonDay, e.HospitalityGiven)
	}
	if err := e.Research.validate(); err != nil {
		return fmt.Errorf("research: %w", err)
	}
	for j, r := range e.Relations {
		if err := r.validate(); err != nil {
			return fmt.Errorf("relation %d: %w", j, err)
		}
	}
	for li := range e.LaneBlocks {
		if err := span("blocked lane", li, -1, len(c.Lanes)-1); err != nil {
			return err
		}
	}
	for k, t := range e.Templates {
		if err := t.validate(); err != nil {
			return fmt.Errorf("template %d: %w", k, err)
		}
	}
	seen := map[[2]int]bool{}
	for k, col := range e.Colonies {
		if col == nil {
			return fmt.Errorf("colony %d: missing", k)
		}
		if err := g.validColony(i, col); err != nil {
			return fmt.Errorf("colony %d: %w", k, err)
		}
		at := [2]int{col.Star, col.PlanetIdx}
		if seen[at] {
			return fmt.Errorf("colony %d: a second colony on planet %d of star %d", k, col.PlanetIdx, col.Star)
		}
		seen[at] = true
	}
	for k, s := range e.Ships {
		if s == nil {
			return fmt.Errorf("ship %d: missing", k)
		}
		if err := g.validShip(s); err != nil {
			return fmt.Errorf("ship %d: %w", k, err)
		}
	}
	return nil
}

// validate checks the topic (NoTech or a tech) and the points sunk.
func (r *Research) validate() error {
	if r.Topic != NoTech && r.Topic >= NumTechs {
		return fmt.Errorf("topic %d", r.Topic)
	}
	if r.Stars < 0 {
		return fmt.Errorf("stars %d", r.Stars)
	}
	for t, p := range r.Progress {
		if p < 0 {
			return fmt.Errorf("tech %d: progress %d", t, p)
		}
	}
	return nil
}

// validate checks the stance and the ledger: days −1 (never) or more,
// counts not negative.
func (r Relation) validate() error {
	switch {
	case r.Stance >= numStances:
		return fmt.Errorf("stance %d", r.Stance)
	case r.Met < -1 || r.LastOffer < -1:
		return fmt.Errorf("met %d, last offer %d", r.Met, r.LastOffer)
	case !(r.Friendship >= 0) || math.IsInf(r.Friendship, 0):
		return fmt.Errorf("friendship %v", r.Friendship)
	}
	for _, x := range [...]int{r.WarDays, r.WarsOn, r.ShipsLost, r.PlanetsLost, r.Intrusions, r.Refused, r.AllyDays,
		r.Broken, r.Trades, r.Hospitality, r.Helped, r.Defamed, r.Encroached} {
		if x < 0 {
			return fmt.Errorf("a negative count %d", x)
		}
	}
	return nil
}

// validate checks a template as AddTemplate does, research aside: the
// hull, at most its slots, each fixed part a gizmo and each Best part a
// category, Slots (checkSlots).
func (t Template) validate() error {
	if t.Hull >= NumHulls {
		return fmt.Errorf("hull %d", t.Hull)
	}
	if len(t.Parts) > t.Hull.Slots() {
		return fmt.Errorf("%d parts on a %s hull of %d slots", len(t.Parts), t.Hull, t.Hull.Slots())
	}
	for k, p := range t.Parts {
		if p.Best && p.Cat >= NumGizmoCats || !p.Best && p.Gizmo >= NumGizmos {
			return fmt.Errorf("part %d: gizmo %d, category %d", k, p.Gizmo, p.Cat)
		}
	}
	return checkSlots(t.Slots, len(t.Parts), t.Hull)
}

// validate checks a design: the hull, its parts gizmos and at most its
// slots, Slots (checkSlots).
func (d *Design) validate() error {
	if d.Hull >= NumHulls {
		return fmt.Errorf("hull %d", d.Hull)
	}
	if len(d.Parts) > d.Hull.Slots() {
		return fmt.Errorf("%d parts on a %s hull of %d slots", len(d.Parts), d.Hull, d.Hull.Slots())
	}
	for k, gz := range d.Parts {
		if gz >= NumGizmos {
			return fmt.Errorf("part %d: gizmo %d", k, gz)
		}
	}
	return checkSlots(d.Slots, len(d.Parts), d.Hull)
}

// validColony checks a colony of empire i: its place (the planet's Owner
// i), the counts, the project and the queue against the planet.
func (g *Game) validColony(i int, col *Colony) error {
	stars := g.Cluster.Stars
	if err := span("star", col.Star, 0, len(stars)-1); err != nil {
		return err
	}
	pls := stars[col.Star].System.Planets
	if err := span(fmt.Sprintf("star %d: planet", col.Star), col.PlanetIdx, 0, len(pls)-1); err != nil {
		return err
	}
	p := pls[col.PlanetIdx]
	if p.Owner != i {
		return fmt.Errorf("planet %d of star %d owned by %d", col.PlanetIdx, col.Star, p.Owner)
	}
	if col.Pop < 0 || col.Growth < 0 || col.Carry < 0 || col.Tractors < 0 {
		return fmt.Errorf("pop %d, growth %d, carry %d, tractors %d", col.Pop, col.Growth, col.Carry, col.Tractors)
	}
	if err := span("blocked by", col.BlockedBy, 0, len(g.Empires)); err != nil {
		return err
	}
	for k, x := range col.OrbitShots {
		if x < 0 {
			return fmt.Errorf("orbit slot %d: %d shots", k, x)
		}
	}
	if pj := col.Project; pj != nil {
		if err := pj.validate(p); err != nil {
			return fmt.Errorf("project: %w", err)
		}
	}
	for k, q := range col.Queue {
		if err := validTarget(p, q.Item, q.Cell, q.Slot, q.Orbital); err != nil {
			return fmt.Errorf("queue %d: %w", k, err)
		}
	}
	return nil
}

// validTarget checks an item and its target on p, a Project's or a
// Queued's: the item, an orbital one's slot (−1 a refit's), a square
// one's cell on the surface (the completions index Surface[Cell]).
func validTarget(p *Planet, it Item, at Cell, slot int, orbital bool) error {
	if err := span("item", int(it), 0, int(NumItems)-1); err != nil {
		return err
	}
	if orbital {
		return span("slot", slot, -1, OrbitSlots-1)
	}
	if _, ok := p.Surface[at]; !ok && (it.OnSquare() || it == Automation) {
		return fmt.Errorf("%s: no square %s", it, at)
	}
	return nil
}

// validate checks a project on p: its target (validTarget, but a refit's,
// which has none), a Ship's or a refit's design, an Automation's Target
// a structure, the counts.
func (pj *Project) validate(p *Planet) error {
	if pj.Refit == "" {
		if err := validTarget(p, pj.Item, pj.Cell, pj.Slot, pj.Orbital); err != nil {
			return err
		}
	} else if err := span("item", int(pj.Item), 0, int(NumItems)-1); err != nil {
		return err
	}
	switch {
	case pj.Progress < 0 || pj.RefitCost < 0:
		return fmt.Errorf("progress %d, refit cost %d", pj.Progress, pj.RefitCost)
	case pj.Design == nil && (pj.Item == Ship || pj.Refit != ""):
		return errors.New("no design")
	case pj.Item == Automation && (pj.Target < 0 || pj.Target >= NumItems):
		return fmt.Errorf("automation of item %d", pj.Target)
	case pj.Target < NoItem || pj.Target >= NumItems:
		return fmt.Errorf("target %d", pj.Target)
	}
	if pj.Design != nil {
		if err := pj.Design.validate(); err != nil {
			return fmt.Errorf("design: %w", err)
		}
	}
	return nil
}

// validShip checks a starship: its design, its place (star, orbit, a lane
// touching its star), the order's star or planet, the counts, Dealt by
// empire, Shots parallel to the parts (Refill keeps them so), Weapon.
func (g *Game) validShip(s *Starship) error {
	c := g.Cluster
	if err := s.Design.validate(); err != nil {
		return fmt.Errorf("design: %w", err)
	}
	if s.Species >= NumSpecies {
		return fmt.Errorf("species %d", s.Species)
	}
	if err := span("star", s.Star, 0, len(c.Stars)-1); err != nil {
		return err
	}
	npl := len(c.Stars[s.Star].System.Planets)
	if err := span("orbit", s.Orbit, -1, npl-1); err != nil {
		return err
	}
	if err := span("lane", s.Lane, -1, len(c.Lanes)-1); err != nil {
		return err
	}
	if l := s.Lane; l >= 0 && c.Lanes[l].A != s.Star && c.Lanes[l].B != s.Star {
		return fmt.Errorf("lane %d does not touch star %d", l, s.Star)
	}
	if err := finite("position", s.Pos); err != nil {
		return err
	}
	if s.Cargo < 0 || s.Invaders < 0 || s.Pop < 0 {
		return fmt.Errorf("cargo %d, invaders %d, pop %d", s.Cargo, s.Invaders, s.Pop)
	}
	o := s.Order
	if o.Kind >= numOrderKinds {
		return fmt.Errorf("order %d", o.Kind)
	}
	switch o.Kind {
	case GoToPlanet:
		if err := span("order: planet", o.Planet, 0, npl-1); err != nil {
			return err
		}
	case GoToStar, GoToSystem:
		if err := span("order: star", o.Star, 0, len(c.Stars)-1); err != nil {
			return err
		}
	}
	if err := finite("order: point", o.Pos); err != nil {
		return err
	}
	if len(s.Dealt) > len(g.Empires) {
		return fmt.Errorf("damage dealt by %d empires of %d", len(s.Dealt), len(g.Empires))
	}
	for e, d := range s.Dealt {
		if d < 0 {
			return fmt.Errorf("damage dealt by %d: %d", e, d)
		}
	}
	if len(s.Shots) != len(s.Design.Parts) {
		return fmt.Errorf("shots for %d parts of %d", len(s.Shots), len(s.Design.Parts))
	}
	for k, x := range s.Shots {
		if x < 0 {
			return fmt.Errorf("part %d: %d shots", k, x)
		}
	}
	return span("weapon", s.Weapon, -1, len(s.Design.Parts)-1)
}

// ValidateEvent checks a logged event against g (save.File.Log): its
// kind, item, tech, star, planet in that star's system, other empire;
// each −1 (NoItem, NoTech) when it does not apply.
func (g *Game) ValidateEvent(ev Event) error {
	ns := len(g.Cluster.Stars)
	switch {
	case ev.Kind >= numEventKinds:
		return fmt.Errorf("event kind %d", ev.Kind)
	case ev.Item < NoItem || ev.Item >= NumItems:
		return fmt.Errorf("event item %d", ev.Item)
	case ev.Tech != NoTech && ev.Tech >= NumTechs:
		return fmt.Errorf("event tech %d", ev.Tech)
	}
	if err := span("event star", ev.Star, -1, ns-1); err != nil {
		return err
	}
	npl := MaxSystemPlanets
	if ev.Star >= 0 {
		npl = len(g.Cluster.Stars[ev.Star].System.Planets)
	}
	if err := span("event planet", ev.Planet, -1, npl-1); err != nil {
		return err
	}
	return span("event empire", ev.Other, -1, len(g.Empires)-1)
}
