package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/save"
)

// queueHome is pickerHome with a Factory running on the hovered square,
// people to spare and Terraforming researched.
func queueHome(t *testing.T) (*app, *planetScreen, *game.Colony, game.Cell) {
	t.Helper()
	a, s, col := pickerHome(t)
	col.Pop += 4
	a.e.Research.Grant(game.Tech(game.Terraforming.Info().ResearchReq))
	at := s.cell
	if err := col.Start(a.e.Research, game.Factory, at); err != nil {
		t.Fatal(err)
	}
	return a, s, col, at
}

// unreached is an empty square of the planet next to c and reached by no
// structure: open only through a plan placing c.
func unreached(t *testing.T, p *game.Planet, c game.Cell) game.Cell {
	t.Helper()
	for _, n := range []game.Cell{{I: c.I - 1, J: c.J}, {I: c.I + 1, J: c.J}, {I: c.I, J: c.J - 1}, {I: c.I, J: c.J + 1}} {
		if sq, ok := p.Surface[n]; ok && sq.Item == game.NoItem && !sq.Ruins && sq.Kind != game.Black && !p.Reachable(n) {
			return n
		}
	}
	t.Skipf("no unreached square next to %v", c)
	return game.Cell{}
}

// With a project running a click on an empty spot opens the picker in
// queuing mode: its verdicts CanEnqueue's, a pick queues. A square next
// to the running project's target opens, though unreachable today.
func TestQueuePicker(t *testing.T) {
	a, s, col, at := queueHome(t)
	next := unreached(t, s.planet, at)
	if !s.open(next) {
		t.Fatalf("%v next to the running Factory is not open", next)
	}
	s.cell, s.cellOK = next, true
	if _, err := s.clickSlot(a); err != nil || s.pick == nil || s.confirm != nil || !strings.HasPrefix(s.pick.title, "Queue at") {
		t.Fatalf("click: %v, picker %+v, question %v", err, s.pick, s.confirm)
	}
	ag := -1
	for i, c := range s.pick.cs {
		v := col.CanEnqueue(a.e.Research, c.item, next)
		if (c.err == nil) != (v == nil) {
			t.Errorf("%s: choice err %v, CanEnqueue %v", c.item, c.err, v)
		}
		if c.item == game.Agriplot {
			ag = i
		}
	}
	if ag < 0 {
		t.Fatal("no Agriplot choice")
	}
	if err := s.pick.choose(a, s, ag); err != nil || s.pick != nil {
		t.Fatalf("choose: %v, picker %v", err, s.pick)
	}
	if col.Project.Item != game.Factory || !slices.Equal(col.Queue, []game.Queued{{Item: game.Agriplot, Cell: next, Slot: -1}}) {
		t.Errorf("project %+v, queue %+v", col.Project, col.Queue)
	}
	// A queued Unique reads as built: blue, as on the planet at the end.
	internet := choice{item: game.Internet}
	if newPicker(s, nil).built(internet) {
		t.Fatal("fixture: an Internet already")
	}
	col.Queue = append(col.Queue, game.Queued{Item: game.Internet, Cell: unreached(t, s.planet, next), Slot: -1})
	if !newPicker(s, nil).built(internet) {
		t.Error("a queued Internet not blue")
	}
	col.Queue = col.Queue[:1]
	// The orbit: a Ship is listed, refused; a Shipyard queues.
	a.e.Research.Grant(game.OrbitalStructures)
	s.cellOK, s.slot, s.slotOK = false, 9, true
	cs := s.choices(a)
	for _, c := range cs {
		if c.item == game.Ship && c.err == nil {
			t.Error("a Ship can be queued")
		}
	}
	if err := s.start(a, game.Shipyard); err != nil || len(col.Queue) != 2 || !col.Queue[1].Orbital || col.Queue[1].Slot != 9 {
		t.Errorf("Shipyard: %v, queue %+v", err, col.Queue)
	}
}

// A click on a queued square removes its last entry (toasted), a square
// with its Terraforming and a Factory losing the Factory first.
func TestQueueClickRemoves(t *testing.T) {
	a, s, col, at := queueHome(t)
	black := unreached(t, s.planet, at)
	s.planet.Surface[black].Kind = game.Black
	for _, it := range []game.Item{game.Terraforming, game.Agriplot} {
		if err := col.Enqueue(a.e.Research, it, black); err != nil {
			t.Fatal(err)
		}
	}
	s.cell, s.cellOK = black, true
	if s.hoverColour(a) != hoverColor || s.queuedAt() != 1 {
		t.Errorf("hover %v, queued at %d", s.hoverColour(a), s.queuedAt())
	}
	if _, err := s.clickSlot(a); err != nil || len(col.Queue) != 1 || col.Queue[0].Item != game.Terraforming || s.pick != nil {
		t.Fatalf("click: %v, queue %+v", err, col.Queue)
	}
	if a.toast != "Agriplot removed from the queue" {
		t.Errorf("toast %q", a.toast)
	}
	s.clickSlot(a)
	if len(col.Queue) != 0 || s.queuedAt() != -1 {
		t.Errorf("second click: queue %+v", col.Queue)
	}
}

