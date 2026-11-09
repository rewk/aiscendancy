package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// lineText is a round log line's texts joined, its flags' empires.
func lineText(l logLine) (string, []int) {
	var b strings.Builder
	var fl []int
	for _, p := range l.parts {
		b.WriteString(p.text + p.keep)
		fl = append(fl, p.empire)
	}
	return b.String(), fl
}

// logFixture is combatApp's screen with the Raider's cast Sighting set
// to sg, and a round of every event kind: ours on the Raider (hull),
// the Raider on ours (absorbed, then knocked down), our home's Missile
// Base on the Raider, the Raider on a structure of the home, and a shot
// that did nothing; the Raider destroyed and the structure razed.
func logFixture(t *testing.T, scan *game.Scan) (a *app, v *systemScreen, logs []game.RoundLog) {
	t.Helper()
	a, v, _ = combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	home := a.e.Colonies[0]
	mbg, pl := game.MassBarrageGun, home.PlanetIdx
	logs = []game.RoundLog{{Star: v.star,
		Shots: []game.Fired{
			{Shooter: gun, Target: foe, Gizmo: game.Plasmatron, TargetEmpire: 1, Damage: 3, Absorbed: 1}, // the shields took part
			{Shooter: foe, Target: gun, Gizmo: mbg, ShooterEmpire: 1, Absorbed: 1},
			{Shooter: foe, Target: gun, Gizmo: mbg, ShooterEmpire: 1, Absorbed: 1, ShieldsDown: true},
			{FromPlanet: true, ShooterPlanet: pl, ShooterSlot: 2, ShooterItem: game.OrbitalMissileBase, Target: foe, TargetEmpire: 1, Damage: 5},
			{Shooter: foe, Gizmo: mbg, ShooterEmpire: 1, AtPlanet: true, TargetPlanet: pl, HitSlot: 5, HitItem: game.OrbitalShield, Damage: 1},
			{Shooter: gun, Target: foe, Gizmo: mbg, TargetEmpire: 1},
		},
		Destroyed: []*game.Starship{foe},
		Razed:     []game.Razing{{Planet: pl, Slot: 5, Item: game.OrbitalShield}},
	}}
	cast := v.replayCast()
	i := slices.IndexFunc(cast, func(b body) bool { return b.ship == foe })
	cast[i].castSeen = &game.Sighting{Empire: 1, Hull: foe.Design.Hull, Scan: scan}
	v.startReplay(a.e, cast, logs)
	if v.replay == nil || v.rlog == nil {
		t.Fatal("no replay")
	}
	return a, v, logs
}

// Each event kind's line, text and colour; the foreign ship never by its
// name: scanned "<species> <design>" as the cast saw it, unscanned
// "<species> <hull>".
func TestRoundLogLines(t *testing.T) {
	for _, scan := range []*game.Scan{{Design: game.Design{Name: "Seen Design"}}, nil} {
		a, v, _ := logFixture(t, scan)
		gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
		p := v.sys.Planets[a.e.Colonies[0].PlanetIdx]
		them := v.g.Empires[1].Species.String() + " " + strings.ToLower(foe.Design.Hull.String())
		if scan != nil {
			them = v.g.Empires[1].Species.String() + " Seen Design"
		}
		mbg, pt := game.Gizmos[game.MassBarrageGun].Name, game.Gizmos[game.Plasmatron].Name
		want := []struct {
			text string
			fl   []int
			c    any
		}{
			{"Round 1", []int{-1}, textColour},
			// shotOrder's: the sides alternate, the Raider's killer last.
			{gun.Name + "  " + pt + " → " + them + "  -3 (1 absorbed)", []int{0, 1}, floatColour[hullHit]},
			{them + "  " + mbg + " → " + gun.Name + "  absorbed", []int{1, 0}, floatColour[absorbed]},
			{gun.Name + "  " + mbg + " → " + them + "  no effect", []int{0, 1}, textColour},
			{them + "  " + mbg + " → " + gun.Name + "  shields down", []int{1, 0}, floatColour[knockDown]},
			{them + "  " + mbg + " → " + p.Name + "  -1 " + game.OrbitalShield.String(), []int{1, 0}, floatColour[hullHit]},
			{p.Name + " (orbital)  " + game.OrbitalMissileBase.String() + " → " + them + "  -5", []int{0, 1}, floatColour[hullHit]},
			{them + " destroyed", []int{1}, logWhite},
			{game.OrbitalShield.String() + " razed", []int{p.Owner}, logWhite},
		}
		ls := v.rlog.lines
		if len(ls) != len(want) {
			t.Fatalf("scan %v: %d lines, want %d", scan != nil, len(ls), len(want))
		}
		for i, w := range want {
			txt, fl := lineText(ls[i])
			if txt != w.text || !slices.Equal(fl, w.fl) || any(ls[i].c) != w.c {
				t.Errorf("scan %v line %d: %q %v %v, want %q %v %v", scan != nil, i, txt, fl, ls[i].c, w.text, w.fl, w.c)
			}
			if strings.Contains(txt, foe.Name) {
				t.Errorf("line %q names the foreign ship", txt)
			}
		}
	}
}

