package main

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/journal"
	"github.com/rewk/aiscendancy/ui"
)

func resEmpire() *game.Empire { return game.NewEmpire(1, game.Snovemdomas, game.Average) }

// A click studies an available tech only: a locked or known one leaves the
// topic.
func TestPick(t *testing.T) {
	e := resEmpire()
	r := e.Research
	av := r.Available()
	if len(av) == 0 || r.Topic != game.NoTech {
		t.Fatalf("fresh empire: available %v, topic %v", av, r.Topic)
	}
	locked := game.Tech(game.NumTechs - 1)
	if r.CanStudy(locked) {
		t.Fatal("the last tech is studiable at start")
	}
	if err := pick(e, locked); err != nil || r.Topic != game.NoTech {
		t.Errorf("locked click: err %v, topic %v", err, r.Topic)
	}
	if err := pick(e, av[0]); err != nil || r.Topic != av[0] {
		t.Errorf("available click: err %v, topic %v, want %v", err, r.Topic, av[0])
	}
	if err := pick(e, locked); err != nil || r.Topic != av[0] {
		t.Errorf("locked click moved the topic to %v", r.Topic)
	}
	r.Grant(av[1])
	if err := pick(e, av[1]); err != nil || r.Topic != av[0] {
		t.Errorf("known click moved the topic to %v", r.Topic)
	}
}

// A pick goes through Empire.Study: journalled once on the window's
// journal, which setGame installs on every game it sets (New Game, -load,
// F9); a refused pick journals nothing.
func TestPickJournalled(t *testing.T) {
	var buf bytes.Buffer
	a := &app{journal: journal.New(&buf)}
	a.setGame(game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average), 0)
	av := a.e.Research.Available()
	if err := pick(a.e, game.Tech(game.NumTechs-1)); err != nil || buf.Len() != 0 {
		t.Fatalf("locked pick: %v, journal %q", err, buf.String())
	}
	if err := pick(a.e, av[0]); err != nil || strings.Count(buf.String(), "\n") != 1 ||
		!strings.Contains(buf.String(), `"act":"study","tech":"`+av[0].String()+`"`) {
		t.Errorf("pick: %v, journal %q", err, buf.String())
	}
}

// detail lists the prerequisites with the known mark and what the tech
// unlocks, from the tables; granting a prerequisite marks it, and granting
// all of them makes the tech available.
func TestDetail(t *testing.T) {
	r := resEmpire().Research
	tech := game.SpacetimeSurfing // two prerequisites
	reqs := game.Techs[tech].Requires
	if len(reqs) != 2 {
		t.Fatalf("%v requires %v", tech, reqs)
	}
	// The name, the days: no points, no prerequisites (the tree shows them).
	if d := detail(r, tech, 0); !slices.Equal(d, []string{tech.String(), "never: no research"}) {
		t.Errorf("detail %q", d)
	}
	r.Grant(reqs[0])
	if r.CanStudy(tech) {
		t.Error("available with one prerequisite")
	}
	r.Grant(reqs[1])
	if !r.CanStudy(tech) {
		t.Error("not available with both prerequisites")
	}
	// Unlocks, from the model: the items' planitem frames (the Ship item
	// replaced by its hulls, the species' smship frames), then the
	// parts' gizmos frames.
	sp := game.Snovemdomas
	for u := game.Tech(0); u < game.NumTechs; u++ {
		items, gizmos := r.Unlocks(u)
		var want []unlock
		for _, it := range items {
			if it != game.Ship {
				want = append(want, unlock{it.String(), "planitem", int(it)})
			}
		}
		for h := range game.NumHulls {
			if h.Tech() == u {
				want = append(want, unlock{h.String() + " hull", shipBank(sp), int(h)})
			}
		}
		for _, g := range gizmos {
			want = append(want, unlock{g.String(), gizmoBank, int(g)})
		}
		if got := unlocks(r, sp, u); !slices.Equal(got, want) {
			t.Errorf("%v: unlocks %v, want %v", u, got, want)
		}
	}
	// Interplanetary Exploration: the Small and Medium hulls, the species'
	// ships, not the Ship item's frame (it is the
	// Orbital Missile Base's picture).
	ie := unlocks(r, sp, game.InterplanetaryExploration)
	if len(ie) < 2 || ie[0] != (unlock{"Small hull", shipBank(sp), int(game.Small)}) || ie[1] != (unlock{"Medium hull", shipBank(sp), int(game.Medium)}) || slices.ContainsFunc(ie, func(u unlock) bool { return u.bank == "planitem" && u.frame == int(game.Ship) }) {
		t.Errorf("Interplanetary Exploration unlocks %v", ie)
	}
	if err := r.Study(tech); err != nil {
		t.Fatal(err)
	}
	if d := detail(r, tech, 3); !slices.Equal(d, []string{tech.String(), fmt.Sprintf("%d days remaining", (r.Cost(tech)+2)/3)}) {
		t.Errorf("topic detail %q", d)
	}
	if d := detail(r, tech, r.Cost(tech)); d[1] != "1 day remaining" {
		t.Errorf("a day left: %q", d)
	}
	r.Grant(tech)
	if d := detail(r, tech, 3); !slices.Equal(d, []string{tech.String(), "Researched"}) {
		t.Errorf("known detail %q", d)
	}
}

// Every tech's detail and all its unlock rows, icons at zoom, fit the
// right box above the error's two lines, each name within the box.
func TestDetailFits(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	graftHabitat(as)
	a := testApp(game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average))
	a.a = as
	r := a.e.Research
	for u := game.Tech(0); u < game.NumTechs; u++ {
		top := unlockTop(len(detail(r, u, 1)) - 1)
		rows := unlockRows(a, u, top, unlockBottom())
		if n := len(unlocks(r, a.e.Species, u)); len(rows) != n {
			t.Errorf("%v: %d of %d unlock rows fit (from y %d to %d)", u, len(rows), n, top, unlockBottom())
		}
		for i, row := range rows {
			if row.w > unlockIconW || i > 0 && row.y < rows[i-1].y+rows[i-1].h {
				t.Errorf("%v row %d: %+v", u, i, row)
			}
			for _, l := range row.names {
				if unlockNameX()+ui.Width(l, row.face) > float64(resBox.X1-padX) {
					t.Errorf("%v row %d: %q past the box", u, i, l)
				}
			}
		}
	}
}
