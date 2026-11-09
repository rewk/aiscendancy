package main

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// pickerHome is the home world's planet screen hovering a reachable empty
// white square, on a fresh one-empire game.
func pickerHome(t *testing.T) (*app, *planetScreen, *game.Colony) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	a := testApp(g)
	s := newPlanetScreen(e.Cluster.Home, 0)
	s.Enter(a)
	if s.col != e.Colonies[0] {
		t.Fatal("the home world's screen found no colony")
	}
	for _, c := range s.planet.Cells() {
		if sq := s.planet.Surface[c]; sq.Kind == game.White && sq.Item == game.NoItem && s.planet.Reachable(c) {
			s.cell, s.cellOK = c, true
			return a, s, s.col
		}
	}
	t.Fatal("no reachable empty white square at home")
	return nil, nil, nil
}

// On a reachable white square at day 0 the choices are the items research
// allows and the square offers (game.Planet.Offers), in Item order, each
// with the colony's verdict; the starting four are buildable; the click
// opens the picker.
func TestPickerChoices(t *testing.T) {
	a, s, col := pickerHome(t)
	a.e.Research.Known[game.ColonyBase.Info().ResearchReq] = true // known, yet never offered
	var want []game.Item
	for it := game.Item(0); it < game.NumItems; it++ {
		if s.planet.Offers(it, s.cell) && a.e.Research.Allows(it) && it != game.ColonyBase {
			want = append(want, it)
		}
	}
	cs := s.choices(a)
	if len(cs) != len(want) {
		t.Fatalf("%d choices, want %d (%v)", len(cs), len(want), want)
	}
	for i, c := range cs {
		v := col.CanStart(a.e.Research, c.item, s.cell)
		if c.item != want[i] || (c.err == nil) != (v == nil) {
			t.Errorf("choice %d: %s err %v, want %s err %v", i, c.item, c.err, want[i], v)
		}
	}
	for _, it := range []game.Item{game.Factory, game.Agriplot, game.Laboratory, game.Outpost, game.TransportTubes} {
		found := false
		for _, c := range cs {
			if c.item == it {
				found = true
				// Tubes are FlagOnBlack, which game.Planet.CanBuild reads
				// as "black too": a white square takes them.
				if c.err != nil {
					t.Errorf("%s refused on a white square: %v", it, c.err)
				}
			}
		}
		if !found {
			t.Errorf("%s not listed", it)
		}
	}
	if !buildable(cs) || !s.canOpen(a) {
		t.Fatal("nothing buildable")
	}
	push, err := s.clickSlot(a)
	if push != nil || err != nil || s.pick == nil || len(s.pick.cs) != len(cs) {
		t.Fatalf("click: %#v, %v, picker %+v", push, err, s.pick)
	}
	if s.pick.title == "" || s.pick.hover != -1 {
		t.Errorf("picker title %q hover %d", s.pick.title, s.pick.hover)
	}
}

// The picker stays shut with a project running (ErrProjectRunning) or no
// free people (ErrNoFreePop): the first choice's refusal is the flash.
func TestPickerRefused(t *testing.T) {
	a, s, col := pickerHome(t)
	col.Pop = col.Busy()
	if col.Free() != 0 {
		t.Fatalf("free %d", col.Free())
	}
	if _, err := s.clickSlot(a); !errors.Is(err, game.ErrNoFreePop) || s.pick != nil || s.canOpen(a) {
		t.Errorf("no free people: %v, picker %v", err, s.pick)
	}
	col.Pop += 2
	if err := col.Start(a.e.Research, game.Factory, s.cell); err != nil {
		t.Fatal(err)
	}
	if _, err := s.clickSlot(a); err != nil || s.pick != nil || s.confirm == nil || s.canOpen(a) {
		t.Errorf("project running: %v, picker %v, question %v", err, s.pick, s.confirm)
	}
	s.answerCancel(false)
	// Nothing hovered: no picker, no error.
	s.cellOK = false
	if _, err := s.clickSlot(a); err != nil || s.pick != nil {
		t.Errorf("no hover: %v, picker %v", err, s.pick)
	}
}