// A line shows at its event: a shot at its landing (each weapon's own
// flight, gunTicks), the header with the first, the blasts as they
// start; the overlay stays past the replay's end, while the screen does.
func TestRoundLogTiming(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.gunTicks = map[game.Gizmo]int{game.MassBarrageGun: 23, game.Plasmatron: 41}
	v.rebuild()
	shots := []game.Fired{
		{Shooter: gun, Target: foe, Gizmo: game.Plasmatron, TargetEmpire: 1, Damage: 3},
		{Shooter: foe, Target: gun, Gizmo: game.MassBarrageGun, ShooterEmpire: 1, Damage: 1},
		{Shooter: gun, Target: foe, Gizmo: game.MassBarrageGun, TargetEmpire: 1, Absorbed: 1},
	}
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: shots, Destroyed: []*game.Starship{gun}}})
	rp, r := v.replay, &v.replay.rounds[0]
	if r.flightOf(0) != 41 || r.flightOf(1) != 23 {
		t.Fatalf("flights %d %d", r.flightOf(0), r.flightOf(1))
	}
	at := func(k int) int { return rp.t0 + r.start + r.landsAt(k) }
	wantT := []int{at(0), at(0), at(1), at(2), rp.t0 + r.start + r.blastStart()}
	for i, l := range v.rlog.lines {
		if l.t != wantT[i] {
			t.Errorf("line %d at %d, want %d", i, l.t, wantT[i])
		}
		if ls := v.rlog.shown(l.t - 1); len(ls) > i {
			t.Errorf("line %d shown a tick early", i)
		}
		if ls := v.rlog.shown(l.t); len(ls) < i+1 {
			t.Errorf("line %d not shown at its tick", i)
		}
	}
	end := rp.t0 + rp.length()
	v.tick = end
	v.endReplay(a)
	if ls := v.rlog.shown(end + 60*60); len(ls) != 5 {
		t.Errorf("a minute past the replay: %d lines, want the 5 kept", len(ls))
	}
}

// At most rlogMax lines, the newest; a skip shows them all at once.
func TestRoundLogCapAndSkip(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	var shots []game.Fired
	for k := range 12 {
		shots = append(shots, game.Fired{Shooter: gun, Target: foe, Gizmo: game.MassBarrageGun, TargetEmpire: 1, Damage: k + 1})
	}
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: shots}})
	if ls := v.rlog.shown(v.tick); len(ls) != 0 {
		t.Errorf("%d lines before any landing", len(ls))
	}
	last := v.rlog.lines[len(v.rlog.lines)-1]
	ls := v.rlog.shown(last.t)
	if len(ls) != rlogMax || ls[rlogMax-1].t != last.t {
		t.Fatalf("all landed: %d lines, want the last %d", len(ls), rlogMax)
	}
	if txt, _ := lineText(ls[0]); !strings.HasSuffix(txt, fmt.Sprintf("-%d", 12-rlogMax+1)) {
		t.Errorf("oldest shown %q, want shot %d's", txt, 12-rlogMax+1)
	}

	a, v, _ = combatApp(t)
	v.rebuild()
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: shots}})
	v.tick += 3
	if !v.skipReplay(a) || v.replay != nil {
		t.Fatal("the skip did not end the replay")
	}
	if ls := v.rlog.shown(v.tick); len(ls) != rlogMax {
		t.Errorf("after a skip: %d lines", len(ls))
	}
}

