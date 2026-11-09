package main

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
)

// roundApp is a fight at the player's home the player survives: its
// Gunboat (full hull, Ion Wrap on) and empire 1's Raider (its bot never
// flees), the system screen of the home over a starmap; the Gunboat is
// returned with a far point order given through the screen.
func roundApp(t *testing.T) (a *app, v *systemScreen, gun *game.Starship) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	if err := game.SpawnFixtures(g, 0, 1, 1); err != nil {
		t.Fatal(err)
	}
	cfg := bot.Default()
	cfg.Genome = bot.Neutral()
	cfg.Genome.FleeRatio = 1e9
	a = testApp(g)
	a.bots = bot.NewAll(g, cfg)
	a.bots[0] = nil
	a.savePath = filepath.Join(t.TempDir(), "quick.json")
	a.push(newStarmapScreen())
	v = newSystemScreen(a.e.Colonies[0].Star)
	a.push(v)
	gun = a.e.Ships[0]
	v.sel = gun
	return a, v, gun
}

// A destination drops the ship out of combat, Repeat Move then hopping
// the last one ordered; X keeps it; in combat mode the ship stays
// selected for the rounds.
func TestOrderDrops(t *testing.T) {
	a, v, gun, _ := peaceApp(t)
	v.sel = gun
	if err := v.order(a.e, farPoint(gun)); err != nil || v.sel != nil || v.last != gun {
		t.Fatalf("peace: %v sel %v last %v", err, v.sel, v.last)
	}
	v.anim = anim{}
	at := gun.Pos
	if err := v.repeat(a.e); err != nil || gun.Pos == at {
		t.Errorf("Repeat Move without a selection: %v, moved %v", err, gun.Pos != at)
	}
	v.sel = gun
	if err := v.order(a.e, game.Order{}); err != nil || v.sel != gun {
		t.Errorf("X: %v sel %v", err, v.sel)
	}

	a, v, gun = roundApp(t)
	if !v.inCombat(a) {
		t.Fatal("fixture: not in combat")
	}
	if err := v.order(a.e, farPoint(gun)); err != nil || v.sel != gun {
		t.Errorf("combat: %v sel %v", err, v.sel)
	}
}

// farPoint is a point order a hop or two away from sh, in reach.
func farPoint(sh *game.Starship) game.Order {
	return game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: -sh.Pos.X * 0.5, Y: 0.3, Z: -sh.Pos.Z * 0.5}}
}

func vdist(p, q game.Vec3) float64 { return math.Hypot(math.Hypot(p.X-q.X, p.Y-q.Y), p.Z-q.Z) }

