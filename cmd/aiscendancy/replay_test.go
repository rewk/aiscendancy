package main

import (
	"fmt"
	"math"
	"os"
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/shp"
	"github.com/rewk/aiscendancy/sound"
)

// combatApp is TestEndDayLogsCombat's fight with the system screen open
// at the home star: the player's Gunboat (shields off, hull = one Mass
// Barrage hit) armed at empire 1's Raider (Ion Wrap absorbs it), the
// Raider's bot with no flight. twin is the same game left without a
// window, the oracle bot.Day is played on.
func combatApp(t *testing.T) (a *app, v *systemScreen, twin func() []game.RoundLog) {
	t.Helper()
	build := func() (*game.Game, []*bot.Bot) {
		g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
		if err := game.SpawnFixtures(g, 0, 1, 1); err != nil {
			t.Fatal(err)
		}
		cfg := bot.Default()
		cfg.Genome = bot.Neutral()
		cfg.Genome.FleeRatio = 1e9
		bots := bot.NewAll(g, cfg)
		bots[0] = nil
		gun, foe := g.Empires[0].Ships[0], g.Empires[1].Ships[0]
		gun.Shields, gun.Hull = false, game.Gizmos[game.MassBarrageGun].Level
		sg, ok := g.SightingOf(g.Empires[0], foe)
		if !ok {
			t.Fatal("the Raider is not seen")
		}
		if err := g.Empires[0].Arm(gun, slices.Index(gun.Design.Parts, game.MassBarrageGun), sg); err != nil {
			t.Fatal(err)
		}
		return g, bots
	}
	g, bots := build()
	a = testApp(g)
	a.bots = bots
	v = newSystemScreen(g.Empires[0].Colonies[0].Star)
	a.push(v)
	return a, v, func() []game.RoundLog {
		g2, b2 := build()
		_, _, rounds := bot.Day(g2, b2)
		return rounds
	}
}

// shotKey is a Fired without its pointers: comparable across twin games.
func shotKey(f game.Fired) string {
	return fmt.Sprintf("%s>%s %d>%d %v %v>%v d%d a%d", f.Shooter.Name, f.Target.Name, f.ShooterEmpire, f.TargetEmpire,
		f.Gizmo, f.From, f.To, f.Damage, f.Absorbed)
}

// After endDay the system screen replays the day's rounds at its star,
// the ones of the twin's bot.Day with something in them; its shots play
// one by one: the first's bullet alone, then its impact as the second
// flies, then both impacts (the Raider's shield, the Gunboat's
// explosion); the destroy, the Gunboat's sprite and selection gone from
// its destroy on; stepped to its end it is gone.
func TestEndDayReplay(t *testing.T) {
	a, v, twin := combatApp(t)
	gun := a.e.Ships[0]
	v.sel = gun
	a.endDay()
	rp := v.replay
	if rp == nil {
		t.Fatal("no replay")
	}
	var want []game.RoundLog
	for _, l := range twin() {
		if l.Star == v.star && len(l.Moves)+len(l.Shots)+len(l.Destroyed) > 0 {
			want = append(want, l)
		}
	}
	rounds := rp.rounds
	if n := len(rounds); n > 0 && rounds[n-1].glide { // the close's: the planets' glide (closeReplay)
		rounds = rounds[:n-1]
	}
	if len(rounds) != len(want) || len(want) == 0 {
		t.Fatalf("%d rounds replayed, the twin played %d", len(rounds), len(want))
	}
	for i, r := range rounds {
		if len(r.log.Moves) != len(want[i].Moves) || len(r.log.Destroyed) != len(want[i].Destroyed) {
			t.Errorf("round %d: %d moves %d destroyed, want %d %d", i, len(r.log.Moves), len(r.log.Destroyed), len(want[i].Moves), len(want[i].Destroyed))
		}
		got, exp := []string{}, []string{}
		for _, f := range r.log.Shots {
			got = append(got, shotKey(f))
		}
		for _, f := range want[i].Shots {
			exp = append(exp, shotKey(f))
		}
		if !slices.Equal(got, exp) {
			t.Errorf("round %d shots:\n got %v\nwant %v", i, got, exp)
		}
	}
	r0 := rp.rounds[0]
	if len(r0.log.Shots) != 2 || !slices.Contains(r0.log.Destroyed, gun) {
		t.Fatalf("round 0: %d shots, destroyed %v: want the two shots and the Gunboat", len(r0.log.Shots), r0.log.Destroyed)
	}
	shown := func() bool {
		v.rebuild()
		return slices.ContainsFunc(v.bodies, func(b body) bool { return b.ship == gun })
	}
	banks := func() []string {
		var out []string
		for _, e := range v.effects() {
			out = append(out, e.bank)
		}
		slices.Sort(out)
		return out
	}
	at := func(t int) { v.tick = rp.t0 + r0.start + t }
	s0, s1 := r0.log.Shots[0], r0.log.Shots[1]
	i0, n0 := impactOf(s0)
	i1, _ := impactOf(s1)
	if n0*fxTicks <= flightTicks || shotAt(1) != r0.flight || landsAt(0) != r0.flight {
		t.Fatalf("fixture: shot 0's impact %d ticks, flight %d; shot 1 at %d", n0*fxTicks, flightTicks, shotAt(1))
	}
	sorted := func(l ...string) []string { slices.Sort(l); return l }
	at(0)
	if got, exp := banks(), []string{bulletBank(s0.Gizmo)}; !slices.Equal(got, exp) || !shown() || v.sel != gun {
		t.Errorf("first flight: effects %v, want %v; Gunboat shown %v, selected %v", got, exp, shown(), v.sel == gun)
	}
	at(r0.flight)
	if got, exp := banks(), sorted(i0, bulletBank(s1.Gizmo)); !slices.Equal(got, exp) || !shown() {
		t.Errorf("first impact: effects %v, want %v; Gunboat shown %v", got, exp, shown())
	}
	at(landsAt(1))
	if got, exp := banks(), sorted(i0, i1); !slices.Equal(got, exp) || !slices.Equal(exp, sorted(explBank(game.MassBarrageGun), shieldBank)) || !shown() {
		t.Errorf("second impact: effects %v, want %v; Gunboat shown %v", got, exp, shown())
	}
	at(r0.flight + r0.impact)
	if got := banks(); !slices.Equal(got, []string{destroyBank}) || shown() || v.sel != nil {
		t.Errorf("destroy: effects %v, Gunboat shown %v, still selected %v", got, shown(), v.sel != nil)
	}
	v.tick = rp.t0
	for range rp.length() + 1 {
		v.tick++
		v.replayTick(a)
	}
	if v.replay != nil || len(v.effects()) != 0 {
		t.Error("the replay outlived its rounds")
	}
}