// A round stepped while the replay runs adds to the same overlay, its
// header the day's round number (RoundsAt); so does one after the replay
// ended: the overlay stays while the screen does, a new screen starts
// empty.
func TestRoundLogAppends(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	shot := game.Fired{Shooter: gun, Target: foe, Gizmo: game.MassBarrageGun, TargetEmpire: 1, Damage: 1}
	v.g.RoundsToday = make([]int, len(v.g.Cluster.Stars))
	v.g.RoundsToday[v.star] = 1
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: []game.Fired{shot}}})
	rl := v.rlog
	v.tick += 2
	v.g.RoundsToday[v.star] = 2
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: []game.Fired{shot}}})
	var heads []string
	for _, l := range rl.lines {
		if txt, _ := lineText(l); strings.HasPrefix(txt, "Round ") {
			heads = append(heads, txt)
		}
	}
	if v.rlog != rl || len(rl.lines) != 4 || !slices.Equal(heads, []string{"Round 1", "Round 2"}) {
		t.Errorf("appended: same %v, %d lines, heads %v", v.rlog == rl, len(rl.lines), heads)
	}
	// The day's close at once: three logs at the star, the last two
	// replayed here numbered after RoundsAt's earlier rounds.
	v.skipReplay(a)
	v.tick += 60 * 60
	v.g.RoundsToday[v.star] = 5
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star}, {Star: v.star + 1, Shots: []game.Fired{shot}}, {Star: v.star, Shots: []game.Fired{shot}}, {Star: v.star, Shots: []game.Fired{shot}}})
	heads = nil
	for _, l := range v.rlog.lines[4:] {
		if txt, _ := lineText(l); strings.HasPrefix(txt, "Round ") {
			heads = append(heads, txt)
		}
	}
	if v.rlog != rl || !slices.Equal(heads, []string{"Round 4", "Round 5"}) {
		t.Errorf("the day's close, a minute on: same %v, heads %v, want Round 4, Round 5 added", v.rlog == rl, heads)
	}
	if w := newSystemScreen(v.star); w.rlog != nil {
		t.Error("a new system screen starts with a log")
	}
}

// The overlay renders with the game's art over the replay, every tick
// and past its end.
func TestRoundLogRenders(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	a, v, _ := logFixture(t, nil)
	a.a = as
	a.preloadGame()
	frame := ebiten.NewImage(screenW, screenH)
	end := v.replay.t0 + v.replay.length() + 60
	for tk := v.replay.t0; tk <= end; tk += 3 {
		v.tick = tk
		if v.replay != nil && tk >= v.replay.t0+v.replay.length() {
			v.endReplay(a)
		}
		v.rebuild()
		v.Render(a, frame)
		v.Text(a, frame)
	}
}

// A replay dropped without its end (beginDay: a new day) leaves no line
// waiting for a tick of its own: the next replay's overlay shows them
// all at once before its own.
func TestRoundLogDroppedReplay(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	shot := game.Fired{Shooter: gun, Target: foe, Gizmo: game.MassBarrageGun, TargetEmpire: 1, Damage: 1}
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: []game.Fired{shot, shot}}})
	v.tick++
	v.replay = nil
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: []game.Fired{shot}}})
	if ls := v.rlog.shown(v.tick); len(ls) != 3 {
		t.Errorf("%d lines shown at the new replay's start, want the dropped one's 3", len(ls))
	}
}

// Lines stay within half the view, right-aligned: a long one loses its
// shooter's side first, its target and outcome kept whole.
func TestRoundLogLayout(t *testing.T) {
	long := logLine{parts: []logPart{{0, strings.Repeat("S", 60), " → "}, {1, "Target  -3", ""}}}
	short := logLine{parts: []logPart{{-1, "Round 1", ""}}}
	ps := layLog([]logLine{short, long}, 15, 25)
	right, half := float64(sysArea.X1-padX), float64(sysArea.X1-sysArea.X0)/2
	for i, p := range ps {
		end := p.textX[len(p.textX)-1] + ui.Width(p.texts[len(p.texts)-1], smallFace)
		if end != right || right-p.plate.x0 > half+3 || p.y != float64(sysArea.Y0+padY)+float64(i)*rlogLineH(25) {
			t.Errorf("line %d: ends %v (want %v), %v wide, at %v", i, end, right, right-p.plate.x0, p.y)
		}
	}
	if p := ps[1]; p.texts[1] != "Target  -3" || len(p.texts[0]) >= len(long.parts[0].text) || !strings.HasSuffix(p.texts[0], " → ") || p.flagX[0] < 0 || p.flagX[1] != p.textX[0]+ui.Width(p.texts[0], smallFace) {
		t.Errorf("the long line: %q, flags at %v", p.texts, p.flagX)
	}
	if ps[0].flagX[0] != -1 {
		t.Error("a header with a flag")
	}
}

