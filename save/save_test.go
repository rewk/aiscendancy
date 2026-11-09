package save

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/journal"
)

// play runs n days of g with its bots (bot.Day: the combat rounds
// included), returning the rounds' reports and the events' texts per day
// (every empire's, prefixed by its index) and appending empire 0's events
// to *log (the window's log is the player's).
func play(g *game.Game, bots []*bot.Bot, n int, log *[]Entry) [][]string {
	var days [][]string
	for range n {
		evs, reps, _ := bot.Day(g, bots)
		var texts []string
		for _, r := range reps {
			texts = append(texts, fmt.Sprintf("%d %s", r.Empire, r.Event.Text))
		}
		for i, ev := range evs {
			for _, x := range ev {
				texts = append(texts, fmt.Sprintf("%d %s", i, x.Text))
				if i == 0 {
					*log = append(*log, Entry{Day: g.Day, Event: x})
				}
			}
		}
		days = append(days, texts)
	}
	return days
}

// saveAll is f for g and its bots.
func saveAll(g *game.Game, bots []*bot.Bot, log []Entry) *File {
	f := &File{Seed: g.Seed, Game: g, Log: log}
	for _, b := range bots {
		f.Bots = append(f.Bots, b.Save())
	}
	return f
}

// A two-empire game saved on day 400 and loaded plays the next 100 days
// exactly as the game that never stopped: the same events day by day, and
// the two saves written on day 500 are the same bytes (colonies, projects,
// ships, research, templates, owners, the bots' progress, the log). The
// loaded empires share the loaded cluster.
func TestRoundTripPlaysOn(t *testing.T) {
	dir := t.TempDir()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	bots := bot.NewAll(g, bot.Default())
	var log []Entry
	play(g, bots, 400, &log)
	for i, e := range g.Empires {
		if len(e.Colonies) < 2 || len(e.Ships) == 0 && len(e.Colonies) < 3 {
			t.Fatalf("day 400, empire %d: %d colonies, %d ships: the fixture should have spread", i, len(e.Colonies), len(e.Ships))
		}
	}
	p := filepath.Join(dir, "a.json")
	if err := Write(p, saveAll(g, bots, log)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p + ".tmp"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the .tmp survived Write: %v", err)
	}
	f, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != Version || f.Seed != 1 || len(f.Bots) != 2 || !reflect.DeepEqual(f.Log, log) {
		t.Fatalf("read back: version %d seed %d bots %d, log equal %v", f.Version, f.Seed, len(f.Bots), reflect.DeepEqual(f.Log, log))
	}
	g2 := f.Game
	var bots2 []*bot.Bot
	for i, e2 := range g2.Empires {
		if e2.Cluster != g2.Cluster || e2.Index != i {
			t.Fatalf("empire %d: not on the loaded cluster, or index %d", i, e2.Index)
		}
		for k, c := range e2.Colonies {
			if c.Planet == nil || c.Planet != e2.Planet(c.Star, c.PlanetIdx) || c.Planet.Owner != i {
				t.Fatalf("empire %d colony %d: Planet not relinked into the loaded cluster", i, k)
			}
		}
		b2 := bot.Resume(e2, f.Bots[i])
		e, b := g.Empires[i], bots[i]
		if len(b2.Founded) != len(b.Founded) {
			t.Fatalf("empire %d Founded: %d colonies, want %d", i, len(b2.Founded), len(b.Founded))
		}
		for k, c := range e.Colonies {
			if b2.Founded[e2.Colonies[k]] != b.Founded[c] {
				t.Errorf("empire %d colony %d founded day %d, want %d", i, k, b2.Founded[e2.Colonies[k]], b.Founded[c])
			}
		}
		bots2 = append(bots2, b2)
	}

	log2 := f.Log
	d1 := play(g, bots, 100, &log)
	d2 := play(g2, bots2, 100, &log2)
	for i := range d1 {
		if !reflect.DeepEqual(d1[i], d2[i]) {
			t.Fatalf("day %d: %q, loaded %q", 401+i, d1[i], d2[i])
		}
	}
	if g.Day != 500 || g2.Day != 500 || g2.Empires[1].Day != 500 {
		t.Fatalf("days %d %d %d", g.Day, g2.Day, g2.Empires[1].Day)
	}
	pa, pb := filepath.Join(dir, "500a.json"), filepath.Join(dir, "500b.json")
	if err := Write(pa, saveAll(g, bots, log)); err != nil {
		t.Fatal(err)
	}
	if err := Write(pb, saveAll(g2, bots2, log2)); err != nil {
		t.Fatal(err)
	}
	ba, _ := os.ReadFile(pa)
	bb, _ := os.ReadFile(pb)
	if !bytes.Equal(ba, bb) {
		t.Error("day 500: the saves of the played-through and the loaded game differ")
	}
	// The run must have changed something the save holds, or the byte
	// comparison proves nothing.
	b400, _ := os.ReadFile(p)
	if bytes.Equal(b400, ba) {
		t.Error("days 401-500 changed nothing")
	}
}

