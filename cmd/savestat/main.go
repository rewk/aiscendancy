// Command savestat compares the empires of one of our saves (package
// save's JSON, not the original's .SAV: that is savedump).
//
//	savestat [file]            the table today, one row per empire, then each
//	                           empire's History every -every days, its ships
//	                           by hull and its built items by count
//	savestat -brief [file]     the table alone
//
// The file defaults to save.DefaultPath() (the quick save). Read only.
// Today's row is computed as game's History sample is (Colony.Shown
// summed); a History sample older than the build may hold the raw sums
// that sample used before (commit 323749c), not Shown.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/save"
)

func main() {
	every := flag.Int("every", 160, "days between the History rows shown")
	brief := flag.Bool("brief", false, "the table today alone")
	flag.Parse()
	path := save.DefaultPath()
	if flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	f, err := save.Read(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: day %d, %d stars, %d empires\n", path, f.Game.Day, len(f.Game.Cluster.Stars), len(f.Game.Empires))
	table(os.Stdout, f)
	if *brief {
		return
	}
	for _, e := range f.Game.Empires {
		fmt.Println()
		detail(os.Stdout, f, e, *every)
	}
}

// row is an empire today, as game's History sample, plus its ships armed
// and its contacts by stance.
type row struct {
	game.Sample
	armed                int
	met, wars, alliances int
}

func today(e *game.Empire) row {
	r := row{Sample: game.Sample{Day: e.Day, Colonies: len(e.Colonies), Ships: len(e.Ships), Systems: e.ControlledSystems()}}
	stars := map[int]bool{}
	for _, c := range e.Colonies {
		ind, res, pro := c.Shown()
		r.Pop += c.Pop
		r.Industry += ind
		r.Research += res
		r.Prosperity += pro
		stars[c.Star] = true
	}
	r.Stars = len(stars)
	for _, sh := range e.Ships {
		r.Fleet += sh.Design.Cost()
		if sh.Design.Weapons() > 0 {
			r.armed++
		}
	}
	for _, k := range e.Research.Known {
		if k {
			r.Techs++
		}
	}
	for _, v := range e.Visited {
		if v {
			r.Explored++
		}
	}
	for j := range e.Relations {
		if j == e.Index {
			continue
		}
		switch e.StanceWith(j) {
		case game.Peace:
			r.met++
		case game.War:
			r.met++
			r.wars++
		case game.Alliance:
			r.met++
			r.alliances++
		}
	}
	return r
}

// who is e's label: index, species, player or bot, defeated or won.
func who(f *save.File, e *game.Empire) string {
	s := fmt.Sprintf("%d %s", e.Index, e.Species)
	if e.Index >= len(f.Bots) || f.Bots[e.Index] == nil {
		s += "*"
	}
	switch {
	case !e.Alive():
		s += " (lost)"
	case e.Won != game.NoVictory:
		s += " (won)"
	}
	return s
}

// table prints one row per empire today; * marks the player.
func table(w io.Writer, f *save.File) {
	fmt.Fprintf(w, "%-20s %4s %5s %5s %5s %5s %5s %4s %4s %5s %7s %4s %4s %4s\n",
		"empire (*player)", "col", "pop", "ind", "res", "pros", "techs", "sys", "expl", "ships", "fleet", "met", "war", "ally")
	for _, e := range f.Game.Empires {
		r := today(e)
		fmt.Fprintf(w, "%-20.20s %4d %5d %5d %5d %5d %5d %4d %4d %2d/%-2d %7d %4d %4d %4d\n",
			who(f, e), r.Colonies, r.Pop, r.Industry, r.Research, r.Prosperity, r.Techs, r.Systems, r.Explored,
			r.armed, r.Ships, r.Fleet, r.met, r.wars, r.alliances)
	}
	fmt.Fprintln(w, "ships = armed/all, fleet = their cost; met/war/ally = empires by stance")
}

// detail prints e's History every `every` days (and its last), its ships
// by hull, its built items by count, its stances.
func detail(w io.Writer, f *save.File, e *game.Empire, every int) {
	fmt.Fprintf(w, "== %s\n", who(f, e))
	fmt.Fprintf(w, "  %5s %4s %5s %5s %5s %5s %5s %7s %5s %4s %4s %4s\n", "day", "col", "pop", "ind", "res", "pros", "ships", "fleet", "techs", "sys", "expl", "war")
	next := 0
	for i, s := range e.History {
		if s.Day < next && i != len(e.History)-1 {
			continue
		}
		next = s.Day + max(1, every)
		fmt.Fprintf(w, "  %5d %4d %5d %5d %5d %5d %5d %7d %5d %4d %4d %4d\n", s.Day, s.Colonies, s.Pop, s.Industry, s.Research, s.Prosperity, s.Ships, s.Fleet, s.Techs, s.Systems, s.Explored, s.Wars)
	}
	hulls := map[string]int{}
	for _, sh := range e.Ships {
		hulls[sh.Design.Hull.String()]++
	}
	fmt.Fprintf(w, "  ships: %s\n", counts(hulls))
	items := map[string]int{}
	for _, c := range e.Colonies {
		for _, q := range c.Planet.Surface {
			if q.Item != game.NoItem {
				items[q.Item.String()]++
			}
		}
		for _, it := range c.Planet.Orbit {
			if it != game.NoItem {
				items[it.String()]++
			}
		}
	}
	fmt.Fprintf(w, "  items: %s\n", counts(items))
	var st []string
	for j, r := range e.Relations {
		if j != e.Index && e.StanceWith(j) != game.Unknown {
			st = append(st, fmt.Sprintf("%d %s, met day %d", j, e.StanceWith(j), r.Met))
		}
	}
	if len(st) == 0 {
		st = []string{"none met"}
	}
	fmt.Fprintf(w, "  stances: %s\n", strings.Join(st, ", "))
}

// counts is m as "k=v", the largest first, ties by name; "-" when empty.
func counts(m map[string]int) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return m[ks[i]] > m[ks[j]] || m[ks[i]] == m[ks[j]] && ks[i] < ks[j] })
	if len(ks) == 0 {
		return "-"
	}
	out := make([]string, len(ks))
	for i, k := range ks {
		out[i] = fmt.Sprintf("%s=%d", k, m[k])
	}
	return strings.Join(out, " ")
}
