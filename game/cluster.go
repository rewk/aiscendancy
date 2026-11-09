package game

import (
	"cmp"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
)

// Density is the galaxy size picked at setup (static.txt's five names,
// strings 155-159).
type Density uint8

const (
	Vacuous Density = iota
	Sparse
	Average
	Dense
	VeryDense
	NumDensities
)

func (d Density) String() string {
	return [...]string{"Vacuous", "Sparse", "Average", "Dense", "Very Dense"}[d]
}

// Stars is the star count of a density, measured in fresh saves.
func (d Density) Stars() int {
	return [...]int{15, 25, 50, 75, 100}[d]
}

// ParseDensity reads a density name, case and spaces ignored ("very
// dense", "VeryDense"), or its index 0-4.
func ParseDensity(s string) (Density, error) {
	if n, err := strconv.Atoi(s); err == nil {
		if n >= 0 && n < int(NumDensities) {
			return Density(n), nil
		}
		return 0, fmt.Errorf("density %d out of 0..%d", n, NumDensities-1)
	}
	key := strings.ToLower(strings.ReplaceAll(s, " ", ""))
	for d := range NumDensities {
		if strings.ToLower(strings.ReplaceAll(d.String(), " ", "")) == key {
			return d, nil
		}
	}
	return 0, fmt.Errorf("unknown density %q", s)
}

// starWeights is starstat.txt's Percent column, the share of each type
// (its Lanes column is not used: lanesPerStar).
var starWeights = [NumStarTypes]float64{
	12.57, 3.14, 5.55, 5.55, 10.35, 12.38, 13.31, 8.50, 4.07, 5.73, 1.48, 9.06, 8.32,
}

// Star is one star of the cluster.
type Star struct {
	Name   string
	Type   StarType
	Pos    Vec3        // in the unit ball, Y up
	Lanes  []int       // into Cluster.Lanes, in System.Lanes' order
	System *StarSystem // its Lanes[i].To is the other star's index
}

// StarLane joins two stars.
type StarLane struct {
	A, B int  // star indices, A < B
	Red  bool // the slow kind: island bridges and diameter shortcuts
}

// Cluster is the galaxy: stars joined by lanes into one connected graph.
type Cluster struct {
	Density Density
	Stars   []Star
	Lanes   []StarLane
	Home    int // the player's home star
}

// Cluster roll constants (docs/galaxy.md). Positions are in the unit ball
// (radius 1 = 440 file units). Lane lengths are in units of 1/√n: half
// the stars sit on the outer sphere, a surface, so the spacing shrinks
// like n^-1/2. Calibrate with `savedump -stats -gen`, never by eye.
var (
	// shellRadii: 440 − 75k file units (k = 0..5) over 440, measured.
	shellRadii = [numShells]float64{440.0 / 440, 365.0 / 440, 290.0 / 440, 215.0 / 440, 140.0 / 440, 65.0 / 440}
)

const (
	numShells = 6
	// minSep keeps two stars from sharing a pixel (≈ 13 file units, under
	// the saves' closest pairs). Assumption.
	minSep     = 0.03
	placeTries = 200
	// lanesPerStar: mean lanes per star, measured (2.15 over 12 galaxies).
	lanesPerStar = 2.15
	// blueCap: no blue lane but a nearest-neighbour one is longer, in lane
	// units (0.48 at Average); it sets the blue islands, hence the bridge
	// reds. Calibrated on the saves.
	blueCap = 3.4
	// extraMaxDeg: an extra blue lane joins stars of at most this many
	// lanes (cycles without hubs; dead ends measure 22-26%).
	extraMaxDeg = 2
	// redPenalty multiplies a red's length per end holding redHubDeg lanes
	// or more, to keep hubs rare. Calibrated.
	redPenalty = 1.5
	redHubDeg  = 3
	// maxHopsFrac caps the hop diameter at this share of n (maxHops):
	// our divergence, the saves' are 27-60%.
	maxHopsFrac = 0.33
	// chokeFrac: no lane may hold more than this share of n alone (its loss
	// cutting them off); cover reds lift the others (a red
	// alone has held 46 of 100 stars).
	chokeFrac = 0.10
)