// A two-empire game with the fixtures fighting by rounds (the Raiders'
// shields off, so the Gunboat wears them down; no flight) saved on day 3
// plays the next 40 days as the game that never stopped: the same events
// and reports, a destruction among them, the same bytes at the end. A
// save between days holds no armed shot.
func TestRoundTripRounds(t *testing.T) {
	dir := t.TempDir()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	if err := game.SpawnFixtures(g, 1, 1, 2); err != nil {
		t.Fatal(err)
	}
	for _, s := range g.Empires[1].Ships {
		s.Shields = false
	}
	// The round tactics would have both sides flee (each overrates the
	// other's unscanned Small hulls, NeutralCombat's HullThreat): a
	// FleeRatio no pressure reaches keeps the warships fighting (finite:
	// JSON has no +Inf).
	cfg := bot.Default()
	cfg.Genome = bot.Neutral()
	cfg.Genome.FleeRatio = 1e9
	cfg.Genome.Temper = 300 // no peace offered or accepted: the war weariness would end the fight
	bots := bot.NewAll(g, cfg)
	var log []Entry
	play(g, bots, 3, &log)
	p := filepath.Join(dir, "a.json")
	if err := Write(p, saveAll(g, bots, log)); err != nil {
		t.Fatal(err)
	}
	f, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Empires {
		for _, s := range e.Ships {
			if len(s.Armed) != 0 {
				t.Fatalf("%s armed between days: %+v", s.Name, s.Armed)
			}
		}
	}
	var bots2 []*bot.Bot
	for i, e2 := range f.Game.Empires {
		b2 := bot.Resume(e2, f.Bots[i])
		b2.G = f.Game
		bots2 = append(bots2, b2)
	}
	log2 := f.Log
	d1 := play(g, bots, 40, &log)
	d2 := play(f.Game, bots2, 40, &log2)
	fought := false
	for i := range d1 {
		if !reflect.DeepEqual(d1[i], d2[i]) {
			t.Fatalf("day %d: %q, loaded %q", 4+i, d1[i], d2[i])
		}
		for _, x := range d1[i] {
			fought = fought || strings.Contains(x, " destroys ")
		}
	}
	if !fought {
		t.Error("no destruction in 40 days: the fixture proves nothing")
	}
	pa, pb := filepath.Join(dir, "a43.json"), filepath.Join(dir, "b43.json")
	if err := Write(pa, saveAll(g, bots, log)); err != nil {
		t.Fatal(err)
	}
	if err := Write(pb, saveAll(f.Game, bots2, log2)); err != nil {
		t.Fatal(err)
	}
	ba, _ := os.ReadFile(pa)
	bb, _ := os.ReadFile(pb)
	if !bytes.Equal(ba, bb) {
		t.Error("day 43: the saves differ")
	}
}

func TestReadMissing(t *testing.T) {
	if _, err := Read(filepath.Join(t.TempDir(), "none.json")); err == nil {
		t.Error("a missing file read")
	}
}

