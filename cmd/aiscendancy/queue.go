package main

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The planet screen's construction queue (game/queue.go). While the
// colony runs a project, or its queue waits for workers (enqueuing), the
// picker and the structure dialog queue instead of starting. Queued
// targets are drawn as ghosts of their item (unbuilt, a dim outline)
// numbered from 1; a click on one removes its last entry. A stack of the
// entries' icons, the head at the bottom, stands right of the globe: a
// click removes an entry, its ▼ (shown on the hovered row, not the
// head's) moves it forward one, toward the head. The project panel's
// cross asks to cancel the project; a yes starts the head at the next
// Update (nextDue).

// The stack, native px: a column of rows queueH high under the project
// buttons, right-aligned with them (x up to 1258), from below the last
// of the five planet-wide projects' buttons (the most there can be) down
// to the status line, built from the bottom (the head's row fixed); its
// ▼ left of the icon, clear of a size-5 grid's x 1160 (over the globe,
// never a square). queueRows rows: past them the last row shows "+N".
const (
	queueIconW, queueH = 56, 44
	queueGap           = 2
	queueUpW           = 20
	queueX0            = projColX1 - queueIconW
	queueY0            = projColY0 + 5*(projBtnH+projBtnGap) + 4
)

// queueY1 is the stack's bottom, above the status line.
var (
	queueY1   = planetArea.Y1 - 12*zoom - 4
	queueRows = (queueY1 - queueY0 + queueGap) / (queueH + queueGap)
)

// cancelRect is the project panel's cancel cross, its top-left corner.
var cancelRect = rect{panels[4].X0 + 4, panels[4].Y0 + 4, panels[4].X0 + 26, panels[4].Y0 + 26}

// ghostColour is a queued target's outline.
var ghostColour = dim(textColour, 0.55)

// enqueuing reports the player's colony queues rather than starts: a
// project Running, or a queue waiting (for workers).
func (s *planetScreen) enqueuing() bool {
	return s.col != nil && (s.col.Running() || len(s.col.Queue) > 0)
}

// plan is the planet as the player's colony plans it while enqueuing
// (Colony.Planned, a copy), else the planet itself.
func (s *planetScreen) plan() *game.Planet {
	if s.enqueuing() {
		return s.col.Planned()
	}
	return s.planet
}

// cancelShown reports the cancel cross is up: a project Running on the
// player's colony.
func (s *planetScreen) cancelShown() bool { return s.col != nil && s.col.Running() }

// queueRect is stack row k's icon cell, row 0 (the head) at the bottom;
// queueUpRect its ▼ button.
func queueRect(k int) rect {
	y := queueY1 - (k+1)*(queueH+queueGap) + queueGap
	return rect{queueX0, y, queueX0 + queueIconW, y + queueH}
}

func queueUpRect(k int) rect {
	r := queueRect(k)
	y := r.Y0 + (queueH-queueUpW)/2
	return rect{r.X0 - 2 - queueUpW, y, r.X0 - 2, y + queueUpW}
}

// queueShown is the entries of n the stack draws and the "+N" of its last
// row (0: none).
func queueShown(n int) (shown, more int) {
	if n <= queueRows {
		return n, 0
	}
	return queueRows - 1, n - (queueRows - 1)
}

// queueAt is the stack entry under native (x,y), up on its ▼ (never the
// head's); −1 none.
func (s *planetScreen) queueAt(x, y float64) (int, bool) {
	if s.col == nil {
		return -1, false
	}
	shown, _ := queueShown(len(s.col.Queue))
	for k := range shown {
		switch {
		case queueRect(k).has(x, y):
			return k, false
		case k > 0 && queueUpRect(k).has(x, y):
			return k, true
		}
	}
	return -1, false
}

// targets reports entry q's target is the hovered square or slot.
func (s *planetScreen) targets(q game.Queued) bool {
	if q.Orbital {
		return s.slotOK && q.Slot == s.slot
	}
	return s.cellOK && q.Cell == s.cell
}

