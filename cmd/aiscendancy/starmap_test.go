package main

import (
	"fmt"
	"image/color"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/cam"
	"github.com/rewk/aiscendancy/game"
)

func TestBrightness(t *testing.T) {
	const dist = 4
	cases := []struct {
		depth float64
		want  int
	}{
		{dist - 1, 3}, {dist - 0.75, 3}, {dist - 0.5, 2}, {dist - 0.01, 2},
		{dist, 1}, {dist + 0.49, 1}, {dist + 0.5, 0}, {dist + 1, 0},
		{dist - 3, 3}, {dist + 3, 0},
	}
	for _, c := range cases {
		if got := brightness(c.depth, dist); got != c.want {
			t.Errorf("brightness(%v, %v) = %d, want %d", c.depth, dist, got, c.want)
		}
	}
}

func TestStarFrame(t *testing.T) {
	if starFrame(0, 0) != 0 || starFrame(3, 3) != 15 || starFrame(4, 0) != 16 || starFrame(12, 3) != 51 {
		t.Error("cos_star frames are 4t+level")
	}
}

func TestPickStar(t *testing.T) {
	// Native px: the reach is 6 original px.
	ps := []cam.Point{{X: 100, Y: 100}, {X: 100 + 4*zoom, Y: 100}, {X: 200, Y: 200}}
	cases := []struct {
		x, y float64
		want int
	}{
		{100 + zoom, 100, 0}, {100 + 3*zoom, 100, 1}, {200, 200 + 6*zoom, 2}, {200, 200 + 6*zoom + 0.1, -1}, {150, 150, -1},
	}
	for _, c := range cases {
		if got := pickStar(ps, c.x, c.y); got != c.want {
			t.Errorf("pickStar(%v,%v) = %d, want %d", c.x, c.y, got, c.want)
		}
	}
	if pickStar(nil, 0, 0) != -1 {
		t.Error("hit on nothing")
	}
}

// The default camera puts the unit ball inside the big area and the depth
// span on [Dist−1, Dist+1], which brightness assumes.
func TestDefaultCamera(t *testing.T) {
	c := mapCamera()
	for _, p := range []game.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		pt := c.Project(p)
		if !mapArea.has(pt.X, pt.Y) || pt.Depth < c.Dist-1-1e-9 || pt.Depth > c.Dist+1+1e-9 {
			t.Errorf("%v projects to %+v, outside the area or the depth span", p, pt)
		}
	}
	c.Pitch, c.Focal = -2, 1
	clampMapCamera(&c)
	if c.Pitch != minPitch || c.Focal != cam.MinFocal {
		t.Errorf("clamped to pitch %v focal %v", c.Pitch, c.Focal)
	}
	c.Pitch, c.Focal = 2, 1e6
	clampMapCamera(&c)
	if c.Pitch != maxPitch || c.Focal != cam.MaxFocal || minPitch >= 0 {
		t.Errorf("clamped to pitch %v focal %v", c.Pitch, c.Focal)
	}
}

func TestTextFits(t *testing.T) {
	h := float64(mapStack[0].Y1 - mapStack[0].Y0)
	if linesFit(h, bigFace) != 2 || linesFit(h, smallFace) != 3 {
		t.Errorf("stacked box: %d lines at 16 px, %d at 12", linesFit(h, bigFace), linesFit(h, smallFace))
	}
	if n := linesFit(float64(mapTop.Y1-mapTop.Y0), bigFace); n < 2 {
		t.Errorf("top box: %d lines", n)
	}
	if clip("a\nb\nc", 2) != "a\nb" || clip("a", 3) != "a" {
		t.Error("clip")
	}
}

// The measured boxes: the five stacked ones 8 px apart, all right of the
// big area.
func TestBoxes(t *testing.T) {
	for k, r := range mapStack {
		if r.Y1-r.Y0 != zoom*31 || r.X0 < mapArea.X1 || (k > 0 && r.Y0-mapStack[k-1].Y1 != zoom*8) {
			t.Errorf("box %d: %+v", k, r)
		}
	}
}

