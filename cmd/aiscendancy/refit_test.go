package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// docksSlot builds Orbital Docks in the first slot of p that takes them.
func docksSlot(t *testing.T, p *game.Planet) int {
	t.Helper()
	for k := range p.Orbit {
		if p.CanBuildOrbital(k, game.OrbitalDocks) {
			if err := p.BuildOrbital(k, game.OrbitalDocks); err != nil {
				t.Fatal(err)
			}
			return k
		}
	}
	t.Fatalf("%s takes no Orbital Docks", p.Name)
	return -1
}

// docksApp is seed 1 with two empires, shipTechs and the Docks' tech
// granted, Orbital Docks at the player's home (a free worker there), and
// two ships in its orbit: worn, the "Colonizer" template (saved here: an
// empire starts with none) with its last Colonizer swapped for a Proton
// Shaver, and done, the template's design itself (nothing to refit). Its
// system screen is entered, worn selected.
func docksApp(t *testing.T) (a *app, v *systemScreen, col *game.Colony, worn, done *game.Starship) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a = testApp(g)
	e := a.e
	for _, tc := range shipTechs {
		e.Research.Grant(tc)
	}
	e.Research.Grant(game.Tech(game.OrbitalDocks.Info().ResearchReq))
	col = e.Colonies[0]
	docksSlot(t, col.Planet)
	col.Pop = col.Busy() + 1
	if err := e.AddTemplate(game.ColonyTemplate(false)); err != nil {
		t.Fatal(err)
	}
	d, ok := e.Templates[0].Realise(e.Research)
	if !ok {
		t.Fatalf("template %q not realisable", e.Templates[0].Name)
	}
	w := d
	w.Parts = slices.Clone(d.Parts)
	w.Parts[len(w.Parts)-1] = game.ProtonShaver
	worn = e.Spawn(w, col.Star, col.PlanetIdx)
	done = e.Spawn(d, col.Star, col.PlanetIdx)
	v = newSystemScreen(col.Star)
	v.Enter(a)
	v.sel, v.tile, v.aim = worn, -1, false
	return a, v, col, worn, done
}

// refitLine is the foot refitFoot should print for sh at c, from the
// rule's own calls: the template's refit when CanRefit takes it, else
// the hand refit's pointer.
func refitLine(t *testing.T, e *game.Empire, c *game.Colony, sh *game.Starship) string {
	t.Helper()
	d, err := e.RefitTarget(sh)
	if err == nil {
		err = e.CanRefit(c, sh, d)
	}
	if err != nil {
		return "refit: edit the parts at " + c.Planet.Name + "'s Orbital Docks"
	}
	cost := game.RefitCost(sh, d)
	ind, _, _ := c.Output()
	if ind <= 0 {
		t.Fatal("the fixture colony makes no industry")
	}
	return fmt.Sprintf("refit to %s: +%d industry, %s", sh.Design.Name, cost, dayCount(max(1, (cost+ind-1)/ind)))
}

// bareShip spawns worn's design under its hull's name at c: a class
// with no template (Empire.RefitTarget refuses it).
func bareShip(t *testing.T, e *game.Empire, c *game.Colony, worn *game.Starship) *game.Starship {
	t.Helper()
	d := worn.Design
	d.Name = d.Hull.String()
	d.Parts = slices.Clone(d.Parts)
	sh := e.Spawn(d, c.Star, c.PlanetIdx)
	if _, err := e.RefitTarget(sh); err == nil {
		t.Fatalf("a template named %q", d.Name)
	}
	return sh
}