// A special's line: violet, "<user>  <gadget> → <target>  <outcome>", a
// planet target "→ <planet>", none "<user>  <gadget>  <outcome>"; a
// foreign user only when the player saw it (its cast Sighting), a
// foreign ship never by its name; another star's none.
func TestUseLines(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	cast := v.replayCast()
	i := slices.IndexFunc(cast, func(b body) bool { return b.ship == foe })
	cast[i].castSeen = &game.Sighting{Empire: 1, Hull: foe.Design.Hull}
	them := v.g.Empires[1].Species.String() + " " + strings.ToLower(foe.Design.Hull.String())
	p := v.sys.Planets[0]
	for _, c := range []struct {
		u    game.SpecialUse
		text string
		fl   []int
	}{
		{game.SpecialUse{User: gun, UserEmpire: 0, Gizmo: game.Disarmer, Star: v.star, Kind: game.ShipTarget, Target: foe, TargetEmpire: 1, Outcome: "Ueberlaser destroyed"},
			gun.Name + "  Disarmer → " + them + "  Ueberlaser destroyed", []int{0, 1}},
		{game.SpecialUse{User: foe, UserEmpire: 1, Gizmo: game.ShieldBlaster, Star: v.star, Kind: game.ShipTarget, Target: gun, TargetEmpire: 0, Outcome: "shields down"},
			them + "  Shield Blaster → " + gun.Name + "  shields down", []int{1, 0}},
		{game.SpecialUse{User: gun, UserEmpire: 0, Gizmo: game.PhaseBomb, Star: v.star, Kind: game.PlanetTarget, Index: 0, TargetEmpire: -1, Outcome: "Factory razed"},
			gun.Name + "  Phase Bomb → " + p.Name + "  Factory razed", []int{0, p.Owner}},
		{game.SpecialUse{User: gun, UserEmpire: 0, Gizmo: game.Accutron, Star: v.star, Kind: game.NoTarget, TargetEmpire: -1, Outcome: "range doubled"},
			gun.Name + "  Accutron  range doubled", []int{0}},
	} {
		l, ok := v.useLine(cast, &c.u, 7, nil)
		txt, fl := lineText(l)
		if !ok || txt != c.text || !slices.Equal(fl, c.fl) || l.c != useColour || l.t != 7 || strings.Contains(txt, foe.Name) {
			t.Errorf("%s: %v %q %v %v, want %q %v", game.Gizmos[c.u.Gizmo].Name, ok, txt, fl, l.c, c.text, c.fl)
		}
	}
	unseen := game.SpecialUse{User: foe, UserEmpire: 1, Gizmo: game.Accutron, Star: v.star, TargetEmpire: -1, Outcome: "range doubled"}
	cast[i].castSeen = nil
	if _, ok := v.useLine(cast, &unseen, 0, nil); ok {
		t.Error("an unseen user's use shown")
	}
	elsewhere := game.SpecialUse{User: gun, UserEmpire: 0, Gizmo: game.Accutron, Star: v.star + 1, TargetEmpire: -1, Outcome: "range doubled"}
	if _, ok := v.useLine(cast, &elsewhere, 0, nil); ok {
		t.Error("another star's use shown")
	}
	if us := specialUses([]game.Report{{Event: game.Event{Use: &unseen}}, {Event: game.Event{Use: &unseen}}, {}}); len(us) != 1 || us[0] != &unseen {
		t.Errorf("specialUses %v: the user's and the victim's copies are one use", us)
	}
}

// A bot's use before a round shows at the round's start, with its header,
// before the first shot's landing; one before a round with nothing to
// replay shows where that round would start, under its header.
func TestUseAtRoundStart(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	cast := v.replayCast()
	i := slices.IndexFunc(cast, func(b body) bool { return b.ship == foe })
	cast[i].castSeen = &game.Sighting{Empire: 1, Hull: foe.Design.Hull}
	u := &game.SpecialUse{User: foe, UserEmpire: 1, Gizmo: game.ShieldBlaster, Star: v.star, Kind: game.ShipTarget, Target: gun, TargetEmpire: 0, Outcome: "shields down"}
	shot := game.Fired{Shooter: gun, Target: foe, Gizmo: game.MassBarrageGun, TargetEmpire: 1, Damage: 1}
	v.startReplay(a.e, cast, []game.RoundLog{{Star: v.star}, {Star: v.star, Shots: []game.Fired{shot}}},
		[]*game.SpecialUse{u}, []*game.SpecialUse{u})
	rp := v.replay
	r := &rp.rounds[0]
	var got []string
	var ts []int
	for _, l := range v.rlog.lines {
		txt, _ := lineText(l)
		got, ts = append(got, txt), append(ts, l.t)
	}
	at := rp.t0 + r.start
	if len(got) != 5 || got[0] != "Round 1" || got[2] != "Round 2" || v.rlog.lines[1].c != useColour || v.rlog.lines[3].c != useColour ||
		!slices.Equal(ts, []int{at, at, at, at, at + r.landsAt(0)}) {
		t.Errorf("lines %q at %v (round start %d)", got, ts, at)
	}
	if ls := v.rlog.shown(at); len(ls) != 4 {
		t.Errorf("%d lines at the round's start, want the two headers and uses", len(ls))
	}
}