// peaceApp is roundApp's two ships at the player's home with no war
// declared (no contact): no fight, the system screen of the home over a
// starmap. The Raider is put beside the Gunboat, in space.
func peaceApp(t *testing.T) (a *app, v *systemScreen, gun, foe *game.Starship) {
	t.Helper()
	g := game.NewGame(2, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	if err := game.SpawnFixtures(g, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	a = testApp(g)
	a.push(newStarmapScreen())
	home := a.e.Colonies[0]
	v = newSystemScreen(home.Star)
	a.push(v)
	gun = a.e.Ships[0]
	foe = g.Empires[1].Spawn(game.FixtureRaider, home.Star, home.PlanetIdx)
	foe.Orbit, foe.Pos = -1, game.Vec3{X: gun.Pos.X + 0.2, Y: gun.Pos.Y, Z: gun.Pos.Z}
	return a, v, gun, foe
}

// Combat mode is derived: a system screen at a fight is in it at once,
// with no day run; one at peace is not, and becomes so the moment war is
// declared; RoundCap rounds played there today end it. The starmap's
// fightStars follows.
func TestCombatModeDerived(t *testing.T) {
	a, v, _ := roundApp(t)
	if !v.inCombat(a) || !slices.Equal(fightStars(a), []int{v.star}) {
		t.Fatalf("at the fight: combat %v, fights %v", v.inCombat(a), fightStars(a))
	}
	a.g.RoundsToday = make([]int, len(a.cl.Stars))
	a.g.RoundsToday[v.star] = bot.RoundCap - 1
	if !v.inCombat(a) {
		t.Error("a round left: no combat mode")
	}
	a.g.RoundsToday[v.star] = bot.RoundCap
	if v.inCombat(a) || len(fightStars(a)) != 0 {
		t.Errorf("RoundCap rounds played: combat %v, fights %v", v.inCombat(a), fightStars(a))
	}

	a, v, _, _ = peaceApp(t)
	if v.inCombat(a) || len(fightStars(a)) != 0 {
		t.Fatalf("at peace: combat %v, fights %v", v.inCombat(a), fightStars(a))
	}
	atWar(t, a.g, 0, 1)
	if !v.inCombat(a) || !slices.Equal(fightStars(a), []int{v.star}) {
		t.Errorf("war declared: combat %v, fights %v", v.inCombat(a), fightStars(a))
	}
}

// In combat mode the left button plays one round there and never ends
// the day: the Gunboat, ordered before (no hand hop where engaged), hops
// once and pays for it, the round count goes up by one, the toast names
// the round just played. Repeat Move and Auto stay refused.
func TestCombatStepButton(t *testing.T) {
	a, v, gun := roundApp(t)
	p0 := gun.Pos
	if err := v.order(a.e, farPoint(gun)); err != nil || gun.Pos != p0 || v.anim.ship != nil {
		t.Fatalf("order at the fight: %v, moved %v, anim %v", err, gun.Pos != p0, v.anim.ship)
	}
	day, power, reach := a.e.Day, gun.Power, gun.Design.Reach()
	v.press(a, endTurnButton)
	d := vdist(p0, gun.Pos)
	if a.e.Day != day || a.g.RoundsAt(v.star) != 1 || d == 0 || d > reach+1e-9 || gun.Power >= power {
		t.Fatalf("step: day %d of %d, rounds %d, moved %.3f (reach %.3f), power %d of %d",
			a.e.Day, day, a.g.RoundsAt(v.star), d, reach, gun.Power, power)
	}
	if !strings.HasPrefix(a.toast, "Round 1: ") || v.replay == nil {
		t.Errorf("toast %q, replay %v", a.toast, v.replay != nil)
	}
	at := gun.Pos
	for _, b := range []int{repeatButton, autoButton} {
		a.toast = ""
		if v.press(a, b); a.toast != roundsMove || gun.Pos != at {
			t.Errorf("button %d in combat mode: toast %q, moved %v", b, a.toast, gun.Pos != at)
		}
	}
	// The left button over the step's replay skips it, no round played;
	// then the next round.
	v.press(a, endTurnButton)
	if v.replay != nil || a.g.RoundsAt(v.star) != 1 {
		t.Fatalf("over the replay: replay %v, rounds %d", v.replay != nil, a.g.RoundsAt(v.star))
	}
	v.press(a, endTurnButton)
	if a.g.RoundsAt(v.star) != 2 || !strings.HasPrefix(a.toast, "Round 2: ") || a.e.Day != day {
		t.Errorf("second step: rounds %d, toast %q, day %d", a.g.RoundsAt(v.star), a.toast, a.e.Day)
	}
	// At RoundCap the left button says no round is left, not that the
	// rounds move.
	v.replay = nil
	a.g.RoundsToday[v.star] = bot.RoundCap
	if v.press(a, endTurnButton); a.toast != noRoundLeft || a.g.RoundsAt(v.star) != bot.RoundCap {
		t.Errorf("at RoundCap: toast %q, rounds %d", a.toast, a.g.RoundsAt(v.star))
	}
}

// A step's news is logged at once, stamped today: combatApp's Gunboat
// (hull at one shot, shields off) is lost in the round stepped.
func TestCombatStepLogs(t *testing.T) {
	a, v, _ := combatApp(t)
	n, day := len(a.log), a.e.Day
	v.press(a, endTurnButton)
	if a.e.Day != day || len(a.log) == n || len(a.e.Ships) != 0 {
		t.Fatalf("day %d of %d, %d logged, %d ships left", a.e.Day, day, len(a.log)-n, len(a.e.Ships))
	}
	for _, l := range a.log[n:] {
		if l.Day != day {
			t.Errorf("%q logged on day %d, want %d", l.Ev.Text, l.Day, day)
		}
	}
	if !strings.HasPrefix(a.toast, "Round 1: ") || !strings.HasSuffix(a.toast, "lost 1") {
		t.Errorf("toast %q", a.toast)
	}
}

// Space at a fight's screen steps one round, through the app's Update
// (the screen's own Update reads no Space: one round a press), and never
// ends the day; a press over the step's replay skips it.
func TestCombatSpace(t *testing.T) {
	a, v, gun := roundApp(t)
	v.order(a.e, farPoint(gun))
	day := a.e.Day
	for k, want := range []int{1, 1, 2} {
		pressing(t, ebiten.KeySpace)
		if a.Update(); a.g.RoundsAt(v.star) != want || a.e.Day != day || a.warn != nil {
			t.Fatalf("press %d: rounds %d, want %d; day %d of %d, warning %v", k, a.g.RoundsAt(v.star), want, a.e.Day, day, a.warn != nil)
		}
	}
}

// The left button out of combat hops every ordered ship here once, one
// anim after another, and the day stays.
func TestHopAll(t *testing.T) {
	a, v, gun, foe := peaceApp(t)
	foe.Pos = game.Vec3{X: -gun.Pos.X, Y: gun.Pos.Y, Z: -gun.Pos.Z} // out of the way
	e := a.e
	b := e.Spawn(gun.Design, gun.Star, 0)
	b.Orbit, b.Pos = -1, game.Vec3{X: gun.Pos.X, Y: gun.Pos.Y + 0.3, Z: gun.Pos.Z}
	idle := e.Spawn(gun.Design, gun.Star, 0) // no order: never hops
	idle.Orbit, idle.Pos = -1, game.Vec3{X: gun.Pos.X, Y: gun.Pos.Y - 0.3, Z: gun.Pos.Z}
	far := func(sh *game.Starship) game.Order { // several hops away
		return game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: sh.Pos.X - 1.2, Y: sh.Pos.Y, Z: sh.Pos.Z}}
	}
	for _, sh := range []*game.Starship{gun, b} {
		if err := e.Order(sh, far(sh)); err != nil {
			t.Fatal(err)
		}
	}
	p, q, r := gun.Pos, b.Pos, idle.Pos
	day := e.Day
	v.press(a, endTurnButton)
	if gun.Pos == p || b.Pos != q || v.anim.ship != gun {
		t.Fatalf("press: first moved %v, second %v, anim %v", gun.Pos != p, b.Pos != q, v.anim.ship != nil)
	}
	for range 3 * hopTicks {
		v.tick++
		v.advance(e)
	}
	if b.Pos == q || idle.Pos != r || vdist(p, gun.Pos) > gun.Design.Reach()+1e-9 || e.Day != day {
		t.Errorf("after the run: second moved %v, idle moved %v, first went %.3f, day %d of %d",
			b.Pos != q, idle.Pos != r, vdist(p, gun.Pos), e.Day, day)
	}
	if v.anim.ship != nil || len(v.pending) != 0 {
		t.Errorf("the run did not end: anim %v, pending %d", v.anim.ship != nil, len(v.pending))
	}
	for _, sh := range []*game.Starship{gun, b} {
		sh.Order = game.Order{}
	}
	a.toast = ""
	if v.press(a, endTurnButton); a.toast != "nothing to move" || e.Day != day {
		t.Errorf("nothing ordered: toast %q, day %d", a.toast, e.Day)
	}
}

