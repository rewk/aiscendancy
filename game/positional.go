package game

import (
	"errors"
	"fmt"
)

// The positional and lane specials (docs/rules.md "Ship specials"):
// UseSpecial's effects that move ships in a system, and those aimed at a
// star lane (a LaneTarget: Index in the user's System.Lanes). targetCheck
// has refused a bad target. Numbers from gizmos.txt, quotes gizhelp.txt.
//
// A displacement stays within SystemReach, never ends within ShipGap of a
// ship in space nor PlanetClear of a planet (clearPath: it stops at the
// farthest clear point; one that cannot move is "not moved", the use spent
// anyway); a moved ship leaves its orbit, its order kept. An area effect
// collects its victims (in space, empire then Ships order), then moves
// them one by one against the live positions.

// specialReach is Special1 or Special2 as a distance. ASSUMPTION: in
// RangeUnit (the Positron Bouncer's 100 is a system radius).
func specialReach(n int) float64 { return float64(n) * RangeUnit }

// keepOff is what a displaced s at star keeps off: the other ships of e's
// peers in space there (ShipGap) and the planets (PlanetClear).
func keepOff(e *Empire, star int, s *Starship) []sphere {
	out := gaps(spaceAt(e.peerList(), star, s))
	for _, p := range e.Cluster.Stars[star].System.Planets {
		out = append(out, sphere{p.Pos, PlanetClear})
	}
	return out
}

// inReach is p pulled back onto the SystemReach sphere when beyond it.
func inReach(p Vec3) Vec3 {
	d := dist(p, Vec3{})
	if d <= SystemReach {
		return p
	}
	f := SystemReach / d
	return Vec3{p.X * f, p.Y * f, p.Z * f}
}

// unit is the direction from a to b, +X when they coincide.
func unit(a, b Vec3) Vec3 {
	d := dist(a, b)
	if d < 1e-12 {
		return Vec3{X: 1}
	}
	return Vec3{(b.X - a.X) / d, (b.Y - a.Y) / d, (b.Z - a.Z) / d}
}

// along is p + k·u.
func along(p, u Vec3, k float64) Vec3 { return Vec3{p.X + k*u.X, p.Y + k*u.Y, p.Z + k*u.Z} }

// displace moves s towards to (inReach), stopped at clearPath's point
// against keepOff; moved false when it ends within 1e-9 of where it was.
func displace(e *Empire, s *Starship, to Vec3) (moved bool) {
	end := clearPath(s.Pos, inReach(to), keepOff(e, s.Star, s))
	if dist(end, s.Pos) < 1e-9 {
		return false
	}
	s.Pos, s.Orbit = end, -1
	return true
}

// spaceNear is every ship of e's peers in space at star within r of c,
// not excepted, in empire then Ships order.
func spaceNear(e *Empire, star int, c Vec3, r float64, not *Starship) []*Starship {
	var out []*Starship
	for _, o := range e.peerList() {
		for _, x := range o.Ships {
			if x != not && x.Star == star && x.InSpace() && dist(c, x.Pos) <= r+1e-9 {
				out = append(out, x)
			}
		}
	}
	return out
}

// movedText is a displacement's report clause.
// movedOutcome is a push's or pull's SpecialUse outcome.
func movedOutcome(moved bool) string {
	if moved {
		return "moved"
	}
	return "not moved"
}

func movedText(name string, moved bool) string {
	if moved {
		return name + " is moved"
	}
	return name + " does not move"
}

// Positron Bouncer: "buffeting it away without damaging it": the target
// pushed Special1 straight away from the user.
func useBouncer(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	m := displace(e, tg, along(tg.Pos, unit(s.Pos, tg.Pos), specialReach(Gizmos[PositronBouncer].Special1)))
	return fmt.Sprintf("%s's Positron Bouncer: %s", s.Name, movedText(tg.Name, m)), movedOutcome(m), nil
}

// refuseTractor: a target alongside (ShipGap or closer) has nowhere to
// come.
func refuseTractor(e *Empire, s *Starship, part int, t Target) error {
	if dist(s.Pos, t.ship().Pos) <= ShipGap+1e-9 {
		return errors.New("already alongside")
	}
	return nil
}

// Tractor Beam: "allows a ship to pull another ship toward it": the
// target (hostile or friendly) pulled Special1 toward the user, at most
// to ShipGap from it.
func useTractor(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	k := min(specialReach(Gizmos[GizmoTractorBeam].Special1), dist(s.Pos, tg.Pos)-ShipGap)
	m := displace(e, tg, along(tg.Pos, unit(tg.Pos, s.Pos), k))
	return fmt.Sprintf("%s's Tractor Beam: %s", s.Name, movedText(tg.Name, m)), movedOutcome(m), nil
}

// Hyperswapper: "causes them to swap positions": the user and the target
// (both in space) exchange their Pos exactly.
func useSwapper(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	s.Pos, tg.Pos = tg.Pos, s.Pos
	return fmt.Sprintf("%s swaps places with %s", s.Name, tg.Name), "swapped", nil
}