// Space, End Turn and a new day: Space skips a running replay without
// ending the day, then ends it; End Turn skips it too; an unknown star
// keeps none.
func TestReplaySkip(t *testing.T) {
	a, v, _ := combatApp(t)
	a.endDay()
	day := a.e.Day
	if v.replay == nil {
		t.Fatal("no replay")
	}
	if a.space(); v.replay != nil || a.e.Day != day {
		t.Errorf("Space: replay %v, day %d of %d", v.replay != nil, a.e.Day, day)
	}
	if a.space(); a.e.Day != day+1 {
		t.Errorf("Space again: day %d, want %d", a.e.Day, day+1)
	}

	a, v, _ = combatApp(t)
	a.endDay()
	day = a.e.Day
	if v.press(a, endTurnButton); v.replay != nil || a.e.Day != day {
		t.Errorf("End Turn: replay %v, day %d of %d", v.replay != nil, a.e.Day, day)
	}

	a, v, twin := combatApp(t)
	cast := v.replayCast()
	if v.startReplay(&game.Empire{}, cast, twin()); v.replay != nil {
		t.Error("a replay at a star the player does not know")
	}
	if v.startReplay(a.e, cast, twin()); v.replay == nil {
		t.Error("no replay at the known star")
	}
}

// replayStack is combatApp's screen over a starmap, the day ended
// (nextDay: the warning passed): the replay runs, the day's own pending climb
// dropped (navDue nil), and elsewhere an event at another star than v's.
func replayStack(t *testing.T) (a *app, v *systemScreen, elsewhere []game.Event) {
	t.Helper()
	a, v, _ = combatApp(t)
	a.stack = nil
	a.push(newStarmapScreen())
	a.push(v)
	a.nextDay()
	if v.replay == nil || a.top() != screen(v) {
		t.Fatalf("after NEXT DAY: replay %v, top %T", v.replay != nil, a.top())
	}
	v.replay.navDue = nil
	other := (v.star + 1) % len(a.cl.Stars)
	return a, v, []game.Event{{Kind: game.Built, Item: game.Factory, Tech: game.NoTech, Star: other, Planet: 0, Other: -1}}
}

// NEXT DAY with the fight on screen: a climb off it waits for the replay
// (pushed at once, the fight would be gone): the system screen stays on
// top until the replay's last tick, then the view climbs to the
// starmap; a skip climbs at once. An event the screen covers leaves
// nothing waiting.
func TestReplayDefersClimb(t *testing.T) {
	a, v, ev := replayStack(t)
	a.report([]game.Event{{Kind: game.Built, Item: game.Factory, Tech: game.NoTech, Star: v.star, Planet: 0, Other: -1}})
	if v.replay.navDue != nil || a.top() != screen(v) {
		t.Fatalf("a covered event: %d waiting, top %T", len(v.replay.navDue), a.top())
	}
	a.report(ev)
	if len(v.replay.navDue) != 1 || a.top() != screen(v) {
		t.Fatalf("an event elsewhere: %d waiting, top %T", len(v.replay.navDue), a.top())
	}
	for n := v.replay.length(); n > 1; n-- {
		if v.tick++; v.replayTick(a) || a.top() != screen(v) {
			t.Fatalf("the view climbed %d ticks before the replay's end", n-1)
		}
	}
	if v.tick++; !v.replayTick(a) || v.replay != nil {
		t.Fatal("the replay's end did not climb")
	}
	if _, ok := a.top().(*starmapScreen); !ok || len(a.stack) != 1 {
		t.Errorf("after the replay: top %T, stack %d, want the starmap alone", a.top(), len(a.stack))
	}

	a, v, ev = replayStack(t)
	a.report(ev)
	if a.space(); v.replay != nil {
		t.Error("Space kept the replay")
	}
	if _, ok := a.top().(*starmapScreen); !ok || len(a.stack) != 1 {
		t.Errorf("after a skip: top %T, stack %d, want the starmap alone", a.top(), len(a.stack))
	}
}

