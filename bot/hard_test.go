package bot

import (
	"encoding/json"
	"testing"
)

// Hard's genome decodes, every gene of the file read (none left at
// Neutral's by a renamed key), and each call is a fresh copy; AtLevel
// gives it to Hard alone, a genome already set winning.
func TestHardGenome(t *testing.T) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(hardJSON, &keys); err != nil {
		t.Fatal(err)
	}
	g := HardGenome()
	b, _ := json.Marshal(g)
	var all map[string]json.RawMessage
	json.Unmarshal(b, &all)
	for k := range all {
		if _, ok := keys[k]; !ok {
			t.Errorf("gene %s not in hard.json", k)
		}
	}
	if h := HardGenome(); h == g {
		t.Error("HardGenome shares its genome")
	}
	if c := AtLevel(Default(), Original); c.Genome != nil || c.Difficulty != Original {
		t.Errorf("Original: %+v", c)
	}
	if c := AtLevel(Default(), Hard); c.Genome == nil || c.Difficulty != Hard || c.Genome.InFlight != g.InFlight {
		t.Errorf("Hard: %+v", c)
	}
	own := Neutral()
	cfg := Default()
	cfg.Genome = own
	if c := AtLevel(cfg, Hard); c.Genome != own {
		t.Error("Hard replaced a given genome")
	}
}