// warnLine is the centre of the warning's line k (1: the first star).
func warnLine(k int) (float64, float64) {
	b, _, _ := quitPrompt.rects()
	return float64(b.X0+b.X1) / 2, float64(b.Y0+dlgTitleY+zoom*promptLineH*k) + zoom*promptLineH/2
}

// Ending the day from the starmap with a ship of energy at a fight warns
// first, naming the star: Cancel keeps the day, End the day closes it, a
// click on the star's line opens its system screen; Enter through the
// app's Update answers yes. No warning once the ship has no usable
// energy, nor where no fight is.
func TestDayWarning(t *testing.T) {
	a, v, gun := roundApp(t)
	a.pop() // the starmap on top
	day := a.e.Day
	if a.space(); a.warn == nil || !slices.Equal(a.warn.stars, []int{v.star}) || a.e.Day != day {
		t.Fatalf("Space: warning %v, day %d of %d", a.warn, a.e.Day, day)
	}
	p, st := a.warnPrompt()
	if len(p.lines) != 2 || p.lines[1] != a.cl.Stars[v.star].Name || !slices.Equal(st, []int{v.star}) {
		t.Errorf("lines %q, stars %v", p.lines, st)
	}
	if a.answerWarn(false); a.warn != nil || a.e.Day != day {
		t.Fatalf("Cancel: warning %v, day %d of %d", a.warn != nil, a.e.Day, day)
	}
	a.pressTurn(nextButton)
	if a.warn == nil || a.e.Day != day {
		t.Fatalf("next day button: warning %v, day %d", a.warn != nil, a.e.Day)
	}
	if a.answerWarn(true); a.warn != nil || a.e.Day != day+1 {
		t.Fatalf("End the day: warning %v, day %d of %d", a.warn != nil, a.e.Day, day+1)
	}
	day = a.e.Day
	a.space()
	if x, y := warnLine(0); a.warnClick(x, y) || a.warn == nil {
		t.Fatal("the title line is clickable")
	}
	if x, y := warnLine(1); !a.warnClick(x, y) {
		t.Fatal("the star's line is not clickable")
	}
	if w, ok := a.top().(*systemScreen); !ok || w.star != v.star || a.warn != nil || a.e.Day != day || len(a.stack) != 2 {
		t.Fatalf("star line: top %T, warning %v, day %d of %d, stack %d", a.top(), a.warn != nil, a.e.Day, day, len(a.stack))
	}
	a.pop()
	a.space()
	pressing(t, ebiten.KeyEnter)
	if a.Update(); a.warn != nil || a.e.Day != day+1 {
		t.Fatalf("Enter: warning %v, day %d of %d", a.warn != nil, a.e.Day, day+1)
	}

	gun.Power = 0
	day = a.e.Day
	if a.space(); a.warn != nil || a.e.Day != day+1 {
		t.Errorf("no usable energy: warning %v, day %d of %d", a.warn != nil, a.e.Day, day+1)
	}
	a, _, _ = roundApp(t)
	a.pop()
	a.g.Empires[1].Ships = nil
	day = a.e.Day
	if a.space(); a.warn != nil || a.e.Day != day+1 {
		t.Errorf("no fight: warning %v, day %d of %d", a.warn != nil, a.e.Day, day+1)
	}
}