// In orbit of the player's docks refitFoot offers the template's refit
// with the rule's cost and the days at the colony's industry; a ship the
// template does not change, or one with no template, gets the hand
// refit's pointer; away from docks (another planet, a colony whose docks
// are gone, a foreign colony with docks) nothing.
func TestRefitFoot(t *testing.T) {
	a, v, col, worn, done := docksApp(t)
	e := a.e
	d, err := e.RefitTarget(worn)
	if err != nil || game.RefitCost(worn, d) <= 0 {
		t.Fatalf("the worn ship: target %v, cost %d", err, game.RefitCost(worn, d))
	}
	want := refitLine(t, e, col, worn)
	if !strings.HasPrefix(want, "refit to "+worn.Design.Name+": +") {
		t.Fatalf("the fixture refit is refused: %q", want)
	}
	if f := refitFoot(e, v.sel); f != want {
		t.Errorf("foot %q, want %q", f, want)
	}
	hand := "refit: edit the parts at " + col.Planet.Name + "'s Orbital Docks"
	v.sel = done
	if f := refitFoot(e, v.sel); f != hand || refitLine(t, e, col, done) != hand {
		t.Errorf("same design, full hull: %q", f)
	}
	if f := refitFoot(e, bareShip(t, e, col, worn)); f != hand {
		t.Errorf("no template: %q", f)
	}
	// A damaged hull is a repair.
	done.Hull--
	if f, w := refitFoot(e, v.sel), refitLine(t, e, col, done); f != w || !strings.HasPrefix(f, "refit to ") {
		t.Errorf("repair: %q, want %q", f, w)
	}
	done.Hull++

	// Another planet of the system, in orbit: no colony there.
	other := -1
	for i := range a.cl.Stars[col.Star].System.Planets {
		if i != col.PlanetIdx && e.Colony(col.Star, i) == nil {
			other = i
			break
		}
	}
	if other < 0 {
		t.Fatal("the home system has one planet")
	}
	away := e.Spawn(worn.Design, col.Star, other)
	v.sel = away
	if f := refitFoot(e, v.sel); f != "" {
		t.Errorf("away from docks: %q", f)
	}

	// The home without docks.
	v.sel = worn
	k := slices.Index(col.Planet.Orbit[:], game.OrbitalDocks)
	col.Planet.DemolishOrbital(k)
	if f := refitFoot(e, v.sel); f != "" {
		t.Errorf("docks gone: %q", f)
	}
	col.Planet.Orbit[k] = game.OrbitalDocks

	// A foreign colony with docks.
	fc := a.g.Empires[1].Colonies[0]
	docksSlot(t, fc.Planet)
	foe := e.Spawn(worn.Design, fc.Star, fc.PlanetIdx)
	v.sel = foe
	if f := refitFoot(e, v.sel); f != "" {
		t.Errorf("foreign docks: %q", f)
	}
}

// refitScreen presses Refit in sh's dialog on ps, the screen pushed
// returned: the design screen's refit mode on sh at col.
func refitScreen(t *testing.T, a *app, ps *planetScreen, col *game.Colony, sh *game.Starship) *designScreen {
	t.Helper()
	ps.openShipDlg(sh)
	if !slices.Contains(ps.dlgActs(a), dlgRefit) {
		t.Fatalf("no Refit for %s: %v", sh.Name, ps.dlgActs(a))
	}
	next, err := ps.press(a, dlgRefit)
	ds, ok := next.(*designScreen)
	if err != nil || !ok || ps.dlg != nil {
		t.Fatalf("Refit: %v, screen %T, dialog %+v", err, next, ps.dlg)
	}
	a.push(ds)
	ds.Enter(a)
	return ds
}