func TestFade(t *testing.T) {
	const dist = 4
	if f := fade(dist-1, dist); f != 1 {
		t.Errorf("near edge: %v", f)
	}
	if f := fade(dist+1, dist); math.Abs(f-fadeFloor) > 1e-9 {
		t.Errorf("far edge: %v", f)
	}
	if f := fade(dist, dist); f <= fadeFloor || f >= 1 {
		t.Errorf("middle: %v", f)
	}
	if f := fade(dist+5, dist); f != fadeFloor {
		t.Errorf("beyond: %v", f)
	}
	if c := shade(color.RGBA{200, 100, 50, 255}, 0.5); c != (color.RGBA{100, 50, 25, 255}) {
		t.Errorf("shade: %v", c)
	}
}

// The turn buttons answer inside their measured holes only, and lie
// outside the big area (a press there is never an area click).
func TestTurnButtons(t *testing.T) {
	s := newStarmapScreen()
	for _, c := range []struct {
		r    rect
		want int
	}{{mapNext, nextButton}, {mapSkip, skipButton}} {
		if c.r.X0 < mapArea.X1 {
			t.Errorf("%+v overlaps the big area", c.r)
		}
		for _, p := range [][2]float64{
			{float64(c.r.X0), float64(c.r.Y0)},
			{float64(c.r.X1) - 0.5, float64(c.r.Y1) - 0.5},
		} {
			if got := s.button(p[0], p[1]); got != c.want {
				t.Errorf("button(%v) = %d, want %d", p, got, c.want)
			}
		}
		for _, p := range [][2]float64{
			{float64(c.r.X0) - 0.5, float64(c.r.Y0)},
			{float64(c.r.X1), float64(c.r.Y0)},
			{float64(c.r.X0), float64(c.r.Y1)},
		} {
			if got := s.button(p[0], p[1]); got != noButton {
				t.Errorf("button(%v) = %d outside the hole", p, got)
			}
		}
	}
	if s.button(float64(mapArea.X0+10), float64(mapArea.Y0+10)) != noButton {
		t.Error("a button in the big area")
	}
}

// Stacked box 0 is the Planets button, 1 the Ships, 2 the Research, 4
// the Species; their presses push their screens (box 3, the Special
// Ability, is TestAbilityBox; the Designs button is the fleet's).
func TestStackButtons(t *testing.T) {
	for _, c := range []struct {
		r      rect
		button int
		pushed func(screen) bool
	}{
		{mapStack[shipButton], shipsButton, func(s screen) bool { _, ok := s.(*fleetScreen); return ok }},
		{mapStack[resButton], researchButton, func(s screen) bool { _, ok := s.(*researchScreen); return ok }},
		{mapStack[planetButton], planetsButton, func(s screen) bool { _, ok := s.(*planetsScreen); return ok }},
		{mapStack[speciesBox], speciesButton, func(s screen) bool { _, ok := s.(*speciesScreen); return ok }},
	} {
		s := newStarmapScreen()
		r := c.r
		for _, p := range [][2]float64{
			{float64(r.X0), float64(r.Y0)},
			{float64(r.X1) - 0.5, float64(r.Y1) - 0.5},
		} {
			if got := s.button(p[0], p[1]); got != c.button {
				t.Errorf("button(%v) = %d, want %d", p, got, c.button)
			}
		}
		for _, p := range [][2]float64{
			{float64(r.X0) - 0.5, float64(r.Y0)},
			{float64(r.X1), float64(r.Y0)},
		} {
			if got := s.button(p[0], p[1]); got != noButton {
				t.Errorf("button(%v) = %d outside box %v", p, got, c.r)
			}
		}
		g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
		e := g.Empires[0]
		a := testApp(g)
		a.push(s)
		s.press(a, s.button(float64(r.X0+5), float64(r.Y0+5)))
		if !c.pushed(a.top()) || len(a.stack) != 2 || e.Day != 0 {
			t.Errorf("box %v: stack %v, day %d", c.r, a.stack, e.Day)
		}
	}
	// The gaps between the boxes and the news box are no button.
	s := newStarmapScreen()
	if got := s.button(float64(newsBox.X0+5), float64(newsBox.Y0+5)); got != noButton {
		t.Errorf("the news box: button %d", got)
	}
	for _, y := range []float64{float64(mapStack[1].Y0) - 0.5, float64(mapStack[1].Y1), float64(mapStack[2].Y1), float64(mapStack[3].Y1)} {
		if got := s.button(float64(mapStack[1].X0+5), y); got != noButton {
			t.Errorf("button at y %v = %d, between the boxes", y, got)
		}
	}
}