// Every weapon has its bullet and explosion banks (NN = its Gizmo index),
// with the frame counts the replay's timing assumes.
func TestFxBanks(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	banks := fxBanks()
	if len(banks) != 4+2*10 {
		t.Errorf("%d banks: want the shield, the destroy, the two lane flares and 10 weapons' two", len(banks))
	}
	for bank, n := range banks {
		b, err := as.Bank(bank)
		if err != nil {
			t.Error(err)
			continue
		}
		if n > 0 && len(b.Frames) != n {
			t.Errorf("%s: %d frames, want %d", bank, len(b.Frames), n)
		}
	}
}

// A destroyed ship's sprite goes at the exact tick its blast starts:
// one tick before, the Gunboat is drawn and no blast; at it, the blast of
// destroy.shp plays its frame 0 on the Gunboat, which is gone.
func TestReplayBlastReplacesShip(t *testing.T) {
	a, v, _ := combatApp(t)
	gun := a.e.Ships[0]
	a.endDay()
	rp := v.replay
	if rp == nil {
		t.Fatal("no replay")
	}
	r0 := &rp.rounds[0]
	if !r0.boom[slices.IndexFunc(rp.cast, func(b body) bool { return b.ship == gun })] {
		t.Fatal("the Gunboat is not destroyed in round 0")
	}
	blasts := func() []effect {
		var out []effect
		for _, e := range v.effects() {
			if e.kind == fxBlast {
				out = append(out, e)
			}
		}
		return out
	}
	shown := func() bool {
		v.rebuild()
		return slices.ContainsFunc(v.bodies, func(b body) bool { return b.ship == gun })
	}
	v.tick = rp.t0 + r0.start + r0.blastStart() - 1
	if bl := blasts(); !shown() || len(bl) != 0 {
		t.Errorf("a tick before the blast: Gunboat shown %v, %d blasts", shown(), len(bl))
	}
	v.tick++
	bl := blasts()
	if shown() || len(bl) != 1 || bl[0].ship == nil || bl[0].ship.ship != gun || bl[0].bank != destroyBank || bl[0].step != 0 {
		t.Errorf("the blast's first tick: Gunboat shown %v, blasts %+v; want one destroy frame 0 on it", shown(), bl)
	}
}

// A round's shots play before its moves, the rule's order: the bullets
// fly from and to the round's starting positions, the target stays there
// through the impacts and the blasts (the shooter shot down in return),
// and its move tweens after them, its engine cued then.
func TestReplayShotsBeforeMoves(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	from := foe.Pos
	to := game.Vec3{X: from.X + 0.1, Y: from.Y, Z: from.Z}
	logs := []game.RoundLog{{Star: v.star,
		Moves: []game.Move{{Ship: foe, Empire: 1, From: from, To: to, Orbit: -1}},
		Shots: []game.Fired{
			{Shooter: gun, Target: foe, Gizmo: game.MassBarrageGun, ShooterEmpire: 0, TargetEmpire: 1, From: gun.Pos, To: from, Damage: 1},
			{Shooter: foe, Target: gun, Gizmo: game.MassBarrageGun, ShooterEmpire: 1, TargetEmpire: 0, From: from, To: gun.Pos, Damage: 1}},
		Destroyed: []*game.Starship{gun},
	}}
	v.startReplay(a.e, v.replayCast(), logs)
	if v.replay == nil {
		t.Fatal("no replay")
	}
	r := &v.replay.rounds[0]
	if r.flight == 0 || r.impact == 0 || r.kill == 0 || r.move != moveTicks || r.blastStart() != r.flight+r.impact ||
		r.moveStart() != r.blastStart()+r.kill || r.length() != r.moveStart()+r.move {
		t.Fatalf("phases: flight %d impact %d kill %d move %d, blast %d, move start %d, length %d",
			r.flight, r.impact, r.kill, r.move, r.blastStart(), r.moveStart(), r.length())
	}
	if !slices.Contains(v.cues, cue{name: thrustSound(foe.Design.Hull), t: v.replay.t0 + r.start + r.moveStart()}) {
		t.Errorf("the engine not cued at the move phase: %+v", v.cues)
	}
	// at is the Raider's drawn position at the round's tick, the effects
	// on it then, and whether the Gunboat is drawn.
	at := func(tick int) (p game.Vec3, on []effect, gunShown bool) {
		t.Helper()
		v.tick = v.replay.t0 + r.start + tick
		v.rebuild()
		i := slices.IndexFunc(v.bodies, func(b body) bool { return b.ship == foe })
		if i < 0 {
			t.Fatalf("tick %d: the Raider not drawn", tick)
		}
		for _, e := range v.effects() {
			if e.ship != nil && e.ship.ship == foe {
				on = append(on, e)
			}
		}
		return v.bodies[i].pos, on, slices.ContainsFunc(v.bodies, func(b body) bool { return b.ship == gun })
	}
	if p, fx, shown := at(0); p != from || len(fx) != 1 || fx[0].kind != fxBullet || fx[0].toPos != from || !shown {
		t.Errorf("flight: Raider at %v (want %v), effects on it %+v, Gunboat shown %v", p, from, fx, shown)
	}
	if p, fx, shown := at(r.flight); p != from || len(fx) != 1 || fx[0].kind != fxImpact || fx[0].pos != from || !shown {
		t.Errorf("impact: Raider at %v (want %v), effects on it %+v, Gunboat shown %v", p, from, fx, shown)
	}
	if p, _, shown := at(r.blastStart()); p != from || shown {
		t.Errorf("blast: Raider at %v (want %v), Gunboat shown %v", p, from, shown)
	}
	if p, fx, shown := at(r.moveStart()); p != from || len(fx) != 0 || shown || len(v.effects()) != 0 {
		t.Errorf("move phase's start: Raider at %v (want %v), effects %+v, Gunboat shown %v", p, from, v.effects(), shown)
	}
	if p, _, _ := at(r.moveStart() + r.move/2); p == from || p == to {
		t.Errorf("mid-move: Raider at %v (from %v to %v)", p, from, to)
	}
}