// A bot's special in a stepped round reaches the round log (bot.Step's
// reps), and in the day's close (DayRun.Round's), at that round's start.
func TestUseFromBots(t *testing.T) {
	for _, close := range []bool{false, true} {
		a, v, gun := roundApp(t)
		foe := a.g.Empires[1].Ships[0]
		foe.Design.Parts = append(slices.Clone(foe.Design.Parts), game.ShieldBlaster, game.TonklinFrequencyAnalyzer)
		foe.Shots = make([]int, len(foe.Design.Parts))
		foe.Power = 20
		gun.Shields, gun.ShieldsDown = true, false
		v.rebuild()
		if close {
			a.endDay()
		} else {
			a.stepRound(v)
		}
		if !close && foe.Shots[slices.Index(foe.Design.Parts, game.ShieldBlaster)] != 1 { // the close refills
			t.Fatalf("close %v: the bot did not blast: shots %v power %d, gun shields %v down %v, lines %d, day %d", close, foe.Shots, foe.Power, gun.Shields, gun.ShieldsDown, len(v.rlog.lines), a.e.Day)
		}
		i := slices.IndexFunc(v.rlog.lines, func(l logLine) bool { return l.c == useColour })
		if i < 0 {
			t.Fatalf("close %v: no use line in %+v", close, v.rlog.lines)
		}
		if r := v.replay.rounds[0]; v.rlog.lines[i].t != v.replay.t0+r.start || i != 1 {
			t.Errorf("close %v: the use line %d at %d, want right after the header at the round's start %d", close, i, v.rlog.lines[i].t, v.replay.t0+r.start)
		}
		if txt, _ := lineText(v.rlog.lines[i]); !strings.Contains(txt, "Shield Blaster → "+gun.Name+"  shields down") || strings.Contains(txt, foe.Name) {
			t.Errorf("close %v: %q", close, txt)
		}
	}
}

// The player's use noted during a replay shows at once, not after the
// replay's lines still to come.
func TestUseDuringReplay(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	shot := game.Fired{Shooter: gun, Target: foe, Gizmo: game.MassBarrageGun, TargetEmpire: 1, Damage: 1}
	v.startReplay(a.e, v.replayCast(), []game.RoundLog{{Star: v.star, Shots: []game.Fired{shot}}})
	v.noteUse(&game.SpecialUse{User: gun, UserEmpire: 0, Gizmo: game.Accutron, Star: v.star, TargetEmpire: -1, Outcome: "range doubled"})
	if ls := v.rlog.shown(v.tick); len(ls) != 1 || ls[0].c != useColour {
		t.Errorf("shown now %+v, want the use alone", ls)
	}
}