// The counter: five digit frames, zero-padded, filling the top box less
// 2 px a side.
func TestDayFrames(t *testing.T) {
	cases := []struct {
		day  int
		want [dayDigits]int
	}{
		{0, [dayDigits]int{55, 55, 55, 55, 55}},
		{340, [dayDigits]int{55, 55, 58, 59, 55}},
		{12345, [dayDigits]int{56, 57, 58, 59, 60}},
		{123456, [dayDigits]int{64, 64, 64, 64, 64}},
	}
	for _, c := range cases {
		if got := dayFrames(c.day); got != c.want {
			t.Errorf("dayFrames(%d) = %v, want %v", c.day, got, c.want)
		}
	}
	if w := mapTop.X1 - mapTop.X0 - zoom*dayDigits*digitW; w != zoom*4 {
		t.Errorf("top box less the digits: %d px", w)
	}
}

// The key help holds its three lines at 12 px.

// The fog on the starmap (game/fog.go): a fresh game's player sees the
// lanes touching its home and no other; an unknown star's box shows its
// type, "unexplored" and its known lanes, its planet list the one line
// "unexplored"; a known star keeps its name and planets.
func TestStarmapFog(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
	e, cl := g.Empires[0], g.Cluster
	home := e.Colonies[0].Star
	var want []int
	for li, l := range cl.Lanes {
		if l.A == home || l.B == home {
			want = append(want, li)
		}
	}
	if got := knownLanes(e, cl); len(want) == 0 || !slices.Equal(got, want) {
		t.Errorf("drawn lanes %v, want the home's %v", got, want)
	}
	if got := knownLanes(nil, cl); len(got) != len(cl.Lanes) {
		t.Errorf("no empire: %d lanes drawn, want all %d", len(got), len(cl.Lanes))
	}

	// A neighbour of the home: unknown, one known lane (the home's) among
	// its own.
	far := cl.Other(cl.Stars[home].Lanes[0], home)
	if e.Knows(far) {
		t.Fatalf("star %d known on day 0", far)
	}
	st := cl.Stars[far]
	txt := starText(g, e, far)
	if !strings.HasPrefix(txt, st.Type.String()+", ") || !strings.Contains(txt, "unexplored, 1 lanes") || strings.Contains(txt, st.Name) {
		t.Errorf("unknown star %d (%d lanes): %q", far, len(st.Lanes), txt)
	}
	// The home: its name, its planet count.
	if txt := starText(g, e, home); !strings.HasPrefix(txt, cl.Stars[home].Name+", ") ||
		!strings.HasSuffix(txt, fmt.Sprintf("%d planets, %d lanes, owned by %s", len(cl.Stars[home].System.Planets), len(cl.Stars[home].Lanes), e.Species)) {
		t.Errorf("home star: %q", txt)
	}
	// The marks: the home is colonised by empire 0, an explored star
	// without a colony gets the dots, an unexplored one nothing.
	if m, c := starMark(e, g, cl, home); !m || !slices.Equal(c, []int{0}) {
		t.Errorf("home mark %v %d", m, c)
	}
	if m, c := starMark(e, g, cl, far); m || c != nil {
		t.Errorf("unexplored mark %v %d", m, c)
	}
	// The flags: the player's home only, then the second empire's once
	// its home star is explored.
	home1 := g.Empires[1].Colonies[0].Star
	if fl := homeFlags(e, g); len(fl) != 1 || fl[0] != (homeFlag{home, e.Species, e.Colour}) {
		t.Errorf("flags on day 0: %v", fl)
	}
	e.Visit(home1)
	if fl := homeFlags(e, g); len(fl) != 2 || fl[1] != (homeFlag{home1, g.Empires[1].Species, g.Empires[1].Colour}) {
		t.Errorf("flags after exploring %d: %v", home1, fl)
	}
	if homeFlags(e, nil) != nil {
		t.Error("flags without a game")
	}
	// Drawn far to near with their stars: the farther home first,
	// whatever the empire order.
	ps := make([]cam.Point, len(cl.Stars))
	ps[home].Depth, ps[home1].Depth = 1, 2
	before := func(i, j int) bool {
		o := mapOrder(cl, ps, nil, nil)
		return slices.Index(o, mapItem{itemStar, i, 0}) < slices.Index(o, mapItem{itemStar, j, 0})
	}
	if !before(home1, home) {
		t.Errorf("order %v", mapOrder(cl, ps, nil, nil))
	}
	ps[home].Depth = 3
	if !before(home, home1) {
		t.Errorf("order %v", mapOrder(cl, ps, nil, nil))
	}
	// A click on it opens nothing; after a ship explores it, its system.
	a := testApp(g)
	a.open([]screen{newStarmapScreen()})
	m := top[*starmapScreen](t, a)
	p := m.project(a)[far]
	m.click(a, p.X, p.Y)
	if len(a.stack) != 1 {
		t.Errorf("a click on an unexplored star pushed %T", a.top())
	}
	// A ship there explores it: its name, its planets, all its lanes.
	e.Spawn(fixtureShip, far, 0)
	if m, c := starMark(e, g, cl, far); !m || c != nil {
		t.Errorf("explored mark %v %d", m, c)
	}
	m.click(a, p.X, p.Y)
	if sys, ok := a.top().(*systemScreen); !ok || sys.star != far {
		t.Errorf("a click on the explored star: top %T", a.top())
	}
	if txt := starText(g, e, far); !strings.HasPrefix(txt, st.Name+", ") || !strings.HasSuffix(txt, fmt.Sprintf(", %d lanes", len(st.Lanes))) {
		t.Errorf("explored star: %q", txt)
	}
}