// The impacts are drawn over their target: right after its body in the
// draw list, whatever the depths, in log order when several hit it; a
// bullet flies to its target as drawn.
func TestImpactOverTarget(t *testing.T) {
	a, v, _ := combatApp(t)
	gun := a.e.Ships[0]
	a.endDay()
	rp := v.replay
	if rp == nil {
		t.Fatal("no replay")
	}
	r0 := &rp.rounds[0]
	v.tick = rp.t0 + r0.start
	v.rebuild()
	for _, e := range v.effects() {
		i := slices.IndexFunc(v.bodies, func(b body) bool { return e.ship != nil && b.ship == e.ship.ship })
		if e.kind != fxBullet || i < 0 || v.bodies[i].pos != e.toPos {
			t.Errorf("bullet %s: ends at %v, not on its target as drawn", e.bank, e.toPos)
		}
	}
	// A second hit on the Gunboat, another weapon's, right after the
	// first: both impacts drawn as the second lands (the first outlasts a
	// flight), the Raider's (shot 0) over by then.
	k := slices.IndexFunc(r0.log.Shots, func(f game.Fired) bool { return f.Target == gun })
	if k != 1 {
		t.Fatalf("the shot at the Gunboat is shot %d, want 1", k)
	}
	extra := r0.log.Shots[k]
	extra.Gizmo = game.Plasmatron
	lg := r0.log
	lg.Shots = append(slices.Clone(lg.Shots), extra)
	cast := rp.cast
	v.replay = nil
	v.startReplay(a.e, cast, []game.RoundLog{lg})
	r0 = &v.replay.rounds[0]
	v.tick = v.replay.t0 + r0.start + landsAt(2)
	v.rebuild()
	want := map[*game.Starship][]string{}
	for i, f := range r0.log.Shots {
		if bank, n := impactOf(f); bank != "" && landsAt(i)+n*fxTicks > landsAt(2) {
			want[f.Target] = append(want[f.Target], bank)
		}
	}
	if len(want) != 1 || len(want[gun]) != 2 {
		t.Fatalf("impacts %v: want two on the Gunboat", want)
	}
	list := v.drawList()
	for sh, banks := range want {
		i := slices.IndexFunc(list, func(it drawItem) bool { return it.body != nil && it.body.ship == sh })
		if i < 0 {
			t.Fatalf("%s not drawn", sh.Name)
		}
		var got []string
		for _, it := range list[i+1:] {
			if it.fx == nil || it.fx.kind != fxImpact || it.fx.ship.ship != sh {
				break
			}
			got = append(got, it.fx.bank)
		}
		if !slices.Equal(got, banks) {
			t.Errorf("%s: impacts right after its body %v, want %v", sh.Name, got, banks)
		}
	}
}

// hidden is the share of ship sprite frame f's opaque pixels, drawn in
// box ship, that frame c drawn k px per art pixel with its origin at
// (x,y) covers with its own (pixel centres sampled).
func hidden(f *shp.Frame, ship rectF, c *shp.Frame, k, x, y float64) float64 {
	ks := ship.w() / float64(f.Box.Dx())
	in, all := 0, 0
	for py := range f.Box.Dy() {
		for px := range f.Box.Dx() {
			if !f.Opaque[py*f.Box.Dx()+px] {
				continue
			}
			all++
			cx := int(math.Floor((ship.x0+ks*(float64(px)+0.5)-x)/k)) + c.Center.X - c.Box.Min.X
			cy := int(math.Floor((ship.y0+ks*(float64(py)+0.5)-y)/k)) + c.Center.Y - c.Box.Min.Y
			if cx >= 0 && cy >= 0 && cx < c.Box.Dx() && cy < c.Box.Dy() && c.Opaque[cy*c.Box.Dx()+cx] {
				in++
			}
		}
	}
	return float64(in) / float64(all)
}