// With a project running, a click on its square asks to cancel it
// (another spot opens the queuing picker); no (or Esc) changes nothing,
// yes drops it and its industry; a project finished under the question
// is not someone else's to cancel.
func TestPlanetCancelProject(t *testing.T) {
	a, s, col := pickerHome(t)
	col.Pop += 2
	at := s.cell
	if err := col.Start(a.e.Research, game.Factory, at); err != nil {
		t.Fatal(err)
	}
	col.Project.Progress = 7
	pj := col.Project
	other, found := game.Cell{}, false
	for _, c := range s.planet.Cells() {
		if c != at && s.planet.CanBuild(c, game.Factory, col.Traits()) {
			other, found = c, true
			break
		}
	}
	if !found {
		t.Fatal("no second buildable square")
	}
	s.cell = other
	if _, err := s.clickSlot(a); err != nil || s.confirm != nil || s.pick == nil || !strings.HasPrefix(s.pick.title, "Queue at") {
		t.Fatalf("click at %v: %v, question %v, picker %+v", other, err, s.confirm, s.pick)
	}
	s.pick = nil
	for _, spot := range []game.Cell{at} {
		s.cell, s.cellOK = spot, true
		if _, err := s.clickSlot(a); err != nil || s.confirm == nil || s.pick != nil {
			t.Fatalf("click at %v: %v, question %v, picker %v", spot, err, s.confirm, s.pick)
		}
		if l := s.confirm.lines; len(l) != 3 || !strings.Contains(l[0], game.Factory.String()) || !strings.Contains(l[1], "(7 of") {
			t.Errorf("question %q", l)
		}
		if d, ok := col.DaysLeft(); !ok || !strings.HasPrefix(s.confirm.lines[1], strconv.Itoa(d)+" days left") {
			t.Errorf("question %q, days left %d %v", s.confirm.lines[1], d, ok)
		}
		s.answerCancel(false)
		if s.confirm != nil || col.Project != pj || pj.Progress != 7 {
			t.Fatalf("no: question %v, project %+v", s.confirm, col.Project)
		}
	}
	s.clickSlot(a)
	if !s.cancel() || s.confirm != nil || col.Project != pj {
		t.Fatalf("Esc: question %v, project %+v", s.confirm, col.Project)
	}
	if s.cancel() {
		t.Error("a second Esc found something to close: it would not pop the screen")
	}
	s.clickSlot(a)
	s.answerCancel(true)
	if s.confirm != nil || col.Project != nil {
		t.Fatalf("yes: question %v, project %+v", s.confirm, col.Project)
	}
	// A project replaced under the question survives a late yes.
	if err := col.Start(a.e.Research, game.Factory, at); err != nil {
		t.Fatal(err)
	}
	s.clickSlot(a)
	col.Project = &game.Project{Item: game.Agriplot, Cell: at}
	next := col.Project
	s.answerCancel(true)
	if col.Project != next {
		t.Errorf("the late yes cancelled %+v", next)
	}
}

// Every prompt is the structure dialog's box, its answers the button
// bar's halves, side by side and apart (two answers; no "" is OK alone).
func TestQuitPromptRects(t *testing.T) {
	b, y, n := (prompt{[]string{"a", "b", "c"}, "y", "n"}).rects()
	if quitBox != dlgBox || b != dlgBox || y.X0 != dlgBar.X0 || n.X1 != dlgBar.X1 || y.X1 != n.X0 || y.Y0 != dlgBar.Y0 || n.Y1 != dlgBar.Y1 || quitYes != y || quitNo != n {
		t.Errorf("prompt %v %v %v, quit %v %v %v", b, y, n, quitBox, quitYes, quitNo)
	}
}

