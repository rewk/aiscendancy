// sim runs a colony's days offline, headless (no Ebiten: no display
// needed): a species' home world, a build script, a research plan, one
// line per day, then the final state.
//
//	sim                      Snovemdomas home world, seed 1, 200 days, the default script
//	sim -species orfa -seed 3 -days 400 -script agriplot,laboratory,factory -research auto
//	sim -quiet               only the final state and the events
//
// Each day, before the colony's EndDay: when no project runs, the next
// script item is started at its BestCell (orbital items: the first slot
// that takes it) if CanStart allows it, else the script waits and the
// project column shows why. Once the script is done, -policy greedy keeps
// building (see bot.GreedyPick); -policy none stops. Research: with no topic,
// the -research queue's head is studied, the queue done the way to
// bot.AimItems' techs, else Available()[0] (see bot.PickTopic; auto: no
// queue); -research none never picks a topic.
//
//	sim -galaxy -density average -seed 1 -species snovemdomas -days 600 [-quiet] [-v]
//
// runs an empire instead: the cluster rolled from the seed, the home world
// at its home star, colony ships founding colonies (see runGalaxy).
//
//	sim -galaxy -empires 3 -seed 1 -days 400
//
// puts three empires on the cluster, each played by a bot: -species, then
// two drawn from the seed (game.PickSpecies); their lines are prefixed by
// the species.
//
//	sim -galaxy -load game.json -days 100
//
// plays 100 more days of a saved game (cmd/aiscendancy's F5, package
// save): its empires and their bots; -seed, -density, -species and
// -empires do not apply, the bot's flags only to an empire the save has no
// bot for (a fresh one plays).
//
//	sim -galaxy -genome g.json -days 1500
//
// every fresh bot plays the genome (bot.ReadGenome, cmd/fuzz -o); a bot
// resumed from -load keeps its own. -genome needs -galaxy.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/journal"
	"github.com/rewk/aiscendancy/save"
)

type config struct {
	species  game.Species
	seed     uint64
	days     int
	script   []game.Item
	greedy   bool
	research string // "auto", "none" or "queue"
	queue    []game.Tech
	quiet    bool
	galaxy   bool // -galaxy: an empire on a cluster, see runGalaxy
	density  game.Density
	verbose  bool            // galaxy: print structures built and growth too
	from     *save.File      // galaxy: start from this save (-load), nil to roll
	empires  int             // galaxy: empires on the cluster (-empires), the first of -species
	genome   *bot.Genome     // galaxy: every fresh bot's Config.Genome (-genome), nil: the fixed policy
	journal  *journal.Writer // galaxy: the action journal (-journal), nil none
}

func main() {
	species := flag.String("species", "Snovemdomas", "species, by name or index")
	seed := flag.Uint64("seed", 1, "home world seed")
	days := flag.Int("days", 200, "days to run")
	script := flag.String("script", "agriplot,laboratory,factory", "items to build first, in order")
	policy := flag.String("policy", "greedy", "after the script: greedy or none")
	research := flag.String("research", "auto", "auto, none, or a comma-separated tech queue (then auto)")
	quiet := flag.Bool("quiet", false, "only the final state and the events")
	galaxy := flag.Bool("galaxy", false, "run an empire on a cluster: colony ships, many colonies")
	density := flag.String("density", "average", "galaxy: cluster density")
	verbose := flag.Bool("v", false, "galaxy: print every event, structures and growth included")
	load := flag.String("load", "", "galaxy: play -days more of this save (its empires and bots)")
	empires := flag.Int("empires", 1, "galaxy: empires on the cluster, -species first, the others drawn from the seed")
	genome := flag.String("genome", "", "galaxy: every fresh bot plays this genome (cmd/fuzz -o)")
	journalPath := flag.String("journal", "", "galaxy: append every action taken (the bots') to this JSON Lines file (package journal)")
	flag.Parse()

	cfg, err := parseConfig(*species, *seed, *days, *script, *policy, *research, *quiet)
	if err == nil && *galaxy {
		cfg, err = galaxyConfig(cfg, *density, *verbose)
	}
	if err == nil {
		cfg.empires = *empires
		switch {
		case *empires < 1 || *empires > int(game.NumSpecies):
			err = fmt.Errorf("-empires %d: 1..%d", *empires, game.NumSpecies)
		case *empires > 1 && !*galaxy:
			err = fmt.Errorf("-empires needs -galaxy")
		case *galaxy && *load == "" && *empires > cfg.density.Stars():
			// game.NewGame panics past one home per star.
			err = fmt.Errorf("-empires %d: a %s cluster has %d stars", *empires, cfg.density, cfg.density.Stars())
		}
	}
	if err == nil {
		cfg, err = genomeConfig(cfg, *genome)
	}
	if err == nil && *load != "" {
		if !*galaxy {
			err = fmt.Errorf("-load needs -galaxy")
		} else {
			cfg.from, err = save.Read(*load)
		}
	}
	if err == nil && *journalPath != "" {
		if !*galaxy {
			err = fmt.Errorf("-journal needs -galaxy")
		} else {
			var jf *os.File
			if jf, err = os.OpenFile(*journalPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644); err == nil {
				defer jf.Close()
				cfg.journal = journal.New(jf)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "sim:", err)
		os.Exit(2)
	}
	run(os.Stdout, cfg)
}

// norm folds a name for matching: lower case, no spaces, underscores,
// hyphens or apostrophes.
func norm(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '_', '-', '\'':
			return -1
		}
		return r
	}, strings.ToLower(s))
}