// The impacts' and blasts' scales derive from the frame boxes, for every
// species' hull sprites: the blast's frame 0 box is centred on the ship's
// drawn box, its inscribed ellipse through the box's corners, hiding at
// least 95% of the ship's pixels, and its bloom is at least 3 times the
// ship's larger side; an impact's largest frame spans the ship's larger
// side, on its centre.
func TestFxScales(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	bank := func(n string) []*shp.Frame {
		b, err := as.Bank(n)
		if err != nil {
			t.Fatal(err)
		}
		return b.Frames
	}
	near := func(x, y float64) bool { return math.Abs(x-y) < 1e-9 }
	extent := func(r rectF) float64 { return max(r.w(), r.h()) }
	blast := bank(destroyBank)
	for sp := range game.NumSpecies {
		for h, f := range bank(shipBank(sp))[:game.NumHulls] {
			ship := shipBox(f, 300, 200, 20)
			cx, cy := ship.centre()
			if !near(ship.w(), 40) {
				t.Errorf("%v hull %d: drawn %v wide, want 2r", sp, h, ship.w())
			}
			k, x, y := blastAt(ship, blast)
			c := scaled(blast[0], k, x, y)
			ccx, ccy := c.centre()
			corner := math.Hypot(ship.w()/c.w(), ship.h()/c.h())
			if share := hidden(f, ship, blast[0], k, x, y); !near(corner, 1) || !near(ccx, cx) || !near(ccy, cy) || share < 0.95 {
				t.Errorf("%v hull %d: frame 0 drawn %v over the ship's %v (corner %.3f), hides %.2f of it", sp, h, c, ship, corner, share)
			}
			bloom := 0.0
			for _, g := range blast {
				if !g.Empty() {
					bloom = max(bloom, extent(scaled(g, k, x, y)))
				}
			}
			if bloom < 3*extent(ship) {
				t.Errorf("%v hull %d: the blast blooms to %.1f px, the ship is %.1f", sp, h, bloom, extent(ship))
			}
			for name, n := range fxBanks() {
				if n == 0 || name == destroyBank || name == blueExitBank || name == redExitBank { // the impacts only
					continue
				}
				fs := bank(name)
				k, x, y := impactAt(ship, fs)
				big := 0.0
				for _, g := range fs {
					big = max(big, float64(max(g.Box.Dx(), g.Box.Dy()))*k)
				}
				if !near(big, extent(ship)) || !near(x, cx) || !near(y, cy) {
					t.Errorf("%v hull %d: %s spans %.1f px at (%.1f,%.1f), the ship %.1f at (%.1f,%.1f)", sp, h, name, big, x, y, extent(ship), cx, cy)
				}
			}
		}
	}
}

// Esc (back) out of the system screen while its replay holds the day's
// climb: the screen goes and the climb runs from the one below (the
// fleet list, which does not cover a Built Factory: up to the starmap),
// never lost with the replay.
func TestReplayBackClimbs(t *testing.T) {
	a, v, ev := replayStack(t)
	a.stack = a.stack[:1]
	a.push(newFleetScreen())
	a.push(v)
	a.report(ev)
	if v.replay == nil || len(v.replay.navDue) != 1 {
		t.Fatalf("replay %v, no climb waiting", v.replay != nil)
	}
	if err := a.back(); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(a.stack, screen(v)) {
		t.Fatal("Esc kept the system screen")
	}
	if _, ok := a.top().(*starmapScreen); !ok || len(a.stack) != 1 {
		t.Errorf("after Esc mid-replay: top %T, stack %d, want the starmap alone", a.top(), len(a.stack))
	}
}

// Tab over a replay whose climb waits shows the log, the replay frozen
// under it; Tab again comes back to the system screen and the replay's
// end climbs then, once.
func TestReplayTabKeepsClimb(t *testing.T) {
	a, v, ev := replayStack(t)
	a.report(ev)
	a.toggleLog()
	if _, ok := a.top().(*logScreen); !ok {
		t.Fatalf("Tab: top %T, want the log", a.top())
	}
	a.toggleLog()
	if a.top() != screen(v) || v.replay == nil || len(v.replay.navDue) != 1 {
		t.Fatalf("Tab again: top %T, want the system screen, its climb waiting", a.top())
	}
	for n := 0; v.replay != nil && n < 10000; n++ {
		v.tick++
		v.replayTick(a)
	}
	if _, ok := a.top().(*starmapScreen); !ok || len(a.stack) != 1 {
		t.Errorf("the replay's end: top %T, stack %d, want the starmap alone", a.top(), len(a.stack))
	}
}

// A ship leaving an orbit in a round stays in its orbit (hidden) through
// the round's shots, impacts and blasts: it leaves only in the move
// phase, after them (shots before moves).
func TestReplayLeavesOrbitAfterShots(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	foe.Orbit = 0
	v.rebuild()
	from := foe.Pos
	to := game.Vec3{X: from.X + 0.1, Y: from.Y, Z: from.Z}
	logs := []game.RoundLog{{Star: v.star,
		Moves: []game.Move{{Ship: foe, Empire: 1, From: from, To: to, Orbit: -1}},
		Shots: []game.Fired{{Shooter: gun, Gizmo: game.MassBarrageGun, ShooterEmpire: 0, TargetEmpire: 1,
			AtPlanet: true, TargetPlanet: 0, HitSlot: -1, HitItem: game.NoItem, From: gun.Pos, To: from}},
	}}
	v.startReplay(a.e, v.replayCast(), logs)
	if v.replay == nil {
		t.Fatal("no replay")
	}
	r := &v.replay.rounds[0]
	if r.orbAt[slices.IndexFunc(v.replay.cast, func(b body) bool { return b.ship == foe })] != 0 {
		t.Fatal("the Raider not in orbit at the round's start")
	}
	inOrbit := func(tick int) bool {
		t.Helper()
		v.tick = v.replay.t0 + r.start + tick
		bs := v.replayBodies()
		i := slices.IndexFunc(bs, func(b body) bool { return b.ship == foe })
		if i < 0 {
			t.Fatalf("tick %d: the Raider not in the cast", tick)
		}
		return bs[i].orbiting
	}
	for _, tk := range []int{0, r.flight / 2, r.moveStart() - 1} {
		if !inOrbit(tk) {
			t.Errorf("tick %d (move phase at %d): the Raider out of its orbit before the moves", tk, r.moveStart())
		}
	}
	if inOrbit(r.moveStart() + r.move/2) {
		t.Error("mid-move: the Raider still in orbit")
	}
}