// at maps each cell's centre to its choice and points outside the cells
// to -1, on a square (22 items, the most: 21 structures and Terraforming,
// in the whole-screen box) and a slot (9, the small box): every cell and
// tile area inside the body, the grid row-major.
func TestPickerAt(t *testing.T) {
	for _, tc := range []struct {
		orbit bool
		n     int
		box   rect
	}{{false, 22, pickFull}, {true, int(game.FirstProject - game.FirstOrbital), pickOrbit}} {
		p := &picker{cs: make([]choice, tc.n), hover: -1, orbit: tc.orbit}
		b := p.box()
		if b != tc.box {
			t.Errorf("orbit %v: box %v, want %v", tc.orbit, b, tc.box)
		}
		for i := range p.cs {
			c := p.cell(i)
			if c.X0 < b.X0 || c.X1 > b.X1 || c.Y0 < p.bodyY() || c.Y1 > p.footY() {
				t.Errorf("orbit %v: cell %d %v outside the body of %v", tc.orbit, i, c, b)
			}
			if got := p.at(float64(c.X0+c.X1)/2, float64(c.Y0+c.Y1)/2); got != i {
				t.Errorf("orbit %v: cell %d's centre maps to %d", tc.orbit, i, got)
			}
			if tl := p.tileArea(i); tl.X0 < c.X0 || tl.X1 > c.X1 || tl.Y0 < c.Y0 || tl.Y1 > c.Y1 {
				t.Errorf("orbit %v: tile area %v outside cell %v", tc.orbit, tl, c)
			}
		}
		for _, pt := range [][2]float64{{float64(b.X0 - 2), float64(b.Y0+b.Y1) / 2}, {float64(b.X0+b.X1) / 2, float64(b.Y0 + 6)},
			{float64(b.X0+b.X1) / 2, float64(b.Y1 - 6)}, {float64(b.X1 + 2), float64(b.Y0+b.Y1) / 2}} {
			if got := p.at(pt[0], pt[1]); got != -1 {
				t.Errorf("orbit %v: %v maps to %d", tc.orbit, pt, got)
			}
		}
		if n := p.cols(); n < len(p.cs) {
			if c := p.cell(n); c.X0 != b.X0 || c.Y0 != p.bodyY()+p.rowH() {
				t.Errorf("orbit %v: choice %d at %v, want row 1 column 0", tc.orbit, n, c)
			}
			// Past the last choice, on its row.
			last := p.cell(len(p.cs) - 1)
			if last.X1+gridColW <= b.X1 {
				if got := p.at(float64(last.X1+gridColW/2), float64(last.Y0+last.Y1)/2); got != -1 {
					t.Errorf("orbit %v: past the last choice maps to %d", tc.orbit, got)
				}
			}
		}
	}
}

// choose starts a buildable choice at the hover and closes the picker; a
// refused choice, or a start the colony refuses since, keeps it open with
// the reason in the footer.
func TestPickerChoose(t *testing.T) {
	a, s, col := pickerHome(t)
	if _, err := s.clickSlot(a); err != nil || s.pick == nil {
		t.Fatalf("no picker: %v", err)
	}
	i := -1
	for k, c := range s.pick.cs {
		if c.item == game.Agriplot {
			i = k
		}
	}
	if i < 0 || s.pick.cs[i].err != nil {
		t.Fatal("no buildable Agriplot choice")
	}

	// Refused: a hand-made choice carrying its refusal.
	p := s.pick
	refused := errors.New("refused here")
	p.cs = append(p.cs, choice{game.Habitat, refused})
	if err := p.choose(a, s, len(p.cs)-1); err != refused || s.pick != p || p.msg != refused.Error() || col.Project != nil {
		t.Errorf("refused choice: %v, picker %p (want %p), msg %q, project %+v", err, s.pick, p, p.msg, col.Project)
	}

	// The colony refuses since: a project started under the picker.
	other := *col
	if err := other.Start(a.e.Research, game.Factory, s.cell); err != nil {
		t.Fatal(err)
	}
	col.Project = other.Project
	// Enqueuing now: the planned square holds the Factory.
	if err := p.choose(a, s, i); err == nil || !strings.Contains(err.Error(), "holds Factory") || s.pick != p || p.msg != err.Error() {
		t.Errorf("start refused: %v, picker %p, msg %q", err, s.pick, p.msg)
	}
	col.Project = nil

	if err := p.choose(a, s, i); err != nil || s.pick != nil {
		t.Fatalf("buildable choice: %v, picker %p", err, s.pick)
	}
	if col.Project == nil || col.Project.Item != game.Agriplot || col.Project.Cell != s.cell {
		t.Errorf("project %+v", col.Project)
	}
}