// NewCluster rolls a cluster from its seed, one PRNG in a fixed order:
// names (a shuffle), types (starWeights), positions (placeStars), lanes
// (layLanes, no draw), the home (pickHome), the systems (1..5 planets
// uniform, 2..4 for the home, whose first planet becomes a home planet),
// then the Xeno ruins on a PRNG of their own (layRuins).
func NewCluster(seed uint64, d Density) *Cluster {
	n := d.Stars()
	if n > len(starNames) {
		panic(fmt.Sprintf("cluster of %d stars, %d names", n, len(starNames)))
	}
	r := rand.New(rand.NewPCG(seed, 0xbb67ae8584caa73b))
	c := &Cluster{Density: d, Stars: make([]Star, n)}

	names := slices.Clone(starNames)
	r.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })
	for i := range c.Stars {
		c.Stars[i].Name = names[i]
	}
	for i := range c.Stars {
		c.Stars[i].Type = rollStarType(r)
	}
	for i, p := range placeStars(r, n) {
		c.Stars[i].Pos = p
	}
	c.layLanes()
	for li, l := range c.Lanes {
		c.Stars[l.A].Lanes = append(c.Stars[l.A].Lanes, li)
		c.Stars[l.B].Lanes = append(c.Stars[l.B].Lanes, li)
	}
	c.Home = c.pickHome(r)

	for i := range c.Stars {
		s := &c.Stars[i]
		// Uniform, as the saved galaxies show.
		planets := 1 + r.IntN(5)
		if i == c.Home {
			planets = 2 + r.IntN(3)
		}
		specs := make([]laneSpec, len(s.Lanes))
		for k, li := range s.Lanes {
			to := c.Other(li, i)
			dx, dz := c.Stars[to].Pos.X-s.Pos.X, c.Stars[to].Pos.Z-s.Pos.Z
			specs[k] = laneSpec{Angle: math.Atan2(dz, dx), Red: c.Lanes[li].Red, To: to}
		}
		s.System = rollSystem(r, s.Name, s.Type, planets, specs)
	}
	home := c.Stars[c.Home].System
	hp := NewHomePlanet(r.Uint64(), home.Planets[0].Name)
	hp.Pos = home.Planets[0].Pos
	home.Planets[0] = hp
	spreadOrbits(home.Planets) // the home's size
	c.layRuins(seed)
	return c
}

// RuinChance is the share of planets holding Xeno ruins (123 of 797 in
// five day-0 saves, one square each; docs/rules.md).
const RuinChance = 0.154

// RuinChanceTiny is a tiny planet's (MinSize, 7 squares) share instead:
// ours, raised over the saves' (size did not matter there, 12-18% per
// size; a reason to settle the tiny ones).
const RuinChanceTiny = 0.25

// ruinChance is p's chance of ruins: RuinChanceTiny or RuinChance.
func ruinChance(p *Planet) float64 {
	if p.Size == MinSize {
		return RuinChanceTiny
	}
	return RuinChance
}

// layRuins marks Xeno ruins on one uniform square of each planet with
// probability ruinChance, never a home (a planet with a structure). Its
// own PRNG, two draws per planet in star then system order, home or not.
func (c *Cluster) layRuins(seed uint64) {
	r := rand.New(rand.NewPCG(seed, 0x3c6ef372fe94f82b))
	for i := range c.Stars {
		for _, p := range c.Stars[i].System.Planets {
			roll, cells := r.Float64(), p.Cells()
			at := cells[r.IntN(len(cells))]
			if roll < ruinChance(p) && !p.HasStructure() {
				p.Surface[at].Ruins = true
			}
		}
	}
}