// shotAt and landsAt are a round's shot k's take-off and landing without
// the archives' sounds (no gunTicks: the tests), every flight flightTicks.
func shotAt(k int) int  { return k * flightTicks }
func landsAt(k int) int { return shotAt(k) + flightTicks }

// A shot flies as long as its weapon's sound (gunTicks): the next takes
// off as it lands, the cues at those ticks; the archives' lengths are
// the gbNN tracks'.
func TestReplayFlightsFollowSounds(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	v.gunTicks = map[game.Gizmo]int{game.MassBarrageGun: 40, game.Plasmatron: 25}
	shots := []game.Fired{
		{Shooter: gun, Target: foe, Gizmo: game.MassBarrageGun, TargetEmpire: 1, From: gun.Pos, To: foe.Pos, Damage: 2},
		{Shooter: foe, Target: gun, Gizmo: game.Plasmatron, ShooterEmpire: 1, From: foe.Pos, To: gun.Pos, Absorbed: 4},
	}
	v.cues = nil
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: shots}})
	r := &v.replay.rounds[0]
	if r.shotAt(1) != 40 || r.landsAt(1) != 65 || r.flight != 40 {
		t.Fatalf("shot 1 at %d lands %d, flight %d; want 40, 65, 40", r.shotAt(1), r.landsAt(1), r.flight)
	}
	v.tick = v.replay.t0 + r.start + 20
	if fx := v.effects(); !slices.ContainsFunc(fx, func(e effect) bool { return e.kind == fxBullet && e.f == 0.5 }) {
		t.Errorf("mid-flight of shot 0: %+v", fx)
	}
	t0 := v.replay.t0 + r.start
	for _, c := range []cue{{name: gunSound(game.MassBarrageGun), t: t0}, {name: hitSound(game.MassBarrageGun), t: t0 + 40},
		{name: gunSound(game.Plasmatron), t: t0 + 40}, {name: shieldSound, t: t0 + 65}} {
		if !slices.Contains(v.cues, c) {
			t.Errorf("no %+v in %+v", c, v.cues)
		}
	}
	g, err := cob.OpenGame(cob.DefaultDir())
	if err != nil {
		t.Skip(err)
	}
	defer g.Close()
	tr, err := sound.Effect(g, gunSound(game.Plasmatron))
	if err != nil {
		t.Fatal(err)
	}
	if n := gunTicks(g)[game.Plasmatron]; n != int(tr.Duration().Seconds()*60+0.5) || n < 2*flightTicks {
		t.Errorf("the Plasmatron's flight %d ticks, its sound %v", n, tr.Duration())
	}
}

// An impact lasts as long as its landing sound and a blast as the
// blast's (sndTicks), their frames spread over it;
// the blast's sound cued as the blasts start. The archives' lengths are
// the tracks'.
func TestReplayFxFollowSounds(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	v.sndTicks = map[string]int{hitSound(game.MassBarrageGun): 50, blastSound: 80}
	shots := []game.Fired{{Shooter: gun, Target: foe, Gizmo: game.MassBarrageGun, TargetEmpire: 1, From: gun.Pos, To: foe.Pos, Damage: 2}}
	v.cues = nil
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: shots, Destroyed: []*game.Starship{foe}}})
	r := &v.replay.rounds[0]
	if r.flight+r.impact != flightTicks+50 || r.kill != 80 {
		t.Fatalf("impact ends %d, kill %d; want %d, 80", r.flight+r.impact, r.kill, flightTicks+50)
	}
	t0 := v.replay.t0 + r.start
	if !slices.Contains(v.cues, cue{name: blastSound, t: t0 + r.blastStart()}) {
		t.Errorf("no blast cue at %d in %+v", t0+r.blastStart(), v.cues)
	}
	step := func(at int, k fxKind) int {
		v.tick = t0 + at
		for _, e := range v.effects() {
			if e.kind == k {
				return e.step
			}
		}
		return -1
	}
	if got := step(r.landsAt(0)+25, fxImpact); got != explFrames/2 {
		t.Errorf("mid-impact frame %d, want %d", got, explFrames/2)
	}
	if got := step(r.blastStart()+79, fxBlast); got != destroyFrames-1 {
		t.Errorf("last blast tick's frame %d, want %d", got, destroyFrames-1)
	}
	g, err := cob.OpenGame(cob.DefaultDir())
	if err != nil {
		t.Skip(err)
	}
	defer g.Close()
	tr, err := sound.Effect(g, blastSound)
	if err != nil {
		t.Fatal(err)
	}
	if n := fxSoundTicks(g)[blastSound]; n != int(tr.Duration().Seconds()*60+0.5) || n <= destroyFrames*fxTicks {
		t.Errorf("the blast %d ticks, its sound %v", n, tr.Duration())
	}
}