// cancel (Esc through app.back) closes the picker and says so; with the
// picker shut it leaves Esc to the app.
func TestPickerCancel(t *testing.T) {
	a, s, _ := pickerHome(t)
	if s.cancel() {
		t.Error("cancel with no picker")
	}
	if _, err := s.clickSlot(a); err != nil || s.pick == nil {
		t.Fatalf("no picker: %v", err)
	}
	if !s.cancel() || s.pick != nil {
		t.Error("cancel kept the picker")
	}
}

// An empty orbit slot lists the orbital items research allows, Ship too,
// each with the colony's orbital verdict.
func TestPickerOrbit(t *testing.T) {
	a, s, col := pickerHome(t)
	for _, it := range []game.Item{game.Shipyard, game.Ship, game.OrbitalShield} {
		a.e.Research.Grant(game.Tech(it.Info().ResearchReq))
	}
	slot := -1
	for k, it := range s.planet.Orbit {
		if it == game.NoItem {
			slot = k
			break
		}
	}
	if slot < 0 {
		t.Fatal("no empty orbit slot")
	}
	s.cellOK, s.slot, s.slotOK = false, slot, true
	var want []game.Item
	for it := game.Item(0); it < game.NumItems; it++ {
		if it.Orbital() && a.e.Research.Allows(it) {
			want = append(want, it)
		}
	}
	cs := s.choices(a)
	if len(cs) != len(want) || len(cs) < 2 {
		t.Fatalf("%d choices, want %d (%v)", len(cs), len(want), want)
	}
	for i, c := range cs {
		v := col.CanStartOrbital(a.e.Research, c.item, slot)
		if c.item != want[i] || !c.item.Orbital() || (c.err == nil) != (v == nil) {
			t.Errorf("choice %d: %s err %v, want %s err %v", i, c.item, c.err, want[i], v)
		}
	}
	if _, err := s.clickSlot(a); err != nil || s.pick == nil || s.pick.title == "" {
		t.Fatalf("slot click: %v, picker %+v", err, s.pick)
	}
	// With a Shipyard, Ship is buildable and its choice opens the
	// blank design screen for this colony, starting no project.
	s.planet.Orbit[(slot+1)%game.OrbitSlots] = game.Shipyard
	col.Pop += game.Shipyard.Info().UsePop // its workers: the Ship's crew stays free
	s.pick = newPicker(s, s.choices(a))
	ship := -1
	for i, c := range s.pick.cs {
		if c.item == game.Ship && c.err == nil {
			ship = i
		}
	}
	if ship < 0 {
		t.Fatalf("no buildable Ship among %v", s.pick.cs)
	}
	if err := s.pick.choose(a, s, ship); err != nil || s.pick != nil || col.Project != nil {
		t.Fatalf("Ship: %v, picker %v, project %v", err, s.pick, col.Project)
	}
	if !blankDesign(a.top(), col) {
		t.Errorf("Ship pushed %#v, want a blank design screen for the colony", a.top())
	}
	// At the fleet cap the picker refuses Ship itself, as BuildShip would.
	for a.e.CanStartShip(col, slot) == nil {
		a.e.Ships = append(a.e.Ships, &game.Starship{Name: "x", Orbit: -1, Lane: -1})
	}
	for _, c := range s.choices(a) {
		if c.item == game.Ship && !errors.Is(c.err, game.ErrShipCap) {
			t.Errorf("Ship at the cap: %v", c.err)
		}
	}
}

