package game

import (
	"errors"
	"fmt"
	"slices"
)

// The planetary Tractor Beam (docs/rules.md "Planets in combat"): the
// surface structure TractorBeam "pull[s] it toward a planet". Any ship
// (own, allied, hostile) in space at the colony's star, as ArmPlanet's
// grounds (a planet does not act on a ship in orbit).
// ASSUMPTION, no number for the planet in the files: the ship gizmo's
// (GizmoTractorBeam), Range measured from the planet's Pos, Special1 the
// pull (specialReach), toward the planet and stopped PlanetClear off it by
// displace (keepOff: the planets, and ShipGap off the ships). One pull a
// round per structure on the surface (Colony.Tractors counts them), no
// energy. "You can't shoot and use this device at the same time": a pull
// drops the colony's armed shots and ArmPlanet refuses until Round (or the
// close) resets Tractors.

// TractorRange is the planetary beam's reach from the planet's Pos.
func TractorRange() float64 { return float64(Gizmos[GizmoTractorBeam].Range) * RangeUnit }

// tractorCheck is the ship t names for col's beam and the point the pull
// ends at, or why not: not e's colony, no beam, none left this round, no
// such ship (t a ShipTarget: Mine one of e's, else a Sighting of a live
// ship of its empire), in a lane, at another star, in orbit, out of
// range, already alongside (within PlanetClear, or nowhere clear to come).
func (e *Empire) tractorCheck(col *Colony, t Target) (*Starship, Vec3, error) {
	if !slices.Contains(e.Colonies, col) {
		return nil, Vec3{}, errors.New("not a colony of this empire")
	}
	p := col.Planet
	if n := p.Count(TractorBeam); n == 0 {
		return nil, Vec3{}, fmt.Errorf("%s has no Tractor Beam", p.Name)
	} else if col.Tractors >= n {
		return nil, Vec3{}, errors.New("Tractor Beam: no pull left this round")
	}
	es := e.peerList()
	tg := t.ship()
	switch {
	case t.Kind != ShipTarget || (t.Mine == nil) == (t.Ship.ship == nil) || tg.Hull <= 0:
		return nil, Vec3{}, errNoTarget
	case t.Mine != nil && !slices.Contains(e.Ships, tg):
		return nil, Vec3{}, errNoTarget
	case t.Mine == nil && (t.Ship.Empire < 0 || t.Ship.Empire >= len(es) || !slices.Contains(es[t.Ship.Empire].Ships, tg)):
		return nil, Vec3{}, errNoTarget
	case tg.Lane >= 0:
		return nil, Vec3{}, errors.New("the target is in a star lane")
	case tg.Star != col.Star:
		return nil, Vec3{}, errors.New("not in the same system")
	case tg.Orbit >= 0:
		return nil, Vec3{}, fmt.Errorf("the target is in orbit of %s: a planet does not act on a ship in orbit", e.Planet(tg.Star, tg.Orbit).Name) // unnamed: a foreign ship never is
	}
	d := dist(p.Pos, tg.Pos)
	if d > TractorRange()+1e-9 {
		return nil, Vec3{}, errors.New("Tractor Beam: out of range")
	}
	if d <= PlanetClear+1e-9 { // the pull below would push it away
		return nil, Vec3{}, errors.New("already alongside")
	}
	k := min(specialReach(Gizmos[GizmoTractorBeam].Special1), d-PlanetClear)
	to := inReach(along(tg.Pos, unit(tg.Pos, p.Pos), k))
	end := clearPath(tg.Pos, to, keepOff(e, tg.Star, tg))
	if dist(end, tg.Pos) < 1e-9 {
		return nil, Vec3{}, errors.New("already alongside")
	}
	return tg, to, nil
}

// CanTractor reports why col's Tractor Beam may not pull t now
// (tractorCheck), nil when it may.
func (e *Empire) CanTractor(col *Colony, t Target) error {
	_, _, err := e.tractorCheck(col, t)
	return err
}

// TractorEnd is where col's beam would leave t's ship (displace's point),
// or tractorCheck's refusal: the bots' foresight.
func (e *Empire) TractorEnd(col *Colony, t Target) (Vec3, error) {
	tg, to, err := e.tractorCheck(col, t)
	if err != nil {
		return Vec3{}, err
	}
	return clearPath(tg.Pos, to, keepOff(e, tg.Star, tg)), nil
}

// UseTractor pulls t's ship toward col's planet at once (tractorCheck
// refusing, nothing changed): one pull of the round counted, col's armed
// shots dropped (no "disarm" entry: the "tractor" one says it). The
// Reports: e's SpecialUsed, the same to the ship's owner when another.
func (e *Empire) UseTractor(col *Colony, t Target) ([]Report, error) {
	tg, to, err := e.tractorCheck(col, t)
	if err != nil {
		return nil, err
	}
	owner := e.Index
	if t.Mine == nil {
		owner = t.Ship.Empire
	}
	if e.journal != nil {
		en := col.entry("tractor")
		en.Star, en.Planet, en.Target, en.Other = col.Star, col.PlanetIdx, tg.Name, owner
		e.journal.Record(en)
	}
	col.Tractors++
	col.Armed = nil
	m := displace(e, tg, to)
	ev := Event{Kind: SpecialUsed, Item: NoItem, Tech: NoTech, Star: col.Star, Planet: -1, Other: -1,
		Text: fmt.Sprintf("%s's Tractor Beam: %s", col.Planet.Name, movedText(tg.Name, m))}
	reps := []Report{{e.Index, ev}}
	if owner != e.Index {
		reps = append(reps, Report{owner, ev})
	}
	return reps, nil
}
