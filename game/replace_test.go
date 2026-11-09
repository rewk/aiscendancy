package game

import (
	"bytes"
	"maps"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/journal"
)

// replaceFixture is empire 0's home on seed 1 with a Factory at f, the
// Industrial Megafacility's tech, every worker busy (Free 0), a journal.
func replaceFixture(t *testing.T) (r *Research, home *Colony, f Cell, buf *bytes.Buffer) {
	t.Helper()
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	buf = &bytes.Buffer{}
	g.SetJournal(journal.New(buf))
	e := g.Empires[0]
	r = e.Research
	r.Grant(Tech(IndustrialMegafacility.Info().ResearchReq))
	home = e.Colonies[0]
	f, ok := home.Planet.BestCell(Factory, Base)
	if !ok || home.Planet.Build(f, Factory) != nil {
		t.Fatal("no square for a Factory")
	}
	home.Pop = home.Busy()
	if home.Free() != 0 {
		t.Fatalf("free %d", home.Free())
	}
	return r, home, f, buf
}

// squares is a copy of the planet's squares, by value.
func squares(p *Planet) map[Cell]Square {
	out := map[Cell]Square{}
	for c, sq := range p.Surface {
		out[c] = *sq
	}
	return out
}

// OutputWith reads the colony's Output with one square swapped and puts
// it back: the same numbers as the real swap.
func TestOutputWith(t *testing.T) {
	_, home, f, _ := replaceFixture(t)
	p := home.Planet
	before := squares(p)
	i0, r0, p0 := home.Output()
	wi, wr, wp := home.OutputWith(f, IndustrialMegafacility)
	ei, er, ep := home.OutputWith(f, NoItem)
	if !maps.Equal(squares(p), before) {
		t.Fatal("OutputWith changed the planet")
	}
	if i, r, pr := home.Output(); i != i0 || r != r0 || pr != p0 {
		t.Fatal("Output moved")
	}
	p.Demolish(f)
	if i, r, pr := home.Output(); i != ei || r != er || pr != ep {
		t.Errorf("empty: OutputWith %d %d %d, real %d %d %d", ei, er, ep, i, r, pr)
	}
	if err := p.Build(f, IndustrialMegafacility); err != nil {
		t.Fatal(err)
	}
	if i, r, pr := home.Output(); i != wi || r != wr || pr != wp || wi <= i0 {
		t.Errorf("megafacility: OutputWith %d %d %d, real %d %d %d (before %d)", wi, wr, wp, i, r, pr, i0)
	}
}

// Replace on a colony with Free 0: the Factory's worker goes to the
// Megafacility, started at the Factory's cell; journalled as a demolish
// then a start.
func TestReplace(t *testing.T) {
	r, home, f, buf := replaceFixture(t)
	if err := home.Replace(r, f, IndustrialMegafacility); err != nil {
		t.Fatal(err)
	}
	if sq := home.Planet.Surface[f]; sq.Item != NoItem {
		t.Errorf("square holds %s", sq.Item)
	}
	if pj := home.Project; pj == nil || pj.Item != IndustrialMegafacility || pj.Cell != f || pj.Orbital {
		t.Fatalf("project %+v", pj)
	}
	ens := drain(t, buf)
	if len(ens) != 2 || ens[0].Act != "demolish" || ens[0].Item != "Factory" || ens[0].Cell != f.String() ||
		ens[1].Act != "start" || ens[1].Item != "Industrial Megafacility" || ens[1].Cell != f.String() {
		t.Errorf("journal %+v", ens)
	}
}

// An ongoing project gives way to a replacement as to any start.
func TestReplaceOverOngoing(t *testing.T) {
	r, home, f, _ := replaceFixture(t)
	home.Project = &Project{Item: EndlessParty, Slot: -1}
	if err := home.Replace(r, f, IndustrialMegafacility); err != nil || home.Project.Item != IndustrialMegafacility {
		t.Errorf("%v, project %s", err, home.Project.Item)
	}
}

// Each refusal leaves the planet, the project and the journal as they
// were.
func TestReplaceRefusals(t *testing.T) {
	cases := []struct {
		name  string
		setup func(r *Research, home *Colony, f Cell) (Cell, Item)
		want  string
	}{
		{"colony base", func(_ *Research, _ *Colony, _ Cell) (Cell, Item) {
			return Cell{Centre, Centre}, IndustrialMegafacility
		}, "Colony Base"},
		{"empty square", func(_ *Research, home *Colony, _ Cell) (Cell, Item) {
			at, _ := home.Planet.BestCell(Agriplot, Base)
			return at, IndustrialMegafacility
		}, "nothing to abandon"},
		{"same item", func(_ *Research, _ *Colony, f Cell) (Cell, Item) { return f, Factory }, "already"},
		{"project running", func(r *Research, home *Colony, f Cell) (Cell, Item) {
			home.Pop++
			at, _ := home.Planet.BestCell(Agriplot, Base)
			if err := home.Start(r, Agriplot, at); err != nil {
				t.Fatal(err)
			}
			return f, IndustrialMegafacility
		}, "project is running"},
		{"orbital", func(_ *Research, _ *Colony, f Cell) (Cell, Item) { return f, Shipyard }, "not a surface"},
		{"project item", func(_ *Research, _ *Colony, f Cell) (Cell, Item) { return f, Terraforming }, "not a surface"},
		{"colony base item", func(_ *Research, _ *Colony, f Cell) (Cell, Item) { return f, ColonyBase }, "Colony Base"},
		{"research", func(_ *Research, _ *Colony, f Cell) (Cell, Item) { return f, ResearchCampus }, "needs"},
		{"automated: no worker freed", func(_ *Research, home *Colony, f Cell) (Cell, Item) {
			if err := home.Planet.Automate(f, Factory); err != nil {
				t.Fatal(err)
			}
			home.Pop = home.Busy()
			return f, IndustrialMegafacility
		}, "no free population"},
		{"black square", func(_ *Research, home *Colony, _ Cell) (Cell, Item) {
			p := home.Planet
			for _, c := range p.Cells() {
				if p.Surface[c].Kind == Black && p.CanBuild(c, TransportTubes, Base) {
					if err := p.Build(c, TransportTubes); err != nil {
						t.Fatal(err)
					}
					home.Pop++ // tubes employ nobody: a worker for the Megafacility
					return c, IndustrialMegafacility
				}
			}
			t.Fatal("no black square for tubes")
			return Cell{}, NoItem
		}, "is black"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, home, f, buf := replaceFixture(t)
			at, it := tc.setup(r, home, f)
			buf.Reset()
			before, pj, pop := squares(home.Planet), home.Project, home.Pop
			err := home.Replace(r, at, it)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%v, want %q", err, tc.want)
			}
			if cerr := home.CanReplace(r, at, it); cerr == nil || cerr.Error() != err.Error() {
				t.Errorf("CanReplace %v, Replace %v", cerr, err)
			}
			if !maps.Equal(squares(home.Planet), before) || home.Project != pj || home.Pop != pop {
				t.Error("the refusal changed the colony")
			}
			none(t, buf, tc.name)
		})
	}
}