// The project buttons: none before research; one per planet-wide project
// researched, in Item order, a column down from the top-right corner,
// clear of every square of every grid size (pixel by pixel, with the
// hover's 2 original px stroke); a click on the first (Alien
// Hospitality, ongoing) starts it; with a project running it asks to
// cancel it, and the squares' hover is orange.
func TestPlanetProjects(t *testing.T) {
	a, s, col := pickerHome(t)
	if bs := s.projButtons(a); len(bs) != 0 {
		t.Fatalf("day 0: %d buttons", len(bs))
	}
	var want []game.Item
	for it := game.FirstProject; it < game.NumItems; it++ {
		if req := it.Info().ResearchReq; req >= 0 && req < int(game.NumTechs) {
			a.e.Research.Grant(game.Tech(req))
			if it.PlanetWide() {
				want = append(want, it)
			}
		}
	}
	bs := s.projButtons(a)
	if len(bs) != len(want) || len(want) == 0 {
		t.Fatalf("%d buttons, want %d: %v", len(bs), len(want), want)
	}
	for i, b := range bs {
		if wantErr := s.col.CanStartProject(a.e.Research, want[i]); b.item != want[i] || fmt.Sprint(b.err) != fmt.Sprint(wantErr) {
			t.Errorf("button %d: %s %v, want %s %v", i, b.item, b.err, want[i], wantErr)
		}
		if i > 0 && b.r.Y0 < bs[i-1].r.Y1 {
			t.Errorf("button %d %v overlaps %v", i, b.r, bs[i-1].r)
		}
		if got, ok := s.projAt(a, float64(b.r.X0), float64(b.r.Y0)); !ok || got.item != b.item {
			t.Errorf("projAt on button %d: %v %v", i, got.item, ok)
		}
	}
	if r := bs[0].r; r.X1 != projColX1 || r.Y0 != projColY0 {
		t.Errorf("column starts at %v, want its top-right at (%d,%d)", r, projColX1, projColY0)
	}
	const stroke = 2 * zoom // the hover outline, centred on the diamond's edge
	for size := 1; size <= game.MaxSize; size++ {
		g := newGrid(size)
		for _, c := range (&game.Planet{Size: size}).Cells() {
			cx, cy := g.Centre(c)
			hw, hh := float64(halfW+stroke), float64(halfH+stroke)
			for _, b := range bs {
				for y := b.r.Y0; y < b.r.Y1; y++ {
					for x := b.r.X0; x < b.r.X1; x++ {
						if math.Abs(float64(x)-cx)/hw+math.Abs(float64(y)-cy)/hh < 1 {
							t.Fatalf("size %d: square %v reaches button %s at (%d,%d)", size, c, b.item, x, y)
						}
					}
				}
			}
		}
	}
	if err := s.clickProject(a, bs[0].item); err != nil || col.Project == nil || col.Project.Item != bs[0].item || s.confirm != nil {
		t.Fatalf("click: %v, project %v", err, col.Project)
	}
	if err := col.Start(a.e.Research, game.Factory, s.cell); err != nil {
		t.Fatal(err)
	}
	if s.hoverColour(a) != orange {
		t.Errorf("hover %v with a project running, want orange", s.hoverColour(a))
	}
	if err := s.clickProject(a, bs[0].item); err != nil || s.confirm == nil {
		t.Errorf("click with a project: %v, question %v", err, s.confirm)
	}
}

// itemFacts lists the non-zero numbers, then the flags; the days round up
// and an idle industry counts as 1 a day. Expectations read off game.Items.
func TestItemFacts(t *testing.T) {
	f, m, sh := game.Factory.Info(), game.Metroplex.Info(), game.OrbitalShield.Info()
	for _, tc := range []struct {
		it            game.Item
		industry      int
		effects, cost string
	}{
		{game.Factory, 7, fmt.Sprintf("ind +%d", f.Industry), fmt.Sprintf("cost %d  %d days", f.Cost, (f.Cost+6)/7)},
		{game.Factory, 0, fmt.Sprintf("ind +%d", f.Industry), fmt.Sprintf("cost %d  %d days", f.Cost, f.Cost)},
		{game.Metroplex, 7, fmt.Sprintf("ind +%d  res +%d  pros +%d  room +%d", m.Industry, m.Research, m.Prosp, m.MaxPop),
			fmt.Sprintf("cost %d  %d days", m.Cost, (m.Cost+6)/7)},
		{game.OrbitalShield, 7, "", fmt.Sprintf("cost %d  %d days  shield", sh.Cost, (sh.Cost+6)/7)},
	} {
		e, c := itemFacts(tc.it, tc.industry)
		if e != tc.effects || c != tc.cost {
			t.Errorf("%s at %d: %q / %q, want %q / %q", tc.it, tc.industry, e, c, tc.effects, tc.cost)
		}
	}
	if e, c := itemFacts(game.Ship, 7); e != "" || strings.Contains(c, strconv.Itoa(game.Ship.Info().Cost)) {
		t.Errorf("Ship's facts %q / %q: planitem's cost is not a ship's", e, c)
	}
	if got, want := factsLine(game.Factory, 7), fmt.Sprintf("ind +%d  cost %d  %d days", f.Industry, f.Cost, (f.Cost+6)/7); got != want {
		t.Errorf("factsLine %q, want %q", got, want)
	}
	if got, want := factsLine(game.OrbitalShield, 7), fmt.Sprintf("cost %d  %d days  shield", sh.Cost, (sh.Cost+6)/7); got != want {
		t.Errorf("factsLine %q, want %q", got, want)
	}
	if f.Prosp != 0 || f.Research != 0 || f.MaxPop != 0 || sh.Industry != 0 || !game.OrbitalShield.Has(game.FlagShield) ||
		m.Industry == 0 || m.Research == 0 || m.Prosp == 0 || m.MaxPop == 0 {
		t.Fatal("the table moved: the cases above prove less")
	}
}