// Refit at the Docks opens the design screen's refit mode on the ship's
// parts aboard, named as its class, even with no template; removing a
// part then Refit (the button, or Enter) starts the project at the
// rule's cost and pops the screen. A ship as it is is refused "nothing
// to refit", a busy colony's refusal is shown, nothing started; the
// screen stays. A ship gone from the orbit under the screen is refused.
func TestRefitButton(t *testing.T) {
	a, _, col, worn, done := docksApp(t)
	e := a.e
	r := e.Research
	ps := newPlanetScreen(col.Star, col.PlanetIdx)
	a.push(ps)

	bare := bareShip(t, e, col, worn)
	s := refitScreen(t, a, ps, col, bare)
	if s.refit != bare || s.col != col || s.ship != nil || s.t.Name != bare.Design.Name || s.loaded != bare.Design.Name ||
		s.t.Hull != bare.Design.Hull || !slices.Equal(s.d.Parts, bare.Aboard().Parts) {
		t.Fatalf("refit screen %+v", s)
	}
	// As it is: nothing to refit, the button and Enter alike.
	want := e.CanRefit(col, bare, s.d)
	if want == nil || want.Error() != "nothing to refit" {
		t.Fatalf("the ship as it is: %v", want)
	}
	if clickR(a, s, desRefit) || s.msg != want.Error() || !s.msgErr || col.Project != nil {
		t.Errorf("Refit unchanged: msg %q, project %+v", s.msg, col.Project)
	}
	if s.handleKeys(a, nil, desKeys{enter: true}) || col.Project != nil {
		t.Error("Enter refitted the ship as it is")
	}
	if _, v, ok := s.titleLines(e); ok || v != want.Error() {
		t.Errorf("verdict %q %v, want %q", v, ok, want)
	}

	// A part out: a refit at the rule's cost, its title and verdict.
	s.remove(r, len(s.t.Parts)-1)
	if err := e.CanRefit(col, bare, s.d); err != nil {
		t.Fatalf("the part removed: %v", err)
	}
	cost := game.RefitCost(bare, s.d)
	n, ok := col.DaysFor(cost)
	if !ok {
		t.Fatal("no industry at home")
	}
	days := dayCount(max(1, n))
	if l, v, ok := s.titleLines(e); l[1] != fmt.Sprintf("refit +%d industry", cost) || l[2] != days || !ok || v != "valid" {
		t.Errorf("title %q, verdict %q %v", l, v, ok)
	}
	// A blank class is refused, as Build and Save refuse a blank name.
	name := s.t.Name
	s.t.Name = "  "
	if clickR(a, s, desRefit) || s.msg != "a name is needed" || !s.msgErr || col.Project != nil {
		t.Errorf("blank class: msg %q, project %+v", s.msg, col.Project)
	}
	s.t.Name = name

	// Busy: the colony's refusal shown, nothing started.
	at, ok := col.Planet.BestCell(game.Factory, col.Traits())
	if !ok || col.Start(r, game.Factory, at) != nil {
		t.Fatal("no Factory to start")
	}
	busy := e.CanRefit(col, bare, s.d)
	if busy == nil {
		t.Fatal("a refit while a Factory rises")
	}
	if clickR(a, s, desRefit) || s.msg != busy.Error() || col.Project.Item != game.Factory || e.Refitting(bare) != nil {
		t.Errorf("busy: msg %q, project %+v", s.msg, col.Project)
	}
	if l, v, _ := s.titleLines(e); l[2] != "busy: "+days || v != busy.Error() {
		t.Errorf("busy title %q, verdict %q", l, v)
	}
	col.Project = nil

	// Refit: the project is the ship's, the design under the title's name.
	s.typed([]rune("x"))
	parts := slices.Clone(s.d.Parts)
	if !clickR(a, s, desRefit) {
		t.Fatalf("Refit refused: %q", s.msg)
	}
	pj := col.Project
	if e.Refitting(bare) != col || pj.RefitCost != cost || pj.Cost() != cost || !slices.Equal(pj.Design.Parts, parts) || pj.Design.Name != s.t.Name {
		t.Fatalf("project %+v, Refitting %v", pj, e.Refitting(bare))
	}
	if f := refitFoot(e, bare); f != "" {
		t.Errorf("foot while refitted: %q", f)
	}
	ps.openShipDlg(bare)
	if slices.Contains(ps.dlgActs(a), dlgRefit) {
		t.Errorf("Refit offered while refitted: %v", ps.dlgActs(a))
	}
	ps.dlg = nil

	// Enter refits too.
	col.Project = nil
	done.Hull--
	s = refitScreen(t, a, ps, col, done)
	if !s.handleKeys(a, nil, desKeys{enter: true}) || e.Refitting(done) != col {
		t.Fatalf("Enter: msg %q, refitting %v", s.msg, e.Refitting(done))
	}
	col.Project = nil
	done.Hull++

	// The ship left the orbit, then is gone, under the screen: refused.
	s = refitScreen(t, a, ps, col, worn)
	s.remove(r, len(s.t.Parts)-1)
	worn.Orbit = -1
	gone := e.CanRefit(col, worn, s.d)
	if gone == nil || clickR(a, s, desRefit) || s.msg != gone.Error() || col.Project != nil {
		t.Errorf("out of orbit: msg %q, want %v", s.msg, gone)
	}
	e.Ships = slices.DeleteFunc(e.Ships, func(sh *game.Starship) bool { return sh == worn })
	gone = e.CanRefit(col, worn, s.d)
	if gone == nil || clickR(a, s, desRefit) || s.msg != gone.Error() || col.Project != nil {
		t.Errorf("gone: msg %q, want %v", s.msg, gone)
	}
	// The title and Use template read the gone ship without a panic.
	_, _, _ = s.titleLines(e)
	_, _ = s.useTemplate(e)
}