// A special used before a round plays its effects at the round's start,
// a round of its own: a ship it destroyed blows up at once (destroy.shp,
// its sprite gone), a ship it moved tweens From→To; the round's shots
// then fly from the new place. An unseen user's use plays nothing.
func TestUseEffectsReplay(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	seen := func(seen bool) []body {
		cast := v.replayCast()
		i := slices.IndexFunc(cast, func(b body) bool { return b.ship == foe })
		cast[i].castSeen = nil
		if seen {
			cast[i].castSeen = &game.Sighting{Empire: 1, Hull: foe.Design.Hull}
		}
		return cast
	}
	to := foe.Pos
	to.X += 0.2
	hop := &game.SpecialUse{User: foe, UserEmpire: 1, Gizmo: game.GravimetricCatapult, Star: v.star, TargetEmpire: -1, Outcome: "round the star",
		Moves: []game.Move{{Ship: foe, Empire: 1, From: foe.Pos, To: to, Orbit: -1}}}
	shot := game.Fired{Shooter: foe, Target: gun, Gizmo: game.MassBarrageGun, ShooterEmpire: 1, From: to, To: gun.Pos, Damage: 1}
	v.startReplay(a.e, seen(true), []game.RoundLog{{Star: v.star, Shots: []game.Fired{shot}}}, []*game.SpecialUse{hop})
	rp := v.replay
	if len(rp.rounds) != 2 || rp.rounds[0].move != moveTicks || rp.rounds[1].start != rp.rounds[0].length() {
		t.Fatalf("rounds %d: want the use's then the shots'", len(rp.rounds))
	}
	pos := func(tick int) game.Vec3 {
		v.tick = rp.t0 + tick
		v.rebuild()
		return drawnBody(t, v.bodies, foe).pos
	}
	if p := pos(0); p != hop.Moves[0].From {
		t.Errorf("the use's start: at %v, want %v", p, hop.Moves[0].From)
	}
	if p := pos(moveTicks / 2); p == hop.Moves[0].From || p == to {
		t.Errorf("mid-tween at %v", p)
	}
	r1 := &rp.rounds[1]
	v.tick = rp.t0 + r1.start
	fx := v.effects()
	if p := pos(r1.start); p != to || len(fx) != 1 || fx[0].kind != fxBullet || fx[0].fromPos != to {
		t.Errorf("the shot: Raider at %v, effects %+v; want a bullet from %v", p, fx, to)
	}
	if !slices.Contains(v.cues, cue{name: thrustSound(foe.Design.Hull), t: rp.t0}) {
		t.Errorf("the use's move not cued: %+v", v.cues)
	}

	// A kill: the blast at the use's tick, the ship gone after.
	a, v, _ = combatApp(t)
	gun, foe = a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	kill := &game.SpecialUse{User: foe, UserEmpire: 1, Gizmo: game.Disintegrator, Star: v.star, Kind: game.ShipTarget, Target: gun, TargetEmpire: 0,
		Outcome: "destroyed", Destroyed: []*game.Starship{gun}}
	v.startReplay(a.e, seen(true), []game.RoundLog{{Star: v.star}}, []*game.SpecialUse{kill})
	rp = v.replay
	if rp == nil || len(rp.rounds) != 1 || rp.rounds[0].kill == 0 {
		t.Fatal("no blast round for the kill")
	}
	v.tick = rp.t0
	v.rebuild()
	if fx := v.effects(); len(fx) != 1 || fx[0].kind != fxBlast || fx[0].ship == nil || fx[0].ship.ship != gun || fx[0].step != 0 ||
		slices.ContainsFunc(v.bodies, func(b body) bool { return b.ship == gun }) {
		t.Errorf("the kill's blast %+v", fx)
	}

	// The same kill by an unseen user: neither line nor blast.
	a, v, _ = combatApp(t)
	gun, foe = a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	kill.User, kill.Target, kill.Destroyed = foe, gun, []*game.Starship{gun}
	v.startReplay(a.e, seen(false), []game.RoundLog{{Star: v.star}}, []*game.SpecialUse{kill})
	if v.replay != nil || len(v.rlog.lines) != 0 {
		t.Errorf("an unseen user's kill: replay %v, %d lines", v.replay != nil, len(v.rlog.lines))
	}
}

// The player's own use out of a replay plays its effects at once over
// the ships as drawn before it: the target it destroyed blows up now,
// the line shown now.
func TestOwnUseEffects(t *testing.T) {
	a, v, _ := combatApp(t)
	gun, foe := a.e.Ships[0], v.g.Empires[1].Ships[0]
	v.rebuild()
	v.noteUse(&game.SpecialUse{User: gun, UserEmpire: 0, Gizmo: game.Disintegrator, Star: v.star, Kind: game.ShipTarget, Target: foe, TargetEmpire: 1,
		Outcome: "destroyed", Destroyed: []*game.Starship{foe}})
	if v.replay == nil || v.replay.t0 != v.tick || v.replay.fight {
		t.Fatalf("no replay of the use at once (fight %v)", v.replay != nil && v.replay.fight)
	}
	if fx := v.effects(); len(fx) != 1 || fx[0].kind != fxBlast || fx[0].ship.ship != foe {
		t.Errorf("the blast now: %+v", fx)
	}
	if ls := v.rlog.shown(v.tick); len(ls) != 1 || ls[0].c != useColour {
		t.Errorf("the line now: %+v", ls)
	}
}