// The picker follows the colony each tick: a tech found adds its item, a
// project started under it (Space with a -bot) closes it.
func TestPickerRefresh(t *testing.T) {
	a, s, col := pickerHome(t)
	if _, err := s.clickSlot(a); err != nil || s.pick == nil {
		t.Fatalf("open: %v", err)
	}
	p := s.pick
	n := len(p.cs)
	tech := game.Tech(game.Habitat.Info().ResearchReq)
	if a.e.Research.Allows(game.Habitat) {
		t.Fatal("Habitat allowed at day 0: the growth below proves nothing")
	}
	a.e.Research.Grant(tech)
	more := 0 // Habitat, and Terraforming on the same tech where offered
	for it := game.Item(0); it < game.NumItems; it++ {
		if s.planet.Offers(it, s.cell) && it.Info().ResearchReq == int(tech) {
			more++
		}
	}
	if !p.refresh(a, s) || len(p.cs) != n+more || s.pick != p {
		t.Fatalf("after %s: %d choices, want %d, picker %v", tech, len(p.cs), n+more, s.pick)
	}
	if err := col.Start(a.e.Research, game.Factory, s.cell); err != nil {
		t.Fatal(err)
	}
	if p.refresh(a, s) || s.pick != nil {
		t.Errorf("a project started under the picker left it open")
	}
}

// Terraforming is offered on a square once researched: buildable on a
// reachable empty black one, as Transport Tubes are, or under tubes on
// black; not listed on white.
func TestPickerTerraforming(t *testing.T) {
	a, s, _ := pickerHome(t)
	white := s.cell
	a.e.Research.Grant(game.Tech(game.Terraforming.Info().ResearchReq))
	verdict := func() (error, bool) {
		for _, c := range s.choices(a) {
			if c.item == game.Terraforming {
				return c.err, true
			}
		}
		return nil, false
	}
	if err, ok := verdict(); ok {
		t.Errorf("white square: Terraforming listed, err %v", err)
	}
	s.planet.Surface[white].Kind = game.Black // the home has no reachable black square
	if err, ok := verdict(); !ok || err != nil {
		t.Errorf("black square %v: Terraforming listed %v, err %v", white, ok, err)
	}
	// Tubes on it: the click on the built square opens their dialog, its
	// Terraform button first, which starts the project there.
	s.planet.Surface[white].Item = game.TransportTubes
	s.col.Pop++ // the tubes' worker
	if _, err := s.clickSlot(a); err != nil || s.pick != nil || s.dlg == nil {
		t.Fatalf("tubes on black: %v, picker %v, dialog %v", err, s.pick, s.dlg)
	}
	if acts := s.dlgActs(a); len(acts) == 0 || acts[0] != dlgTerraform {
		t.Errorf("tubes' dialog %v, want Terraform first", acts)
	}
	if push, err := s.press(a, dlgTerraform); push != nil || err != nil || s.dlg != nil {
		t.Errorf("Terraform pressed: %#v, %v, dialog %v", push, err, s.dlg)
	}
	if pj := s.col.Project; pj == nil || pj.Item != game.Terraforming || pj.Cell != white {
		t.Errorf("project %+v, want Terraforming at %v", pj, white)
	}
	// White under tubes: no Terraform button.
	s.col.Cancel()
	s.planet.Surface[white].Kind = game.White
	if s.openDlg(); slices.Contains(s.dlgActs(a), dlgTerraform) {
		t.Errorf("tubes on white: %v", s.dlgActs(a))
	}
}