// refuseArea is an area effect's refusal: nobody to move.
func refuseArea(vs []*Starship) error {
	if len(vs) == 0 {
		return errors.New("no ship in range")
	}
	return nil
}

// toRadius displaces each of vs along the line from c to r from c (out
// for the nearer ones, in for the farther: the callers' picks), one by
// one; the count moved.
func toRadius(e *Empire, vs []*Starship, c Vec3, r float64) int {
	n := 0
	for _, v := range vs {
		if displace(e, v, along(c, unit(c, v.Pos), r)) {
			n++
		}
	}
	return n
}

// areaText is an area effect's report line.
// areaOutcome is an area special's SpecialUse outcome: the ships moved.
func areaOutcome(n int) string { return fmt.Sprintf("%d %s moved", n, plural(n, "ship", "ships")) }

func areaText(s *Starship, dev string, n int) string {
	return fmt.Sprintf("%s's %s moves %d %s", s.Name, dev, n, plural(n, "ship", "ships"))
}

// disperserVictims: every ship in space within Special1 of the target
// but the target (the user included).
func disperserVictims(e *Empire, s *Starship, t Target) []*Starship {
	tg := t.ship()
	return spaceNear(e, s.Star, tg.Pos, specialReach(Gizmos[FleetDisperser].Special1), tg)
}

func refuseDisperser(e *Empire, s *Starship, part int, t Target) error {
	return refuseArea(disperserVictims(e, s, t))
}

// Fleet Disperser: "repels other ships away from its target": every other
// ship in space within Special1 of the target, the user included, pushed
// out to Special1 from it.
func useDisperser(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	c := t.ship().Pos
	n := toRadius(e, disperserVictims(e, s, t), c, specialReach(Gizmos[FleetDisperser].Special1))
	return areaText(s, "Fleet Disperser", n), areaOutcome(n), nil
}

func distorterVictims(e *Empire, s *Starship) []*Starship {
	return spaceNear(e, s.Star, s.Pos, specialReach(Gizmos[GravityDistorter].Special1), s)
}

func refuseDistorter(e *Empire, s *Starship, part int, _ Target) error {
	return refuseArea(distorterVictims(e, s))
}

// Gravity Distorter: "pushes small objects, such as space vessels, away
// from the ship": every other ship in space within Special1 of the user
// pushed out to Special1.
func useDistorter(e *Empire, s *Starship, part int, _ Target) (string, string, []Report) {
	n := toRadius(e, distorterVictims(e, s), s.Pos, specialReach(Gizmos[GravityDistorter].Special1))
	return areaText(s, "Gravity Distorter", n), areaOutcome(n), nil
}

// massVictims: within Special1 of the target, the target excepted (the
// user included), farther than Special2 (one closer is not pulled).
func massVictims(e *Empire, s *Starship, t Target) []*Starship {
	in := &Gizmos[MassCondensor]
	tg := t.ship()
	return farther(spaceNear(e, s.Star, tg.Pos, specialReach(in.Special1), tg), tg.Pos, specialReach(in.Special2))
}

// farther is the ships of vs farther than r from c.
func farther(vs []*Starship, c Vec3, r float64) []*Starship {
	var out []*Starship
	for _, v := range vs {
		if dist(c, v.Pos) > r+1e-9 {
			out = append(out, v)
		}
	}
	return out
}

func refuseMass(e *Empire, s *Starship, part int, t Target) error {
	return refuseArea(massVictims(e, s, t))
}

// Mass Condensor: "All ships will be attracted toward the affected ship":
// every other ship in space within Special1 of the target, the user
// included, pulled down to Special2 (5: ShipGap).
func useMass(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	n := toRadius(e, massVictims(e, s, t), t.ship().Pos, specialReach(Gizmos[MassCondensor].Special2))
	return areaText(s, "Mass Condensor", n), areaOutcome(n), nil
}

// condensorVictims: every ship in space at the star (the user included)
// within Special1 of the star, farther than Special2.
func condensorVictims(e *Empire, s *Starship) []*Starship {
	in := &Gizmos[GravimetricCondensor]
	return farther(spaceNear(e, s.Star, Vec3{}, specialReach(in.Special1), nil), Vec3{}, specialReach(in.Special2))
}

func refuseCondensor(e *Empire, s *Starship, part int, _ Target) error {
	return refuseArea(condensorVictims(e, s))
}

// Gravimetric Condensor: "all the ships in the system to be pulled toward
// the star": every ship in space within Special1 of the origin, the user
// included, pulled to Special2 from it.
func useCondensor(e *Empire, s *Starship, part int, _ Target) (string, string, []Report) {
	n := toRadius(e, condensorVictims(e, s), Vec3{}, specialReach(Gizmos[GravimetricCondensor].Special2))
	return areaText(s, "Gravimetric Condensor", n), areaOutcome(n), nil
}

// refuseCatapult: at the star (within ShipGap of it) there is no
// opposite side.
func refuseCatapult(e *Empire, s *Starship, part int, _ Target) error {
	if dist(s.Pos, Vec3{}) < ShipGap {
		return errors.New("at the star")
	}
	return nil
}

