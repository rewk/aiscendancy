package bot

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// Past the singularity the mix aims at ResShare 0, IndShare kept: a
// colony that wanted research at ResShare 1 no longer does. Before it,
// or with no Research, ec as is.
func TestSingularEconomy(t *testing.T) {
	ec := NeutralEconomy()
	if got := ec.singular(game.NewResearch()); got != ec {
		t.Errorf("incomplete: %+v, want %+v", got, ec)
	}
	if got := ec.singular(nil); got != ec {
		t.Errorf("nil: %+v", got)
	}
	want := ec
	want.ResShare = 0
	if got := ec.singular(allKnown()); got != want {
		t.Errorf("complete: %+v, want %+v", got, want)
	}

	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	col := e.Colonies[0]
	col.Pop = col.Busy() + 1
	ind, _, _ := col.Planet.Output()
	res := Economy{Upgrade: 3, ResShare: 1, IndFloor: float64(ind)}
	if k := wanted(col, res.singular(e.Research)); k != 1 {
		t.Fatalf("fixture: wanted %d, want research", k)
	}
	for tc := range game.NumTechs {
		e.Research.Grant(tc)
	}
	if k := wanted(col, res.singular(e.Research)); k == 1 {
		t.Error("complete: research still wanted")
	}
}

// Past the singularity idle never picks the Scientist Takeover: the
// party instead, even with the room under idleParty.
func TestSingularIdle(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	b := New(g.Empires[0], Default())
	col := b.E.Colonies[0]
	col.Pop = col.Capacity() // no room: the takeover before the singularity
	r := b.E.Research
	r.Grant(game.Tech(game.EndlessParty.Info().ResearchReq))
	r.Grant(game.Tech(game.ScientistTakeover.Info().ResearchReq))
	b.idle(col)
	if col.Project == nil || col.Project.Item != game.ScientistTakeover {
		t.Fatalf("fixture: project %+v, want the takeover", col.Project)
	}
	col.Project = nil
	for tc := range game.NumTechs {
		r.Grant(tc)
	}
	b.idle(col)
	if col.Project == nil || col.Project.Item != game.EndlessParty {
		t.Errorf("complete: project %+v, want the party", col.Project)
	}
}
