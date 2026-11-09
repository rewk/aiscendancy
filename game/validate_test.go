package game_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

// played runs n days of g with bots (bot.Day: the rounds included),
// Validate after each; fails at the first refusal.
func played(t *testing.T, g *game.Game, bots []*bot.Bot, n int) {
	t.Helper()
	if err := g.Validate(); err != nil {
		t.Fatalf("day %d: %v", g.Day, err)
	}
	for range n {
		bot.Day(g, bots)
		if err := g.Validate(); err != nil {
			t.Fatalf("day %d: %v", g.Day, err)
		}
	}
}

// Validate accepts what the engine makes, after every day: bot-played
// games (four empires in a hostile cluster to day 1000: colonies, wars,
// ships destroyed, planets invaded; a Hard genome game), the fixtures and the arena, every staged
// scenario, each then played on.
func TestValidatePlayed(t *testing.T) {
	sps := []game.Species{game.Snovemdomas, game.Orfa, game.Mebes, game.Capelons}
	g := game.NewGame(6, sps, game.Average)
	g.Atmosphere = game.Hostile
	played(t, g, bot.NewAll(g, bot.Default()), 1000)
	wars, sunk, invaded, ships := 0, 0, 0, 0
	for _, e := range g.Empires {
		ships += len(e.Ships)
		for _, r := range e.Relations {
			wars += r.WarDays
			sunk += r.ShipsLost
			invaded += r.PlanetsLost
		}
	}
	if wars == 0 || sunk == 0 || invaded == 0 || ships == 0 {
		t.Errorf("day 1000: %d war days, %d ships lost, %d planets invaded, %d ships: the fixture walked no fight", wars, sunk, invaded, ships)
	}

	g = game.NewGame(3, game.PickSpecies(3, game.Orfa, 3), game.Sparse)
	cfg := bot.Default()
	cfg.Genome, cfg.Difficulty = bot.Neutral(), bot.Hard
	played(t, g, bot.NewAll(g, cfg), 400)

	g = game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	if err := game.SpawnFixtures(g, 2, 2, 3); err != nil {
		t.Fatal(err)
	}
	played(t, g, bot.NewAll(g, bot.Default()), 30)

	a, b, err := game.ParseArena("lancer,cruiser:raider,battleship")
	if err != nil {
		t.Fatal(err)
	}
	g = game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	if err := game.SpawnArena(g, a, b); err != nil {
		t.Fatal(err)
	}
	played(t, g, bot.NewAll(g, bot.Default()), 30)

	files, _ := filepath.Glob("../scenarios/*.json")
	if len(files) == 0 {
		t.Fatal("no scenario in ../scenarios")
	}
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		s, err := game.ParseScenario(fh)
		fh.Close()
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
		if err := game.ApplyScenario(g, s); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		played(t, g, bot.NewAll(g, bot.Default()), 20)
	}
}