// The stack: a row per entry, the head at the bottom; the ▼ on every row
// but the head's moves it forward, a click on the icon removes it; past
// queueRows the last row is "+N", inert. The hover leaves the grid.
func TestQueueStack(t *testing.T) {
	a, s, col, _ := queueHome(t)
	a.e.Research.Grant(game.OrbitalStructures)
	for k := range 6 {
		if err := col.EnqueueOrbital(a.e.Research, game.Shipyard, k); err != nil {
			t.Fatal(err)
		}
	}
	centre := func(r rect) (float64, float64) { return float64(r.X0+r.X1) / 2, float64(r.Y0+r.Y1) / 2 }
	if shown, more := queueShown(queueRows + 2); shown != queueRows-1 || more != 3 {
		t.Errorf("shown %d more %d of %d", shown, more, queueRows+2)
	}
	if shown, more := queueShown(queueRows); shown != queueRows || more != 0 {
		t.Errorf("a full stack: shown %d more %d", shown, more)
	}
	if r0, r1 := queueRect(0), queueRect(1); r0.Y0 <= r1.Y0 || r0.Y1 != queueY1 {
		t.Errorf("rows %v, %v: the head's row is not the bottom one", r0, r1)
	}
	if queueRows < 8 {
		t.Errorf("%d rows: the stack should reach the status line", queueRows)
	}
	q := col.Queue
	for len(col.Queue) < queueRows+2 { // past the rows (not valid entries: the stack only draws them)
		col.Queue = append(col.Queue, q[0])
	}
	if k, _ := s.queueAt(centre(queueRect(queueRows - 1))); k != -1 {
		t.Errorf("the +N row answers as entry %d", k)
	}
	col.Queue = q
	if k, up := s.queueAt(centre(queueUpRect(0))); k != -1 || up {
		t.Errorf("the head's ▼: %d %v", k, up)
	}
	x, y := centre(queueUpRect(2))
	s.hoverAt(a, x, y)
	if s.qOn != 2 || !s.qUp || s.cellOK || s.slotOK {
		t.Fatalf("hover on row 2's ▼: %d %v cell %v slot %v", s.qOn, s.qUp, s.cellOK, s.slotOK)
	}
	if _, err := s.leftClick(a); err != nil || col.Queue[1].Slot != 2 || col.Queue[2].Slot != 1 {
		t.Errorf("▼: %v, queue %+v", err, col.Queue)
	}
	x, y = centre(queueRect(0))
	s.hoverAt(a, x, y)
	if s.qOn != 0 || s.qUp {
		t.Fatalf("hover on the head: %d %v", s.qOn, s.qUp)
	}
	if got := queueLine(col.Queue[0], 0, false); got != "Shipyard (queue 1): click removes it" {
		t.Errorf("status %q", got)
	}
	if _, err := s.leftClick(a); err != nil || len(col.Queue) != 5 || col.Queue[0].Slot != 2 {
		t.Errorf("remove the head: %v, queue %+v", err, col.Queue)
	}
}

// The stack and the cross sit clear of the grid, the orbit and the
// project buttons: no point of theirs maps to a size-5 grid's square, an
// orbit slot or a button.
func TestQueueStackClear(t *testing.T) {
	g := newGrid(game.MaxSize)
	var rs []rect
	for k := range queueRows {
		rs = append(rs, queueRect(k), queueUpRect(k))
	}
	rs = append(rs, cancelRect)
	for _, r := range rs {
		for x := r.X0; x < r.X1; x += 2 {
			for y := r.Y0; y < r.Y1; y += 2 {
				fx, fy := float64(x), float64(y)
				if c, ok := g.CellAt(fx, fy); ok {
					t.Fatalf("(%d,%d) of %v is square %v", x, y, r, c)
				}
				if k, ok := SlotAt(fx, fy); ok {
					t.Fatalf("(%d,%d) of %v is orbit slot %d", x, y, r, k)
				}
			}
		}
		if last := projColY0 + 5*(projBtnH+projBtnGap); r != cancelRect && r.X1 > projColX1-projBtnW && r.Y0 < last {
			t.Errorf("%v reaches the project buttons (down to y %d)", r, last)
		}
		if r.Y1 > planetArea.Y1-12*zoom {
			t.Errorf("%v reaches the status line", r)
		}
	}
	if p := panels[4]; cancelRect.X0 < p.X0 || cancelRect.Y0 < p.Y0 || cancelRect.X1 > p.X1 || cancelRect.Y1 > p.Y1 {
		t.Errorf("the cross %v outside the project panel %v", cancelRect, p)
	}
}

