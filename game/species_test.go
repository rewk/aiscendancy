package game

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestSpeciesMatchGame(t *testing.T) {
	// raceNN.dip: "Diplomacy text for <name> - SA_<ability>." The header's
	// own slips: race02 has no SA_ part, race03 spells KAMBUCHA.
	dipName := map[Species]string{Kambuchka: "kambucha"}
	for s := range NumSpecies {
		in := s.Info()
		l := gameTxt(t, fmt.Sprintf("race%02d.dip", s))[0]
		h, ok := strings.CutPrefix(l, "Diplomacy text for ")
		if !ok {
			t.Fatalf("race%02d.dip: %q", s, l)
		}
		name, sa, hasSA := strings.Cut(strings.TrimSuffix(h, "."), " - ")
		name = strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(name, "the "), "The "))
		want := strings.ToLower(in.Name)
		if n, ok := dipName[s]; ok {
			want = n
		}
		if name != want {
			t.Errorf("race%02d.dip names %q, typed %q", s, name, in.Name)
		}
		if hasSA != (s != Orfa) || hasSA && sa != in.Ability {
			t.Errorf("race%02d.dip ability %q, typed %q", s, sa, in.Ability)
		}
	}

	// help.txt: "// SA_x", "help abilNN", "text", "<Name> Special Ability";
	// "help abresNN" only for the abilities that are used, hence recharge.
	lines := gameTxt(t, "help.txt")
	seen := map[int]bool{}
	active := map[int]bool{}
	for i, l := range lines {
		if n, ok := strings.CutPrefix(l, "help abres"); ok {
			k, err := strconv.Atoi(n)
			if err != nil {
				t.Fatalf("help.txt %q", l)
			}
			active[k] = true
		}
		n, ok := strings.CutPrefix(l, "help abil")
		if !ok {
			continue
		}
		k, err := strconv.Atoi(n)
		if err != nil || k < 0 || k >= int(NumSpecies) || i < 1 || i+2 >= len(lines) {
			t.Fatalf("help.txt %q", l)
		}
		in := Species(k).Info()
		seen[k] = true
		if sa := strings.TrimPrefix(lines[i-1], "// "); sa != in.Ability {
			t.Errorf("abil%02d: %q, typed %q", k, sa, in.Ability)
		}
		if name := strings.TrimSuffix(lines[i+2], " Special Ability"); name != in.Name {
			t.Errorf("abil%02d: %q, typed %q", k, name, in.Name)
		}
	}
	if len(seen) != int(NumSpecies) {
		t.Errorf("help.txt has %d abil sections, want %d", len(seen), NumSpecies)
	}
	for s := range NumSpecies {
		if active[int(s)] != (s.Info().Recharge > 0) {
			t.Errorf("%v: abres section %v, recharge %d", s, active[int(s)], s.Info().Recharge)
		}
	}

	desc := gameTxt(t, "newgame.txt")
	if len(desc) != int(NumSpecies) {
		t.Fatalf("newgame.txt: %d lines, want %d", len(desc), NumSpecies)
	}
	for s := range NumSpecies {
		if desc[s] != s.Info().Description {
			t.Errorf("%v description:\n file %q\n typed %q", s, desc[s], s.Info().Description)
		}
	}
}

func TestSpeciesConstants(t *testing.T) {
	if NumSpecies != 21 {
		t.Fatalf("NumSpecies %d", NumSpecies)
	}
	for s, name := range map[Species]string{Minions: "Minions", Orfa: "Orfa",
		Govorom: "Govorom", Chronomyst: "Chronomyst", Nimbuloids: "Nimbuloids"} {
		if s.String() != name {
			t.Errorf("%d is %q, want %q", s, s, name)
		}
	}
	for s := range NumSpecies {
		in := s.Info()
		if in.HomeSize < MinSize || in.HomeSize > MaxSize || in.HomeType >= NumPlanetTypes {
			t.Errorf("%v: home size %d type %d", s, in.HomeSize, in.HomeType)
		}
	}
}