// tamperGame is a small valid game holding one of everything Validate
// walks: a project and a queue entry on empire 0's home, a template, an
// offer, rounds today, fixture ships of both empires (empire 0's colony
// ship in orbit, warships in space).
func tamperGame(t *testing.T) *game.Game {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	if err := game.SpawnFixtures(g, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	col := g.Empires[0].Colonies[0]
	var free []game.Cell
	for _, c := range col.Planet.Cells() {
		if col.Planet.Surface[c].Item == game.NoItem {
			free = append(free, c)
		}
	}
	col.Project = &game.Project{Item: game.Factory, Cell: free[0]}
	col.Queue = []game.Queued{{Item: game.Agriplot, Cell: free[1]}, {Item: game.Shipyard, Slot: 0, Orbital: true}}
	g.Empires[0].Templates = []game.Template{{Name: "T", Hull: game.Small, Parts: []game.Part{{Best: true, Cat: game.Weapon}, {Gizmo: game.Colonizer}}}}
	g.Offers = []game.Offer{{From: 1, To: 0, Kind: game.PeaceOffer, Third: -1}}
	g.RoundsToday = make([]int, len(g.Cluster.Stars))
	if err := g.Validate(); err != nil {
		t.Fatalf("the fixture: %v", err)
	}
	return g
}

// Each tampering of a saved field Validate guards is refused, its message
// naming the place: one case per family (cluster, star, lane, planet,
// square, orbit, empire, research, relation, template, colony, project,
// queue, ship, design, order, game, offer).
func TestValidateTampered(t *testing.T) {
	home := func(g *game.Game) *game.Colony { return g.Empires[0].Colonies[0] }
	hp := func(g *game.Game) *game.Planet {
		c := home(g)
		return g.Cluster.Stars[c.Star].System.Planets[c.PlanetIdx]
	}
	ship := func(g *game.Game, k int) *game.Starship { return g.Empires[0].Ships[k] }
	for _, tc := range []struct {
		name, want string
		f          func(g *game.Game)
	}{
		{"density", "density", func(g *game.Game) { g.Cluster.Density = game.NumDensities }},
		{"home star", "home star", func(g *game.Game) { g.Cluster.Home = len(g.Cluster.Stars) }},
		{"lane end", "lane 0: stars", func(g *game.Game) { g.Cluster.Lanes[0].B = len(g.Cluster.Stars) }},
		{"lane loop", "lane 0: stars", func(g *game.Game) { g.Cluster.Lanes[0].B = g.Cluster.Lanes[0].A }},
		{"star type", "star 3: type", func(g *game.Game) { g.Cluster.Stars[3].Type = game.NumStarTypes }},
		{"no system", "star 2: no system", func(g *game.Game) { g.Cluster.Stars[2].System = nil }},
		{"star lane index", "star 0: lane 0: index", func(g *game.Game) { g.Cluster.Stars[0].Lanes[0] = len(g.Cluster.Lanes) }},
		{"star lanes vs endpoints", "lanes,", func(g *game.Game) {
			s := &g.Cluster.Stars[0]
			s.Lanes = s.Lanes[:len(s.Lanes)-1]
		}},
		{"lane not touching", "does not touch it", func(g *game.Game) {
			c := g.Cluster
			for li, l := range c.Lanes {
				if l.A != 0 && l.B != 0 {
					c.Stars[0].Lanes[0] = li
					return
				}
			}
		}},
		{"endpoint to", "endpoint 0: to", func(g *game.Game) { g.Cluster.Stars[0].System.Lanes[0].To = -1 }},
		{"too many planets", "planets, at most", func(g *game.Game) {
			sys := g.Cluster.Stars[0].System
			for len(sys.Planets) <= game.MaxSystemPlanets {
				sys.Planets = append(sys.Planets, game.NewPlanet(1, "x", game.Husk, 1))
			}
		}},
		{"nil planet", "planet 0: missing", func(g *game.Game) { g.Cluster.Stars[0].System.Planets[0] = nil }},
		{"position", "position", func(g *game.Game) { g.Cluster.Stars[0].Pos.X = 1e300 }},
		{"planet type", "type", func(g *game.Game) { hp(g).Type = game.NumPlanetTypes }},
		{"planet size", "size 6", func(g *game.Game) { hp(g).Size = 6 }},
		{"planet size vs squares", "squares, size", func(g *game.Game) { hp(g).Size-- }},
		{"square missing", "missing", func(g *game.Game) {
			p := hp(g)
			c := p.Cells()[0]
			delete(p.Surface, c)
			p.Surface[game.Cell{I: 99, J: 99}] = &game.Square{Item: game.NoItem}
		}},
		{"square nil", "missing", func(g *game.Game) { hp(g).Surface[hp(g).Cells()[0]] = nil }},
		{"square kind", "kind", func(g *game.Game) { hp(g).Surface[hp(g).Cells()[0]].Kind = game.NumSquareKinds }},
		{"square item", "not a surface structure", func(g *game.Game) { hp(g).Surface[hp(g).Cells()[0]].Item = game.Shipyard }},
		{"orbit item", "not orbital", func(g *game.Game) { hp(g).Orbit[3] = game.Factory }},
		{"orbit damage", "damage -1", func(g *game.Game) { hp(g).OrbitDamage[3] = -1 }},
		{"owner range", "owner 2 out of -1..1", func(g *game.Game) { hp(g).Owner = 2 }},
		{"owner without colony", "owner 1 has no colony", func(g *game.Game) { g.Cluster.Stars[0].System.Planets[0].Owner = 1 }},
		{"empire nil", "empire 1: missing", func(g *game.Game) { g.Empires[1] = nil }},
		{"empire index", "empire 1: index", func(g *game.Game) { g.Empires[1].Index = 0 }},
		{"species", "species", func(g *game.Game) { g.Empires[1].Species = game.NumSpecies }},
		{"colour", "colour", func(g *game.Game) { g.Empires[1].Colour = game.NumColours }},
		{"no research", "no research", func(g *game.Game) { g.Empires[1].Research = nil }},
		{"visited", "stars visited", func(g *game.Game) { g.Empires[0].Visited = make([]bool, len(g.Cluster.Stars)+1) }},
		{"lanes known", "lanes known", func(g *game.Game) { g.Empires[0].LanesKnown = make([]bool, len(g.Cluster.Lanes)+1) }},
		{"relations", "relations for", func(g *game.Game) { g.Empires[0].Relations = g.Empires[0].Relations[:1] }},
		{"victory", "victory", func(g *game.Game) { g.Empires[0].Won = game.UniteVictory + 1 }},
		{"lost", "lost -1", func(g *game.Game) { g.Empires[0].Lost = -1 }},
		{"lane block", "blocked lane", func(g *game.Game) { g.Empires[0].LaneBlocks = map[int]int{len(g.Cluster.Lanes): 9} }},
		{"topic", "research: topic", func(g *game.Game) { g.Empires[0].Research.Topic = game.NumTechs }},
		{"progress", "progress -1", func(g *game.Game) { g.Empires[0].Research.Progress[4] = -1 }},
		{"stance", "relation 1: stance", func(g *game.Game) { g.Empires[0].Relations[1].Stance = game.Alliance + 1 }},
		{"met", "relation 1: met", func(g *game.Game) { g.Empires[0].Relations[1].Met = -2 }},
		{"ledger", "negative count", func(g *game.Game) { g.Empires[0].Relations[1].ShipsLost = -1 }},
		{"template hull", "template 0: hull", func(g *game.Game) { g.Empires[0].Templates[0].Hull = game.NumHulls }},
		{"template part", "template 0: part 1", func(g *game.Game) { g.Empires[0].Templates[0].Parts[1].Gizmo = game.NumGizmos }},
		{"template best", "template 0: part 0", func(g *game.Game) { g.Empires[0].Templates[0].Parts[0].Cat = game.NumGizmoCats }},
		{"template slots", "template 0: slot", func(g *game.Game) { g.Empires[0].Templates[0].Slots = []int{3, 1} }},
		{"template too many", "parts on a Small hull", func(g *game.Game) {
			tp := &g.Empires[0].Templates[0]
			for len(tp.Parts) <= game.Small.Slots() {
				tp.Parts = append(tp.Parts, game.Part{Gizmo: game.Colonizer})
			}
		}},
		{"colony nil", "colony 0: missing", func(g *game.Game) { g.Empires[0].Colonies[0] = nil }},
		{"colony star", "colony 0: star", func(g *game.Game) { home(g).Star = -1 }},
		{"colony planet", "out of 0..", func(g *game.Game) { home(g).PlanetIdx = 9 }},
		{"colony owner", "owned by 1", func(g *game.Game) { hp(g).Owner = 1 }},
		{"colony twice", "a second colony", func(g *game.Game) {
			e := g.Empires[0]
			cp := *e.Colonies[0]
			e.Colonies = append(e.Colonies, &cp)
		}},
		{"pop", "pop -1", func(g *game.Game) { home(g).Pop = -1 }},
		{"orbit shots", "shots", func(g *game.Game) { home(g).OrbitShots[2] = -1 }},
		{"project item", "project: item", func(g *game.Game) { home(g).Project.Item = game.NumItems }},
		{"project cell", "project: Factory: no square", func(g *game.Game) { home(g).Project.Cell = game.Cell{I: -1, J: 40} }},
		{"project slot", "project: slot", func(g *game.Game) {
			home(g).Project = &game.Project{Item: game.Shipyard, Orbital: true, Slot: game.OrbitSlots}
		}},
		{"ship project design", "project: no design", func(g *game.Game) { home(g).Project = &game.Project{Item: game.Ship, Orbital: true} }},
		{"refit design", "project: no design", func(g *game.Game) {
			home(g).Project = &game.Project{Item: game.OrbitalDocks, Orbital: true, Slot: -1, Refit: "x"}
		}},
		{"project design part", "project: design: part 0", func(g *game.Game) {
			home(g).Project = &game.Project{Item: game.Ship, Orbital: true, Design: &game.Design{Parts: []game.Gizmo{game.NumGizmos}}}
		}},
		{"automation target", "automation of item", func(g *game.Game) {
			pj := home(g).Project
			pj.Item, pj.Target = game.Automation, game.NoItem
		}},
		{"target", "target", func(g *game.Game) { home(g).Project.Target = game.NumItems }},
		{"progress negative", "progress -1", func(g *game.Game) { home(g).Project.Progress = -1 }},
		{"queue item", "queue 0: item", func(g *game.Game) { home(g).Queue[0].Item = -2 }},
		{"queue cell", "queue 0: Agriplot: no square", func(g *game.Game) { home(g).Queue[0].Cell = game.Cell{I: 50, J: 50} }},
		{"queue slot", "queue 1: slot", func(g *game.Game) { home(g).Queue[1].Slot = game.OrbitSlots }},
		{"ship nil", "ship 0: missing", func(g *game.Game) { g.Empires[0].Ships[0] = nil }},
		{"ship hull", "ship 0: design: hull", func(g *game.Game) { ship(g, 0).Design.Hull = game.NumHulls }},
		{"ship part", "ship 0: design: part", func(g *game.Game) {
			s := ship(g, 0) // Spawn shares the fixture design's Parts: mutate a copy
			s.Design.Parts = append([]game.Gizmo{game.NumGizmos}, s.Design.Parts[1:]...)
		}},
		{"ship slots", "ship 1: design: ", func(g *game.Game) { ship(g, 1).Design.Slots = []int{0} }},
		{"ship species", "species", func(g *game.Game) { ship(g, 0).Species = game.NumSpecies }},
		{"ship star", "ship 0: star", func(g *game.Game) { ship(g, 0).Star = len(g.Cluster.Stars) }},
		{"ship orbit", "ship 0: orbit", func(g *game.Game) { ship(g, 0).Orbit = game.MaxSystemPlanets }},
		{"ship lane", "ship 0: lane", func(g *game.Game) { ship(g, 0).Lane = len(g.Cluster.Lanes) }},
		{"ship lane far", "does not touch star", func(g *game.Game) {
			s := ship(g, 0)
			for li, l := range g.Cluster.Lanes {
				if l.A != s.Star && l.B != s.Star {
					s.Lane = li
					return
				}
			}
		}},
		{"ship cargo", "cargo -1", func(g *game.Game) { ship(g, 0).Cargo = -1 }},
		{"order kind", "order 7", func(g *game.Game) { ship(g, 0).Order.Kind = game.GoToSystem + 1 }},
		{"order planet", "order: planet", func(g *game.Game) {
			ship(g, 0).Order = game.Order{Kind: game.GoToPlanet, Planet: game.MaxSystemPlanets}
		}},
		{"order star", "order: star", func(g *game.Game) { ship(g, 0).Order = game.Order{Kind: game.GoToSystem, Star: -1} }},
		{"order point", "order: point", func(g *game.Game) { ship(g, 0).Order.Pos.Z = -2e3 }},
		{"dealt", "damage dealt by 3 empires", func(g *game.Game) { ship(g, 0).Dealt = []int{0, 0, 0} }},
		{"dealt negative", "damage dealt by 1: -4", func(g *game.Game) { ship(g, 0).Dealt = []int{0, -4} }},
		{"shots", "shots for", func(g *game.Game) { ship(g, 0).Shots = nil }},
		{"weapon", "weapon", func(g *game.Game) { ship(g, 0).Weapon = len(ship(g, 0).Design.Parts) }},
		{"game day", "day -1", func(g *game.Game) { g.Day = -1 }},
		{"atmosphere", "atmosphere", func(g *game.Game) { g.Atmosphere = game.Hostile + 1 }},
		{"rounds today", "rounds today", func(g *game.Game) { g.RoundsToday = g.RoundsToday[1:] }},
		{"rounds negative", "rounds today", func(g *game.Game) { g.RoundsToday[4] = -1 }},
		{"offer from", "offer 0: from", func(g *game.Game) { g.Offers[0].From = 2 }},
		{"offer kind", "offer 0: kind", func(g *game.Game) { g.Offers[0].Kind = game.AskAllianceWith + 1 }},
		{"offer third", "offer 0: third", func(g *game.Game) { g.Offers[0].Third = -2 }},
		{"request third", "offer 0: third", func(g *game.Game) { g.Offers[0].Kind, g.Offers[0].Third = game.AskWarOn, -1 }},
		{"no empire", "no empire", func(g *game.Game) { g.Empires = nil }},
		{"no cluster", "no cluster", func(g *game.Game) { g.Cluster = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := tamperGame(t)
			tc.f(g)
			err := g.Validate()
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%v: want %q in it", err, tc.want)
			}
		})
	}
}