// Skip asks before its first day, then stops before a day where the
// warning would apply, toasted, without ending it.
func TestSkipWarns(t *testing.T) {
	a, v, _ := roundApp(t)
	a.pop()
	day := a.e.Day
	if a.pressTurn(skipButton); a.warn == nil || a.e.Day != day {
		t.Fatalf("Skip: warning %v, day %d of %d", a.warn != nil, a.e.Day, day)
	}
	a.answerWarn(true)
	if want := "combat waiting at " + a.cl.Stars[v.star].Name; a.e.Day != day+1 || a.toast != want || a.warn != nil {
		t.Errorf("Skip: day %d, want %d; toast %q, want %q; warning %v", a.e.Day, day+1, a.toast, want, a.warn != nil)
	}
}

// The close plays a fight by Auto in one endDay: the day goes on, the
// rounds replayed at the fight's screen.
func TestCloseFights(t *testing.T) {
	a, v, gun := roundApp(t)
	v.order(a.e, farPoint(gun))
	day := a.e.Day
	if a.endDay(); a.e.Day != day+1 || v.replay == nil || !v.replay.fight {
		t.Errorf("endDay: day %d of %d, replay %v", a.e.Day, day+1, v.replay != nil)
	}
}

// A combat star the player is not engaged at, though known (a third
// empire with nothing there), is no combat mode and plays at the close:
// the rounds played there replayed.
func TestCloseNotEngaged(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
	if err := game.SpawnFixtures(g, 0, 1, 1); err != nil {
		t.Fatal(err)
	}
	cfg := bot.Default()
	cfg.Genome = bot.Neutral()
	cfg.Genome.FleeRatio = 1e9
	a := &app{g: g, player: 2, e: g.Empires[2], cl: g.Cluster, bots: bot.NewAll(g, cfg)}
	a.bots[2] = nil
	star := g.Empires[0].Colonies[0].Star
	a.e.Visit(star)
	v := newSystemScreen(star)
	a.push(v)
	foe := g.Empires[1].Ships[0]
	if g.Engaged(a.e, star) || !g.Engaged(g.Empires[0], star) || v.inCombat(a) {
		t.Fatal("fixture: the third empire engaged, or empire 0 not")
	}
	hull, day := foe.Hull, a.e.Day
	if a.endDay(); a.e.Day != day+1 {
		t.Fatalf("day %d of %d", a.e.Day, day)
	}
	if v.replay == nil || foe.Hull >= hull && len(v.replay.rounds[0].log.Shots) == 0 {
		t.Errorf("no round replayed at the fight (replay %v)", v.replay != nil)
	}
}