// shotOrder alternates the sides, each side's and each target's order
// kept (the outcomes hang on it), the shot that destroyed a ship last;
// a shot at the wreck after it stays before; a third side's shot at the
// same target never passes an earlier one.
func TestShotOrder(t *testing.T) {
	a := &game.Starship{Name: "a"}
	b := &game.Starship{Name: "b"}
	c := &game.Starship{Name: "c"}
	f := func(by int, at *game.Starship, dmg int) game.Fired {
		return game.Fired{ShooterEmpire: by, Target: at, Damage: dmg}
	}
	shots := []game.Fired{f(0, b, 1), f(0, b, 2), f(0, b, 3), f(1, a, 4), f(1, a, 5), f(0, b, 0), f(2, b, 6), f(1, c, 7)}
	got := shotOrder(shots, []*game.Starship{b})
	var dmg []int
	for _, s := range got {
		dmg = append(dmg, s.Damage)
	}
	// Sides 0, 1, 2 in turn; b's killer is its last hull hit (6, side
	// 2): last; side 0's no-effect 0 after its 3; c's 7 before the end.
	if want := []int{1, 4, 2, 5, 3, 7, 0, 6}; !slices.Equal(dmg, want) {
		t.Errorf("order %v, want %v", dmg, want)
	}
	if len(shotOrder(nil, nil)) != 0 {
		t.Error("no shots")
	}
}

// A round's shots play one by one in log order: shot k's bullet flies
// alone of its own from shotAt(k) (none of it a tick before) and lands
// at landsAt(k), the next one's take-off, its impact starting then (the
// weapon's explosion on hull, the shield's absorbed or knocked down,
// none for a shot that did nothing); the blasts start as the last impact
// ends, the moves after them. Each shot cues its gun at its take-off,
// its hit or the shield's as it lands, each at its own tick.
func TestReplayShotsInSequence(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	mbg := game.MassBarrageGun
	shots := []game.Fired{
		{Shooter: gun, Target: foe, Gizmo: mbg, TargetEmpire: 1, From: gun.Pos, To: foe.Pos, Absorbed: 1},
		{Shooter: gun, Target: foe, Gizmo: game.Ueberlaser, TargetEmpire: 1, From: gun.Pos, To: foe.Pos}, // nothing: no impact
		{Shooter: gun, Target: foe, Gizmo: game.FourierMissiles, TargetEmpire: 1, From: gun.Pos, To: foe.Pos, Absorbed: 1, ShieldsDown: true},
		{Shooter: foe, Target: gun, Gizmo: game.Plasmatron, ShooterEmpire: 1, From: foe.Pos, To: gun.Pos, Damage: 4},
	} // a weapon each: a bullet's bank tells its shot; shotOrder's order (one side, the killer last)
	to := foe.Pos
	to.X += 0.1
	v.cues = nil
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: shots, Destroyed: []*game.Starship{gun},
		Moves: []game.Move{{Ship: foe, Empire: 1, From: foe.Pos, To: to, Orbit: -1}}}})
	if v.replay == nil {
		t.Fatal("no replay")
	}
	r := &v.replay.rounds[0]
	end := 0
	for k, f := range shots {
		_, n := impactOf(f)
		end = max(end, landsAt(k)+n*fxTicks)
	}
	if r.flight != flightTicks || r.blastStart() != end || r.moveStart() != end+destroyFrames*fxTicks || r.move != moveTicks {
		t.Fatalf("phases: flight %d impact %d (blasts at %d, want %d), kill %d, move %d", r.flight, r.impact, r.blastStart(), end, r.kill, r.move)
	}
	wantImpact := []string{shieldBank, "", shieldBank, explBank(game.Plasmatron)}
	of := func(tick, k int, kind fxKind) []effect {
		v.tick = v.replay.t0 + r.start + tick
		var out []effect
		for _, e := range v.effects() {
			if e.kind != kind {
				continue
			}
			for j, f := range shots { // a shot's effect: its bank, its target
				if j == k && e.ship != nil && e.ship.ship == f.Target && (kind == fxBullet && e.bank == bulletBank(f.Gizmo) || e.bank == wantImpact[k]) {
					out = append(out, e)
				}
			}
		}
		return out
	}
	for k := range shots {
		if shotAt(k) != k*flightTicks || landsAt(k) != shotAt(k+1) {
			t.Fatalf("shot %d: takes off at %d, lands at %d", k, shotAt(k), landsAt(k))
		}
		bullets := 0
		v.tick = v.replay.t0 + r.start + shotAt(k)
		for _, e := range v.effects() {
			if e.kind == fxBullet {
				bullets++
			}
		}
		if b := of(shotAt(k), k, fxBullet); bullets != 1 || len(b) != 1 || b[0].step != 0 || b[0].f != 0 {
			t.Errorf("shot %d's take-off: %d bullets, its own %+v", k, bullets, b)
		}
		if k > 0 && len(of(shotAt(k)-1, k, fxBullet)) != 0 {
			t.Errorf("shot %d's bullet a tick early", k)
		}
		imp := of(landsAt(k), k, fxImpact)
		if len(of(landsAt(k), k, fxBullet)) != 0 || wantImpact[k] != "" && (len(imp) != 1 || imp[0].step != 0) || wantImpact[k] == "" && len(imp) != 0 {
			t.Errorf("shot %d's landing: impacts %+v, want %q", k, imp, wantImpact[k])
		}
		take := cue{name: gunSound(shots[k].Gizmo), t: v.replay.t0 + r.start + shotAt(k)}
		if !slices.Contains(v.cues, take) {
			t.Errorf("shot %d: no %+v in %+v", k, take, v.cues)
		}
	}
	at := func(k int) int { return v.replay.t0 + r.start + landsAt(k) }
	for _, c := range []cue{{name: shieldSound, t: at(0)}, {name: shieldSound, t: at(2)}, {name: hitSound(game.Plasmatron), t: at(3)}} {
		if !slices.Contains(v.cues, c) {
			t.Errorf("no %+v in %+v", c, v.cues)
		}
	}
	if n := len(v.cues); n != 2*len(shots)+1 { // a gun each, a landing each but the last, the blast, the engine
		t.Errorf("%d cues, want %d: %+v", n, 2*len(shots)+1, v.cues)
	}
	v.tick = v.replay.t0 + r.start + r.blastStart() - 1
	for _, e := range v.effects() {
		if e.kind == fxBlast {
			t.Error("a blast before the last impact's end")
		}
	}
}

