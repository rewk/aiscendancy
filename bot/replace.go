package bot

import (
	"math"

	"github.com/rewk/aiscendancy/game"
)

// replacePick is the structure col replaces by a better one
// (Colony.Replace), ok false when none qualifies. Candidates: a surface
// structure old with an output and no MaxPop (never a Habitat,
// Metroplex, Outpost, Tubes, defence, plant or the Colony Base), and a
// surface item new research allows with g.Build > 0, an output and no
// MaxPop, CanReplace nil. On the colony without its ongoing project
// (yielded), Σ over industry, research and prosperity:
//
//	gain = Σ(OutputWith(at, new) − Output())
//	loss = ⌈Cost(new) / industry of OutputWith(at, NoItem)⌉ × Σ(Output() − OutputWith(at, NoItem))
//
// It qualifies when gain > 0 and gain × g.ReplaceDays ≥ loss (an empty
// colony making no industry never qualifies unless the old structure
// costs nothing). The best gain/loss wins (loss 0 first), ties in Cells
// order then item index. The gain is unweighted: Σ output only grows
// swap after swap, so a reverse swap never qualifies (a weight on the
// scarcest output made the bots swap Factory and Laboratory back and
// forth).
func replacePick(col *game.Colony, r *game.Research, g *Genome) (at game.Cell, it game.Item, ok bool) {
	return bestSwap(col, r, g.ReplaceDays, func(_, nw game.Item) bool { return g.Build[nw] > 0 && upgradeOpen(col, nw, g.Upgrade) })
}

// tierReplace is the fixed policy's replacement: replacePick's rule at
// replaceDays, a structure of an output family (family) swapped only for
// its family's best known tier (bestTier): Factory for an Industrial
// Megafacility, Laboratory for a Research Campus, Agriplot for an
// Artificial Hydroponifer. No Upgrade threshold: build calls it on a
// planet full or out of workers, where no base structure can be added
// (a small planet never reaches 3 of a family).
func tierReplace(col *game.Colony, r *game.Research) (at game.Cell, it game.Item, ok bool) {
	return bestSwap(col, r, replaceDays, func(old, nw game.Item) bool {
		f := family(old)
		return f >= 0 && nw == bestTier(r, f, nil)
	})
}

// bestSwap is replacePick's search, horizon its ReplaceDays, the pairs
// old → nw limited to those swap allows.
func bestSwap(col *game.Colony, r *game.Research, horizon float64, swap func(old, nw game.Item) bool) (at game.Cell, it game.Item, ok bool) {
	y := yielded(col)
	p := col.Planet
	bi, br, bp := y.Output()
	before := bi + br + bp
	bestRatio := 0.0
	for _, c := range p.Cells() {
		old := p.Surface[c].Item
		if old == game.NoItem || !replaceable(old) {
			continue
		}
		ei, er, ep := y.OutputWith(c, game.NoItem)
		lost := before - (ei + er + ep)
		for nw := game.Item(0); nw < game.FirstOrbital; nw++ {
			if nw == old || !swap(old, nw) || !replaceable(nw) || !r.Allows(nw) {
				continue
			}
			ai, ar, ap := y.OutputWith(c, nw)
			gain := float64(ai + ar + ap - before)
			if gain <= 0 {
				continue
			}
			loss := 0.0
			if lost > 0 {
				if ei <= 0 {
					continue
				}
				days := (nw.Info().Cost + ei - 1) / ei
				loss = float64(days * lost)
			}
			if gain*horizon < loss {
				continue
			}
			ratio := math.Inf(1)
			if loss > 0 {
				ratio = gain / loss
			}
			if ok && ratio <= bestRatio || col.CanReplace(r, c, nw) != nil {
				continue
			}
			at, it, ok, bestRatio = c, nw, true, ratio
		}
	}
	return at, it, ok
}

// replaceable is a structure replacePick weighs: an output, no MaxPop.
func replaceable(it game.Item) bool {
	in := it.Info()
	return in.MaxPop == 0 && (in.Industry > 0 || in.Research > 0 || in.Prosp > 0)
}

// full reports col's planet has no cell left for any surface item of
// g.Build > 0 research allows (Planet.BestCell: squares only, workers
// aside).
func full(col *game.Colony, r *game.Research, g *Genome) bool {
	tr := col.Traits()
	for it := game.Item(0); it < game.FirstOrbital; it++ {
		if it == game.ColonyBase || g.Build[it] <= 0 || !r.Allows(it) {
			continue
		}
		if _, ok := col.Planet.BestCell(it, tr); ok {
			return false
		}
	}
	return true
}
