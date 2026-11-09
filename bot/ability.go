package bot

import (
	"math"

	"github.com/rewk/aiscendancy/game"
)

// The bot's timed ability (docs/rules.md "The bot's abilities"): one
// predicate per species decides from what the fog shows me, never another
// empire's Ships or Research.

// ability uses my species' timed ability when ready and wantAbility
// holds. Without G, nothing.
func (b *Bot) ability() {
	if b.G == nil {
		return
	}
	me := b.E.Index
	if ready, _ := b.G.AbilityState(me); !ready || !b.wantAbility() {
		return
	}
	b.say(b.G.UseAbility(me))
}

// wantAbility is my species' use rule on a ready ability (Baliflids force
// peace, Marmosians defame, Fludentri repair, Swaparamans double power,
// Shevar drain, Dubtaks steal, Chamachies finish the topic, Nimbuloids a
// day's industry, Mebes capacity, Capelons invincibility, Ungooma bump,
// Frutmaka warp, Arbryls block, Govorom cornucopia). Dubtaks and Mebes
// always try (the game refuses "nothing to steal"); the Govorom home
// alone waits for a young colony. Passive species: false.
func (b *Bot) wantAbility() bool {
	switch b.E.Species {
	case game.Baliflids:
		return b.peaceWanted()
	case game.Marmosians:
		return b.atWar()
	case game.Fludentri:
		return b.damaged()
	case game.Swaparamans:
		return b.armedAtFight()
	case game.Shevar:
		return b.hostileWherePresent()
	case game.Dubtaks, game.Mebes:
		return true
	case game.Chamachies:
		return b.longTopic()
	case game.Nimbuloids:
		return b.projectRunning()
	case game.Capelons, game.Frutmaka:
		return b.hostileAtColony()
	case game.Ungooma:
		return !b.mineInLane() && b.trafficAtColony()
	case game.Arbryls:
		return b.atWar() && b.trafficAtColony()
	case game.Govorom:
		return len(b.E.Colonies) >= 2
	}
	return false
}

// peaceWanted: some empire at War with me has my attitude at AcceptPeace
// or more.
func (b *Bot) peaceWanted() bool {
	th := b.Thresholds()
	for j := range b.E.Relations {
		if j == b.E.Index || b.E.StanceWith(j) != game.War {
			continue
		}
		if att, ok := b.Attitude(j); ok && att >= th.AcceptPeace {
			return true
		}
	}
	return false
}

// atWar: my stance is War toward some empire.
func (b *Bot) atWar() bool { return b.stanced(game.War) }

// stanced: my stance is st toward some other empire.
func (b *Bot) stanced(st game.Stance) bool {
	for j := range b.E.Relations {
		if j != b.E.Index && b.E.StanceWith(j) == st {
			return true
		}
	}
	return false
}

// damaged: some ship of mine, armed or not, has Hull < RepairAt × MaxHull.
func (b *Bot) damaged() bool {
	for _, s := range b.E.Ships {
		if float64(s.Hull) < b.cg.RepairAt*float64(s.MaxHull()) {
			return true
		}
	}
	return false
}

// armedAtFight: an armed ship of mine out of the lanes at a combat star.
func (b *Bot) armedAtFight() bool {
	fight := b.combatStars()
	for _, s := range b.E.Ships {
		if s.Lane < 0 && fight[s.Star] && armed(s.Design) {
			return true
		}
	}
	return false
}

// hostileWherePresent: Hostiles seen at a star where I am Present (a
// colony, or a ship of mine out of the lanes).
func (b *Bot) hostileWherePresent() bool {
	for _, s := range b.E.Ships {
		if s.Lane < 0 && len(b.G.HostilesAll(b.E, s.Star)) > 0 {
			return true
		}
	}
	return b.hostileAtColony()
}

// hostileAtColony: Hostiles seen at a star holding a colony of mine.
func (b *Bot) hostileAtColony() bool {
	for _, c := range b.E.Colonies {
		if len(b.G.HostilesAll(b.E, c.Star)) > 0 {
			return true
		}
	}
	return false
}

// longTopic: a topic under study whose days left at my research rate
// reach round(FreeResearchDays), or never come.
func (b *Bot) longTopic() bool {
	r := b.E.Research
	if r.Topic == game.NoTech {
		return false
	}
	rate := 0
	for _, c := range b.E.Colonies {
		_, res, _ := c.Output()
		rate += res
	}
	d := r.Days(r.Topic, rate)
	return d < 0 || d >= int(math.Round(b.dipGenes().FreeResearchDays))
}

// projectRunning: some colony of mine runs a project not Ongoing.
func (b *Bot) projectRunning() bool {
	for _, c := range b.E.Colonies {
		if pj := c.Project; pj != nil && !pj.Item.Ongoing() {
			return true
		}
	}
	return false
}

// mineInLane: a ship of mine travels a lane.
func (b *Bot) mineInLane() bool {
	for _, s := range b.E.Ships {
		if s.Lane >= 0 {
			return true
		}
	}
	return false
}

// trafficAtColony: LaneTraffic on some lane ending at a star holding a
// colony of mine.
func (b *Bot) trafficAtColony() bool {
	c := b.E.Cluster
	for _, col := range b.E.Colonies {
		for _, li := range c.Stars[col.Star].Lanes {
			if b.G.LaneTraffic(b.E, li) > 0 {
				return true
			}
		}
	}
	return false
}
