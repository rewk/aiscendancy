package game

import "slices"

// Fog of war: an empire knows a star once one of its ships or
// colonies has been there; until then only its position and type. A lane
// is known from either end, or learnt (LearnLane). Knowledge is live: a
// known system shows today's planets and owners (ASSUMPTION).
//
// Partial fog, the original's: a known system's foreign ships are seen
// with owner, hull size and position, not armament (ShipsSeen); a known
// lane shows who travels it and how far along (LaneShips, the starmap's
// pennants; the bots read only LaneTraffic's count). The bots decide from
// this view only. A scanner shows more (Scan; docs/rules.md "Scanners").
//
// The cloakers: an Orbital Cloaker standing in a
// planet's orbit hides that orbit from every empire but its owner, its
// structures and the ships in it (SeesOrbit, ShipsSeen); a Surface
// Cloaker hides the colony's surface (SeesSurface). ScanAll sees through.
// The bots are not blinded: they read ShipsSeenAll and
// HostilesAll, the same view without the cloakers.

// Visit records star as explored; the slice grows to the cluster's size.
func (e *Empire) Visit(star int) {
	if star < 0 {
		return
	}
	if n := len(e.Cluster.Stars); len(e.Visited) < n {
		e.Visited = append(e.Visited, make([]bool, n-len(e.Visited))...)
	}
	e.Visited[star] = true
}

// LearnLane records lane li as known whatever its ends; the slice grows
// to the cluster's lane count.
func (e *Empire) LearnLane(li int) {
	if li < 0 {
		return
	}
	if n := len(e.Cluster.Lanes); len(e.LanesKnown) < n {
		e.LanesKnown = append(e.LanesKnown, make([]bool, n-len(e.LanesKnown))...)
	}
	e.LanesKnown[li] = true
}

// Revisit Visits the stars of every colony and every ship out of a lane.
func (e *Empire) Revisit() {
	for _, c := range e.Colonies {
		e.Visit(c.Star)
	}
	for _, s := range e.Ships {
		if s.Lane < 0 {
			e.Visit(s.Star)
		}
	}
}

// Knows reports the star explored: its name, planets and lanes are
// visible.
func (e *Empire) Knows(star int) bool {
	return star >= 0 && star < len(e.Visited) && e.Visited[star]
}

// KnowsLane reports lane li visible: learnt (LanesKnown), one of its
// ends is known, or the species sees every lane (Oculons).
func (e *Empire) KnowsLane(li int) bool {
	if e.Species.Traits().KnowsStarLanes() || li >= 0 && li < len(e.LanesKnown) && e.LanesKnown[li] {
		return true
	}
	l := e.Cluster.Lanes[li]
	return e.Knows(l.A) || e.Knows(l.B)
}

// Present reports the empire at star today: a colony there, or a ship
// out of lanes.
func (e *Empire) Present(star int) bool {
	for _, c := range e.Colonies {
		if c.Star == star {
			return true
		}
	}
	for _, s := range e.Ships {
		if s.Star == star && s.Lane < 0 {
			return true
		}
	}
	return false
}

// StarLabel is how the empire names a star: its name when known, else
// its type ("Red Giant").
func (e *Empire) StarLabel(star int) string {
	st := e.Cluster.Stars[star]
	if e.Knows(star) {
		return st.Name
	}
	return st.Type.String()
}

// Sighting is a foreign ship as an empire sees it: owner, hull size and
// place, the rest only in Scan. ship lets game act on it unseen.
type Sighting struct {
	Empire int
	Hull   Hull
	Star   int
	Pos    Vec3
	Orbit  int
	Scan   *Scan // nil: not scanned
	// Dealt is the hull points the seeing empire's own shots took off the
	// ship (Starship.Dealt): known scanned or not.
	Dealt int
	ship  *Starship
}