// Box 3 is the Special Ability: its lines say the ability and its state,
// a press uses it (a Baliflids player at war forces peace), a passive
// species' or a recharging one's press only toasts.
func TestAbilityBox(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Baliflids, game.Orfa}, game.Average)
	g.Contact(0, 1)
	if _, err := g.DeclareWar(1, 0); err != nil {
		t.Fatal(err)
	}
	charge(g, 0)
	a := testApp(g)
	s := newStarmapScreen()
	a.push(s)
	r := mapStack[abilityBox]
	if b := s.button(float64(r.X0+5), float64(r.Y0+5)); b != abilityButton {
		t.Fatalf("button %d", b)
	}
	if ls := abilityLines(a); ls[0] != game.Baliflids.Info().AbilityText || ls[1] != "ready" {
		t.Errorf("lines %q", ls)
	}
	s.press(a, abilityButton)
	if g.Empires[0].StanceWith(1) != game.Peace || a.e.AbilityUsed != g.Day || a.toast == "" {
		t.Errorf("after the press: stance %v, used %d, toast %q", g.Empires[0].StanceWith(1), a.e.AbilityUsed, a.toast)
	}
	if ls := abilityLines(a); ls[1] != fmt.Sprintf("in %d days", game.Baliflids.Info().Recharge) {
		t.Errorf("recharging: %q", ls)
	}
	n := len(a.log)
	s.press(a, abilityButton)
	if len(a.log) != n || !strings.Contains(a.toast, "ready in") {
		t.Errorf("recharging press: log %d -> %d, toast %q", n, len(a.log), a.toast)
	}
	a = testApp(game.NewGame(1, []game.Species{game.Orfa}, game.Average))
	a.push(newStarmapScreen())
	if ls := abilityLines(a); ls[1] != "passive" {
		t.Errorf("Orfa: %q", ls)
	}
	s.press(a, abilityButton)
	if a.e.AbilityUsed != 0 || !strings.Contains(a.toast, "passive") {
		t.Errorf("passive press: used %d, toast %q", a.e.AbilityUsed, a.toast)
	}
}