// The refit mode locks the hull (the button refuses, so does Load of
// another hull's template); Use template is offered only with a template
// named as the ship's class (RefitTarget) and loads that Template, its
// Best parts kept.
func TestRefitScreenTemplates(t *testing.T) {
	a, _, col, worn, _ := docksApp(t)
	e := a.e
	ps := newPlanetScreen(col.Star, col.PlanetIdx)
	a.push(ps)
	s := refitScreen(t, a, ps, col, worn)
	h := s.t.Hull
	if clickR(a, s, desHull); s.t.Hull != h || s.msg != "a refit keeps the hull" || !s.msgErr {
		t.Errorf("hull button: %s, msg %q", s.t.Hull, s.msg)
	}

	other := game.Template{Name: "Big", Hull: game.Medium, Parts: fixed(game.TonklinMotor)}
	e.Templates = append(e.Templates, other)
	parts := slices.Clone(s.t.Parts)
	clickR(a, s, desLoad)
	k := len(e.Templates) - 1
	x, _ := centre(desList)
	if s.leftClick(a, x, float64(pickTop(k)+pickPitch/2)); s.t.Hull != h || !slices.Equal(s.t.Parts, parts) ||
		s.msg != "a refit keeps the hull: Big is Medium" || !s.picking {
		t.Errorf("another hull loaded: %s %v, msg %q", s.t.Hull, s.t.Parts, s.msg)
	}
	s.cancel()

	tp, err := e.RefitTarget(worn)
	if err != nil {
		t.Fatal(err)
	}
	if hoverR(a, s, desUse); s.hover != hoverUse {
		t.Errorf("Use template hover %v", s.hover)
	}
	// The template's Best parts come in as today's gizmos: a Best part is
	// a template's, the refit fixes it.
	if clickR(a, s, desUse) || s.editing || !slices.Equal(s.t.Parts, tp.Fixed().Parts) || !slices.Equal(s.d.Parts, tp.Parts) {
		t.Errorf("Use template: %v, want %v", s.t.Parts, tp.Fixed().Parts)
	}
	if !slices.ContainsFunc(e.Templates[0].Parts, func(p game.Part) bool { return p.Best }) {
		t.Fatal("fixture: the template holds no Best part")
	}
	if hoverR(a, s, desRefit); s.hover != hoverRefit {
		t.Errorf("Refit hover %v", s.hover)
	}

	// No template of the class: no Use template.
	s = refitScreen(t, a, ps, col, bareShip(t, e, col, worn))
	if _, ok := s.useTemplate(e); ok {
		t.Error("Use template with no template")
	}
	parts = slices.Clone(s.t.Parts)
	if hoverR(a, s, desUse); s.hover == hoverUse {
		t.Error("Use template hovered with no template")
	}
	if clickR(a, s, desUse); !slices.Equal(s.t.Parts, parts) {
		t.Errorf("a hidden Use template loaded %v", s.t.Parts)
	}
	// A refit mode never has a Build button.
	if hoverR(a, s, desBuild); s.hover == hoverBuild {
		t.Error("Build hovered in the refit mode")
	}
}

// The title box's row in the refit mode: Load, Save, Use template, Refit
// left to right, inside it, btnGap apart, each wide enough for its label.
func TestRefitButtonsFit(t *testing.T) {
	bs := []struct {
		r     rect
		label string
	}{{desLoad, "Load template"}, {desSave, "Save as template"}, {desUse, useLabel}, {desRefit, "Refit"}}
	for i, b := range bs {
		if b.r.Y1 != desTitle.Y1-padY || b.r.X0 < desTitle.X0+padX || b.r.X1 > desTitle.X1-padX ||
			float64(b.r.X1-b.r.X0) < ui.Width(b.label, bigFace)+2*padX {
			t.Errorf("%s at %+v, title %+v", b.label, b.r, desTitle)
		}
		if i > 0 && b.r.X0 < bs[i-1].r.X1+btnGap {
			t.Errorf("%s at %+v overlaps %s at %+v", b.label, b.r, bs[i-1].label, bs[i-1].r)
		}
	}
}

