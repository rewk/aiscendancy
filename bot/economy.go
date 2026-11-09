package bot

import (
	"math"

	"github.com/rewk/aiscendancy/game"
)

// Economy is the structure pick's genes alone: a nil genome plays
// NeutralEconomy, the fixed policy and the GA through one code path.
type Economy struct {
	Upgrade            float64 // a family's upper tier waits for this many of its structures on the planet (rounded)
	IndShare, ResShare float64 // the output mix aimed at (wanted); prosperity takes the rest
	IndFloor           float64 // industry wanted first while the structures make less (wanted): a colony's own build rate
}

// NeutralEconomy is the economy genes a nil genome plays: an upper tier
// once 3 structures of its family stand, industry 0.5 and research 0.3
// of the output, prosperity 0.2 (the best day-900 economy of a grid,
// docs/rules.md "The fixed policy's build and research"), industry first
// below indFloor.
func NeutralEconomy() Economy {
	return Economy{Upgrade: 3, IndShare: 0.5, ResShare: 0.3, IndFloor: indFloor}
}

// indFloor is NeutralEconomy's IndFloor.
const indFloor = 3

// Economy is g's economy genes.
func (g *Genome) Economy() Economy { return Economy{g.Upgrade, g.IndShare, g.ResShare, g.IndFloor} }

func (g *Genome) setEconomy(e Economy) {
	g.Upgrade, g.IndShare, g.ResShare, g.IndFloor = e.Upgrade, e.IndShare, e.ResShare, e.IndFloor
}

// econ is the bot's economy genes: its Genome's, else NeutralEconomy.
func (b *Bot) econ() Economy {
	if g := b.Cfg.Genome; g != nil {
		return g.Economy()
	}
	return NeutralEconomy()
}

// singular is ec past the singularity (r Complete: research no longer
// discovers, Colony.Output turns part of it to industry): ResShare 0,
// prosperity taking the rest. ec as is otherwise, or with r nil.
func (ec Economy) singular(r *game.Research) Economy {
	if r != nil && r.Complete() {
		ec.ResShare = 0
	}
	return ec
}

// wanted is the output col's next structure should make, 0 industry, 1
// research, 2 prosperity, on its outputs without the ongoing project
// (yielded): prosperity while it is negative, or nil with room to grow
// (the upkeep −pop/4 covered first), or when the last free worker goes
// and the next unit's upkeep would bring it to nil (no one left to build
// the Agriplot: the colony would stall); then industry while the
// structures' industry (Planet.Output, before the power curve: a Factory
// 1, a Megafacility 3) is under IndFloor (the colony's own build rate
// first: a Factory speeds all that follows, research goes to the
// empire's pool); else the output furthest under its share of the
// total, target·T − output, ties in that order. The targets are
// IndShare, ResShare and the rest (never below 0) for prosperity; at
// Capacity prosperity grows nothing, so industry and research alone,
// their targets rescaled to a sum of 1, over T = industry + research.
func wanted(col *game.Colony, ec Economy) int {
	y := yielded(col)
	ind, res, pros := y.Output()
	capped := col.Pop >= col.Capacity()
	next := (col.Pop+1)/4 - col.Pop/4 // the next unit's upkeep
	if pros < 0 || !capped && (pros == 0 || y.Free() == 1 && pros-next <= 0) {
		return 2
	}
	if sumInd, _, _ := y.Planet.Output(); float64(sumInd) < ec.IndFloor {
		return 0
	}
	target := [3]float64{ec.IndShare, ec.ResShare, max(0, 1-ec.IndShare-ec.ResShare)}
	out := [3]int{ind, res, pros}
	n := 3
	if capped {
		n = 2
		if s := target[0] + target[1]; s > 0 {
			target[0], target[1] = target[0]/s, target[1]/s
		}
	}
	total := 0
	for k := range n {
		total += out[k]
	}
	best, bestGap := 0, math.Inf(-1)
	for k := range n {
		if gap := target[k]*float64(total) - float64(out[k]); gap > bestGap {
			best, bestGap = k, gap
		}
	}
	return best
}

// wants is the test "in makes the output col wants" (wanted).
func wants(col *game.Colony, ec Economy) func(in *game.ItemInfo) bool {
	k := wanted(col, ec)
	return func(in *game.ItemInfo) bool { return [3]int{in.Industry, in.Research, in.Prosp}[k] > 0 }
}

// baseTier is family f's item of least output, ties the cheaper: the one
// no Upgrade threshold holds back (Factory, Laboratory, Agriplot).
func baseTier(f int) game.Item {
	best := game.NoItem
	for it := game.Item(0); it < game.FirstOrbital; it++ {
		if family(it) != f {
			continue
		}
		if best == game.NoItem || output(it, f) < output(best, f) ||
			output(it, f) == output(best, f) && it.Info().Cost < best.Info().Cost {
			best = it
		}
	}
	return best
}

// upgradeOpen reports it may be built or replaced into on col: any item
// but a family's upper tier, which waits for round(upgrade) structures
// of its family (any tier) on the planet.
func upgradeOpen(col *game.Colony, it game.Item, upgrade float64) bool {
	f := family(it)
	if f < 0 || it == baseTier(f) {
		return true
	}
	n := 0
	for _, sq := range col.Planet.Surface {
		if sq.Item != game.NoItem && family(sq.Item) == f {
			n++
		}
	}
	return n >= int(math.Round(upgrade))
}