// TestResearchLines: "None" without a topic; the topic's name and its
// days at the empire's rate with one; "Singularity" once all is known.
func TestResearchLines(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Orfa}, game.Average)
	a := testApp(g)
	a.e.Research.Topic = game.NoTech
	if ls := researchLines(a); ls != [2]string{"None", ""} {
		t.Errorf("no topic: %q", ls)
	}
	var topic game.Tech
	for tc := range game.NumTechs {
		if a.e.Research.CanStudy(tc) {
			topic = tc
			break
		}
	}
	if err := a.e.Research.Study(topic); err != nil {
		t.Fatal(err)
	}
	state := "never"
	if d := a.e.Research.Days(topic, a.e.ResearchRate()); d >= 0 {
		state = fmt.Sprintf("in %d days", d)
	}
	want := [2]string{game.Techs[topic].Name, state}
	if ls := researchLines(a); ls != want {
		t.Errorf("topic: %q, want %q", ls, want)
	}
	for tc := range game.NumTechs {
		a.e.Research.Grant(tc)
	}
	if ls := researchLines(a); ls != [2]string{"Singularity", "reached"} {
		t.Errorf("complete: %q", ls)
	}
}

// The Planets and Ships boxes' lines: the colonies
// and their systems; the fleet as the cap counts it, over the cap, and
// the ships under construction on the second line.
func TestPlanetsShipsLines(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Orfa}, game.Average)
	a := testApp(g)
	e := a.e
	if ls := planetsLines(a); ls != [2]string{"1 planet in 1 system", ""} {
		t.Errorf("home alone: %q", ls)
	}
	n, capa := e.Fleet()
	if n != 0 || capa != game.ShipsPerSystem || shipsLines(a) != [2]string{fmt.Sprintf("0/%d", capa), ""} {
		t.Errorf("no ship: fleet %d/%d, lines %q", n, capa, shipsLines(a))
	}
	home := e.Colonies[0]
	sys := e.Cluster.Stars[home.Star].System
	other := -1
	for i := range sys.Planets {
		if i != home.PlanetIdx {
			other = i
			break
		}
	}
	if other < 0 {
		t.Skip("a one-planet home system")
	}
	settler := e.Spawn(fixtureShip, home.Star, other)
	at, ok := sys.Planets[other].BestCell(game.ColonyBase, e.Species.Traits())
	if !ok {
		t.Fatal("no square for the Colony Base")
	}
	if err := e.FoundAt(settler, at); err != nil {
		t.Fatal(err)
	}
	if ls := planetsLines(a); ls != [2]string{"2 planets in 1 system", ""} {
		t.Errorf("two colonies: %q", ls)
	}
	home.Project = &game.Project{Item: game.Ship, Slot: 0, Orbital: true}
	n, capa = e.Fleet()
	if n != 2 || shipsLines(a) != [2]string{fmt.Sprintf("2/%d", capa), "1 building"} {
		t.Errorf("one flying, one building: fleet %d/%d, lines %q", n, capa, shipsLines(a))
	}
}