// queuedAt is the last entry targeting the hovered square or slot, −1
// none (a Factory after its square's Terraforming goes first).
func (s *planetScreen) queuedAt() int {
	if s.col == nil {
		return -1
	}
	for i := len(s.col.Queue) - 1; i >= 0; i-- {
		if s.targets(s.col.Queue[i]) {
			return i
		}
	}
	return -1
}

// dequeue removes entry i, toasted.
func (s *planetScreen) dequeue(a *app, i int) error {
	if i < 0 || i >= len(s.col.Queue) {
		return nil
	}
	it := s.col.Queue[i].Item
	if err := s.col.Dequeue(i); err != nil {
		return err
	}
	a.toastFor(it.String()+" removed from the queue", false)
	return nil
}

// next starts the queue's head (Colony.Next) after the player's cancel
// or Abandon (a worker freed); its Unqueued drops go to the log and flash.
func (s *planetScreen) next(a *app) {
	if s.col == nil || a.e == nil {
		return
	}
	var reps []game.Report
	for _, ev := range s.col.Next(a.e.Research) {
		reps = append(reps, game.Report{Empire: a.player, Event: ev})
	}
	if len(reps) > 0 {
		a.record(reps)
		s.msg, s.msgUntil = reps[0].Event.Text, s.tick+120
	}
}

// queueLine is the status line of stack entry q at index i: what a
// click does there.
func queueLine(q game.Queued, i int, up bool) string {
	if up {
		return fmt.Sprintf("%s (queue %d): click moves it forward", q.Item, i+1)
	}
	return fmt.Sprintf("%s (queue %d): click removes it", q.Item, i+1)
}

// entryCentre is entry q's target's centre on the grid or the orbit.
func (s *planetScreen) entryCentre(q game.Queued) (float64, float64) {
	if q.Orbital {
		return slotCentre(q.Slot)
	}
	return s.grid.Centre(q.Cell)
}

// drawGhosts paints each queued target: its item unbuilt (an Automation
// leaves its structure as drawn) under a dim outline; the hovered stack
// entry's in hoverColor.
func (s *planetScreen) drawGhosts(a *app, sc *ebiten.Image) {
	if s.col == nil {
		return
	}
	for _, q := range s.col.Queue {
		s.drawGhost(a, sc, q)
	}
	if s.qOn >= 0 && s.qOn < len(s.col.Queue) {
		q := s.col.Queue[s.qOn]
		x, y := s.entryCentre(q)
		if q.Orbital {
			strokeSquare(sc, x, y, hoverColor, 2*zoom)
		} else {
			strokeDiamond(sc, x, y, hoverColor, 2*zoom)
		}
	}
}

// drawGhost paints entry q's ghost: its item unbuilt (an Automation
// leaves its structure as drawn) under a dim outline.
func (s *planetScreen) drawGhost(a *app, sc *ebiten.Image, q game.Queued) {
	x, y := s.entryCentre(q)
	switch {
	case q.Orbital:
		drawBuilding(sc, a.spr("planitem", int(q.Item)), x, y, 0)
		strokeSquare(sc, x, y, ghostColour, zoom)
	default:
		if q.Item != game.Automation {
			drawBuilding(sc, a.spr("planitem", int(q.Item)), x, y-halfH, 0)
		}
		strokeDiamond(sc, x, y, ghostColour, zoom)
	}
}

// drawQueueStack paints the stack: each shown entry's picker icon in its
// cell (white outline hovered), the hovered row's ▼ (not the head's), the
// "+N" row's box.
func (s *planetScreen) drawQueueStack(a *app, sc *ebiten.Image) {
	if s.col == nil {
		return
	}
	shown, more := queueShown(len(s.col.Queue))
	for k := range shown {
		r := queueRect(k)
		fillRect(sc, r)
		drawMini(sc, pickerSprite(a, s.col.Queue[k].Item), rect{r.X0 + 4, r.Y0 + 4, r.X1 - 4, r.Y1 - 4})
		c := dim(textColour, 0.5)
		if s.qOn == k {
			c = textColour
		}
		strokeRect(sc, r, c)
		if s.qOn == k && k > 0 {
			u := queueUpRect(k)
			fillRect(sc, u)
			c := textColour
			if s.qUp {
				c = yellow
			}
			strokeRect(sc, u, c)
			drawDownArrow(sc, u, c)
		}
	}
	if more > 0 {
		r := queueRect(shown)
		fillRect(sc, r)
		strokeRect(sc, r, dim(textColour, 0.5))
	}
}