// Scan is a foreign ship's status as a scanner reads it; its parts only
// through X Ray Megaglasses (xray), else Design is Name and Hull alone.
type Scan struct {
	Design  Design
	Parts   bool // Design.Parts is known (xray); false: "parts unknown"
	HullPts int  // hull points left
	Shields bool
	// ShieldsDown: on but dark (Starship.ShieldsDown): absorbing nothing.
	ShieldsDown bool
	// Absorb and ShieldHits (Starship's): what the shields take off a shot
	// and the hits they can still take; the window prints them only with
	// Parts (xray), as PowerPerDay.
	Absorb, ShieldHits int
	Cargo              int
	Energy             int // its energy left today (Starship.Power): can it still fire?
	MaxHull            int // Starship.MaxHull: the hull bar's full length
	// PowerPerDay (Design.PowerPerDay) scales the window's energy bar; the
	// window prints it only with Parts (xray).
	PowerPerDay int
}

// ScanRange is the best Scanner part's Range × RangeUnit, 0 without one.
// Scanners are passive, their Level ignored (range only).
func (s *Starship) ScanRange() float64 {
	r := 0
	for _, g := range s.Design.Parts {
		if in := &Gizmos[g]; in.Cat == Scanner {
			r = max(r, in.Range)
		}
	}
	return float64(r) * RangeUnit
}

// ObservationRange is an Observation Installation's scan Range from the
// planet (ASSUMPTION: the Hyperwave Tympanum's).
const ObservationRange = 100

// scanned reports t within ScanRange of one of e's ships at its star, or
// ObservationRange of one of its colonies'; a Cloaker blocks both.
// ScanAll: always.
func (e *Empire) scanned(t *Starship) bool {
	if e.ScanAll {
		return true
	}
	if t.Design.count(func(g Gizmo) bool { return g == Cloaker }) > 0 {
		return false
	}
	for _, c := range e.Colonies {
		if c.Star == t.Star && t.Lane < 0 && c.Planet.Count(ObservationInstallation) > 0 &&
			dist(c.Planet.Pos, t.Pos) <= ObservationRange*RangeUnit+1e-9 {
			return true
		}
	}
	for _, s := range e.Ships {
		if s.Star == t.Star && s.Lane < 0 && t.Lane < 0 {
			if r := s.ScanRange(); r > 0 && dist(s.Pos, t.Pos) <= r+1e-9 {
				return true
			}
		}
	}
	return false
}

// xray reports e reading t's parts once scanned: X Ray Megaglasses on one
// of e's ships at t's star ("view the contents of any ship you are able
// to scan"), or ScanAll.
func (e *Empire) xray(t *Starship) bool {
	if e.ScanAll {
		return true
	}
	for _, s := range e.Ships {
		if s.Star == t.Star && s.Lane < 0 && t.Lane < 0 &&
			slices.Contains(s.Design.Parts, XRayMegaglasses) {
			return true
		}
	}
	return false
}

// OrbitCloaked reports an Orbital Cloaker standing in p's orbit.
func (p *Planet) OrbitCloaked() bool { return slices.Contains(p.Orbit[:], OrbitalCloaker) }

// SurfaceCloaked reports a Surface Cloaker standing on p's surface.
func (p *Planet) SurfaceCloaked() bool { return p.Count(SurfaceCloaker) > 0 }

// SeesOrbit reports e seeing p's orbit, its structures and the ships in
// it: e owns p, ScanAll, or no Orbital Cloaker there. A nil e (the views'
// bare tests) sees everything.
func (e *Empire) SeesOrbit(p *Planet) bool {
	return e == nil || e.ScanAll || p.Owner == e.Index || !p.OrbitCloaked()
}

// SeesSurface reports e seeing p's surface, its structures and its
// colony's detail: SeesOrbit's rule for the Surface Cloaker.
func (e *Empire) SeesSurface(p *Planet) bool {
	return e == nil || e.ScanAll || p.Owner == e.Index || !p.SurfaceCloaked()
}

// ShipsSeen is every other empire's ship at star out of the lanes, in
// empire then Ships order, when e Knows star, less those in an orbit e
// does not SeesOrbit; scanned ones carry a Scan.
func (g *Game) ShipsSeen(e *Empire, star int) []Sighting { return g.sightings(e, star, false) }