func parseItem(s string) (game.Item, error) {
	n := norm(s)
	if n == "lab" {
		return game.Laboratory, nil
	}
	for it := game.Item(0); it < game.NumItems; it++ {
		if norm(it.String()) == n {
			return it, nil
		}
	}
	return game.NoItem, fmt.Errorf("unknown item %q", s)
}

func parseTech(s string) (game.Tech, error) {
	for t := game.Tech(0); t < game.NumTechs; t++ {
		if norm(t.String()) == norm(s) {
			return t, nil
		}
	}
	return game.NoTech, fmt.Errorf("unknown tech %q", s)
}

// list splits a comma list, dropping empty fields.
func list(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func parseConfig(species string, seed uint64, days int, script, policy, research string, quiet bool) (config, error) {
	cfg := config{seed: seed, days: days, quiet: quiet}
	var err error
	if cfg.species, err = game.ParseSpecies(species); err != nil {
		return cfg, err
	}
	for _, f := range list(script) {
		it, err := parseItem(f)
		if err != nil {
			return cfg, err
		}
		if !it.Surface() && !it.Orbital() {
			return cfg, fmt.Errorf("%s is a project: the colony builds structures only", it)
		}
		cfg.script = append(cfg.script, it)
	}
	switch policy {
	case "greedy":
		cfg.greedy = true
	case "none":
	default:
		return cfg, fmt.Errorf("policy %q: greedy or none", policy)
	}
	switch norm(research) {
	case "auto", "none":
		cfg.research = norm(research)
	default:
		cfg.research = "queue"
		for _, f := range list(research) {
			t, err := parseTech(f)
			if err != nil {
				return cfg, err
			}
			cfg.queue = append(cfg.queue, t)
		}
	}
	return cfg, nil
}

// sim is one run's state.
type sim struct {
	cfg    config
	col    *game.Colony
	r      *game.Research
	next   int         // script index
	queue  []game.Tech // research still queued
	events []dated
}

type dated struct {
	day int
	e   game.Event
}

// try starts it at its best place, or says why not.
func (s *sim) try(it game.Item) error { return bot.Try(s.col, s.r, it) }

// plan starts a project and picks a topic when idle; it returns why the
// script waits, "" when it does not.
func (s *sim) plan() string {
	var why string
	if s.col.Project == nil {
		if s.next < len(s.cfg.script) {
			if err := s.try(s.cfg.script[s.next]); err != nil {
				why = err.Error()
			} else {
				s.next++
			}
		} else if s.cfg.greedy {
			if it := bot.GreedyPick(s.col, s.r, bot.NeutralEconomy()); it != game.NoItem {
				if err := s.try(it); err != nil {
					panic(err) // greedyPick checked CanStart at the same cell
				}
			}
		}
	}
	bot.PickTopic(s.r, &s.queue, s.cfg.research == "none")
	return why
}

// run plays cfg.days days and writes the log to w. Deterministic: the
// game package rolls from the seed alone.
func run(w io.Writer, cfg config) {
	if cfg.galaxy {
		runGalaxy(w, cfg)
		return
	}
	name := cfg.species.String() + " Prime"
	p := cfg.species.HomePlanet(cfg.seed, name)
	s := &sim{cfg: cfg, col: game.NewLoneColony(p, cfg.species), r: game.NewResearch(), queue: cfg.queue}
	info := cfg.species.Info()
	fmt.Fprintf(w, "%s: %s size %d, seed %d, capacity %d\n\n", name, info.HomeType, info.HomeSize, cfg.seed, s.col.Capacity())

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if !cfg.quiet {
		fmt.Fprintln(tw, "day\tpop\tbusy/free/cap\tind\tres\tpros\tproject\ttopic\tevents")
	}
	for day := 1; day <= cfg.days; day++ {
		why := s.plan()
		// The line shows the day's output and the project before EndDay
		// spends it, then what the day produced.
		ind, res, pros := s.col.Output()
		project := "-"
		if pj := s.col.Project; pj != nil {
			project = fmt.Sprintf("%s(%d/%d)", projectName(pj), pj.Progress, projectCost(pj))
		} else if why != "" {
			project = "waiting: " + why
		}
		topic := "-"
		if t := s.r.Topic; t != game.NoTech {
			topic = fmt.Sprintf("%s(%d/%d)", t, s.r.Progress[t], s.r.Cost(t))
		}
		pop := fmt.Sprintf("%d/%d/%d", s.col.Busy(), s.col.Free(), s.col.Capacity())
		popN := s.col.Pop
		ev := s.col.EndDay(s.r, game.DigKey(cfg.seed, 0, day))
		var texts []string
		for _, e := range ev {
			s.events = append(s.events, dated{day, e})
			texts = append(texts, e.Text)
		}
		if cfg.quiet {
			continue // the summary lists the events
		}
		fmt.Fprintf(tw, "%d\t%d\t%s\t%d\t%d\t%d\t%s\t%s\t%s\n",
			day, popN, pop, ind, res, pros, project, topic, strings.Join(texts, "; "))
	}
	tw.Flush()
	summary(w, s)
}

// projectName is the project's item, "refit <ship>" for a refit (its
// Item is the Orbital Docks it occupies).
func projectName(pj *game.Project) string {
	if pj.Refit != "" {
		return "refit " + pj.Refit
	}
	return pj.Item.String()
}

// projectCost is the single colony's printed cost: the item's table cost
// as it always printed, a refit's own Cost.
func projectCost(pj *game.Project) int {
	if pj.Refit != "" {
		return pj.Cost()
	}
	return pj.Item.Info().Cost
}

// summary prints the final state: the colony, its built squares and
// orbit, the known techs, the events by day.
func summary(w io.Writer, s *sim) {
	c, p := s.col, s.col.Planet
	ind, res, pros := c.Output()
	fmt.Fprintf(w, "\nafter %d days: pop %d (busy %d, free %d, capacity %d, growth %d/%d), output %d/%d/%d\n",
		s.cfg.days, c.Pop, c.Busy(), c.Free(), c.Capacity(), c.Growth, game.GrowthCost, ind, res, pros)
	if pj := c.Project; pj != nil {
		fmt.Fprintf(w, "project: %s %d/%d\n", projectName(pj), pj.Progress, projectCost(pj))
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "\nsquare\tkind\titem")
	for _, cl := range p.Cells() {
		if sq := p.Surface[cl]; sq.Item != game.NoItem {
			fmt.Fprintf(tw, "(%d,%d)\t%s\t%s\n", cl.I, cl.J, sq.Kind, sq.Item)
		}
	}
	for slot, it := range p.Orbit {
		if it != game.NoItem {
			fmt.Fprintf(tw, "orbit %d\t\t%s\n", slot, it)
		}
	}
	tw.Flush()

	var known []string
	for t := game.Tech(0); t < game.NumTechs; t++ {
		if s.r.Known[t] {
			known = append(known, t.String())
		}
	}
	fmt.Fprintf(w, "\nknown (%d): %s\n", len(known), strings.Join(known, ", "))
	if t := s.r.Topic; t != game.NoTech {
		fmt.Fprintf(w, "studying: %s %d/%d\n", t, s.r.Progress[t], s.r.Cost(t))
	}

	fmt.Fprintln(w, "\nevents:")
	tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, d := range s.events {
		fmt.Fprintf(tw, "day %d\t%s\t%s\n", d.day, d.e.Kind, d.e.Text)
	}
	tw.Flush()
}

// galaxyConfig turns cfg into a galaxy run: the density, -v, and the
// guide's research queue in place of plain auto.
func galaxyConfig(cfg config, density string, verbose bool) (config, error) {
	d, err := game.ParseDensity(density)
	if err != nil {
		return cfg, err
	}
	cfg.galaxy, cfg.density, cfg.verbose = true, d, verbose
	if cfg.research == "auto" {
		cfg.research, cfg.queue = "queue", bot.GuideResearch
	}
	return cfg, nil
}

// genomeConfig reads the -genome file into cfg.genome: none for "",
// refused without -galaxy.
func genomeConfig(cfg config, path string) (config, error) {
	if path == "" {
		return cfg, nil
	}
	if !cfg.galaxy {
		return cfg, fmt.Errorf("-genome needs -galaxy")
	}
	g, err := bot.ReadGenome(path)
	cfg.genome = g
	return cfg, err
}

// galaxy is one game run's state; the policy lives in package bot, the
// rules in game.Game.
type galaxy struct {
	cfg  config
	g    *game.Game
	bots []*bot.Bot // one per empire, in order
}

// tag is the prefix of empire i's lines: none with one empire (the
// one-empire output predates -empires), else "[<Species>] ".
func (g *galaxy) tag(i int) string {
	if len(g.g.Empires) == 1 {
		return ""
	}
	return "[" + g.g.Empires[i].Species.String() + "] "
}

// pad is the width of the widest tag, for the status columns.
func (g *galaxy) pad() int {
	n := 0
	for i := range g.g.Empires {
		n = max(n, len(g.tag(i)))
	}
	return n
}

// runGalaxy plays cfg.days days of a game of cfg.empires empires, each
// day bot.Day (the fights' destructions printed before the day's
// events, stamped with the day that closes). Every 25 days a line of
// totals per empire; the events unless -quiet (ships built, colonies
// founded, discoveries, invasions; -v adds structures and growth); a
// summary per empire at the end. With several empires each line is
// prefixed "[<Species>] ". With cfg.from the game and the bots are the
// save's (a fresh bot from cfg for an empire it has none for) and the
// days run on from the save's.
func runGalaxy(w io.Writer, cfg config) *galaxy {
	bc := bot.Config{Script: cfg.script, Research: cfg.queue, Greedy: cfg.greedy, NoResearch: cfg.research == "none",
		Genome: cfg.genome}
	var gm *game.Game
	var bots []*bot.Bot
	seed := cfg.seed
	if f := cfg.from; f != nil {
		gm, seed = f.Game, f.Seed
		bots = make([]*bot.Bot, len(gm.Empires))
		for i, st := range f.Bots {
			if st != nil {
				bots[i] = bot.Resume(gm.Empires[i], st)
			}
		}
	} else {
		gm = game.NewGame(cfg.seed, game.PickSpecies(cfg.seed, cfg.species, max(cfg.empires, 1)), cfg.density)
		bots = make([]*bot.Bot, len(gm.Empires))
	}
	for i, b := range bots {
		if b == nil {
			bots[i] = bot.New(gm.Empires[i], bc)
		}
		bots[i].G = gm // the bots fight
	}
	gm.SetJournal(cfg.journal)
	g := &galaxy{cfg: cfg, g: gm, bots: bots}
	c := gm.Cluster
	for i, e := range gm.Empires {
		// The first colony: the home in a new game; in a loaded one it
		// may be another (the home captured) or none at all.
		if len(e.Colonies) == 0 {
			fmt.Fprintf(w, "%sno colony (%d stars, %s), seed %d\n", g.tag(i), len(c.Stars), c.Density, seed)
			continue
		}
		home := e.Colonies[0]
		fmt.Fprintf(w, "%s%s at %s (star %d of %d, %s), seed %d\n", g.tag(i), home.Planet.Name, c.Stars[home.Star].Name,
			home.Star, len(c.Stars), c.Density, seed)
	}
	if cfg.from != nil {
		fmt.Fprintf(w, "loaded on day %d\n", gm.Day)
	}
	fmt.Fprintf(w, "\n%*s%5s %8s %5s %5s %5s %8s %8s\n", g.pad(), "", "day", "colonies", "pop", "ships", "techs", "industry", "research")
	for range cfg.days {
		evs, reps, _ := bot.Day(gm, bots)
		if !cfg.quiet {
			for _, r := range reps {
				fmt.Fprintf(w, "day %d: %s%s\n", gm.Day, g.tag(r.Empire), r.Event.Text)
			}
			for i, ev := range evs {
				for _, x := range ev {
					if cfg.verbose || x.Kind == game.Discovered || x.Kind == game.Invaded || x.Kind == game.Repelled ||
						x.Kind == game.Built && (x.Item == game.Ship || x.Item == game.ColonyBase) {
						fmt.Fprintf(w, "day %d: %s%s\n", gm.Day, g.tag(i), x.Text)
					}
				}
			}
		}
		if gm.Day%25 == 0 {
			for i := range gm.Empires {
				g.status(w, i)
			}
		}
	}
	for i := range gm.Empires {
		g.summary(w, i)
	}
	return g
}

// status prints empire k's totals line.
func (g *galaxy) status(w io.Writer, k int) {
	e := g.g.Empires[k]
	pop, ind, res := 0, 0, 0
	for _, col := range e.Colonies {
		i, r, _ := col.Output()
		pop, ind, res = pop+col.Pop, ind+i, res+r
	}
	fmt.Fprintf(w, "%-*s%5d %8d %5d %5d %5d %8d %8d\n", g.pad(), g.tag(k), e.Day, len(e.Colonies), pop, len(e.Ships), known(e.Research), ind, res)
}

func known(r *game.Research) int {
	n := 0
	for _, k := range r.Known {
		if k {
			n++
		}
	}
	return n
}

// summary prints empire k's colonies (founded: the bot's founding day,
// c<day> for a colony captured that day), ships and known techs, headed
// "== <Species> ==" when there are several empires.
func (g *galaxy) summary(w io.Writer, k int) {
	e, c, b := g.g.Empires[k], g.g.Cluster, g.bots[k]
	if len(g.g.Empires) > 1 {
		fmt.Fprintf(w, "\n== %s ==\n", e.Species)
	}
	fmt.Fprintf(w, "\nafter %d days: %d colonies in %d systems, %d ships\n", e.Day, len(e.Colonies), e.ControlledSystems(), len(e.Ships))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "\nstar\tplanet\ttype\tsize\tfounded\tpop\tstructures\tproject")
	for _, col := range e.Colonies {
		n := 0
		for _, sq := range col.Planet.Surface {
			if sq.Item != game.NoItem {
				n++
			}
		}
		for _, it := range col.Planet.Orbit {
			if it != game.NoItem {
				n++
			}
		}
		project := "-"
		if pj := col.Project; pj != nil {
			project = fmt.Sprintf("%s(%d/%d)", projectName(pj), pj.Progress, pj.Cost())
		}
		founded := fmt.Sprint(b.Founded[col])
		if day, ok := b.Captured[col]; ok {
			founded = fmt.Sprintf("c%d", day) // captured that day
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%d\t%d\t%s\n", c.Stars[col.Star].Name, col.Planet.Name, col.Planet.Type,
			col.Planet.Size, founded, col.Pop, n, project)
	}
	tw.Flush()

	tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "\nship\tdesign\tcargo\twhere")
	for _, s := range e.Ships {
		where := "space at " + c.Stars[s.Star].Name
		switch {
		case s.Lane >= 0:
			where = fmt.Sprintf("lane %s -> %s, out day %d", c.Stars[s.Star].Name, c.Stars[c.Other(s.Lane, s.Star)].Name, s.Arrive)
		case s.Orbit >= 0:
			where = "orbit of " + e.Planet(s.Star, s.Orbit).Name
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", s.Name, s.Design, s.Cargo, where)
	}
	tw.Flush()

	var names []string
	for t := game.Tech(0); t < game.NumTechs; t++ {
		if e.Research.Known[t] {
			names = append(names, t.String())
		}
	}
	fmt.Fprintf(w, "\nknown (%d): %s\n", len(names), strings.Join(names, ", "))
	if t := e.Research.Topic; t != game.NoTech {
		fmt.Fprintf(w, "studying: %s %d/%d\n", t, e.Research.Progress[t], e.Research.Cost(t))
	}
}
