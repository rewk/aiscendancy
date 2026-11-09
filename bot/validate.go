package bot

import (
	"fmt"

	"github.com/rewk/aiscendancy/game"
)

// Validate checks a decoded State against its game (save.Read), so Resume
// and the bot's play cannot panic on it: the script's items and the
// research's techs on their tables, Next not negative (Script[next]), the
// Dest stars in the cluster, the days and idle counts −1 (none) or more,
// the Seen stances, Difficulty, the genome's InvadeMargin. The lists' lengths
// need nothing: fromList skips what is past its keys, Resume restarts the
// diplomacy from slices of another length.
func (s *State) Validate(g *game.Game) error {
	cfg := s.Cfg
	for k, it := range cfg.Script {
		if it < 0 || it >= game.NumItems {
			return fmt.Errorf("script %d: item %d", k, it)
		}
	}
	for k, t := range cfg.Research {
		if t >= game.NumTechs {
			return fmt.Errorf("research %d: tech %d", k, t)
		}
	}
	for k, t := range s.Queue {
		if t >= game.NumTechs {
			return fmt.Errorf("queue %d: tech %d", k, t)
		}
	}
	if cfg.Difficulty < 0 || cfg.Difficulty >= NumDifficulties {
		return fmt.Errorf("difficulty %d", cfg.Difficulty)
	}
	if s.Next < 0 {
		return fmt.Errorf("script step %d", s.Next)
	}
	ns := len(g.Cluster.Stars)
	for k, st := range s.Dest {
		if st < -1 || st >= ns {
			return fmt.Errorf("ship %d: destination %d of %d stars", k, st, ns)
		}
	}
	for name, l := range map[string][]int{"founded": s.Founded, "captured": s.Captured, "idle": s.Idle} {
		for k, x := range l {
			if x < -1 {
				return fmt.Errorf("%s %d: %d", name, k, x)
			}
		}
	}
	for j, r := range s.Seen {
		if r.Stance > game.Alliance {
			return fmt.Errorf("seen %d: stance %d", j, r.Stance)
		}
	}
	// A genome is played as saved, out of Clamp's bounds too (the tests'
	// FleeRatio 1e9): only InvadeMargin counts loop steps (invasionDesign
	// walks down from need + it), so only it must stay within them.
	if gn := cfg.Genome; gn != nil {
		c := *gn
		c.Clamp()
		if c.InvadeMargin != gn.InvadeMargin {
			return fmt.Errorf("genome: InvadeMargin %g out of its bounds", gn.InvadeMargin)
		}
	}
	return nil
}
