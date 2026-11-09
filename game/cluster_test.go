package game

import (
	"bufio"
	"bytes"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/cob"
)

// gameTxt reads one of the game's text files, or skips without the game.
func gameTxt(t *testing.T, name string) []string {
	t.Helper()
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	g, err := cob.OpenGame(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	b, err := g.Read(name)
	if err != nil {
		t.Fatal(err)
	}
	// CRLF lines, then blank ones and a DOS ^Z end-of-file byte.
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		if l := strings.Trim(sc.Text(), "\r\x1a \t"); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// starstat.txt: "TYPE_n<TAB>pp.pp Percent<TAB>l.ll Lanes".
func TestStarWeightsMatchGame(t *testing.T) {
	lines := gameTxt(t, "starstat.txt")
	if len(lines) != NumStarTypes {
		t.Fatalf("%d lines, want %d", len(lines), NumStarTypes)
	}
	for i, l := range lines {
		f := strings.Fields(l)
		if len(f) != 5 || f[0] != "TYPE_"+strconv.Itoa(i) || f[2] != "Percent" {
			t.Fatalf("line %d: %q", i, l)
		}
		pct, err := strconv.ParseFloat(f[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		if pct != starWeights[i] {
			t.Errorf("type %d: %v%%, typed %v", i, pct, starWeights[i])
		}
	}
}

func TestStarNamesMatchGame(t *testing.T) {
	lines := gameTxt(t, "names.txt")
	if !slices.Equal(lines, starNames) {
		t.Errorf("names.txt has %d names, typed %d, or they differ", len(lines), len(starNames))
	}
	seen := map[string]bool{}
	for _, n := range starNames {
		if seen[n] {
			t.Errorf("duplicate name %q", n)
		}
		seen[n] = true
	}
}

func TestParseDensity(t *testing.T) {
	for d := range NumDensities {
		for _, s := range []string{d.String(), strings.ToUpper(d.String()),
			strings.ReplaceAll(d.String(), " ", ""), strconv.Itoa(int(d))} {
			if got, err := ParseDensity(s); err != nil || got != d {
				t.Errorf("ParseDensity(%q) = %v, %v; want %v", s, got, err, d)
			}
		}
	}
	if d, _ := ParseDensity("very dense"); d != VeryDense {
		t.Errorf("very dense: %v", d)
	}
	for _, s := range []string{"", "5", "-1", "huge"} {
		if _, err := ParseDensity(s); err == nil {
			t.Errorf("ParseDensity(%q): no error", s)
		}
	}
}

func TestClusterDeterministic(t *testing.T) {
	a, b, c := NewCluster(7, Average), NewCluster(7, Average), NewCluster(8, Average)
	same := true
	for i := range a.Stars {
		if a.Stars[i].Pos != b.Stars[i].Pos || a.Stars[i].Name != b.Stars[i].Name {
			t.Errorf("seed 7 twice: star %d differs", i)
		}
		same = same && a.Stars[i].Pos == c.Stars[i].Pos
	}
	if !slices.Equal(a.Lanes, b.Lanes) || a.Home != b.Home {
		t.Error("seed 7 twice: lanes or home differ")
	}
	if same {
		t.Error("seeds 7 and 8 give the same cluster")
	}
}

// No two stars closer than minSep (unit-ball units).
func TestClusterMinSep(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		c := NewCluster(seed, Average)
		for a := range c.Stars {
			for b := a + 1; b < len(c.Stars); b++ {
				if d := c.Dist(a, b); d < minSep {
					t.Errorf("seed %d: stars %d %d %.4f apart", seed, a, b, d)
				}
			}
		}
	}
}

// Every star on one of the six shells; the outer shell holds about half
// the stars, the next about a quarter (the geometric draw).
func TestClusterShells(t *testing.T) {
	var count [numShells]int
	total := 0
	for seed := uint64(1); seed <= 10; seed++ {
		c := NewCluster(seed, Average)
		for i, s := range c.Stars {
			r := math.Sqrt(s.Pos.X*s.Pos.X + s.Pos.Y*s.Pos.Y + s.Pos.Z*s.Pos.Z)
			k := slices.IndexFunc(shellRadii[:], func(sr float64) bool { return math.Abs(r-sr) < 1e-9 })
			if k < 0 {
				t.Errorf("seed %d star %d: radius %.6f on no shell", seed, i, r)
				continue
			}
			count[k]++
			total++
		}
	}
	if f := float64(count[0]) / float64(total); f < 0.40 || f > 0.60 {
		t.Errorf("outer shell %.0f%% of the stars, want 40-60%% (%v)", 100*f, count)
	}
	if f := float64(count[1]) / float64(total); f < 0.15 || f > 0.35 {
		t.Errorf("shell 1 %.0f%% of the stars, want 15-35%% (%v)", 100*f, count)
	}
}

func TestClusterShape(t *testing.T) {
	for d := range NumDensities {
		for seed := uint64(1); seed <= 10; seed++ {
			checkCluster(t, NewCluster(seed, d), seed)
		}
	}
}

func checkCluster(t *testing.T, c *Cluster, seed uint64) {
	t.Helper()
	n := c.Density.Stars()
	if len(c.Stars) != n {
		t.Fatalf("%v seed %d: %d stars", c.Density, seed, len(c.Stars))
	}
	names := map[string]bool{}
	for i, s := range c.Stars {
		if names[s.Name] {
			t.Errorf("%v seed %d: name %q twice", c.Density, seed, s.Name)
		}
		names[s.Name] = true
		p := s.Pos
		if p.X*p.X+p.Y*p.Y+p.Z*p.Z > 1+1e-9 || s.Type >= NumStarTypes {
			t.Errorf("%v seed %d star %d: at %v, type %d", c.Density, seed, i, p, s.Type)
		}
	}

	pairs := map[[2]int]bool{}
	deg := make([]int, n)
	if !slices.IsSortedFunc(c.Lanes, func(p, q StarLane) int {
		if p.A != q.A {
			return p.A - q.A
		}
		return p.B - q.B
	}) {
		t.Errorf("%v seed %d: lanes not sorted by (A,B)", c.Density, seed)
	}
	for _, l := range c.Lanes {
		if l.A >= l.B || l.A < 0 || l.B >= n {
			t.Errorf("%v seed %d: lane %+v", c.Density, seed, l)
			continue
		}
		if pairs[[2]int{l.A, l.B}] {
			t.Errorf("%v seed %d: lane %+v twice", c.Density, seed, l)
		}
		pairs[[2]int{l.A, l.B}] = true
		deg[l.A]++
		deg[l.B]++
	}

	// The whole graph is connected; no blue lane is longer than the cap,
	// but a star's nearest-neighbour lane, laid whatever its length.
	if maxComp(components(n, c.Lanes, true)) != 0 {
		t.Errorf("%v seed %d: lanes not connected", c.Density, seed)
	}
	// No lane alone holds more than chokeFrac·n stars: removed, the
	// smaller side is at most that (components, not chokes' DFS).
	for i, l := range c.Lanes {
		rest := slices.Delete(slices.Clone(c.Lanes), i, i+1)
		comp, side := components(n, rest, true), 0
		for _, k := range comp {
			if k == comp[l.A] {
				side++
			}
		}
		if small := min(side, n-side); float64(small) > chokeFrac*float64(n) {
			t.Errorf("%v seed %d: lane %+v alone holds %d stars", c.Density, seed, l, small)
		}
	}
	for _, l := range c.Lanes {
		d := c.Dist(l.A, l.B)
		nnLane := nearest(c, l.A) == l.B || nearest(c, l.B) == l.A
		if !l.Red && !nnLane && d > blueCap/math.Sqrt(float64(n))+1e-12 {
			t.Errorf("%v seed %d: blue lane %+v %.3f long, cap %.3f", c.Density, seed, l, d, blueCap/math.Sqrt(float64(n)))
		}
	}

	for i, s := range c.Stars {
		if deg[i] < 1 || len(s.Lanes) != deg[i] || !slices.IsSorted(s.Lanes) {
			t.Errorf("%v seed %d star %d: degree %d, lanes %v", c.Density, seed, i, deg[i], s.Lanes)
		}
		for _, li := range s.Lanes {
			if l := c.Lanes[li]; l.A != i && l.B != i {
				t.Errorf("%v seed %d star %d: lane %d is %+v", c.Density, seed, i, li, l)
			}
		}
		sys := s.System
		if sys.Name != s.Name || sys.Star != s.Type || len(sys.Lanes) != len(s.Lanes) {
			t.Fatalf("%v seed %d star %d: system %q type %d, %d lanes", c.Density, seed, i, sys.Name, sys.Star, len(sys.Lanes))
		}
		if n := len(sys.Planets); n < 1 || n > 5 || i == c.Home && (n < 2 || n > 4) {
			t.Errorf("%v seed %d star %d: %d planets", c.Density, seed, i, n)
		}
		for k, li := range s.Lanes {
			end, to := sys.Lanes[k], c.Other(li, i)
			if end.To != to || end.Red != c.Lanes[li].Red {
				t.Errorf("%v seed %d star %d end %d: to %d red %v, lane %d is %+v", c.Density, seed, i, k, end.To, end.Red, li, c.Lanes[li])
			}
			p, q := s.Pos, c.Stars[to].Pos
			want := math.Atan2(q.Z-p.Z, q.X-p.X)
			if d := math.Remainder(math.Atan2(end.Pos.Z, end.Pos.X)-want, 2*math.Pi); math.Abs(d) > 1e-9 {
				t.Errorf("%v seed %d star %d end %d: %.3g rad off its destination", c.Density, seed, i, k, d)
			}
		}
	}

	home := c.Stars[c.Home]
	if l := len(home.Lanes); l < 2 || l > 3 {
		t.Errorf("%v seed %d: home has %d lanes", c.Density, seed, l)
	}
	ps := home.System.Planets
	if len(ps) < 2 || len(ps) > 4 {
		t.Fatalf("%v seed %d: home has %d planets", c.Density, seed, len(ps))
	}
	sq := ps[0].Surface[Cell{Centre, Centre}]
	if sq == nil || sq.Item != ColonyBase || ps[0].Name != home.Name+" I" || ps[0].Pos == (Vec3{}) {
		t.Errorf("%v seed %d: home planet %q at %v, centre %+v", c.Density, seed, ps[0].Name, ps[0].Pos, sq)
	}
}

// components labels each star with its component, the lowest star
// index in it, by a BFS over the lanes (all of them, or the blue ones
// only when withRed is false).
func components(n int, lanes []StarLane, withRed bool) []int {
	adj := make([][]int, n)
	for _, l := range lanes {
		if withRed || !l.Red {
			adj[l.A] = append(adj[l.A], l.B)
			adj[l.B] = append(adj[l.B], l.A)
		}
	}
	comp := make([]int, n)
	for i := range comp {
		comp[i] = -1
	}
	for s := range n {
		if comp[s] >= 0 {
			continue
		}
		comp[s] = s
		for q := []int{s}; len(q) > 0; q = q[1:] {
			for _, o := range adj[q[0]] {
				if comp[o] < 0 {
					comp[o] = s
					q = append(q, o)
				}
			}
		}
	}
	return comp
}

// maxComp is the highest component label: 0 when all are one.
func maxComp(comp []int) int { return slices.Max(comp) }

// nearest is a star's nearest neighbour, the lowest index on a tie.
func nearest(c *Cluster, a int) int {
	nn := -1
	for b := range c.Stars {
		if b != a && (nn < 0 || c.Dist(a, b) < c.Dist(a, nn)) {
			nn = b
		}
	}
	return nn
}

// hopDiameter is the most lanes on a shortest path between two stars, by
// a BFS from every star; −1 when some star is unreached.
func hopDiameter(c *Cluster) int {
	n := len(c.Stars)
	most := 0
	for s := range n {
		hops := make([]int, n)
		for i := range hops {
			hops[i] = -1
		}
		hops[s] = 0
		for q := []int{s}; len(q) > 0; q = q[1:] {
			for _, li := range c.Stars[q[0]].Lanes {
				if o := c.Other(li, q[0]); hops[o] < 0 {
					hops[o] = hops[q[0]] + 1
					q = append(q, o)
				}
			}
		}
		for _, h := range hops {
			if h < 0 {
				return -1
			}
			most = max(most, h)
		}
	}
	return most
}

// Every star's nearest neighbour is a blue partner, so no star is a blue
// island of one and no star's only lane is red.
func TestNNBlue(t *testing.T) {
	for d := range NumDensities {
		for seed := uint64(1); seed <= 10; seed++ {
			c := NewCluster(seed, d)
			blue := map[[2]int]bool{}
			for _, l := range c.Lanes {
				if !l.Red {
					blue[[2]int{l.A, l.B}] = true
				}
			}
			for a, s := range c.Stars {
				nn := nearest(c, a)
				if !blue[[2]int{min(a, nn), max(a, nn)}] {
					t.Errorf("%v seed %d: star %d's nearest neighbour %d is no blue partner", d, seed, a, nn)
				}
				if len(s.Lanes) == 1 && c.Lanes[s.Lanes[0]].Red {
					t.Errorf("%v seed %d: star %d's only lane is red", d, seed, a)
				}
			}
		}
	}
}

// No star more than maxHops(n) lanes from another; and the cycle reds
// (past the islands − 1 bridges) are seen to fire somewhere.
func TestDiameter(t *testing.T) {
	fired := false
	for d := range NumDensities {
		for seed := uint64(1); seed <= 10; seed++ {
			c := NewCluster(seed, d)
			n := len(c.Stars)
			if h := hopDiameter(c); h < 0 || h > maxHops(n) {
				t.Errorf("%v seed %d: diameter %d hops, cap %d", d, seed, h, maxHops(n))
			}
			isl := components(n, c.Lanes, false)
			islands, reds := 0, 0
			for i, k := range isl {
				if k == i {
					islands++
				}
			}
			for _, l := range c.Lanes {
				if l.Red {
					reds++
				}
			}
			fired = fired || reds > islands-1
		}
	}
	if !fired {
		t.Error("no cluster holds a red past the bridges: the hop cap never fired")
	}
}

// maxHops is ceil(33% of n): 100 stars give 33, not 34 (a float trap).
func TestMaxHops(t *testing.T) {
	for d, want := range [NumDensities]int{5, 9, 17, 25, 33} {
		if got := maxHops(Density(d).Stars()); got != want {
			t.Errorf("%v: maxHops %d, want %d", Density(d), got, want)
		}
	}
}

// The reds bridge the blue islands: at least islands − 1 of them; the reds
// across two islands join them all into one component (the others lie
// inside an island, already connected); no red shorter than the closest
// pair of the islands it joins. The generator's order is not observable
// (Lanes are sorted by (A,B)), so the bridges are not told from the cycle
// reds that also cross islands.
func TestRedBridges(t *testing.T) {
	for d := range NumDensities {
		for seed := uint64(1); seed <= 10; seed++ {
			c := NewCluster(seed, d)
			n := len(c.Stars)
			isl := components(n, c.Lanes, false)
			islands := 0
			for i, k := range isl {
				if k == i {
					islands++
				}
			}
			uf := newUnionFind(n)
			red := 0
			for _, l := range c.Lanes {
				if !l.Red {
					continue
				}
				red++
				ia, ib := isl[l.A], isl[l.B]
				if ia == ib {
					continue
				}
				uf.union(ia, ib)
				short := math.Inf(1)
				for a := range n {
					for b := range n {
						if isl[a] == ia && isl[b] == ib {
							short = min(short, c.Dist(a, b))
						}
					}
				}
				if c.Dist(l.A, l.B) < short {
					t.Errorf("%v seed %d: red %+v %.4f, shorter than its islands' closest pair %.4f", d, seed, l, c.Dist(l.A, l.B), short)
				}
			}
			for i, k := range isl {
				if k == i && uf.find(k) != uf.find(isl[0]) {
					t.Errorf("%v seed %d: island %d not bridged to island %d by the reds", d, seed, k, isl[0])
				}
			}
			if red < islands-1 {
				t.Errorf("%v seed %d: %d reds over %d blue islands", d, seed, red, islands)
			}
			if maxComp(components(n, c.Lanes, true)) != 0 {
				t.Errorf("%v seed %d: not connected", d, seed)
			}
		}
	}
}

// The means land near the saves' (docs/galaxy.md): lanes per star, red
// share, dead ends, pooled over the seeds. The cycle reds raise the lanes
// and the red share past the saves' on purpose (most at Vacuous).
func TestClusterStats(t *testing.T) {
	for _, k := range []struct {
		d              Density
		seeds          uint64
		lpsLo, lpsHi   float64
		redLo, redHi   float64
		deadLo, deadHi float64
	}{
		{Average, 30, 2.05, 2.4, 0.05, 0.16, 0.15, 0.32},
		{VeryDense, 30, 2.05, 2.4, 0.06, 0.18, 0.15, 0.32},
		{Vacuous, 20, 2.0, 2.8, 0.05, 0.25, 0, 1},
	} {
		lanes, red, stars, dead := 0, 0, 0, 0
		for seed := uint64(1); seed <= k.seeds; seed++ {
			c := NewCluster(seed, k.d)
			lanes += len(c.Lanes)
			stars += len(c.Stars)
			for _, l := range c.Lanes {
				if l.Red {
					red++
				}
			}
			for _, s := range c.Stars {
				if len(s.Lanes) == 1 {
					dead++
				}
			}
		}
		lps, rs, de := 2*float64(lanes)/float64(stars), float64(red)/float64(lanes), float64(dead)/float64(stars)
		if lps < k.lpsLo || lps > k.lpsHi || rs < k.redLo || rs > k.redHi || de < k.deadLo || de > k.deadHi {
			t.Errorf("%v: %.2f lanes per star, %.1f%% red, %.1f%% dead ends", k.d, lps, 100*rs, 100*de)
		}
	}
}

// Other and Dist on a hand-made cluster.
func TestOtherAndDist(t *testing.T) {
	c := &Cluster{Stars: []Star{{Pos: Vec3{}}, {Pos: Vec3{3, 4, 0}}}, Lanes: []StarLane{{A: 0, B: 1}}}
	if c.Other(0, 0) != 1 || c.Other(0, 1) != 0 || c.Dist(0, 1) != 5 {
		t.Errorf("Other %d %d, Dist %v", c.Other(0, 0), c.Other(0, 1), c.Dist(0, 1))
	}
}

// TestStarNamesMatchStatic: static.txt's strings 178-190 are the star type
// names in suns.shp order.
func TestStarNamesMatchStatic(t *testing.T) {
	lines := gameTxt(t, "static.txt")
	var got []string
	for i, l := range lines {
		if strings.HasPrefix(l, "// 178 - 190") {
			for _, q := range lines[i+1 : i+1+int(NumStarTypes)] {
				got = append(got, strings.Trim(q, "\""))
			}
			break
		}
	}
	if len(got) != int(NumStarTypes) {
		t.Fatalf("found %d names: %q", len(got), got)
	}
	for i, want := range got {
		if StarType(i).String() != want {
			t.Errorf("star type %d: %q, static.txt says %q", i, StarType(i), want)
		}
	}
}

// pickHomes spreads the homes: distinct, Home first, on 2-3 lane stars
// while some remain, each next home the farthest of its tier from its
// nearest earlier home (re-derived from a full hop matrix, BFS of Lanes),
// ties by the lowest index. Two homes are at least the cluster's median
// pairwise hop distance apart; from three on the greedy pick does NOT
// keep every pair there (seeds 1-20: 1-3 hops short on ~1/3 of the n = 4-5
// cases), so the median bound is asserted for n = 2 only.
func TestPickHomes(t *testing.T) {
	for _, d := range []Density{Average, Dense} {
		for seed := uint64(1); seed <= 20; seed++ {
			c := NewCluster(seed, d)
			adj := make([][]int, len(c.Stars))
			for _, l := range c.Lanes {
				adj[l.A] = append(adj[l.A], l.B)
				adj[l.B] = append(adj[l.B], l.A)
			}
			hops := make([][]int, len(c.Stars))
			var all []int
			for s := range c.Stars {
				hops[s] = bfsHops(adj, s, nil)
				for o := s + 1; o < len(c.Stars); o++ {
					all = append(all, hops[s][o])
				}
			}
			slices.Sort(all)
			median := all[len(all)/2]
			tier := func(s int) int { // 0: 2-3 lanes, 1: at most 3, 2: more
				switch l := len(c.Stars[s].Lanes); {
				case l == 2 || l == 3:
					return 0
				case l <= 3:
					return 1
				}
				return 2
			}
			// minHops is the hop count from s to the nearest of homes.
			minHops := func(s int, homes []int) int {
				m := -1
				for _, h := range homes {
					if m < 0 || hops[s][h] < m {
						m = hops[s][h]
					}
				}
				return m
			}
			for n := 2; n <= 5; n++ {
				homes := c.pickHomes(n)
				if len(homes) != n || homes[0] != c.Home {
					t.Fatalf("%v seed %d n %d: homes %v, Home %d", d, seed, n, homes, c.Home)
				}
				for i := 1; i < n; i++ {
					h, earlier := homes[i], homes[:i]
					if slices.Contains(earlier, h) {
						t.Fatalf("%v seed %d n %d: home %d twice", d, seed, n, h)
					}
					// The oracle: the best tier left, then the farthest, then the lowest index.
					want := -1
					for s := range c.Stars {
						if slices.Contains(earlier, s) {
							continue
						}
						if want < 0 || tier(s) < tier(want) || tier(s) == tier(want) && minHops(s, earlier) > minHops(want, earlier) {
							want = s
						}
					}
					if h != want {
						t.Errorf("%v seed %d n %d: home %d is %d (tier %d, %d hops), want %d (tier %d, %d hops)", d, seed, n, i,
							h, tier(h), minHops(h, earlier), want, tier(want), minHops(want, earlier))
					}
				}
				if n == 2 && hops[homes[0]][homes[1]] < median {
					t.Errorf("%v seed %d: homes %v %d hops apart, median %d", d, seed, homes, hops[homes[0]][homes[1]], median)
				}
				if seed == 1 && d == Average && n == 4 {
					t.Logf("seed 1 Average n 4: homes %v, median %d, hops 0-1 %d 0-2 %d 0-3 %d 1-2 %d 1-3 %d 2-3 %d", homes, median,
						hops[homes[0]][homes[1]], hops[homes[0]][homes[2]], hops[homes[0]][homes[3]],
						hops[homes[1]][homes[2]], hops[homes[1]][homes[3]], hops[homes[2]][homes[3]])
				}
			}
		}
	}
}
