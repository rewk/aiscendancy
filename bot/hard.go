package bot

import (
	_ "embed"
	"encoding/json"
)

// hardJSON is Hard's genome: fuzz -mode evolve -fresh -species orfa
// -seeds 1-6 -pop 24 -gens 8 (genomes/orfa.json), embedded.
// Over every species, seeds 1-4 solo, it wins all, median day 1833
// (Hard's inFlight floor alone 2324); as empire 0 of 4 against the
// default it holds more systems in 62 games of 63.
//
//go:embed hard.json
var hardJSON []byte

// HardGenome is a fresh copy of Hard's genome, read as ReadGenome does.
func HardGenome() *Genome {
	g := Neutral()
	g.HabitatShare, g.HabitatFill = 0, 0
	if err := json.Unmarshal(hardJSON, g); err != nil {
		panic("bot: hard.json: " + err.Error()) // embedded: a test reads it
	}
	g.Clamp()
	return g
}

// AtLevel is cfg at difficulty d: Hard plays HardGenome unless cfg
// already has a genome (the window's -genome wins).
func AtLevel(cfg Config, d Difficulty) Config {
	cfg.Difficulty = d
	if d == Hard && cfg.Genome == nil {
		cfg.Genome = HardGenome()
	}
	return cfg
}