// The depth pass cuts a lane in laneSegs pieces sorted with the stars
// and the lane ships: a far star goes under a lane's near pieces and
// over its far ones; a star covers its own lanes' ends, a pennant the
// pieces at its depth.
func TestMapOrder(t *testing.T) {
	cl := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average).Cluster
	li := 0
	l := cl.Lanes[li]
	ps := make([]cam.Point, len(cl.Stars))
	for i := range ps {
		ps[i].Depth = 10 // every other star behind
	}
	ps[l.A].Depth, ps[l.B].Depth = 2, 4 // A near, B far
	mid := slices.IndexFunc(ps, func(p cam.Point) bool { return p.Depth == 10 })
	ps[mid].Depth = 3 // between the lane's pieces
	ships := []game.LaneSighting{{Lane: li, From: l.A, Frac: 0.5}}
	o := mapOrder(cl, ps, []int{li}, ships)
	if len(o) != laneSegs+1+len(ps) {
		t.Fatalf("%d draws, want %d", len(o), laneSegs+1+len(ps))
	}
	at := func(it mapItem) int { return slices.Index(o, it) }
	seg := func(k int) int { return at(mapItem{itemSeg, li, k}) }
	for k := 1; k < laneSegs; k++ {
		if seg(k) > seg(k-1) { // piece 0 at A, the near end
			t.Errorf("piece %d drawn after piece %d", k, k-1)
		}
	}
	if a, b := at(mapItem{itemStar, l.A, 0}), at(mapItem{itemStar, l.B, 0}); a < seg(0) || b < seg(laneSegs-1) {
		t.Errorf("a star under its lane's end: A %d after piece 0 %d? B %d after the last %d?", a, seg(0), b, seg(laneSegs-1))
	}
	if m := at(mapItem{itemStar, mid, 0}); m < seg(laneSegs-1) || m > seg(0) {
		t.Errorf("the star at depth 3 at %d, not between the far piece %d and the near one %d", m, seg(laneSegs-1), seg(0))
	}
	// At depth 3: over the piece ending there (a tie), under the nearer.
	if p := at(mapItem{itemShip, 0, 0}); p < seg(laneSegs/2-1) || p > seg(laneSegs/2-2) {
		t.Errorf("the pennant at mid-lane at %d, pieces %d and %d", p, seg(laneSegs/2-1), seg(laneSegs/2-2))
	}
	if slices.ContainsFunc(mapOrder(cl, ps, nil, nil), func(it mapItem) bool { return it.kind != itemStar }) {
		t.Error("pieces with the lanes hidden")
	}
}

// A ship in a lane is drawn on it: from the star it left toward the far
// one, further each day, at the far star on Arrive, its depth between
// theirs.
func TestLaneAt(t *testing.T) {
	a := fleetApp(t)
	e, c := a.e, a.cl
	sh := e.Ships[1]
	ps := make([]cam.Point, len(c.Stars))
	far := c.Other(sh.Lane, sh.Star)
	ps[sh.Star], ps[far] = cam.Point{X: 100, Y: 200, Depth: 3}, cam.Point{X: 300, Y: 600, Depth: 5}
	prev := -1.0
	for d := e.Day; d <= sh.Arrive; d++ {
		f := sh.LaneProgress(c, d)
		x, y, z := laneAt(c, ps, game.LaneSighting{Lane: sh.Lane, From: sh.Star, Frac: f})
		if f < prev || f < 0 || f > 1 || math.Abs(x-(100+200*f)) > 1e-9 || math.Abs(y-(200+400*f)) > 1e-9 || math.Abs(z-(3+2*f)) > 1e-9 {
			t.Fatalf("day %d: frac %.3f (prev %.3f), at (%.1f,%.1f) depth %.2f", d, f, prev, x, y, z)
		}
		prev = f
	}
	if prev != 1 {
		t.Errorf("on Arrive: frac %.3f, want 1", prev)
	}
}

