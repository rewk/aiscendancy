// Package sav reads Ascendancy's save games (01.SAV … 09.SAV, RESUME.GAM)
// far enough to get the galaxy (stars, their planets and the star lanes)
// and the colonies (population, owner, project, the output as shown).
// Read only; nothing here writes a save.
//
// A save is the game's arrays dumped one after the other, each as a u32
// count then count fixed-size records. The records still hold the in-memory
// pointers of the DOS build (0x2xxxxx, one heap base per array); they are
// only used to cross-check, never followed. Little endian throughout.
//
//	0        180-byte header. u16 at 0 = the game day. u32 at 0x74 = size
//	         of the next blob (7538);
//	         u32s at 0x78.. = 96, 123, 494, 39, 414, 13: record sizes
//	         (star, planet, player, lane, then two unknown). Checked as a
//	         version stamp. 0x70 = the save slot (0-7, 3073704 in a
//	         .GAM); the rest of the header is not decoded.
//	180      the technology blob, 7538 bytes (names "Orbital Structures"…)
//	7718     u32 n stars, n × 96-byte star records
//	         u32 n squares, n × 4 bytes: every planet's surface and orbital
//	         squares (squares+10 per planet, planet after planet), pointed
//	         at by planet +16
//	         u32 n planets, n × 123-byte planet records
//	         u32 n ships, n × 358-byte records (not decoded)
//	         u32 n players, n × 494-byte records (not decoded)
//	         u32 n lanes, n × 39-byte lane records
//	         the rest is not decoded
//
// Star record (96 bytes):
//
//	+0   u32 star type 0-12 (starstat.txt TYPE_n = suns.shp frame)
//	+4   u32 index, 0-based = the record's position
//	+8   3 × f32 position (x, y, z), within about ±450
//	+20  8 bytes, unknown (per-player flags?)
//	+28  name, NUL ended, 16 bytes
//	+44  6 × u32 pointers to the star's lane records (first +68 valid)
//	+68  u16 lane count
//	+70  5 × u32 pointers to its planet records (first +90 valid)
//	+90  u16 planet count
//	+92  i32 -1 (owner? 0 seen once)
//
// Planet record (123 bytes), a star's planets consecutive, in star order:
//
//	+12  u16 star index (0-based); +14 u16 planet number in its system
//	+16  u32 pointer to its squares; +20 u16 size-1 (0-4); +22 u16 type
//	(0-10, planitem.txt order); +24 u16 squares (7/11/23/45/73); +26
//	squares+10; +28/+30 free surface/orbital squares; +36 name, NUL ended;
//	+66 u8 the population; +68, +70, +72 u16 the day's industry, research
//	and prosperity as the colony screen shows them (prosperity net of the
//	upkeep, 0 at worst; an AI's industry with its cheat); +74 u8 the
//	population capacity (docs/rules.md: 5 + non-black/3 on a bare
//	planet); +76 u8 probably the busy population; +84 u8 the project's
//	planitem index, ff none; +87 u8 the owner, 0 the player (meaningful
//	with a population).
//
// Ship record (358 bytes, packed: fields on 2-byte boundaries), read by
// savedump -diff, little of it confirmed:
//
//	+0   u32 ship id; the records are sorted by it, a freed id is reused
//	+56  name, NUL ended, 30 bytes (the player's, or "Medium", "Large"…)
//	+86  u16 ≤ the day: likely the day built
//	+108 u32 pointer, 0 for a ship never placed
//	+112 3 × f32, zero on most idle ships
//	+124 3 × f32 a position (multiples of 32 in orbit, shared by the ships
//	     at one place; not a star's position scaled)
//	+136 u16 9 in orbit, 1 in space
//	+140 u32 the energy left today: −MoveCost a hop, refilled in full to
//	     Σ generator Power each day (measured, test saves 12-18), less
//	     the shields' Power while they are on (their reservation)
//	+144 u16, +156 u16: 10/20/30/40 on Small/Medium/Large/Enormous hulls
//	+148 u32 the active shields' strength (Σ Level, 0 off or knocked
//	     down: a hit the energy could not pay the draw for, test saves 12-16)
//	+152 u32 often Σ drive Level (2 on a Tonklin, 10 on a Space Bender), but
//	     12 → 0 on an enemy entering a hostile orbit: unknown
//	+162 3 × f32 (2-aligned) a copy of the position, or on some ships a
//	     value growing by a constant a day (not the energy)
//	+175 7-byte part entries: byte 0 a gizmo index (ff empty), byte 1
//	     its Uses left today (a shot −1); bytes 3-6 ffffffff for a
//	     shield switched on
//
// Square (4 bytes): byte 0 the colour code (0 black, 1 white, 2/3/4 the
// colours; 8-12 Xeno ruins on a square of code−8: at most one per
// planet, item 38 the Xeno Archeological Dig on it while dug, gone once
// dug), byte 1 the planitem index (ff none), byte 2 1 built / 0 under
// construction, byte 3 unknown.
//
// Lane record (39 bytes):
//
//	+0   u32 star A, +4 u32 star B (0-based indices)
//	+8   3 × f32 a vector along B−A, |v| 2000-2600 (not the distance);
//	+20  its negation
//	+32  u8 flags, bit 0 often set, values 0-30: per-player "seen"? unknown
//	+33  u8 0/1, unknown (explored?)
//	+35  u8 1 = RED lane. The red lanes are always the last ones of the
//	     array (laid after the blue web) and 2-3× longer on average.
//
// The lane pointers of the star records are checked against the lane
// array: (pointer − lowest pointer)/39 must be a lane whose ends are
// exactly the stars that point at it.
package sav

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"
)

