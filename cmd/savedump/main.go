// Command savedump prints the galaxy of Ascendancy save games, or of our
// own generator measured the same way.
//
//	savedump <file>              stars, planets per star, lanes, then the stats
//	savedump -stats <files...>   one stats line per distinct galaxy (saves of
//	                             the same game are merged by their star names)
//	                             and a mean row
//	savedump -gen average:7      game.NewCluster(7, Average) dumped like a save
//	savedump -stats -gen dense:1-30 [files...]
//	                             one line per seed, measured by the same
//	                             sav.Stats as the saves, then the mean row
//	savedump -diff a.sav b.sav   the bytes that differ, record by record
//	                             (diff.go); -diff-section ships keeps one
//	savedump -ships [-fmax 5000] file
//	                             per ship record, its 4-aligned f32 and the
//	                             one at +162 of magnitude in [1e-3, fmax]
//	savedump -colonies file      per colony its owner, population, project
//	                             and output as recorded, against
//	                             game.Colony.Shown on its squares
//	                             (colonies.go); exit 1 when a player's
//	                             colony differs
//
// -gen takes <density>:<seeds>, the density as game.ParseDensity reads it,
// the seeds one number or lo-hi. A cluster is converted with its positions
// × genScale so the extents read like a save's.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/sav"
)

// genScale maps the unit ball to the saves' outer shell radius 440 (file
// units, measured in the saves).
const genScale = 440

func main() {
	stats := flag.Bool("stats", false, "one stats line per distinct galaxy, then a mean row")
	gen := flag.String("gen", "", "measure our generator: <density>:<seed> or <density>:<lo>-<hi>")
	diff := flag.Bool("diff", false, "the byte differences between two saves")
	only := flag.String("diff-section", "", "with -diff: this section alone (header, tech, stars, squares, planets, ships, players, lanes, rest)")
	ships := flag.Bool("ships", false, "per ship record, the f32 that may be a position")
	fmax := flag.Float64("fmax", 5000, "with -ships: the largest magnitude shown")
	cols := flag.Bool("colonies", false, "per colony the output as recorded against the model's on its squares")
	flag.Parse()
	if *cols {
		if flag.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "usage: savedump -colonies file")
			os.Exit(2)
		}
		g, err := sav.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if colonies(os.Stdout, g) > 0 {
			os.Exit(1)
		}
		return
	}
	if *diff || *ships {
		os.Exit(raw(*diff, *only, *fmax))
	}
	var gens []galaxy
	if *gen != "" {
		var err error
		if gens, err = generate(*gen); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	switch {
	case *stats && (flag.NArg() > 0 || len(gens) > 0):
		gs := loadFiles(flag.Args())
		statsTable(append(gs, gens...))
	case !*stats && len(gens) == 1 && flag.NArg() == 0:
		dump(gens[0].g)
	case !*stats && len(gens) == 0 && flag.NArg() == 1:
		g, err := sav.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		dump(g)
	default:
		fmt.Fprintln(os.Stderr, "usage: savedump <file> | savedump -gen <density>:<seed> | savedump -stats [-gen <density>:<lo>-<hi>] [files...]")
		os.Exit(2)
	}
}

