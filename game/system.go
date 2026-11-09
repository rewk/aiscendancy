package game

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// Vec3 is a position in a star system: X,Z in the ecliptic plane, Y up.
// The world unit is the radius of the system display's grid disc.
type Vec3 struct{ X, Y, Z float64 }

// StarType is starstat.txt's TYPE_n, which is also suns.shp's frame n.
type StarType uint8

// starTypeNames are static.txt's strings 178-190, in suns.shp's frame order:
// a colour and a size, "Blue Medium" being the system screen's subtitle.
var starTypeNames = [NumStarTypes]string{
	"Blue Giant", "Pink-Red Giant", "Red Giant", "Orange Giant",
	"Blue Medium", "White Medium", "Yellow Medium", "Light Yellow Medium",
	"Orange Medium", "Pink-Red Medium", "Blue Dwarf", "White Dwarf", "Red Dwarf",
}

func (t StarType) String() string {
	if t < NumStarTypes {
		return starTypeNames[t]
	}
	return fmt.Sprintf("star type %d", t)
}

const NumStarTypes = 13

// Lane is a star lane's endpoint in a system.
type Lane struct {
	Pos Vec3 // the endpoint, radius 1.0-1.15: just outside the grid disc
	Red bool // red lanes are slow
	To  int  // index of the star it leads to in its Cluster, -1 when none
}

// StarSystem is one star, its planets and its lane endpoints.
type StarSystem struct {
	Name    string
	Star    StarType
	Planets []*Planet
	Lanes   []Lane
}

// Colonist is the empire with the most of the system's colonies (ties:
// the lowest index), −1 for none: the starmap's ring (galactic.txt).
func (s *StarSystem) Colonist() int {
	n := map[int]int{}
	best := -1
	for _, p := range s.Planets {
		if p.Owner < 0 {
			continue
		}
		n[p.Owner]++
		if best < 0 || n[p.Owner] > n[best] || n[p.Owner] == n[best] && p.Owner < best {
			best = p.Owner
		}
	}
	return best
}

// Controller is the owner of every colony in the system (help.txt: "A
// species controls a system by having the only colonies there"), −1 with
// none or two. The fan manual's lane rule is not modelled.
func (s *StarSystem) Controller() int {
	c := -1
	for _, p := range s.Planets {
		switch {
		case p.Owner < 0:
		case c < 0:
			c = p.Owner
		case p.Owner != c:
			return -1
		}
	}
	return c
}

// System counts NewStarSystem accepts.
const (
	MaxSystemPlanets = 8
	MinSystemLanes   = 1
	MaxSystemLanes   = 6
)

// StarHeight is the star's height above the disc centre, in radii: the
// original's elevation line under the star is ~50 px for a 220 px radius.
const StarHeight = 0.25

// NewStarSystem rolls a lone system: lane k of m at angle φ + 2πk/m ±
// π/(3m) from +X toward +Z (random phase φ), red one time in four, To −1;
// then rollSystem.
func NewStarSystem(seed uint64, name string, star StarType, planets, lanes int) *StarSystem {
	if lanes < MinSystemLanes || lanes > MaxSystemLanes {
		panic(fmt.Sprintf("system lanes %d", lanes))
	}
	r := rand.New(rand.NewPCG(seed, 0x6a09e667f3bcc909))
	step := 2 * math.Pi / float64(lanes)
	phase := 2 * math.Pi * r.Float64()
	specs := make([]laneSpec, lanes)
	for k := range specs {
		jitter := step / 3 * (r.Float64() - 0.5) // ±π/(3m)
		specs[k] = laneSpec{Angle: phase + float64(k)*step + jitter, Red: r.IntN(4) == 0, To: -1}
	}
	return rollSystem(r, name, star, planets, specs)
}

// laneSpec is what the caller decides of a lane endpoint: its direction
// from the star (radians from +X toward +Z), its kind, its destination.
type laneSpec struct {
	Angle float64
	Red   bool
	To    int
}

// rollSystem rolls the planets and places the lane endpoints from r.
// Planet k of n: uniform type and size, its own surface sub-seed, named
// "<name> <k+1 in roman>", at radius 0.25+0.65*(k+u)/n, a uniform angle,
// height [0,0.6), then spreadOrbits (no draw: a height may move, up to
// 0.8). Lane endpoint k: lanes[k].Angle, radius [1,1.15), height [0,1.2).
// Everything floats above the plane, as in the original.
func rollSystem(r *rand.Rand, name string, star StarType, planets int, lanes []laneSpec) *StarSystem {
	if planets < 0 || planets > MaxSystemPlanets {
		panic(fmt.Sprintf("system planets %d", planets))
	}
	if star >= NumStarTypes {
		panic(fmt.Sprintf("star type %d", star))
	}
	s := &StarSystem{Name: name, Star: star}
	uniform := func(lo, hi float64) float64 { return lo + (hi-lo)*r.Float64() }
	onCircle := func(radius, angle, y float64) Vec3 {
		return Vec3{radius * math.Cos(angle), y, radius * math.Sin(angle)}
	}
	for k := range planets {
		typ := rollPlanetType(r, star)
		size := MinSize + r.IntN(MaxSize-MinSize+1)
		p := NewPlanet(r.Uint64(), name+" "+roman(k+1), typ, size)
		radius := 0.25 + 0.65*(float64(k)+r.Float64())/float64(planets)
		p.Pos = onCircle(radius, uniform(0, 2*math.Pi), uniform(0, 0.6))
		s.Planets = append(s.Planets, p)
	}
	spreadOrbits(s.Planets)
	for _, l := range lanes {
		pos := onCircle(uniform(1, 1.15), l.Angle, uniform(0, 1.2))
		s.Lanes = append(s.Lanes, Lane{Pos: pos, Red: l.Red, To: l.To})
	}
	return s
}

// roman writes 1..39 in roman numerals, enough for planet names.
func roman(n int) string {
	if n < 1 || n > 39 {
		panic(fmt.Sprintf("roman %d", n))
	}
	ones := [...]string{"", "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX"}
	out := ""
	for ; n >= 10; n -= 10 {
		out += "X"
	}
	return out + ones[n]
}

// rollPlanetType is uniform over the 11 types, whatever the star (a
// census of 1250 saved planets; docs/formats.md).
func rollPlanetType(r *rand.Rand, _ StarType) PlanetType {
	return PlanetType(r.IntN(int(NumPlanetTypes)))
}