// Without a Running project the picker starts as before; while the head
// waits for people it queues, and the cross is gone.
func TestQueueEnqueuing(t *testing.T) {
	a, s, col := pickerHome(t)
	if s.enqueuing() || s.cancelShown() {
		t.Fatal("an idle colony enqueues")
	}
	if err := col.Enqueue(a.e.Research, game.Agriplot, s.cell); err != nil {
		t.Fatal(err)
	}
	col.Pop = col.Busy()
	if !s.enqueuing() || s.cancelShown() {
		t.Errorf("waiting queue: enqueuing %v, cross %v", s.enqueuing(), s.cancelShown())
	}
}

// A Built or Unqueued of a colony whose queue started the next project
// does not climb; once idle it does.
func TestQueuedNewsStays(t *testing.T) {
	a, home, other := scopeApp(t)
	m := a.stack[0]
	col := a.e.Colony(home, 0)
	at, ok := col.Planet.BestCell(game.Agriplot, col.Traits())
	if !ok {
		t.Fatal("no square")
	}
	if err := col.Start(a.e.Research, game.Agriplot, at); err != nil {
		t.Fatal(err)
	}
	for _, k := range []game.EventKind{game.Built, game.Unqueued} {
		a.stack = []screen{m}
		a.push(newSystemScreen(other))
		if a.navigate([]game.Event{ev(k, home, 0)}); len(a.stack) != 2 {
			t.Errorf("%v with the queue running: stack %v", k, a.stack)
		}
	}
	col.Cancel()
	a.navigate([]game.Event{ev(game.Unqueued, home, 0)})
	if len(a.stack) != 1 || a.top() != m {
		t.Errorf("idle colony: stack %v", a.stack)
	}
	if queuedOn(a.e, ev(game.Grew, home, 0)) || queuedOn(nil, ev(game.Built, home, 0)) {
		t.Error("queuedOn: a Grew, or no empire")
	}
}

// A Built, Unqueued or Grew recorded while the colony runs nothing, people
// free, is logged Idle, drawn red (dimmed by age like the rest); one its
// queue took over or waits on (a free worker: "Waiting: <head>"), one
// with no free people (the line ending ", no free pop": no order would
// start), or another kind, is not. Idle goes through the save.
func TestIdleNewsRed(t *testing.T) {
	a, home, _ := scopeApp(t)
	col := a.e.Colony(home, 0)
	at, ok := col.Planet.BestCell(game.Agriplot, col.Traits())
	if !ok {
		t.Fatal("no square")
	}
	if err := col.Start(a.e.Research, game.Agriplot, at); err != nil {
		t.Fatal(err)
	}
	a.log = nil
	rep := func(k game.EventKind) []game.Report { return []game.Report{{Empire: a.player, Event: ev(k, home, 0)}} }
	a.record(rep(game.Built))
	col.Cancel()
	a.record(rep(game.Unqueued))
	a.record(rep(game.Grew))
	if got := []bool{a.log[0].Idle, a.log[1].Idle, a.log[2].Idle}; !slices.Equal(got, []bool{false, true, true}) {
		t.Fatalf("Idle %v", got)
	}
	today := a.e.Day
	if c := lineColour(a.log[1], today); c != red {
		t.Errorf("idle today: %v", c)
	}
	if c := lineColour(logEntry{Day: today - 3, Idle: true}, today); c != dim(red, 0.7) {
		t.Errorf("idle 3 days old: %v", c)
	}
	if c := lineColour(a.log[0], today); c != textColour {
		t.Errorf("queued: %v", c)
	}
	arrived := logEntry{Day: today - 9, Ev: ev(game.Arrived, home, -1)}
	if c := lineColour(arrived, today); c != dim(shipNews, 0.45) {
		t.Errorf("a ship's line: %v", c)
	}
	met := logEntry{Day: today, Ev: game.Event{Kind: game.Contacted, Star: -1, Planet: -1, Other: 1}}
	if c := lineColour(met, today); c != speciesNews {
		t.Errorf("a contact's line: %v", c)
	}
	// A queue waiting for a free worker starts on its own: not idle.
	col.Queue = []game.Queued{{Item: game.Factory, Cell: at, Slot: -1}}
	if idleAfter(a.e, ev(game.Built, home, 0)) {
		t.Error("a waiting queue counted idle")
	}
	var pp projPanel
	if pp.fill(col, ""); pp.name != "Waiting: Factory" {
		t.Errorf("waiting panel %q", pp.name)
	}
	if _, _, _, proj, _ := colonyLines(a.e, a.g.Cluster, col); proj != "Waiting: Factory" {
		t.Errorf("waiting row %q", proj)
	}
	col.Pop = col.Busy()
	a.record(rep(game.Built))
	if l := a.log[len(a.log)-1]; l.Idle || !strings.HasSuffix(l.Ev.Text, ", no free pop") {
		t.Errorf("waiting, no free pop: %+v", l)
	}
	col.Queue = nil
	a.record(rep(game.Built)) // idle, but no one free to start anything
	if l := a.log[len(a.log)-1]; l.Idle || !strings.HasSuffix(l.Ev.Text, ", no free pop") {
		t.Errorf("idle, no free pop: %+v", l)
	}
	path := filepath.Join(t.TempDir(), "s.json")
	if err := save.Write(path, a.file()); err != nil {
		t.Fatal(err)
	}
	if err := a.loadFrom(path); err != nil {
		t.Fatal(err)
	}
	if len(a.log) != 5 || !a.log[1].Idle || a.log[0].Idle {
		t.Errorf("reloaded log %+v", a.log)
	}
}

