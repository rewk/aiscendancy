package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// speciesApp is seed 1 with three empires, the player having met empire
// 1 only (Game.Contact), the species screen pushed over the starmap.
func speciesApp(t *testing.T) (*app, *speciesScreen) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
	a := testApp(g)
	a.push(newStarmapScreen())
	a.record(g.Contact(0, 1))
	s := newSpeciesScreen(-1)
	a.push(s)
	return a, s
}

// rowClick clicks row r's middle, left of the columns (on the flag).
func rowClick(a *app, s *speciesScreen, r int) {
	s.click(a, float64(mapArea.X0+padX+4), speciesRowY(r)+float64(speciesRowH)/2)
}

// One row, empire 1's, read from the game: no attitude column (a bot's
// mood is cheating) even with a bot; no offer of theirs pending, "".
func TestSpeciesRows(t *testing.T) {
	a, s := speciesApp(t)
	if m := met(a); !slices.Equal(m, []int{1}) {
		t.Fatalf("met %v, want [1]", m)
	}
	o := a.g.Empires[1]
	a.bots = make([]*bot.Bot, len(a.g.Empires))
	a.bots[1] = bot.New(o, bot.Default())
	want := [len(speciesCols)]string{o.Species.String(), "peace", "0", "0", "day 0", "0", ""}
	if got := speciesLine(a, 1); got != want {
		t.Errorf("line %q, want %q", got, want)
	}
	if slices.Contains(speciesHeads[:], "attitude") {
		t.Errorf("heads %q", speciesHeads)
	}
	if j := s.empire(a, s.row(speciesRowY(0)+1)); j != 1 {
		t.Errorf("row 0: empire %d", j)
	}
	if j := s.empire(a, s.row(speciesRowY(1)+1)); j != -1 {
		t.Errorf("row 1: empire %d, want none", j)
	}
	if s.selected(a) != 1 {
		t.Errorf("selected %d, want the first row's", s.selected(a))
	}
	// A known colony counts as a system once the player knows its star.
	a.e.Visit(o.Colonies[0].Star)
	if n := knownSystems(a, 1); n != 1 {
		t.Errorf("systems %d after the visit", n)
	}
	sc := ebiten.NewImage(screenW, screenH)
	s.Text(a, sc)
}