// ShipsSeenAll is ShipsSeen through the cloakers: the bots' view.
func (g *Game) ShipsSeenAll(e *Empire, star int) []Sighting { return g.sightings(e, star, true) }

// sightings is ShipsSeen, the cloaked orbits' ships kept with all.
func (g *Game) sightings(e *Empire, star int, all bool) []Sighting {
	if !e.Knows(star) {
		return nil
	}
	pls := g.Cluster.Stars[star].System.Planets
	var out []Sighting
	for _, o := range g.Empires {
		if o == e {
			continue
		}
		for _, s := range o.Ships {
			if s.Star == star && s.Lane < 0 && (all || s.Orbit < 0 || e.SeesOrbit(pls[s.Orbit])) {
				sg := Sighting{Empire: o.Index, Hull: s.Design.Hull, Star: s.Star, Pos: s.Pos, Orbit: s.Orbit, ship: s}
				if e.Index < len(s.Dealt) {
					sg.Dealt = s.Dealt[e.Index]
				}
				if e.scanned(s) {
					d, parts := Design{Name: s.Design.Name, Hull: s.Design.Hull}, e.xray(s)
					if parts {
						d.Parts = slices.Clone(s.Design.Parts)
					}
					sg.Scan = &Scan{Design: d, Parts: parts, HullPts: s.Hull, Shields: s.Shields, ShieldsDown: s.ShieldsDown, Absorb: s.Absorb(), ShieldHits: s.ShieldHits(), Cargo: s.Cargo, Energy: s.Power,
						MaxHull: s.MaxHull(), PowerPerDay: s.Design.PowerPerDay()}
				}
				out = append(out, sg)
			}
		}
	}
	return out
}

// Hostiles is ShipsSeen less the empires not Hostile to e.
func (g *Game) Hostiles(e *Empire, star int) []Sighting { return g.hostile(e, g.ShipsSeen(e, star)) }

// HostilesAll is Hostiles through the cloakers (ShipsSeenAll): the bots'.
func (g *Game) HostilesAll(e *Empire, star int) []Sighting {
	return g.hostile(e, g.ShipsSeenAll(e, star))
}

// Of reports sg is the sighting of s (the views pair bodies with it).
func (sg Sighting) Of(s *Starship) bool { return sg.ship == s }

// hostile keeps of seen the empires Hostile to e.
func (g *Game) hostile(e *Empire, seen []Sighting) []Sighting {
	var out []Sighting
	for _, sg := range seen {
		if g.Hostile(e, g.Empires[sg.Empire]) {
			out = append(out, sg)
		}
	}
	return out
}

// LaneSighting is a ship in a lane as an empire sees it on the starmap:
// its empire, its lane, the star it left and how far along, 0 to 1
// (Starship.LaneProgress).
type LaneSighting struct {
	Empire, Lane, From int
	Frac               float64
}

// LaneShips is every ship e sees in a lane: its own, then the others' on
// the lanes e KnowsLane, in empire then Ships order.
func (g *Game) LaneShips(e *Empire) []LaneSighting {
	var out []LaneSighting
	add := func(o *Empire, s *Starship) {
		out = append(out, LaneSighting{o.Index, s.Lane, s.Star, s.LaneProgress(g.Cluster, o.Day)})
	}
	for _, s := range e.Ships {
		if s.Lane >= 0 {
			add(e, s)
		}
	}
	for _, o := range g.Empires {
		if o == e {
			continue
		}
		for _, s := range o.Ships {
			if s.Lane >= 0 && e.KnowsLane(s.Lane) {
				add(o, s)
			}
		}
	}
	return out
}

// LaneTraffic is how many other empires' ships travel lane, when e
// KnowsLane it, else 0.
func (g *Game) LaneTraffic(e *Empire, lane int) int {
	if !e.KnowsLane(lane) {
		return 0
	}
	n := 0
	for _, o := range g.Empires {
		if o == e {
			continue
		}
		for _, s := range o.Ships {
			if s.Lane == lane {
				n++
			}
		}
	}
	return n
}