func TestHomePlanet(t *testing.T) {
	for _, c := range []struct {
		s     Species
		typ   PlanetType
		size  int
		cells int
	}{
		{Snovemdomas, Primordial, 5, 73},
		{Chronomyst, Eden, 3, 23},
	} {
		p := c.s.HomePlanet(7, "Home")
		if p.Type != c.typ || p.Size != c.size || len(p.Surface) != c.cells || len(Layout(c.size)) != c.cells {
			t.Errorf("%v: %v size %d, %d cells", c.s, p.Type, p.Size, len(p.Surface))
		}
		sq := p.Surface[Cell{Centre, Centre}]
		if sq == nil || sq.Kind != White || sq.Item != ColonyBase {
			t.Errorf("%v: centre %+v", c.s, sq)
		}
		q := c.s.HomePlanet(7, "Home")
		for cell, sq := range p.Surface {
			if *q.Surface[cell] != *sq {
				t.Errorf("%v: seed 7 rolls %v differently", c.s, cell)
			}
		}
	}
	// Different seeds give different worlds (a 73-square Primordial roll).
	a, b := Snovemdomas.HomePlanet(1, ""), Snovemdomas.HomePlanet(2, "")
	same := true
	for cell, sq := range a.Surface {
		same = same && *b.Surface[cell] == *sq
	}
	if same {
		t.Error("seeds 1 and 2 roll the same world")
	}
}

// Traits follow the ability texts: the one about black squares builds on
// black, the one "twice as fast" doubles the lane speed, the one about
// "double strength hulls" doubles the hull, the one "see all star lanes"
// knows every lane, the one whose "invasions always succeed" invades
// always, the one to "talk to all races" knows them, the one to "see all
// home stars" knows those, every other species is Base.
func TestSpeciesTraits(t *testing.T) {
	for sp := range NumSpecies {
		txt := strings.ToLower(sp.Info().AbilityText)
		black, fast := strings.Contains(txt, "black"), strings.Contains(txt, "twice as fast")
		double := strings.Contains(txt, "double strength hulls")
		lanes := strings.Contains(txt, "see all star lanes")
		marines := strings.Contains(txt, "invasions always succeed")
		races := strings.Contains(txt, "talk to all races")
		homes := strings.Contains(txt, "see all home stars")
		tr := sp.Traits()
		if tr.KnowsRaces() != races {
			t.Errorf("%v (%q): KnowsRaces %v", sp, txt, tr.KnowsRaces())
		}
		if tr.KnowsHomeStars() != homes {
			t.Errorf("%v (%q): KnowsHomeStars %v", sp, txt, tr.KnowsHomeStars())
		}
		if tr.InvadesAlways() != marines {
			t.Errorf("%v (%q): InvadesAlways %v", sp, txt, tr.InvadesAlways())
		}
		if tr.KnowsStarLanes() != lanes {
			t.Errorf("%v (%q): KnowsStarLanes %v", sp, txt, tr.KnowsStarLanes())
		}
		if tr.BuildsOnBlack() != black {
			t.Errorf("%v (%q): BuildsOnBlack %v", sp, txt, tr.BuildsOnBlack())
		}
		want := 1.0
		if fast {
			want = 2
		}
		if tr.LaneFactor() != want {
			t.Errorf("%v (%q): LaneFactor %v, want %v", sp, txt, tr.LaneFactor(), want)
		}
		hull := 1
		if double {
			hull = 2
		}
		if tr.HullFactor() != hull {
			t.Errorf("%v (%q): HullFactor %d, want %d", sp, txt, tr.HullFactor(), hull)
		}
		if !black && !fast && !double && !lanes && !marines && !races && !homes && tr != Base {
			t.Errorf("%v: traits %T, want Base", sp, tr)
		}
	}
	if !Orfa.Traits().BuildsOnBlack() || Chronomyst.Traits().LaneFactor() != 2 || Snovemdomas.Traits().HullFactor() != 2 ||
		!Minions.Traits().InvadesAlways() || !Hanshaks.Traits().KnowsRaces() || !Kambuchka.Traits().KnowsHomeStars() {
		t.Error("Orfa, Chronomyst, Snovemdomas, Minions, Hanshaks or Kambuchka lost its trait")
	}
}