// Line 1's last column is their oldest pending offer to us in words (one
// of ours to them is not theirs); a request names its third.
func TestSpeciesRowOffer(t *testing.T) {
	a, _ := speciesApp(t)
	g := a.g
	a.record(g.Contact(0, 2))
	g.Contact(1, 2)
	if _, err := g.OfferAlliance(0, 1); err != nil {
		t.Fatal(err)
	}
	if got := speciesLine(a, 1)[speciesOfferCol]; got != "" {
		t.Errorf("our offer shown as theirs: %q", got)
	}
	g.Offers = nil
	if _, err := g.OfferAlliance(1, 0); err != nil {
		t.Fatal(err)
	}
	if got := speciesLine(a, 1)[speciesOfferCol]; got != "offers an alliance" {
		t.Errorf("alliance offer %q", got)
	}
	if _, err := g.Answer(g.Offers[0], true); err != nil {
		t.Fatal(err)
	}
	g.Day += game.OfferCooldown
	if _, err := g.Request(1, 0, game.AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	if got := speciesLine(a, 1)[speciesOfferCol]; got != "asks war on the Mebes" {
		t.Errorf("request %q", got)
	}
	for k, want := range map[game.OfferKind]string{game.PeaceOffer: "offers peace", game.TradeStars: "offers an exchange of stars",
		game.TradeLanes: "offers an exchange of lanes", game.TradeResearch: "offers an exchange of research"} {
		if got := theirOfferWords(a, game.Offer{From: 1, To: 0, Kind: k, Third: -1}); got != want {
			t.Errorf("%s: %q, want %q", k, got, want)
		}
	}
}

// Line 2 is the Relation's counts on one line, the hospitality we gave
// read from their ledger.
func TestSpeciesLedgerLine(t *testing.T) {
	a, _ := speciesApp(t)
	r := a.e.Rel(1)
	r.WarDays, r.WarsOn, r.ShipsLost, r.PlanetsLost, r.Intrusions, r.Refused = 7, 1, 1, 2, 46, 5
	r.AllyDays, r.Broken, r.Trades, r.Hospitality, r.Helped = 13, 2, 1, 171, 4
	a.g.Empires[1].Rel(0).Hospitality = 10032
	want := "war 7 days · wars on us 1 · ships lost 1 · colonies lost 2 · intrusions 46 · refused 5 · " +
		"allied 13 days · broke 2 · exchanges 1 · hospitality in 171 / out 10032 · granted 4"
	if got := ledgerLine(a, 1); got != want {
		t.Errorf("ledger\n%q, want\n%q", got, want)
	}
	// Their hospitality barring our war: the closes left, last.
	r.Friendship = 55
	if got := ledgerLine(a, 1); got != want+" · no war for 10 days" {
		t.Errorf("barred ledger %q", got)
	}
	r.Stance, a.g.Empires[1].Rel(0).Stance = game.War, game.War // they declared it
	if got := ledgerLine(a, 1); got != want {
		t.Errorf("at war: ledger %q", got)
	}
	r.Stance, a.g.Empires[1].Rel(0).Stance = game.Peace, game.Peace
	// Big counts still fit the row's lines whole, broken between facts,
	// and the lines inside the row under line 1.
	r.WarDays, r.Intrusions, r.Hospitality, r.AllyDays = 99999, 99999, 999999, 99999
	a.g.Empires[1].Rel(0).Hospitality = 999999
	r.Friendship = 999999
	w := speciesLedgerW()
	ls := ledgerLines(a, 1, w)
	if len(ls) > speciesLedgerRows || strings.Join(ls, ledgerSep) != ledgerLine(a, 1) {
		t.Errorf("lines %q, want %d at most joining to the ledger", ls, speciesLedgerRows)
	}
	for _, l := range ls {
		if ui.Width(l, bigFace) > w {
			t.Errorf("%q wider than %.0f", l, w)
		}
	}
	if bottom := float64(speciesTextDY) + (2+speciesLedgerRows)*logLineH; bottom > speciesRowH {
		t.Errorf("ledger bottom %.1f past the row's %d", bottom, speciesRowH)
	}
	if float64(speciesRowH) < float64(flagH*zoom+2*speciesRowPad) {
		t.Errorf("row %d shorter than the flag", speciesRowH)
	}
	if bottom := speciesRowY(speciesMaxRows); bottom > float64(mapArea.Y1-padY)-logLineH {
		t.Errorf("rows end at %.0f, over the flash line", bottom)
	}
}

// A click on a row opens Talk with its empire (framed); a click past the
// rows opens nothing.
func TestSpeciesClickTalks(t *testing.T) {
	a, s := speciesApp(t)
	a.record(a.g.Contact(0, 2))
	n := len(a.stack)
	rowClick(a, s, 2)
	if len(a.stack) != n || s.sel != -1 {
		t.Fatalf("a click past the rows: stack %d -> %d, sel %d", n, len(a.stack), s.sel)
	}
	rowClick(a, s, 1)
	if ns := top[*negotiateScreen](t, a); ns.j != 2 || s.sel != 2 {
		t.Errorf("talking to %d, sel %d, want 2", ns.j, s.sel)
	}
}

// No contact: no row, no selection; Text says so.
func TestSpeciesNoneMet(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	s := newSpeciesScreen(-1)
	a.push(s)
	if len(met(a)) != 0 || s.selected(a) != -1 {
		t.Errorf("met %v, selected %d", met(a), s.selected(a))
	}
	s.Text(a, ebiten.NewImage(screenW, screenH))
}

// The log: a diplomatic line is a link to the species screen, its empire
// selected. Key S pushes the screen from the starmap, the log, the fleet
// and the planets list.
func TestSpeciesLinks(t *testing.T) {
	a, _ := speciesApp(t)
	a.pop()
	reps, err := a.g.DeclareWar(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	a.record(reps)
	i := len(a.log) - 1
	ev := a.log[i].Ev
	if ev.Kind != game.WarDeclared || !linked(ev) {
		t.Fatalf("log %+v, linked %v", ev, linked(ev))
	}
	scs := (&logScreen{}).open(a, i)
	if len(scs) != 1 {
		t.Fatalf("opens %#v", scs)
	}
	if sp, ok := scs[0].(*speciesScreen); !ok || sp.sel != 1 {
		t.Errorf("opens %#v, want the species screen on 1", scs[0])
	}
	for _, sc := range []screen{newStarmapScreen(), newLogScreen(), newFleetScreen(), newPlanetsScreen()} {
		a.stack = nil
		a.push(sc)
		if onSpeciesKey(a, false) || len(a.stack) != 1 {
			t.Fatalf("%T: pushed with no key", sc)
		}
		if !onSpeciesKey(a, true) {
			t.Fatalf("%T: S refused", sc)
		}
		if sp := top[*speciesScreen](t, a); sp.sel != -1 {
			t.Errorf("%T: sel %d", sc, sp.sel)
		}
	}
}

// Render and Text with the game's art: the war, their offer pending, a
// flash.
func TestSpeciesDraws(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	a, s := speciesApp(t)
	a.a = as
	if _, err := a.g.DeclareWar(0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := a.g.OfferPeace(1, 0); err != nil {
		t.Fatal(err)
	}
	s.flash("a refusal")
	sc := ebiten.NewImage(screenW, screenH)
	s.Render(a, sc)
	s.Text(a, sc)
}

// Alien Hospitality's foot in the project panel and the planets list:
// "ongoing: +N hospitality", N game's Diverted share.
func TestHospitalityFoot(t *testing.T) {
	a, _, col := ongoingHome(t)
	a.e.Research.Grant(game.Tech(game.AlienHospitality.Info().ResearchReq))
	if err := col.StartProject(a.e.Research, game.AlienHospitality); err != nil {
		t.Fatal(err)
	}
	it, n := col.Diverted()
	want := fmt.Sprintf("ongoing: +%d hospitality", n)
	var pp projPanel
	pp.fill(col, "idle")
	if it != game.AlienHospitality || pp.foot != want {
		t.Errorf("panel %v %q, want %q", it, pp.foot, want)
	}
	if _, _, _, _, foot := colonyLines(a.e, a.cl, col); foot != want {
		t.Errorf("planets list %q", foot)
	}
}

// abilityApp is speciesApp with the player of species sp, at war with
// empire 1 when war.
func abilityApp(t *testing.T, sp game.Species, war bool) (*app, *speciesScreen) {
	t.Helper()
	g := game.NewGame(1, []game.Species{sp, game.Govorom, game.Capelons}, game.Average)
	charge(g, 0)
	a := testApp(g)
	a.push(newStarmapScreen())
	a.record(g.Contact(0, 1))
	if war {
		reps, err := g.DeclareWar(1, 0)
		if err != nil {
			t.Fatal(err)
		}
		a.record(reps)
	}
	s := newSpeciesScreen(-1)
	a.push(s)
	return a, s
}

// pressAbility clicks the ability button's centre.
func pressAbility(a *app, s *speciesScreen) {
	_, r, _, _ := s.ability(a)
	s.click(a, float64(r.X0+r.X1)/2, float64(r.Y0+r.Y1)/2)
}

// The Baliflids at war: "ready", pressed it forces peace (the game's
// Reports logged), then "in 100 days" dimmed and a second press flashes
// the game's refusal.
func TestSpeciesAbilityBaliflids(t *testing.T) {
	a, s := abilityApp(t, game.Baliflids, true)
	info := game.Baliflids.Info()
	if ls, _, dim, _ := s.ability(a); ls != [2]string{info.AbilityText, "ready"} || dim {
		t.Fatalf("label %q dim %v", ls, dim)
	}
	n := len(a.log)
	pressAbility(a, s)
	if a.e.StanceWith(1) != game.Peace || a.g.Empires[1].StanceWith(0) != game.Peace || s.msg != "" {
		t.Fatalf("after the use: %v, flash %q", a.e.StanceWith(1), s.msg)
	}
	if len(a.log) <= n || a.log[n].Ev.Kind != game.AbilityUsed {
		t.Errorf("log %+v", a.log[n:])
	}
	if ls, _, dim, _ := s.ability(a); ls[1] != fmt.Sprintf("in %d days", info.Recharge) || !dim {
		t.Errorf("label %q dim %v", ls, dim)
	}
	pressAbility(a, s)
	if s.msg != fmt.Sprintf("ready in %d days", info.Recharge) {
		t.Errorf("flash %q", s.msg)
	}
}

// The Orfa's ability is passive: dimmed, a press does nothing. The
// Mebes': used, the capacity grows.
func TestSpeciesAbilityPassive(t *testing.T) {
	a, s := abilityApp(t, game.Orfa, false)
	if ls, _, dim, passive := s.ability(a); ls[1] != "passive" || !dim || !passive {
		t.Fatalf("label %q dim %v passive %v", ls, dim, passive)
	}
	n := len(a.log)
	pressAbility(a, s)
	if s.msg != "" || len(a.log) != n || a.e.AbilityUsed != 0 {
		t.Errorf("passive press: flash %q, log %d -> %d", s.msg, n, len(a.log))
	}
	a, s = abilityApp(t, game.Mebes, false) // a timed one: used, the capacity grows
	capa := a.e.Colonies[0].Capacity()
	pressAbility(a, s)
	if s.msg != "" || a.e.AbilityUsed != a.g.Day || a.e.Colonies[0].Capacity() != capa+game.MebesPopBonus {
		t.Errorf("Mebes: flash %q, used %d, capacity %d -> %d", s.msg, a.e.AbilityUsed, capa, a.e.Colonies[0].Capacity())
	}
	sc := ebiten.NewImage(screenW, screenH)
	s.Text(a, sc)
}

// For every species the ability's text fits its stacked box: at most
// abilityTextRows smallFace lines joining to it, then the state's line.
func TestSpeciesAbilityPlace(t *testing.T) {
	for sp := range game.NumSpecies {
		// The longest name met (Snovemdomas) unless it is ours.
		others := []game.Species{game.Snovemdomas, game.Chamachies}
		if sp == game.Snovemdomas || sp == game.Chamachies {
			others = []game.Species{game.Swaparamans, game.Chronomyst}
		}
		g := game.NewGame(1, append([]game.Species{sp}, others...), game.Average)
		a := testApp(g)
		a.record(g.Contact(0, 1))
		s := newSpeciesScreen(-1)
		ls, r, _, _ := s.ability(a)
		if r != mapStack[abilityStack] {
			t.Errorf("%s: %v", sp, r)
		}
		tl := abilityWords(ls[0])
		if strings.Join(tl, " ") != ls[0] {
			t.Errorf("%s: %q cut", sp, tl)
		}
		if bottom := float64(r.Y0+padY) + float64(len(tl))*smallFace.Size*1.25 + bigFace.Size; bottom > float64(r.Y1) {
			t.Errorf("%s: state line at %.0f past the box's %d", sp, bottom, r.Y1)
		}
	}
}

// A defeated empire's row reads "defeated"; a click on it opens no Talk
// (out of the game, nothing to say) but flashes why.
func TestSpeciesDefeated(t *testing.T) {
	a, s := speciesApp(t)
	a.g.Empires[1].Lost = a.e.Day + 1
	if l := speciesLine(a, 1); l[1] != "defeated" {
		t.Errorf("stance column %q", l[1])
	}
	n := len(a.stack)
	rowClick(a, s, 0)
	if len(a.stack) != n || top[*speciesScreen](t, a) != s || s.msg == "" || s.tick >= s.msgUntil {
		t.Errorf("click on the defeated: stack %d -> %d, flash %q", n, len(a.stack), s.msg)
	}
	s.Text(a, ebiten.NewImage(screenW, screenH)) // the greyed row (Render's flag needs the game's art)
}

// Allied with empire 1, the ledger opens on its wars and alliances: a
// met empire named, an unmet one counted, a defeated one left out; at
// peace, nothing of theirs.
func TestSpeciesAllyFacts(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes, game.Govorom, game.Shevar}, game.Average)
	a := testApp(g)
	a.push(newStarmapScreen())
	stance := func(i, j int, s game.Stance) { g.Empires[i].Rel(j).Stance, g.Empires[j].Rel(i).Stance = s, s }
	stance(0, 1, game.Peace)
	stance(0, 2, game.Peace)
	stance(1, 2, game.War)
	stance(1, 3, game.War)
	stance(1, 4, game.Alliance)
	g.Empires[4].Lost = 1
	if f := allyFacts(a, 1, 0); f != nil {
		t.Errorf("at peace: %q", f)
	}
	stance(0, 1, game.Alliance)
	want := []string{"at war with the Mebes and 1 unmet", "allied with none"}
	if f := allyFacts(a, 1, 0); !slices.Equal(f, want) {
		t.Errorf("allied: %q, want %q", f, want)
	}
	if l := ledgerLine(a, 1); !strings.HasPrefix(l, want[0]+ledgerSep+want[1]+ledgerSep+"war ") {
		t.Errorf("ledger %q", l)
	}
	stance(0, 3, game.Peace)
	if got := empireList(a, []int{2, 3}); got != "the Mebes and the Govorom" {
		t.Errorf("list %q", got)
	}
	g.Empires[1].Lost = 1
	if f := allyFacts(a, 1, 0); f != nil {
		t.Errorf("defeated ally: %q", f)
	}
}

// A ledger running over its rows drops whole facts and ends in
// ledgerMore: no number cut short.
func TestSpeciesLedgerCut(t *testing.T) {
	a, _ := speciesApp(t)
	r := a.e.Rel(1)
	r.WarDays, r.Intrusions, r.Hospitality, r.AllyDays, r.Refused = 99999, 99999, 999999, 99999, 99999
	a.g.Empires[1].Rel(0).Hospitality = 999999
	w := speciesLedgerW() / 2
	ls := ledgerLines(a, 1, w)
	if len(ls) != speciesLedgerRows {
		t.Fatalf("lines %q, want %d", ls, speciesLedgerRows)
	}
	last := ls[len(ls)-1]
	if !strings.HasSuffix(last, ledgerMore) {
		t.Fatalf("last line %q, want ending %q", last, ledgerMore)
	}
	facts := ledgerFacts(a, 1, w)
	for _, l := range ls {
		if ui.Width(l, bigFace) > w {
			t.Errorf("%q wider than %.0f", l, w)
		}
		for _, f := range strings.Split(strings.TrimSuffix(l, ledgerMore), ledgerSep) {
			if !slices.Contains(facts, f) {
				t.Errorf("fact %q cut", f)
			}
		}
	}
}

// An ally at war with every other empire of a 7-empire game (the new
// game's most), the longest names, all met: no line wider than the
// ledger, no name cut, the names left out counted.
func TestSpeciesAllyFactsFit(t *testing.T) {
	var sp []game.Species
	for s := range game.NumSpecies {
		sp = append(sp, s)
	}
	slices.SortStableFunc(sp, func(x, y game.Species) int { return len(y.String()) - len(x.String()) })
	g := game.NewGame(1, sp[:maxEmpires], game.VeryDense)
	a := testApp(g)
	a.push(newStarmapScreen())
	stance := func(i, j int, s game.Stance) { g.Empires[i].Rel(j).Stance, g.Empires[j].Rel(i).Stance = s, s }
	stance(0, 1, game.Alliance)
	for k := 2; k < maxEmpires; k++ {
		stance(0, k, game.Peace)
		stance(1, k, game.War)
	}
	names := map[string]bool{}
	for _, s := range sp {
		names["the "+s.String()] = true
	}
	w := speciesLedgerW()
	ls := ledgerLines(a, 1, w)
	counted := 0
	for _, l := range ls {
		if ui.Width(l, bigFace) > w {
			t.Errorf("%q wider than %.0f", l, w)
		}
		for _, f := range strings.Split(strings.TrimSuffix(l, ledgerMore), ledgerSep) {
			list, ok := strings.CutPrefix(f, "at war with ")
			if !ok {
				continue
			}
			for _, x := range strings.Split(strings.ReplaceAll(list, " and ", ", "), ", ") {
				var n int
				if _, err := fmt.Sscanf(x, "%d more", &n); err == nil {
					counted += n
					continue
				}
				if !names[x] {
					t.Errorf("war list item %q cut (line %q)", x, l)
				} else {
					counted++
				}
			}
		}
	}
	if counted != maxEmpires-2 {
		t.Errorf("%d enemies told, want %d: %q", counted, maxEmpires-2, ls)
	}
}