// Other is the far end of a lane from one of its stars.
func (c *Cluster) Other(lane, star int) int {
	if l := c.Lanes[lane]; l.A == star {
		return l.B
	} else {
		return l.A
	}
}

// Dist is the Euclidean distance between two stars, in cluster units.
func (c *Cluster) Dist(a, b int) float64 { return dist(c.Stars[a].Pos, c.Stars[b].Pos) }

func dist(a, b Vec3) float64 {
	return math.Sqrt((a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y) + (a.Z-b.Z)*(a.Z-b.Z))
}

// rollStarType draws a type by starWeights: a uniform in [0,100) walked
// down the cumulative sum (100.01).
func rollStarType(r *rand.Rand) StarType {
	u := 100 * r.Float64()
	for t, w := range starWeights {
		if u < w {
			return StarType(t)
		}
		u -= w
	}
	return NumStarTypes - 1
}

// placeStars draws n points: a shell k, the heads of a fair coin capped
// at 5 (each shell about half the next outer one, as in the saves), then
// a uniform direction at shellRadii[k], redrawn up to placeTries times
// while closer than minSep to an earlier star.
func placeStars(r *rand.Rand, n int) []Vec3 {
	out := make([]Vec3, 0, n)
	for range n {
		k := 0
		for k < numShells-1 && r.IntN(2) == 1 {
			k++
		}
		var p Vec3
		for try := 0; try < placeTries; try++ {
			x, y, z := r.NormFloat64(), r.NormFloat64(), r.NormFloat64()
			l := math.Sqrt(x*x + y*y + z*z)
			if l == 0 {
				continue
			}
			f := shellRadii[k] / l
			p = Vec3{x * f, y * f, z * f}
			if !slices.ContainsFunc(out, func(q Vec3) bool { return dist(p, q) < minSep }) {
				break
			}
		}
		out = append(out, p)
	}
	return out
}

// unionFind labels components; find compresses paths.
type unionFind []int

func newUnionFind(n int) unionFind {
	u := make(unionFind, n)
	for i := range u {
		u[i] = i
	}
	return u
}

func (u unionFind) find(i int) int {
	for u[i] != i {
		u[i] = u[u[i]]
		i = u[i]
	}
	return i
}

// union joins the components of a and b, false when already one.
func (u unionFind) union(a, b int) bool {
	ra, rb := u.find(a), u.find(b)
	if ra == rb {
		return false
	}
	u[ra] = rb
	return true
}

// maxHops is the hop diameter cap of an n-star cluster (our rule):
// no star more than ceil(maxHopsFrac·n) lanes from another.
func maxHops(n int) int { return int(math.Ceil(maxHopsFrac * float64(n))) }