// A day's close logs each Report once, stamped with the day it brings,
// and returns them.
func TestCloseLogsOnce(t *testing.T) {
	a, _, _ := combatApp(t)
	n := len(a.log)
	ev := a.endDay()
	seen := map[string]bool{}
	for _, l := range a.log[n:] {
		if seen[l.Ev.Text] || l.Day != a.e.Day {
			t.Errorf("day %d (now %d): %q logged twice or misdated", l.Day, a.e.Day, l.Ev.Text)
		}
		seen[l.Ev.Text] = true
	}
	if len(a.log) == n || len(ev) != len(a.log)-n {
		t.Errorf("%d logged, %d events returned", len(a.log)-n, len(ev))
	}
}

// Combat mode locks nothing: after a step F5 saves, Esc pops off the
// screen, the log opens; C does nothing to the fight (no Resolve).
func TestCombatLocksNothing(t *testing.T) {
	a, v, _ := roundApp(t)
	pressing(t, ebiten.KeyC)
	day := a.e.Day
	if v.Update(a); a.e.Day != day || a.g.RoundsAt(v.star) != 0 {
		t.Fatalf("C: day %d of %d, rounds %d", a.e.Day, day, a.g.RoundsAt(v.star))
	}
	pressing(t)
	a.stepRound(v)
	if a.g.RoundsAt(v.star) != 1 {
		t.Fatal("no step")
	}
	a.save()
	if _, err := os.Stat(a.savePath); err != nil || !strings.HasPrefix(a.toast, "saved to ") {
		t.Errorf("F5 after a step: %v, toast %q", err, a.toast)
	}
	if a.toggleLog(); len(a.stack) != 3 {
		t.Errorf("the log did not open: stack %d", len(a.stack))
	}
	a.pop()
	v.sel = nil
	if a.back(); a.top() == screen(v) || a.quitting {
		t.Errorf("Esc: top %T, quit prompt %v", a.top(), a.quitting)
	}
}

// The starmap with a fight renders (its marker) without a panic.
func TestFightMarkerRenders(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	a, _, _ := roundApp(t)
	a.a = as
	a.preloadGame()
	a.pop()
	frame := ebiten.NewImage(screenW, screenH)
	a.g.RoundsToday = make([]int, len(a.cl.Stars))
	a.g.RoundsToday[a.e.Colonies[0].Star] = 3
	a.top().Render(a, frame)
	a.top().Text(a, frame)
}

// A replay dropped by a new day (the day ended from a screen above the
// system screen, its replay still running) takes its shots' and moves'
// sounds still to come with it: they would play over the next replay.
func TestDroppedReplayDropsItsCues(t *testing.T) {
	a, v, gun := roundApp(t)
	foe := a.g.Empires[1].Ships[0]
	sg, err := hostileSighting(a, foe)
	if err != nil {
		t.Fatal(err)
	}
	part, err := gun.ReadyWeapon(a.cl, foe)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.e.Arm(gun, part, sg); err != nil {
		t.Fatal(err)
	}
	a.stepRound(v)
	future := func() int {
		n := 0
		for _, c := range v.cues {
			if c.t > v.tick && !c.lane {
				n++
			}
		}
		return n
	}
	if v.replay == nil || future() == 0 {
		t.Fatalf("fixture: replay %v, cues to come %d", v.replay != nil, future())
	}
	a.push(newPlanetScreen(v.star, a.e.Colonies[0].PlanetIdx))
	if a.topSystem() != v {
		t.Fatal("fixture: the system screen left the stack")
	}
	a.beginDay()
	if v.replay != nil || future() != 0 {
		t.Errorf("after beginDay: replay %v, the dropped replay's cues to come %d", v.replay != nil, future())
	}
}

