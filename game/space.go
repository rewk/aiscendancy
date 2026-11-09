package game

import "math"

// The exclusion radius (ShipGap): where a ship in space may stop.

// InSpace reports a ship out of the lanes and out of orbit: the ships
// ShipGap keeps apart.
func (s *Starship) InSpace() bool { return s.Lane < 0 && s.Orbit < 0 }

// spaceAt is the Pos of every ship of es in space at star, not excepted,
// in empire then Ships order (nil when none).
func spaceAt(es []*Empire, star int, not *Starship) []Vec3 {
	var out []Vec3
	for _, e := range es {
		for _, o := range e.Ships {
			if o != not && o.Star == star && o.InSpace() {
				out = append(out, o.Pos)
			}
		}
	}
	return out
}

// Obstacles is what a view's Hop of s keeps ShipGap from: every other
// ship in space at s's star.
func (g *Game) Obstacles(s *Starship) []Vec3 { return spaceAt(g.Empires, s.Star, s) }

// lerp is a + (b − a)·f.
func lerp(a, b Vec3, f float64) Vec3 {
	return Vec3{a.X + (b.X-a.X)*f, a.Y + (b.Y-a.Y)*f, a.Z + (b.Z-a.Z)*f}
}

// spaceDirs is the 26 directions (dx, dy, dz in {−1, 0, 1}, not all 0)
// normalised, x then y then z loop order: the free-spot search's.
var spaceDirs = func() (d [26]Vec3) {
	i := 0
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			for dz := -1; dz <= 1; dz++ {
				if dx == 0 && dy == 0 && dz == 0 {
					continue
				}
				n := math.Sqrt(float64(dx*dx + dy*dy + dz*dz))
				d[i] = Vec3{float64(dx) / n, float64(dy) / n, float64(dz) / n}
				i++
			}
		}
	}
	return d
}()

// crowds reports q within ShipGap (less 1e-9) of one of obstacles.
func crowds(q Vec3, obstacles []Vec3) bool {
	for _, o := range obstacles {
		if dist(q, o) < ShipGap-1e-9 {
			return true
		}
	}
	return false
}

// sphere is a ball a ship may not end in: a ship in space (ShipGap) or a
// planet (PlanetClear).
type sphere struct {
	c Vec3
	r float64
}

// gaps is a ShipGap sphere round each of obstacles.
func gaps(obstacles []Vec3) []sphere {
	out := make([]sphere, 0, len(obstacles))
	for _, o := range obstacles {
		out = append(out, sphere{o, ShipGap})
	}
	return out
}

// FreeSpot is where a ship placed at p in space goes, freeOf the ShipGap
// of obstacles (the Pos of the ships in space there).
func FreeSpot(p Vec3, obstacles []Vec3) Vec3 {
	if !crowds(p, obstacles) {
		return p
	}
	return freeOf(p, gaps(obstacles))
}

// freeOf is p when no sphere holds it, else the first of p + k·ShipGap·u
// (k = 1, 2, ..., u over spaceDirs) free and within SystemReach of the
// origin; p itself when none is out to 2·SystemReach.
func freeOf(p Vec3, obs []sphere) Vec3 {
	free := func(q Vec3) bool {
		for _, o := range obs {
			if dist(q, o.c) < o.r-1e-9 {
				return false
			}
		}
		return true
	}
	if free(p) {
		return p
	}
	for k := 1; float64(k)*ShipGap <= 2*SystemReach; k++ {
		r := float64(k) * ShipGap
		for _, u := range spaceDirs {
			q := Vec3{p.X + r*u.X, p.Y + r*u.Y, p.Z + r*u.Z}
			if dist(q, Vec3{}) <= SystemReach && free(q) {
				return q
			}
		}
	}
	return p
}

// clearEnd is where a hop from a to b in space stops for obstacles: b
// when it is ShipGap or more from each, else the farthest point of the
// segment that is (only the end counts, the path may pass by). On a +
// t(b − a), each obstacle forbids the open interval of t within r of it,
// r = ShipGap shrunk to |a − o| when a starts inside (it may leave, not
// end closer); t starts at 1 and drops to the lower end of any interval
// holding it until none does. Independent of the obstacles' order.
func clearEnd(a, b Vec3, obstacles []Vec3) Vec3 {
	d := Vec3{b.X - a.X, b.Y - a.Y, b.Z - a.Z}
	qa := d.X*d.X + d.Y*d.Y + d.Z*d.Z
	if qa == 0 || len(obstacles) == 0 {
		return b
	}
	type span struct{ lo, hi float64 }
	var bad []span
	for _, o := range obstacles {
		w := Vec3{a.X - o.X, a.Y - o.Y, a.Z - o.Z}
		w2 := w.X*w.X + w.Y*w.Y + w.Z*w.Z
		r2 := min(ShipGap*ShipGap, w2)
		hb := d.X*w.X + d.Y*w.Y + d.Z*w.Z // half the linear term
		disc := hb*hb - qa*(w2-r2)
		if disc <= 0 {
			continue
		}
		sq := math.Sqrt(disc)
		bad = append(bad, span{(-hb - sq) / qa, (-hb + sq) / qa})
	}
	const eps = 1e-12
	t := 1.0
	for moved := true; moved; {
		moved = false
		for _, sp := range bad {
			if nt := max(sp.lo, 0); t > sp.lo+eps && t < sp.hi-eps && nt < t {
				t, moved = nt, true
			}
		}
	}
	if t == 1 {
		return b
	}
	return Vec3{a.X + t*d.X, a.Y + t*d.Y, a.Z + t*d.Z}
}

// clearPath is clearEnd with a radius per sphere. Not merged: clearEnd's
// ShipGap² is the constant's, a ulp off the float product o.r*o.r.
func clearPath(a, b Vec3, obs []sphere) Vec3 {
	d := Vec3{b.X - a.X, b.Y - a.Y, b.Z - a.Z}
	qa := d.X*d.X + d.Y*d.Y + d.Z*d.Z
	if qa == 0 {
		return b
	}
	type span struct{ lo, hi float64 }
	var bad []span
	for _, o := range obs {
		w := Vec3{a.X - o.c.X, a.Y - o.c.Y, a.Z - o.c.Z}
		w2 := w.X*w.X + w.Y*w.Y + w.Z*w.Z
		r2 := min(o.r*o.r, w2)
		hb := d.X*w.X + d.Y*w.Y + d.Z*w.Z
		disc := hb*hb - qa*(w2-r2)
		if disc <= 0 {
			continue
		}
		sq := math.Sqrt(disc)
		bad = append(bad, span{(-hb - sq) / qa, (-hb + sq) / qa})
	}
	const eps = 1e-12
	t := 1.0
	for moved := true; moved; {
		moved = false
		for _, sp := range bad {
			if nt := max(sp.lo, 0); t > sp.lo+eps && t < sp.hi-eps && nt < t {
				t, moved = nt, true
			}
		}
	}
	if t == 1 {
		return b
	}
	return Vec3{a.X + t*d.X, a.Y + t*d.Y, a.Z + t*d.Z}
}
