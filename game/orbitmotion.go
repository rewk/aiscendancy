package game

import "math"

// Orbits (docs/rules.md "Orbits"; in the original the planets turn each
// turn, the inner ones faster; the lane endpoints stay). Once a day, at
// the close (Game.EndDay) before the ships' moves, every planet turns
// about its star's vertical axis by 2π/OrbitPeriod(r),
// r = hypot(X, Z), its r and Y kept: counterclockwise seen from above
// (+Y), from +X towards −Z. No state of its own: the step applies to the
// saved Pos. The ships in its orbit go with it; a ship in space it ran
// over on its way (swept) is pushed off its new Pos.

// OrbitDays0 is the period in days of a planet at OrbitR0, the
// innermost radius the generator places. Ours: tuned by playing.
const OrbitDays0 = 30

// OrbitR0 is the radius whose period is OrbitDays0 (rollSystem's
// innermost, 0.25).
const OrbitR0 = 0.25

// OrbitMargin is the gap two planets' bodies keep at least where their
// orbits come closest (rollSystem, spreadOrbits). Ours.
const OrbitMargin = 0.02

// PlanetRadius is a planet's body radius in world units by size (the
// system view draws it so; spreadOrbits keeps the bodies apart).
func PlanetRadius(size int) float64 { return 0.025 + 0.015*float64(size) }

// OrbitPeriod is the days a planet at horizontal radius r takes round
// its star: OrbitDays0·(r/OrbitR0)^1.5 (Kepler's third law's shape).
func OrbitPeriod(r float64) float64 { return OrbitDays0 * math.Pow(r/OrbitR0, 1.5) }

// orbitStep is p one day further on its orbit; the star's axis itself
// (r = 0) stays.
func orbitStep(p Vec3) Vec3 {
	r := math.Hypot(p.X, p.Z)
	if r < 1e-12 {
		return p
	}
	a := math.Atan2(p.Z, p.X) - 2*math.Pi/OrbitPeriod(r)
	return Vec3{r * math.Cos(a), p.Y, r * math.Sin(a)}
}

// swept reports a planet moving from a to b running over p: p within
// PlanetClear of b, or of the segment at a point strictly inside it (the
// planet passed it on the way). One the planet only moved away from
// (nearest a) is not: a push would pull it in.
func swept(p, a, b Vec3) bool {
	if dist(p, b) < PlanetClear-1e-9 {
		return true
	}
	d := Vec3{b.X - a.X, b.Y - a.Y, b.Z - a.Z}
	q := d.X*d.X + d.Y*d.Y + d.Z*d.Z
	if q == 0 {
		return false
	}
	t := ((p.X-a.X)*d.X + (p.Y-a.Y)*d.Y + (p.Z-a.Z)*d.Z) / q
	return t > 0 && t < 1 && dist(p, lerp(a, b, t)) < PlanetClear-1e-9
}

// turnPlanets is the close's orbits: stars in index order, their planets
// in index order, each one orbitStep. A ship at the star out of the lanes
// (Empires then Ships order) in its orbit takes its new Pos; one in space
// it ran over (swept) is pushed PlanetClear off the new Pos, away from
// it (along its motion when on it), then to the
// freeOf spot clear of the other ships in space (ShipGap, live) and of
// the planet; its order kept.
func (g *Game) turnPlanets() {
	at := make([][]*Starship, len(g.Cluster.Stars))
	for _, e := range g.Empires {
		for _, s := range e.Ships {
			if s.Lane < 0 && s.Star >= 0 && s.Star < len(at) {
				at[s.Star] = append(at[s.Star], s)
			}
		}
	}
	for st, star := range g.Cluster.Stars {
		for k, p := range star.System.Planets {
			old := p.Pos
			p.Pos = orbitStep(old)
			for _, s := range at[st] {
				switch {
				case s.Orbit == k:
					s.Pos = p.Pos
				case s.Orbit < 0 && swept(s.Pos, old, p.Pos):
					u := unit(p.Pos, s.Pos)
					if dist(p.Pos, s.Pos) < 1e-12 {
						u = unit(old, p.Pos)
					}
					obs := append(gaps(spaceAt(g.Empires, st, s)), sphere{p.Pos, PlanetClear})
					s.Pos = freeOf(along(p.Pos, u, PlanetClear), obs)
				}
			}
		}
	}
}

// spreadOrbits keeps every pair of planets' bodies OrbitMargin apart
// where their orbits come closest (aligned on one side: hypot(r1 − r2,
// y1 − y2) ≥ PlanetRadius(a) + PlanetRadius(b) + OrbitMargin), with no
// draw: each planet in index order, too close to an earlier one, moves
// its Y to the nearest clear height in [0, 0.6] (rollSystem's band; a tie
// goes up), else in [0, 0.8], else just above the highest earlier
// planet's reach. Planet 0 never moves; the others only when needed, so
// a second call after a home planet replaced planet 0 is safe.
func spreadOrbits(ps []*Planet) {
	for k, p := range ps {
		r := math.Hypot(p.Pos.X, p.Pos.Z)
		type span struct{ lo, hi float64 } // the open heights too close to one
		var bad []span
		for _, q := range ps[:k] {
			need := PlanetRadius(p.Size) + PlanetRadius(q.Size) + OrbitMargin
			dr := r - math.Hypot(q.Pos.X, q.Pos.Z)
			if h2 := need*need - dr*dr; h2 > 0 {
				h := math.Sqrt(h2)
				bad = append(bad, span{q.Pos.Y - h, q.Pos.Y + h})
			}
		}
		clear := func(y float64) bool {
			for _, b := range bad {
				if y > b.lo+1e-12 && y < b.hi-1e-12 {
					return false
				}
			}
			return true
		}
		if clear(p.Pos.Y) {
			continue
		}
		// The nearest clear height lies on an interval's end or the band's.
		pick := func(top float64) (float64, bool) {
			best, ok := 0.0, false
			try := func(y float64) {
				if y < 0 || y > top || !clear(y) {
					return
				}
				d, bd := math.Abs(y-p.Pos.Y), math.Abs(best-p.Pos.Y)
				if !ok || d < bd || d == bd && y > best {
					best, ok = y, true
				}
			}
			try(0)
			try(top)
			for _, b := range bad {
				try(b.lo)
				try(b.hi)
			}
			return best, ok
		}
		y, ok := pick(0.6)
		if !ok {
			y, ok = pick(0.8)
		}
		if !ok {
			for _, b := range bad {
				y = max(y, b.hi)
			}
		}
		p.Pos.Y = y
	}
}