// A replay still running is continued: each round stepped is laid after
// the running one's end, t0 kept, the earlier rounds untouched.
func TestReplayAppends(t *testing.T) {
	a, v, gun := roundApp(t)
	v.order(a.e, farPoint(gun))
	a.stepRound(v)
	rp := v.replay
	if rp == nil {
		t.Fatal("no replay of round 0")
	}
	t0, n, end := rp.t0, len(rp.rounds), rp.length()
	first := rp.rounds[0]
	v.tick += 3 // still running
	a.stepRound(v)
	if v.replay != rp || rp.t0 != t0 || len(rp.rounds) != n+1 || rp.rounds[n].start != end {
		t.Fatalf("replay %v (same %v), t0 %d of %d, rounds %d of %d+1, start %d want %d",
			v.replay != nil, v.replay == rp, rp.t0, t0, len(rp.rounds), n, rp.rounds[len(rp.rounds)-1].start, end)
	}
	if rp.rounds[0].start != first.start || rp.rounds[0].length() != first.length() {
		t.Error("the running round changed")
	}
	i := -1
	for k, b := range rp.cast {
		if b.ship == gun {
			i = k
		}
	}
	if r := rp.rounds[n]; i < 0 || r.at[i] != rp.rounds[n-1].to[i] || r.to[i] != gun.Pos {
		t.Errorf("the appended round does not start where the last one ended (cast %d)", i)
	}
}

// The close's moves are tweened on any day: a ship ordered to a point in
// a system without a fight is replayed from where it stood to where the
// close took it, over closeTicks, and the replay ends in the model.
func TestCloseTween(t *testing.T) {
	e, v, ships := shipScreen(t)
	a := testApp(v.g)
	a.push(v)
	sh := ships[0]
	sh.Orbit = -1
	from := sh.Pos
	if err := e.Order(sh, farPoint(sh)); err != nil {
		t.Fatal(err)
	}
	a.endDay()
	if sh.Pos == from {
		t.Fatal("fixture: not moved")
	}
	rp := v.replay
	if rp == nil || len(rp.rounds) != 1 {
		t.Fatalf("replay %v: want one close phase", rp != nil)
	}
	r := rp.rounds[0]
	if r.move != closeTicks || r.length() != closeTicks || len(r.log.Moves) != 1 {
		t.Fatalf("close phase: move %d, length %d, %d moves", r.move, r.length(), len(r.log.Moves))
	}
	if m := r.log.Moves[0]; m.Ship != sh || m.From != from || m.To != sh.Pos || m.From == m.To {
		t.Errorf("move %v -> %v of %v, want %v -> %v", m.From, m.To, m.Ship == sh, from, sh.Pos)
	}
	// The close's moves alone do not spend a press: Space cuts them and
	// ends the next day.
	day := a.e.Day
	if a.space(); a.e.Day != day+1 {
		t.Errorf("Space over the close's moves: day %d, want %d", a.e.Day, day+1)
	}
	for n := 0; v.replay != nil && n < 1000; n++ {
		v.tick++
		v.replayTick(a)
	}
	if v.replay != nil {
		t.Error("the close replay never ended")
	}
	a.endDay() // the ship idle: no move, the planets' glide alone
	if rp := v.replay; rp == nil || len(rp.rounds) != 1 || !rp.rounds[0].glide || len(rp.rounds[0].log.Moves) != 0 {
		t.Error("a quiet close: want the planets' glide alone")
	}
}