// A round of moves alone has no shots' phases; one of shots that did
// nothing still plays them one by one, the blasts after the last's
// landing.
func TestReplayPhasesEdges(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	to := foe.Pos
	to.X += 0.1
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Moves: []game.Move{{Ship: foe, Empire: 1, From: foe.Pos, To: to, Orbit: -1}}}})
	if r := v.replay.rounds[0]; r.flight != 0 || r.impact != 0 || r.kill != 0 || r.moveStart() != 0 || r.length() != moveTicks {
		t.Errorf("moves alone: flight %d impact %d kill %d, move start %d, length %d", r.flight, r.impact, r.kill, r.moveStart(), r.length())
	}
	v.replay = nil
	nothing := game.Fired{Shooter: gun, Target: foe, Gizmo: game.MassBarrageGun, TargetEmpire: 1, From: gun.Pos, To: foe.Pos}
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: []game.Fired{nothing, nothing}, Destroyed: []*game.Starship{foe}}})
	if r := v.replay.rounds[0]; r.flight != flightTicks || r.blastStart() != landsAt(1) || r.kill == 0 {
		t.Errorf("two empty shots: flight %d, blasts at %d, want %d; kill %d", r.flight, r.blastStart(), landsAt(1), r.kill)
	}
}

// Each impact kind's floating text: hull or structure damage "-N" red,
// all absorbed "0" light blue, a knock-down "shields down" yellow, a shot
// that did nothing none.
func TestFloatOf(t *testing.T) {
	for _, c := range []struct {
		f    game.Fired
		kind impactKind
		text string
		bank string
	}{
		{game.Fired{Gizmo: game.Plasmatron, Damage: 3}, hullHit, "-3", explBank(game.Plasmatron)},
		{game.Fired{AtPlanet: true, HitSlot: 2, HitItem: game.OrbitalShield, Damage: 5}, hullHit, "-5", explBank(game.MassBarrageGun)},
		{game.Fired{Absorbed: 1}, absorbed, "0", shieldBank},
		{game.Fired{Absorbed: 1, ShieldsDown: true}, knockDown, "shields down", shieldBank},
		{game.Fired{}, noImpact, "", ""},
	} {
		txt, col, ok := floatOf(c.f)
		bank, _ := impactOf(c.f)
		if k := kindOf(c.f); k != c.kind || txt != c.text || ok != (c.text != "") || ok && col != floatColour[k] || bank != c.bank {
			t.Errorf("%+v: kind %d %q %v (%v), bank %q; want %d %q %q", c.f, k, txt, col, ok, bank, c.kind, c.text, c.bank)
		}
	}
	red, blue, yel := floatColour[hullHit], floatColour[absorbed], floatColour[knockDown]
	if red.R <= red.B || blue.B <= blue.R || yel != yellow {
		t.Errorf("colours: hull %v, absorbed %v, knock-down %v", red, blue, yel)
	}
}

// A shot's floating text lives floatTicks from its landing, older ones
// still up as the next lands, over its target's body.
func TestFloats(t *testing.T) {
	a, v, _ := combatApp(t)
	gun := a.e.Ships[0]
	a.endDay()
	rp := v.replay
	if rp == nil {
		t.Fatal("no replay")
	}
	r0 := &rp.rounds[0]
	at := func(tick int) []floatText {
		v.tick = rp.t0 + r0.start + tick
		return v.floats()
	}
	if fs := at(landsAt(0) - 1); len(fs) != 0 {
		t.Errorf("before the first landing: %+v", fs)
	}
	t0, _, _ := floatOf(r0.log.Shots[0])
	if fs := at(landsAt(0)); len(fs) != 1 || fs[0].text != t0 || fs[0].age != 0 || fs[0].to == nil || fs[0].to.ship != r0.log.Shots[0].Target {
		t.Errorf("the first landing: %+v, want %q over its target", fs, t0)
	}
	t1, _, _ := floatOf(r0.log.Shots[1])
	if fs := at(landsAt(1)); len(fs) != 2 || fs[1].text != t1 || fs[0].age != flightTicks || fs[1].to.ship != gun {
		t.Errorf("the second landing: %+v, want %q too over the Gunboat", fs, t1)
	}
	if fs := at(landsAt(1) + floatTicks); len(fs) != 0 {
		t.Errorf("after their life: %+v", fs)
	}
}