// Gravimetric Catapult: "whips past the sun, stopping opposite its
// previous position": Pos → −Pos, then freeOf keepOff, out of orbit.
func useCatapult(e *Empire, s *Starship, part int, _ Target) (string, string, []Report) {
	s.Pos = freeOf(inReach(Vec3{-s.Pos.X, -s.Pos.Y, -s.Pos.Z}), keepOff(e, s.Star, s))
	s.Orbit = -1
	return fmt.Sprintf("%s whips round the star", s.Name), "round the star", nil
}

// laneOf is the cluster lane a LaneTarget names from s's star, and its
// far end.
func laneOf(e *Empire, s *Starship, t Target) (lane, far int) {
	lane = e.Cluster.Stars[s.Star].Lanes[t.Index]
	return lane, e.Cluster.Other(lane, s.Star)
}

// refuseLaneBlocker: a lane e blocks already today.
func refuseLaneBlocker(e *Empire, s *Starship, part int, t Target) error {
	if li, _ := laneOf(e, s, t); e.Day < e.LaneBlocks[li] {
		return errors.New("already blocked")
	}
	return nil
}

// Lane Blocker: "Star Lane Drives and Star Lane Hyperdrives can no longer
// overcome it": e.LaneBlocks[lane] = Day + BlockDays (the Arbryls'), the
// lane closed both ways to every other empire (LaneBlocker; the ships
// inside go on). ASSUMPTION: one use. Expired entries are dropped here.
func useLaneBlocker(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	li, far := laneOf(e, s, t)
	for k, until := range e.LaneBlocks {
		if until <= e.Day {
			delete(e.LaneBlocks, k)
		}
	}
	if e.LaneBlocks == nil {
		e.LaneBlocks = map[int]int{}
	}
	e.LaneBlocks[li] = e.Day + BlockDays
	return fmt.Sprintf("%s blocks the lane to %s for %d days", s.Name, e.StarLabel(far), BlockDays), fmt.Sprintf("blocked %d days", BlockDays), nil
}

// inLane is every ship of e's peers inside cluster lane li, either way.
func inLane(e *Empire, li int) []*Starship {
	var out []*Starship
	for _, o := range e.peerList() {
		for _, x := range o.Ships {
			if x.Lane == li {
				out = append(out, x)
			}
		}
	}
	return out
}

// refuseDestabilizer: nobody inside the lane.
func refuseDestabilizer(e *Empire, s *Starship, part int, t Target) error {
	if li, _ := laneOf(e, s, t); len(inLane(e, li)) == 0 {
		return errors.New("no ship in the lane")
	}
	return nil
}

// Lane Destabilizer: "the ships inside the star lane to be thrown quickly
// toward their destination point": every ship inside the lane, any empire
// either way, gets Arrive ≤ Day + 1 (out at the next close).
func useDestabilizer(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	li, far := laneOf(e, s, t)
	vs := inLane(e, li)
	for _, v := range vs {
		v.Arrive = min(v.Arrive, e.Day+1)
	}
	n := fmt.Sprintf("%d %s", len(vs), plural(len(vs), "ship", "ships"))
	return fmt.Sprintf("%s: %s hurried to %s", s.Name, n, e.StarLabel(far)), n + " hurried", nil
}

// refuseMagnetron is Order's refusals of the GoToStar through the lane
// (Check's, blockedOrder), and TiedDown.
func refuseMagnetron(e *Empire, s *Starship, part int, t Target) error {
	if s.TiedDown {
		return fmt.Errorf("%s is tied down", s.Name)
	}
	_, far := laneOf(e, s, t)
	o := Order{Kind: GoToStar, Star: far}
	if err := s.Check(e.Cluster, o); err != nil {
		return err
	}
	return e.blockedOrder(s, o)
}

// Lane Magnetron: "slip through the star lane almost instantly ... can be
// used only once": the user takes the GoToStar order, enters the lane now
// (out of orbit, unarmed) and comes out at the next close.
func useMagnetron(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	li, far := laneOf(e, s, t)
	s.Order = Order{Kind: GoToStar, Star: far}
	s.Pos = e.Cluster.Stars[s.Star].System.Lanes[t.Index].Pos
	s.Lane, s.Orbit, s.Arrive, s.Armed = li, -1, e.Day+1, nil
	return fmt.Sprintf("%s slips into the lane to %s", s.Name, e.StarLabel(far)), "into the lane", nil
}

// refuseEndoscope: the far star explored already.
func refuseEndoscope(e *Empire, s *Starship, part int, t Target) error {
	if _, far := laneOf(e, s, t); e.Knows(far) {
		return errors.New("already explored")
	}
	return nil
}

// Lane Endoscope: "scan the system at the other end of a star lane": the
// far star Visited and the lane learnt.
func useEndoscope(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	li, far := laneOf(e, s, t)
	e.Visit(far)
	e.LearnLane(li)
	return fmt.Sprintf("%s scans %s through the lane", s.Name, e.StarLabel(far)), "far end scanned", nil
}