// A Unique bonus structure the planet holds is drawn blue in the picker
// (built), still a valid choice; the others and a planet without it not.
func TestPickerBuiltBlue(t *testing.T) {
	_, s, _ := pickerHome(t)
	p := newPicker(s, []choice{{game.HyperpowerPlant, nil}, {game.Factory, nil}, {game.Internet, nil}})
	if p.built(p.cs[0]) || p.built(p.cs[1]) || p.built(p.cs[2]) {
		t.Fatal("blue before any plant")
	}
	at, ok := s.planet.BestCell(game.HyperpowerPlant, game.Base)
	if !ok {
		t.Fatal("no cell")
	}
	if err := s.planet.Build(at, game.HyperpowerPlant); err != nil {
		t.Fatal(err)
	}
	if !p.built(p.cs[0]) || p.built(p.cs[1]) || p.built(p.cs[2]) {
		t.Error("only the built plant is blue")
	}
	if (&picker{}).built(choice{game.HyperpowerPlant, nil}) {
		t.Error("no planet: blue")
	}
}

// A yes from the target or the panel's cross cancels and, at the next
// Update, starts the queue's head; a no starts nothing; a yes asked from
// a project button leaves the colony idle (the head waits for the close).
func TestCancelThenNext(t *testing.T) {
	a, s, col := pickerHome(t)
	col.Pop += 2
	at := s.cell
	var other game.Cell
	found := false
	for _, c := range s.planet.Cells() {
		if c != at && s.planet.CanBuild(c, game.Factory, col.Traits()) {
			other, found = c, true
			break
		}
	}
	if !found {
		t.Fatal("no second buildable square")
	}
	start := func() {
		col.Queue = nil
		if err := col.Start(a.e.Research, game.Factory, at); err != nil {
			t.Fatal(err)
		}
		if err := col.Enqueue(a.e.Research, game.Agriplot, other); err != nil {
			t.Fatal(err)
		}
	}
	start()
	s.cell, s.cellOK = at, true
	s.clickSlot(a)
	s.answerCancel(false)
	if s.Update(a); col.Project == nil || col.Project.Item != game.Factory || len(col.Queue) != 1 {
		t.Fatalf("no: project %+v, queue %v", col.Project, col.Queue)
	}
	// The cross, then yes: cancelled at once, the head started by the Update.
	s.hoverAt(a, float64(cancelRect.X0+2), float64(cancelRect.Y0+2))
	if _, err := s.leftClick(a); err != nil || s.confirm == nil {
		t.Fatalf("cross: %v, question %v", err, s.confirm)
	}
	s.answerCancel(true)
	if col.Project != nil || len(col.Queue) != 1 {
		t.Fatalf("yes: project %+v, queue %v before the Update", col.Project, col.Queue)
	}
	if s.Update(a); col.Project == nil || col.Project.Item != game.Agriplot || col.Project.Cell != other || len(col.Queue) != 0 {
		t.Fatalf("yes: project %+v, queue %v, want the Agriplot started", col.Project, col.Queue)
	}
	// From a project button: the colony stays idle.
	col.Cancel()
	start()
	if err := s.clickProject(a, game.LushGrowthBomb); err != nil || s.confirm == nil {
		t.Fatalf("project button: %v, question %v", err, s.confirm)
	}
	s.answerCancel(true)
	if s.Update(a); col.Project != nil || len(col.Queue) != 1 {
		t.Errorf("project button's yes: project %+v, queue %v", col.Project, col.Queue)
	}
}