// raw runs -diff (two files) or -ships (one) and returns the exit code.
func raw(diff bool, only string, fmax float64) int {
	if diff && flag.NArg() != 2 || !diff && flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: savedump -diff [-diff-section name] a.sav b.sav | savedump -ships [-fmax f] file")
		return 2
	}
	var bs [][]byte
	for _, p := range flag.Args() {
		b, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		bs = append(bs, b)
	}
	var err error
	if diff {
		_, err = diffSaves(os.Stdout, bs[0], bs[1], only)
	} else {
		err = shipFloats(os.Stdout, bs[0], fmax)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// galaxy is one measured galaxy and its table label.
type galaxy struct {
	label string
	g     *sav.Game
}

// generate rolls the clusters of a -gen spec.
func generate(spec string) ([]galaxy, error) {
	ds, seeds, ok := strings.Cut(spec, ":")
	if !ok {
		return nil, fmt.Errorf("-gen %q: want <density>:<seeds>", spec)
	}
	d, err := game.ParseDensity(ds)
	if err != nil {
		return nil, err
	}
	los, his, rng := strings.Cut(seeds, "-")
	if !rng {
		his = los
	}
	lo, err1 := strconv.ParseUint(los, 10, 64)
	hi, err2 := strconv.ParseUint(his, 10, 64)
	if err1 != nil || err2 != nil || hi < lo {
		return nil, fmt.Errorf("-gen %q: bad seeds %q", spec, seeds)
	}
	var out []galaxy
	for seed := lo; ; seed++ {
		label := strings.ToLower(strings.ReplaceAll(d.String(), " ", "")) + "/" + strconv.FormatUint(seed, 10)
		out = append(out, galaxy{label, fromCluster(game.NewCluster(seed, d))})
		if seed == hi {
			return out, nil
		}
	}
}

// fromCluster converts a cluster into the save's galaxy: names, types,
// positions × genScale, planets, lanes as they are.
func fromCluster(c *game.Cluster) *sav.Game {
	g := &sav.Game{}
	for i, s := range c.Stars {
		g.Stars = append(g.Stars, sav.Star{
			Name:        s.Name,
			Type:        int(s.Type),
			Pos:         [3]float32{float32(s.Pos.X * genScale), float32(s.Pos.Y * genScale), float32(s.Pos.Z * genScale)},
			PlanetCount: len(s.System.Planets),
		})
		for _, p := range s.System.Planets {
			g.Planets = append(g.Planets, sav.Planet{Name: p.Name, Star: i, Size: p.Size, Type: int(p.Type)})
		}
	}
	for _, l := range c.Lanes {
		g.Lanes = append(g.Lanes, sav.Lane{A: l.A, B: l.B, Red: l.Red})
	}
	return g
}

// loadFiles opens the saves, drops the repeats of a galaxy (same Key) and
// labels each by its file name, with its directory when two share a name.
func loadFiles(paths []string) []galaxy {
	seen, labels := map[string]bool{}, map[string]bool{}
	var out []galaxy
	for _, p := range paths {
		g, err := sav.Open(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		if seen[g.Key()] {
			continue
		}
		seen[g.Key()] = true
		label := filepath.Base(p)
		if labels[label] { // 01.sav in two directories
			label = filepath.Join(filepath.Base(filepath.Dir(p)), label)
		}
		labels[label] = true
		out = append(out, galaxy{label, g})
	}
	return out
}

func dump(g *sav.Game) {
	fmt.Printf("%d stars\n", len(g.Stars))
	for i, s := range g.Stars {
		fmt.Printf("%3d %-14s %-20s %8.1f %8.1f %8.1f  %d planets\n",
			i, s.Name, game.StarType(s.Type), s.Pos[0], s.Pos[1], s.Pos[2], s.PlanetCount)
	}
	fmt.Printf("\n%d lanes\n", len(g.Lanes))
	for _, l := range g.Lanes {
		kind := "blue"
		if l.Red {
			kind = "RED"
		}
		fmt.Printf("%3d %-14s %3d %-14s %7.1f %s\n", l.A, g.Stars[l.A].Name, l.B, g.Stars[l.B].Name, g.Dist(l.A, l.B), kind)
	}
	s := g.Stats()
	fmt.Println()
	fmt.Printf("lanes per star %.2f, red %d (%.0f%%), blue %.1f (%.2f sp), red %.1f (%.2f sp), red/blue %.2f\n",
		s.LanesPerStar, s.Red, 100*s.RedShare, s.BlueLen, s.BlueSp, s.RedLen, s.RedSp, ratio(s))
	fmt.Printf("dead ends %.0f%%, longest dead-end chain %d, connected %v, diameter %d hops\n", 100*s.DeadEnds, s.LongestChain, s.Connected, s.Diameter)
	fmt.Printf("blue islands %d, nearest neighbour blue %.0f%%, red / shortest bridge %.2f\n", s.Islands, 100*s.NNBlue, s.RedBridge)
	fmt.Printf("degrees %s\n", degrees(s))
	fmt.Printf("extent %.0f x %.0f x %.0f, spacing %.1f, mean radius %.1f, y/xz %.2f\n",
		s.Extent[0], s.Extent[1], s.Extent[2], s.Spacing, s.MeanRadius, s.YFlat)
}

func ratio(s sav.Stats) float64 {
	if s.BlueLen == 0 {
		return 0
	}
	return s.RedLen / s.BlueLen
}

func degrees(s sav.Stats) string {
	var ks []int
	for k := range s.Degree {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	out := ""
	for _, k := range ks {
		out += fmt.Sprintf(" %d:%d", k, s.Degree[k])
	}
	return out
}

const head = "%-14s %5s %5s %5s %6s %6s %6s %6s %6s %5s %5s %4s %4s %5s %5s %5s %s\n"
const row = "%-14s %5d %5d %5.2f %5.1f%% %6.2f %6.2f %6.2f %5.1f%% %5d %5v %4.2f %4d %4.0f%% %5.2f %5d %s\n"

// redCols are the columns of the mean row averaged over the galaxies
// with red lanes only (red share, red length, red/blue); brdgCol over the
// galaxies with a red between two blue islands only (a generated galaxy
// may hold cycle reds alone, its RedBridge 0).
var redCols = map[int]bool{1: true, 3: true, 4: true}

const brdgCol = 10

func statsTable(gs []galaxy) {
	fmt.Printf(head, "file", "stars", "lanes", "l/s", "red", "blue", "red", "r/b", "dead", "chain", "conn", "y/xz", "isl", "nnB", "brdg", "diam", "degrees")
	fmt.Printf(head, "", "", "", "", "", "sp", "sp", "", "", "", "", "", "", "", "", "", "")
	var m [12]float64
	n, nr, nb := 0, 0, 0
	for _, x := range gs {
		s := x.g.Stats()
		fmt.Printf(row, x.label, s.Stars, s.Lanes, s.LanesPerStar, 100*s.RedShare, s.BlueSp, s.RedSp, ratio(s),
			100*s.DeadEnds, s.LongestChain, s.Connected, s.YFlat, s.Islands, 100*s.NNBlue, s.RedBridge, s.Diameter, degrees(s))
		for i, v := range []float64{s.LanesPerStar, 100 * s.RedShare, s.BlueSp, s.RedSp, ratio(s), 100 * s.DeadEnds,
			float64(s.LongestChain), s.YFlat, float64(s.Islands), 100 * s.NNBlue, s.RedBridge, float64(s.Diameter)} {
			if s.Red > 0 || !redCols[i] { // brdg: 0 without a bridge red
				m[i] += v
			}
		}
		n++
		if s.Red > 0 {
			nr++
		}
		if s.RedBridge > 0 {
			nb++
		}
	}
	if n == 0 {
		return
	}
	for i := range m {
		if i == brdgCol {
			if nb > 0 {
				m[i] /= float64(nb)
			}
		} else if redCols[i] {
			if nr > 0 {
				m[i] /= float64(nr)
			}
		} else {
			m[i] /= float64(n)
		}
	}
	fmt.Printf("%-14s %5s %5s %5.2f %5.1f%% %6.2f %6.2f %6.2f %5.1f%% %5.1f %5s %4.2f %4.1f %4.0f%% %5.2f %5.1f  (red figures over the %d with reds, brdg over the %d with bridges)\n",
		fmt.Sprintf("mean of %d", n), "", "", m[0], m[1], m[2], m[3], m[4], m[5], m[6], "", m[7], m[8], m[9], m[10], m[11], nr, nb)
}