// A foe out of a lane at the close opens a fight: one Engaged line in
// the log, at its star, whose click opens that system in combat mode.
func TestFightOpenedLogged(t *testing.T) {
	a, _, _, raider := peaceApp(t)
	atWar(t, a.g, 0, 1)
	home := a.e.Colonies[0].Star
	raider.Lane = a.cl.Stars[home].Lanes[0] // out of the fight
	lane := a.cl.Stars[home].Lanes[0]
	far := a.cl.Other(lane, home)
	a.cl.Stars[far].Pos = a.cl.Stars[home].Pos // a short lane
	foe := a.g.Empires[1].Spawn(game.FixtureFrigate, home, a.e.Colonies[0].PlanetIdx)
	foe.Star, foe.Orbit, foe.Pos = far, -1, game.Vec3{}
	foe.Refill()
	if err := a.g.Empires[1].Order(foe, game.Order{Kind: game.GoToStar, Star: home}); err != nil {
		t.Fatal(err)
	}
	if a.g.Engaged(a.e, home) {
		t.Fatal("engaged before the foe arrives")
	}
	a.stack = a.stack[:1]
	for d := 0; foe.Star != home || foe.Lane >= 0; d++ {
		if d > 20 {
			t.Fatalf("foe not home: %+v", foe)
		}
		a.endDay()
	}
	a.endDay() // the fight goes on: no second line
	var lines []int
	for i, l := range a.log {
		if l.Ev.Kind == game.Engaged {
			lines = append(lines, i)
		}
	}
	if len(lines) != 1 || a.log[lines[0]].Ev.Star != home {
		t.Fatalf("Engaged lines %v in %d", lines, len(a.log))
	}
	ss := newLogScreen().open(a, lines[0])
	if len(ss) != 1 {
		t.Fatalf("%q opens %v", a.log[lines[0]].Ev.Text, ss)
	}
	if v, ok := ss[0].(*systemScreen); !ok || v.star != home || !v.inCombat(a) {
		t.Errorf("%q opens %#v", a.log[lines[0]].Ev.Text, ss[0])
	}
}

// Three identical weapons, two generators: the weapon tile's fire mode
// arms the first, then, the mode entered again, the second (aimPart
// skips an armed part, else six twins would arm one shot); the third
// is refused, the weapons band counting no action left and the tile
// saying "no gen".
func TestAimArmsEachTwin(t *testing.T) {
	a, v, gun := roundApp(t)
	foe := a.g.Empires[1].Ships[0]
	gun.Design.Parts = []game.Gizmo{game.ProtonShaver, game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.MassBarrageGun, game.MassBarrageGun}
	gun.Shots = make([]int, len(gun.Design.Parts))
	gun.Power = gun.Design.PowerPerDay()
	v.rebuild()
	weapon := panelTiles(gun)[0]
	if got := actionsText(gun); got != "actions 2/2" || tileState(gun, weapon) != "" {
		t.Fatalf("before: %q, state %q", got, tileState(gun, weapon))
	}
	for i, want := range []int{3, 4} {
		v.sel, v.aim = gun, true
		gun.Weapon = 3
		if err := v.aimClick(a, shipOf(v, foe)); err != nil || len(gun.Armed) != i+1 || gun.Armed[i].Part != want {
			t.Fatalf("click %d: %v, armed %+v, want part %d", i+1, err, gun.Armed, want)
		}
	}
	if got := actionsText(gun); got != "actions 0/2" || tileState(gun, weapon) != noGen {
		t.Errorf("after two: %q, state %q", got, tileState(gun, weapon))
	}
	v.sel, v.aim = gun, true
	if err := v.aimClick(a, shipOf(v, foe)); err == nil || !strings.Contains(err.Error(), "no generator free") {
		t.Errorf("third: %v", err)
	}
}

// "no gen" needs a part of the tile left to arm: a lone Ueberlaser
// (two shots a day, one arm a round) armed on the ship's one generator
// has none, whatever its shots left today.
func TestNoGenNeedsAPartLeft(t *testing.T) {
	_, _, gun := roundApp(t)
	gun.Design.Parts = []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.Ueberlaser}
	gun.Shots = make([]int, len(gun.Design.Parts))
	gun.Power = 100
	gun.Armed = []game.Shot{{Part: 2}}
	weapon := panelTiles(gun)[0]
	if gun.Design.Generators() != 1 || weapon.g != game.Ueberlaser {
		t.Fatalf("fixture: %d generators, tile %v", gun.Design.Generators(), weapon.g)
	}
	if st := tileState(gun, weapon); st != "" {
		t.Errorf("its one part armed: state %q, want none", st)
	}
	gun.Design.Parts = append(gun.Design.Parts, game.Ueberlaser)
	gun.Shots = make([]int, len(gun.Design.Parts))
	if st := tileState(gun, panelTiles(gun)[0]); st != noGen {
		t.Errorf("a twin left unarmed: state %q, want %q", st, noGen)
	}
}

