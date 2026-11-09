package main

import (
	"math"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/cam"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The layout from game.Techs: a tech's level is 1 + its prerequisites'
// highest (0 for none), the roots at −120°, 0°, +120° in index order,
// every level's circular gaps at least its g, and two builds equal.
func TestTreeLayout(t *testing.T) {
	var roots []game.Tech
	for tech := range game.NumTechs {
		want := 0
		for _, p := range game.Techs[tech].Requires {
			want = max(want, resTree[p].level+1)
		}
		if len(game.Techs[tech].Requires) == 0 {
			roots = append(roots, tech)
		}
		if resTree[tech].level != want {
			t.Errorf("%v: level %d, want %d", tech, resTree[tech].level, want)
		}
		if resTree[tech].level >= resLevels {
			t.Errorf("%v: level %d of %d", tech, resTree[tech].level, resLevels)
		}
	}
	if len(roots) != 3 {
		t.Fatalf("roots %v", roots)
	}
	for i, r := range roots {
		if want := float64(i-1) * 2 * math.Pi / 3; math.Abs(resTree[r].theta-want) > 1e-12 {
			t.Errorf("root %v at %.4f, want %.4f", r, resTree[r].theta, want)
		}
	}
	byLevel := make([][]float64, resLevels)
	for tech := range game.NumTechs {
		byLevel[resTree[tech].level] = append(byLevel[resTree[tech].level], resTree[tech].theta)
	}
	slack := math.Inf(1)
	for l, th := range byLevel {
		if len(th) == 0 {
			t.Errorf("level %d empty", l)
			continue
		}
		slices.Sort(th)
		g := resGap(len(th))
		for i := range th {
			if th[i] < -math.Pi || th[i] > math.Pi {
				t.Errorf("level %d: angle %.4f out of [−π, π]", l, th[i])
			}
			if len(th) < 2 {
				continue
			}
			gap := th[(i+1)%len(th)] - th[i]
			if i == len(th)-1 {
				gap += 2 * math.Pi
			}
			slack = min(slack, gap-g)
			if gap < g-1e-6 {
				t.Errorf("level %d (%d techs): gap %.2f° < %.2f°", l, len(th), gap*180/math.Pi, g*180/math.Pi)
			}
		}
	}
	t.Logf("%d levels, least gap slack %.2e rad", resLevels, slack)
	if again, n := layoutTree(); again != resTree || n != resLevels {
		t.Error("two layouts differ")
	}
}

// treeApp is a fresh game's app with the research screen pushed (Enter
// centres the view).
func treeApp(t *testing.T) (*app, *researchScreen) {
	t.Helper()
	a := testApp(game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average))
	s := newResearchScreen()
	a.push(s)
	return a, s
}

// A click (press, release without moving) on a studiable root's centre
// studies it and hovers it; a locked tech is neither drawn nor hovered nor
// picked at its projected centre.
func TestTreeClick(t *testing.T) {
	a, s := treeApp(t)
	r := a.e.Research
	av := r.Available()
	if len(av) == 0 || r.Topic != game.NoTech {
		t.Fatalf("available %v, topic %v", av, r.Topic)
	}
	var root game.Tech = game.NoTech
	for _, tech := range av {
		if len(game.Techs[tech].Requires) == 0 {
			root = tech
			break
		}
	}
	if root == game.NoTech {
		t.Fatalf("no studiable root in %v", av)
	}
	p := s.treeProject(root)
	if !resArea.has(p.X, p.Y) {
		t.Fatalf("%v projects at (%.0f, %.0f), off the area", root, p.X, p.Y)
	}
	s.treeInput(a, p.X, p.Y, false, false, false, 0, 0, 0)
	if s.hover != root {
		t.Errorf("hover %v at %v's centre", s.hover, root)
	}
	s.treeInput(a, p.X, p.Y, true, false, false, 0, 0, 0)
	s.treeInput(a, p.X, p.Y, false, true, false, 0, 0, 0)
	if r.Topic != root {
		t.Errorf("click on %v: topic %v", root, r.Topic)
	}

	// The locked techs: none drawn; at a locked one's centre, no pick.
	shown := map[game.Tech]bool{}
	for _, b := range s.bubbles(r) {
		shown[b.t] = true
	}
	tried := 0
	for tech := range game.NumTechs {
		if r.Known[tech] || r.CanStudy(tech) || tech == r.Topic {
			if !shown[tech] {
				t.Errorf("%v not drawn", tech)
			}
			continue
		}
		if shown[tech] {
			t.Errorf("locked %v drawn", tech)
		}
		s.yaw, s.yc = resTree[tech].theta, -float64(resTree[tech].level)*resLevelH
		q := s.treeProject(tech)
		s.treeInput(a, q.X, q.Y, true, false, false, 0, 0, 0)
		s.treeInput(a, q.X, q.Y, false, true, false, 0, 0, 0)
		if r.Topic != root || s.hover == tech {
			t.Errorf("locked %v: topic %v, hover %v", tech, r.Topic, s.hover)
		}
		tried++
	}
	if tried == 0 {
		t.Error("no locked tech tried")
	}
}