const (
	headerSize = 180
	starSize   = 96
	planetSize = 123
	shipSize   = 358
	playerSize = 494
	laneSize   = 39
	orbitSlots = 10
)

// Star is one star record.
type Star struct {
	Name        string
	Type        int // 0-12, suns.shp order
	Pos         [3]float32
	PlanetCount int
}

// Planet is one planet record.
type Planet struct {
	Name     string
	Star     int // 0-based star index
	Size     int // 1-5
	Type     int // 0-10
	Capacity int // +74, the population capacity
	// Pop is +66, the population (0: no colony); Owner +87, the colony's
	// player, 0 the human one.
	Pop, Owner int
	// Industry, Research, Prosperity are +68, +70, +72: the day's output
	// as shown (Prosperity net of the upkeep, at least 0).
	Industry, Research, Prosperity int
	// Project is +84, the planitem index of the project, NoProject none.
	Project int
	// Squares are its squares (planet +26 of them): the surface ones, then
	// the 10 orbit slots.
	Squares []Square
}

// NoProject is Planet.Project without one.
const NoProject = 0xff

// Square is one 4-byte square record.
type Square struct {
	Code  int  // byte 0: 0 black, 1 white, 2-4 the colours, 8-12 Xeno ruins on code−8
	Item  int  // byte 1: planitem index, 0xff none
	Built bool // byte 2 bit 0: 1 or 3 built, 0 under construction
	// Automated is byte 2 bit 1 (3: built and automated), confirmed in the
	// game on Ribulon I's 17 automated Factories (test save 01).
	Automated bool
}

// Black reports a black square (codes 0 and 8): it adds no capacity.
func (q Square) Black() bool { return q.Code == 0 || q.Code == 8 }

// Ruins reports Xeno ruins on the square (codes 8-12).
func (q Square) Ruins() bool { return q.Code >= 8 && q.Code <= 12 }

// Surface is the planet's surface squares, its orbit slots left out.
func (p Planet) Surface() []Square { return p.Squares[:max(0, len(p.Squares)-orbitSlots)] }

// Lane is one star lane; A and B are 0-based star indices.
type Lane struct {
	A, B int
	Red  bool
}

// Game is the decoded galaxy of one save.
type Game struct {
	Day     int // header u16 at 0
	Stars   []Star
	Lanes   []Lane
	Planets []Planet
}