// Several colonists at a star: the dominant first, by colony count, then
// population, then empire index.
func TestStarColonists(t *testing.T) {
	type col struct{ owner, pl, pop int }
	for _, tc := range []struct {
		cols []col
		want []int
	}{
		{[]col{{2, 0, 3}}, []int{2}},
		{[]col{{2, 0, 3}, {1, 1, 5}}, []int{1, 2}},            // as many colonies, more people
		{[]col{{2, 0, 3}, {1, 1, 5}, {2, 2, 1}}, []int{2, 1}}, // more colonies beat people
		{[]col{{2, 0, 4}, {1, 1, 4}}, []int{1, 2}},            // a tie: the lower index
	} {
		g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
		cl := g.Cluster
		star := -1
		for i, st := range cl.Stars {
			if len(st.System.Planets) >= 3 && st.System.Colonist() < 0 {
				star = i
				break
			}
		}
		if star < 0 {
			t.Fatal("fixture: no free three-planet star")
		}
		for _, c := range tc.cols {
			p := cl.Stars[star].System.Planets[c.pl]
			at, ok := p.BestCell(game.ColonyBase, g.Empires[c.owner].Species.Traits())
			if !ok {
				t.Fatalf("fixture: no square on %s", p.Name)
			}
			p.Surface[at].Item, p.Owner = game.ColonyBase, c.owner
			nc := game.NewColony(p)
			nc.Star, nc.PlanetIdx, nc.Pop = star, c.pl, c.pop
			g.Empires[c.owner].Colonies = append(g.Empires[c.owner].Colonies, nc)
		}
		if got := starColonists(nil, g, cl, star); !slices.Equal(got, tc.want) {
			t.Errorf("%v: %v, want %v", tc.cols, got, tc.want)
		}
	}
}

// A Surface Cloaker hides its colony's people from the ranking: as many
// colonies, the cloaked one's population counts for its owner and
// ScanAll alone, the others rank on what they see.
func TestStarColonistsCloaked(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
	cl := g.Cluster
	star := -1
	for i, st := range cl.Stars {
		if len(st.System.Planets) >= 2 && st.System.Colonist() < 0 {
			star = i
			break
		}
	}
	if star < 0 {
		t.Fatal("fixture: no free two-planet star")
	}
	var big *game.Planet
	for k, c := range []struct{ owner, pop int }{{2, 3}, {1, 5}} {
		p := cl.Stars[star].System.Planets[k]
		at, ok := p.BestCell(game.ColonyBase, g.Empires[c.owner].Species.Traits())
		if !ok {
			t.Fatalf("fixture: no square on %s", p.Name)
		}
		p.Surface[at].Item, p.Owner = game.ColonyBase, c.owner
		nc := game.NewColony(p)
		nc.Star, nc.PlanetIdx, nc.Pop = star, k, c.pop
		g.Empires[c.owner].Colonies = append(g.Empires[c.owner].Colonies, nc)
		big = p
	}
	for _, c := range big.Cells() {
		if big.Surface[c].Item == game.NoItem {
			big.Surface[c].Item = game.SurfaceCloaker
			break
		}
	}
	seen := starColonists(nil, g, cl, star) // every population counted
	if !slices.Equal(seen, []int{1, 2}) {
		t.Fatalf("fixture: %v", seen)
	}
	e0, e1 := g.Empires[0], g.Empires[1]
	if e0.SeesSurface(big) {
		t.Fatal("fixture: the surface seen")
	}
	if got := starColonists(e0, g, cl, star); !slices.Equal(got, []int{2, 1}) {
		t.Errorf("the cloaked people counted: %v", got)
	}
	if got := starColonists(e1, g, cl, star); !slices.Equal(got, seen) {
		t.Errorf("the owner: %v", got)
	}
	e0.ScanAll = true
	if got := starColonists(e0, g, cl, star); !slices.Equal(got, seen) {
		t.Errorf("ScanAll: %v", got)
	}
}