// layLanes lays the lanes from the positions alone. Blue: every star's
// nearest neighbour (no island of one), a Kruskal forest of the pairs at
// most blueCap/√n long (I islands left), then extras of the same cap
// between stars of at most extraMaxDeg lanes, up to round(lanesPerStar·n/2)
// − (I − 1). Red: I − 1 bridges (Kruskal on score), then cycle reds while
// two stars lie more than maxHops(n) apart, then cover reds while a
// choke is left (chokes). Sorted by (A,B).
func (c *Cluster) layLanes() {
	n := len(c.Stars)
	type pair struct {
		a, b int
		d    float64
	}
	pairs := make([]pair, 0, n*(n-1)/2)
	for a := range n {
		for b := a + 1; b < n; b++ {
			pairs = append(pairs, pair{a, b, c.Dist(a, b)})
		}
	}
	slices.SortFunc(pairs, func(p, q pair) int { return cmp.Or(cmp.Compare(p.d, q.d), p.a-q.a, p.b-q.b) })
	maxLen := blueCap / math.Sqrt(float64(n))
	deg := make([]int, n)
	adj := make([][]int, n)
	laid := map[[2]int]bool{}
	add := func(p pair, red bool) {
		c.Lanes = append(c.Lanes, StarLane{A: p.a, B: p.b, Red: red})
		laid[[2]int{p.a, p.b}] = true
		deg[p.a]++
		deg[p.b]++
		adj[p.a] = append(adj[p.a], p.b)
		adj[p.b] = append(adj[p.b], p.a)
	}
	// score: a red's length × redPenalty per end already a hub.
	score := func(p pair) float64 {
		s := p.d
		for _, e := range [2]int{p.a, p.b} {
			if deg[e] >= redHubDeg {
				s *= redPenalty
			}
		}
		return s
	}

	// Blue: every star's nearest neighbour (lowest index on a tie), a
	// mutual pair laid once.
	uf := newUnionFind(n)
	islands := n
	for a := range n {
		nn, best := -1, math.Inf(1)
		for b := range n {
			if d := c.Dist(a, b); b != a && d < best {
				nn, best = b, d
			}
		}
		if nn < 0 {
			continue
		}
		p := pair{min(a, nn), max(a, nn), best}
		if !laid[[2]int{p.a, p.b}] {
			add(p, false)
		}
		if uf.union(p.a, p.b) {
			islands--
		}
	}
	// Then the capped Kruskal forest over the NN components, then the extras.
	for _, p := range pairs {
		if p.d > maxLen {
			break
		}
		if uf.union(p.a, p.b) {
			add(p, false)
			islands--
		}
	}
	want := int(math.Round(lanesPerStar*float64(n)/2)) - (islands - 1)
	for _, p := range pairs {
		if len(c.Lanes) >= want || p.d > maxLen {
			break
		}
		if !laid[[2]int{p.a, p.b}] && deg[p.a] <= extraMaxDeg && deg[p.b] <= extraMaxDeg {
			add(p, false)
		}
	}

	// Bridge reds: islands − 1, the cheapest across two components each
	// round (the first in length order on a tie).
	for range islands - 1 {
		best, bestScore := -1, math.Inf(1)
		for i, p := range pairs {
			if uf.find(p.a) == uf.find(p.b) {
				continue
			}
			if s := score(p); s < bestScore {
				best, bestScore = i, s
			}
		}
		uf.union(pairs[best].a, pairs[best].b)
		add(pairs[best], true)
	}

	// Cycle reds: the cheapest pair beyond maxHops (first in length order
	// on a tie), the hops recomputed after each.
	limit := maxHops(n)
	hops := make([][]int, n)
	for {
		for s := range n {
			hops[s] = bfsHops(adj, s, hops[s])
		}
		best, bestScore := -1, math.Inf(1)
		for i, p := range pairs {
			if hops[p.a][p.b] > limit {
				if s := score(p); s < bestScore {
					best, bestScore = i, s
				}
			}
		}
		if best < 0 {
			break
		}
		add(pairs[best], true)
	}

	// Cover reds: while a lane alone holds more than chokeFrac·n stars,
	// the pair crossing the most such chokes per score (first in length
	// order on a tie): one red from a chain's end to another's lifts
	// several at once.
	for {
		ch := chokes(adj, int(chokeFrac*float64(n)))
		if len(ch) == 0 {
			break
		}
		best, bestW := -1, 0.0
		for i, p := range pairs {
			if laid[[2]int{p.a, p.b}] {
				continue
			}
			k := 0
			for _, h := range ch {
				if h.holds(p.a) != h.holds(p.b) {
					k++
				}
			}
			if w := float64(k) / score(p); k > 0 && w > bestW {
				best, bestW = i, w
			}
		}
		add(pairs[best], true)
	}

	slices.SortFunc(c.Lanes, func(p, q StarLane) int { return cmp.Or(p.A-q.A, p.B-q.B) })
}

// choke is a lane whose loss cuts a side off: the DFS subtree of its
// lower end, the stars of preorder [lo, hi).
type choke struct {
	pre    []int
	lo, hi int
}

// holds reports star s on the choke's cut-off side.
func (h choke) holds(s int) bool { return h.pre[s] >= h.lo && h.pre[s] < h.hi }