// A drag turns and moves along the axis without picking; the arrows turn
// and move each tick; the wheel zooms (rings grow) within cam's limits;
// yc stays within the levels; R re-centres on the real Update.
func TestTreeSteer(t *testing.T) {
	a, s := treeApp(t)
	r := a.e.Research
	cx, cy := float64(resArea.X0+resArea.X1)/2, float64(resArea.Y0+resArea.Y1)/2
	s.yc = -2 * resLevelH
	yaw, yc := s.yaw, s.yc
	s.treeInput(a, cx, cy, true, false, false, 0, 0, 0)
	s.treeInput(a, cx+40, cy+30, false, false, true, 0, 0, 0)
	if s.yaw >= yaw || s.yc <= yc {
		t.Errorf("drag right and down: yaw %.3f -> %.3f, yc %.3f -> %.3f", yaw, s.yaw, yc, s.yc)
	}
	s.treeInput(a, cx+40, cy+30, false, true, false, 0, 0, 0)
	if r.Topic != game.NoTech {
		t.Errorf("a drag picked %v", r.Topic)
	}
	// A press outside the area drags nothing.
	yaw, yc = s.yaw, s.yc
	s.treeInput(a, float64(resBox.X0+10), cy, true, false, false, 0, 0, 0)
	s.treeInput(a, float64(resBox.X0+60), cy+50, false, false, true, 0, 0, 0)
	s.treeInput(a, float64(resBox.X0+60), cy+50, false, true, false, 0, 0, 0)
	if s.yaw != yaw || s.yc != yc {
		t.Error("a drag from the box moved the tree")
	}
	// The arrows: right turns as a drag right, down moves as a drag down.
	for _, c := range []struct{ turn, along, dyaw, dyc float64 }{
		{1, 0, -resSpinV, 0}, {-1, 0, resSpinV, 0}, {0, 1, 0, resScrollV}, {0, -1, 0, -resScrollV},
	} {
		yaw, yc := s.yaw, s.yc
		s.treeInput(a, cx, cy, false, false, false, 0, c.turn, c.along)
		if math.Abs(s.yaw-yaw-c.dyaw) > 1e-12 || math.Abs(s.yc-yc-c.dyc) > 1e-12 {
			t.Errorf("arrows %v/%v: dyaw %.4f dyc %.4f", c.turn, c.along, s.yaw-yaw, s.yc-yc)
		}
	}
	// Down pulls the scene down (the starmap's pan): toward the roots.
	for range 200 {
		s.treeInput(a, cx, cy, false, false, false, 0, 0, 1)
	}
	if s.yc != 0 {
		t.Errorf("down past the roots: yc %.3f", s.yc)
	}
	for range 1000 {
		s.treeInput(a, cx, cy, false, false, false, 0, 0, -1)
	}
	if want := -float64(resLevels-1) * resLevelH; math.Abs(s.yc-want) > 1e-12 {
		t.Errorf("up past the last level: yc %.3f, want %.3f", s.yc, want)
	}
	// The wheel: up zooms in, the rings grow; clamped both ways.
	tech := r.Available()[0]
	k0 := s.bubbles(r)[0].k
	s.treeInput(a, cx, cy, false, false, false, 1, 0, 0)
	if s.treeFocal() <= resFocal || s.bubbles(r)[0].k <= k0 {
		t.Errorf("wheel up: focal %.0f, k %.3f -> %.3f", s.treeFocal(), k0, s.bubbles(r)[0].k)
	}
	for range 100 {
		s.treeInput(a, cx, cy, false, false, false, 1, 0, 0)
	}
	if s.treeFocal() != cam.MaxFocal {
		t.Errorf("zoomed in: focal %.0f", s.treeFocal())
	}
	for range 100 {
		s.treeInput(a, cx, cy, false, false, false, -1, 0, 0)
	}
	if s.treeFocal() != cam.MinFocal {
		t.Errorf("zoomed out: focal %.0f", s.treeFocal())
	}
	// R: back to Enter's view.
	pressing(t, ebiten.KeyR)
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	if s.treeFocal() != resFocal || s.yaw != resTree[tech].theta || s.yc != -float64(resTree[tech].level)*resLevelH {
		t.Errorf("R: focal %.0f yaw %.3f yc %.3f", s.treeFocal(), s.yaw, s.yc)
	}
}

// Enter centres on the topic: its angle to the front, its level's height;
// no topic, the first studiable tech.
func TestTreeCentre(t *testing.T) {
	a, s := treeApp(t)
	r := a.e.Research
	first := r.Available()[0]
	if s.yaw != resTree[first].theta || s.yc != -float64(resTree[first].level)*resLevelH {
		t.Errorf("no topic: yaw %.3f yc %.3f, want %v's", s.yaw, s.yc, first)
	}
	// A deeper topic: grant its prerequisites' closure.
	topic := game.SpacetimeSurfing
	var grant func(game.Tech)
	grant = func(tech game.Tech) {
		for _, p := range game.Techs[tech].Requires {
			grant(p)
			r.Grant(p)
		}
	}
	grant(topic)
	if err := r.Study(topic); err != nil {
		t.Fatal(err)
	}
	s.Enter(a)
	p := s.treeProject(topic)
	cx := float64(resArea.X0+resArea.X1) / 2
	if math.Abs(p.X-cx) > 1e-9 || resTree[topic].level == 0 || s.yc != -float64(resTree[topic].level)*resLevelH {
		t.Errorf("topic %v at x %.1f (centre %.1f), yc %.3f", topic, p.X, cx, s.yc)
	}
}