// Write creates the directories it needs; a save without a bot reads back
// with a nil Bot.
func TestWriteNestedNoBot(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x", "y", "z.json")
	if err := Write(p, &File{Game: game.NewGame(2, []game.Species{game.Snovemdomas}, game.Sparse)}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if f.Bots != nil || f.Version != Version {
		t.Errorf("bots %v, version %d", f.Bots, f.Version)
	}
}

// Read refuses another version (the previous one among them), a bot
// count that is not the empire count, and a colony on a planet not in the
// cluster; Write refuses the bot count too.
func TestReadRefuses(t *testing.T) {
	dir := t.TempDir()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	e := g.Empires[0]
	p := filepath.Join(dir, "v.json")
	if err := Write(p, &File{Game: g, Bots: []*bot.State{nil, nil}}); err != nil {
		t.Fatal(err)
	}
	b := plain(t, p)
	if _, err := Read(p); err != nil {
		t.Fatalf("two nil bots: %v", err)
	}
	writePlain(t, p, bytes.Replace(b, fmt.Appendf(nil, `"Version":%d,`, Version), fmt.Appendf(nil, `"Version":%d,`, Version-1), 1))
	if _, err := Read(p); err == nil {
		t.Error("the previous version read")
	}
	writePlain(t, p, bytes.Replace(b, []byte(`"Bots":[null,null]`), []byte(`"Bots":[null]`), 1))
	if _, err := Read(p); err == nil {
		t.Error("one bot for two empires read")
	}
	if err := Write(p, &File{Game: g, Bots: []*bot.State{nil}}); err == nil {
		t.Error("one bot for two empires written")
	}
	e.Colonies[0].PlanetIdx = len(e.Cluster.Stars[e.Colonies[0].Star].System.Planets)
	if err := Write(p, &File{Game: g}); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(p); err == nil {
		t.Error("a colony past its system's planets read")
	}
}

// Visited survives a round trip: a star explored by a ship gone since
// (no presence for Revisit to restore) is still known after the load.
func TestRoundTripKeepsVisited(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	home := g.Empires[0].Colonies[0].Star
	far := -1
	for st := range g.Cluster.Stars {
		if !g.Empires[0].Knows(st) && !g.Empires[1].Present(st) {
			far = st
			break
		}
	}
	g.Empires[0].Visit(far)
	path := filepath.Join(t.TempDir(), "v.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	e := f.Game.Empires[0]
	if !e.Knows(far) || !e.Knows(home) || e.Present(far) {
		t.Errorf("loaded: knows far %v, home %v, present far %v", e.Knows(far), e.Knows(home), e.Present(far))
	}
}

// Planet.Lush is saved: a bombed home loads bombed, its capacity with
// the bonus.
func TestRoundTripKeepsLush(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	c := g.Empires[0].Colonies[0]
	c.Planet.Lush = true
	want := c.Capacity()
	path := filepath.Join(t.TempDir(), "l.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if lc := f.Game.Empires[0].Colonies[0]; !lc.Planet.Lush || lc.Capacity() != want {
		t.Errorf("loaded: lush %v, capacity %d, want %d", lc.Planet.Lush, lc.Capacity(), want)
	}
	if f.Game.Empires[1].Colonies[0].Planet.Lush {
		t.Error("empire 1's home loaded bombed")
	}
}

// Square.Automated is saved, and a running Automation's Target: the
// home's automated Colony Base loads automated (its worker free), an
// Agriplot's Automation (a Target not the zero Item) at the same cost.
func TestRoundTripKeepsAutomated(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	e := g.Empires[0]
	c := e.Colonies[0]
	base := game.Cell{I: game.Centre, J: game.Centre}
	c.Planet.Surface[base].Automated = true
	f, ok := c.Planet.BestCell(game.Agriplot, c.Traits())
	if !ok {
		t.Fatal("no Agriplot square")
	}
	if err := c.Planet.Build(f, game.Agriplot); err != nil {
		t.Fatal(err)
	}
	e.Research.Grant(game.Tech(game.Automation.Info().ResearchReq))
	c.Pop = c.Busy() + 1
	if err := c.Start(e.Research, game.Automation, f); err != nil {
		t.Fatal(err)
	}
	busy, cost := c.Busy(), c.Project.Cost()
	path := filepath.Join(t.TempDir(), "a.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	ld, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	lc := ld.Game.Empires[0].Colonies[0]
	if !lc.Planet.Surface[base].Automated || lc.Planet.Surface[f].Automated || lc.Busy() != busy {
		t.Errorf("loaded: base automated %v, agriplot %v, busy %d, want true, false, %d",
			lc.Planet.Surface[base].Automated, lc.Planet.Surface[f].Automated, lc.Busy(), busy)
	}
	if pj := lc.Project; pj == nil || pj.Item != game.Automation || pj.Target != game.Agriplot || pj.Cost() != cost {
		t.Errorf("loaded project %+v, want the Agriplot's Automation at cost %d", pj, cost)
	}
}

// Square.Ruins is saved: every generated ruin loads where it was, and a
// ruin set by hand on the home too.
func TestRoundTripKeepsRuins(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	home := g.Empires[0].Colonies[0].Planet
	at := home.Cells()[0]
	home.Surface[at].Ruins = true
	ruins := func(g *game.Game) map[[2]int]game.Cell {
		m := map[[2]int]game.Cell{}
		for i := range g.Cluster.Stars {
			for k, p := range g.Cluster.Stars[i].System.Planets {
				if c, ok := p.RuinsAt(); ok {
					m[[2]int{i, k}] = c
				}
			}
		}
		return m
	}
	want := ruins(g)
	path := filepath.Join(t.TempDir(), "r.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := ruins(f.Game); len(want) < 2 || !reflect.DeepEqual(got, want) {
		t.Errorf("loaded ruins %v, want %v", got, want)
	}
	if lc, ok := f.Game.Empires[0].Colonies[0].Planet.RuinsAt(); !ok || lc != at {
		t.Errorf("home ruins at %v %v, want %v", lc, ok, at)
	}
}

// The empires' colours are saved: a game with the player blue loads with
// every empire's colour as it was.
func TestColoursSaved(t *testing.T) {
	g := game.NewGame(1, game.PickSpecies(1, game.Orfa, 4), game.Average)
	g.SetColour(0, game.Blue)
	var want []int
	for _, e := range g.Empires {
		want = append(want, e.Colour)
	}
	path := filepath.Join(t.TempDir(), "c.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range f.Game.Empires {
		if e.Colour != want[i] {
			t.Errorf("empire %d: colour %d, want %d", i, e.Colour, want[i])
		}
	}
}

// A bot's genome is saved with its Config: two empires on a Neutral
// genome saved on day 300 and loaded play the next 100 days as the game
// that never stopped (the same events, the same day-400 save bytes).
func TestRoundTripKeepsGenome(t *testing.T) {
	if Version < 34 {
		t.Fatalf("Version %d, want 34 or later (the replacement genes)", Version)
	}
	dir := t.TempDir()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	cfg := bot.Default()
	cfg.Genome = bot.Neutral()
	cfg.Genome.FrontierWorth = 5 // off Neutral, so a lost genome plays differently
	cfg.Genome.Build[game.Habitat] = 3
	cfg.Genome.HullReach[game.Large], cfg.Genome.Standoff, cfg.Genome.Rounds = 0.6, 0.7, 12 // the round genes too
	cfg.Genome.RefitGain, cfg.Genome.RepairAt = 1.5, 0.3                                    // and the refit's
	cfg.Genome.ReplaceDays, cfg.Genome.ReplaceFull = 200, 1                                 // and the replacement's (Version 34)
	bots := bot.NewAll(g, cfg)
	var log []Entry
	play(g, bots, 300, &log)
	p := filepath.Join(dir, "a.json")
	if err := Write(p, saveAll(g, bots, log)); err != nil {
		t.Fatal(err)
	}
	f, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	var bots2 []*bot.Bot
	for i, e2 := range f.Game.Empires {
		b2 := bot.Resume(e2, f.Bots[i])
		if gn := b2.Cfg.Genome; gn == nil || *gn != *cfg.Genome {
			t.Fatalf("empire %d: genome %v, want %v", i, gn, cfg.Genome)
		}
		b2.G = f.Game
		bots2 = append(bots2, b2)
	}
	log2 := f.Log
	d1 := play(g, bots, 100, &log)
	d2 := play(f.Game, bots2, 100, &log2)
	for i := range d1 {
		if !reflect.DeepEqual(d1[i], d2[i]) {
			t.Fatalf("day %d: %q, loaded %q", 301+i, d1[i], d2[i])
		}
	}
	pa, pb := filepath.Join(dir, "400a.json"), filepath.Join(dir, "400b.json")
	if err := Write(pa, saveAll(g, bots, log)); err != nil {
		t.Fatal(err)
	}
	if err := Write(pb, saveAll(f.Game, bots2, log2)); err != nil {
		t.Fatal(err)
	}
	ba, _ := os.ReadFile(pa)
	bb, _ := os.ReadFile(pb)
	if !bytes.Equal(ba, bb) {
		t.Error("day 400: the saves of the played-through and the loaded game differ")
	}
}

// A running refit survives a round trip: the loaded game holds the same
// project and completes it on the same day, the ship refitted alike.
func TestRoundTripRefit(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	for _, tc := range []game.Tech{game.OrbitalStructures, game.InterplanetaryExploration, game.TonklinDiary,
		game.Xenobiology, game.EnvironmentalEncapsulation, game.Tech(game.Items[game.OrbitalDocks].ResearchReq)} {
		e.Research.Grant(tc)
	}
	home := e.Colonies[0]
	if err := home.Planet.BuildOrbital(0, game.OrbitalDocks); err != nil {
		t.Fatal(err)
	}
	home.Pop = home.Busy() + 1
	d, ok := game.ColonyShip(e.Research, false)
	if !ok {
		t.Fatal("no colony ship")
	}
	s := e.Spawn(d, home.Star, home.PlanetIdx)
	s.Hull -= 3
	if err := e.Refit(home, s, s.Design); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "r.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	le := f.Game.Empires[0]
	if !reflect.DeepEqual(le.Colonies[0].Project, home.Project) || le.Refitting(le.Ships[0]) != le.Colonies[0] {
		t.Fatalf("loaded project %+v, want %+v", le.Colonies[0].Project, home.Project)
	}
	for day := 1; day <= 200; day++ {
		a, b := g.EndDay(), f.Game.EndDay()
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("day %d: %v vs %v", day, a, b)
		}
		if home.Project == nil {
			if le.Colonies[0].Project != nil || !reflect.DeepEqual(le.Ships[0], s) || s.Hull != s.MaxHull() {
				t.Errorf("after the refit: loaded ship %+v, ship %+v", le.Ships[0], s)
			}
			return
		}
	}
	t.Fatal("the refit never completed")
}

// Planet.OrbitDamage (hidden, per slot) and Colony.OrbitShots (the
// orbital weapons' shots today) are saved.
func TestRoundTripOrbitDamage(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	col := g.Empires[0].Colonies[0]
	for slot, it := range map[int]game.Item{2: game.OrbitalMegaShield, 7: game.LongRangeOrbitalWhopper} {
		if err := col.Planet.BuildOrbital(slot, it); err != nil {
			t.Fatal(err)
		}
	}
	col.Planet.OrbitDamage[2], col.Planet.OrbitDamage[7] = 21, 4
	col.OrbitShots[7] = 2
	path := filepath.Join(t.TempDir(), "o.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	lc := f.Game.Empires[0].Colonies[0]
	if lc.Planet.OrbitDamage != col.Planet.OrbitDamage || lc.OrbitShots != col.OrbitShots {
		t.Errorf("loaded damage %v shots %v, want %v %v", lc.Planet.OrbitDamage, lc.OrbitShots, col.Planet.OrbitDamage, col.OrbitShots)
	}
}

// A ship's Invasion Modules left (Starship.Invaders) and an Invade order
// are saved: the loaded game captures the planet as the original does,
// and the captured colony, written again, loads with its captor's Traits.
func TestRoundTripInvade(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	e0, col := g.Empires[0], g.Empires[1].Colonies[0]
	g.Contact(0, 1)
	if _, err := g.DeclareWar(0, 1); err != nil {
		t.Fatal(err)
	}
	d := game.Design{Name: "Invader", Hull: game.Small, Parts: []game.Gizmo{game.InvasionModule, game.InvasionModule}}
	s := e0.Spawn(d, col.Star, col.PlanetIdx)
	s.Invaders = 1 // a live count apart from the design's
	if err := e0.Order(s, game.Order{Kind: game.Invade}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "i.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	ls := f.Game.Empires[0].Ships[0]
	if ls.Invaders != 1 || ls.Order != s.Order {
		t.Fatalf("loaded ship: %d modules, order %+v", ls.Invaders, ls.Order)
	}
	l0, l1 := f.Game.Empires[0], f.Game.Empires[1]
	if l0.StanceWith(1) != game.War || l1.StanceWith(0) != game.War || *l1.Rel(0) != *g.Empires[1].Rel(0) {
		t.Fatalf("loaded relations: %v / %+v", l0.StanceWith(1), *l1.Rel(0))
	}
	a, b := g.EndDay(), f.Game.EndDay()
	if !reflect.DeepEqual(a, b) || f.Game.Cluster.Stars[col.Star].System.Planets[col.PlanetIdx].Owner != 0 {
		t.Errorf("after the close: %v vs %v", a, b)
	}
	// A colony has no species in the file: the captured Orfa one loads
	// with its Snovemdomas captor's Traits (no building on black, hulls
	// doubled: an unadopted colony's base Traits have 1).
	if err := Write(path, &File{Version: Version, Game: f.Game}); err != nil {
		t.Fatal(err)
	}
	raw := plain(t, path)
	var doc struct {
		Game struct {
			Empires []struct{ Colonies []map[string]any }
		}
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, e := range doc.Game.Empires {
		for _, c := range e.Colonies {
			if _, ok := c["Species"]; ok {
				t.Fatal("a colony's Species in the save")
			}
		}
	}
	if f, err = Read(path); err != nil {
		t.Fatal(err)
	}
	if lc := f.Game.Empires[0].Colony(col.Star, col.PlanetIdx); lc == nil || lc.Traits().BuildsOnBlack() || lc.Traits().HullFactor() != 2 {
		t.Errorf("captured colony loaded %+v", lc)
	}
}

// Diplomacy lot 2 is saved: an alliance (both ways, AllyDays), a lane
// learnt by an exchange (Empire.LanesKnown, Trades) and Hospitality load
// as they were.
func TestRoundTripKeepsAlliance(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	e0, e1 := g.Empires[0], g.Empires[1]
	g.Contact(0, 1)
	if _, err := g.OfferAlliance(0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Answer(g.PendingOffers(1)[0], true); err != nil {
		t.Fatal(err)
	}
	lane := -1
	for li, l := range g.Cluster.Lanes {
		if !e0.Knows(l.A) && !e0.Knows(l.B) && !e1.KnowsLane(li) {
			lane = li
			break
		}
	}
	if lane < 0 {
		t.Fatal("fixture: no lane unknown to both")
	}
	e1.LearnLane(lane)
	if _, err := g.OfferTrade(1, 0, game.TradeLanes); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Answer(g.PendingOffers(0)[0], true); err != nil {
		t.Fatal(err)
	}
	g.EndDay()
	e1.Relations[0].Hospitality = 7
	path := filepath.Join(t.TempDir(), "a.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	l0, l1 := f.Game.Empires[0], f.Game.Empires[1]
	if !reflect.DeepEqual(l0.Relations, e0.Relations) || !reflect.DeepEqual(l1.Relations, e1.Relations) {
		t.Errorf("relations %+v / %+v, want %+v / %+v", l0.Relations, l1.Relations, e0.Relations, e1.Relations)
	}
	if l0.StanceWith(1) != game.Alliance || l0.Rel(1).AllyDays != 1 || l0.Rel(1).Trades != 1 || l1.Rel(0).Hospitality != 7 {
		t.Errorf("loaded: %+v / %+v", *l0.Rel(1), *l1.Rel(0))
	}
	if !l0.KnowsLane(lane) || !slices.Equal(l0.LanesKnown, e0.LanesKnown) {
		t.Errorf("loaded: lane %d known %v", lane, l0.KnowsLane(lane))
	}
}

// Diplomacy lot 3 is saved: a pending request with its Third, the
// ledger's Helped and Defamed, AbilityUsed (a use, and 0 the start) load as
// they were.
func TestRoundTripKeepsRequest(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Baliflids, game.Orfa, game.Mebes}, game.Average)
	g.Contact(0, 1)
	g.Contact(0, 2)
	g.Contact(1, 2)
	if _, err := g.OfferAlliance(1, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Answer(g.PendingOffers(0)[0], true); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Request(0, 1, game.AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	g.Empires[0].Relations[1].Helped = 3
	g.Empires[2].Relations[1].Defamed = 2
	g.Empires[0].AbilityUsed = 5
	path := filepath.Join(t.TempDir(), "r.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	l := f.Game
	if !slices.Equal(l.Offers, g.Offers) || l.Offers[0].Third != 2 || l.Offers[0].Kind != game.AskWarOn {
		t.Errorf("offers %+v, want %+v", l.Offers, g.Offers)
	}
	for i, e := range g.Empires {
		if !reflect.DeepEqual(l.Empires[i].Relations, e.Relations) || l.Empires[i].AbilityUsed != e.AbilityUsed {
			t.Errorf("%d: %+v used %d, want %+v used %d", i, l.Empires[i].Relations, l.Empires[i].AbilityUsed, e.Relations, e.AbilityUsed)
		}
	}
	if l.Empires[0].Rel(1).Helped != 3 || l.Empires[2].Rel(1).Defamed != 2 || l.Empires[0].AbilityUsed != 5 || l.Empires[1].AbilityUsed != 0 {
		t.Errorf("loaded: %+v / %+v", *l.Empires[0].Rel(1), *l.Empires[2].Rel(1))
	}
}

// The bots' diplomacy of lot 2 is saved: two bots trading a while hold
// goodwill and the days of their last proposals, which load and Resume
// as they were (Resume would start a mismatched slice fresh).
func TestRoundTripKeepsGoodwill(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	g.Contact(0, 1)
	bots := bot.NewAll(g, bot.Default())
	var log []Entry
	play(g, bots, 30, &log)
	before := saveAll(g, bots, log)
	traded := false
	for _, st := range before.Bots {
		traded = traded || slices.ContainsFunc(st.Goodwill, func(x float64) bool { return x != 0 }) && slices.Contains(st.LastTrade, 0)
	}
	if !traded {
		t.Fatalf("fixture: no goodwill nor proposal: %v %v / %v %v",
			before.Bots[0].Goodwill, before.Bots[0].LastTrade, before.Bots[1].Goodwill, before.Bots[1].LastTrade)
	}
	path := filepath.Join(t.TempDir(), "a.json")
	if err := Write(path, before); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range f.Game.Empires {
		want := bots[i].Save()
		got := bot.Resume(e, f.Bots[i]).Save()
		if !reflect.DeepEqual(got.Goodwill, want.Goodwill) || !slices.Equal(got.LastTrade, want.LastTrade) {
			t.Errorf("empire %d: goodwill %v lastTrade %v, want %v %v", i, got.Goodwill, got.LastTrade, want.Goodwill, want.LastTrade)
		}
	}
}

// The abilities' state is saved: a Mebes' PopBonus, a Capelons'
// Invincible, an Arbryls' BlockedUntil load as they were, the others'
// defaults (0, −1, 0) too.
func TestRoundTripKeepsAbilities(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Mebes, game.Capelons, game.Arbryls}, game.Average)
	for i, e := range g.Empires {
		e.AbilityUsed = g.Day - e.Species.Info().Recharge // charged
		if _, err := g.UseAbility(i); err != nil {
			t.Fatal(err)
		}
	}
	g.Empires[0].Colonies[0].Planet.PopBonus = 3
	g.Empires[1].Invincible = 7
	g.Empires[2].BlockedUntil = 41
	path := filepath.Join(t.TempDir(), "a.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range g.Empires {
		l := f.Game.Empires[i]
		if l.Colonies[0].Planet.PopBonus != e.Colonies[0].Planet.PopBonus || l.Invincible != e.Invincible || l.BlockedUntil != e.BlockedUntil {
			t.Errorf("%d: bonus %d invincible %d blocked %d, want %d %d %d", i, l.Colonies[0].Planet.PopBonus, l.Invincible, l.BlockedUntil,
				e.Colonies[0].Planet.PopBonus, e.Invincible, e.BlockedUntil)
		}
	}
	if l := f.Game.Empires; l[0].Colonies[0].Planet.PopBonus != 3 || l[1].Invincible != 7 || l[2].BlockedUntil != 41 ||
		l[0].Invincible != -1 || l[1].Colonies[0].Planet.PopBonus != 0 || l[0].BlockedUntil != 0 {
		t.Errorf("loaded: %d %d %d", l[0].Colonies[0].Planet.PopBonus, l[1].Invincible, l[2].BlockedUntil)
	}
}

// The day's specials are saved: Boosted (the Accutron), Invulnerable
// (the Invulnerablizer), TiedDown, Scrambled and ShieldsDown (the
// targeted specials) load as they were.
func TestRoundTripSpecials(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	d := game.Design{Name: "Bubble", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor}}
	a, b := e.Spawn(d, e.Colonies[0].Star, 0), e.Spawn(d, e.Colonies[0].Star, 0)
	a.Boosted, b.Invulnerable = true, true
	a.TiedDown, a.ShieldsDown, b.Scrambled = true, true, true
	path := filepath.Join(t.TempDir(), "s.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	la, lb := f.Game.Empires[0].Ships[0], f.Game.Empires[0].Ships[1]
	if !la.Boosted || la.Invulnerable || lb.Boosted || !lb.Invulnerable {
		t.Errorf("loaded: %v %v / %v %v", la.Boosted, la.Invulnerable, lb.Boosted, lb.Invulnerable)
	}
	if !la.TiedDown || la.Scrambled || !la.ShieldsDown || lb.TiedDown || !lb.Scrambled || lb.ShieldsDown {
		t.Errorf("loaded flags: %v %v %v / %v %v %v", la.TiedDown, la.Scrambled, la.ShieldsDown, lb.TiedDown, lb.Scrambled, lb.ShieldsDown)
	}
}

// The Lane Blockers' closures are saved: an empire's LaneBlocks load as
// they were (lane → until), the other's stays nil, and the block still
// closes the lane to the other after the load.
func TestRoundTripLaneBlocks(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	home := g.Empires[0].Colonies[0].Star
	li := g.Cluster.Stars[home].Lanes[0]
	g.Empires[0].LaneBlocks = map[int]int{li: g.Day + game.BlockDays, li + 1: 3}
	path := filepath.Join(t.TempDir(), "l.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	l := f.Game.Empires
	if !reflect.DeepEqual(l[0].LaneBlocks, g.Empires[0].LaneBlocks) || l[1].LaneBlocks != nil {
		t.Errorf("loaded %v / %v, want %v / nil", l[0].LaneBlocks, l[1].LaneBlocks, g.Empires[0].LaneBlocks)
	}
	if b := l[1].LaneBlocker(home, li, f.Game.Day); b != l[0] {
		t.Errorf("loaded block: %v", b)
	}
}

// The action journal is not saved: a game played with one is written
// byte for byte as without it (the Game's, Empires' and Colonies' Writer
// fields unexported), and reads back.
func TestJournalNotSaved(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	bots := bot.NewAll(g, bot.Default())
	var buf bytes.Buffer
	g.SetJournal(journal.New(&buf))
	var log []Entry
	play(g, bots, 60, &log)
	if buf.Len() == 0 {
		t.Fatal("60 days of bots journalled nothing")
	}
	dir := t.TempDir()
	with, without := filepath.Join(dir, "with.json"), filepath.Join(dir, "without.json")
	if err := Write(with, saveAll(g, bots, log)); err != nil {
		t.Fatal(err)
	}
	g.SetJournal(nil)
	if err := Write(without, saveAll(g, bots, log)); err != nil {
		t.Fatal(err)
	}
	a, errA := os.ReadFile(with)
	b, errB := os.ReadFile(without)
	if errA != nil || errB != nil || !bytes.Equal(a, b) {
		t.Fatalf("saves differ (%v, %v): %d and %d bytes", errA, errB, len(a), len(b))
	}
	if strings.Contains(string(a), "journal") {
		t.Error("a journal field in the save")
	}
	if _, err := Read(with); err != nil {
		t.Fatal(err)
	}
}

// An empire's defeat day is saved: Lost loads as it was, 0 for the
// living; Version 32 or later (the field's bump).
func TestRoundTripKeepsLost(t *testing.T) {
	if Version < 32 {
		t.Fatalf("Version %d, want 32 or later", Version)
	}
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	g.Empires[1].Lost = 17
	path := filepath.Join(t.TempDir(), "lost.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if l := f.Game.Empires; l[0].Lost != 0 || !l[0].Alive() || l[1].Lost != 17 || l[1].Alive() {
		t.Errorf("loaded: lost %d and %d", l[0].Lost, l[1].Lost)
	}
}

// A ship's damage ledger (Starship.Dealt) survives the round trip.
func TestRoundTripDealt(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	if err := game.SpawnFixtures(g, 1, 1, 0); err != nil {
		t.Fatal(err)
	}
	s := g.Empires[0].Ships[0]
	s.Hull -= 3
	s.Dealt = []int{0, 3}
	p := filepath.Join(t.TempDir(), "a.json")
	if err := Write(p, &File{Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Game.Empires[0].Ships[0]; got.Name != s.Name || !reflect.DeepEqual(got.Dealt, s.Dealt) {
		t.Errorf("%s: Dealt %v, want %v", got.Name, got.Dealt, s.Dealt)
	}
}

// Colony.Queue is saved, in order, its targets whole.
func TestRoundTripKeepsQueue(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	e := g.Empires[0]
	c := e.Colonies[0]
	e.Research.Grant(game.OrbitalStructures)
	at, ok := c.Planet.BestCell(game.Factory, c.Traits())
	if !ok {
		t.Fatal("no Factory square")
	}
	if err := c.Enqueue(e.Research, game.Factory, at); err != nil {
		t.Fatal(err)
	}
	if err := c.EnqueueOrbital(e.Research, game.Shipyard, 3); err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(c.Queue)
	path := filepath.Join(t.TempDir(), "q.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	ld, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := ld.Game.Empires[0].Colonies[0].Queue; !slices.Equal(got, want) {
		t.Errorf("loaded queue %+v, want %+v", got, want)
	}
}

// A mid-day step (RoundsToday) survives a save: the loaded game's close
// refills the stepped star's ships, not Stepping them, as the unsaved
// game's does (same events, same positions and energy).
func TestRoundTripRoundsToday(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	g.Contact(0, 1)
	if _, err := g.DeclareWar(0, 1); err != nil {
		t.Fatal(err)
	}
	home := g.Empires[0].Colonies[0]
	s := g.Empires[0].Spawn(game.FixtureGunboat, home.Star, home.PlanetIdx)
	foe := g.Empires[1].Spawn(game.FixtureRaider, home.Star, home.PlanetIdx)
	s.Orbit, foe.Orbit = -1, -1
	s.Order = game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: game.SystemReach}}
	if _, _, _, err := g.StepRound(g.Empires[0], home.Star, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "r.json")
	if err := Write(path, &File{Version: Version, Game: g}); err != nil {
		t.Fatal(err)
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.Game.RoundsToday, g.RoundsToday) || f.Game.RoundsAt(home.Star) != 1 {
		t.Fatalf("loaded RoundsToday: %d at the star, want 1", f.Game.RoundsAt(home.Star))
	}
	ls := f.Game.Empires[0].Ships[0]
	a, b := g.EndDay(), f.Game.EndDay()
	if !reflect.DeepEqual(a, b) {
		t.Errorf("after the close: %v vs %v", a, b)
	}
	if ls.Pos != s.Pos || ls.Power != s.Power || ls.Order != s.Order {
		t.Errorf("loaded ship at the close: %+v energy %d, unsaved %+v energy %d", ls.Pos, ls.Power, s.Pos, s.Power)
	}
	if f.Game.RoundsToday != nil {
		t.Errorf("loaded RoundsToday %v after the close", f.Game.RoundsToday)
	}
}
