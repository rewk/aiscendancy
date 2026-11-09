package game

import (
	"math"
	"math/rand/v2"
	"testing"
)

func positions(s *StarSystem) []Vec3 {
	var out []Vec3
	for _, p := range s.Planets {
		out = append(out, p.Pos)
	}
	for _, l := range s.Lanes {
		out = append(out, l.Pos)
	}
	return out
}

func TestStarSystemDeterministic(t *testing.T) {
	a := positions(NewStarSystem(7, "Sol", 3, 5, 4))
	b := positions(NewStarSystem(7, "Sol", 3, 5, 4))
	c := positions(NewStarSystem(8, "Sol", 3, 5, 4))
	same := true
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("seed 7 twice: body %d at %v then %v", i, a[i], b[i])
		}
		same = same && a[i] == c[i]
	}
	if same {
		t.Error("seeds 7 and 8 give the same system")
	}
}

func radius(v Vec3) float64 { return math.Hypot(v.X, v.Z) }

func TestStarSystemShape(t *testing.T) {
	for seed := uint64(0); seed < 50; seed++ {
		n, m := int(seed%9), 1+int(seed%6)
		s := NewStarSystem(seed, "Vega", StarType(seed%NumStarTypes), n, m)
		if len(s.Planets) != n || len(s.Lanes) != m {
			t.Fatalf("seed %d: %d planets, %d lanes, want %d, %d", seed, len(s.Planets), len(s.Lanes), n, m)
		}
		prev := 0.0
		for k, p := range s.Planets {
			r := radius(p.Pos)
			// Planet k's radius lies in [0.25+0.65k/n, 0.25+0.65(k+1)/n).
			lo, hi := 0.25+0.65*float64(k)/float64(n), 0.25+0.65*float64(k+1)/float64(n)
			if r < lo-1e-9 || r >= hi+1e-9 || r < prev {
				t.Errorf("seed %d planet %d: radius %.3f outside [%.3f,%.3f) or below %.3f", seed, k, r, lo, hi, prev)
			}
			prev = r
			if p.Pos.Y < 0 || p.Pos.Y > 0.8 { // [0, 0.6) rolled, spreadOrbits may lift it
				t.Errorf("seed %d planet %d: height %.3f", seed, k, p.Pos.Y)
			}
			if p.Size < MinSize || p.Size > MaxSize || p.Type >= NumPlanetTypes || len(p.Surface) == 0 {
				t.Errorf("seed %d planet %d: %s size %d, %d squares", seed, k, p.Type, p.Size, len(p.Surface))
			}
		}
		step := 2 * math.Pi / float64(m)
		for k, l := range s.Lanes {
			if r := radius(l.Pos); r < 1-1e-9 || r > 1.15 {
				t.Errorf("seed %d lane %d: radius %.3f", seed, k, r)
			}
			if l.Pos.Y < 0 || l.Pos.Y >= 1.2 || l.To != -1 {
				t.Errorf("seed %d lane %d: height %.3f, to %d", seed, k, l.Pos.Y, l.To)
			}
			// The angle off its slot, 2πk/m past lane 0's, wrapped to (-π,π],
			// is within ±2π/(3m): both lanes carry their own ±π/(3m) jitter.
			a0 := math.Atan2(s.Lanes[0].Pos.Z, s.Lanes[0].Pos.X)
			d := math.Remainder(math.Atan2(l.Pos.Z, l.Pos.X)-a0-float64(k)*step, 2*math.Pi)
			if math.Abs(d) > step/3+1e-9 {
				t.Errorf("seed %d lane %d: %.3f rad off its slot", seed, k, d)
			}
		}
	}
}

func TestStarSystemNames(t *testing.T) {
	s := NewStarSystem(1, "Chippendale", 0, 5, 1)
	for k, want := range []string{"I", "II", "III", "IV", "V"} {
		if got := s.Planets[k].Name; got != "Chippendale "+want {
			t.Errorf("planet %d: %q", k, got)
		}
	}
}

func TestRoman(t *testing.T) {
	want := []string{"I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X",
		"XI", "XII", "XIII", "XIV", "XV", "XVI", "XVII", "XVIII", "XIX", "XX"}
	for i, w := range want {
		if got := roman(i + 1); got != w {
			t.Errorf("roman(%d) = %q, want %q", i+1, got, w)
		}
	}
}

func TestStarSystemPanics(t *testing.T) {
	for _, c := range [][2]int{{-1, 3}, {9, 3}, {3, 0}, {3, 7}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%d planets, %d lanes: no panic", c[0], c[1])
				}
			}()
			NewStarSystem(1, "x", 0, c[0], c[1])
		}()
	}
}

// TestPlanetTypesUniform: every type within 2 sigma of 1/11 over 22000 rolls.
func TestPlanetTypesUniform(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 7))
	var n [NumPlanetTypes]int
	for range 22000 {
		n[rollPlanetType(r, 0)]++
	}
	for typ, c := range n {
		if c < 1900 || c > 2100 {
			t.Errorf("%v: %d rolls of 22000", PlanetType(typ), c)
		}
	}
}

// Colonist: none on a bare system, the owner of the home, the majority
// owner when two empires share a system, the lower index on a tie.
func TestColonist(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	home := g.Empires[0].Colonies[0].Star
	sys := g.Cluster.Stars[home].System
	if sys.Colonist() != 0 {
		t.Errorf("home colonist %d", sys.Colonist())
	}
	bare := g.Cluster.Stars[g.Empires[1].Colonies[0].Star].System
	bare.Planets[0].Owner = -1
	if bare.Colonist() != -1 {
		t.Errorf("bare colonist %d", bare.Colonist())
	}
	if len(sys.Planets) < 3 {
		t.Skip("home system too small for the tie")
	}
	sys.Planets[1].Owner, sys.Planets[2].Owner = 1, 1
	if sys.Colonist() != 1 {
		t.Errorf("majority colonist %d", sys.Colonist())
	}
	sys.Planets[2].Owner = 0
	if sys.Colonist() != 0 {
		t.Errorf("tie colonist %d", sys.Colonist())
	}
}

// Controller: the sole coloniser, none on a bare or a shared system
// (help.txt: "the only colonies there"); Colonist keeps the majority.
func TestController(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	sys := g.Cluster.Stars[g.Empires[0].Colonies[0].Star].System
	if sys.Controller() != 0 {
		t.Errorf("home controller %d", sys.Controller())
	}
	if len(sys.Planets) < 2 {
		t.Skip("home system too small to share")
	}
	sys.Planets[1].Owner = 1
	if sys.Controller() != -1 || sys.Colonist() < 0 {
		t.Errorf("shared: controller %d, colonist %d", sys.Controller(), sys.Colonist())
	}
	for _, p := range sys.Planets {
		p.Owner = -1
	}
	if sys.Controller() != -1 {
		t.Errorf("bare controller %d", sys.Controller())
	}
}
