package main

import (
	"fmt"
	"io"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/sav"
)

// colony rebuilds a save's colony as the model's: its built squares
// (Kind the colour code, a ruins square's code−8; the last 10 the orbit
// slots), Pop, an ongoing project (the only kind Output reads); nil
// without a population.
func colony(p sav.Planet) *game.Colony {
	if p.Pop == 0 {
		return nil
	}
	pl := &game.Planet{Name: p.Name, Surface: map[game.Cell]*game.Square{}, Owner: p.Owner}
	for i := range pl.Orbit {
		pl.Orbit[i] = game.NoItem
	}
	surf := p.Surface()
	for k, q := range p.Squares {
		it := game.NoItem
		if q.Built && q.Item < int(game.NumItems) {
			it = game.Item(q.Item)
		}
		if k >= len(surf) {
			pl.Orbit[k-len(surf)] = it
			continue
		}
		kind := q.Code
		if q.Ruins() {
			kind -= 8
		}
		pl.Surface[game.Cell{I: k}] = &game.Square{Kind: game.SquareKind(kind), Item: it, Ruins: q.Ruins(), Automated: q.Automated}
	}
	c := &game.Colony{Planet: pl, Pop: p.Pop, Star: -1, PlanetIdx: -1}
	if p.Project < int(game.NumItems) && game.Item(p.Project).Ongoing() {
		c.Project = &game.Project{Item: game.Item(p.Project)}
	}
	return c
}

// shown is c's output as a save records it: Colony.Shown, the
// prosperity at least 0.
func shown(c *game.Colony) [3]int {
	ind, res, pros := c.Shown()
	return [3]int{ind, res, max(0, pros)}
}

// recorded is the planet record's output, in shown's order.
func recorded(p sav.Planet) [3]int { return [3]int{p.Industry, p.Research, p.Prosperity} }

// colonies prints every colony of g, the record's output then the
// model's on its squares in brackets, "<<" on a player's colony that
// differs; then the player's tally and the AIs' industry over the
// model's (antag.exe's AIs cheat). It returns the player's disagreements.
func colonies(w io.Writer, g *sav.Game) int {
	fmt.Fprintf(w, "day %d; output as recorded [as game.Colony.Shown computes it on the squares]\n", g.Day)
	fmt.Fprintf(w, "%-16s %3s %3s %-24s %-10s %-10s %-10s\n", "planet", "own", "pop", "project", "industry", "research", "prosperity")
	agree, differ := 0, 0
	aiRec, aiOurs := 0, 0
	for _, p := range g.Planets {
		c := colony(p)
		if c == nil {
			continue
		}
		proj := "-"
		if p.Project < int(game.NumItems) {
			proj = game.Item(p.Project).String()
		}
		rec, ours := recorded(p), shown(c)
		fmt.Fprintf(w, "%-16s %3d %3d %-24.24s", p.Name, p.Owner, p.Pop, proj)
		for k := range rec {
			fmt.Fprintf(w, " %-10s", fmt.Sprintf("%d [%d]", rec[k], ours[k]))
		}
		switch {
		case p.Owner != 0:
			aiRec, aiOurs = aiRec+rec[0], aiOurs+ours[0]
		case rec == ours:
			agree++
		default:
			differ++
			fmt.Fprint(w, " <<")
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "the player's colonies: %d agree, %d differ\n", agree, differ)
	if aiOurs > 0 {
		fmt.Fprintf(w, "the AIs' industry: %d recorded for %d computed, x%.2f\n", aiRec, aiOurs, float64(aiRec)/float64(aiOurs))
	}
	return differ
}
