package bot

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// ongoingBot is a fresh home with nothing to build (no script, not
// greedy, nothing researched but the techs of the ongoing projects in
// grant), its bot playing g.
func ongoingBot(t *testing.T, g *Genome, grant ...game.Item) (*Bot, *game.Colony) {
	t.Helper()
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	for _, it := range grant {
		e.Research.Grant(game.Tech(it.Info().ResearchReq))
	}
	return New(e, Config{Genome: g}), e.Colonies[0]
}

// A colony that starts nothing runs an Endless Party while it has room
// past IdleParty, else a Scientist Takeover; the only one researched
// when one is; neither: idle. Full, it switches.
func TestIdleOngoing(t *testing.T) {
	both := []game.Item{game.EndlessParty, game.ScientistTakeover}
	item := func(c *game.Colony) game.Item {
		if c.Project == nil {
			return game.NoItem
		}
		return c.Project.Item
	}
	b, col := ongoingBot(t, nil, both...)
	if col.Pop >= col.Capacity() {
		t.Fatalf("fixture full: pop %d of %d", col.Pop, col.Capacity())
	}
	if b.planColony(0, col, b.claimed()); item(col) != game.EndlessParty {
		t.Errorf("room left: %s, want Endless Party", item(col))
	}
	pj := col.Project
	if b.planColony(0, col, b.claimed()); col.Project != pj {
		t.Error("the right one running was replaced")
	}
	col.Pop = col.Capacity()
	if b.planColony(0, col, b.claimed()); item(col) != game.ScientistTakeover {
		t.Errorf("full: %s, want Scientist Takeover", item(col))
	}

	// IdleParty: a party only while the room left is above it.
	n := Neutral()
	b, col = ongoingBot(t, n, both...)
	room := float64(col.Capacity()-col.Pop) / float64(col.Capacity())
	for _, tc := range []struct {
		gene float64
		want game.Item
	}{{room - 0.01, game.EndlessParty}, {room, game.ScientistTakeover}, {0, game.EndlessParty}} {
		n.IdleParty = tc.gene
		if b.planColony(0, col, b.claimed()); item(col) != tc.want {
			t.Errorf("IdleParty %v, room %v: %s, want %s", tc.gene, room, item(col), tc.want)
		}
	}

	for _, tc := range []struct {
		grant []game.Item
		full  bool
		want  game.Item
	}{
		{[]game.Item{game.ScientistTakeover}, false, game.ScientistTakeover},
		{[]game.Item{game.EndlessParty}, true, game.EndlessParty},
		{nil, false, game.NoItem},
		{nil, true, game.NoItem},
	} {
		b, col := ongoingBot(t, nil, tc.grant...)
		if tc.full {
			col.Pop = col.Capacity()
		}
		if b.planColony(0, col, b.claimed()); item(col) != tc.want {
			t.Errorf("researched %v, full %v: %s, want %s", tc.grant, tc.full, item(col), tc.want)
		}
	}
}

// A colony on an ongoing project counts as idle in Act: the greedy pick,
// weighed on the colony without it (its industry back), replaces it.
func TestOngoingGivesWayToBuild(t *testing.T) {
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	e.Research.Grant(game.Tech(game.EndlessParty.Info().ResearchReq))
	col := e.Colonies[0]
	col.Pop = col.Busy() + 2
	b := New(e, Config{Greedy: true, NoResearch: true})
	want := GreedyPick(col, e.Research, NeutralEconomy())
	if want == game.NoItem {
		t.Fatal("nothing to pick")
	}
	if err := col.StartProject(e.Research, game.EndlessParty); err != nil {
		t.Fatal(err)
	}
	b.Act()
	if col.Project == nil || col.Project.Item != want {
		t.Errorf("project %v, want %s", col.Project, want)
	}
}

// IdleParty: 0 for Neutral and a nil genome, clamped to [0, 1].
func TestIdlePartyGene(t *testing.T) {
	if Neutral().IdleParty != 0 || (&Bot{}).idleParty() != 0 {
		t.Errorf("neutral %v, nil genome %v", Neutral().IdleParty, (&Bot{}).idleParty())
	}
	for _, c := range [][2]float64{{-1, 0}, {0.3, 0.3}, {2, 1}} {
		g := Neutral()
		g.IdleParty = c[0]
		if g.Clamp(); g.IdleParty != c[1] {
			t.Errorf("Clamp(%v) = %v, want %v", c[0], g.IdleParty, c[1])
		}
	}
}