// The paint depth z orders only the cylinder's near and far sides: a
// front tech high above the view's height still paints over a back one
// at it, and two techs at one angle on different levels tie.
func TestTreePaintDepth(t *testing.T) {
	s := &researchScreen{}
	r := game.NewEmpire(1, game.Snovemdomas, game.Average).Research
	for tech := range game.NumTechs {
		r.Known[tech] = true
	}
	s.yc = -float64(resLevels-1) * resLevelH // the deepest level: every other above
	bs := s.bubbles(r)
	for _, a := range bs {
		for _, b := range bs {
			ca, cb := math.Cos(resTree[a.t].theta), math.Cos(resTree[b.t].theta)
			if ca > cb+1e-9 && a.z >= b.z {
				t.Fatalf("%v (front, level %d) z %.4f not under %v (level %d) z %.4f", a.t, resTree[a.t].level, a.z, b.t, resTree[b.t].level, b.z)
			}
		}
	}
	for i := 1; i < len(bs); i++ {
		if bs[i].z > bs[i-1].z {
			t.Fatalf("bubbles not back to front at %d", i)
		}
	}
}

// A click on a known tech selects it without touching the topic; the
// right box shows the hover, else the selection, else the topic; a click
// on nothing drops the selection; a click on a studiable one studies it
// and drops it too.
func TestTreeSelect(t *testing.T) {
	a, s := treeApp(t)
	r := a.e.Research
	av := r.Available()
	if len(av) < 2 {
		t.Fatalf("available %v", av)
	}
	topic, known := av[0], av[1]
	if err := a.e.Study(topic); err != nil {
		t.Fatal(err)
	}
	r.Known[known] = true
	click := func(x, y float64) {
		s.treeInput(a, x, y, true, false, false, 0, 0, 0)
		s.treeInput(a, x, y, false, true, false, 0, 0, 0)
	}
	s.yaw, s.yc = resTree[known].theta, -float64(resTree[known].level)*resLevelH
	p := s.treeProject(known)
	click(p.X, p.Y)
	if s.sel != known || r.Topic != topic {
		t.Fatalf("click on known %v: sel %v, topic %v", known, s.sel, r.Topic)
	}
	// Off the bubble (the title strip: no assets for the box's rows here):
	// the selection shows.
	s.treeInput(a, float64(resArea.X0+padX), 4, false, false, false, 0, 0, 0)
	if s.hover != game.NoTech || s.shown(r) != known {
		t.Errorf("off the bubbles: hover %v, shown %v, want %v", s.hover, s.shown(r), known)
	}
	// A click on the area's empty corner drops it: the topic shows.
	click(float64(resArea.X0+2), float64(resArea.Y1-2))
	if s.sel != game.NoTech || s.shown(r) != topic {
		t.Errorf("empty click: sel %v, shown %v", s.sel, s.shown(r))
	}
	// A studiable one studies and leaves no selection.
	s.sel = known
	var other game.Tech = game.NoTech
	for _, tech := range r.Available() {
		if tech != topic {
			other = tech
			break
		}
	}
	if other == game.NoTech {
		t.Fatal("no other studiable tech")
	}
	s.yaw, s.yc = resTree[other].theta, -float64(resTree[other].level)*resLevelH
	q := s.treeProject(other)
	click(q.X, q.Y)
	if r.Topic != other || s.sel != game.NoTech {
		t.Errorf("click on studiable %v: topic %v, sel %v", other, r.Topic, s.sel)
	}
}

// Select current drops the selection and turns the view back on the
// topic; its label fits the button.
func TestSelectCurrent(t *testing.T) {
	a, s := treeApp(t)
	r := a.e.Research
	topic := r.Available()[0]
	if err := a.e.Study(topic); err != nil {
		t.Fatal(err)
	}
	s.sel, s.yaw, s.yc, s.focal = game.OrbitalStructures, 1, -3, 1000
	s.selectCurrent(r)
	if s.sel != game.NoTech || s.shown(r) != topic || s.yaw != resTree[topic].theta || s.yc != -float64(resTree[topic].level)*resLevelH || s.focal != 0 {
		t.Errorf("sel %v shown %v yaw %v yc %v focal %v", s.sel, s.shown(r), s.yaw, s.yc, s.focal)
	}
	if w := ui.Width(resCurrentLabel, bigFace); w > float64(resCurrent.X1-resCurrent.X0) {
		t.Errorf("label %.0f px in a %d px button", w, resCurrent.X1-resCurrent.X0)
	}
}