// Open reads and decodes one save file.
func Open(path string) (*Game, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	g, err := Decode(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return g, nil
}

type reader struct {
	b   []byte
	err error
}

func (r *reader) u16(o int) int {
	if r.err == nil && (o < 0 || o+2 > len(r.b)) {
		r.err = fmt.Errorf("read past the end at %d", o)
	}
	if r.err != nil {
		return 0
	}
	return int(binary.LittleEndian.Uint16(r.b[o:]))
}

func (r *reader) u32(o int) int {
	if r.err == nil && (o < 0 || o+4 > len(r.b)) {
		r.err = fmt.Errorf("read past the end at %d", o)
	}
	if r.err != nil {
		return 0
	}
	return int(binary.LittleEndian.Uint32(r.b[o:]))
}

func (r *reader) f32(o int) float32 { return math.Float32frombits(uint32(r.u32(o))) }

func (r *reader) str(o, max int) string {
	if r.err != nil || o+max > len(r.b) {
		return ""
	}
	s := r.b[o : o+max]
	for i, c := range s {
		if c == 0 {
			return string(s[:i])
		}
	}
	return string(s)
}

// array reads a u32 count at o, checks count × size fits, and returns the
// first record's offset and the count.
func (r *reader) array(o, size, max int, what string) (int, int) {
	n := r.u32(o)
	if r.err == nil && (n > max || o+4+n*size > len(r.b)) {
		r.err = fmt.Errorf("%s: count %d at %d does not fit", what, n, o)
	}
	if r.err != nil {
		return 0, 0
	}
	return o + 4, n
}

// Section is one part of a save as Sections lays the file out. An array
// (stars … lanes) starts at Off, past its u32 count, with Count records of
// Size bytes; a blob (header, tech, rest) is one record of its whole length.
type Section struct {
	Name  string // header, tech, stars, squares, planets, ships, players, lanes, rest
	Off   int
	Size  int
	Count int
}

// End is the offset just past the section's last record.
func (s Section) End() int { return s.Off + s.Size*s.Count }

// Record is record i of the section in b.
func (s Section) Record(b []byte, i int) []byte {
	return b[s.Off+i*s.Size : s.Off+(i+1)*s.Size]
}

// Sections walks a save: the header, the tech blob, the six arrays, then
// the undecoded rest, in file order. The arrays' u32 counts lie between
// the sections, in no record. Only the layout is checked (the header's
// record sizes, every array fitting the file), not the records.
func Sections(b []byte) ([]Section, error) {
	r := &reader{b: b}
	secs := r.sections()
	return secs, r.err
}

// arrays are the save's arrays in file order, with a sanity cap on counts.
var arrays = []struct {
	name      string
	size, max int
}{
	{"stars", starSize, 1000}, {"squares", 4, 1 << 20}, {"planets", planetSize, 5000},
	{"ships", shipSize, 10000}, {"players", playerSize, 16}, {"lanes", laneSize, 10000},
}

func (r *reader) sections() []Section {
	for i, want := range []int{starSize, planetSize, playerSize, laneSize} {
		if got := r.u32(0x78 + 4*i); got != want && r.err == nil {
			r.err = fmt.Errorf("header record size %d at 0x%x, want %d: not a known save", got, 0x78+4*i, want)
		}
	}
	tech := r.u32(0x74)
	if r.err == nil && headerSize+tech > len(r.b) {
		r.err = fmt.Errorf("tech blob of %d past the end", tech)
	}
	if r.err != nil {
		return nil
	}
	secs := []Section{{"header", 0, headerSize, 1}, {"tech", headerSize, tech, 1}}
	o := headerSize + tech
	for _, a := range arrays {
		at, n := r.array(o, a.size, a.max, a.name)
		if r.err != nil {
			return nil
		}
		secs = append(secs, Section{a.name, at, a.size, n})
		o = at + n*a.size
	}
	return append(secs, Section{"rest", o, len(r.b) - o, 1})
}

// ShipName is the offset of a ship record's name, NUL ended, at most
// shipNameLen bytes: the name the player gave or the hull size's
// ("Medium", "Large", …); after the NUL the field may hold stale bytes.
const (
	ShipName    = 56
	shipNameLen = 30
)

// Label is a record's name when its section holds one (stars, planets,
// ships) and it reads as printable text, else "".
func Label(section string, rec []byte) string {
	at, n := 0, 0
	switch section {
	case "stars":
		at, n = 28, 16
	case "planets":
		at, n = 36, planetSize-36
	case "ships":
		at, n = ShipName, shipNameLen
	default:
		return ""
	}
	if at+n > len(rec) {
		return ""
	}
	s := (&reader{b: rec}).str(at, n)
	for _, c := range []byte(s) {
		if c < 0x20 || c > 0x7e {
			return ""
		}
	}
	return s
}

// Decode decodes a save held in memory.
func Decode(b []byte) (*Game, error) {
	r := &reader{b: b}
	secs := r.sections()
	if r.err != nil {
		return nil, r.err
	}
	// secs: header, tech, then the arrays in their order
	so, ns := secs[2].Off, secs[2].Count
	qo, nq := secs[3].Off, secs[3].Count
	po, np := secs[4].Off, secs[4].Count
	lo, nl := secs[7].Off, secs[7].Count

	g := &Game{Day: r.u16(0)}
	for i := 0; i < ns; i++ {
		o := so + i*starSize
		if ix := r.u32(o + 4); ix != i {
			return nil, fmt.Errorf("star record %d holds index %d", i, ix)
		}
		g.Stars = append(g.Stars, Star{
			Name:        r.str(o+28, 16),
			Type:        r.u32(o),
			Pos:         [3]float32{r.f32(o + 8), r.f32(o + 12), r.f32(o + 16)},
			PlanetCount: r.u16(o + 90),
		})
	}
	q := 0 // the squares are laid planet after planet, +26 each
	for i := 0; i < np; i++ {
		o := po + i*planetSize
		p := Planet{Name: r.str(o+36, planetSize-36), Star: r.u16(o + 12), Size: r.u16(o+20) + 1, Type: r.u16(o + 22), Capacity: int(b[o+74]),
			Pop: int(b[o+66]), Owner: int(b[o+87]), Industry: r.u16(o + 68), Research: r.u16(o + 70), Prosperity: r.u16(o + 72),
			Project: int(b[o+84])}
		if p.Star >= ns {
			return nil, fmt.Errorf("planet %d %q: star %d of %d", i, p.Name, p.Star, ns)
		}
		n := r.u16(o + 26)
		if q+n > nq {
			return nil, fmt.Errorf("planet %d %q: squares %d past the %d", i, p.Name, q+n, nq)
		}
		for k := q; k < q+n; k++ {
			x := qo + 4*k
			p.Squares = append(p.Squares, Square{Code: int(b[x]), Item: int(b[x+1]), Built: b[x+2]&1 != 0, Automated: b[x+2]&2 != 0})
		}
		q += n
		g.Planets = append(g.Planets, p)
	}
	if q != nq {
		return nil, fmt.Errorf("the planets hold %d squares of %d", q, nq)
	}
	for i := 0; i < nl; i++ {
		o := lo + i*laneSize
		l := Lane{A: r.u32(o), B: r.u32(o + 4), Red: b[o+35] != 0}
		if l.A >= ns || l.B >= ns || l.A == l.B {
			return nil, fmt.Errorf("lane %d: stars %d-%d of %d", i, l.A, l.B, ns)
		}
		g.Lanes = append(g.Lanes, l)
	}
	if r.err != nil {
		return nil, r.err
	}
	if err := g.checkLanePointers(r, so); err != nil {
		return nil, err
	}
	return g, nil
}

// checkLanePointers matches the stars' lane pointers (+44, count +68)
// with the lane array: lane k = (pointer − lowest)/39 must join exactly
// the stars pointing at it.
func (g *Game) checkLanePointers(r *reader, so int) error {
	base := -1
	for i := range g.Stars {
		o := so + i*starSize
		for j := 0; j < r.u16(o+68) && j < 6; j++ {
			if p := r.u32(o + 44 + 4*j); base < 0 || p < base {
				base = p
			}
		}
	}
	ends := make([][]int, len(g.Lanes))
	for i := range g.Stars {
		o := so + i*starSize
		n := r.u16(o + 68)
		if n > 6 {
			return fmt.Errorf("star %d: %d lanes", i, n)
		}
		for j := 0; j < n; j++ {
			d := r.u32(o+44+4*j) - base
			k := d / laneSize
			if d%laneSize != 0 || k >= len(ends) {
				return fmt.Errorf("star %d: lane pointer off the array", i)
			}
			ends[k] = append(ends[k], i)
		}
	}
	for k, l := range g.Lanes {
		e := ends[k]
		if len(e) != 2 || !(e[0] == l.A && e[1] == l.B || e[0] == l.B && e[1] == l.A) {
			return fmt.Errorf("lane %d (%d-%d): stars pointing at it %v", k, l.A, l.B, e)
		}
	}
	return nil
}

// Dist is the distance between two stars in the file's units.
func (g *Game) Dist(a, b int) float64 {
	p, q := g.Stars[a].Pos, g.Stars[b].Pos
	dx, dy, dz := float64(p[0]-q[0]), float64(p[1]-q[1]), float64(p[2]-q[2])
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// Stats is the galaxy measured in the generator's vocabulary
// (docs/galaxy.md, game/cluster.go).
type Stats struct {
	Stars, Lanes, Red int
	LanesPerStar      float64     // mean degree, 2·lanes/stars
	Degree            map[int]int // degree → star count
	RedShare          float64     // red lanes / lanes
	BlueLen, RedLen   float64     // mean lengths, file units
	Spacing           float64     // cbrt(bounding box volume / stars)
	BlueSp, RedSp     float64     // mean lengths in spacings
	DeadEnds          float64     // share of degree-1 stars
	LongestChain      int         // lanes from a dead end to the first star of degree ≥ 3
	Connected         bool
	Extent            [3]float64 // bounding box size x, y, z
	MeanRadius        float64    // mean distance from the centroid
	YFlat             float64    // y extent / mean of the x and z extents
	Islands           int        // connected components of the blue lanes alone (an isolated star counts)
	NNBlue            float64    // share of stars whose nearest neighbour is a blue partner
	RedBridge         float64    // mean over the reds joining two blue islands of length / the islands' closest pair; 0 without such reds
	Diameter          int        // the most lanes on a shortest path between two stars (hops); −1 when not connected
}

// Stats measures the galaxy.
func (g *Game) Stats() Stats {
	n := len(g.Stars)
	s := Stats{Stars: n, Lanes: len(g.Lanes), Degree: map[int]int{}}
	if n == 0 {
		return s
	}
	adj := make([][]int, n)
	var blue, red float64
	for _, l := range g.Lanes {
		adj[l.A] = append(adj[l.A], l.B)
		adj[l.B] = append(adj[l.B], l.A)
		if l.Red {
			s.Red++
			red += g.Dist(l.A, l.B)
		} else {
			blue += g.Dist(l.A, l.B)
		}
	}
	if nb := s.Lanes - s.Red; nb > 0 {
		s.BlueLen = blue / float64(nb)
	}
	if s.Red > 0 {
		s.RedLen = red / float64(s.Red)
	}
	if s.Lanes > 0 {
		s.RedShare = float64(s.Red) / float64(s.Lanes)
	}
	s.LanesPerStar = 2 * float64(s.Lanes) / float64(n)
	dead := 0
	for i := range adj {
		s.Degree[len(adj[i])]++
		if len(adj[i]) == 1 {
			dead++
			// walk the chain while the stars have degree 2
			prev, cur, k := i, adj[i][0], 1
			for len(adj[cur]) == 2 && k < n {
				nx := adj[cur][0]
				if nx == prev {
					nx = adj[cur][1]
				}
				prev, cur, k = cur, nx, k+1
			}
			if k > s.LongestChain {
				s.LongestChain = k
			}
		}
	}
	s.DeadEnds = float64(dead) / float64(n)

	seen := make([]bool, n)
	stack, count := []int{0}, 0
	seen[0] = true
	for len(stack) > 0 {
		a := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		for _, b := range adj[a] {
			if !seen[b] {
				seen[b] = true
				stack = append(stack, b)
			}
		}
	}
	s.Connected = count == n
	s.Diameter = -1
	if s.Connected {
		s.Diameter = diameter(adj)
	}

	var lo, hi, c [3]float64
	for i, st := range g.Stars {
		for k := 0; k < 3; k++ {
			v := float64(st.Pos[k])
			if i == 0 || v < lo[k] {
				lo[k] = v
			}
			if i == 0 || v > hi[k] {
				hi[k] = v
			}
			c[k] += v / float64(n)
		}
	}
	vol := 1.0
	for k := 0; k < 3; k++ {
		s.Extent[k] = hi[k] - lo[k]
		vol *= s.Extent[k]
	}
	for _, st := range g.Stars {
		dx, dy, dz := float64(st.Pos[0])-c[0], float64(st.Pos[1])-c[1], float64(st.Pos[2])-c[2]
		s.MeanRadius += math.Sqrt(dx*dx+dy*dy+dz*dz) / float64(n)
	}
	if xz := (s.Extent[0] + s.Extent[2]) / 2; xz > 0 {
		s.YFlat = s.Extent[1] / xz
	}
	s.Spacing = math.Cbrt(vol / float64(n))
	if s.Spacing > 0 {
		s.BlueSp, s.RedSp = s.BlueLen/s.Spacing, s.RedLen/s.Spacing
	}
	g.blueStats(&s)
	return s
}

// diameter is the largest hop distance of a connected graph: a BFS from
// every star (n ≤ 100, n² steps).
func diameter(adj [][]int) int {
	hops := make([]int, len(adj))
	most := 0
	for s := range adj {
		for i := range hops {
			hops[i] = -1
		}
		hops[s] = 0
		for q := []int{s}; len(q) > 0; q = q[1:] {
			for _, o := range adj[q[0]] {
				if hops[o] < 0 {
					hops[o] = hops[q[0]] + 1
					most = max(most, hops[o])
					q = append(q, o)
				}
			}
		}
	}
	return most
}

// blueStats fills Islands, NNBlue and RedBridge. The islands are labelled
// by a union-find over the blue lanes; a star's nearest neighbour is the
// lowest index at the smallest distance.
func (g *Game) blueStats(s *Stats) {
	n := len(g.Stars)
	root := make([]int, n)
	for i := range root {
		root[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if root[i] != i {
			root[i] = find(root[i])
		}
		return root[i]
	}
	blue := map[[2]int]bool{}
	for _, l := range g.Lanes {
		if !l.Red {
			blue[[2]int{min(l.A, l.B), max(l.A, l.B)}] = true
			if a, b := find(l.A), find(l.B); a != b {
				root[a] = b
			}
		}
	}
	for i := range root {
		if find(i) == i {
			s.Islands++
		}
	}
	nnBlue := 0
	for a := range n {
		nn, best := -1, math.Inf(1)
		for b := range n {
			if d := g.Dist(a, b); b != a && d < best {
				nn, best = b, d
			}
		}
		if nn >= 0 && blue[[2]int{min(a, nn), max(a, nn)}] {
			nnBlue++
		}
	}
	s.NNBlue = float64(nnBlue) / float64(n)
	sum, k := 0.0, 0
	for _, l := range g.Lanes {
		ia, ib := find(l.A), find(l.B)
		if !l.Red || ia == ib {
			continue
		}
		short := math.Inf(1)
		for a := range n {
			if find(a) != ia {
				continue
			}
			for b := range n {
				if find(b) == ib {
					short = min(short, g.Dist(a, b))
				}
			}
		}
		sum += g.Dist(l.A, l.B) / short
		k++
	}
	if k > 0 {
		s.RedBridge = sum / float64(k)
	}
}

// Key identifies a galaxy across saves of the same game: the star names
// and positions in record order (stars never move). The names alone are
// not enough: a 100-star galaxy uses all of names.txt's 100 names.
func (g *Game) Key() string {
	var b strings.Builder
	for _, s := range g.Stars {
		fmt.Fprintf(&b, "%s %v\n", s.Name, s.Pos)
	}
	return b.String()
}