// chokes is the bridges of the connected graph adj (Tarjan's low links,
// a DFS from star 0) cutting off more than most stars on either side.
func chokes(adj [][]int, most int) []choke {
	n := len(adj)
	pre := make([]int, n)
	for i := range pre {
		pre[i] = -1
	}
	low, size := make([]int, n), make([]int, n)
	var out []choke
	t := 0
	var dfs func(v, parent int)
	dfs = func(v, parent int) {
		pre[v], low[v], size[v] = t, t, 1
		t++
		skip := true // one lane back to the parent; a second is a cycle
		for _, w := range adj[v] {
			switch {
			case w == parent && skip:
				skip = false
			case pre[w] >= 0:
				low[v] = min(low[v], pre[w])
			default:
				dfs(w, v)
				low[v] = min(low[v], low[w])
				size[v] += size[w]
				if low[w] > pre[v] && min(size[w], n-size[w]) > most {
					out = append(out, choke{pre, pre[w], pre[w] + size[w]})
				}
			}
		}
	}
	dfs(0, -1)
	return out
}

// bfsHops fills dst (reused when long enough) with the lane count from s
// to every star, −1 where unreached.
func bfsHops(adj [][]int, s int, dst []int) []int {
	if len(dst) != len(adj) {
		dst = make([]int, len(adj))
	}
	for i := range dst {
		dst[i] = -1
	}
	dst[s] = 0
	for q := []int{s}; len(q) > 0; q = q[1:] {
		for _, o := range adj[q[0]] {
			if dst[o] < 0 {
				dst[o] = dst[q[0]] + 1
				q = append(q, o)
			}
		}
	}
	return dst
}

// pickHome is the first star, in a shuffled order, with 2 or 3 lanes; else
// the first with the most lanes up to 3.
func (c *Cluster) pickHome(r *rand.Rand) int {
	order := r.Perm(len(c.Stars))
	for _, i := range order {
		if l := len(c.Stars[i].Lanes); l == 2 || l == 3 {
			return i
		}
	}
	home := order[0]
	for _, i := range order {
		if l := len(c.Stars[i].Lanes); l <= 3 && (len(c.Stars[home].Lanes) > 3 || l > len(c.Stars[home].Lanes)) {
			home = i
		}
	}
	return home
}

// pickHomes is n home stars, no draw: Home, then each time the star
// farthest in hops from the chosen ones (lowest index on a tie), among
// those with 2 or 3 lanes, else at most 3, else any. Panics past the star
// count.
func (c *Cluster) pickHomes(n int) []int {
	if n > len(c.Stars) {
		panic(fmt.Sprintf("%d homes on %d stars", n, len(c.Stars)))
	}
	if n <= 0 {
		return nil
	}
	adj := make([][]int, len(c.Stars))
	for _, l := range c.Lanes {
		adj[l.A] = append(adj[l.A], l.B)
		adj[l.B] = append(adj[l.B], l.A)
	}
	homes := []int{c.Home}
	chosen := make([]bool, len(c.Stars))
	chosen[c.Home] = true
	near := bfsHops(adj, c.Home, nil) // hops to the nearest chosen home
	tiers := []func(lanes int) bool{
		func(l int) bool { return l == 2 || l == 3 },
		func(l int) bool { return l <= 3 },
		func(int) bool { return true },
	}
	var hops []int
	for len(homes) < n {
		best := -1
		for _, ok := range tiers {
			for i := range c.Stars {
				if !chosen[i] && ok(len(c.Stars[i].Lanes)) && (best < 0 || near[i] > near[best]) {
					best = i
				}
			}
			if best >= 0 {
				break
			}
		}
		homes = append(homes, best)
		chosen[best] = true
		hops = bfsHops(adj, best, hops)
		for i, h := range hops {
			if h >= 0 && (near[i] < 0 || h < near[i]) {
				near[i] = h
			}
		}
	}
	return homes
}