// ValidateEvent takes the engine's events (every −1 that does not apply)
// and refuses a kind, item, tech, star, planet or empire off its range.
func TestValidateEvent(t *testing.T) {
	g := tamperGame(t)
	ok := game.Event{Kind: game.Engaged, Item: game.NoItem, Tech: game.NoTech, Star: -1, Planet: -1, Other: -1}
	if err := g.ValidateEvent(ok); err != nil {
		t.Fatal(err)
	}
	st := g.Empires[0].Colonies[0].Star
	npl := len(g.Cluster.Stars[st].System.Planets)
	for name, f := range map[string]func(*game.Event){
		"kind":   func(e *game.Event) { e.Kind = game.Blockade + 1 },
		"item":   func(e *game.Event) { e.Item = game.NumItems },
		"tech":   func(e *game.Event) { e.Tech = game.NumTechs },
		"star":   func(e *game.Event) { e.Star = len(g.Cluster.Stars) },
		"planet": func(e *game.Event) { e.Star, e.Planet = st, npl },
		"other":  func(e *game.Event) { e.Other = len(g.Empires) },
	} {
		ev := ok
		f(&ev)
		if err := g.ValidateEvent(ev); err == nil {
			t.Errorf("%s: accepted %+v", name, ev)
		}
	}
}

// The bounds Validate takes from each enum's last value are its String's
// table length: a kind appended past it panics here until the bound
// (validate.go) moves too.
func TestEnumBounds(t *testing.T) {
	names := map[string]func(){
		"stance":  func() { _ = (game.Alliance + 1).String() },
		"offer":   func() { _ = (game.AskAllianceWith + 1).String() },
		"victory": func() { _ = (game.UniteVictory + 1).String() },
		"event":   func() { _ = (game.Blockade + 1).String() },
	}
	for name, f := range names {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: a value past the bound has a name: move its bound in validate.go", name)
				}
			}()
			f()
		}()
	}
	if s := (game.GoToSystem + 1).String(); !strings.HasPrefix(s, "order ") {
		t.Errorf("order kind past GoToSystem named %q: move numOrderKinds", s)
	}
	_ = game.Alliance.String() + game.AskAllianceWith.String() + game.UniteVictory.String() + game.Engaged.String()
}