// Stop is lit by an armed shot alone and cancels it, as X (else the fire
// orders could not be taken back).
func TestStopDisarms(t *testing.T) {
	a, v, gun := roundApp(t)
	foe := a.g.Empires[1].Ships[0]
	if err := v.order(a.e, game.Order{}); err != nil {
		t.Fatal(err)
	}
	if _, on := v.stopState(a.e); on {
		t.Fatal("fixture: Stop lit with nothing to stop")
	}
	sg, err := hostileSighting(a, foe)
	if err != nil {
		t.Fatal(err)
	}
	part, err := gun.ReadyWeapon(a.cl, foe)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.e.Arm(gun, part, sg); err != nil {
		t.Fatal(err)
	}
	if sh, on := v.stopState(a.e); !sh || !on {
		t.Fatalf("an armed shot: Stop shown %v lit %v", sh, on)
	}
	r := stopRect()
	if err := v.panelClick(a, float64(r.X0+r.X1)/2, float64(r.Y0+r.Y1)/2); err != nil || len(gun.Armed) != 0 || v.sel != gun {
		t.Errorf("Stop: %v, armed %+v, sel %v", err, gun.Armed, v.sel)
	}
	if err := a.e.Arm(gun, part, sg); err != nil {
		t.Fatal(err)
	}
	if v.order(a.e, game.Order{}); len(gun.Armed) != 0 {
		t.Errorf("X left %+v armed", gun.Armed)
	}
}

// An armed shot shows until the round: its weapon's tile counts it
// (tileArmed, the hover text), the view dashes a line to its target
// (drawArmed, rendered with the game's assets); the round clears both.
func TestArmedMarked(t *testing.T) {
	a, v, gun := roundApp(t)
	foe := a.g.Empires[1].Ships[0]
	v.sel = gun
	sg, err := hostileSighting(a, foe)
	if err != nil {
		t.Fatal(err)
	}
	part, err := gun.ReadyWeapon(a.cl, foe)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.e.Arm(gun, part, sg); err != nil {
		t.Fatal(err)
	}
	var wt panelTile
	for _, tl := range panelTiles(gun) {
		if tl.band == weaponBand && tileArmed(gun, tl) > 0 {
			wt = tl
		}
	}
	if tileArmed(gun, wt) != 1 || !strings.Contains(tileText(gun, wt), "armed 1 for the next round") {
		t.Fatalf("armed tile %+v: %d, %q", wt, tileArmed(gun, wt), tileText(gun, wt))
	}
	if dir := cob.DefaultDir(); func() bool { _, err := os.Stat(dir); return err == nil }() {
		as, err := assets.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer as.Close()
		a.a = as
		a.preloadGame()
		frame := ebiten.NewImage(screenW, screenH)
		a.top().Render(a, frame)
		a.top().Text(a, frame)
	}
	v.press(a, endTurnButton)
	if len(gun.Armed) != 0 || tileArmed(gun, wt) != 0 {
		t.Errorf("after the round: armed %v", gun.Armed)
	}
}

// roundNews leaves every zero out: a quiet round says "no hits", a hit
// without shields no "0 shields", a single hit no plural.
func TestRoundNews(t *testing.T) {
	g := &game.Game{Cluster: &game.Cluster{Stars: []game.Star{{System: &game.StarSystem{}}}}}
	foe := &game.Starship{}
	for _, c := range []struct {
		shots []game.Fired
		dead  []*game.Starship
		want  string
	}{
		{nil, nil, "Round 3: no hits"},
		{[]game.Fired{{ShooterEmpire: 1, TargetEmpire: 0, Damage: 5}}, nil, "Round 3: took 1 hit (5 hull)"},
		{[]game.Fired{{ShooterEmpire: 0, TargetEmpire: 1, Damage: 20}, {ShooterEmpire: 0, TargetEmpire: 1, Absorbed: 4}}, []*game.Starship{foe},
			"Round 3: dealt 2 hits (20 hull, 4 shields), destroyed 1"},
	} {
		if got := roundNews(g, 0, 3, nil, game.RoundLog{Shots: c.shots, Destroyed: c.dead}); got != c.want {
			t.Errorf("%q, want %q", got, c.want)
		}
	}
}