// The planet screen names a running refit "Refit of <ship>" (panel and
// cancel question), draws no project at a slot, does not light the
// Orbital Docks as in progress; cancelling it frees the ship.
func TestRefitPlanetScreen(t *testing.T) {
	a, _, col, worn, _ := docksApp(t)
	d, err := a.e.RefitTarget(worn)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.e.Refit(col, worn, d); err != nil {
		t.Fatal(err)
	}
	pj := col.Project
	s := newPlanetScreen(col.Star, col.PlanetIdx)
	s.Enter(a)
	if s.col != col {
		t.Fatal("the home screen found no colony")
	}
	name := "Refit of " + worn.Name
	if _, pp := s.panelTexts(); pp.name != name {
		t.Errorf("project panel %+v, want %q", pp, name)
	}
	if buildsInPlace(pj) {
		t.Error("the refit is drawn at a slot")
	}
	if s.running(game.OrbitalDocks) {
		t.Error("the Orbital Docks lit as in progress")
	}
	s.hoverAt(a, float64(cancelRect.X0+2), float64(cancelRect.Y0+2)) // the panel's cross: a refit has no target
	if _, err := s.leftClick(a); err != nil || !s.xOn || s.confirm == nil {
		t.Fatalf("click: %v, question %v", err, s.confirm)
	}
	if l := s.confirm.lines; l[0] != name || strings.Contains(strings.Join(l, " "), "slot") {
		t.Errorf("question %q", l)
	}
	s.answerCancel(true)
	if col.Project != nil || a.e.Refitting(worn) != nil {
		t.Fatalf("cancelled: project %+v", col.Project)
	}
	if err := a.e.Order(worn, game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: 0.5}}); err != nil {
		t.Errorf("the freed ship: %v", err)
	}
	// A ship build still draws in place.
	if !buildsInPlace(&game.Project{Item: game.Ship, Orbital: true, Slot: 0}) {
		t.Error("a ship build not drawn at its slot")
	}
}

// Refit while the Docks' colony runs a project puts up the cancel
// question first: No keeps the project and opens
// nothing; Yes cancels it and the next Update pushes the refit screen.
func TestRefitCancelQuestion(t *testing.T) {
	a, _, col, worn, _ := docksApp(t)
	r := a.e.Research
	ps := newPlanetScreen(col.Star, col.PlanetIdx)
	a.push(ps)
	at, ok := col.Planet.BestCell(game.Factory, col.Traits())
	if !ok || col.Start(r, game.Factory, at) != nil {
		t.Fatal("no Factory to start")
	}
	pj := col.Project
	ask := func() {
		t.Helper()
		ps.openShipDlg(worn)
		next, err := ps.press(a, dlgRefit)
		if next != nil || err != nil || ps.dlg != nil || ps.confirm == nil || ps.doomed != pj {
			t.Fatalf("Refit with a project: screen %T, %v, question %v", next, err, ps.confirm)
		}
	}
	n := len(a.stack)
	ask()
	ps.answerCancel(false)
	if err := ps.Update(a); err != nil || col.Project != pj || len(a.stack) != n || ps.refitDue != nil {
		t.Fatalf("No: project %+v, stack %d (was %d)", col.Project, len(a.stack), n)
	}
	ask()
	ps.answerCancel(true)
	if col.Project != nil {
		t.Fatalf("Yes: project %+v kept", col.Project)
	}
	if err := ps.Update(a); err != nil || len(a.stack) != n+1 {
		t.Fatalf("Yes: stack %d, want %d", len(a.stack), n+1)
	}
	if ds, ok := a.stack[n].(*designScreen); !ok || ds.refit != worn || ds.col != col {
		t.Fatalf("pushed %T, not the refit screen on %s", a.stack[n], worn.Name)
	}
	if ps.refitDue != nil || ps.refitAsk != nil {
		t.Error("the pending refit kept")
	}
	// A question from another spot never opens a refit screen.
	a.stack = a.stack[:n]
	if col.Start(r, game.Factory, at) != nil {
		t.Fatal("no Factory to restart")
	}
	ps.askCancel()
	ps.answerCancel(true)
	if ps.Update(a); len(a.stack) != n {
		t.Errorf("a plain cancel pushed %T", a.stack[len(a.stack)-1])
	}
}