// The planets list sorts by the queue's length, longest first, and the
// project line ends on "queue N".
func TestPlanetsQueueKey(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	c := e.Colonies[0]
	if colonyKey(c, sortQueue) != 0 || queueCount(c) != "" {
		t.Errorf("empty queue: key %d, %q", colonyKey(c, sortQueue), queueCount(c))
	}
	c.Queue = []game.Queued{{Item: game.Factory, Slot: -1}, {Item: game.Agriplot, Slot: -1}}
	if colonyKey(c, sortQueue) != 2 || queueCount(c) != "queue 2" || sortNames[sortQueue] != "queue" {
		t.Errorf("two entries: key %d, %q", colonyKey(c, sortQueue), queueCount(c))
	}
}

// A colony of another empire's planet builds and refits its owner's
// ships: the project's picture comes from the owner's bank.
func TestCapturedColonyShips(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	theirs := g.Empires[1].Colonies[0]
	theirs.Planet.Owner = 0
	if got, want := colonyShips(g, theirs), shipBank(game.Snovemdomas); got != want {
		t.Errorf("captured Orfa colony: %s, want %s", got, want)
	}
}

// A queue waiting for a free worker starts as soon as an Abandon frees
// one, as after a cancel: not at the close, a day of industry lost, the
// panel saying "Waiting" with a worker free.
func TestQueueStartsAfterAbandon(t *testing.T) {
	a, s, f := dlgHome(t)
	col, r := s.col, a.e.Research
	empty, ok := s.planet.BestCell(game.Agriplot, col.Traits())
	if !ok {
		t.Fatal("no empty square")
	}
	col.Pop = col.Busy()
	col.Queue = []game.Queued{{Item: game.Agriplot, Cell: empty, Slot: -1}}
	if col.Next(r); col.Project != nil || len(col.Queue) != 1 {
		t.Fatalf("fixture: the head should wait, project %+v queue %+v", col.Project, col.Queue)
	}
	dlgHoverCell(s, f)
	s.openDlg()
	if _, err := s.press(a, dlgAbandon); err != nil {
		t.Fatal(err)
	}
	if pj := col.Project; pj == nil || pj.Item != game.Agriplot || len(col.Queue) != 0 {
		t.Errorf("after the Abandon: project %+v, queue %+v", pj, col.Queue)
	}
}

// A ghost's queue number is left out where the open orbit picker's box
// covers it, kept elsewhere; without the picker, every target has one.
func TestQueueMarksUnderPicker(t *testing.T) {
	_, s, col, _ := queueHome(t)
	for c, sq := range s.planet.Surface {
		if sq.Item == game.NoItem {
			col.Queue = append(col.Queue, game.Queued{Item: game.Factory, Cell: c, Slot: -1})
		}
	}
	all := s.queueMarks()
	if len(all) != len(col.Queue) {
		t.Fatalf("%d marks for %d entries", len(all), len(col.Queue))
	}
	in := func(m queueMark) bool { return pickOrbit.has(m.x, m.y) }
	if !slices.ContainsFunc(all, in) {
		t.Fatal("fixture: no mark under the orbit picker's box")
	}
	s.pick = &picker{orbit: true}
	kept := s.queueMarks()
	if slices.ContainsFunc(kept, in) || len(kept) == 0 || len(kept) == len(all) {
		t.Errorf("picker open: %d of %d marks kept, some under its box: %v", len(kept), len(all), slices.ContainsFunc(kept, in))
	}
}