// drawDownArrow fills a ▼ in r, a quarter of it margin.
func drawDownArrow(sc *ebiten.Image, r rect, c color.RGBA) {
	m := float32(r.X1-r.X0) / 4
	x0, y0, x1, y1 := float32(r.X0)+m, float32(r.Y0)+m, float32(r.X1)-m, float32(r.Y1)-m
	var p vector.Path
	p.MoveTo((x0+x1)/2, y1)
	p.LineTo(x1, y0)
	p.LineTo(x0, y0)
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.FillPath(sc, &p, &vector.FillOptions{}, op)
}

// drawCancelCross paints the cancel cross (yellow hovered) when shown.
func (s *planetScreen) drawCancelCross(sc *ebiten.Image) {
	if !s.cancelShown() {
		return
	}
	r := cancelRect
	c := textColour
	if s.xOn {
		c = yellow
	}
	fillRect(sc, r)
	strokeRect(sc, r, c)
	m := float32(6)
	x0, y0, x1, y1 := float32(r.X0)+m, float32(r.Y0)+m, float32(r.X1)-m, float32(r.Y1)-m
	vector.StrokeLine(sc, x0, y0, x1, y1, zoom, c, true)
	vector.StrokeLine(sc, x0, y1, x1, y0, zoom, c, true)
}

// queueMark is a ghost's queue number: the text at its centre.
type queueMark struct {
	t    string
	x, y float64
}

// queueMarks are the ghosts' queue numbers (a square's entries joined,
// "1,2"), first entry first; one touching the open picker's box is left
// out: the box hides its square.
func (s *planetScreen) queueMarks() []queueMark {
	var order []game.Queued // the targets, Item zeroed, first entry first
	nums := map[game.Queued][]string{}
	for i, q := range s.col.Queue {
		q.Item = 0
		if _, ok := nums[q]; !ok {
			order = append(order, q)
		}
		nums[q] = append(nums[q], strconv.Itoa(i+1))
	}
	var out []queueMark
	for _, q := range order {
		x, y := s.entryCentre(q)
		t := strings.Join(nums[q], ",")
		w, h := ui.Width(t, smallFace), smallFace.Size*1.2
		if s.pick != nil && overlaps(s.pick.box(), x-w/2-3, y-h/2, x+w/2+3, y+h/2) {
			continue
		}
		out = append(out, queueMark{t, x, y})
	}
	return out
}

// queueText writes the queueMarks and the stack's numbers: each row's,
// the "+N".
func (s *planetScreen) queueText(frame *ebiten.Image) {
	if s.col == nil {
		return
	}
	for _, m := range s.queueMarks() {
		w, h := ui.Width(m.t, smallFace), smallFace.Size*1.2
		vector.FillRect(frame, float32(m.x-w/2-3), float32(m.y-h/2), float32(w+6), float32(h), color.RGBA{0, 0, 0, 170}, false)
		ui.Print(frame, m.t, smallFace, m.x-w/2, m.y-h/2, textColour)
	}
	shown, more := queueShown(len(s.col.Queue))
	for k := range shown {
		r := queueRect(k)
		ui.Print(frame, strconv.Itoa(k+1), smallFace, float64(r.X0+3), float64(r.Y0+1), dim(textColour, 0.8))
	}
	if more > 0 {
		r := queueRect(shown)
		t := fmt.Sprintf("+%d", more)
		ui.Print(frame, t, bigFace, float64(r.X0+r.X1)/2-ui.Width(t, bigFace)/2, float64(r.Y0+r.Y1)/2-bigFace.Size*0.6, textColour)
	}
}

// overlaps reports the box (x0,y0)-(x1,y1) meeting r.
func overlaps(r rect, x0, y0, x1, y1 float64) bool {
	return x1 > float64(r.X0) && x0 < float64(r.X1) && y1 > float64(r.Y0) && y0 < float64(r.Y1)
}
